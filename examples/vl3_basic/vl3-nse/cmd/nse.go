// Copyright 2019 Cisco Systems, Inc.
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"github.com/networkservicemesh/examples/examples/universal-cnf/vppagent/pkg/ucnf"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/networkservice"
	"golang.org/x/sync/errgroup"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/danielvladco/k8s-vnet/pkg/nseconfig"
	"github.com/dtornow/cnns-nsr/cnns-ipam/api/ipprovider"
	"github.com/gofrs/uuid"
	"github.com/networkservicemesh/examples/examples/universal-cnf/vppagent/pkg/vppagent"
	"github.com/networkservicemesh/networkservicemesh/pkg/tools"
	"github.com/networkservicemesh/networkservicemesh/sdk/common"
	"github.com/networkservicemesh/networkservicemesh/sdk/endpoint"
	"github.com/sirupsen/logrus"
)

const (
	defaultConfigPath = "/etc/universal-cnf/config.yaml"
	//defaultConfigPath   = "/home/mistrate/go/src/github.com/istratem/examples/config.yaml"
	defaultPluginModule = ""
)

// Flags holds the command line flags as supplied with the binary invocation
type Flags struct {
	ConfigPath string
	Verify     bool
}

type fnGetNseName func() string

// Process will parse the command line flags and init the structure members
func (mf *Flags) Process() {
	flag.StringVar(&mf.ConfigPath, "file", defaultConfigPath, " full path to the configuration file")
	flag.BoolVar(&mf.Verify, "verify", false, "only verify the configuration, don't run")
	flag.Parse()
}

type vL3CompositeEndpoint struct {
	IpamAllocator     ipprovider.AllocatorClient
	registeredSubnets chan *ipprovider.Subnet
	mu                *sync.RWMutex
}

func (e vL3CompositeEndpoint) Cleanup(ctx context.Context) error {
	var errs errors
	e.mu.Lock()
	defer e.mu.Unlock()
	for s := range e.registeredSubnets {
		_, err := e.IpamAllocator.FreeSubnet(ctx, s)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (e vL3CompositeEndpoint) Renew(ctx context.Context, errorHandler func(err error)) error {
	g, ctx := errgroup.WithContext(ctx)
	for subnet := range e.registeredSubnets {
		subnet := subnet
		g.Go(func() error {
			for {
				//TODO: change to subnet.LeaseTime
				//time.NewTicker(time.Duration(s.leaseCheckPeriod) * time.Hour)
				time.Sleep(time.Duration(30) * time.Second)
				_, err := e.IpamAllocator.RenewSubnetLease(ctx, subnet)
				if err != nil {
					errorHandler(err)
				}
			}
		})
	}
	return g.Wait()
}

func (e vL3CompositeEndpoint) AddCompositeEndpoints(nsConfig *common.NSConfiguration, ucnfEndpoint *nseconfig.Endpoint) *[]networkservice.NetworkServiceServer {
	var subnet *ipprovider.Subnet
	for i := 0; i < 6; i++ {
		var err error
		subnet, err = e.IpamAllocator.AllocateSubnet(context.Background(), &ipprovider.SubnetRequest{
			Identifier: &ipprovider.Identifier{
				Fqdn:               ucnfEndpoint.CNNS.Address,
				Name:               ucnfEndpoint.CNNS.Name + uuid.Must(uuid.NewV4()).String(),
				ConnectivityDomain: ucnfEndpoint.CNNS.ConnectivityDomain,
			},
			AddrFamily: &ipprovider.IpFamily{Family: ipprovider.IpFamily_IPV4},
			// TODO default value 24 add the value to config
			PrefixLen: 24,
		})
		if err != nil {
			if i == 5 {
				logrus.Fatal("ipam allocation not successful: ", err)
			}
			logrus.Errorf("ipam allocation not successful: %v \n waiting 60 seconds before retrying \n", err)
			time.Sleep(60 * time.Second)
		} else {
			break
		}
	}
	e.mu.Lock()
	e.registeredSubnets <- subnet
	e.mu.Unlock()

	prefixPool := subnet.Prefix.Subnet

	logrus.WithFields(logrus.Fields{
		"prefixPool":         prefixPool,
		"nsConfig.IPAddress": nsConfig.IPAddress,
	}).Infof("Creating vL3 IPAM endpoint")
	ipamEp := endpoint.NewIpamEndpoint(&common.NSConfiguration{
		IPAddress: prefixPool,
	})

	var nsRemoteIpList []string
	nsRemoteIpListStr, ok := os.LookupEnv("NSM_REMOTE_NS_IP_LIST")
	if ok {
		nsRemoteIpList = strings.Split(nsRemoteIpListStr, ",")
	}
	compositeEndpoints := []networkservice.NetworkServiceServer{
		ipamEp,
		newVL3ConnectComposite(nsConfig, prefixPool,
			&vppagent.UniversalCNFVPPAgentBackend{}, nsRemoteIpList, func() string {
				return ucnfEndpoint.NseName
			}),
	}

	return &compositeEndpoints
}

// exported the symbol named "CompositeEndpointPlugin"

func main() {
	// Capture signals to cleanup before exiting
	c := tools.NewOSSignalChannel()

	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.TraceLevel)

	mainFlags := &Flags{}
	mainFlags.Process()

	ipamAddress, ok := os.LookupEnv("IPAM_ADDRESS")
	if !ok {
		ipamAddress = "cnns-ipam:50051"
	}
	conn, err := grpc.Dial(ipamAddress, grpc.WithInsecure())
	if err != nil {
		logrus.Fatal("unable to connect to ipam server", err)
	}

	ipamAllocator := ipprovider.NewAllocatorClient(conn)
	vl3 := vL3CompositeEndpoint{
		IpamAllocator:     ipamAllocator,
		registeredSubnets: make(chan *ipprovider.Subnet),
		mu:                &sync.RWMutex{},
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		logrus.Info("begin the renew process")
		if err := vl3.Renew(ctx, func(err error) {
			if err != nil {
				logrus.Error("unable to renew the subnet", err)
			}
		}); err != nil {
			logrus.Error(err)
		}
	}()

	ucnfNse := ucnf.NewUcnfNse(mainFlags.ConfigPath, mainFlags.Verify, &vppagent.UniversalCNFVPPAgentBackend{}, vl3)
	logrus.Info("endpoint started")

	select {
	case _ = <-c:
		logrus.Info("cleaning up")
		if err := vl3.Cleanup(ctx); err != nil {
			logrus.Error(err)
		}
		ucnfNse.Cleanup()
		logrus.Info("cleaning up complete")
		cancel()
		conn.Close()
	}

}

type errors []error

func (es errors) Error() string {
	buff := bytes.NewBufferString("multiple errors: \n")
	for _, e := range es {
		_, _ = fmt.Fprintf(buff, "\t%s\n", e)
	}
	return buff.String()
}

/*
var (
	nsmEndpoint *endpoint.NsmEndpoint
)

func main() {

	// Capture signals to cleanup before exiting
	c := tools.NewOSSignalChannel()

	composite := endpoint.NewCompositeEndpoint(
		endpoint.NewMonitorEndpoint(nil),
		endpoint.NewIpamEndpoint(nil),
		newVL3ConnectComposite(nil),
		endpoint.NewConnectionEndpoint(nil))

	nsme, err := endpoint.NewNSMEndpoint(context.TODO(), nil, composite)
	if err != nil {
		logrus.Fatalf("%v", err)
	}
	nsmEndpoint = nsme
	_ = nsmEndpoint.Start()
	logrus.Infof("Started NSE --got name %s", nsmEndpoint.GetName())
	defer func() { _ = nsmEndpoint.Delete() }()

	// Capture signals to cleanup before exiting
	<-c
}

func GetMyNseName() string {
	return nsmEndpoint.GetName()
}

*/

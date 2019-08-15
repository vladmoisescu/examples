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
	"flag"
	"github.com/networkservicemesh/examples/examples/universal-cnf/vppagent/pkg/ucnf"
	"github.com/networkservicemesh/examples/examples/universal-cnf/vppagent/pkg/vppagent"
	"github.com/networkservicemesh/networkservicemesh/pkg/tools"
	"github.com/networkservicemesh/networkservicemesh/sdk/common"
	"github.com/networkservicemesh/networkservicemesh/sdk/endpoint"
	"github.com/sirupsen/logrus"
	"os"
)
const (
	defaultConfigPath = "/etc/universal-cnf/config.yaml"
	defaultPluginModule = ""
)

// Flags holds the command line flags as supplied with the binary invocation
type Flags struct {
	ConfigPath string
	Verify     bool
}

// Process will parse the command line flags and init the structure members
func (mf *Flags) Process() {
	flag.StringVar(&mf.ConfigPath, "file", defaultConfigPath, " full path to the configuration file")
	flag.BoolVar(&mf.Verify, "verify", false, "only verify the configuration, don't run")
	flag.Parse()
}

type vL3CompositeEndpoint string

func (vL3ce vL3CompositeEndpoint) AddCompositeEndpoints(nsConfig *common.NSConfiguration) *[]endpoint.ChainedEndpoint {
	compositeEndpoints := []endpoint.ChainedEndpoint{
		newVL3ConnectComposite(nsConfig),
	}

	return &compositeEndpoints
}

// exported the symbol named "CompositeEndpointPlugin"
var  CompositeEndpointPlugin vL3CompositeEndpoint

func main() {
	// Capture signals to cleanup before exiting
	c := tools.NewOSSignalChannel()

	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.TraceLevel)

	mainFlags := &Flags{}
	mainFlags.Process()

	//var defCEAddon defaultCompositeEndpointAddon
	ucnfNse := ucnf.NewUcnfNse(mainFlags.ConfigPath, mainFlags.Verify, &vppagent.UniversalCNFVPPAgentBackend{}, CompositeEndpointPlugin)
	defer ucnfNse.Cleanup()
	<-c
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
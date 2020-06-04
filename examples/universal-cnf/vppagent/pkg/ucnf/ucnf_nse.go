package ucnf

import (
	"bytes"
	"context"
	"fmt"
	"github.com/danielvladco/k8s-vnet/pkg/nseconfig"
	"github.com/davecgh/go-spew/spew"
	"github.com/gofrs/uuid"
	"github.com/networkservicemesh/examples/api/ipam/ipprovider"
	"github.com/networkservicemesh/examples/examples/universal-cnf/vppagent/pkg/config"
	"github.com/networkservicemesh/networkservicemesh/sdk/common"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"
	"os"
	"time"
)

type ipamService struct {
	IpamAllocator     ipprovider.AllocatorClient
	registeredSubnets chan *ipprovider.Subnet
	ctx               *context.Context
}

func (i *ipamService) AllocateSubnet(ucnfEndpoint *nseconfig.Endpoint) (string, error) {
	var subnet *ipprovider.Subnet
	for j := 0; j < 6; j++ {
		var err error
		subnet, err = i.IpamAllocator.AllocateSubnet(context.Background(), &ipprovider.SubnetRequest{
			Identifier: &ipprovider.Identifier{
				Fqdn:               ucnfEndpoint.CNNS.Address,
				Name:               ucnfEndpoint.CNNS.Name + uuid.Must(uuid.NewV4()).String(),
				ConnectivityDomain: ucnfEndpoint.CNNS.ConnectivityDomain,
			},
			AddrFamily: &ipprovider.IpFamily{Family: ipprovider.IpFamily_IPV4},
			PrefixLen:  uint32(ucnfEndpoint.VL3.IPAM.PrefixLength),
		})
		if err != nil {
			if j == 5 {

			}
			logrus.Errorf("ipam allocation not successful: %v \n waiting 60 seconds before retrying \n", err)
			time.Sleep(60 * time.Second)
		} else {
			break
		}
	}
	i.registeredSubnets <- subnet
	return subnet.Prefix.Subnet, nil
}

func (i *ipamService) Renew(errorHandler func(err error)) error {
	g, ctx := errgroup.WithContext(*i.ctx)
	for subnet := range i.registeredSubnets {
		subnet := subnet
		g.Go(func() error {
			for range time.Tick(time.Duration(subnet.LeaseTimeout-1) * time.Hour) {
				_, err := i.IpamAllocator.RenewSubnetLease(ctx, subnet)
				if err != nil {
					errorHandler(err)
				}
			}
			return nil
		})
	}
	logrus.Info("Cleaning registered subnets")
	err := i.Cleanup()
	if err != nil {
		errorHandler(err)
	}
	return g.Wait()
}

func (i *ipamService) Cleanup() error {
	var errs errors
	for s := range i.registeredSubnets {
		_, err := i.IpamAllocator.FreeSubnet(*i.ctx, s)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

//
type UcnfNse struct {
	processEndpoints *config.ProcessEndpoints
}

func (ucnf *UcnfNse) Cleanup() {
	ucnf.processEndpoints.Cleanup()
}

//type IpamServiceFactory func(addr string) IpamService

func NewUcnfNse(configPath string, verify bool, backend config.UniversalCNFBackend, ceAddons config.CompositeEndpointAddons, ctx *context.Context) *UcnfNse {
	cnfConfig := &nseconfig.Config{}
	f, err := os.Open(configPath)
	if err != nil {
		logrus.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	err = nseconfig.NewConfig(yaml.NewDecoder(f), cnfConfig)
	if err != nil {
		logrus.Fatal(err)
	}

	if err := backend.NewUniversalCNFBackend(); err != nil {
		logrus.Fatal(err)
	}

	if verify {
		spew.Dump(cnfConfig)
		return nil
	}

	configuration := common.FromEnv()
	newIpamService := func(addr string) config.IpamService {
		conn, err := grpc.Dial(addr, grpc.WithInsecure())
		if err != nil {
			logrus.Fatal("unable to connect to ipam server: %v", err)
		}

		ipamAllocator := ipprovider.NewAllocatorClient(conn)
		ipamService := ipamService{
			IpamAllocator:     ipamAllocator,
			registeredSubnets: make(chan *ipprovider.Subnet),
			ctx:               ctx,
		}
		go func() {
			logrus.Info("begin the renew process")
			if err := ipamService.Renew(func(err error) {
				if err != nil {
					logrus.Error("unable to renew the subnet", err)
				}
			}); err != nil {
				logrus.Error(err)
			}
		}()
		return &ipamService
	}
	//add logic here

	pe := config.NewProcessEndpoints(backend, cnfConfig.Endpoints, configuration, ceAddons, newIpamService)

	ucnfnse := &UcnfNse{
		processEndpoints: pe,
	}

	logrus.Infof("Starting endpoints")
	// defer pe.Cleanup()

	if err := pe.Process(); err != nil {
		logrus.Fatalf("Error processing the new endpoints: %v", err)
	}
	return ucnfnse
}

type errors []error

func (es errors) Error() string {
	buff := bytes.NewBufferString("multiple errors: \n")
	for _, e := range es {
		_, _ = fmt.Fprintf(buff, "\t%s\n", e)
	}
	return buff.String()
}

package main

import (
	"context"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/local/connection"
	"github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/local/networkservice"
	"github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/registry"
	"github.com/networkservicemesh/networkservicemesh/pkg/tools"
	"github.com/networkservicemesh/networkservicemesh/sdk/common"
	"github.com/networkservicemesh/networkservicemesh/sdk/endpoint"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"os"
)

const (
	NSREGISTRY_ADDR  = "nsmmgr.nsm-system"
	NSREGISTRY_PORT = "5000"
)

type vL3ConnectComposite struct {
	endpoint.BaseCompositeEndpoint
	vl3NsePeers     map[string]string
	nsRegGrpcClient   *grpc.ClientConn
	nsDiscoveryClient registry.NetworkServiceDiscoveryClient
}

func (vxc *vL3ConnectComposite) Request(ctx context.Context,
	request *networkservice.NetworkServiceRequest) (*connection.Connection, error) {
	logrus.Infof("vL3ConnectComposite Request handler")
	if vxc.GetNext() == nil {
		logrus.Fatal("Should have Next set")
	}

	incoming, err := vxc.GetNext().Request(ctx, request)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}

	logrus.Infof("vL3ConnectComposite serviceRegistry.DiscoveryClient")
	if vxc.nsDiscoveryClient == nil {
		logrus.Error("nsDiscoveryClient is nil")
	} else {
		req := &registry.FindNetworkServiceRequest{
			NetworkServiceName: "vl3-service", // TODO: get pod's service name
		}
		logrus.Infof("vL3ConnectComposite FindNetworkService")
		response, err := vxc.nsDiscoveryClient.FindNetworkService(context.Background(), req)
		if err != nil {
			logrus.Error(err)
		} else {
			logrus.Infof("vL3ConnectComposite found network service; going through endpoints")
			for _, vl3endpoint := range response.NetworkServiceEndpoints {
				if vl3endpoint.EndpointName != GetMyNseName() {
					logrus.Infof("Found vL3 service %s peer %s", vl3endpoint.NetworkServiceName,
						vl3endpoint.EndpointName)
					vxc.vl3NsePeers[vl3endpoint.EndpointName] = vl3endpoint.NetworkServiceManagerName
				} else {
					logrus.Infof("Found my vL3 service %s instance endpoint name: %s", vl3endpoint.NetworkServiceName,
						vl3endpoint.EndpointName)
				}
			}
		}
	}
	logrus.Infof("vL3ConnectComposite request done")
	return incoming, nil
}

func (vxc *vL3ConnectComposite) Close(ctx context.Context, conn *connection.Connection) (*empty.Empty, error) {
	// remove from connections

	if vxc.GetNext() != nil {
		vxc.GetNext().Close(ctx, conn)
	}

	return &empty.Empty{}, nil
}

// Name returns the composite name
func (vxc *vL3ConnectComposite) Name() string {
	return "vL3 NSE"
}

// NewVppAgentComposite creates a new VPP Agent composite
func newVL3ConnectComposite(configuration *common.NSConfiguration) *vL3ConnectComposite {
	nsRegAddr, ok := os.LookupEnv("NSREGISTRY_ADDR")
	if !ok {
		nsRegAddr = NSREGISTRY_ADDR
	}
	nsRegPort, ok := os.LookupEnv("NSREGISTRY_PORT")
	if !ok {
		nsRegPort = NSREGISTRY_PORT
	}

	// ensure the env variables are processed
	if configuration == nil {
		configuration = &common.NSConfiguration{}
	}
	configuration.CompleteNSConfiguration()

	logrus.Infof("newVL3ConnectComposite")

	var nsDiscoveryClient registry.NetworkServiceDiscoveryClient

	/*
	regAddr := net.ParseIP(nsRegAddr)
	if regAddr == nil {
		regAddrList, err := net.LookupHost(nsRegAddr)
		if err != nil {
			logrus.Errorf("nsmConnection registry address resolution Error: %v", err)
		} else {
			logrus.Infof("newVL3ConnectComposite: resolved %s to %v", nsRegAddr, regAddrList)
			for _, regAddrVal := range regAddrList {
				if regAddr = net.ParseIP(regAddrVal); regAddr != nil {
					logrus.Infof("newVL3ConnectComposite: NSregistry using IP %s", regAddrVal)
					break
				}
			}
		}
	}
	regPort, _ := strconv.Atoi(nsRegPort)
	nsRegGrpcClient, err := tools.SocketOperationCheck(&net.TCPAddr{IP: regAddr, Port: regPort})
	*/
	nsRegGrpcClient, err := tools.DialTCP(nsRegAddr + ":" + nsRegPort)
	if err != nil {
		logrus.Errorf("nsmConnection GRPC Client Socket Error: %v", err)
		//return nil
	} else {
		logrus.Infof("newVL3ConnectComposite socket operation ok... create networkDiscoveryClient")
		nsDiscoveryClient = registry.NewNetworkServiceDiscoveryClient(nsRegGrpcClient)
		logrus.Infof("newVL3ConnectComposite networkDiscoveryClient ok")
	}
	newVL3ConnectComposite := &vL3ConnectComposite{
		vl3NsePeers: make(map[string]string),
		nsRegGrpcClient: nsRegGrpcClient,
		nsDiscoveryClient: nsDiscoveryClient,
	}

	logrus.Infof("newVL3ConnectComposite returning")

	return newVL3ConnectComposite
}
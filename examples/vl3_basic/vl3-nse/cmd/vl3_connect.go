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
	"net"
	"os"
	"strconv"
)

const (
	NSREGISTRY_ADDR  = "nsmgr.nsm-system"
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
				logrus.Infof("Found vL3 service %s peer %s", vl3endpoint.NetworkServiceName,
					vl3endpoint.EndpointName)
				vxc.vl3NsePeers[vl3endpoint.EndpointName] = vl3endpoint.NetworkServiceManagerName
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

	regPort, _ := strconv.Atoi(nsRegPort)
	nsRegGrpcClient, err := tools.SocketOperationCheck(&net.TCPAddr{IP: net.ParseIP(nsRegAddr), Port: regPort})
	if err != nil {
		logrus.Errorf("nsmConnection Error: %v", err)
		return nil
	}
	logrus.Infof("newVL3ConnectComposite socket operation ok... create networkDiscoveryClient")
	nsDiscoveryClient := registry.NewNetworkServiceDiscoveryClient(nsRegGrpcClient)
	logrus.Infof("newVL3ConnectComposite networkDiscoveryClient ok")

	newVL3ConnectComposite := &vL3ConnectComposite{
		vl3NsePeers: make(map[string]string),
		nsRegGrpcClient: nsRegGrpcClient,
		nsDiscoveryClient: nsDiscoveryClient,
	}

	return newVL3ConnectComposite
}
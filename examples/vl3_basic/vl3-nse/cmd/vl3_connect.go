package main

import (
	"context"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/connectioncontext"
	"github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/local/connection"
	"github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/local/networkservice"
	remote_connection "github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/remote/connection"
	remote_networkservice "github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/remote/networkservice"
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

type vL3PeerState int

const (
	PEER_STATE_NOTCONN vL3PeerState = iota
	PEER_STATE_CONN
	PEER_STATE_CONNERR
	PEER_STATE_CONN_INPROG
)

type vL3NsePeer struct {
	endpointName string
	networkServiceManagerName string
	state vL3PeerState
}

type vL3ConnectComposite struct {
	endpoint.BaseCompositeEndpoint
	vl3NsePeers     map[string]vL3NsePeer
	nsRegGrpcClient   *grpc.ClientConn
	nsDiscoveryClient registry.NetworkServiceDiscoveryClient
}

func (vxc *vL3ConnectComposite) addPeer(endpointName, networkServiceManagerName string) vL3NsePeer {
	peer, ok := vxc.vl3NsePeers[endpointName]
	if !ok {
		vxc.vl3NsePeers[endpointName] = vL3NsePeer{
			endpointName: endpointName,
			networkServiceManagerName: networkServiceManagerName,
			state: PEER_STATE_NOTCONN,
		}
	}
	return vxc.vl3NsePeers[endpointName]
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
					vxc.ConnectPeerEndpoint(vxc.addPeer(vl3endpoint.EndpointName, vl3endpoint.NetworkServiceManagerName))
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

func (vxc *vL3ConnectComposite) createPeerConnectionRequest(peer vL3NsePeer) {
	mechanismType := common.MechanismFromString("KERNEL")
	outgoingMechanism, err := remote_connection.Mechanism{
		Type:                 0,
		Parameters:           nil,
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     nil,
		XXX_sizecache:        0,
	}
		"Peer connection")
	if err != nil {
		logrus.Errorf("Failure to prepare the outgoing mechanism preference with error: %+v", err)
		return nil, err
	}

	routes := []*connectioncontext.Route{}
	/*for _, r := range nsmc.Configuration.Routes {
		routes = append(routes, &connectioncontext.Route{
			Prefix: r,
		})
	}*/

	outgoingRequest := &remote_networkservice.NetworkServiceRequest{
		Connection: &remote_connection.Connection{
			NetworkService: nsmc.Configuration.OutgoingNscName,
			Context: &connectioncontext.ConnectionContext{
				IpContext: &connectioncontext.IPContext{
					SrcIpRequired: true,
					DstIpRequired: true,
					SrcRoutes:     routes,
				},
			},
			Labels: nsmc.OutgoingNscLabels,
		},
		MechanismPreferences: []*remote_connection.Mechanism{
			outgoingMechanism,
		},
}

func (vxc *vL3ConnectComposite) ConnectPeerEndpoint(peer vL3NsePeer) {
	// build connection object
	// perform remote networkservice request
	logrus.WithFields(logrus.Fields{
		"endpointName": peer.endpointName,
		"networkServiceManagerName": peer.networkServiceManagerName,
	}).Info("newVL3Connect ConnectPeerEndpoint")

	switch peer.state {
	case PEER_STATE_NOTCONN:
		// TODO do connection request
		logrus.WithFields(logrus.Fields{
			"endpointName": peer.endpointName,
			"networkServiceManagerName": peer.networkServiceManagerName,
		}).Info("request remote connection")
	case PEER_STATE_CONN:
		logrus.WithFields(logrus.Fields{
			"endpointName": peer.endpointName,
			"networkServiceManagerName": peer.networkServiceManagerName,
		}).Info("remote connection already established")
	case PEER_STATE_CONNERR:
		logrus.WithFields(logrus.Fields{
			"endpointName": peer.endpointName,
			"networkServiceManagerName": peer.networkServiceManagerName,
		}).Info("remote connection attempted prior and errored")
	case PEER_STATE_CONN_INPROG:
		logrus.WithFields(logrus.Fields{
			"endpointName": peer.endpointName,
			"networkServiceManagerName": peer.networkServiceManagerName,
		}).Info("remote connection in progress")
	}
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

	// TODO create remote_networkservice API connection

	newVL3ConnectComposite := &vL3ConnectComposite{
		vl3NsePeers: make(map[string]string),
		nsRegGrpcClient: nsRegGrpcClient,
		nsDiscoveryClient: nsDiscoveryClient,
	}

	logrus.Infof("newVL3ConnectComposite returning")

	return newVL3ConnectComposite
}
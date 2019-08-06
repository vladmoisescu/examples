package main

import (
	"context"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/connectioncontext"
	"github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/local/connection"
	"github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/local/networkservice"
	"strings"

	//remote_connection "github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/remote/connection"
	//remote_networkservice "github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/remote/networkservice"
	"github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/registry"
	"github.com/networkservicemesh/networkservicemesh/pkg/tools"
	"github.com/networkservicemesh/networkservicemesh/sdk/common"
	"github.com/networkservicemesh/networkservicemesh/sdk/endpoint"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"os"
	"time"
)

const (
	NSREGISTRY_ADDR  = "nsmmgr.nsm-system"
	NSREGISTRY_PORT = "5000"
	LABEL_NSESOURCE = "vl3Nse/nseSource/endpointName"
)

type vL3PeerState int

const (
	PEER_STATE_NOTCONN vL3PeerState = iota
	PEER_STATE_CONN
	PEER_STATE_CONNERR
	PEER_STATE_CONN_INPROG
	PEER_STATE_CONN_RX
)

type vL3NsePeer struct {
	endpointName string
	networkServiceManagerName string
	state vL3PeerState
	connHdl *connection.Connection
	connErr error
}

type vL3ConnectComposite struct {
	endpoint.BaseCompositeEndpoint
	vl3NsePeers     map[string]*vL3NsePeer
	nsRegGrpcClient   *grpc.ClientConn
	nsDiscoveryClient registry.NetworkServiceDiscoveryClient
}

func (vxc *vL3ConnectComposite) setPeerState(endpointName string, state vL3PeerState) {
	vxc.vl3NsePeers[endpointName].state = state
}

func (vxc *vL3ConnectComposite) setPeerConnHdl(endpointName string, connHdl *connection.Connection) {
	vxc.vl3NsePeers[endpointName].connHdl = connHdl
}

func (vxc *vL3ConnectComposite) addPeer(endpointName, networkServiceManagerName string) *vL3NsePeer {
	_, ok := vxc.vl3NsePeers[endpointName]
	if !ok {
		vxc.vl3NsePeers[endpointName] = &vL3NsePeer{
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

	if vl3SrcEndpointName, ok := request.GetConnection().GetLabels()[LABEL_NSESOURCE]; ok {
		// request is from another vl3 NSE
		logrus.Infof("vL3ConnectComposite received connection request from vL3 NSE %s", vl3SrcEndpointName)
		peer := vxc.addPeer(vl3SrcEndpointName, request.GetConnection().GetSourceNetworkServiceManagerName())
		logrus.WithFields(logrus.Fields{
			"endpointName": peer.endpointName,
			"networkServiceManagerName": peer.networkServiceManagerName,
			"prior_state": peer.state,
			"new_state": PEER_STATE_CONN_RX,
		}).Infof("vL3ConnectComposite vl3 NSE peer %s added", vl3SrcEndpointName)
		vxc.setPeerState(vl3SrcEndpointName, PEER_STATE_CONN_RX)
	} else {

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
				for _, vl3endpoint := range response.GetNetworkServiceEndpoints() {
					if vl3endpoint.GetEndpointName() != GetMyNseName() {
						logrus.Infof("Found vL3 service %s peer %s", vl3endpoint.NetworkServiceName,
							vl3endpoint.GetEndpointName())
						go vxc.ConnectPeerEndpoint(vxc.addPeer(vl3endpoint.GetEndpointName(), vl3endpoint.NetworkServiceManagerName))
					} else {
						logrus.Infof("Found my vL3 service %s instance endpoint name: %s", vl3endpoint.NetworkServiceName,
							vl3endpoint.GetEndpointName())
					}
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

func (vxc *vL3ConnectComposite) createPeerConnectionRequest(peer *vL3NsePeer) {
	/* This func impl is based on sdk/client/client.go::NsmClient.Connect() */
	vxc.setPeerState(peer.endpointName, PEER_STATE_CONN_INPROG)
	mechanismType := common.MechanismFromString("KERNEL")
	description := "Peer vL3 NSE connection"
	outgoingMechanism, err := connection.NewMechanism(mechanismType, strings.Replace(GetMyNseName(), "vl3-service", "", -1), description)
	if err != nil {
		logrus.Errorf("Failure to prepare the outgoing mechanism preference with error: %+v", err)
		vxc.setPeerState(peer.endpointName, PEER_STATE_CONNERR)
		peer.connErr = err
		return
	}

	routes := []*connectioncontext.Route{}
	for _, r := range nsmEndpoint.Configuration.Routes {
		routes = append(routes, &connectioncontext.Route{
			Prefix: r,
		})
	}

	labels := tools.ParseKVStringToMap(nsmEndpoint.Configuration.OutgoingNscLabels,",", "=")
	labels[LABEL_NSESOURCE] = GetMyNseName()
	outgoingRequest := &networkservice.NetworkServiceRequest{
		Connection: &connection.Connection{
			NetworkService: nsmEndpoint.Configuration.AdvertiseNseName,
			Context: &connectioncontext.ConnectionContext{
				IpContext: &connectioncontext.IPContext{
					SrcIpRequired: true,
					DstIpRequired: true,
					SrcRoutes:     routes,
				},
			},
			Labels: labels,
			//SourceNetworkServiceManagerName: nsmEndpoint.NsmConnection.
			DestinationNetworkServiceManagerName: peer.networkServiceManagerName,
			NetworkServiceEndpointName: peer.endpointName,
		},
		MechanismPreferences: []*connection.Mechanism{
			outgoingMechanism,
		},
	}
	peer.connHdl, err = vxc.performPeerConnectRequest(outgoingRequest)
	if err != nil {
		vxc.setPeerState(peer.endpointName, PEER_STATE_CONNERR)
		peer.connErr = err
		logrus.Errorf("Peer connect request failure with error: %+v", err)
		return
	}
	vxc.setPeerState(peer.endpointName, PEER_STATE_CONN)
}

func (vxc *vL3ConnectComposite) performPeerConnectRequest(outgoingRequest *networkservice.NetworkServiceRequest) (*connection.Connection, error) {
	var outgoingConnection *connection.Connection
	connectRetries := 5
	connectSleep := 5 * time.Second
	connectTimeout := 10 * time.Second
	start := time.Now()
	for iteration := connectRetries; iteration > 0; <-time.After(connectSleep) {
		var err error
		logrus.Infof("vL3 Sending outgoing request %v", outgoingRequest)

		ctx, cancel := context.WithTimeout(nsmEndpoint.Context, connectTimeout)
		defer cancel()
		outgoingConnection, err = nsmEndpoint.NsClient.Request(ctx, outgoingRequest)

		if err != nil {
			logrus.Errorf("vL3 failure to request connection with error: %+v", err)
			iteration--
			if iteration > 0 {
				continue
			}
			logrus.Errorf("vL3 Connect failed after %v iterations and %v", connectRetries, time.Since(start))
			return nil, err
		}

		//nsmEndpoint.OutgoingConnections = append(nsmEndpoint.OutgoingConnections, outgoingConnection)
		logrus.Infof("vL3 Received outgoing connection after %v: %v", time.Since(start), outgoingConnection)
		break
	}

	return outgoingConnection, nil
}

func (vxc *vL3ConnectComposite) ConnectPeerEndpoint(peer *vL3NsePeer) {
	// build connection object
	// perform remote networkservice request
	logrus.WithFields(logrus.Fields{
		"endpointName": peer.endpointName,
		"networkServiceManagerName": peer.networkServiceManagerName,
		"state": peer.state,
	}).Info("newVL3Connect ConnectPeerEndpoint")

	switch peer.state {
	case PEER_STATE_NOTCONN:
		// TODO do connection request
		logrus.WithFields(logrus.Fields{
			"endpointName": peer.endpointName,
			"networkServiceManagerName": peer.networkServiceManagerName,
		}).Info("request remote connection")
		vxc.createPeerConnectionRequest(peer)
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
	case PEER_STATE_CONN_RX:
		logrus.WithFields(logrus.Fields{
			"endpointName": peer.endpointName,
			"networkServiceManagerName": peer.networkServiceManagerName,
		}).Info("remote connection already established--rx from peer")
	default:
		logrus.WithFields(logrus.Fields{
			"endpointName": peer.endpointName,
			"networkServiceManagerName": peer.networkServiceManagerName,
		}).Info("remote connection state unknown")
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

	// TODO? create remote_networkservice API connection

	newVL3ConnectComposite := &vL3ConnectComposite{
		vl3NsePeers: make(map[string]*vL3NsePeer),
		nsRegGrpcClient: nsRegGrpcClient,
		nsDiscoveryClient: nsDiscoveryClient,
	}

	logrus.Infof("newVL3ConnectComposite returning")

	return newVL3ConnectComposite
}
package main

import (
	"context"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/networkservicemesh/examples/examples/universal-cnf/vppagent/pkg/config"
	"github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/connectioncontext"
	"github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/local/connection"
	"github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/local/networkservice"
	"github.com/networkservicemesh/networkservicemesh/sdk/client"
	//remote_connection "github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/remote/connection"
	//remote_networkservice "github.com/networkservicemesh/networkservicemesh/controlplane/pkg/apis/remote/networkservice"
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
	NSCLIENT_PORT = "5001"
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
	excludedPrefixes []string
}

type vL3ConnectComposite struct {
	endpoint.BaseCompositeEndpoint
	myEndpointName string
	nsConfig *common.NSConfiguration
	ipamCidr string
	vl3NsePeers     map[string]*vL3NsePeer
	nsRegGrpcClient   *grpc.ClientConn
	nsDiscoveryClient registry.NetworkServiceDiscoveryClient
	//nsClient networkservice.NetworkServiceClient
	nsmClient  *client.NsmClient
	ipamEndpoint *endpoint.IpamEndpoint
	backend config.UniversalCNFBackend
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
func (vxc *vL3ConnectComposite) SetMyNseName(request *networkservice.NetworkServiceRequest) {
	if vxc.myEndpointName == "" {
		logrus.Infof("Setting vL3connect composite endpoint name to %s", request.GetConnection().GetNetworkServiceEndpointName())
		vxc.myEndpointName = request.GetConnection().GetNetworkServiceEndpointName()
	}
}

func (vxc *vL3ConnectComposite) GetMyNseName() string {
	return vxc.myEndpointName
}

func (vxc *vL3ConnectComposite) Request(ctx context.Context,
	request *networkservice.NetworkServiceRequest) (*connection.Connection, error) {
	logrus.WithFields(logrus.Fields{
		"endpointName": request.GetConnection().GetNetworkServiceEndpointName(),
		"networkServiceManagerName": request.GetConnection().GetSourceNetworkServiceManagerName(),
	}).Infof("vL3ConnectComposite Request handler")
	if vxc.GetNext() == nil {
		logrus.Fatal("Should have Next set")
	}

	/* NOTE: for IPAM we assume there's no IPAM endpoint in the composite endpoint list */
	/* -we are taking care of that here in this handler */
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
		peer.excludedPrefixes = removeDuplicates(append(peer.excludedPrefixes, incoming.Context.IpContext.ExcludedPrefixes...))
		incoming.Context.IpContext.ExcludedPrefixes = peer.excludedPrefixes
		peer.connHdl = request.GetConnection()

		/* tell my peer to route to me for my ipamCIDR */
		mySubnetRoute := connectioncontext.Route{
			Prefix:               vxc.ipamCidr,
		}
		incoming.Context.IpContext.DstRoutes = append(incoming.Context.IpContext.DstRoutes, &mySubnetRoute)
		vxc.setPeerState(vl3SrcEndpointName, PEER_STATE_CONN_RX)
	} else {
		vxc.SetMyNseName(request)
		logrus.Infof("vL3ConnectComposite serviceRegistry.DiscoveryClient")
		if vxc.nsDiscoveryClient == nil {
			logrus.Error("nsDiscoveryClient is nil")
		} else {
			/* set NSC route to this NSE for full vL3 CIDR */
			nscVL3Route := connectioncontext.Route{
				Prefix:               vxc.nsConfig.IPAddress,
			}
			incoming.Context.IpContext.DstRoutes = append(incoming.Context.IpContext.DstRoutes, &nscVL3Route)

			/* Find all NSEs registered as the same type as this one */
			req := &registry.FindNetworkServiceRequest{
				NetworkServiceName: request.GetConnection().GetNetworkService(),
			}
			logrus.Infof("vL3ConnectComposite FindNetworkService for NS=%s", request.GetConnection().GetNetworkService())
			response, err := vxc.nsDiscoveryClient.FindNetworkService(context.Background(), req)
			if err != nil {
				logrus.Error(err)
			} else {
				logrus.Infof("vL3ConnectComposite found network service; going through endpoints")
				err = vxc.processNsEndpoints(ctx, response)
				if err != nil {
					logrus.Errorf("Failed process endpoints - %v", err)
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

func (vxc *vL3ConnectComposite) processNsEndpoints(ctx context.Context, response *registry.FindNetworkServiceResponse) error {
	/* TODO: For NSs with multiple endpoint types how do we know their type?
	   - do we need to match the name portion?  labels?
	*/
	for _, vl3endpoint := range response.GetNetworkServiceEndpoints() {
		if vl3endpoint.GetEndpointName() != vxc.GetMyNseName() {
			logrus.Infof("Found vL3 service %s peer %s", vl3endpoint.NetworkServiceName,
				vl3endpoint.GetEndpointName())
			peer := vxc.addPeer(vl3endpoint.GetEndpointName(), vl3endpoint.NetworkServiceManagerName)
			//peer.excludedPrefixes = removeDuplicates(append(peer.excludedPrefixes, incoming.Context.IpContext.ExcludedPrefixes...))
			err := vxc.ConnectPeerEndpoint(ctx, peer)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"peerEndpoint": vl3endpoint.GetEndpointName(),
				}).Errorf("Failed to connect to vL3 Peer")
			} else {
				if peer.connHdl != nil {
					logrus.WithFields(logrus.Fields{
						"peerEndpoint":         vl3endpoint.GetEndpointName(),
						"srcIP":                peer.connHdl.Context.IpContext.SrcIpAddr,
						"ConnExcludedPrefixes": peer.connHdl.Context.IpContext.ExcludedPrefixes,
						"peerExcludedPrefixes": peer.excludedPrefixes,
						"peer.DstRoutes": peer.connHdl.Context.IpContext.DstRoutes,
					}).Infof("Connected to vL3 Peer")
				} else {
					logrus.WithFields(logrus.Fields{
						"peerEndpoint":         vl3endpoint.GetEndpointName(),
						"peerExcludedPrefixes": peer.excludedPrefixes,
					}).Infof("Connected to vL3 Peer but connhdl == nil")
				}
			}
		} else {
			logrus.Infof("Found my vL3 service %s instance endpoint name: %s", vl3endpoint.NetworkServiceName,
				vl3endpoint.GetEndpointName())
		}
	}
	return nil
}

func (vxc *vL3ConnectComposite) createPeerConnectionRequest(ctx context.Context, peer *vL3NsePeer, routes []string) error {
	/* This func impl is based on sdk/client/client.go::NsmClient.Connect() */
	vxc.setPeerState(peer.endpointName, PEER_STATE_CONN_INPROG)

	var dpconfig interface{}
	peer.connHdl, peer.connErr = vxc.performPeerConnectRequest(ctx, peer, routes, dpconfig)
	if peer.connErr != nil {
		logrus.WithFields(logrus.Fields{
			"peer.Endpoint": peer.endpointName,
		}).Errorf("NSE peer connection failed - %v", peer.connErr)
		vxc.setPeerState(peer.endpointName, PEER_STATE_CONNERR)
		return peer.connErr
	}

	if peer.connErr = vxc.backend.ProcessDPConfig(dpconfig); peer.connErr != nil {
		logrus.Errorf("endpoint %s Error processing dpconfig: %+v", peer.endpointName, dpconfig)
		vxc.setPeerState(peer.endpointName, PEER_STATE_CONNERR)
		return peer.connErr
	}

	vxc.setPeerState(peer.endpointName, PEER_STATE_CONN)
	return nil
}

func (vxc *vL3ConnectComposite) performPeerConnectRequest(ctx context.Context, peer *vL3NsePeer, routes []string, dpconfig interface{}) (*connection.Connection, error) {
    ifName := peer.endpointName
	conn, err := vxc.nsmClient.ConnectToEndpoint(peer.endpointName, peer.networkServiceManagerName, ifName, "mem", "VPP interface "+ifName, routes)
	if err != nil {
		logrus.Errorf("Error creating %s: %v", ifName, err)
		return nil, err
	}

	err = vxc.backend.ProcessClient(dpconfig, ifName, conn)

	return conn, nil
}

func (vxc *vL3ConnectComposite) ConnectPeerEndpoint(ctx context.Context, peer *vL3NsePeer) error {
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
		routes := []string{vxc.ipamCidr}
		return vxc.createPeerConnectionRequest(ctx, peer, routes)
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
	return nil
}

func removeDuplicates(elements []string) []string {
	encountered := map[string]bool{}
	result := []string{}

	for v := range elements {
		if !encountered[elements[v]] {
			encountered[elements[v]] = true
			result = append(result, elements[v])
		}
	}
	return result
}

// NewVppAgentComposite creates a new VPP Agent composite
func newVL3ConnectComposite(configuration *common.NSConfiguration, ipamCidr string, backend config.UniversalCNFBackend) *vL3ConnectComposite {
	nsRegAddr, ok := os.LookupEnv("NSREGISTRY_ADDR")
	if !ok {
		nsRegAddr = NSREGISTRY_ADDR
	}
	nsRegPort, ok := os.LookupEnv("NSREGISTRY_PORT")
	if !ok {
		nsRegPort = NSREGISTRY_PORT
	}

	/*nsPort, ok := os.LookupEnv("NSCLIENT_PORT")
	if !ok {
		nsPort = NSCLIENT_PORT
	}*/

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
		logrus.Errorf("nsmRegistryConnection GRPC Client Socket Error: %v", err)
		//return nil
	} else {
		logrus.Infof("newVL3ConnectComposite socket operation ok... create networkDiscoveryClient")
		nsDiscoveryClient = registry.NewNetworkServiceDiscoveryClient(nsRegGrpcClient)
		logrus.Infof("newVL3ConnectComposite networkDiscoveryClient ok")
	}

	// TODO? create remote_networkservice API connection

	//var nsClient networkservice.NetworkServiceClient
	/*
	nsGrpcClient, err := tools.DialTCP(nsRegAddr + ":" + nsPort)
	if err != nil {
		logrus.Errorf("nsmConnection GRPC Client Socket Error: %v", err)
		//return nil
	} else {
		logrus.Infof("newVL3ConnectComposite socket operation ok... create network-service client")
		nsClient = networkservice.NewNetworkServiceClient(nsGrpcClient)
		logrus.Infof("newVL3ConnectComposite network-service client ok")
	}
	*/
	// Call the NS Client initiation
	nsConfig := &common.NSConfiguration{
		OutgoingNscName:   configuration.AdvertiseNseName,
		OutgoingNscLabels: "",
		Routes:            configuration.Routes,
	}
	var nsmClient *client.NsmClient
	nsmClient, err = client.NewNSMClient(context.TODO(), nsConfig)
	if err != nil {
		logrus.Errorf("Unable to create the NSM client %v", err)
	}
	/*
	nsmConn, err := common.NewNSMConnection(context.TODO(), configuration)
	if err != nil {
		logrus.Errorf("nsmConnection Client Connection Error: %v", err)
	} else {
		nsClient = nsmConn.NsClient
	}
	 */

	newVL3ConnectComposite := &vL3ConnectComposite{
		nsConfig: configuration,
		ipamCidr: ipamCidr,
		myEndpointName: "",
		vl3NsePeers: make(map[string]*vL3NsePeer),
		nsRegGrpcClient: nsRegGrpcClient,
		nsDiscoveryClient: nsDiscoveryClient,
		nsmClient: nsmClient,
		backend: backend,
	}

	logrus.Infof("newVL3ConnectComposite returning")

	return newVL3ConnectComposite
}
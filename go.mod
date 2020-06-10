module github.com/tiswanso/examples

go 1.12

require (
	git.fd.io/govpp.git v0.2.1-0.20200131102335-2df59463fcbb // indirect
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Microsoft/go-winio v0.4.7 // indirect
	github.com/Nordix/simple-ipam v1.0.0
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/Shopify/sarama v1.20.1 // indirect
	github.com/alicebob/miniredis v2.5.0+incompatible // indirect
	github.com/codahale/hdrhistogram v0.0.0-20161010025455-3a0bb77429bd // indirect
	github.com/containerd/continuity v0.0.0-20171215195539-b2b946a77f59 // indirect
	github.com/coreos/bbolt v1.3.3 // indirect
	github.com/coreos/go-iptables v0.4.0 // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/cli v0.0.0-20190822175708-578ab52ece34 // indirect
	github.com/docker/docker v0.0.0-20180620002508-3dfb26ab3cbf // indirect
	github.com/docker/go-connections v0.3.0 // indirect
	github.com/docker/go-units v0.3.2 // indirect
	github.com/docker/libnetwork v0.0.0-20180726175142-9ffeaf7d8b64 // indirect
	github.com/fsnotify/fsnotify v1.4.7
	github.com/fsouza/go-dockerclient v1.2.2 // indirect
	github.com/golang/protobuf v1.3.2
	github.com/google/go-cmp v0.4.0 // indirect
	github.com/gorilla/websocket v1.4.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.11.3 // indirect
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645
	github.com/hashicorp/go-uuid v1.0.1 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jhump/protoreflect v1.5.0 // indirect
	github.com/ligato/cn-infra v2.2.0+incompatible // indirect
	github.com/ligato/vpp-agent v2.5.1+incompatible
	github.com/mitchellh/go-ps v0.0.0-20170309133038-4fdf99ab2936 // indirect
	github.com/networkservicemesh/networkservicemesh/controlplane/api v0.3.0
	github.com/networkservicemesh/networkservicemesh/forwarder/api v0.0.0-00010101000000-000000000000 // indirect
	github.com/networkservicemesh/networkservicemesh/pkg v0.3.0
	github.com/networkservicemesh/networkservicemesh/utils v0.3.0 // indirect
	github.com/onsi/gomega v1.10.0 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v1.0.0-rc5 // indirect
	github.com/opentracing/opentracing-go v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/pkg/profile v1.3.0 // indirect
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.3 // indirect
	github.com/spf13/viper v1.5.0
	github.com/teris-io/shortid v0.0.0-20171029131806-771a37caa5cf // indirect
	github.com/uber/jaeger-lib v2.2.0+incompatible // indirect
	github.com/vishvananda/netlink v0.0.0-20180910184128-56b1bd27a9a3 // indirect
	github.com/yuin/gopher-lua v0.0.0-20190514113301-1cd887cd7036 // indirect
	go.ligato.io/cn-infra/v2 v2.5.0-alpha.0.20200313154441-b0d4c1b11c73 // indirect
	go.uber.org/multierr v1.2.0 // indirect
	golang.org/x/net v0.0.0-20200114155413-6afb5195e5aa
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20200114203027-fcfc50b29cbb // indirect
	google.golang.org/grpc v1.27.1
	gopkg.in/yaml.v2 v2.2.4
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c // indirect
	gotest.tools v2.2.0+incompatible // indirect
)

replace (
	github.com/census-instrumentation/opencensus-proto v0.1.0-0.20181214143942-ba49f56771b8 => github.com/census-instrumentation/opencensus-proto v0.0.3-0.20181214143942-ba49f56771b8
	github.com/networkservicemesh/networkservicemesh/controlplane => github.com/tiswanso/networkservicemesh/controlplane v0.2.0-vl3
	github.com/networkservicemesh/networkservicemesh/controlplane/api => github.com/tiswanso/networkservicemesh/controlplane/api v0.2.0-vl3
	github.com/networkservicemesh/networkservicemesh/forwarder/api => github.com/tiswanso/networkservicemesh/forwarder/api v0.2.0-vl3
	github.com/networkservicemesh/networkservicemesh/pkg => github.com/tiswanso/networkservicemesh/pkg v0.2.0-vl3
	github.com/networkservicemesh/networkservicemesh/sdk => github.com/tiswanso/networkservicemesh/sdk v0.2.0-vl3
	github.com/networkservicemesh/networkservicemesh/utils => github.com/tiswanso/networkservicemesh/utils v0.2.0-vl3
)

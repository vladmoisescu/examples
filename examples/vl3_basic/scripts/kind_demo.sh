#!/usr/bin/env bash

K8S_STARTPORT=${K8S_STARTPORT:-38790}
JAEGER_STARTPORT=${JAEGER_STARTPORT:-38900}

function fatal {
  echo "$@" >&2
  exit 1
}

function kind_create_cluster {
    local name=$1; shift
    local kconf=$1; shift
    if [[ $# > 1 ]]; then
        local hostip=$1; shift
        local portoffset=$1; shift
    fi

    if [[ -n ${hostip} ]]; then
        HOSTIP=${hostip}
        K8S_HOSTPORT=$((${K8S_STARTPORT} + $portoffset))
        JAEGER_HOSTPORT=$((${JAEGER_STARTPORT} + $portoffset))
        cat <<EOF > ${KINDCFGDIR}/${name}.yaml
kind: Cluster
apiVersion: kind.sigs.k8s.io/v1alpha3
kubeadmConfigPatchesJson6902:
- group: kubeadm.k8s.io
  version: v1beta2
  kind: ClusterConfiguration
  patch: |
    - op: add
      path: /apiServer/certSANs/-
      value: "${HOSTIP}"
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 6443
    hostPort: ${K8S_HOSTPORT}
    listenAddress: ${HOSTIP}
  - containerPort:  31922
    hostPort: ${JAEGER_HOSTPORT}
    listenAddress: ${HOSTIP}
EOF
        kind create cluster --name ${name} --config ${KINDCFGDIR}/${name}.yaml
    else
        kind create cluster --name ${name}
    fi
    kind get kubeconfig --name=${name} > ${kconf}
}

if [[ ${1} == "--delete" ]]; then
  kind delete cluster --name="kind-1"
  kind delete cluster --name="kind-2"
  exit 0
fi

[[ -z "$KUBECONFDIR" ]] && fatal "KUBECONFDIR env var is not set"
[[ -z "$NSM_DIR" ]] && fatal "NSM_DIR env var is not set"
[[ -z "$NSE_HUB" ]] && fatal "NSE_HUB env var is not set"
[[ -z "$NSE_TAG" ]] && fatal "NSE_TAG env var is not set"

SCRIPT_DIR=$(dirname "$0")
EXAMPLES_DIR=$(realpath "$SCRIPT_DIR/../")
NSM_DIR=$(realpath "$NSM_DIR")

pushd "$SCRIPT_DIR" || fatal "directory does not exist mos likely because script was executed from PATH"

mkdir -p "$KUBECONFDIR"

kind_create_cluster kind-1 "$KUBECONFDIR/kind-1.kubeconfig"
kind_create_cluster kind-2 "$KUBECONFDIR/kind-2.kubeconfig"

EXAMPLES_DIR=${EXAMPLES_DIR} KUBECONFDIR=${KUBECONFDIR} ./vl3_nse.sh --nse-hub="${NSE_HUB}" --nse-tag="${NSE_TAG}" --nsm-dir="$NSM_DIR"
KUBECONFDIR=${KUBECONFDIR} ./test_interdomain_conn.sh

popd || fatal "can't popd"
#!/usr/bin/env bash

NSM_HUB="${NSM_HUB:-"tiswanso"}"
NSM_TAG="${NSM_TAG:-"vl3_api_rebase"}"
NSE_HUB="${NSE_HUB:-"tiswanso"}"
NSE_TAG="${NSE_TAG:-"kind_ci"}"
NSC_DELAY=false

function print_usage() {
  echo "
  --nsm-hub=STRING
  --nsm-tag=STRING
  --nse-hub=STRING
  --nse-tag=STRING
  --nsm-dir=STRING
  --no-nsc-delay
  --help -h
  "
}
#!/usr/bin/env bash

# fatal is a helper to log an error and exit current session
function fatal {
  echo "$@" >&2
  exit 1
}

# kubewait is a shortcut for waiting on pods until they are ready
function kubeready {
  local labels=$1; shift
  echo "# **** Waiting on $labels"
  kubectl wait --timeout=150s --for condition=Ready -l "app in ($labels)" "$@" pod
}

for i in "$@"; do
  case $i in
  --nsm-hub=*)
    NSM_HUB="${i#*=}"
    ;;
  --nsm-tag=*)
    NSM_TAG="${i#*=}"
    ;;
  --nse-hub=*)
    NSE_HUB="${i#*=}"
    ;;
  --nse-tag=*)
    NSE_TAG="${i#*=}"
    ;;
  --nsm-dir=*)
    NSMDIR=$(realpath "${i#*=}")
    ;;
  --no-nsc-delay)
    NSC_DELAY=false
    ;;
  -h | --help)
    print_usage
    exit 0
    ;;
  *)
    print_usage
    exit 1
    ;;
  esac
done

[[ -z "$EXAMPLES_DIR" ]] && fatal "EXAMPLES_DIR env var should be set"
[[ -z "$KUBECONFDIR" ]] && fatal "KUBECONFDIR env var should be set"
[[ -z "$NSMDIR" ]] && fatal "NSMDIR env var should be set"

pushd "$EXAMPLES_DIR" || fatal "'$EXAMPLES_DIR' directory does not exist"

mapfile -t kubeconfs < <(ls -d "${KUBECONFDIR}"/*)

[[ ${#kubeconfs[@]} == 0 ]] && fatal "No kubeconfigs to install NSM+vL3 into: KUBECONFDIR=${KUBECONFDIR}"

for kconf in ${kubeconfs[@]}; do
  echo "Cluster = ${kconf}"
  kubectl get nodes --kubeconfig "${kconf}"

  NSMDIR=$NSMDIR KCONF=$kconf scripts/nsm_install_interdomain.sh --nsm-hub="${NSM_HUB}" --nsm-tag="${NSM_TAG}"
done

echo "# **** Wait for NSM pods to be ready in each cluster"
for kconf in ${kubeconfs[@]}; do
  kubeready nsm-admission-webhook,nsmgr-daemonset,proxy-nsmgr-daemonset,nsm-vpp-plane -n nsm-system --kubeconfig "$kconf"
done

declare -A KconfToRemoteIP
for kconf in ${kubeconfs[@]}; do
  KconfToRemoteIP[${kconf}]=$(kubectl get node --kubeconfig "${kconf}" --selector='node-role.kubernetes.io/master' -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
done

VL3_IPAMOCTET=22
for kconf in ${kubeconfs[@]}; do
  REMOTE_IPS=
  for kconf2 in "${!KconfToRemoteIP[@]}"; do
    if [[ ${kconf} != "$kconf2" ]]; then
      if [[ -n ${REMOTE_IPS} ]]; then
        REMOTE_IPS="${REMOTE_IPS},${KconfToRemoteIP[${kconf2}]}"
      else
        REMOTE_IPS=${KconfToRemoteIP[${kconf2}]}
      fi
    fi
  done
  if [[ -z ${REMOTE_IPS} ]]; then
    # TODO: set REMOTE_IP param to garbage since the vL3 helm chart always sets up the deployment
    # to look for the remote-list config-map
    # (vL3 will just move on when it can't connect to the remote)
    REMOTE_IPS="no-remote.hack"
  fi
  echo "Install vL3 in cluster ${kconf}"

  REMOTE_IP=$REMOTE_IPS KCONF=$kconf scripts/vl3_interdomain.sh \
    --ipamOctet="$VL3_IPAMOCTET" \
    --nse-hub="$NSE_HUB" \
    --nse-tag="$NSE_TAG" || fatal "vl3_interdomain.sh failed! see error above"

  let "VL3_IPAMOCTET++"
done

for kconf in ${kubeconfs[@]}; do
  if [[ "${NSC_DELAY}" == "true" ]]; then
    echo "Delaying 1min before installing NSCs in ${kconf}"
    sleep 60
  fi

  echo "# **** Install helloworld on cluster ${kconf}"
  kubectl apply --kubeconfig "${kconf}" -f k8s/vl3-hello.yaml
done

for kconf in ${kubeconfs[@]}; do
  echo "# **** wait for helloworld on cluster ${kconf}"
  kubeready helloworld --kubeconfig "$kconf"
done

popd

pushd "$(dirname "$0")" || fatal "directory does not exist mos likely because script was executed from PATH"

#!/bin/bash
print_usage() {
  echo "$(basename "$0")
Usage: $(basename "$0") [options...]
Options:
  --nse-hub=STRING          Hub for vL3 NSE images
  --nse-tag=STRING          Tag for vL3 NSE images
  --service=STRING          Tag for vL3 NSE images
" >&2
}

NSE_HUB=${NSE_HUB:-"mistrate"}
NSE_TAG=${NSE_TAG:-"kind_ipam"}
PULLPOLICY=${PULLPOLICY:-IfNotPresent}
INSTALL_OP=${INSTALL_OP:-apply}
SERVICENAME=${SERVICENAME:-vl3-nse}

for i in "$@"; do
    case $i in
        --nse-hub=?*)
	    NSE_HUB=${i#*=}
	    ;;
        --nse-tag=?*)
            NSE_TAG=${i#*=}
	    ;;
        -h|--help)
            usage
            exit
            ;;
        --ipamPool=?*)
            IPAMPOOL=${i#*=}
            ;;
        --cnnsNsrCD?*)
            CNNS_NSRCD=${i#*=}
            ;;
        --service=?*)
            SERVICENAME=${i#*=}
            ;;
        --delete)
            INSTALL_OP=delete
            ;;
        *)
            print_usage
            exit 1
            ;;
    esac
done

CNNS_NSRNAME=${SERVICENAME}
CNNS_NSRADDR="${CNNS_NSRNAME}.cnns-cisco.com"
CNNS_NSRCD="${CNNS_NSRNAME}-connectivity-domain"

sdir=$(dirname ${0})

if [[ -n ${CNNS_NSRADDR} ]]; then
    REMOTE_IP=${CNNS_NSRADDR}
fi

NSMDIR=${NSMDIR:-${sdir}/../../../../networkservicemesh}

VL3HELMDIR=${VL3HELMDIR:-${sdir}/../helm}

MFSTDIR=${MFSTDIR:-${sdir}/../k8s}

CFGMAP="configmap nsm-vl3-${SERVICENAME}"
if [[ "${INSTALL_OP}" == "delete" ]]; then
    echo "delete configmap"
    kubectl delete ${KCONF:+--kubeconfig $KCONF} "${CFGMAP}"
fi

echo "---------------Install NSE-------------"
echo "${CNNS_NSRADDR}"
helm template "${VL3HELMDIR}"/vl3 --set org="${NSE_HUB}" --set tag="${NSE_TAG}" ${SERVICENAME:+ --set image=${SERVICENAME}} --set pullPolicy="${PULLPOLICY}" ${REMOTE_IP:+ --set remote.ipList=${REMOTE_IP}} ${IPAMPOOL:+ --set ipam.defaultPrefixPool=${IPAMPOOL}} ${CNNS_NSRNAME:+ --set cnns.nsr.name=${CNNS_NSRNAME}} --set cnns.nsr.addr="${CNNS_NSRADDR}" --set cnns.nsr.cd="${CNNS_NSRCD}" ${CNNS_NSRPORT:+ --set cnns.nsr.port=${CNNS_NSRPORT}} --set nsm.serviceName="${SERVICENAME}" | kubectl ${INSTALL_OP} ${KCONF:+--kubeconfig $KCONF} -f -

if [[ "$INSTALL_OP" != "delete" ]]; then
  sleep 20
  kubectl wait ${KCONF:+--kubeconfig $KCONF} --timeout=150s --for condition=Ready -l networkservicemesh.io/app=vl3-nse-"${SERVICENAME}" pod
fi

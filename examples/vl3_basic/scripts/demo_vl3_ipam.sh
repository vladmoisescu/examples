#!/bin/bash

usage() {
  echo "usage: $0 [OPTIONS]"
  echo ""
  echo "  MANDATORY OPTIONS:"
  echo ""
  echo "  --kconf_clus1=<kubeconfig>           set the kubeconfig for the first cluster"
  echo "  --kconf_clus2=<kubeconfig>           set the kubeconfig for the second cluster"
  echo "  --service                            set nse docker image"
  echo ""
  echo "  Optional OPTIONS:"
  echo ""
  echo "  --namespace=<namespace>    set the namespace to watch for NSM clients"
  echo "  --delete                   delete the installation"
  echo "  --nowait                   don't wait for user input prior to moving to next step"
  echo ""
}

sdir=$(dirname ${0})
MFSTDIR=${MFSTDIR:-${sdir}/../k8s}

for i in "$@"; do
    case $i in
        -h|--help)
            usage
            exit
            ;;
        --kconf_clus1=?*)
            KCONF_CLUS1=${i#*=}
            echo "setting cluster 1=${KCONF_CLUS1}" 
            ;;
        --kconf_clus2=?*)
            KCONF_CLUS2=${i#*=}
            echo "setting cluster 2=${KCONF_CLUS2}" 
            ;;
        --service=?*)
            SERVICENAME=${i#*=}
            ;;
        --delete)
            DELETE=true
            ;;
        --nowait)
            NOWAIT=true
            ;;
        *)
            usage
            exit 1
            ;;
    esac
done

if [[ -z ${KCONF_CLUS1} || -z ${KCONF_CLUS2} ]]; then
    echo "ERROR: One or more of kubeconfigs not set."
    usage
    exit 1
fi

clus1_IP=$(kubectl get node --kubeconfig ${KCONF_CLUS1} -o jsonpath='{.items[0].status.addresses[?(@.type=="ExternalIP")].address}')
clus2_IP=$(kubectl get node --kubeconfig ${KCONF_CLUS2} -o jsonpath='{.items[0].status.addresses[?(@.type=="ExternalIP")].address}')
if [[ "${clus1_IP}" == "" ]]; then
    clus1_IP=$(kubectl get node --kubeconfig ${KCONF_CLUS1} --selector='node-role.kubernetes.io/master' -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
fi
if [[ "${clus2_IP}" == "" ]]; then
    clus2_IP=$(kubectl get node --kubeconfig ${KCONF_CLUS2} --selector='node-role.kubernetes.io/master' -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
fi

########################
# include the magic
########################
DEMOMAGIC=${DEMOMAGIC:-${sdir}/demo-magic.sh}
. "${DEMOMAGIC}" -d ${NOWAIT:+-n}

# hide the evidence
clear

function pc {
    pe "$@"
    #pe "clear"
    echo "----DONE---- $@"
    if [[ -z ${NOWAIT} ]]; then
        wait
    fi
}

p "# --------------------- Virtual L3 Setup ------------------------"

pe "# **** Loading NSE images"
pc "${DELETE:+INSTALL_OP=delete} ./load_nse.sh --kconf_clus1=kind-1 --kconf_clus2=kind-2 --org=mistrate --tag=kind_ipam --image=${SERVICENAME}"
echo
pe "# **** Creating Connectivity domain"
#TODO:add kubeconfig param
pc "$GOPATH/src/github.com/cnns-system/connectivity_domain/setup_connectivity_domain_cr.sh --kubeconfig=/home/mistrate/kubeconfigs/central/kind-3.kubeconfig ${DELETE+--delete} ${SERVICENAME:+--name=${SERVICENAME}}"
echo "Waiting for CD to load"
if [[ "$DELETE" != "true" ]]; then
  sleep 60
fi
pe "# **** Install vL3 in cluster 1"
pc "${DELETE:+INSTALL_OP=delete} REMOTE_IP=${clus2_IP} KCONF=${KCONF_CLUS1} PULLPOLICY=IfNotPresent ./vl3_interdomain_ipam.sh ${SERVICENAME:+--service=${SERVICENAME}}"
pc "kubectl get pods --kubeconfig ${KCONF_CLUS1} -o wide"
echo
pe "# **** Install vL3  in cluster 2"
pc "${DELETE:+INSTALL_OP=delete} REMOTE_IP=${clus1_IP} KCONF=${KCONF_CLUS2} PULLPOLICY=IfNotPresent ./vl3_interdomain_ipam.sh ${SERVICENAME:+--service=${SERVICENAME}}"
pc "kubectl get pods --kubeconfig ${KCONF_CLUS2} -o wide"

p "# **** Cluster 1 vL3 NSEs"
pe "kubectl get pods --kubeconfig ${KCONF_CLUS1} -l networkservicemesh.io/app=vl3-nse-${SERVICENAME} -o wide"
echo
p "# **** Cluster 2 vL3 NSEs"
pc "kubectl get pods --kubeconfig ${KCONF_CLUS2} -l networkservicemesh.io/app=vl3-nse-${SERVICENAME}  -o wide"
echo

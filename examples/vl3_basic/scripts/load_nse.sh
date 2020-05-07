#!/bin/bash

print_usage() {
  echo "$(basename "$0")
Usage: $(basename "$0") [options...]
Options:
  --kconf_clus1=<kubeconfig>    set the kubeconfig for the first cluster
  --kconf_clus2=<kubeconfig>    set the kubeconfig for the second cluster
  --org=STRING                  Docker organisation vL3 NSE images
                                (default=\"tiswanso\", environment variable: NSE_HUB)
  --tag=STRING                  Tag for vL3 NSE images
                                (default=\"kind_ipam\", environment variable: NSE_TAG)
  --image=STRING                Docker image name
                                (default=\"kind_ci\", environment variable: NSE_TAG)
" >&2
}

ORG="tiswanso"
TAG="kind_ipam"
IMAGE="vl3_ucnf-vl3-nse"

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
        --org=?*)
            ORG=${i#*=}
            echo "setting docker org=${ORG}"
            ;;
        --tag=?*)
            TAG=${i#*=}
            echo "setting docker tag=${TAG}"
            ;;
        --image=?*)
            IMAGE=${i#*=}
            echo "setting docker image=${IMAGE}"
            ;;
        --delete)
            INSTALL_OP=delete
            ;;
        *)
            usage
            exit 1
            ;;
    esac
done

if [[ "$INSTALL_OP" != "delete" ]]; then
  #TODO:remove istratem
  echo "---Building Docker Image---"
  cd "$GOPATH"/src/github.com/istratem/examples || return
  export "ORG=${ORG}"
  export "TAG=${TAG}"
  export "IMAGE=${IMAGE}"
  make docker-vl3-ipam
  echo "---DONE---"

  echo "---Loading NSE Docker Image---"
  kind load docker-image "${ORG}"/"${IMAGE}":"${TAG}" --name="${KCONF_CLUS1}"
  kind load docker-image "${ORG}"/"${IMAGE}":"${TAG}" --name="${KCONF_CLUS2}"
  echo "---DONE---"
fi
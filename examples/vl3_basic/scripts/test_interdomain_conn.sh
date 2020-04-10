#!/usr/bin/env bash

[[ -z "$KUBECONFDIR" ]] && (echo "KUBECONFDIR env var should be set" && exit 1)

mapfile -t kubeconfs < <(ls -d "${KUBECONFDIR}"/*)

[[ ${#kubeconfs[@]} == 0 ]] && (echo "No kubeconfigs to install NSM+vL3 into: KUBECONFDIR=${KUBECONFDIR}" && exit 1)

if [[ ${#kubeconfs[@]} > 1 ]]; then
    K1_PODNM=$(kubectl get pods --kubeconfig ${kubeconfs[0]} -l "app=helloworld" -o jsonpath="{.items[0].metadata.name}")
    K2_PODNM=$(kubectl get pods --kubeconfig ${kubeconfs[1]} -l "app=helloworld" -o jsonpath="{.items[0].metadata.name}")

    K1_PODIP=$(kubectl exec -t $K1_PODNM -c helloworld --kubeconfig ${kubeconfs[0]} -- ip a show dev nsm0 | awk '$1 == "inet" {gsub(/\/.*$/, "", $2); print $2}')

    K2_PODIP=$(kubectl exec -t $K2_PODNM -c helloworld --kubeconfig ${kubeconfs[1]} -- ip a show dev nsm0 | awk '$1 == "inet" {gsub(/\/.*$/, "", $2); print $2}')

else
    echo "Single cluster mode so checking curl from pod to pod in same cluster through NSM intf"
    # assumes 2+ helloworld pods in deployment replica count
    K1_PODNM=$(kubectl get pods --kubeconfig ${kubeconfs[0]} -l "app=helloworld" -o jsonpath="{.items[0].metadata.name}")
    K2_PODNM=$(kubectl get pods --kubeconfig ${kubeconfs[0]} -l "app=helloworld" -o jsonpath="{.items[1].metadata.name}")

    K1_PODIP=$(kubectl exec -t $K1_PODNM -c helloworld --kubeconfig ${kubeconfs[0]} -- ip a show dev nsm0 | awk '$1 == "inet" {gsub(/\/.*$/, "", $2); print $2}')

    K2_PODIP=$(kubectl exec -t $K2_PODNM -c helloworld --kubeconfig ${kubeconfs[0]} -- ip a show dev nsm0 | awk '$1 == "inet" {gsub(/\/.*$/, "", $2); print $2}')
fi

echo "# **** Cluster 1 pod ${K1_PODNM} nsm0 IP = ${K1_PODIP}"
echo "# **** Cluster 2 pod ${K2_PODNM} nsm0 IP = ${K2_PODIP}"

echo "# **** Check helloworld response from remote's nsm0 interface -- curl http://${K2_PODIP}:5000/hello"
cmdout=$(kubectl exec -t $K1_PODNM -c helloworld --kubeconfig ${kubeconfs[0]} curl http://${K2_PODIP}:5000/hello)
echo $cmdout

# cmd return 0 for success, 1 failure
echo $cmdout | grep ${K2_PODNM}
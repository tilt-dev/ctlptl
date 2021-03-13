#!/bin/bash
# Tests creating a cluster with a registry,
# building a container in that cluster,
# then running that container.

set -exo pipefail

cd $(dirname $(realpath $0))
ctlptl apply -f registry.yaml
ctlptl apply -f cluster.yaml

# The ko-builder runs in an image tagged with the host as visible from the local machine.
docker build -t localhost:5005/ko-builder .
docker push localhost:5005/ko-builder

# The ko-builder builds an image tagged with the host as visible from the cluster network.
HOST=$(ctlptl get cluster kind-ctlptl-test-cluster -o template --template '{{.status.localRegistryHosting.hostFromClusterNetwork}}')
cat builder.yaml | sed "s/REGISTRY_HOST_PLACEHOLDER/$HOST/" | kubectl apply -f -
kubectl wait --for=condition=complete job/ko-builder --timeout=180s
cat simple-server.yaml | sed "s/REGISTRY_HOST_PLACEHOLDER/$HOST/" | kubectl apply -f -
kubectl wait --for=condition=available deployment/simple-server --timeout=60s

# Check to see we started the right kubernetes version.
k8sVersion=$(ctlptl get cluster kind-ctlptl-test-cluster -o go-template --template='{{.status.kubernetesVersion}}')

ctlptl delete -f cluster.yaml

if [[ "$k8sVersion" != "v1.18.15" ]]; then
    echo "Expected kubernetes version v1.18.15 but got $k8sVersion"
    exit 1
fi

echo "kind-cluster-network test passed!"

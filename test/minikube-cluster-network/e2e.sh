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
HOST=$(ctlptl get cluster minikube-ctlptl-test-cluster -o template --template '{{.status.localRegistryHosting.hostFromClusterNetwork}}')
cat builder.yaml | sed "s/REGISTRY_HOST_PLACEHOLDER/$HOST/g" | kubectl apply -f -
kubectl wait --for=condition=complete job/ko-builder --timeout=180s
cat simple-server.yaml | sed "s/REGISTRY_HOST_PLACEHOLDER/$HOST/g" | kubectl apply -f -
kubectl wait --for=condition=available deployment/simple-server --timeout=60s
ctlptl delete -f cluster.yaml

echo "minikube-cluster-network test passed!"

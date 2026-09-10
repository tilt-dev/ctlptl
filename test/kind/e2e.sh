#!/bin/bash
# Tests creating a cluster with a registry,
# building a container in that cluster,
# then running that container.

set -exo pipefail

export DOCKER_BUILDKIT="1"

cd $(dirname $(realpath $0))
CLUSTER_NAME="kind-ctlptl-test-cluster"
REGISTRY_NAME="ctlptl-test-registry"
ctlptl apply -f registry.yaml
ctlptl apply -f cluster.yaml

# Idempotence check
source ../idempotence.sh
REGISTRY_ID=$(registry_id "$REGISTRY_NAME")
CLUSTER_ID=$(cluster_id "$CLUSTER_NAME")
ctlptl apply -f registry.yaml
ctlptl apply -f cluster.yaml
assert_unchanged "registry $REGISTRY_NAME" "$REGISTRY_ID" "$(registry_id "$REGISTRY_NAME")"
assert_unchanged "cluster $CLUSTER_NAME" "$CLUSTER_ID" "$(cluster_id "$CLUSTER_NAME")"

# The ko-builder runs in an image tagged with the host as visible from the local machine.
docker buildx build --load -t localhost:5005/ko-builder .
docker push localhost:5005/ko-builder

# The ko-builder builds an image tagged with the host as visible from the cluster network.
HOST_FROM_CONTAINER_RUNTIME=$(ctlptl get cluster "$CLUSTER_NAME" -o template --template '{{.status.localRegistryHosting.host}}')
HOST_FROM_CLUSTER_NETWORK=$(ctlptl get cluster "$CLUSTER_NAME" -o template --template '{{.status.localRegistryHosting.hostFromClusterNetwork}}')
cat builder.yaml | \
    sed "s/HOST_FROM_CONTAINER_RUNTIME/$HOST_FROM_CONTAINER_RUNTIME/g" | \
    sed "s/HOST_FROM_CLUSTER_NETWORK/$HOST_FROM_CLUSTER_NETWORK/g" | \
    kubectl apply -f -

set +e
kubectl wait --for=condition=complete job/ko-builder --timeout=180s
RESULT="$?"
set -e

if [[ "$RESULT" != "0" ]]; then
    echo "ko-builder never became healthy"
    kubectl describe pods -l app=ko-builder
    kubectl logs -l app=ko-builder --all-containers
    exit 1
fi

# Test registry from both localhost and the connected network.
cat simple-server.yaml | \
    sed "s/HOST_FROM_CONTAINER_RUNTIME/$HOST_FROM_CONTAINER_RUNTIME/g" | \
    kubectl apply -f -
kubectl wait --for=condition=ready pods -l app=simple-server --timeout=60s
kubectl delete deployment simple-server

cat simple-server.yaml | \
    sed "s/HOST_FROM_CONTAINER_RUNTIME/$HOST_FROM_CLUSTER_NETWORK/g" | \
    kubectl apply -f -
kubectl wait --for=condition=ready pods -l app=simple-server --timeout=60s

# Check to see we started the right kubernetes version.
k8sVersion=$(ctlptl get cluster "$CLUSTER_NAME" -o go-template --template='{{.status.kubernetesVersion}}')

ctlptl delete -f cluster.yaml

if [[ "$k8sVersion" != "v1.34.11" ]]; then
    echo "Expected kubernetes version v1.34.11 but got $k8sVersion"
    exit 1
fi

echo "kind e2e test passed!"

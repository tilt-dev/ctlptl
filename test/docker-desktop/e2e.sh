#!/bin/bash
# Tests creating a cluster with a registry,
# building a container in that cluster,
# then running that container.

set -exo pipefail

export DOCKER_BUILDKIT="1"

cd $(dirname $(realpath $0))
CLUSTER_NAME="kind-ctlptl-test-cluster"
ctlptl apply -f cluster.yaml

# The ko-builder runs in an image tagged with the host as visible from the local machine.
docker buildx build --load -t ko-builder .
kubectl apply -f builder.yaml

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

kubectl apply -f simple-server.yaml
kubectl wait --for=condition=ready pods -l app=simple-server --timeout=180s

ctlptl delete -f cluster.yaml

echo "docker-desktop e2e test passed!"

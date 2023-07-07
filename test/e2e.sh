#!/bin/bash
# Integration tests that create a full cluster.

set -exo pipefail

cd $(dirname $(dirname $(realpath $0)))
make install
test/k3d/e2e.sh
test/kind/e2e.sh
test/minikube/e2e.sh

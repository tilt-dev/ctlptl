#!/bin/bash

set -exuo pipefail

REPO_ROOT=$(dirname $(dirname $(dirname "$0")))
cd "${REPO_ROOT}"

# Keep the generator in lockstep with the k8s libraries we depend on,
# so it never drifts far enough behind to stop building.
K8S_VERSION=$(go list -m -f '{{.Version}}' k8s.io/api)

rm -f pkg/api/*.deepcopy.go
rm -f pkg/api/*/*.deepcopy.go
go install "k8s.io/code-generator/cmd/deepcopy-gen@${K8S_VERSION}"
deepcopy-gen \
   --go-header-file hack/boilerplate.go.txt \
   ./pkg/api \
   ./pkg/api/k3dv1alpha4 \
   ./pkg/api/k3dv1alpha5

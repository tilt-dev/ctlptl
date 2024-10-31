#!/bin/bash

set -exuo pipefail

REPO_ROOT=$(dirname $(dirname $(dirname "$0")))
cd "${REPO_ROOT}"

GOROOT="$(go env GOROOT)"
rm -f pkg/api/*.deepcopy.go
rm -f pkg/api/*/*.deepcopy.go
go install k8s.io/code-generator/cmd/deepcopy-gen@v0.31.2
deepcopy-gen \
   --go-header-file hack/boilerplate.go.txt \
   ./pkg/api \
   ./pkg/api/k3dv1alpha4 \
   ./pkg/api/k3dv1alpha5

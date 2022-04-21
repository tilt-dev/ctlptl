#!/bin/bash

set -euo pipefail

REPO_ROOT=$(dirname $(dirname $(dirname "$0")))
cd "${REPO_ROOT}"

GOROOT="$(go env GOROOT)"
deepcopy-gen \
   -i "./pkg/api" \
   -O zz_generated.deepcopy \
   --go-header-file hack/boilerplate.go.txt

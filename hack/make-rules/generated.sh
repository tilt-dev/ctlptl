#!/bin/bash

REPO_ROOT=$(dirname $(dirname $(dirname "$0")))
cd "${REPO_ROOT}"

go run k8s.io/code-generator/cmd/deepcopy-gen \
   -i "./pkg/api" \
   -O zz_generated.deepcopy \
   --go-header-file hack/boilerplate.go.txt

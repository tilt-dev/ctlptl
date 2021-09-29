#!/bin/bash
#
# Do a complete release. Run on CI.

set -ex

if [[ "$GITHUB_TOKEN" == "" ]]; then
    echo "Missing GITHUB_TOKEN"
    exit 1
fi

if [[ "$DOCKER_TOKEN" == "" ]]; then
    echo "Missing DOCKER_TOKEN"
    exit 1
fi

DIR=$(dirname "$0")
cd "$DIR/.."

echo "$DOCKER_TOKEN" | docker login --username "$DOCKER_USERNAME" --password-stdin

git fetch --tags
goreleaser --rm-dist

VERSION=$(git describe --abbrev=0 --tags)

./hack/release-update-install.sh "$VERSION"

#!/bin/bash
#
# Updates the Tilt repo with the latest version info.
#
# Usage:
# scripts/update-tilt-repo.sh $VERSION
# where VERSION is of the form 0.1.0

set -euo pipefail

if [[ "${GITHUB_TOKEN-}" == "" ]]; then
    echo "Missing GITHUB_TOKEN"
    exit 1
fi

VERSION=${1//v/}
VERSION_PATTERN="^[0-9]+\\.[0-9]+\\.[0-9]+$"
if ! [[ $VERSION =~ $VERSION_PATTERN ]]; then
    echo "Version did not match expected pattern. Actual: $VERSION"
    exit 1
fi

DIR=$(dirname "$0")
cd "$DIR/.."

ROOT=$(mktemp -d)
git clone https://tilt-releaser:"$GITHUB_TOKEN"@github.com/tilt-dev/ctlptl "$ROOT"

set -x
cd "$ROOT"
sed -i -E "s/CTLPTL_VERSION=\".*\"/CTLPTL_VERSION=\"$VERSION\"/" INSTALL.md
sed -i -E "s/CTLPTL_VERSION = \".*\"/CTLPTL_VERSION = \"$VERSION\"/" INSTALL.md
git add .
git config --global user.email "hi@tilt.dev"
git config --global user.name "Tilt Dev"
git commit -a -m "Update version numbers: $VERSION"
git push origin main

rm -fR "$ROOT"

#!/bin/bash

set -euo pipefail

BUILDER=buildx-multiarch
IMAGE_NAME=docker/tilt-ctlptl-ci

docker buildx inspect $BUILDER || docker buildx create --name=$BUILDER --driver=docker-container --driver-opt=network=host
docker buildx build --builder=$BUILDER --pull --platform=linux/amd64,linux/arm64 --push -t "$IMAGE_NAME" -f .circleci/Dockerfile .

# add some bash code to pull the image and pull out the tag
docker pull "$IMAGE_NAME"
DIGEST="$(docker inspect --format '{{.RepoDigests}}' "$IMAGE_NAME" | tr -d '[]' | awk '{print $2}')"

yq eval -i ".jobs.e2e-remote-docker.docker[0].image = \"$DIGEST\"" .circleci/config.yml


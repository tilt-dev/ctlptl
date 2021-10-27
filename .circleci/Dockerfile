# Builds a Docker image with:
# - ctlptl
# - docker
# - kubectl
# - kind
# - socat
# - golang build toolchain
#
# Similar to the release image (which contains everything BUT the build
# toolchain)

FROM golang:1.17-buster

RUN apt update && apt install -y curl ca-certificates liblz4-tool rsync socat

# Install docker
# Adapted from https://github.com/circleci/circleci-images/blob/staging/shared/images/Dockerfile-basic.template
# Check https://download.docker.com/linux/static/stable/x86_64/ for latest versions
ENV DOCKER_VERSION=20.10.7
RUN set -exu \
  && DOCKER_URL="https://download.docker.com/linux/static/stable/x86_64/docker-${DOCKER_VERSION}.tgz" \
  && echo Docker URL: $DOCKER_URL \
  && curl --silent --show-error --location --fail --retry 3 --output /tmp/docker.tgz "${DOCKER_URL}" \
  && ls -lha /tmp/docker.tgz \
  && tar -xz -C /tmp -f /tmp/docker.tgz \
  && mv /tmp/docker/* /usr/bin \
  && rm -rf /tmp/docker /tmp/docker.tgz \
  && which docker \
  && (docker version || true)

# Install kubectl client
RUN apt install -y apt-transport-https gnupg \
  && curl -fsS https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - \
  && touch /etc/apt/sources.list.d/kubernetes.list \
  && echo "deb http://apt.kubernetes.io/ kubernetes-xenial main" | tee -a /etc/apt/sources.list.d/kubernetes.list \
  && apt update && apt install -y kubectl

# install Kind
ENV KIND_VERSION=v0.11.1
RUN set -exu \
  && curl -fLo ./kind-linux-amd64 "https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-linux-amd64" \
  && chmod +x ./kind-linux-amd64 \
  && mv ./kind-linux-amd64 /usr/local/bin/kind


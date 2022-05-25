# Builds a Docker image with:
# - ctlptl
# - docker
# - kubectl
# - kind
# - socat
#
# Good base image for anyone that wants to use ctlptl in a CI environment
# to set up a one-time-use cluster.
#
# Built with goreleaser.

FROM debian:bullseye

RUN apt update && apt install -y curl ca-certificates liblz4-tool rsync socat

# Install docker
# Adapted from https://github.com/circleci/circleci-images/blob/staging/shared/images/Dockerfile-basic.template
# Check https://download.docker.com/linux/static/stable/x86_64/ for latest versions
ENV DOCKER_VERSION=20.10.15
RUN set -exu \
  && DOCKER_URL="https://download.docker.com/linux/static/stable/$(uname -m)/docker-${DOCKER_VERSION}.tgz" \
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
ENV KIND_VERSION=v0.14.0
RUN set -exu \
  && case $(uname -m) in aarch64) arch=arm64 ;; x86_64) arch=amd64 ;; *) arch=$(uname -m) ;; esac \
  && curl -fLo ./kind-linux-$arch "https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-linux-$arch" \
  && chmod +x ./kind-linux-$arch \
  && mv ./kind-linux-$arch /usr/local/bin/kind

COPY ctlptl /usr/local/bin/ctlptl

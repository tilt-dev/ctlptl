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

FROM golang:1.21-bookworm

RUN apt update && apt install -y curl ca-certificates liblz4-tool rsync socat gpg

# Install docker
# Adapted from https://github.com/circleci/circleci-images/blob/staging/shared/images/Dockerfile-basic.template
# Check https://download.docker.com/linux/static/stable/x86_64/ for latest versions
ENV DOCKER_VERSION=24.0.5
RUN install -m 0755 -d /etc/apt/keyrings \
    && curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg \
    && chmod a+r /etc/apt/keyrings/docker.gpg \
    && echo "deb [arch="$(dpkg --print-architecture)" signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian \
       "$(. /etc/os-release && echo "$VERSION_CODENAME")" stable" | \
       tee /etc/apt/sources.list.d/docker.list > /dev/null

RUN apt update && apt install -y docker-ce-cli docker-buildx-plugin \
  && which docker \
  && (docker version || true)

# Install kubectl client
RUN apt install -y apt-transport-https gnupg \
  && curl -fsS https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - \
  && touch /etc/apt/sources.list.d/kubernetes.list \
  && echo "deb http://apt.kubernetes.io/ kubernetes-xenial main" | tee -a /etc/apt/sources.list.d/kubernetes.list \
  && apt update && apt install -y kubectl

# install Kind
ENV KIND_VERSION=v0.21.0
RUN set -exu \
  && curl -fLo ./kind-linux-amd64 "https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-linux-amd64" \
  && chmod +x ./kind-linux-amd64 \
  && mv ./kind-linux-amd64 /usr/local/bin/kind


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

FROM golang:1.22-bookworm

RUN apt update && apt install -y curl ca-certificates liblz4-tool rsync socat gpg

# Install docker CLI
RUN set -exu \
  # Add Docker's official GPG key:
  && install -m 0755 -d /etc/apt/keyrings \
  && curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc \
  && chmod a+r /etc/apt/keyrings/docker.asc \
  # Add the repository to Apt sources: 
  && echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian \
    $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  tee /etc/apt/sources.list.d/docker.list > /dev/null \
  && apt update \
  && apt install -y docker-ce-cli=5:25.0.3-1~debian.12~bookworm docker-buildx-plugin

# Install kubectl client
ENV KUBECTL_VERSION=v1.29.1
RUN curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
    && curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl.sha256" \
    && echo "$(cat kubectl.sha256)  kubectl" | sha256sum --check \
    && install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# install Kind
ENV KIND_VERSION=v0.23.0
RUN set -exu \
  && curl -fLo ./kind-linux-amd64 "https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-linux-amd64" \
  && chmod +x ./kind-linux-amd64 \
  && mv ./kind-linux-amd64 /usr/local/bin/kind


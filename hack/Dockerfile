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

FROM debian:bookworm-slim

RUN apt update && apt install -y curl ca-certificates liblz4-tool rsync socat

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
  && apt install -y docker-ce-cli=5:25.0.3-1~debian.12~bookworm 

# Install kubectl client
ARG TARGETARCH
ENV KUBECTL_VERSION=v1.29.1
RUN curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${TARGETARCH}/kubectl" \
    && curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${TARGETARCH}/kubectl.sha256" \
    && echo "$(cat kubectl.sha256)  kubectl" | sha256sum --check \
    && install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Install Kind
ENV KIND_VERSION=v0.24.0
RUN set -exu \
  && KIND_URL="https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-linux-$TARGETARCH" \
  && curl --silent --show-error --location --fail --retry 3 --output ./kind-linux-$TARGETARCH "$KIND_URL" \
  && chmod +x ./kind-linux-$TARGETARCH \
  && mv ./kind-linux-$TARGETARCH /usr/local/bin/kind

COPY ctlptl /usr/local/bin/ctlptl

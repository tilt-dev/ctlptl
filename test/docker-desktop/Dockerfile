# syntax=docker/dockerfile:1

FROM golang:1.24-alpine
RUN apk update && apk add bash git curl tar
ENV CGO_ENABLED=0
ENV KO_VERSION=0.14.1
RUN curl -fsSL https://github.com/ko-build/ko/releases/download/v${KO_VERSION}/ko_${KO_VERSION}_Linux_$(uname -m).tar.gz \
    | tar -xzv ko && \
    mv ko /usr/local/bin/ko
WORKDIR /go/github.com/tilt-dev/ctlptl/test/cluster-network
ADD . .

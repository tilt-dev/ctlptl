# syntax=docker/dockerfile:1

FROM golang:1.20-alpine
RUN apk update && apk add bash git
ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/google/ko@latest
WORKDIR /go/github.com/tilt-dev/ctlptl/test/cluster-network
ADD . .

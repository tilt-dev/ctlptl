FROM golang:1.18
RUN go install github.com/google/ko@latest
WORKDIR /go/github.com/tilt-dev/ctlptl/test/cluster-network
ADD . .

FROM golang:1.17
RUN go install github.com/google/ko@latest
WORKDIR /go/github.com/tilt-dev/ctlptl/test/cluster-network
ADD . .

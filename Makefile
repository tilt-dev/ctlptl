GOPATH = $(shell go env GOPATH)

.PHONY: generate test vendor

install:
	go install ./cmd/ctlptl

test:
	go test -timeout 30s -v ./...

generated:
	hack/make-rules/generated.sh

tidy:
	go mod tidy

e2e:
	test/e2e.sh

.PHONY: golangci-lint
golangci-lint: $(GOLANGCILINT)
	$(GOPATH)/bin/golangci-lint run --verbose

$(GOLANGCILINT):
	(cd /; GO111MODULE=on GOPROXY="direct" GOSUMDB=off go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.30.0)
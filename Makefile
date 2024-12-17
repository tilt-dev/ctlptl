GOPATH = $(shell go env GOPATH)

.PHONY: generate test vendor publish-ci-image

install:
	CGO_ENABLED=0 go install ./cmd/ctlptl

test:
	go test -timeout 30s -v ./...

generated:
	hack/make-rules/generated.sh

fmt:
	goimports -w -l -local github.com/tilt-dev/ctlptl cmd/ internal/ pkg/

tidy:
	go mod tidy

e2e:
	test/e2e.sh

.PHONY: golangci-lint
golangci-lint: $(GOLANGCILINT)
	$(GOPATH)/bin/golangci-lint run --verbose --timeout=120s

$(GOLANGCILINT):
	(cd /; GO111MODULE=on GOPROXY="direct" GOSUMDB=off go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0)

BUILDER=buildx-multiarch

publish-ci-image:
	./hack/publish-ci-image.sh

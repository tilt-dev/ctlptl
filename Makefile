GOPATH = $(shell go env GOPATH)

.PHONY: generate test vendor publish-ci-image

install:
	go install ./cmd/ctlptl

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
	$(GOPATH)/bin/golangci-lint run --verbose

$(GOLANGCILINT):
	(cd /; GO111MODULE=on GOPROXY="direct" GOSUMDB=off go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.30.0)

BUILDER=buildx-multiarch

publish-ci-image:
	docker buildx inspect $(BUILDER) || docker buildx create --name=$(BUILDER) --driver=docker-container --driver-opt=network=host
	docker buildx build --builder=$(BUILDER) --pull --platform=linux/amd64,linux/arm64 --push -t docker/tilt-ctlptl-ci -f .circleci/Dockerfile .

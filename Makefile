.PHONY: generate test vendor

install:
	go install ./cmd/ctlptl

test:
	go test -timeout 30s -v ./...

generated:
	hack/make-rules/generated.sh

tidy:
	go mod tidy

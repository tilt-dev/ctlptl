.PHONY: generate test vendor

test:
	go test -timeout 30s -v ./...

generated:
	hack/make-rules/generated.sh

tidy:
	go mod tidy

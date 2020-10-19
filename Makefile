.PHONY: generate test vendor

test:
	go test -v ./...

generated:
	hack/make-rules/generated.sh

tidy:
	go mod tidy

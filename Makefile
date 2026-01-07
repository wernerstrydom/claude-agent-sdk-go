.PHONY: test example-hello example-stream fmt vet lint security tidy imports

test:
	go test -v ./...

test-acceptance:
	go test -tags=acceptance -v ./test/...

example-hello:
	go run ./examples/hello

example-stream:
	go run ./examples/stream

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

security:
	gosec ./...

tidy:
	go mod tidy

imports:
	goimports -w .

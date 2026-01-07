.PHONY: test example-hello example-stream

test:
	go test -v ./...

example-hello:
	go run ./examples/hello

example-stream:
	go run ./examples/stream

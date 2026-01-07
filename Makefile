.PHONY: test example-hello

test:
	go test -v ./...

example-hello:
	go run ./examples/hello

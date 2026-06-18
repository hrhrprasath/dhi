.PHONY: build test lint fmt clean

build:
	go build -o build/dhi ./cmd/dhi

test:
	go test ./...

lint:
	golangci-lint run

fmt:
	go fmt ./...

clean:
	rm -rf build/

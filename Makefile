VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build build-release test lint fmt clean release

build:
	go build $(LDFLAGS) -o build/dhi ./cmd/dhi

build-release:
	goreleaser build --snapshot --clean

test:
	go test ./... -count=1

lint:
	golangci-lint run

fmt:
	go fmt ./...

clean:
	rm -rf build/ dist/

release:
	goreleaser release --clean

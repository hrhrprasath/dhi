VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build build-release test cover lint vet tidy fmt clean release

build:
	go build $(LDFLAGS) -o build/dhi ./cmd/dhi

build-release:
	goreleaser build --snapshot --clean

test:
	go test ./... -count=1

cover:
	go test ./... -count=1 -coverprofile=coverage.out

lint:
	golangci-lint run

vet:
	go vet ./...

tidy:
	go mod tidy

fmt:
	go fmt ./...

clean:
	rm -rf build/ dist/

release:
	goreleaser release --clean

BINARY  := nvtop
PKG     := ./cmd/nvtop
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

# go-nvml's bundled headers mark several unused symbols deprecated.
export CGO_CFLAGS ?= -Wno-deprecated-declarations

.PHONY: all build run demo test cover lint fmt clean install

all: lint test build

build:
	go build -trimpath -ldflags '$(LDFLAGS)' -o bin/$(BINARY) $(PKG)

install:
	go install -trimpath -ldflags '$(LDFLAGS)' $(PKG)

run: build
	./bin/$(BINARY)

demo: build
	./bin/$(BINARY) -mock 2

test:
	go test -race -count=1 ./...

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...

clean:
	rm -rf bin coverage.out dist

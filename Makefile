BINARY ?= k0rdent-mcp-server
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X github.com/k0rdent/mcp-k0rdent-server/internal/version.Version=$(VERSION) \
	-X github.com/k0rdent/mcp-k0rdent-server/internal/version.GitCommit=$(GIT_COMMIT) \
	-X github.com/k0rdent/mcp-k0rdent-server/internal/version.BuildDate=$(BUILD_DATE)

.PHONY: build test run

build:
	mkdir -p bin
	go build -o bin/$(BINARY) -ldflags "$(LDFLAGS)" ./cmd/server

test:
	go test ./...

run: build
	./bin/$(BINARY)

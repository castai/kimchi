SHELL := /usr/bin/env bash -e

ARCH := $(shell go env GOARCH)
OS := $(shell go env GOOS)
ROOT_DIR := $(abspath .)

GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := \
	-X github.com/castai/kimchi/internal/version.Version=$(GIT_VERSION) \
	-X github.com/castai/kimchi/internal/version.GitCommit=$(GIT_COMMIT) \
	-X github.com/castai/kimchi/internal/version.BuildDate=$(BUILD_DATE)

.PHONY: all build test clean

all: build

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/kimchi .

test:
	go test ./...

clean:
	rm -rf bin/

install: build
	cp bin/kimchi /usr/local/bin/ || mkdir -p ~/.local/bin && cp bin/kimchi ~/.local/bin/

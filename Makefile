# Makefile for gofs - A lightweight HTTP file server

BINARY_NAME=gofs
VERSION?=dev
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Build flags for size optimization
LDFLAGS=-ldflags "-s -w -X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildTime=${BUILD_TIME}"
BUILDFLAGS=-trimpath

.PHONY: build clean test dev help all

## Build optimized binary
build:
	@echo "Building ${BINARY_NAME}..."
	go build ${BUILDFLAGS} ${LDFLAGS} -o ${BINARY_NAME} ./cmd/gofs

## Run tests
test:
	@echo "Running tests..."
	go test -v ./...

## Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f ${BINARY_NAME}
	rm -rf bin/
	rm -f coverage.out coverage.html

## Start development server
dev: build
	@echo "Starting development server..."
	./${BINARY_NAME} -port 8000 -dir .

## Show help
help:
	@echo "gofs Makefile"
	@echo ""
	@echo "Available targets:"
	@grep -E '^##' Makefile | sed 's/## /  /'

# Default target
all: build
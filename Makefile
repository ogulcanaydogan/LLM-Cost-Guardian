.PHONY: all build build-lcg build-guardian test test-cover lint fmt fmt-check bench docker-build clean help

BINARY_DIR := bin
GO := go
GOFLAGS := -race
VERSION ?= dev
LDFLAGS := -s -w -X github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/cli.Version=$(VERSION)

all: build

## Build

build: build-lcg build-guardian

build-lcg:
	$(GO) build -ldflags="$(LDFLAGS)" -o $(BINARY_DIR)/lcg ./cmd/lcg

build-guardian:
	$(GO) build -ldflags="$(LDFLAGS)" -o $(BINARY_DIR)/guardian ./cmd/guardian

## Test

test:
	$(GO) test $(GOFLAGS) -coverprofile=coverage.out ./...

test-cover: test
	$(GO) tool cover -html=coverage.out -o coverage.html

## Lint

lint:
	golangci-lint run ./...

fmt:
	gofmt -w cmd/ internal/ pkg/

fmt-check:
	@test -z "$$(gofmt -l cmd/ internal/ pkg/)" || \
		(echo "Go files are not formatted. Run: make fmt" && exit 1)

## Benchmark

bench:
	$(GO) test -bench=. -benchmem -benchtime=3s ./pkg/tracker/... ./pkg/tokenizer/... ./pkg/providers/...

## Docker

docker-build:
	docker build -f deploy/docker/Dockerfile -t llm-cost-guardian:latest .

## Clean

clean:
	rm -rf $(BINARY_DIR) coverage.out coverage.html dist/

## Help

help:
	@echo "LLM Cost Guardian"
	@echo ""
	@echo "  make build        Build all binaries"
	@echo "  make build-lcg    Build CLI binary"
	@echo "  make build-guardian Build proxy service binary"
	@echo "  make test         Run tests with race detector"
	@echo "  make test-cover   Run tests with HTML coverage report"
	@echo "  make lint         Run golangci-lint"
	@echo "  make fmt          Format Go source files"
	@echo "  make bench        Run benchmarks"
	@echo "  make docker-build Build Docker image"
	@echo "  make clean        Clean build artifacts"

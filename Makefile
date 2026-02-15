# Makefile for wt - AI-friendly development commands

.PHONY: help build build-release test test-unit test-verbose test-integration test-all test-cover lint lint-fix clean fmt install deps run check precommit tools

# Variables
BINARY_NAME=wt
BUILD_DIR=bin
CMD_DIR=cmd/wt
GO=go
GOFLAGS=-v

# Default target
.DEFAULT_GOAL := help

## help: Display this help message
help:
	@echo "wt - Git Worktree + Tmux CLI Tool"
	@echo ""
	@echo "Development Commands:"
	@grep -E '^## ' Makefile | sed 's/## /  /' | sort

## deps: Download dependencies
deps:
	$(GO) mod download
	$(GO) mod tidy

## fmt: Format Go code
fmt:
	$(GO) fmt ./...
	@if command -v gofumpt >/dev/null 2>&1; then \
		gofumpt -w .; \
	fi

## lint: Run linters (requires golangci-lint)
lint:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi
	golangci-lint run --config .golangci.yml ./...

## lint-fix: Run linters with auto-fix
lint-fix:
	golangci-lint run --config .golangci.yml --fix ./...

## test: Run unit tests only (fast, no external processes)
test:
	$(GO) test -race ./...

## test-unit: Alias for test (explicit naming)
test-unit: test

## test-verbose: Run unit tests with verbose output
test-verbose:
	$(GO) test -race -v ./...

## test-integration: Run integration tests with real git/tmux
test-integration:
	WT_INTEGRATION_TEST=1 $(GO) test -race -v ./...

## test-all: Run both unit and integration tests
test-all:
	$(MAKE) test
	$(MAKE) test-integration

## test-cover: Run unit tests and show coverage in browser
test-cover:
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)

## build-release: Build release binaries for multiple platforms
build-release:
	@echo "Building release binaries..."
	@mkdir -p $(BUILD_DIR)
	@for osarch in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do \
		echo "Building for $$osarch..."; \
		GOOS=$${osarch%/*} GOARCH=$${osarch#*/} \
			$(GO) build $(GOFLAGS) -o "$(BUILD_DIR)/$(BINARY_NAME)_$${osarch%/*}_$${osarch#*/}" ./$(CMD_DIR) || exit 1; \
	done
	@echo "Release binaries built in $(BUILD_DIR)/"

## install: Install the binary to GOPATH/bin
install:
	$(GO) install ./$(CMD_DIR)

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out
	@$(GO) clean

## run: Build and run once
run: build
	./$(BUILD_DIR)/$(BINARY_NAME) --help

## check: Run all checks (fmt, lint, test)
check: fmt lint test

## precommit: Run pre-commit checks
precommit: fmt lint test

## tools: Install development tools
tools:
	@echo "Installing development tools..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin
	@$(GO) install mvdan.cc/gofumpt@latest
	@echo "Tools installed successfully!"

# Makefile for wt - AI-friendly development commands

.PHONY: help build test lint clean fmt install deps dev run

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

## test: Run tests
test:
	$(GO) test -race -coverprofile=coverage.out ./...

## test-verbose: Run tests with verbose output
test-verbose:
	$(GO) test -race -v ./...

## test-cover: Run tests and show coverage in browser
test-cover: test
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
	@gox -osarch="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64" -output "$(BUILD_DIR)/$(BINARY_NAME)_{{.OS}}_{{.Arch}}" ./$(CMD_DIR)

## install: Install the binary to GOPATH/bin
install:
	$(GO) install ./$(CMD_DIR)

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out
	@$(GO) clean

## dev: Run with hot-reload (requires air)
dev:
	@if ! command -v air >/dev/null 2>&1; then \
		echo "air not installed. Run: go install github.com/cosmtrek/air@latest"; \
		exit 1; \
	fi
	air

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
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@$(GO) install github.com/cosmtrek/air@latest
	@$(GO) install github.com/mitchellh/gox@latest
	@if command -v gofumpt >/dev/null 2>&1; then \
		$(GO) install mvdan.cc/gofumpt@latest; \
	fi
	@echo "Tools installed successfully!"

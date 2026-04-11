-include .env
# Makefile for Compozy

# -----------------------------------------------------------------------------
# Go Parameters & Setup
# -----------------------------------------------------------------------------
GOCMD=$(shell which go)
GOVERSION ?= $(shell awk '/^go /{print $$2}' go.mod 2>/dev/null || echo "1.26")
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOFMT=gofmt -s -w
BINARY_NAME=compozy
BINARY_DIR=bin
SRC_DIRS=./...
GOLANGCI_LINT_VERSION=v2.11.4
LINTCMD=golangci-lint

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
NC := \033[0m # No Color

# -----------------------------------------------------------------------------
# Build Variables
# -----------------------------------------------------------------------------
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION := $(shell git describe --tags --match="v*" --always 2>/dev/null || echo "unknown")

# Build flags for injecting version info (aligned with GoReleaser format)
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
MODULE_PATH := $(shell $(GOCMD) list -m 2>/dev/null)
ifeq ($(MODULE_PATH),)
MODULE_PATH := github.com/compozy/compozy
endif
LDFLAGS := -X $(MODULE_PATH)/internal/version.Version=$(VERSION) -X $(MODULE_PATH)/internal/version.Commit=$(GIT_COMMIT) -X $(MODULE_PATH)/internal/version.Date=$(BUILD_DATE)

.PHONY: all test lint fmt clean build install deps help verify tidy test-coverage test-nocache check-go-version setup link-skills build-extension-sdks publish-extension-sdks

# -----------------------------------------------------------------------------
# Setup & Version Checks
# -----------------------------------------------------------------------------
check-go-version:
	@echo "Checking Go version..."
	@GO_VERSION=$$($(GOCMD) version 2>/dev/null | awk '{print $$3}' | sed 's/go//'); \
	REQUIRED_VERSION=$(GOVERSION); \
	if [ -z "$$GO_VERSION" ]; then \
		echo "$(RED)Error: Go is not available$(NC)"; \
		echo "Please ensure Go $(GOVERSION) is installed via mise"; \
		exit 1; \
	elif CURRENT_NUM=$$(echo "$$GO_VERSION" | awk -F. '{maj=$$1+0; min=($$2==""?0:$$2)+0; pat=($$3==""?0:$$3)+0; printf "%03d%03d%03d", maj, min, pat}'); \
	REQUIRED_NUM=$$(echo "$$REQUIRED_VERSION" | awk -F. '{maj=$$1+0; min=($$2==""?0:$$2)+0; pat=($$3==""?0:$$3)+0; printf "%03d%03d%03d", maj, min, pat}'); \
	[ "$$CURRENT_NUM" -lt "$$REQUIRED_NUM" ]; then \
		echo "$(YELLOW)Warning: Go version $$GO_VERSION found, but $(GOVERSION) is required$(NC)"; \
		echo "Please update Go to version $(GOVERSION) with: mise use go@$(GOVERSION)"; \
		exit 1; \
	else \
		echo "$(GREEN)Go version $$GO_VERSION is compatible$(NC)"; \
	fi

link-skills:
	@bash scripts/link-skills.sh

setup: check-go-version deps link-skills
	@echo "$(GREEN)Setup complete! You can now run 'make build' or 'make verify'$(NC)"

# -----------------------------------------------------------------------------
# Main Targets
# -----------------------------------------------------------------------------
all: test lint fmt

clean:
	rm -rf $(BINARY_DIR)/
	$(GOCMD) clean

build: check-go-version
	mkdir -p $(BINARY_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/compozy
	chmod +x $(BINARY_DIR)/$(BINARY_NAME)

build-extension-sdks:
	npm run build --workspace @compozy/extension-sdk --workspace @compozy/create-extension

publish-extension-sdks:
	npm publish --workspace @compozy/extension-sdk
	npm publish --workspace @compozy/create-extension

install: build
	$(GOCMD) install -ldflags "$(LDFLAGS)" ./cmd/compozy

# -----------------------------------------------------------------------------
# Code Quality & Formatting
# -----------------------------------------------------------------------------
lint:
	$(LINTCMD) run --fix --allow-parallel-runners
	@echo "Linting completed successfully"

fmt:
	@echo "Formatting code..."
	$(LINTCMD) fmt
	@echo "Formatting completed successfully"

# -----------------------------------------------------------------------------
# Verification Pipeline (BLOCKING GATE for any change)
# -----------------------------------------------------------------------------
verify: fmt lint test build
	@echo "$(GREEN)All verification checks passed$(NC)"

# -----------------------------------------------------------------------------
# Development & Dependencies
# -----------------------------------------------------------------------------
tidy:
	@echo "Tidying modules..."
	$(GOCMD) mod tidy

deps: check-go-version
	@echo "Installing Go dependencies..."
	@echo "Installing gotestsum..."
	@$(GOCMD) install gotest.tools/gotestsum@latest
	@echo "Installing golangci-lint v2..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $$($(GOCMD) env GOPATH)/bin $(GOLANGCI_LINT_VERSION)
	@echo "$(GREEN)All dependencies installed successfully$(NC)"

# -----------------------------------------------------------------------------
# Testing
# -----------------------------------------------------------------------------
test:
	@gotestsum --format pkgname -- -race -parallel=4 ./...

test-coverage:
	@gotestsum --format pkgname -- -race -parallel=4 -coverprofile=coverage.out -covermode=atomic ./...

test-nocache:
	@gotestsum --format pkgname -- -race -count=1 -parallel=4 ./...

# -----------------------------------------------------------------------------
# Help
# -----------------------------------------------------------------------------
help:
	@echo "Available targets:"
	@echo "  make build          - Build the compozy binary"
	@echo "  make build-extension-sdks - Build the npm extension SDK and scaffolder packages"
	@echo "  make install        - Build and install to GOPATH/bin"
	@echo "  make test           - Run tests with race detector"
	@echo "  make lint           - Run golangci-lint"
	@echo "  make fmt            - Format code"
	@echo "  make verify         - Run full verification pipeline (fmt + lint + test + build)"
	@echo "  make deps           - Install development dependencies"
	@echo "  make tidy           - Tidy Go modules"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make setup          - Full setup (check Go version + deps)"
	@echo "  make publish-extension-sdks - Publish the npm extension SDK and scaffolder packages"
	@echo "  make test-coverage  - Run tests with coverage"
	@echo "  make test-nocache   - Run tests without cache"
	@echo "  make help           - Show this help"

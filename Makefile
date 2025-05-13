# Makefile for Compozy Go Project

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOFMT=gofmt -s -w
BINARY_NAME=compozy
BINARY_DIR=bin
SRC_DIRS=./...

.PHONY: all test lint fmt clean build dev dev-weather deps schemagen help integration-test tidy

# Default target
all: test lint fmt

# Run tests using gotestsum
test:
	gotestsum -f testdox -- $(SRC_DIRS)

# Run integration tests
integration-test:
	gotestsum -f testdox -- ./test/integration/...

# Run linter using golangci-lint
lint:
	golangci-lint run --fix
	@echo "Linting completed successfully"

# Format code using gofmt
fmt:
	@echo "Formatting code..."
	@find . -name "*.go" -not -path "./vendor/*" -exec $(GOFMT) {} \;
	@find . -name "*.go" -not -path "./vendor/*" -exec golines -m 120 -w {} \;
	@echo "Formatting completed successfully"

# Clean build artifacts
clean:
	rm -rf $(BINARY_DIR)/
	$(GOCMD) clean

# Build the application
build:
	mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(BINARY_DIR)/$(BINARY_NAME) .
	chmod +x $(BINARY_DIR)/$(BINARY_NAME)

# Run the development server
dev:
	$(GOCMD) run . dev

# Run the development server with weather-agent example
dev-weather:
	wgo run . dev --cwd examples/weather-agent --debug

# Tidy go.mod and go.sum
tidy:
	@echo "Tidying modules..."
	$(GOCMD) mod tidy

# Install development dependencies
deps:
	$(GOCMD) install gotest.tools/gotestsum@latest
	$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOCMD) install github.com/bokwoon95/wgo@latest
	$(GOCMD) install github.com/segmentio/golines@latest

# Generate JSON schemas
schemagen:
	$(GOCMD) run cmd/schemagen/generate.go -out=./schemas

# Help command
help:
	@echo "-------------------------------------------------------------------------"
	@echo "Available commands:"
	@echo "  make all         - Run tests, lint, and format code (default)"
	@echo "  make test        - Run tests using gotestsum"
	@echo "  make integration-test - Run integration tests"
	@echo "  make lint        - Run linter using golangci-lint"
	@echo "  make fmt         - Format code using gofmt"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make build       - Build the application"
	@echo "  make dev         - Run the development server"
	@echo "  make dev-weather - Run the dev server with weather-agent example"
	@echo "  make deps        - Install development dependencies"
	@echo "  make tidy        - Tidy go.mod and go.sum files"
	@echo "  make schemagen   - Generate JSON schemas"
	@echo "  make help        - Show this help message"
	@echo "-------------------------------------------------------------------------"

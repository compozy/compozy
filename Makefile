.PHONY: all test lint clean build

# Default target
all: test lint

# Run tests using gotestsum
test:
	gotestsum --format testname -- ./...

# Run linter using golangci-lint
lint:
	golangci-lint run

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Build the application
build:
	go build -o bin/app ./cmd/app

# Install development dependencies
deps:
	go install gotest.tools/gotestsum@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Help command
help:
	@echo "Available commands:"
	@echo "  make test    - Run tests using gotestsum"
	@echo "  make lint    - Run linter using golangci-lint"
	@echo "  make clean   - Clean build artifacts"
	@echo "  make build   - Build the application"
	@echo "  make deps    - Install development dependencies"
	@echo "  make help    - Show this help message" 
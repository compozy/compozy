.PHONY: all test lint clean build dev dev-weather deps schemagen help

# Default target
all: test lint

# Run tests using gotestsum
test:
	gotestsum -- ./...

# Run linter using golangci-lint
lint:
	golangci-lint run

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Build the application
build:
	mkdir -p bin
	go build -o bin/compozy .
	chmod +x bin/compozy

# Run the development server
dev:
	go run . dev

# Run the development server with weather-agent example
dev-weather:
	go run . dev --cwd examples/weather-agent

# Install development dependencies
deps:
	go install gotest.tools/gotestsum@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Generate JSON schemas
schemagen:
	go run cmd/schemagen/generate.go -out=./schemas

# Help command
help:
	@echo "Available commands:"
	@echo "  make test    - Run tests using gotestsum"
	@echo "  make lint    - Run linter using golangci-lint"
	@echo "  make clean   - Clean build artifacts"
	@echo "  make build   - Build the application"
	@echo "  make dev     - Run the development server"
	@echo "  make dev-weather - Run the dev server with weather-agent example"
	@echo "  make deps    - Install development dependencies"
	@echo "  make help    - Show this help message"
	@echo "  make schemagen - Generate JSON schemas"
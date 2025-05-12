.PHONY: all test lint fmt clean build dev dev-weather deps schemagen help integration-test

# Default target
all: test lint fmt

# Run tests using gotestsum
test:
	gotestsum -f testdox -- ./...

# Run linter using golangci-lint
lint:
	golangci-lint run
	@echo "Linting completed successfully"

# Format code using gofmt
fmt:
	@echo "Formatting code..."
	@find . -name "*.go" -not -path "./vendor/*" -exec gofmt -s -w {} \;
	@echo "Formatting completed successfully"

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
	wgo run . dev --cwd examples/weather-agent --debug

# Install development dependencies
deps:
	go install gotest.tools/gotestsum@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/bokwoon95/wgo@latest

# Generate JSON schemas
schemagen:
	go run cmd/schemagen/generate.go -out=./schemas

# Run integration tests
integration-test:
	gotestsum -f testdox -- ./test/integration/...

# Help command
help:
	@echo "Available commands:"
	@echo "  make test    - Run tests using gotestsum"
	@echo "  make integration-test - Run integration tests"
	@echo "  make lint    - Run linter using golangci-lint"
	@echo "  make fmt     - Format code using gofmt"
	@echo "  make clean   - Clean build artifacts"
	@echo "  make build   - Build the application"
	@echo "  make dev     - Run the development server"
	@echo "  make dev-weather - Run the dev server with weather-agent example"
	@echo "  make deps    - Install development dependencies"
	@echo "  make help    - Show this help message"
	@echo "  make schemagen - Generate JSON schemas"

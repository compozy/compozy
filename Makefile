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

# Docker compose
start-nats:
	docker compose -f docker-compose.yml up -d

# Docker compose down
stop-nats:
	docker compose -f docker-compose.yml down

clean-nats:
	docker compose -f docker-compose.yml down --volumes --rmi all

restart-nats:
	make stop-nats
	make start-nats

# Run tests
test:
	make start-nats
	make test-runtime
	make stop-nats

test-runtime:
	@echo "Running runtime tests..."
	@sleep 1
	@make start-nats
	@deno test --allow-sys --allow-env --allow-net --allow-read packages/runtime/tests/
	@make stop-nats

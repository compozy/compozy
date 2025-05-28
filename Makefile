# Makefile for Compozy Go Project

# -----------------------------------------------------------------------------
# Go Parameters & Setup
# -----------------------------------------------------------------------------
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOFMT=gofmt -s -w
BINARY_NAME=compozy
BINARY_DIR=bin
SRC_DIRS=./...

# -----------------------------------------------------------------------------
# Protobuf Generation
# -----------------------------------------------------------------------------
PROTO_DIR=./proto
PROTO_OUT_DIR=./pkg/pb
GO_MODULE_NAME := $(shell go list -m)
EVENT_PROTO_FILES=$(shell find $(PROTO_DIR) -path '*/events/*.proto')
COMMAND_PROTO_FILES=$(shell find $(PROTO_DIR) -path '*/cmds/*.proto')
COMMON_PROTO_FILES=$(shell find $(PROTO_DIR)/common -name '*.proto')

# -----------------------------------------------------------------------------
# Swagger/OpenAPI
# -----------------------------------------------------------------------------
SWAGGER_DIR=./docs
SWAGGER_OUTPUT=$(SWAGGER_DIR)/swagger.json

.PHONY: all test lint fmt clean build dev dev-weather deps schemagen help integration-test
.PHONY: tidy proto proto-deps test-go test-runtime start-nats stop-nats clean-nats restart-nats
.PHONY: swagger swagger-deps swagger-gen swagger-serve

# -----------------------------------------------------------------------------
# Main Targets
# -----------------------------------------------------------------------------
all: proto swagger test lint fmt

clean:
	rm -rf $(BINARY_DIR)/
	rm -rf $(SWAGGER_DIR)/
	$(GOCMD) clean

build: proto swagger
	mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(BINARY_DIR)/$(BINARY_NAME) .
	chmod +x $(BINARY_DIR)/$(BINARY_NAME)

# -----------------------------------------------------------------------------
# Code Quality & Formatting
# -----------------------------------------------------------------------------
lint:
	golangci-lint run --fix
	@echo "Linting completed successfully"

fmt:
	@echo "Formatting code..."
	@golangci-lint fmt
	@gofumpt -l -w .
	@echo "Formatting completed successfully"

# -----------------------------------------------------------------------------
# Development & Dependencies
# -----------------------------------------------------------------------------
dev:
	$(GOCMD) run . dev

dev-weather:
	wgo run . dev --cwd examples/weather-agent --debug

tidy:
	@echo "Tidying modules..."
	$(GOCMD) mod tidy

deps: proto-deps swagger-deps
	$(GOCMD) install gotest.tools/gotestsum@latest
	$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOCMD) install github.com/bokwoon95/wgo@latest
	$(GOCMD) install github.com/segmentio/golines@latest

proto-deps:
	@echo "Installing Go protoc plugins..."
	$(GOCMD) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GOCMD) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Go protoc plugins installation complete."

swagger-deps:
	@echo "Installing Swagger dependencies..."
	$(GOCMD) install github.com/swaggo/swag/cmd/swag@latest
	@echo "Swagger dependencies installation complete."

proto:
	@echo "Generating Go code from protobuf definitions via script..."
	@chmod +x scripts/proto.sh
	@bash scripts/proto.sh "$(PROTO_DIR)" "$(PROTO_OUT_DIR)" "$(GO_MODULE_NAME)"

# -----------------------------------------------------------------------------
# Swagger/OpenAPI Generation
# -----------------------------------------------------------------------------
swagger: swagger-gen

swagger-gen:
	@echo "Generating Swagger documentation..."
	@mkdir -p $(SWAGGER_DIR)
	@swag init --dir ./ --generalInfo main.go --output $(SWAGGER_DIR) --parseDependency --parseInternal 2>&1 | grep -v "warning: failed to evaluate const" | grep -v "reflect: call of reflect.Value" | grep -v "strconv.ParseUint: parsing" || true
	@echo "Swagger documentation generated at $(SWAGGER_DIR)"

swagger-validate:
	@echo "Validating Swagger documentation..."
	@swag init --dir ./ --generalInfo main.go --output $(SWAGGER_DIR) --parseDependency --parseInternal --quiet
	@echo "Swagger documentation is valid"

# -----------------------------------------------------------------------------
# Schema Generation
# -----------------------------------------------------------------------------
schemagen:
	$(GOCMD) run pkg/schemagen/generate.go -out=./schemas

# -----------------------------------------------------------------------------
# Testing
# -----------------------------------------------------------------------------
integration-test:
	gotestsum -f testdox -- ./test/integration/...

test-go:
	gotestsum --format testdox ./...

test-go-nocache:
	gotestsum --format testdox -- -count=1 ./...

test-runtime:
	@echo "Running runtime tests..."
	@sleep 1
	@make start-nats
	@deno test --allow-sys --allow-env --allow-net --allow-read pkg/runtime/tests/
	@make stop-nats

test:
	make start-nats
	make test-go
	make test-runtime
	make stop-nats

# -----------------------------------------------------------------------------
# Docker & NATS Management
# -----------------------------------------------------------------------------
start-nats:
	docker compose -f docker-compose.yml up -d

stop-nats:
	docker compose -f docker-compose.yml down

clean-nats:
	docker compose -f docker-compose.yml down --volumes --rmi all

restart-nats:
	make stop-nats
	make start-nats

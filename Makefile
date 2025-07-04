-include .env
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
LINTCMD=golangci-lint-v2

# -----------------------------------------------------------------------------
# Build Variables
# -----------------------------------------------------------------------------
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION := $(shell git describe --tags --always 2>/dev/null || echo "unknown")

# Build flags for injecting version info
LDFLAGS := "-X 'github.com/compozy/compozy/engine/infra/monitoring.Version=$(VERSION)' -X 'github.com/compozy/compozy/engine/infra/monitoring.CommitHash=$(GIT_COMMIT)'"

# -----------------------------------------------------------------------------
# Swagger/OpenAPI
# -----------------------------------------------------------------------------
SWAGGER_DIR=./docs
SWAGGER_OUTPUT=$(SWAGGER_DIR)/swagger.json

.PHONY: all test lint fmt clean build dev deps schemagen help integration-test
.PHONY: tidy test-go start-docker stop-docker clean-docker reset-docker mcp-proxy rebuild-mcp-proxy
.PHONY: swagger swagger-deps swagger-gen swagger-serve

# -----------------------------------------------------------------------------
# Main Targets
# -----------------------------------------------------------------------------
all: swagger test lint fmt

clean:
	rm -rf $(BINARY_DIR)/
	rm -rf $(SWAGGER_DIR)/
	$(GOCMD) clean

build: swagger
	mkdir -p $(BINARY_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BINARY_DIR)/$(BINARY_NAME) .
	chmod +x $(BINARY_DIR)/$(BINARY_NAME)

# -----------------------------------------------------------------------------
# Code Quality & Formatting
# -----------------------------------------------------------------------------
lint:
	$(LINTCMD) run --fix --allow-parallel-runners
	@echo "Linting completed successfully"

fmt:
	@echo "Formatting code..."
	$(LINTCMD) fmt
	@deno task prettier:fix
	@echo "Formatting completed successfully"

# -----------------------------------------------------------------------------
# Development & Dependencies
# -----------------------------------------------------------------------------

dev: EXAMPLE=weather
dev:
	wgo run . dev --cwd examples/$(EXAMPLE) --env-file .env --debug --watch

mcp-proxy:
	$(GOCMD) run . mcp-proxy

tidy:
	@echo "Tidying modules..."
	$(GOCMD) mod tidy

deps: swagger-deps
	$(GOCMD) install gotest.tools/gotestsum@latest
	$(GOCMD) install github.com/bokwoon95/wgo@latest
	$(GOCMD) install github.com/pressly/goose/v3/cmd/goose@latest
	$(GOCMD) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6

swagger-deps:
	@echo "Installing Swagger dependencies..."
	$(GOCMD) install github.com/swaggo/swag/cmd/swag@latest
	@echo "Swagger dependencies installation complete."

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
	$(GOCMD) run pkg/schemagen/main.go -out=./schemas

# -----------------------------------------------------------------------------
# Testing
# -----------------------------------------------------------------------------

E2E_TESTS=./test/e2e/...

test:
	gotestsum --format pkgname -- -race -parallel=4 ./...

test-nocache:
	gotestsum --format pkgname -- -race -count=1 -parallel=4 ./...

# -----------------------------------------------------------------------------
# Docker & Database Management
# -----------------------------------------------------------------------------
start-docker:
	docker compose -f ./cluster/docker-compose.yml up -d

stop-docker:
	docker compose -f ./cluster/docker-compose.yml down

clean-docker:
	docker compose -f ./cluster/docker-compose.yml down --volumes

reset-docker:
	make clean-docker
	make start-docker

rebuild-mcp-proxy:
	docker compose -f ./cluster/docker-compose.yml build mcp-proxy
	docker compose -f ./cluster/docker-compose.yml up -d mcp-proxy

# -----------------------------------------------------------------------------
# Database
# -----------------------------------------------------------------------------
DB_USER ?= postgres
DB_PASSWORD ?= postgres
DB_HOST ?= localhost
DB_PORT ?= 5432
DB_NAME ?= compozy

GOOSE_DBSTRING=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable
GOOSE_COMMAND = GOOSE_DRIVER=postgres GOOSE_DBSTRING=${GOOSE_DBSTRING} goose -dir ./engine/infra/store/migrations

migrate-status:
	$(GOOSE_COMMAND) status

migrate-up:
	$(GOOSE_COMMAND) up

migrate-down:
	$(GOOSE_COMMAND) down

migrate-create:
	$(GOOSE_COMMAND) create $(name) sql

migrate-validate:
	$(GOOSE_COMMAND) validate

migrate-reset:
	$(GOOSE_COMMAND) reset

reset-db:
	@make reset-docker
	@sleep 2
	@make migrate-up

# -----------------------------------------------------------------------------
# Redis
# -----------------------------------------------------------------------------
REDIS_PASSWORD ?= redis_secret
REDIS_HOST ?= localhost
REDIS_PORT ?= 6379

redis-cli:
	docker exec -it redis redis-cli -a ${REDIS_PASSWORD}

redis-info:
	docker exec redis redis-cli -a ${REDIS_PASSWORD} info

redis-monitor:
	docker exec -it redis redis-cli -a ${REDIS_PASSWORD} monitor

redis-flush:
	docker exec redis redis-cli -a ${REDIS_PASSWORD} flushall

test-redis:
	@echo "Testing Redis connection..."
	@docker exec redis redis-cli -a ${REDIS_PASSWORD} ping

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
# Swagger/OpenAPI
# -----------------------------------------------------------------------------
SWAGGER_DIR=./docs
SWAGGER_OUTPUT=$(SWAGGER_DIR)/swagger.json

.PHONY: all test lint fmt clean build dev dev-weather deps schemagen help integration-test
.PHONY: tidy test-go start-docker stop-docker clean-docker reset-docker
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
	$(GOBUILD) -o $(BINARY_DIR)/$(BINARY_NAME) .
	chmod +x $(BINARY_DIR)/$(BINARY_NAME)

# -----------------------------------------------------------------------------
# Code Quality & Formatting
# -----------------------------------------------------------------------------
lint:
	$(LINTCMD) run --fix
	@echo "Linting completed successfully"

fmt:
	@echo "Formatting code..."
	$(LINTCMD) fmt
	@echo "Formatting completed successfully"

# -----------------------------------------------------------------------------
# Development & Dependencies
# -----------------------------------------------------------------------------
dev:
	$(GOCMD) run . dev

dev-weather:
	wgo run . dev --cwd examples/weather-agent --env-file .env --debug --watch

tidy:
	@echo "Tidying modules..."
	$(GOCMD) mod tidy

deps: swagger-deps
	$(GOCMD) install gotest.tools/gotestsum@latest
	$(GOCMD) install github.com/bokwoon95/wgo@latest
	$(GOCMD) install github.com/pressly/goose/v3/cmd/goose@latest

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
	$(GOCMD) run pkg/schemagen/generate.go -out=./schemas

# -----------------------------------------------------------------------------
# Testing
# -----------------------------------------------------------------------------
# Fast tests for daily development (excludes slow integration/worker tests)
test:
	gotestsum --format testdox -- -parallel=8 $(shell go list ./... | grep -v '/test/integration/worker')

test-nocache:
	gotestsum --format testdox -- -count=1 -parallel=8 ./...

test-all:
	gotestsum --format testdox -- -parallel=8 ./...

test-worker:
	gotestsum --format testdox -- -parallel=16 ./test/integration/worker/...

test-no-worker:
	gotestsum --format testdox -- -parallel=16 $(shell go list ./... | grep -v '/test/integration/worker')

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
	make stop-docker
	make start-docker

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
	@make migrate-reset
	@make migrate-up

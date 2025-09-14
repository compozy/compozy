-include .env
# Makefile for Compozy Go Project

# -----------------------------------------------------------------------------
# Go Parameters & Setup
# -----------------------------------------------------------------------------
GOCMD=$(shell which go)
GOVERSION ?= $(shell awk '/^go /{print $$2}' go.mod 2>/dev/null || echo "1.25")
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOFMT=gofmt -s -w
BINARY_NAME=compozy
BINARY_DIR=bin
SRC_DIRS=./...
LINTCMD=golangci-lint
BUNCMD=bun

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
LDFLAGS := -X github.com/compozy/compozy/pkg/version.Version=$(VERSION) -X github.com/compozy/compozy/pkg/version.CommitHash=$(GIT_COMMIT) -X github.com/compozy/compozy/pkg/version.BuildDate=$(BUILD_DATE)

# -----------------------------------------------------------------------------
# Swagger/OpenAPI
# -----------------------------------------------------------------------------
SWAGGER_DIR=./docs
SWAGGER_OUTPUT=$(SWAGGER_DIR)/swagger.json

.PHONY: all test lint fmt modernize clean build dev deps schemagen schemagen-watch help integration-test
.PHONY: tidy test-go start-docker stop-docker clean-docker reset-docker
.PHONY: swagger swagger-deps swagger-gen swagger-serve check-go-version setup clean-go-cache

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
	elif [ "$$(printf '%s\n' "$$REQUIRED_VERSION" "$$GO_VERSION" | sort -V | head -n1)" != "$$REQUIRED_VERSION" ]; then \
		echo "$(YELLOW)Warning: Go version $$GO_VERSION found, but $(GOVERSION) is required$(NC)"; \
		echo "Please update Go to version $(GOVERSION) with: mise use go@$(GOVERSION)"; \
		exit 1; \
	else \
		echo "$(GREEN)✓ Go version $$GO_VERSION is compatible$(NC)"; \
	fi

setup: check-go-version deps
	@echo "$(GREEN)✓ Setup complete! You can now run 'make build' or 'make dev'$(NC)"

# -----------------------------------------------------------------------------
# Main Targets
# -----------------------------------------------------------------------------
all: swagger test lint fmt

clean:
	rm -rf $(BINARY_DIR)/
	rm -rf $(SWAGGER_DIR)/
	$(GOCMD) clean

build: check-go-version swagger
	mkdir -p $(BINARY_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BINARY_DIR)/$(BINARY_NAME) ./
	chmod +x $(BINARY_DIR)/$(BINARY_NAME)

# -----------------------------------------------------------------------------
# Code Quality & Formatting
# -----------------------------------------------------------------------------
lint:
	$(BUNCMD) run lint
	$(LINTCMD) run --fix --allow-parallel-runners
	@echo "Running modernize analyzer for min/max suggestions..."
	@echo "Linting completed successfully"

fmt:
	@echo "Formatting code..."
	$(BUNCMD) run format
	$(LINTCMD) fmt
	@echo "Formatting completed successfully"

modernize:
	$(GOCMD) run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest -fix ./...

# -----------------------------------------------------------------------------
# Development & Dependencies
# -----------------------------------------------------------------------------

dev: EXAMPLE=weather
dev:
	wgo run . dev --cwd examples/$(EXAMPLE) --env-file .env --debug --watch

tidy:
	@echo "Tidying modules..."
	$(GOCMD) mod tidy

deps: check-go-version clean-go-cache swagger-deps
	@echo "Installing Go dependencies..."
	@echo "Installing gotestsum..."
	@$(GOCMD) install gotest.tools/gotestsum@latest
	@echo "Installing wgo for hot reload..."
	@$(GOCMD) install github.com/bokwoon95/wgo@latest
	@echo "Installing goose for migrations..."
	@$(GOCMD) install github.com/pressly/goose/v3/cmd/goose@latest
	@echo "Installing golangci-lint v2..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $$($(GOCMD) env GOPATH)/bin v2.2.1
	@echo "$(GREEN)✓ All dependencies installed successfully$(NC)"

clean-go-cache:
	@echo "Cleaning Go build cache for fresh setup..."
	@$(GOCMD) clean -cache -testcache -modcache 2>/dev/null || true
	@echo "$(GREEN)✓ Go cache cleaned$(NC)"

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
	@echo "Running pre-commit on generated swagger files..."
	@pre-commit run --files $(SWAGGER_DIR)/docs.go $(SWAGGER_DIR)/swagger.json $(SWAGGER_DIR)/swagger.yaml || true
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

schemagen-watch:
	$(GOCMD) run pkg/schemagen/main.go -out=./schemas -watch

# -----------------------------------------------------------------------------
# Release Management
# -----------------------------------------------------------------------------
.PHONY: release release-dry-run release-minor release-major release-patch release-deps compozy-release

# Build the compozy-release binary
compozy-release:
	mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(BINARY_DIR)/compozy-release ./pkg/release

# Install go-semantic-release
release-deps:
	@echo "Installing go-semantic-release..."
	@$(GOCMD) install github.com/go-semantic-release/semantic-release/v2/cmd/semantic-release@latest

# Run semantic-release in dry-run mode to preview the next version
release-dry-run: release-deps
	@echo "Running semantic-release in dry-run mode..."
	@semantic-release --dry --allow-initial-development-versions

# Create a new release based on conventional commits
release: release-deps
	@echo "Creating new release based on conventional commits..."
	@semantic-release --allow-initial-development-versions

# Force a patch release
release-patch: release-deps
	@echo "Creating patch release..."
	@echo "fix: patch release" | git commit --allow-empty -F -
	@semantic-release --allow-initial-development-versions

# Force a minor release
release-minor: release-deps
	@echo "Creating minor release..."
	@echo "feat: minor release" | git commit --allow-empty -F -
	@semantic-release --allow-initial-development-versions

# Force a major release
release-major: release-deps
	@echo "Creating major release..."
	@echo "feat!: major release" | git commit --allow-empty -F -
	@semantic-release --allow-initial-development-versions

# -----------------------------------------------------------------------------
# Testing
# -----------------------------------------------------------------------------

test:
	@bun run test
	@gotestsum --format pkgname -- -race -parallel=4 ./...

test-coverage:
	@bun run test
	@gotestsum --format pkgname -- -race -parallel=4 -coverprofile=coverage.out -covermode=atomic ./...

test-nocache:
	@bun run test
	@gotestsum --format pkgname -- -race -count=1 -parallel=4 ./...

# Run Redis-dependent tests marked with the 'distributed' build tag
.PHONY: test-distributed
test-distributed:
	@echo "Running distributed (Redis) test lane..."
	@gotestsum --format pkgname -- -race -parallel=4 -tags=distributed ./...

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

# -----------------------------------------------------------------------------
# Database
# -----------------------------------------------------------------------------
DB_USER ?= postgres
DB_PASSWORD ?= postgres
DB_HOST ?= localhost
DB_PORT ?= 5432
DB_NAME ?= compozy

GOOSE_DBSTRING=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable
GOOSE_COMMAND = GOOSE_DRIVER=postgres GOOSE_DBSTRING=${GOOSE_DBSTRING} goose -dir ./engine/infra/postgres/migrations

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
# -----------------------------------------------------------------------------
# Help
# -----------------------------------------------------------------------------
help:
	@echo "$(GREEN)Compozy Makefile Commands$(NC)"
	@echo ""
	@echo "$(YELLOW)Setup & Build:$(NC)"
	@echo "  make setup          - Complete setup with Go version check and dependencies"
	@echo "  make deps           - Install all required dependencies"
	@echo "  make build          - Build the compozy binary"
	@echo "  make clean          - Clean build artifacts"
	@echo ""
	@echo "$(YELLOW)Development:$(NC)"
	@echo "  make dev            - Run in development mode with hot reload"
		@echo "  make test           - Run all tests"
		@echo "  make test-distributed - Run Redis-only tests (build tag: distributed)"
		@echo "  make lint           - Run linters and fix issues"
		@echo "  make fmt            - Format code"
	@echo ""
	@echo "$(YELLOW)Docker & Database:$(NC)"
	@echo "  make start-docker   - Start Docker services"
	@echo "  make stop-docker    - Stop Docker services"
	@echo "  make migrate-up     - Run database migrations"
	@echo "  make migrate-down   - Rollback last migration"
	@echo ""
	@echo "$(YELLOW)Requirements:$(NC)"
	@echo "  Go $(GOVERSION) or later (via mise)"
	@echo "  Bun (see https://bun.sh for install instructions or use Homebrew: brew install oven-sh/bun/bun)"
	@echo "  Docker & Docker Compose"
	@echo ""
	@echo "$(GREEN)Quick Start:$(NC)"
	@echo "  1. make setup        # Install dependencies"
	@echo "  2. make start-docker # Start services"
	@echo "  3. make migrate-up   # Setup database"
	@echo "  4. make dev          # Start development server"

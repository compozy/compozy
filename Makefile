-include .env
# Makefile for Compozy Go Project
# This Makefile delegates to Mage for parallel execution and smart caching
# Install mage: go install github.com/magefile/mage@latest

# -----------------------------------------------------------------------------
# Configuration
# -----------------------------------------------------------------------------
MAGE=$(shell which mage 2>/dev/null)

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
NC := \033[0m # No Color

.PHONY: all test lint fmt modernize clean build dev deps schemagen schemagen-watch help integration-test typecheck
.PHONY: tidy start-docker stop-docker clean-docker reset-docker check-mage
.PHONY: swagger swagger-validate setup
.PHONY: lint-main lint-sdk typecheck-main typecheck-sdk fmt-main fmt-sdk modernize-main modernize-sdk
.PHONY: test-main test-sdk test-coverage test-coverage-main test-coverage-sdk test-nocache test-nocache-main test-nocache-sdk

# -----------------------------------------------------------------------------
# Mage Check
# -----------------------------------------------------------------------------
check-mage:
	@if [ -z "$(MAGE)" ]; then \
		echo "$(RED)Error: mage is not installed$(NC)"; \
		echo "Install with: go install github.com/magefile/mage@latest"; \
		exit 1; \
	fi

# -----------------------------------------------------------------------------
# Main Targets (Mage Delegates - Parallel Execution)
# -----------------------------------------------------------------------------
all: check-mage
	@$(MAGE) all

setup: check-mage
	@$(MAGE) setup

clean: check-mage
	@$(MAGE) clean

build: check-mage
	@$(MAGE) build

# -----------------------------------------------------------------------------
# Code Quality & Formatting (Mage Delegates - Parallel Execution)
# -----------------------------------------------------------------------------
lint: check-mage
	@$(MAGE) quality:lint

lint-main: check-mage
	@$(MAGE) lintMain

lint-sdk: check-mage
	@$(MAGE) lintSDK

typecheck: check-mage
	@$(MAGE) quality:typecheck

typecheck-main: check-mage
	@$(MAGE) typecheckMain

typecheck-sdk: check-mage
	@$(MAGE) typecheckSDK

fmt: check-mage
	@$(MAGE) quality:fmt

fmt-main: check-mage
	@$(MAGE) fmtMain

fmt-sdk: check-mage
	@$(MAGE) fmtSDK

modernize: check-mage
	@$(MAGE) quality:modernize

modernize-main: check-mage
	@$(MAGE) modernizeMain

modernize-sdk: check-mage
	@$(MAGE) modernizeSDK

# -----------------------------------------------------------------------------
# Development & Dependencies (Mage Delegates)
# -----------------------------------------------------------------------------
dev: check-mage
	@$(MAGE) dev

tidy: check-mage
	@$(MAGE) tidy

deps: check-mage
	@$(MAGE) deps

# -----------------------------------------------------------------------------
# Swagger/OpenAPI Generation (Mage Delegates)
# -----------------------------------------------------------------------------
swagger: check-mage
	@$(MAGE) swagger

swagger-validate: check-mage
	@$(MAGE) swaggerValidate

# -----------------------------------------------------------------------------
# Schema Generation (Mage Delegates)
# -----------------------------------------------------------------------------
schemagen: check-mage
	@$(MAGE) schema:generate

schemagen-watch: check-mage
	@$(MAGE) schema:watch

# -----------------------------------------------------------------------------
# Testing (Mage Delegates - Parallel Execution)
# -----------------------------------------------------------------------------
test: check-mage
	@$(MAGE) test

test-main: check-mage
	@$(MAGE) testMain

test-sdk: check-mage
	@$(MAGE) testSDK

test-coverage: check-mage
	@$(MAGE) testCoverage

test-coverage-main: check-mage
	@$(MAGE) testCoverageMain

test-coverage-sdk: check-mage
	@$(MAGE) testCoverageSDK

test-nocache: check-mage
	@$(MAGE) testNoCache

test-nocache-main: check-mage
	@$(MAGE) testNoCacheMain

test-nocache-sdk: check-mage
	@$(MAGE) testNoCacheSDK

integration-sdk-compozy: check-mage
	@$(MAGE) integration:sdkCompozy

# -----------------------------------------------------------------------------
# Docker & Database Management (Mage Delegates)
# -----------------------------------------------------------------------------
start-docker: check-mage
	@$(MAGE) docker:start

stop-docker: check-mage
	@$(MAGE) docker:stop

clean-docker: check-mage
	@$(MAGE) docker:clean

reset-docker: check-mage
	@$(MAGE) docker:reset

# -----------------------------------------------------------------------------
# Database (Mage Delegates)
# -----------------------------------------------------------------------------
migrate-status: check-mage
	@$(MAGE) database:status

migrate-up: check-mage
	@$(MAGE) database:up

migrate-down: check-mage
	@$(MAGE) database:down

migrate-create: check-mage
	@$(MAGE) database:create

migrate-validate: check-mage
	@$(MAGE) database:validate

migrate-reset: check-mage
	@$(MAGE) database:reset

reset-db: check-mage
	@$(MAGE) docker:reset

# -----------------------------------------------------------------------------
# Redis (Mage Delegates)
# -----------------------------------------------------------------------------
redis-cli: check-mage
	@$(MAGE) redis:cli

redis-info: check-mage
	@$(MAGE) redis:info

redis-monitor: check-mage
	@$(MAGE) redis:monitor

redis-flush: check-mage
	@$(MAGE) redis:flush

test-redis: check-mage
	@$(MAGE) redis:testConnection
# -----------------------------------------------------------------------------
# Help
# -----------------------------------------------------------------------------
help:
	@echo "$(GREEN)Compozy Makefile Commands (Powered by Mage)$(NC)"
	@echo ""
	@echo "$(YELLOW)⚡ Performance: Tests run ~2-3x faster with parallel execution!$(NC)"
	@echo ""
	@echo "$(YELLOW)Setup & Build:$(NC)"
	@echo "  make setup          - Complete setup with Go version check and dependencies"
	@echo "  make deps           - Install all required dependencies"
	@echo "  make build          - Build the compozy binary (with smart caching)"
	@echo "  make clean          - Clean build artifacts"
	@echo ""
	@echo "$(YELLOW)Development:$(NC)"
	@echo "  make dev            - Run in development mode with hot reload"
	@echo "  make test           - Run all tests (main + sdk + bun) in parallel"
	@echo "  make test-coverage  - Run all tests with coverage reports"
	@echo "  make test-nocache   - Run all tests without cache"
	@echo "  make lint           - Run linters in parallel (main + sdk + bun)"
	@echo "  make fmt            - Format code in parallel (main + sdk + bun)"
	@echo "  make typecheck      - Type check all modules in parallel"
	@echo "  make modernize      - Modernize code patterns in parallel"
	@echo ""
	@echo "$(YELLOW)Docker & Database:$(NC)"
	@echo "  make start-docker   - Start Docker services"
	@echo "  make stop-docker    - Stop Docker services"
	@echo "  make reset-docker   - Reset Docker environment"
	@echo "  make migrate-up     - Run database migrations"
	@echo "  make migrate-down   - Rollback last migration"
	@echo "  make migrate-create - Create new migration (use: make migrate-create name=my_migration)"
	@echo ""
	@echo "$(YELLOW)Redis:$(NC)"
	@echo "  make redis-cli      - Open Redis CLI"
	@echo "  make redis-info     - Show Redis info"
	@echo "  make redis-monitor  - Monitor Redis commands"
	@echo "  make redis-flush    - Flush all Redis data"
	@echo "  make test-redis     - Test Redis connection"
	@echo ""
	@echo "$(YELLOW)Other:$(NC)"
	@echo "  make swagger        - Generate Swagger documentation (with caching)"
	@echo "  make schemagen      - Generate JSON schemas"
	@echo "  make all            - Run all checks (tests + lint + format)"
	@echo ""
	@echo "$(YELLOW)Advanced:$(NC)"
	@echo "  mage -l             - List all available Mage targets"
	@echo "  mage help           - Show detailed Mage help"
	@echo "  mage <target>       - Run Mage target directly"
	@echo ""
	@echo "$(YELLOW)Requirements:$(NC)"
	@echo "  Go 1.25 or later (via mise)"
	@echo "  Mage (install: go install github.com/magefile/mage@latest)"
	@echo "  Bun (see https://bun.sh)"
	@echo "  Docker & Docker Compose"
	@echo ""
	@echo "$(GREEN)Quick Start:$(NC)"
	@echo "  1. go install github.com/magefile/mage@latest  # Install mage"
	@echo "  2. make setup                                  # Install dependencies"
	@echo "  3. make start-docker                           # Start services"
	@echo "  4. make migrate-up                             # Setup database"
	@echo "  5. make dev                                    # Start development server"

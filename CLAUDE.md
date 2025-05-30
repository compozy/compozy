# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Commands

### Development
- `make dev` - Run Compozy server in development mode
- `make dev-weather` - Run with weather example using wgo (file watcher)

### Building and Dependencies
- `make build` - Build the Compozy binary (includes protobuf and swagger generation)
- `make deps` - Install all required dependencies (protobuf, swagger, testing tools)
- `make proto` - Generate Go code from protobuf definitions
- `make swagger` - Generate OpenAPI/Swagger documentation

### Testing
- `make test` - Run all tests (Go + TypeScript runtime tests with NATS)
- `make test-go` - Run only Go tests with testdox format
- `make test-runtime` - Run TypeScript runtime tests in Deno
- `make integration-test` - Run integration tests

### Code Quality
- `make lint` - Run golangci-lint with auto-fixing
- `make fmt` - Format code using golangci-lint and gofumpt

### Database and Services
- `make start-nats` - Start NATS server via Docker Compose
- `make stop-nats` - Stop NATS server
- `make restart-nats` - Restart NATS server

## Architecture Overview

Compozy is a workflow orchestration engine built with a multi-language architecture:

### Core Components

**Engine** (`/engine/`): The core orchestration logic written in Go
- `agent/` - AI agent execution and management
- `task/` - Task definition and execution logic
- `tool/` - Tool integration and execution
- `workflow/` - Workflow orchestration and state management
- `core/` - Shared types, events, and core abstractions
- `infra/` - Infrastructure layer (NATS messaging, HTTP server, database)

**Runtime** (`/pkg/runtime/`): TypeScript/Deno runtime for executing tools and agents
- Handles tool execution in sandboxed Deno environment
- Communicates with Go engine via NATS messaging
- Provides logging and error handling for runtime operations

### Configuration System

Compozy uses YAML-based configuration with a reference system:
- `compozy.yaml` - Main project configuration file
- Component definitions can be inline or referenced from separate files
- Supports template variables with `{{ .env.VAR_NAME }}` syntax
- JSON Schema validation for all component types

### Messaging Architecture

- **NATS** is used for inter-service communication
- Three main streams: COMMANDS, EVENTS, LOGS
- Event-driven architecture with command/event separation
- Protobuf-based message serialization

### Database

- SQLite with migrations support
- SQLC for type-safe query generation
- Store layer abstracts database operations

## Go Development Standards

### Code Style
- Function length should not exceed 80 lines or 50 statements
- Line length should not exceed 120 characters
- Cyclomatic complexity should be kept below 15
- **DON'T ADD** comments on each function or code block
- Use section comments with dashes for visual separation:
  ```go
  // -----------------------------------------------------------------------------
  // Section Name
  // -----------------------------------------------------------------------------
  ```
- Avoid extra line breaks between variable definitions and code blocks in small functions

### Error Handling
- Always check and handle errors explicitly
- Use wrapped errors for context: `fmt.Errorf("failed to upsert task: %w", err)`
- Return early on errors to avoid deep nesting

### Dependencies and Interfaces
- Prefer explicit dependency injection through constructor functions
- Use interfaces to define behavior and enable testing
- Follow established patterns for service implementation

### Testing
- Write tests using `t.Run("Should...")`
- Use testify for assertions and mocks
- Test both success and error paths
- Ensure test coverage for all exported functions

### Before Delivery
- Always run `make test-go` before committing
- Always run `make lint` before committing

## Development Workflow

1. **Setup**: Run `make deps` to install all dependencies
2. **Development**: Use `make dev` for standard development or `make dev-weather` for file watching
3. **Testing**: Always run `make test` before committing changes
4. **Code Quality**: Run `make lint` and `make fmt` before committing

## Component Types

- **Workflows**: Top-level orchestration units that define execution flow
- **Tasks**: Individual units of work that can contain agents and tools
- **Agents**: AI-powered components that process inputs and make decisions
- **Tools**: Executable functions (TypeScript/Deno) that perform specific operations

## Key Files for Understanding

- `engine/core/types.go` - Central type definitions and constants
- `engine/infra/server/config.go` - Server configuration
- `pkg/runtime/src/processor.ts` - TypeScript runtime processor
- `examples/` - Working examples showing complete workflows

## Testing Notes

- Go tests use `gotestsum` with testdox format for readable output
- TypeScript tests run in Deno with full system permissions
- Integration tests require NATS server (automatically managed by make targets)
- Always test both Go engine and TypeScript runtime components
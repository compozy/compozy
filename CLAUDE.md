# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Compozy is a workflow orchestration engine for AI agents that enables building AI-powered applications through declarative YAML configuration and a robust Go backend. It integrates with various LLM providers and supports the Model Context Protocol (MCP) for extending AI capabilities.

## Development Commands

### Essential Commands

```bash
# Quick setup
make deps && make start-docker && make migrate-up

# Start development server with hot reload
make dev

# Run tests (excludes E2E/slow tests)
make test

# Run all tests including E2E
make test-all

# Format and lint code (ALWAYS run before committing)
make fmt && make lint

# Run specific test
go test -v ./engine/task -run TestExecutor_Execute
```

### Database Commands

```bash
make migrate-up     # Apply migrations
make migrate-down   # Rollback last migration
make migrate-status # Check migration status
make reset-db       # Reset database completely
```

## Architecture Overview

### Core Components

- **engine/**: Core business logic
    - **agent/**: AI agent management and configuration
    - **task/**: Task orchestration (basic, parallel, collection, router types)
    - **tool/**: Tool execution framework (TypeScript/Deno-based)
    - **workflow/**: Workflow definition and execution
    - **mcp/**: Model Context Protocol integration for external tool servers
    - **llm/**: LLM service integration (OpenAI, Groq, Ollama)
    - **runtime/**: Deno runtime for executing TypeScript tools
    - **worker/**: Temporal-based workflow execution
    - **infra/**: Infrastructure (server, database, cache, messaging)
- **cli/**: Command-line interface
- **pkg/**: Shared packages (mcp-proxy, utils, logger, tplengine)

### Infrastructure Stack

- **PostgreSQL**: Main database (5432) + Temporal database (5433)
- **Redis**: Caching, config storage, and pub/sub (6379)
- **Temporal**: Workflow orchestration (7233, UI: 8080)
- **MCP Proxy**: HTTP proxy for MCP servers (8081)
- **NATS**: Messaging system

## Critical Testing Standards

### MANDATORY Test Pattern

All tests MUST use the `t.Run("Should...")` pattern:

```go
// ✅ CORRECT
func TestTaskExecutor_Execute(t *testing.T) {
    t.Run("Should execute task successfully", func(t *testing.T) {
        // test implementation
    })
}

// ❌ WRONG - Never write tests without t.Run
```

### Table-Driven Tests

ONLY use table-driven tests when you have 5+ similar test cases. Avoid for just 2-3 cases.

## Code Standards

### Linting Requirements

- **Function length:** Max 80 lines or 50 statements
- **Line length:** Max 120 characters
- **Cyclomatic complexity:** Max 15
- **Error handling:** All errors must be checked

### Error Handling

```go
// Use custom errors
return core.NewError(err, "ERROR_CODE", map[string]any{"detail": value})

// Transaction pattern
defer func() {
    if err != nil { tx.Rollback(ctx) } else { tx.Commit(ctx) }
}()
```

### Key Patterns

1. **Declarative YAML Configuration**: All components use `$ref` for references and `$use` for dependency injection
2. **Tool Execution**: TypeScript files executed in Deno with JSON I/O
3. **MCP Integration**: External tool servers via HTTP/SSE transport
4. **Thread-Safe Structs**: Always embed mutex for concurrent access
5. **Factory Pattern**: Use for creating implementations based on config
6. **Graceful Shutdown**: Handle context cancellation and OS signals

### Common Libraries

- **Web:** `gin-gonic/gin`
- **DB:** `jackc/pgx/v5`, `redis/go-redis/v9`
- **Testing:** `stretchr/testify`
- **Logging:** `charmbracelet/log` (use logger.Info/Error/Debug)
- **Validation:** `go-playground/validator/v10`

## Working with MCP

The MCP integration is currently being developed. Key locations:

- `engine/mcp/`: MCP client implementation
- `pkg/mcp-proxy/`: HTTP proxy for MCP servers
- `engine/llm/proxy_tool.go`: Tool for proxying MCP calls

MCP servers are configured in YAML under the `mcps` section and the proxy runs on port 8081.

## Project Configuration

### compozy.yaml Structure

```yaml
name: project-name
version: 0.1.0
workflows:
    - source: ./workflow.yaml
models:
    - provider: ollama|openai|groq
      model: model-name
      api_key: "{{ .env.API_KEY }}"
runtime:
    permissions:
        - --allow-read
        - --allow-net
```

## Important Development Notes

1. **Before commits:** Always run `make fmt && make lint && make test`
2. **API changes:** Update Swagger annotations (`swag` comments)
3. **Schema changes:** Create migrations with `make migrate-create name=<name>`
4. **New features:** Include comprehensive tests following the mandatory pattern
5. **Logging:** Use `pkg/logger` for structured logging
6. **Core types:** Use `core.ID` for UUIDs, `core.Ref` for polymorphic references
7. **Backwards Compatibility:** NOT REQUIRED - Compozy is in development/alpha phase. Feel free to make breaking changes to focus on best architecture and code quality.

The project uses Go 1.24+ features and requires all external dependencies to be mocked in tests.

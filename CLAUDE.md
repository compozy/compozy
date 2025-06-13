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

## 🚨 CRITICAL: Code Formatting Standards

### ⚠️ MANDATORY LINE SPACING RULE - NEVER VIOLATE

**ABSOLUTELY CRITICAL:** Never add blank lines inside function bodies, code blocks, or any enclosed scope.

```go
// ✅ CORRECT - No blank lines inside blocks
t.Run("Should execute task successfully", func(t *testing.T) {
    proxyHandlers := &ProxyHandlers{
        globalAuthTokens: []string{},
    }
    result := combineAuthTokens(proxyHandlers.globalAuthTokens, []string{})
    assert.Empty(t, result)
})

func processWorkflow() error {
    workflow := loadWorkflow()
    validated := validate(workflow)
    return execute(validated)
}

// ❌ WRONG - Never add blank lines inside blocks
t.Run("Should handle errors", func(t *testing.T) {
    proxyHandlers := &ProxyHandlers{
        globalAuthTokens: nil,
    }

    result := combineAuthTokens(proxyHandlers.globalAuthTokens, nil)

    assert.Nil(t, result)
})
```

**Blank lines are ONLY allowed:**

- Between separate function definitions
- Between separate `t.Run()` test cases
- Between separate struct/interface definitions
- Between separate const/var blocks

**Blank lines are FORBIDDEN:**

- Inside function bodies (even with comments)
- Inside test cases (`t.Run` blocks)
- Inside struct definitions
- Inside if/for/switch/select blocks
- Inside method receivers

## Critical Testing Standards

### MANDATORY Test Pattern

All tests MUST use the `t.Run("Should...")` pattern. Use `testify/mock` only when necessary:

```go
// ✅ CORRECT - Simple test without mocks
func TestCalculateSum(t *testing.T) {
    t.Run("Should calculate sum correctly", func(t *testing.T) {
        result := CalculateSum(2, 3)
        assert.Equal(t, 5, result)
    })
}

// ✅ CORRECT - Test with mocks for external dependencies
func TestTaskExecutor_Execute(t *testing.T) {
    t.Run("Should execute task successfully", func(t *testing.T) {
        mockService := new(MockService)
        mockService.On("Process", mock.Anything).Return(nil)

        executor := NewTaskExecutor(mockService)
        err := executor.Execute(ctx)

        assert.NoError(t, err)
        mockService.AssertExpectations(t)
    })
}

// ❌ WRONG - Never write tests without t.Run or use custom mocks (use mocks only when necessary)
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
- **Testing:** `stretchr/testify` (assertions + mocks)
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
5. **Taskmaster tasks:** Follow mandatory code review workflow via MCP tools (`mcp_zen_codereview` with Gemini 2.5 Pro and O3 models + `mcp_zen_precommit` + structured commits)
6. **Logging:** Use `pkg/logger` for structured logging
7. **Core types:** Use `core.ID` for UUIDs, `core.Ref` for polymorphic references
8. **Backwards Compatibility:** NOT REQUIRED - Compozy is in development/alpha phase. Feel free to make breaking changes to focus on best architecture and code quality.

The project uses Go 1.24+ features and requires all external dependencies to be mocked in tests.

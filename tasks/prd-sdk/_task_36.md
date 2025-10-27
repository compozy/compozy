## status: completed

<task_context>
<domain>sdk/mcp</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>sdk/internal/errors, engine/mcp, pkg/mcp-proxy</dependencies>
</task_context>

# Task 36.0: MCP: Command/URL Basics (S)

## Overview

Implement the MCP builder base with command-based (stdio) and URL-based (SSE/HTTP) configuration.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md and tasks/prd-sdk/03-sdk-entities.md
</critical>

<requirements>
- MCP builder with command and URL support
- Context-first Build(ctx) pattern
- Error accumulation following BuildError pattern
- Produces engine/mcp.Config
</requirements>

## Subtasks

- [x] 36.1 Create sdk/mcp/builder.go with Builder struct
- [x] 36.2 Implement New(id string) constructor
- [x] 36.3 Implement WithCommand(command string, args ...string) method
- [x] 36.4 Implement WithURL(url string) method
- [x] 36.5 Implement Build(ctx) with validation
- [x] 36.6 Add unit tests for MCP builder basics

## Implementation Details

Reference from 03-sdk-entities.md section 8:

```go
type Builder struct {
    config *mcp.Config
    errors []error
}

func New(id string) *Builder

// Command-based MCP (stdio transport)
func (b *Builder) WithCommand(command string, args ...string) *Builder

// URL-based MCP (SSE/HTTP transport)
func (b *Builder) WithURL(url string) *Builder

func (b *Builder) Build(ctx context.Context) (*mcp.Config, error)
```

Examples from architecture:
```go
// Command-based (stdio)
mcp.New("filesystem").
    WithCommand("mcp-server-filesystem")

// URL-based (SSE/HTTP)
mcp.New("github-api").
    WithURL("https://api.github.com/mcp/v1")
```

### Relevant Files

- `sdk/mcp/builder.go` (new)
- `sdk/internal/errors/build_error.go` (existing)
- `engine/mcp/types.go` (engine types)
- `pkg/mcp-proxy/types.go` (transport types)

### Dependent Files

- Tasks 37-39 will extend this builder
- Future MCP integration examples

## Deliverables

- `sdk/mcp/builder.go` implementing MCP Builder
- Constructor and command/URL configuration methods
- Build(ctx) producing engine mcp.Config
- Package documentation

## Tests

Unit tests mapped from `_tests.md`:

- [x] New validates non-empty MCP ID
- [x] WithCommand stores command and args correctly
- [x] WithURL stores URL correctly
- [x] Build(ctx) validates either command or URL (mutually exclusive)
- [x] Error cases: empty command, empty URL, both set
- [x] Edge cases: command with no args, URL with query params

## Success Criteria

- MCP builder follows context-first pattern
- All unit tests pass
- make lint and make test pass
- Ready for transport/headers/env extensions

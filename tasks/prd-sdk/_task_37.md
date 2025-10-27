## status: completed

<task_context>
<domain>sdk/mcp</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>sdk/mcp/builder.go, pkg/mcp-proxy</dependencies>
</task_context>

# Task 37.0: MCP: Transport (stdio/SSE) (S)

## Overview

Extend MCP builder with transport type configuration (stdio, SSE, HTTP).

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md and tasks/prd-sdk/03-sdk-entities.md
</critical>

<requirements>
- Transport type configuration method
- Support stdio, SSE, HTTP transports
- Validation for transport compatibility with command/URL
- Context-first validation
</requirements>

## Subtasks

- [x] 37.1 Add WithTransport(transport mcpproxy.TransportType) method
- [x] 37.2 Update Build(ctx) validation for transport compatibility
- [x] 37.3 Add unit tests for transport configuration

## Implementation Details

Reference from 03-sdk-entities.md section 8:

```go
// Transport configuration
func (b *Builder) WithTransport(transport mcpproxy.TransportType) *Builder
```

Transport types from pkg/mcp-proxy:
- TransportStdio (default for command-based)
- TransportSSE (default for URL-based)
- TransportHTTP (alternative for URL-based)

Validation rules:
- stdio transport requires command (not URL)
- SSE/HTTP transports require URL (not command)

Example from architecture:
```go
mcp.New("github-api").
    WithURL("https://api.github.com/mcp/v1").
    WithTransport(mcpproxy.TransportSSE)
```

### Relevant Files

- `sdk/mcp/builder.go` (extend existing)
- `pkg/mcp-proxy/types.go` (transport types)

### Dependent Files

- Task 36.0 output (MCP builder base)
- Future MCP examples

## Deliverables

- WithTransport method in MCP Builder
- Validation for transport/command/URL compatibility
- Updated package documentation

## Tests

Unit tests mapped from `_tests.md`:

- [ ] WithTransport sets transport type correctly
- [ ] Build(ctx) validates stdio transport requires command
- [ ] Build(ctx) validates SSE/HTTP transports require URL
- [ ] Error cases: stdio with URL, SSE with command
- [ ] Edge cases: default transport inference from command/URL

## Success Criteria

- Transport configuration follows builder pattern
- All unit tests pass
- make lint and make test pass
- Ready for headers/env extensions

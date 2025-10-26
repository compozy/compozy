## status: pending

<task_context>
<domain>sdk/mcp</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>sdk/mcp/builder.go, engine/mcp</dependencies>
</task_context>

# Task 39.0: MCP: Proto + Sessions (S)

## Overview

Extend MCP builder with protocol version and session management configuration.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md and tasks/prd-sdk/03-sdk-entities.md
</critical>

<requirements>
- Protocol version configuration
- Max sessions configuration
- Context-first validation
- Complete MCP builder feature set
</requirements>

## Subtasks

- [ ] 39.1 Add WithProto(version string) method
- [ ] 39.2 Add WithMaxSessions(max int) method
- [ ] 39.3 Update Build(ctx) validation for proto/sessions
- [ ] 39.4 Add unit tests for proto and sessions

## Implementation Details

Reference from 03-sdk-entities.md section 8:

```go
// Protocol version
func (b *Builder) WithProto(version string) *Builder

// Session management
func (b *Builder) WithMaxSessions(max int) *Builder
```

Protocol versions:
- "2025-03-26" (current MCP protocol version)
- Validation: must be valid date format or version string

Max sessions:
- Applies to both command and URL-based MCPs
- Default: unlimited (0)
- Validation: >= 0

Example from architecture:
```go
mcp.New("github-api").
    WithURL("https://api.github.com/mcp/v1").
    WithProto("2025-03-26").
    WithMaxSessions(10)
```

### Relevant Files

- `sdk/mcp/builder.go` (extend existing)
- `engine/mcp/types.go` (config structures)

### Dependent Files

- Tasks 36-38 output (MCP builder base + extensions)
- Future MCP integration examples

## Deliverables

- Protocol version method in MCP Builder
- Max sessions method in MCP Builder
- Validation for proto format and session limits
- Updated package documentation
- Complete MCP builder ready for use

## Tests

Unit tests mapped from `_tests.md`:

- [ ] WithProto sets protocol version correctly
- [ ] WithMaxSessions sets session limit
- [ ] Build(ctx) validates proto format
- [ ] Build(ctx) validates max sessions >= 0
- [ ] Error cases: invalid proto format, negative max sessions
- [ ] Edge cases: zero max sessions (unlimited)
- [ ] Complete MCP config validation with all features

## Success Criteria

- Proto and sessions methods follow builder pattern
- All unit tests pass
- make lint and make test pass
- MCP builder feature-complete per SDK entities spec

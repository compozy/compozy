## status: pending

<task_context>
<domain>sdk/mcp</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>sdk/mcp/builder.go, engine/mcp</dependencies>
</task_context>

# Task 38.0: MCP: Headers/Env/Timeouts (S)

## Overview

Extend MCP builder with HTTP headers, environment variables, and timeout configuration.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md and tasks/prd-sdk/03-sdk-entities.md
</critical>

<requirements>
- HTTP headers for URL-based MCPs
- Environment variables for command-based MCPs
- Start timeout configuration
- Context-first validation
</requirements>

## Subtasks

- [ ] 38.1 Add WithHeaders(headers map[string]string) method
- [ ] 38.2 Add WithHeader(key, value string) method
- [ ] 38.3 Add WithEnv(env map[string]string) method
- [ ] 38.4 Add WithEnvVar(key, value string) method
- [ ] 38.5 Add WithStartTimeout(timeout time.Duration) method
- [ ] 38.6 Update Build(ctx) validation for headers/env usage
- [ ] 38.7 Add unit tests for headers, env, and timeouts

## Implementation Details

Reference from 03-sdk-entities.md section 8:

```go
// HTTP headers (for URL-based MCPs)
func (b *Builder) WithHeaders(headers map[string]string) *Builder
func (b *Builder) WithHeader(key, value string) *Builder

// Process configuration (for command-based MCPs)
func (b *Builder) WithEnv(env map[string]string) *Builder
func (b *Builder) WithEnvVar(key, value string) *Builder
func (b *Builder) WithStartTimeout(timeout time.Duration) *Builder
```

Validation rules:
- Headers only apply to URL-based MCPs
- Env vars only apply to command-based MCPs
- Timeout applies to command-based MCPs

Examples from architecture:
```go
// URL-based with headers
mcp.New("github-api").
    WithURL("https://api.github.com/mcp/v1").
    WithHeader("Authorization", "Bearer {{.env.GITHUB_TOKEN}}")

// Command-based with env
mcp.New("filesystem").
    WithCommand("mcp-server-filesystem").
    WithEnvVar("ROOT_DIR", "/data").
    WithStartTimeout(10 * time.Second)
```

### Relevant Files

- `sdk/mcp/builder.go` (extend existing)
- `engine/mcp/types.go` (config structures)

### Dependent Files

- Task 36.0 and 37.0 output (MCP builder base)
- Future MCP examples

## Deliverables

- Headers configuration methods in MCP Builder
- Environment variables methods in MCP Builder
- Timeout configuration method in MCP Builder
- Validation for appropriate usage context
- Updated package documentation

## Tests

Unit tests mapped from `_tests.md`:

- [ ] WithHeaders sets headers map correctly
- [ ] WithHeader adds individual headers
- [ ] WithEnv sets environment variables map
- [ ] WithEnvVar adds individual env vars
- [ ] WithStartTimeout sets timeout duration
- [ ] Build(ctx) validates headers only with URL
- [ ] Build(ctx) validates env only with command
- [ ] Error cases: headers with command, env with URL
- [ ] Edge cases: template vars in headers/env values

## Success Criteria

- Headers, env, and timeout methods follow builder pattern
- All unit tests pass
- make lint and make test pass
- Ready for protocol/sessions extensions

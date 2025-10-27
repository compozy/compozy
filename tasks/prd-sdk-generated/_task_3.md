## status: pending

<task_context>
<domain>sdk/mcp</domain>
<type>implementation</type>
<scope>code_generation</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 3.0: Migrate mcp Package to Functional Options

## Overview

Migrate the `sdk/mcp` package (Model Context Protocol) from builder pattern to functional options. MCP configs enable AI agents to connect to external services through standardized protocols.

**Estimated Time:** 1 hour

<critical>
- **GREENFIELD APPROACH:** Build fresh in sdk2/, keep sdk/ for reference
- **SIMPLE VALIDATION:** Mainly ID and transport type validation
</critical>

<requirements>
- Generate options from engine/mcp/config.go
- Validate MCP ID (required, non-empty)
- Validate transport type (stdio or http)
- Validate command for stdio transport
- Validate URL for http transport
- Deep copy and comprehensive tests
</requirements>

## Subtasks

- [ ] 3.1 Create sdk2/mcp/ directory structure
- [ ] 3.2 Create generate.go
- [ ] 3.3 Generate options
- [ ] 3.4 Create constructor with validation
- [ ] 3.5 Create comprehensive tests
- [ ] 3.6 Verify linter and tests
- [ ] 3.7 Create README.md

## Implementation Details

### Engine Fields (~6 fields)
- ID (string)
- Transport (string: "stdio" or "http")
- Command (string, required for stdio)
- Args ([]string)
- Env (map[string]string)
- URL (string, required for http)

### Key Validation
- Transport must be "stdio" or "http"
- If stdio: command required
- If http: URL required and valid

### Relevant Files

**Reference (for understanding):**
- `sdk/mcp/builder.go` - Old builder pattern to understand requirements
- `sdk/mcp/builder_test.go` - Old tests to understand test cases
- `engine/mcp/config.go` - Source struct for generation

**To Create in sdk2/mcp/:**
- `generate.go` - go:generate directive
- `options_generated.go` - Auto-generated
- `constructor.go` - Validation logic (~50 lines)
- `constructor_test.go` - Test suite
- `README.md` - API documentation

**Note:** Do NOT delete or modify anything in `sdk/mcp/` - keep for reference during transition

## Tests
- [ ] Valid stdio MCP with command
- [ ] Valid http MCP with URL
- [ ] Invalid transport type fails
- [ ] Stdio without command fails
- [ ] HTTP without URL fails
- [ ] Invalid URL format fails

## Success Criteria
- [ ] sdk2/mcp/ directory created with proper structure
- [ ] Transport validation enforced
- [ ] Tests pass: `gotestsum -- ./sdk2/mcp`
- [ ] Linter clean: `golangci-lint run ./sdk2/mcp/...`
- [ ] Reduction: ~117 LOC → ~50 LOC (57% reduction)

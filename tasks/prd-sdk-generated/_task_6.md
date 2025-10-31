## status: completed

<task_context>
<domain>sdk/tool</domain>
<type>implementation</type>
<scope>code_generation</scope>
<complexity>medium</complexity>
<dependencies>sdk/model</dependencies>
</task_context>

# Task 6.0: Migrate tool Package to Functional Options

## Overview

Migrate `sdk/tool` for agent tool configurations. Tools extend agent capabilities with file operations, API calls, data processing, and custom business logic.

**Estimated Time:** 2-3 hours

**Dependency:** Requires Task 1.0 (model) complete

<critical>
- **SCHEMA DEPENDENCY:** Tools use JSON Schema for input/output validation
- **COMPLEX VALIDATION:** Must validate tool types, handler paths, and schemas
</critical>

<requirements>
- Generate options from engine/tool/config.go
- Validate tool type (native, http, mcp)
- Validate handler/endpoint based on type
- Validate input/output schemas if provided
- Handle tool collections and dependencies
- Deep copy and comprehensive tests
</requirements>

## Subtasks

- [x] 6.1 Create sdk/tool/ directory structure
- [x] 6.2 Create generate.go
- [x] 6.3 Generate options (~13 fields)
- [x] 6.4 Constructor with type-specific validation
- [x] 6.5 Schema validation integration
- [x] 6.6 Tests for all tool types
- [x] 6.7 Verify and document

## Implementation Details

### Engine Fields (~8 fields)
- ID, Type (native/http/mcp), Handler, Endpoint, InputSchema, OutputSchema, Timeout, Retry

### Type-Specific Validation
```go
switch cfg.Type {
case "native":
    // Validate handler path exists
case "http":
    // Validate endpoint URL format
case "mcp":
    // Validate MCP server reference
}
```

### Schema Integration
- InputSchema: *schema.Schema (optional)
- OutputSchema: *schema.Schema (optional)
- Must validate schema structure if provided

### Relevant Files

**Reference (for understanding):**
- `sdk/tool/builder.go` - Old builder pattern to understand requirements
- `sdk/tool/builder_test.go` - Old tests to understand test cases
- `engine/tool/config.go` - Source struct for generation

**To Create in sdk/tool/:**
- `generate.go` - go:generate directive
- `options_generated.go` - Auto-generated
- `constructor.go` - Validation logic
- `constructor_test.go` - Test suite
- `README.md` - API documentation

**Note:** Do NOT delete or modify anything in `sdk/tool/` - keep for reference during transition

## Tests

- [x] Minimal tool configuration
- [x] Full tool configuration with all options
- [x] Tool with input schema
- [x] Tool with output schema
- [x] Empty ID validation fails
- [x] Invalid ID validation fails
- [x] Empty name/description/runtime/code fails
- [x] Invalid runtime fails (must be bun)
- [x] Invalid timeout format fails
- [x] Negative/zero timeout fails
- [x] Timeout parsing for various formats
- [x] Runtime case-insensitive normalization
- [x] Whitespace trimming
- [x] Deep copy verification
- [x] Nil context handling
- [x] Multiple validation errors collected

## Success Criteria

- [x] sdk/tool/ directory created
- [x] Type-based validation logic complete
- [x] Schema integration working
- [x] All tool types tested
- [x] Tests pass: `gotestsum -- ./sdk/tool` (40 tests, all passing)
- [x] Linter clean: `golangci-lint run ./sdk/tool/...` (0 issues)
- [x] Generated 13 option functions (Resource, ID, Name, Description, Runtime, Code, Timeout, InputSchema, OutputSchema, With, Config, Env, CWD)
- [x] Comprehensive validation: ID, Name, Description, Runtime, Code, Timeout
- [x] README.md with API documentation and migration guide

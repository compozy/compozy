## status: pending

<task_context>
<domain>sdk/tool</domain>
<type>implementation</type>
<scope>code_generation</scope>
<complexity>medium</complexity>
<dependencies>sdk2/model</dependencies>
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

- [ ] 6.1 Create sdk2/tool/ directory structure
- [ ] 6.2 Create generate.go
- [ ] 6.3 Generate options (~8 fields)
- [ ] 6.4 Constructor with type-specific validation
- [ ] 6.5 Schema validation integration
- [ ] 6.6 Tests for all tool types
- [ ] 6.7 Verify and document

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

**To Create in sdk2/tool/:**
- `generate.go` - go:generate directive
- `options_generated.go` - Auto-generated
- `constructor.go` - Validation logic
- `constructor_test.go` - Test suite
- `README.md` - API documentation

**Note:** Do NOT delete or modify anything in `sdk/tool/` - keep for reference during transition

## Tests

- [ ] Native tool with handler
- [ ] HTTP tool with endpoint
- [ ] MCP tool with server ref
- [ ] Tool with input schema
- [ ] Tool with output schema
- [ ] Invalid type fails
- [ ] Native without handler fails
- [ ] HTTP with invalid URL fails
- [ ] Invalid schema structure fails
- [ ] Timeout parsing
- [ ] Retry policy configuration

## Success Criteria

- [ ] sdk2/tool/ directory created
- [ ] Type-based validation logic complete
- [ ] Schema integration working
- [ ] All tool types tested
- [ ] Tests pass: `gotestsum -- ./sdk2/tool`
- [ ] Linter clean: `golangci-lint run ./sdk2/tool/...`
- [ ] Reduction: ~239 LOC → ~90 LOC (62% reduction)

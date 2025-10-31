## status: completed

<task_context>
<domain>sdk/memory</domain>
<type>implementation</type>
<scope>code_generation</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 5.0: Migrate memory Package to Functional Options

## Overview

Migrate `sdk/memory` for persistent agent memory storage. Memory configs define storage backends (Redis, PostgreSQL, etc.) and retention policies.

**Estimated Time:** 1 hour

<requirements>
- Generate options from engine/memory/config.go
- Validate backend type (redis, postgres, memory)
- Validate connection strings
- Validate retention settings
- Deep copy and tests
</requirements>

## Subtasks

- [x] 5.1 Create sdk/memory/ directory structure
- [x] 5.2 Create generate.go
- [x] 5.3 Generate options (~18 fields)
- [x] 5.4 Constructor with backend validation
- [x] 5.5 Tests for different backends
- [x] 5.6 Verify clean

## Implementation Details

### Engine Fields
- ID, Backend (redis/postgres/memory), ConnectionString, Retention, MaxSize

### Validation
- Backend enum validation
- Connection string format for redis/postgres
- Retention duration parsing
- MaxSize positive integer

### Relevant Files

**Reference (for understanding):**
- `sdk/memory/builder.go` - Old builder pattern to understand requirements
- `sdk/memory/builder_test.go` - Old tests to understand test cases
- `engine/memory/config.go` - Source struct for generation

**To Create in sdk/memory/:**
- `generate.go` - go:generate directive
- `options_generated.go` - Auto-generated
- `constructor.go` - Validation logic
- `constructor_test.go` - Test suite
- `README.md` - API documentation

**Note:** Do NOT delete or modify anything in `sdk/memory/` - keep for reference during transition

## Tests
- [x] Valid redis memory
- [x] Valid in-memory backend
- [x] In-memory backend
- [x] Invalid type fails
- [x] Invalid persistence fails
- [x] Negative limits fail

## Success Criteria
- [x] sdk/memory/ directory created
- [x] Backend validation complete
- [x] Persistence type and TTL validation
- [x] Tests pass: `gotestsum -- ./sdk/memory` (19 tests, all pass)
- [x] Linter clean: `golangci-lint run ./sdk/memory/...` (0 issues)
- [x] ~727 LOC total (4 files: generate.go, options_generated.go, constructor.go, constructor_test.go)

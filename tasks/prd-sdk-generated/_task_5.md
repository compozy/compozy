## status: pending

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

- [ ] 5.1 Create sdk2/memory/ directory structure
- [ ] 5.2 Create generate.go
- [ ] 5.3 Generate options (~5 fields)
- [ ] 5.4 Constructor with backend validation
- [ ] 5.5 Tests for different backends
- [ ] 5.6 Verify clean

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

**To Create in sdk2/memory/:**
- `generate.go` - go:generate directive
- `options_generated.go` - Auto-generated
- `constructor.go` - Validation logic
- `constructor_test.go` - Test suite
- `README.md` - API documentation

**Note:** Do NOT delete or modify anything in `sdk/memory/` - keep for reference during transition

## Tests
- [ ] Valid redis memory
- [ ] Valid postgres memory
- [ ] In-memory backend
- [ ] Invalid backend fails
- [ ] Invalid connection string fails
- [ ] Negative retention fails

## Success Criteria
- [ ] sdk2/memory/ directory created
- [ ] Backend validation complete
- [ ] Connection string parsing
- [ ] Tests pass: `gotestsum -- ./sdk2/memory`
- [ ] Linter clean: `golangci-lint run ./sdk2/memory/...`
- [ ] ~200 LOC → ~60 LOC (70% reduction)

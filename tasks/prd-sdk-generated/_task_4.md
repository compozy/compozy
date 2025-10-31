## status: completed

<task_context>
<domain>sdk/runtime</domain>
<type>implementation</type>
<scope>code_generation</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 4.0: Migrate runtime Package to Functional Options

## Overview

Migrate `sdk/runtime` for native function execution in workflows. Runtime configs specify JavaScript/Python/Go execution environments.

**Estimated Time:** 1 hour

<requirements>
- Generate options from engine/runtime/config.go
- Validate runtime type enum (javascript, python, go)
- Validate handler path exists
- Deep copy and tests
</requirements>

## Subtasks

- [x] 4.1 Create sdk/runtime/ directory structure
- [x] 4.2 Create generate.go
- [x] 4.3 Generate options (~13 fields)
- [x] 4.4 Constructor with runtime validation
- [x] 4.5 Tests covering runtime types
- [x] 4.6 Verify clean
- [x] 4.7 Create README.md

## Implementation Details

### Engine Fields
- ID, Type (javascript/python/go), Handler, Env, Timeout

### Validation
- Type must be valid enum
- Handler path required

### Relevant Files

**Reference (for understanding):**
- `sdk/runtime/builder.go` - Old builder pattern to understand requirements
- `sdk/runtime/builder_test.go` - Old tests to understand test cases
- `engine/runtime/config.go` - Source struct for generation

**To Create in sdk/runtime/:**
- `generate.go` - go:generate directive
- `options_generated.go` - Auto-generated
- `constructor.go` - Validation logic (~55 lines)
- `constructor_test.go` - Test suite
- `README.md` - API documentation

**Note:** Do NOT delete or modify anything in `sdk/runtime/` - keep for reference during transition

## Tests
- [x] Valid bun runtime
- [x] Valid node runtime
- [x] Invalid type fails
- [x] Deep copy behavior verified

## Success Criteria
- [x] sdk/runtime/ directory created with proper structure
- [x] Runtime enum validated
- [x] Tests pass: `gotestsum -- ./sdk/runtime` (23 tests, 71.7% coverage)
- [x] Linter clean: `golangci-lint run ./sdk/runtime/...` (0 issues)
- [x] 654 LOC → 180 LOC (72% reduction, exceeded target!)

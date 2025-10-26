## status: completed

<task_context>
<domain>sdk/internal/validate</domain>
<type>implementation</type>
<scope>core_infrastructure</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 03.0: Validation Helpers (S)

## Overview

Create validation helper functions used across all builders for common validation patterns: required fields, ID format, non-empty strings, URL validation, etc.

<critical>
- **ALWAYS READ** .cursor/rules/go-coding-standards.mdc
- **MUST** use context.Context for all validation functions
- **MUST** return clear, actionable error messages
- **NEVER** use panic or global state
</critical>

<requirements>
- Create validation helpers for required fields
- Validate ID formats (alphanumeric + hyphens)
- Validate non-empty strings
- Validate URL formats
- Validate time durations
- Validate numeric ranges
- All validators accept context.Context
</requirements>

## Subtasks

- [x] 03.1 Create sdk/internal/validate/validate.go
- [x] 03.2 Implement ValidateRequired(ctx, name, value) error
- [x] 03.3 Implement ValidateID(ctx, id) error
- [x] 03.4 Implement ValidateNonEmpty(ctx, name, value) error
- [x] 03.5 Implement ValidateURL(ctx, url) error
- [x] 03.6 Implement ValidateDuration(ctx, d) error
- [x] 03.7 Implement ValidateRange(ctx, name, val, min, max) error
- [x] 03.8 Add comprehensive unit tests

## Implementation Details

Reference: tasks/prd-sdk/02-architecture.md (Context-First Architecture)

### Validation Functions

```go
// sdk/internal/validate/validate.go
package validate

import "context"

// ValidateRequired checks if a required field is set
func ValidateRequired(ctx context.Context, name string, value interface{}) error

// ValidateID checks if an ID follows the required format (alphanumeric + hyphens)
func ValidateID(ctx context.Context, id string) error

// ValidateNonEmpty checks if a string is not empty
func ValidateNonEmpty(ctx context.Context, name, value string) error

// ValidateURL checks if a URL is valid
func ValidateURL(ctx context.Context, url string) error

// ValidateDuration checks if a duration is positive
func ValidateDuration(ctx context.Context, d time.Duration) error

// ValidateRange checks if a value is within a range
func ValidateRange(ctx context.Context, name string, val, min, max int) error
```

### Relevant Files

- `sdk/internal/validate/validate.go` (NEW)
- `sdk/internal/validate/validate_test.go` (NEW)

### Dependent Files

- All builder packages will import this

## Deliverables

- ✅ `sdk/internal/validate/validate.go` with all helper functions
- ✅ All validators accept context.Context as first parameter
- ✅ Clear, actionable error messages for each validator
- ✅ Unit tests with 95%+ coverage
- ✅ Examples in tests showing usage patterns

## Tests

Reference: tasks/prd-sdk/_tests.md

- Unit tests for validation helpers:
  - [x] Test ValidateRequired with nil, empty, and valid values
  - [x] Test ValidateID with valid and invalid IDs
  - [x] Test ValidateNonEmpty with empty and non-empty strings
  - [x] Test ValidateURL with valid and invalid URLs
  - [x] Test ValidateDuration with negative, zero, and positive durations
  - [x] Test ValidateRange with values inside and outside range
  - [x] Test error messages are clear and actionable
  - [x] Test all functions accept context.Context

## Success Criteria

- All validation functions accept context.Context
- Error messages clearly indicate what failed and why
- ID validation matches pattern: alphanumeric + hyphens
- URL validation uses standard library
- All tests pass with 95%+ coverage
- Functions are reusable across all builders

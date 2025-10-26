## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>v2/docs</domain>
<type>documentation</type>
<scope>troubleshooting</scope>
<complexity>low</complexity>
<dependencies>task_53,task_54</dependencies>
</task_context>

# Task 55.0: Troubleshooting Guide (S)

## Overview

Create comprehensive troubleshooting guide covering common errors, debugging patterns, validation issues, and resolution steps for SDK users.

<critical>
- **ALWAYS READ** tasks/prd-modules/06-migration-guide.md (troubleshooting section)
- **ALWAYS READ** tasks/prd-modules/07-testing-strategy.md (validation patterns)
- **MUST** organize by error type (compile, runtime, validation, integration)
- **MUST** include resolution steps with example fixes
</critical>

<requirements>
- Cover all common error categories
- Provide diagnostic commands (go test, go vet, linting)
- Include debugging techniques (logging, Build error inspection)
- Link to relevant SDK documentation sections
- Provide real error message examples with fixes
</requirements>

## Subtasks

- [ ] 55.1 Document compilation/import errors and fixes
- [ ] 55.2 Document validation errors (BuildError handling)
- [ ] 55.3 Document context-related errors (missing logger/config)
- [ ] 55.4 Document integration errors (DB, MCP, external services)
- [ ] 55.5 Document template expression errors
- [ ] 55.6 Create diagnostic commands reference
- [ ] 55.7 Add debugging best practices section

## Implementation Details

**Based on:** tasks/prd-modules/06-migration-guide.md (troubleshooting), tasks/prd-modules/07-testing-strategy.md

### Content Structure

```markdown
# SDK Troubleshooting Guide

## Quick Diagnostic Commands
- `go mod tidy` → Fix dependencies
- `go vet ./v2/...` → Static analysis
- `golangci-lint run ./v2/...` → Comprehensive checks
- Build error inspection → err.Error() details

## Error Categories

### 1. Compilation Errors
**Issue:** Import errors, package not found
**Symptoms:** [Real error messages]
**Resolution:** [Step-by-step fixes]
**Example:** [Code before/after]

### 2. Validation Errors
**Issue:** Required fields, value ranges, reference resolution
**Symptoms:** BuildError with accumulated errors
**Resolution:** Inspect err.Error() for all validation issues
**Example:** [Validation error handling pattern]

### 3. Context Errors
**Issue:** Logger/config not in context
**Symptoms:** Nil pointer or missing context values
**Resolution:** Use logger.WithLogger and config.WithConfig
**Example:** [Context setup pattern]

### 4. Integration Errors
**Issue:** Database connection, MCP transport, external services
**Symptoms:** Connection refused, timeout, auth failures
**Resolution:** [Service-specific debugging]
**Example:** [Connection testing patterns]

### 5. Template Errors
**Issue:** Template syntax, undefined variables
**Symptoms:** Template parsing errors
**Resolution:** Verify template syntax and variable paths
**Example:** [Template debugging]

## Debugging Techniques
- Enable verbose logging
- Use BuildError.Errors() for all validation issues
- Test builders in isolation
- Use integration tests for external dependencies

## Common Patterns
- Always check Build(ctx) errors
- Use t.Context() in tests (not context.Background)
- Verify imports are from v2/ packages
- Check go.work includes both modules
```

### Relevant Files

- tasks/prd-modules/06-migration-guide.md (troubleshooting section)
- tasks/prd-modules/07-testing-strategy.md (testing patterns)
- tasks/prd-modules/02-architecture.md (context-first, BuildError)

### Dependent Files

- Task 53.0 deliverable (link to context setup)
- Task 54.0 deliverable (link to advanced patterns)

## Deliverables

- `/Users/pedronauck/Dev/compozy/compozy/v2/docs/troubleshooting.md` (new file)
  - Quick diagnostic commands section
  - 5+ error categories with real error messages and fixes
  - Debugging techniques section
  - Common patterns/best practices
  - Cross-references to migration guides
- Include at least 15 distinct error scenarios
- Each error must have: symptoms, cause, resolution, example

## Tests

Documentation validation:
- [ ] All diagnostic commands are correct and tested
- [ ] Error messages are realistic (from actual SDK usage)
- [ ] Resolution steps are complete and actionable
- [ ] Example fixes compile and resolve the stated issue
- [ ] Links to other documentation work
- [ ] Context error section references proper setup pattern

Manual verification:
- [ ] Run all diagnostic commands and verify output
- [ ] Test resolution steps for top 5 errors
- [ ] Verify BuildError handling example works
- [ ] Check integration error debugging steps

## Success Criteria

- User can self-diagnose 80% of common errors
- Every error type has clear resolution path
- Diagnostic commands provide actionable output
- Debugging techniques are practical and effective
- Document reduces support burden by covering FAQ issues
- Links connect users to relevant deep-dive documentation

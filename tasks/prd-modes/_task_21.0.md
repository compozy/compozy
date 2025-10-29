## status: pending

<task_context>
<domain>tooling</domain>
<type>code_generation</type>
<scope>metadata</scope>
<complexity>low</complexity>
<dependencies>schemas</dependencies>
</task_context>

# Task 21.0: Regenerate Generated Files

## Overview

Regenerate auto-generated files (Swagger docs, golden test files, schema-generated code) to reflect the new three-mode system.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from `_techspec.md` Phase 5.2 before start
- **DEPENDENCIES:** This task depends on Task 20.0 (Update JSON Schemas) being completed
</critical>

<research>
# When you need information about code generation:
- use perplexity to find out how Swagger generation works in Go projects
- check project Makefile for generation commands
</research>

<requirements>
- Regenerate Swagger/OpenAPI documentation
- Regenerate golden test files with new mode names
- Regenerate any schema-generated code
- Verify all generated files are consistent with new mode system
</requirements>

## Subtasks

- [ ] 21.1 Regenerate Swagger documentation (`make swagger`)
- [ ] 21.2 Regenerate golden test files (`UPDATE_GOLDEN=1`)
- [ ] 21.3 Regenerate schema-generated code (if applicable)
- [ ] 21.4 Verify all generated files are correct

## Implementation Details

See `_techspec.md` Phase 5.2 for complete implementation details.

### Generation Commands

**Swagger docs:**
```bash
make swagger
```

**Golden files:**
```bash
UPDATE_GOLDEN=1 go test ./cli/cmd/config/...
```

**Schema-generated code (if applicable):**
```bash
go run pkg/schemagen/main.go  # If this exists
```

### Relevant Files

**Generated files to update:**
- Swagger/OpenAPI documentation
- `testdata/config-diagnostics-standalone.golden` → update to memory
- `testdata/config-show-mixed.golden` → update mode references
- `testdata/config-show-standalone.golden` → update to memory
- Any schema-generated code files

### Dependent Files

- `schemas/config.json` (Task 20.0)
- `schemas/compozy.json` (Task 20.0)
- `pkg/config/config.go`
- `pkg/config/resolver.go`

## Deliverables

- Regenerated Swagger documentation with new modes
- Updated golden test files with memory/persistent/distributed
- Regenerated schema code (if applicable)
- All generated files pass validation

## Tests

- Generated file validation:
  - [ ] Swagger docs contain correct mode enums
  - [ ] Golden files contain updated mode names
  - [ ] Golden test comparisons pass
  - [ ] No "standalone" references in generated files
  - [ ] All config tests pass with updated golden files

## Success Criteria

- All auto-generated files reflect new mode system
- Golden test files updated and passing
- Swagger documentation shows correct modes
- No "standalone" references in generated files
- All generation commands execute successfully

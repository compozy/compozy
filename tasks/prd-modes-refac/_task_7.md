## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>examples|schemas</domain>
<type>documentation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>config</dependencies>
</task_context>

# Task 7.0: Examples & Schema Regeneration

## Overview

This task updates example configuration files to use the correct modes (memory/persistent/distributed) and regenerates JSON schemas using the make schemagen command. Schema files are auto-generated and should not be manually edited.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the _techspec.md and the _prd.md docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

<research>
# When you need information about a library or external API:
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase, the same for the topic param of context7
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
- Search for all example YAML files with `mode: standalone`
- Update example configurations to use memory/persistent/distributed
- Regenerate schemas using `make schemagen` (do not manually edit schema files)
- Verify schema generation completes successfully
- Verify examples are valid YAML
</requirements>

## Subtasks

- [x] 7.1 Search for example files: `grep -r "mode: standalone" examples/`
- [x] 7.2 Update example YAML files to use `mode: memory` or `mode: persistent`
- [x] 7.3 Update example YAML files to use `mode: distributed` where appropriate
- [x] 7.4 Verify all example files are valid YAML
- [x] 7.5 Run `make schemagen` to regenerate JSON schemas
- [x] 7.6 Verify schema generation completes successfully
- [x] 7.7 Verify generated schemas are valid JSON
- [x] 7.8 Check that schema changes reflect config struct renames from Task 1.0

## Implementation Details

See Phase 5.3 in the techspec for detailed implementation steps.

Key changes:
- Search for `mode: standalone` in example files
- Replace with appropriate mode (`memory`, `persistent`, or `distributed`)
- Run `make schemagen` to regenerate schemas from updated config structs
- Do not manually edit schema files

### Relevant Files

- `examples/**/*.yaml` - Example configuration files
- `schemas/*.json` - Auto-generated schema files (do not edit manually)

### Dependent Files

- `pkg/config/config.go` - Source of truth for schema generation
- `pkg/schemagen/` - Schema generation tooling

## Deliverables

- All example YAML files updated to use correct modes
- All examples are valid YAML
- Schemas regenerated successfully via `make schemagen`
- Generated schemas reflect config struct changes
- No manual edits to schema files
- Examples demonstrate correct mode usage

## Tests

- Validation tests:
  - [x] Verify all example YAML files parse correctly
  - [x] Verify example configurations validate successfully
  - [x] Verify generated schemas are valid JSON

- Schema tests:
  - [x] Verify schema generation completes without errors
  - [x] Verify schema changes reflect struct renames
  - [x] Verify schema validation works with updated configs

- Manual verification:
  - [x] Review example files for correct mode usage
  - [x] Test loading example configurations
  - [x] Verify schema matches config structure

## Success Criteria

- All example files use memory/persistent/distributed modes
- No `mode: standalone` in example files
- All examples are valid YAML
- Schema generation completes successfully
- Generated schemas are valid JSON
- Schemas reflect config struct changes
- Examples are clear and helpful
- No manual edits to schema files

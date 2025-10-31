## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/worker/embedded</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>temporal</dependencies>
</task_context>

# Task 3.0: Update Embedded Temporal Package

## Overview

This task updates the embedded Temporal package to use the new configuration struct names and standardize terminology. It includes updating type aliases, imports, and comments throughout the embedded Temporal package.

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
- Update type aliases to reference `EmbeddedTemporalConfig`
- Update imports if needed
- Update all comments to replace "standalone" with "embedded"
- Ensure consistency with renamed config structs from Task 1.0
</requirements>

## Subtasks

- [x] 3.1 Review `engine/worker/embedded/config.go` for type alias or separate struct
- [x] 3.2 Update type alias to reference `pkg/config.EmbeddedTemporalConfig` if applicable
- [x] 3.3 Update all comments in `engine/worker/embedded/config.go` to use "embedded" terminology
- [x] 3.4 Update all comments in `engine/worker/embedded/server.go` to use "embedded" terminology
- [x] 3.5 Update all comments in `engine/worker/embedded/builder.go` to use "embedded" terminology
- [x] 3.6 Verify imports are correct after config struct changes
- [x] 3.7 Update any function documentation that references "standalone"

## Implementation Details

See Phase 2.2 in the techspec for detailed implementation steps.

Key changes:
- Update type alias: `type Config = pkg/config.EmbeddedTemporalConfig` (if using alias)
- Update all comments to replace "standalone" with "embedded"
- Ensure consistency with Task 1.0 changes

### Relevant Files

- `engine/worker/embedded/config.go`
- `engine/worker/embedded/server.go`
- `engine/worker/embedded/builder.go`

### Dependent Files

- `pkg/config/config.go` - Defines `EmbeddedTemporalConfig` (from Task 1.0)
- `engine/infra/server/dependencies.go` - Uses embedded Temporal config

## Deliverables

- Type aliases updated to reference `EmbeddedTemporalConfig`
- All comments updated to use "embedded" terminology
- All function documentation updated
- Imports verified and correct
- Package consistent with config changes from Task 1.0
- All tests passing

## Tests

- Unit tests:
- [x] Verify embedded Temporal config type works correctly
- [x] Verify type alias resolves correctly (if using alias)

- Integration tests:
- [x] Verify embedded Temporal server starts correctly
- [x] Verify embedded Temporal configuration is applied correctly

- Regression tests:
- [x] Run existing embedded Temporal tests
- [x] Run server startup tests that use embedded Temporal

## Success Criteria

- All type references updated to `EmbeddedTemporalConfig`
- All comments use "embedded" terminology
- Package consistent with config changes
- All tests pass (`make test`)
- Linter passes (`make lint`)
- Code compiles without errors
- No references to "standalone" in embedded Temporal package (except validation errors)

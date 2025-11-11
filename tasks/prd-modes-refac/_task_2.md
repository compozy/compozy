## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/infra/cache</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>cache</dependencies>
</task_context>

# Task 2.0: Rename Cache Layer Functions & Types

## Overview

This task renames cache layer functions and types from "standalone" to "embedded" terminology. It includes renaming the `MiniredisStandalone` type to `MiniredisEmbedded` and updating related function names and comments.

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
- Rename `setupStandaloneCache` → `setupEmbeddedCache`
- Rename `MiniredisStandalone` → `MiniredisEmbedded`
- Consider renaming file: `miniredis_standalone.go` → `miniredis_embedded.go`
- Update all references to renamed types and functions
- Update comments to use "embedded" terminology
- Update field comment in Cache struct
</requirements>

## Subtasks

- [x] 2.1 Rename `setupStandaloneCache` → `setupEmbeddedCache` in `engine/infra/cache/mod.go`
- [x] 2.2 Update `setupMemoryCache` to call `setupEmbeddedCache`
- [x] 2.3 Update `setupPersistentCache` to call `setupEmbeddedCache`
- [x] 2.4 Update Cache struct field comment for `embedded` field
- [x] 2.5 Rename `MiniredisStandalone` → `MiniredisEmbedded` in `engine/infra/cache/miniredis_standalone.go`
- [x] 2.6 Rename `NewMiniredisStandalone` → `NewMiniredisEmbedded`
- [x] 2.7 Update all references to `MiniredisStandalone` in `engine/infra/cache/mod.go`
- [x] 2.8 Update type comments to use "embedded" terminology
- [x] 2.9 Consider renaming file `miniredis_standalone.go` → `miniredis_embedded.go` (optional, but recommended)
- [x] 2.10 Update any imports or references if file is renamed

## Implementation Details

See Phase 2.4 and Phase 2.5 in the techspec for detailed implementation steps.

Key changes:
- Function `setupStandaloneCache` becomes `setupEmbeddedCache`
- Type `MiniredisStandalone` becomes `MiniredisEmbedded`
- Constructor `NewMiniredisStandalone` becomes `NewMiniredisEmbedded`
- Update all call sites and references
- Update comments to reflect embedded terminology

### Relevant Files

- `engine/infra/cache/mod.go`
- `engine/infra/cache/miniredis_standalone.go`

### Dependent Files

- `engine/infra/cache/miniredis_standalone_test.go` - May need updates if file renamed
- `engine/infra/server/dependencies.go` - Uses cache setup (indirect)

## Deliverables

- `setupStandaloneCache` renamed to `setupEmbeddedCache`
- `MiniredisStandalone` type renamed to `MiniredisEmbedded`
- `NewMiniredisStandalone` renamed to `NewMiniredisEmbedded`
- All references updated
- Comments updated to use "embedded" terminology
- File optionally renamed (recommended)
- All tests passing
- No compilation errors

## Tests

- Unit tests:
  - [x] Verify cache setup functions work correctly with new names
  - [x] Verify embedded cache creation works with renamed type
  - [x] Verify memory cache mode works correctly
  - [x] Verify persistent cache mode works correctly

- Integration tests:
  - [x] Verify embedded Redis starts correctly
  - [x] Verify cache operations work with renamed types
  - [x] Verify cleanup functions work correctly

- Regression tests:
  - [x] Run existing cache tests to ensure no breakage
  - [x] Run server startup tests that use cache

## Success Criteria

- All cache layer functions use "embedded" terminology
- All cache layer types use "embedded" terminology
- All comments updated
- File renamed (if decided)
- All tests pass (`make test`)
- Linter passes (`make lint`)
- Code compiles without errors
- No references to "standalone" in cache layer code (except validation errors)

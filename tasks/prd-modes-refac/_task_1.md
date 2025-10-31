## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>pkg/config</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>database|temporal|http_server</dependencies>
</task_context>

# Task 1.0: Core Configuration & Server Functions Refactoring

## Overview

This task covers the core refactoring work to eliminate legacy "standalone" terminology from the configuration system and server startup logic. It includes removing dead code, adding missing validation, renaming configuration structs, updating builders and validators, and renaming server dependency functions.

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
- Remove unreachable legacy compatibility code from cache layer
- Add MCPProxy mode validation (missing validation gap)
- Rename configuration structs: `StandaloneConfig` → `EmbeddedTemporalConfig`, `RedisStandaloneConfig` → `EmbeddedRedisConfig`
- Update builder functions to use new struct names
- Rename validation functions: `validateStandalone*` → `validateEmbedded*`
- Rename server dependency functions: `maybeStartStandaloneTemporal` → `maybeStartEmbeddedTemporal`, etc.
- Update environment variable prefixes: `TEMPORAL_STANDALONE_*` → `TEMPORAL_EMBEDDED_*`
- Keep YAML tags as "standalone" for backward compatibility
- All changes must maintain existing functionality
</requirements>

## Subtasks

- [x] 1.1 Remove dead code from cache layer (lines 63-71 in `engine/infra/cache/mod.go`)
- [x] 1.2 Remove `legacyModeStandalone` constant from cache layer
- [x] 1.3 Add `validateMCPProxyMode` function to `pkg/config/loader.go`
- [x] 1.4 Update `validateMCPProxy` to call mode validation first
- [x] 1.5 Rename `StandaloneConfig` → `EmbeddedTemporalConfig` in `pkg/config/config.go`
- [x] 1.6 Update `TemporalConfig.Standalone` field with new type and updated comment
- [x] 1.7 Update all environment variable prefixes in `EmbeddedTemporalConfig`
- [x] 1.8 Rename `RedisStandaloneConfig` → `EmbeddedRedisConfig` in `pkg/config/config.go`
- [x] 1.9 Update `RedisConfig.Standalone` field with new type and updated comment
- [x] 1.10 Update `buildTemporalConfig` to use `EmbeddedTemporalConfig`
- [x] 1.11 Update `buildRedisConfig` to use `EmbeddedRedisConfig`
- [x] 1.12 Rename `validateStandaloneTemporalConfig` → `validateEmbeddedTemporalConfig`
- [x] 1.13 Rename all `validateStandalone*` helper functions → `validateEmbeddedTemporal*`
- [x] 1.14 Update validation function signatures to use `EmbeddedTemporalConfig`
- [x] 1.15 Update call site in `validateTemporal` function
- [x] 1.16 Rename `maybeStartStandaloneTemporal` → `maybeStartEmbeddedTemporal`
- [x] 1.17 Rename `standaloneEmbeddedConfig` → `embeddedTemporalConfig`
- [x] 1.18 Rename `standaloneTemporalCleanup` → `embeddedTemporalCleanup`
- [x] 1.19 Update all call sites of renamed server functions

## Implementation Details

See Phase 1.1, 1.2, Phase 2.1, 2.1.2, 2.1.3, and Phase 2.3 in the techspec for detailed implementation steps.

Key changes:
- Remove unreachable legacy mode mapping code (already rejected by loader validation)
- Add MCPProxy mode validation to match Redis/Temporal validation patterns
- Rename structs while keeping YAML tags for backward compatibility
- Update environment variable prefixes consistently
- Rename all validation and server functions to use "embedded" terminology

### Relevant Files

- `engine/infra/cache/mod.go`
- `pkg/config/config.go`
- `pkg/config/loader.go`
- `engine/infra/server/dependencies.go`

### Dependent Files

- `pkg/config/resolver.go` - Uses config structs
- `engine/infra/server/server.go` - Calls server dependency functions
- `pkg/config/definition/schema.go` - References config structs

## Deliverables

- Dead code removed from cache layer
- MCPProxy mode validation implemented and tested
- All configuration structs renamed (`StandaloneConfig` → `EmbeddedTemporalConfig`, `RedisStandaloneConfig` → `EmbeddedRedisConfig`)
- All builder functions updated to use new struct names
- All validation functions renamed and updated
- All server dependency functions renamed
- Environment variable prefixes updated (`TEMPORAL_STANDALONE_*` → `TEMPORAL_EMBEDDED_*`)
- All tests passing
- No compilation errors
- Code follows project standards

## Tests

- Unit tests for MCPProxy mode validation:
  - [x] Test MCPProxy mode validation rejects "standalone" mode
  - [x] Test MCPProxy mode validation accepts valid modes (memory, persistent, distributed)
  - [x] Test MCPProxy mode validation accepts empty string (inheritance)
  - [x] Test MCPProxy mode validation rejects invalid mode values
  - [x] Test MCPProxy port validation still works with new mode validation

- Integration tests:
  - [x] Verify embedded Temporal starts correctly with renamed functions
  - [x] Verify configuration loading works with renamed structs
  - [x] Verify mode resolution still works correctly
  - [x] Verify validation errors are clear and helpful

- Regression tests:
  - [x] Run existing config tests to ensure no breakage
  - [x] Run existing cache tests to ensure dead code removal doesn't break anything
  - [x] Run existing server startup tests

## Success Criteria

- All dead code removed from cache layer
- MCPProxy mode validation matches Redis/Temporal validation patterns
- All configuration structs use "Embedded" terminology
- All validation functions use "Embedded" terminology
- All server functions use "Embedded" terminology
- Environment variable prefixes updated consistently
- All tests pass (`make test`)
- Linter passes (`make lint`)
- Code compiles without errors
- No grep results for inappropriate "standalone" usage in renamed code

## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/infra/server</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>temporal</dependencies>
</task_context>

# Task 6.0: Update Temporal Wiring

## Overview

Update Temporal wiring in `engine/infra/server/dependencies.go` to support three modes with intelligent database path defaults and updated validation logic.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technicals docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

<research>
# When you need information about a library or external API:
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase, the same for the topic param of context7
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
- Update maybeStartStandaloneTemporal() to start embedded Temporal for both memory and persistent modes
- Update standaloneEmbeddedConfig() to set intelligent database path defaults based on mode
- Update validateDatabaseConfig() to remove hardcoded "standalone" string references
- Memory mode should default to `:memory:` for Temporal database
- Persistent mode should default to `./.compozy/temporal.db` for Temporal database
- Add clear logging showing mode, database path, and ports
</requirements>

## Subtasks

- [x] 6.1 Update maybeStartStandaloneTemporal() to handle memory and persistent modes
- [x] 6.2 Update standaloneEmbeddedConfig() with intelligent database path defaults
- [x] 6.3 Update validateDatabaseConfig() to use mode checks instead of hardcoded strings
- [x] 6.4 Add comprehensive logging for Temporal startup
- [x] 6.5 Validate Temporal starts correctly in all three modes

## Implementation Details

See **Phase 2.2: Update Temporal Wiring** in `_techspec.md` (lines 611-755).

**Key Changes:**

1. **Lines 378-414** - Update `maybeStartStandaloneTemporal()`:
   - Check for `mode == ModeMemory || mode == ModePersistent` to start embedded Temporal
   - Distributed mode continues to use external Temporal (no change)
   - Add detailed logging with mode, database path, and ports

2. **Lines 416-430** - Update `standaloneEmbeddedConfig()`:
   - If `DatabaseFile` is empty and mode is `persistent`: default to `./.compozy/temporal.db`
   - If `DatabaseFile` is empty and mode is `memory`: default to `:memory:`
   - Explicit config values always take precedence

3. **Lines 133-160** - Update `validateDatabaseConfig()`:
   - Replace hardcoded "standalone" strings with mode variable
   - Add mode to warning/error log messages
   - Improve guidance messages to reference correct modes

### Relevant Files

- `engine/infra/server/dependencies.go` - Temporal startup and database validation

### Dependent Files

- `pkg/config/resolver.go` - Mode constants and resolution
- `pkg/config/config.go` - Configuration structure
- Embedded Temporal package (`temporal.io/server/temporal`)

## Deliverables

- Updated `maybeStartStandaloneTemporal()` supporting memory and persistent modes
- Intelligent database path defaults in `standaloneEmbeddedConfig()`
- Mode-aware validation messages in `validateDatabaseConfig()`
- Clear logging showing Temporal configuration and startup status
- All existing Temporal tests passing with updated modes

## Tests

Unit tests mapped from `_tests.md` for Temporal layer:
- [ ] Test Temporal startup in memory mode (uses :memory: database)
- [ ] Test Temporal startup in persistent mode (uses file database)
- [ ] Test Temporal skips startup in distributed mode
- [ ] Test default database path for memory mode
- [ ] Test default database path for persistent mode
- [ ] Test explicit database path override in each mode
- [ ] Test validateDatabaseConfig() warnings with mode context
- [ ] Verify Temporal state persists in persistent mode after restart
- [ ] Verify Temporal state is ephemeral in memory mode

## Success Criteria

- `make lint` passes with no errors
- `go test ./engine/infra/server/... -run TestMaybeStartStandaloneTemporal -v` passes
- `go test ./engine/infra/server/... -run TestValidateDatabaseConfig -v` passes
- Embedded Temporal starts correctly in memory mode with :memory: database
- Embedded Temporal starts correctly in persistent mode with file database
- Distributed mode continues to use external Temporal (no regression)
- Logging clearly indicates mode, database path, and ports
- Database validation warnings include mode context

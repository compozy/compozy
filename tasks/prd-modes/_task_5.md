## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/infra/cache</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>redis</dependencies>
</task_context>

# Task 5.0: Update Cache Layer

## Overview

Update cache layer (`engine/infra/cache/mod.go`) to support three modes (memory/persistent/distributed) with auto-configuration of persistence settings.

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
- Replace old mode constants (standalone/distributed) with new modes (memory/persistent/distributed)
- Auto-disable persistence for memory mode
- Auto-enable persistence for persistent mode with default paths
- Both memory and persistent modes use the same `setupStandaloneCache()` function
- Add informative logging for mode transitions
</requirements>

## Subtasks

- [x] 5.1 Update mode constants in cache/mod.go
- [x] 5.2 Update SetupCache() switch statement with mode-specific persistence logic
- [x] 5.3 Add logging for cache mode and persistence configuration
- [x] 5.4 Validate cache setup works correctly in all three modes

## Implementation Details

See **Phase 2.1: Update Cache Layer** in `_techspec.md` (lines 543-609).

**Key Changes:**
- Lines 12-15: Update mode constants from `modeStandalone`/`modeDistributed` to `modeMemory`/`modePersistent`/`modeDistributed`
- Lines 60-69: Update SetupCache() switch with auto-configuration:
  - `memory`: Force persistence OFF
  - `persistent`: Auto-enable persistence with default path `./.compozy/redis`
  - `distributed`: Use external Redis (unchanged)

**Key Insight:** Both memory and persistent modes use the SAME `setupStandaloneCache()` function (from Redis PRD), differentiated only by persistence settings.

### Relevant Files

- `engine/infra/cache/mod.go` - Cache factory and mode routing

### Dependent Files

- `pkg/config/resolver.go` - Mode constants and resolution
- Redis PRD infrastructure:
  - `MiniredisStandalone` wrapper
  - `SnapshotManager` with BadgerDB

## Deliverables

- Updated mode constants in cache layer
- Mode-aware persistence configuration in SetupCache()
- Clear logging showing cache mode and persistence status
- All existing cache tests passing with updated modes

## Tests

Unit tests mapped from `_tests.md` for cache layer:
- [x] Test cache setup in memory mode (persistence forced OFF)
- [x] Test cache setup in persistent mode (persistence auto-enabled)
- [x] Test cache setup in distributed mode (external Redis)
- [x] Test default persistence path for persistent mode
- [x] Test explicit persistence override in persistent mode
- [x] Verify no persistence files created in memory mode
- [x] Verify persistence files created in persistent mode

## Success Criteria

- `make lint` passes with no errors
- `go test ./engine/infra/cache/... -v` passes all tests
- Cache initializes correctly in each mode
- Persistence behavior matches mode intent (ephemeral for memory, persisted for persistent)
- Logging clearly indicates active mode and persistence state
- No breaking changes to distributed mode behavior

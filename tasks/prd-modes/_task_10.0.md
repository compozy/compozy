## status: pending

<task_context>
<domain>test/helpers</domain>
<type>testing</type>
<scope>test_infrastructure</scope>
<complexity>medium</complexity>
<dependencies>database|config|mode_system</dependencies>
</task_context>

# Task 10.0: Add Database Mode Helper

## Overview

Create a new test helper `SetupDatabaseWithMode` that intelligently selects database backend (SQLite memory/file or PostgreSQL) based on mode configuration, simplifying test setup for mode-aware tests.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start (tasks/prd-modes/_techspec.md)
- **YOU SHOULD ALWAYS** have in mind that this is a greenfield approach - no backwards compatibility required
- **MUST** verify Phase 1 (Core Config) is complete before starting
</critical>

<research>
When you need information about mode-based configuration:
- Use perplexity to find patterns for test configuration management
- Reference completed Phase 1 tasks for mode constant definitions
</research>

<requirements>
- Create `SetupDatabaseWithMode(t *testing.T, mode string)` helper
- Map mode to appropriate database backend (memory→SQLite :memory:, persistent→SQLite file, distributed→PostgreSQL)
- Use `t.Context()` for proper context inheritance
- Configure database paths appropriately for each mode
- Add comprehensive documentation and examples
</requirements>

## Subtasks

- [ ] 10.1 Create `SetupDatabaseWithMode` function signature
- [ ] 10.2 Implement mode-to-backend mapping logic
- [ ] 10.3 Handle SQLite memory mode configuration
- [ ] 10.4 Handle SQLite persistent mode configuration (temporary file)
- [ ] 10.5 Handle distributed mode configuration (PostgreSQL)
- [ ] 10.6 Add helper documentation with usage examples
- [ ] 10.7 Create example test demonstrating mode switching

## Implementation Details

### Objective
Provide a unified test helper that abstracts database setup based on mode, making it easy to test mode-specific behavior without manual configuration.

### Key Implementation

**File:** `test/helpers/database.go`

**New function:**
```go
// SetupDatabaseWithMode configures database based on deployment mode.
// - "memory": SQLite :memory: (fastest, ephemeral)
// - "persistent": SQLite temp file (survives test duration)
// - "distributed": PostgreSQL testcontainer (full features)
func SetupDatabaseWithMode(t *testing.T, mode string) (*sqlx.DB, func())
```

**Mode mapping:**
- `"memory"` → SQLite with `:memory:` path
- `"persistent"` → SQLite with `t.TempDir() + "/compozy.db"` path
- `"distributed"` → Call `SetupPostgresContainer(t)`

**Return:**
- Configured database connection
- Cleanup function to close connection and remove temp files

### Usage Example

```go
func TestWithMemoryMode(t *testing.T) {
    db, cleanup := helpers.SetupDatabaseWithMode(t, "memory")
    defer cleanup()

    // Test runs with in-memory SQLite
}

func TestWithDistributedMode(t *testing.T) {
    db, cleanup := helpers.SetupDatabaseWithMode(t, "distributed")
    defer cleanup()

    // Test runs with PostgreSQL testcontainer
}
```

### Relevant Files

- `test/helpers/database.go` - Primary implementation

### Dependent Files

- Phase 1 tasks: `pkg/config/resolver.go` (mode constants)
- Task 9.0: Database helpers

## Deliverables

- `SetupDatabaseWithMode` function implementation
- Documentation comments explaining mode mapping
- Example test demonstrating mode switching
- Unit tests for the helper function itself

## Tests

- [ ] Unit test: memory mode returns SQLite :memory: connection
- [ ] Unit test: persistent mode returns SQLite file connection with temp directory
- [ ] Unit test: distributed mode returns PostgreSQL connection
- [ ] Integration test: mode switching works correctly across test cases
- [ ] Verify cleanup functions properly close connections and remove temp files
- [ ] Confirm `t.Context()` usage for context inheritance

## Success Criteria

- Helper correctly maps all three modes to appropriate backends
- Cleanup functions work properly (no leaked connections or temp files)
- Documentation clearly explains when to use this helper
- Example test demonstrates practical usage
- All unit and integration tests pass

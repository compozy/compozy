## status: pending

<task_context>
<domain>test/integration</domain>
<type>testing</type>
<scope>test_infrastructure</scope>
<complexity>medium</complexity>
<dependencies>mode_system|temporal|config</dependencies>
</task_context>

# Task 12.0: Update Integration Test Helpers

## Overview

Update integration test helper files to reference new mode names (memory/persistent/distributed) and ensure proper mode switching test coverage.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start (tasks/prd-modes/_techspec.md)
- **YOU SHOULD ALWAYS** have in mind that this is a greenfield approach - no backwards compatibility required
- **MUST** complete Phase 1 (Core Config) before starting
- **MUST** complete Task 11.0 (Integration Test Migration) before starting
</critical>

<research>
When you need information about test organization:
- Use perplexity to find Go integration test best practices
- Reference completed Phase 1 tasks for mode constant usage
</research>

<requirements>
- Update `test/integration/standalone/helpers.go` to use "memory" terminology
- Update `test/integration/temporal/mode_switching_test.go` for three modes
- Add test coverage for persistent mode
- Ensure all helpers use `t.Context()` for context inheritance
- Verify mode resolution works correctly in test scenarios
</requirements>

## Subtasks

- [ ] 12.1 Update `test/integration/standalone/helpers.go` mode references
- [ ] 12.2 Rename "standalone" references to "memory" throughout helpers
- [ ] 12.3 Update `mode_switching_test.go` to test all three modes
- [ ] 12.4 Add `TestModeResolver_Persistent` test case
- [ ] 12.5 Update `TestModeResolver_Memory` (renamed from standalone)
- [ ] 12.6 Verify `TestModeResolver_Distributed` still works
- [ ] 12.7 Add integration test for mode inheritance behavior
- [ ] 12.8 Run mode switching tests and verify all pass

## Implementation Details

### Objective
Update integration test infrastructure to align with new three-mode system and verify mode resolution logic works correctly.

### Key Changes

**File:** `test/integration/standalone/helpers.go`

1. **Update terminology:**
   - Replace "standalone" with "memory" in function names and comments
   - Update documentation to reflect new mode system
   - Ensure helpers configure memory mode correctly

2. **Consider renaming directory:**
   - Evaluate if `test/integration/standalone/` should be renamed to `test/integration/memory/`
   - If renamed, update all import paths

**File:** `test/integration/temporal/mode_switching_test.go`

**Existing test structure (needs update):**
```go
func TestModeResolver_Distributed(t *testing.T) {
    // ... test distributed mode
}

func TestModeResolver_Standalone(t *testing.T) {  // RENAME
    // ... test memory mode
}
```

**New test structure:**
```go
func TestModeResolver_Memory(t *testing.T) {
    cfg := &config.Config{Mode: "memory"}

    // Verify resolution
    assert.Equal(t, "memory", config.ResolveMode(cfg, ""))
    assert.Equal(t, "sqlite", cfg.EffectiveDatabaseDriver())
    assert.Equal(t, "memory", cfg.EffectiveTemporalMode())
}

func TestModeResolver_Persistent(t *testing.T) {
    cfg := &config.Config{Mode: "persistent"}

    // Verify resolution
    assert.Equal(t, "persistent", config.ResolveMode(cfg, ""))
    assert.Equal(t, "sqlite", cfg.EffectiveDatabaseDriver())
    assert.Equal(t, "persistent", cfg.EffectiveTemporalMode())
}

func TestModeResolver_Distributed(t *testing.T) {
    cfg := &config.Config{Mode: "distributed"}

    // Verify resolution
    assert.Equal(t, "distributed", config.ResolveMode(cfg, ""))
    assert.Equal(t, "postgres", cfg.EffectiveDatabaseDriver())
    assert.Equal(t, "remote", cfg.EffectiveTemporalMode())
}

func TestModeResolver_Inheritance(t *testing.T) {
    cfg := &config.Config{
        Mode: "memory",
        Temporal: config.TemporalConfig{
            Mode: "persistent",  // Override global
        },
    }

    // Verify component override
    assert.Equal(t, "memory", config.ResolveMode(cfg, ""))
    assert.Equal(t, "persistent", cfg.EffectiveTemporalMode())
}
```

### Relevant Files

- `test/integration/standalone/helpers.go`
- `test/integration/temporal/mode_switching_test.go`

### Dependent Files

- Phase 1: `pkg/config/resolver.go` (mode resolution logic)
- Task 11.0: Migrated integration tests

## Deliverables

- Updated helper files with correct mode terminology
- Comprehensive mode switching tests for all three modes
- Mode inheritance test coverage
- Documentation of mode resolution behavior in tests

## Tests

- [ ] `TestModeResolver_Memory` passes
- [ ] `TestModeResolver_Persistent` passes
- [ ] `TestModeResolver_Distributed` passes
- [ ] `TestModeResolver_Inheritance` passes (component override)
- [ ] All integration tests using helpers still pass
- [ ] Verify `t.Context()` usage in all test helpers
- [ ] Run `make test` to confirm no regressions

## Success Criteria

- All mode switching tests pass
- Helper files use correct mode terminology
- Mode inheritance behavior is tested and verified
- No regressions in existing integration tests
- Documentation clearly explains mode resolution in tests

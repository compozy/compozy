# Task 1.0: Update Mode Constants & Defaults

<task_context>
<phase>Phase 1: Core Configuration</phase>
<priority>CRITICAL - BLOCKING</priority>
<complexity>Medium</complexity>
<estimated_duration>1 day</estimated_duration>
</task_context>

---

## Objective

Update mode constants in `pkg/config/resolver.go` to support three modes (memory/persistent/distributed) and change the default mode from `distributed` to `memory`.

**Impact**: BLOCKING - All other work depends on this change.

---

<critical>
**MANDATORY VALIDATION:**
- Run `go test ./pkg/config -run TestResolveMode` - MUST PASS
- Run `go test ./pkg/config -run TestEffectiveDatabaseDriver` - MUST PASS
- Run `make lint` on pkg/config - MUST BE CLEAN

**BREAKING CHANGE:**
- Default mode changes from "distributed" to "memory"
- "standalone" mode constant removed (breaking for alpha users)
- All mode references must use new constants
  
**YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

---

<requirements>

### Mode System Changes

**New Mode Constants:**
- `ModeMemory = "memory"` - In-memory SQLite, fastest
- `ModePersistent = "persistent"` - File-based SQLite
- `ModeDistributed = "distributed"` - PostgreSQL + external services
- `ModeRemoteTemporal = "remote"` - UNCHANGED

**Default Mode:**
- OLD: `ModeDistributed`
- NEW: `ModeMemory`

**Rationale**: Zero-dependency quickstart, faster test execution

</requirements>

---

<research>
**Implementation Reference**: See `_techspec.md` Section "Phase 1.1: Update Mode Constants" (lines 290-369)

**Key Files to Understand:**
- `pkg/config/resolver.go` - Mode resolution logic
- `pkg/config/config.go` - Configuration structure
- Redis PRD implementation - Persistence configuration patterns

**Testing Approach:**
- Unit tests for mode resolution
- Tests for database driver selection
- Tests for temporal mode mapping
</research>

---

## Subtasks

### 1.1 Replace Mode Constants
**File**: `pkg/config/resolver.go` (lines 6-11)

- [ ] Remove `ModeStandalone` constant
- [ ] Add `ModeMemory` constant with comment
- [ ] Add `ModePersistent` constant with comment
- [ ] Keep `ModeDistributed` constant (update comment)
- [ ] Keep `ModeRemoteTemporal` constant (unchanged)

**Reference**: `_techspec.md` lines 297-311

---

### 1.2 Update Default Mode
**File**: `pkg/config/resolver.go` (line 26)

- [ ] Change `return ModeDistributed` to `return ModeMemory`
- [ ] Update function docstring (line 18) to reflect new default

**Reference**: `_techspec.md` lines 314-329

---

### 1.3 Update EffectiveTemporalMode Logic
**File**: `pkg/config/resolver.go` (lines 36-42)

- [ ] Update logic to handle `ModeMemory` and `ModePersistent`
- [ ] Both memory and persistent should return embedded mode
- [ ] Only `ModeDistributed` returns `ModeRemoteTemporal`
- [ ] Add comment explaining mode mapping

**Reference**: `_techspec.md` lines 332-341

---

### 1.4 Update EffectiveDatabaseDriver Logic
**File**: `pkg/config/resolver.go` (lines 49-65)

- [ ] Update nil check to return SQLite (changed from Postgres)
- [ ] Check for `ModeMemory || ModePersistent` → return SQLite
- [ ] Check for `ModeDistributed` → return Postgres
- [ ] Default fallback to SQLite
- [ ] Add comprehensive comments

**Reference**: `_techspec.md` lines 344-362

---

## Relevant Files

### Primary Files (Modified)
- `pkg/config/resolver.go` - Mode constants and resolution

### Dependent Files (Reference Only)
- `pkg/config/config.go` - Will be updated in Task 2.0
- `pkg/config/definition/schema.go` - Will be updated in Task 3.0
- `engine/infra/cache/mod.go` - Will be updated in Task 5.0
- `engine/infra/server/dependencies.go` - Will be updated in Task 6.0

---

## Deliverables

1. **Updated Constants**
   - `ModeMemory`, `ModePersistent`, `ModeDistributed` constants defined
   - `ModeStandalone` constant removed
   - Clear comments explaining each mode

2. **Updated Default**
   - Default mode changed to `ModeMemory`
   - Docstrings updated

3. **Updated Mode Resolution**
   - `EffectiveTemporalMode()` handles three modes
   - `EffectiveDatabaseDriver()` handles three modes
   - Logical defaults for each mode

4. **Tests Pass**
   - All existing tests updated and passing
   - No lint errors

---

## Tests

### Unit Tests to Update
**File**: `pkg/config/resolver_test.go` (Task 4.0 will handle this)

Expected test coverage:
- Mode resolution with new constants
- Database driver selection for each mode
- Temporal mode selection for each mode
- Default mode behavior

### Validation Commands
```bash
# Must pass before completing task
go test ./pkg/config -run TestResolveMode -v
go test ./pkg/config -run TestEffectiveDatabaseDriver -v
make lint
```

---

## Success Criteria

- [x] All mode constants updated (memory/persistent/distributed)
- [x] Default mode changed to `ModeMemory`
- [x] `EffectiveTemporalMode()` logic updated
- [x] `EffectiveDatabaseDriver()` logic updated
- [x] All docstrings and comments updated
- [x] Code compiles without errors
- [x] No lint warnings or errors
- [x] Tests pass (will be comprehensive after Task 4.0)

---

## Dependencies

**Blocks:**
- Task 2.0 (Configuration Validation)
- Task 3.0 (Configuration Registry)
- Task 4.0 (Configuration Tests)
- ALL Phase 2, 3, 4, 5, 6 tasks

**Depends On:**
- None (first task in sequence)

---

## Notes

- This is a BREAKING CHANGE for alpha users
- Old "standalone" mode maps conceptually to new "memory" mode
- Implementation details in `_techspec.md` lines 290-369
- Keep changes focused - avoid scope creep to other files

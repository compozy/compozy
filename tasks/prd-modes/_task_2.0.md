# Task 2.0: Update Configuration Validation

<task_context>
<phase>Phase 1: Core Configuration</phase>
<priority>CRITICAL</priority>
<complexity>Medium</complexity>
<estimated_duration>1 day</estimated_duration>
</task_context>

---

## Objective

Update configuration struct validation tags and documentation in `pkg/config/config.go` to reflect the new three-mode system (memory/persistent/distributed).

**Impact**: Ensures configuration validation catches invalid modes and provides clear documentation for developers.

---

<critical>
**MANDATORY VALIDATION:**
- Run `go test ./pkg/config -run TestConfigValidation` - MUST PASS
- Run `make lint` on pkg/config - MUST BE CLEAN
- Verify struct tags are correctly formatted

**BREAKING CHANGE:**
- Mode validation rejects "standalone" (breaking for alpha users)
- Only accepts "memory", "persistent", "distributed"
</critical>

---

<requirements>

### Configuration Changes

**Mode Field Validation Tag:**
- OLD: `validate:"omitempty,oneof=standalone distributed"`
- NEW: `validate:"omitempty,oneof=memory persistent distributed"`

**Mode Field Documentation:**
- Update doc comments to explain all three modes
- Include use case guidance for each mode
- Reference deployment scenarios

**Constant Cleanup:**
- Remove `mcpProxyModeStandalone` constant (obsolete)
- Keep database driver constants

</requirements>

---

<research>
**Implementation Reference**: See `_techspec.md` Section "Phase 1.2: Update Configuration Validation" (lines 371-419)

**Key Concepts:**
- Go struct tag validation (validator package)
- Koanf configuration binding
- Environment variable mapping
- Documentation comments for Go structs

**Related Files:**
- `pkg/config/config.go` - Main config struct
- `pkg/config/resolver.go` - Mode resolution (Task 1.0)
</research>

---

## Subtasks

### 2.1 Update Mode Field Validation
**File**: `pkg/config/config.go` (line 56)

- [ ] Update `validate` struct tag to accept new modes
- [ ] Change `oneof=standalone distributed` to `oneof=memory persistent distributed`
- [ ] Verify all other struct tags remain unchanged (koanf, env, json, yaml, mapstructure)

**Reference**: `_techspec.md` lines 375-381

---

### 2.2 Update Mode Documentation
**File**: `pkg/config/config.go` (lines 52-55)

- [ ] Update Mode field doc comment
- [ ] Explain "memory" mode (default, in-memory, fastest)
- [ ] Explain "persistent" mode (file-based, local dev)
- [ ] Explain "distributed" mode (production, external services)

**Reference**: `_techspec.md` lines 384-397

---

### 2.3 Clean Up Obsolete Constants
**File**: `pkg/config/config.go` (line 17)

- [ ] Remove `mcpProxyModeStandalone = "standalone"` constant
- [ ] Keep `databaseDriverPostgres` constant
- [ ] Keep `databaseDriverSQLite` constant
- [ ] Verify no other code references the removed constant

**Reference**: `_techspec.md` lines 400-413

---

## Relevant Files

### Primary Files (Modified)
- `pkg/config/config.go` - Configuration struct and validation

### Dependent Files (Reference Only)
- `pkg/config/resolver.go` - Mode constants (updated in Task 1.0)
- `pkg/config/definition/schema.go` - Will be updated in Task 3.0

### Files to Search (Verify No References)
- Grep codebase for `mcpProxyModeStandalone` usage before removing

---

## Deliverables

1. **Updated Validation Tags**
   - Mode field validates against memory/persistent/distributed
   - Rejects "standalone" mode
   - All struct tags properly formatted

2. **Updated Documentation**
   - Clear mode field documentation
   - Use case guidance for each mode
   - Deployment scenario references

3. **Cleaned Constants**
   - Obsolete `mcpProxyModeStandalone` removed
   - No dangling references in codebase

4. **Tests Pass**
   - Configuration validation tests pass
   - No lint errors

---

## Tests

### Unit Tests to Verify
**File**: `pkg/config/config_test.go` (Task 4.0 will update these)

Expected validation behavior:
- "memory" mode validates successfully
- "persistent" mode validates successfully
- "distributed" mode validates successfully
- "standalone" mode fails validation
- Invalid modes fail validation

### Validation Commands
```bash
# Must pass before completing task
go test ./pkg/config -run TestConfigValidation -v
make lint

# Verify no references to removed constant
grep -r "mcpProxyModeStandalone" . --include="*.go"
# Should return no results (except in git history)
```

---

## Success Criteria

- [x] Mode field validation tag updated
- [x] Mode field documentation updated
- [x] Obsolete constant removed
- [x] No dangling references to removed constant
- [x] Code compiles without errors
- [x] No lint warnings or errors
- [x] Validation tests will pass (comprehensive after Task 4.0)

---

## Dependencies

**Blocks:**
- Task 4.0 (Configuration Tests) - needs updated validation

**Depends On:**
- Task 1.0 (Mode Constants) - needs new mode constant definitions

---

## Notes

- Validation changes are BREAKING for alpha users
- Clear error messages important for migration experience
- Implementation details in `_techspec.md` lines 371-419
- Keep changes focused on config.go only

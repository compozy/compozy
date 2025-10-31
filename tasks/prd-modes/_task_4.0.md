# Task 4.0: Update Configuration Tests

<task_context>
<phase>Phase 1: Core Configuration</phase>
<priority>CRITICAL</priority>
<complexity>Large</complexity>
<estimated_duration>2 days</estimated_duration>
</task_context>

---

## Objective

Update all configuration tests in `pkg/config/*_test.go` to validate the new three-mode system (memory/persistent/distributed) and ensure comprehensive test coverage for mode resolution and validation.

**Impact**: Validates Phase 1 implementation and ensures no regressions in configuration logic.

---

<critical>
**MANDATORY VALIDATION:**
- Run `make lint` - MUST BE CLEAN
- Run `go test ./pkg/config/... -v` - ALL TESTS MUST PASS
- Run `make test` - FULL TEST SUITE MUST PASS

**COMPLETION CRITERIA:**
- Cannot complete this task until ALL tests pass
- Cannot complete this task until linter is clean
- This task BLOCKS all remaining phases

**TEST COVERAGE:**
- Mode resolution for all three modes
- Database driver selection for all modes
- Temporal mode selection for all modes
- Validation accepts valid modes
- Validation rejects invalid modes (including "standalone")
</critical>

---

<requirements>

### Test File Updates

**Files to Update:**
1. `pkg/config/resolver_test.go` - Mode resolution tests
2. `pkg/config/config_test.go` - Validation tests
3. `pkg/config/loader_test.go` - Config loading tests (if needed)

**Test Coverage Required:**
- Mode resolution with new constants
- Database driver selection for each mode
- Temporal mode selection for each mode
- Default mode behavior (memory)
- Validation accepts memory/persistent/distributed
- Validation rejects standalone/invalid

</requirements>

---

<research>
**Implementation Reference**: See `_techspec.md` Section "Phase 1.4: Update Configuration Tests" (lines 466-532)

**Key Test Patterns:**
- Table-driven tests for mode resolution
- Validation test cases
- Edge case handling (empty modes, nil configs)
- Error message validation

**Related Files:**
- `pkg/config/resolver_test.go` - Primary focus
- `pkg/config/config_test.go` - Validation focus
- `pkg/config/resolver.go` - Implementation (Tasks 1.0)
- `pkg/config/config.go` - Struct (Task 2.0)
</research>

---

## Subtasks

### 4.1 Update Mode Resolution Tests
**File**: `pkg/config/resolver_test.go`

- [ ] Update `TestResolveMode` test cases
- [ ] Test component mode override (distributed → memory)
- [ ] Test global mode inheritance (persistent → components use persistent)
- [ ] Test default mode behavior (empty → memory)
- [ ] Add test for all three modes explicitly

**Reference**: `_techspec.md` lines 470-499

**Example Test Structure:**
```go
func TestResolveMode(t *testing.T) {
    tests := []struct {
        name          string
        globalMode    string
        componentMode string
        want          string
    }{
        {
            name:          "Should use component mode when set",
            globalMode:    "distributed",
            componentMode: "memory",
            want:          "memory",
        },
        {
            name:          "Should use global mode when component not set",
            globalMode:    "persistent",
            componentMode: "",
            want:          "persistent",
        },
        {
            name:          "Should default to memory when neither set",
            globalMode:    "",
            componentMode: "",
            want:          "memory",  // Changed from "distributed"
        },
    }
    // ... test implementation
}
```

---

### 4.2 Update Database Driver Selection Tests
**File**: `pkg/config/resolver_test.go`

- [ ] Update `TestEffectiveDatabaseDriver` test cases
- [ ] Test memory mode → SQLite
- [ ] Test persistent mode → SQLite
- [ ] Test distributed mode → PostgreSQL
- [ ] Test nil config → SQLite (default changed)
- [ ] Test explicit driver override

---

### 4.3 Update Temporal Mode Selection Tests
**File**: `pkg/config/resolver_test.go`

- [ ] Update `TestEffectiveTemporalMode` test cases (if exists)
- [ ] Test memory mode → embedded temporal
- [ ] Test persistent mode → embedded temporal
- [ ] Test distributed mode → remote temporal
- [ ] Test explicit temporal mode override

---

### 4.4 Update Mode Validation Tests
**File**: `pkg/config/config_test.go`

- [ ] Update `TestModeValidation` test cases
- [ ] Test "memory" mode validates successfully
- [ ] Test "persistent" mode validates successfully
- [ ] Test "distributed" mode validates successfully
- [ ] Test "standalone" mode FAILS validation (breaking change)
- [ ] Test invalid modes FAIL validation
- [ ] Verify error messages are helpful

**Reference**: `_techspec.md` lines 505-518

**Example Test Structure:**
```go
func TestModeValidation(t *testing.T) {
    tests := []struct {
        mode    string
        wantErr bool
    }{
        {"memory", false},
        {"persistent", false},
        {"distributed", false},
        {"standalone", true},  // No longer valid
        {"invalid", true},
    }
    // ... test implementation
}
```

---

### 4.5 Update Config Loading Tests
**File**: `pkg/config/loader_test.go`

- [ ] Review loader tests for mode-specific logic
- [ ] Update test configs with new mode values
- [ ] Verify mode defaults applied correctly
- [ ] Test environment variable overrides

---

### 4.6 Run Full Validation
**All Test Files**

- [ ] Run `make lint` - verify zero warnings
- [ ] Run `go test ./pkg/config/... -v` - all tests pass
- [ ] Run `make test` - full suite passes
- [ ] Verify test coverage maintained (>80%)

---

## Relevant Files

### Primary Files (Modified)
- `pkg/config/resolver_test.go` - Mode resolution tests
- `pkg/config/config_test.go` - Validation tests
- `pkg/config/loader_test.go` - Loader tests (if needed)

### Dependent Files (Implementation)
- `pkg/config/resolver.go` - Implementation (Task 1.0)
- `pkg/config/config.go` - Struct (Task 2.0)
- `pkg/config/definition/schema.go` - Registry (Task 3.0)

---

## Deliverables

1. **Updated Test Cases**
   - All mode resolution tests updated
   - All validation tests updated
   - All loader tests updated (if needed)

2. **Comprehensive Coverage**
   - All three modes tested
   - Edge cases covered
   - Error cases validated

3. **All Tests Passing**
   - `make lint` clean
   - `go test ./pkg/config/...` passes
   - `make test` passes

4. **Helpful Error Messages**
   - Invalid modes produce clear errors
   - "standalone" rejection includes migration hint

---

## Tests

### Test Execution Commands
```bash
# Individual test packages
go test ./pkg/config -v
go test ./pkg/config/definition -v

# Specific test functions
go test ./pkg/config -run TestResolveMode -v
go test ./pkg/config -run TestEffectiveDatabaseDriver -v
go test ./pkg/config -run TestModeValidation -v

# Full validation
make lint
make test
```

### Expected Outcomes
- All tests pass (100%)
- Linter clean (zero warnings)
- Test coverage >80%
- No flaky tests

---

## Success Criteria

- [x] Mode resolution tests updated and passing
- [x] Database driver tests updated and passing
- [x] Temporal mode tests updated and passing
- [x] Validation tests updated and passing
- [x] Loader tests updated and passing (if needed)
- [x] All tests pass: `go test ./pkg/config/... -v`
- [x] Linter clean: `make lint`
- [x] Full suite passes: `make test`
- [x] Test coverage maintained (>80%)
- [x] Error messages are clear and helpful

---

## Dependencies

**Blocks:**
- ALL Phase 2 tasks (Infrastructure)
- ALL Phase 3 tasks (Test Infrastructure)
- ALL Phase 4 tasks (Documentation)
- ALL Phase 5 tasks (Schemas)
- ALL Phase 6 tasks (Validation)

**Depends On:**
- Task 1.0 (Mode Constants) - implementation to test
- Task 2.0 (Configuration Validation) - validation to test
- Task 3.0 (Configuration Registry) - registry to test

---

## Notes

- This task is the **final gate for Phase 1**
- Cannot proceed to Phase 2 until ALL tests pass
- Focus on comprehensive coverage for all three modes
- Validation failures should produce helpful error messages
- Implementation details in `_techspec.md` lines 466-532
- This is the most critical testing task in Phase 1

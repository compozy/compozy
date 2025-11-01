# Task 3.0: Update Configuration Registry

## status: completed

<task_context>
<phase>Phase 1: Core Configuration</phase>
<priority>CRITICAL</priority>
<complexity>Medium</complexity>
<estimated_duration>1 day</estimated_duration>
</task_context>

---

## Objective

Update field definitions in `pkg/config/definition/schema.go` to register new mode defaults, help text, and metadata for the configuration system.

**Impact**: Ensures CLI flags, environment variables, and config help text reflect the new three-mode system.

---

<critical>
**MANDATORY VALIDATION:**
- Run `go test ./pkg/config/definition -v` - MUST PASS
- Run `make lint` on pkg/config/definition - MUST BE CLEAN
- Verify CLI help text shows correct modes

**BREAKING CHANGE:**
- Default mode changes from "distributed" to "memory" in registry
- Help text updated for all mode fields

**YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

---

<requirements>

### Registry Changes

**Global Mode Field:**
- Default: `"memory"` (changed from `"distributed"`)
- Help: Updated to explain all three modes

**Component Mode Fields:**
- `temporal.mode`: Help text updated for memory/persistent/remote
- `redis.mode`: Help text updated for memory/persistent/distributed
- Both inherit from global mode if unset (empty default)

**CLI Integration:**
- Flags show correct help text
- Environment variables documented
- Default values correct

</requirements>

---

<research>
**Implementation Reference**: See `_techspec.md` Section "Phase 1.3: Update Configuration Registry" (lines 421-464)

**Key Concepts:**
- Configuration field registry pattern
- CLI flag generation from registry
- Environment variable mapping
- Help text generation

**Related Files:**
- `pkg/config/definition/schema.go` - Field registry
- `pkg/config/resolver.go` - Mode constants (Task 1.0)
- `pkg/config/config.go` - Config struct (Task 2.0)
</research>

---

## Subtasks

### 3.1 Update Global Mode Registration
**File**: `pkg/config/definition/schema.go` (~line 733)

- [x] Change `Default` from `"distributed"` to `"memory"`
- [x] Update `Help` text to explain all three modes
- [x] Verify `CLIFlag` is `"mode"`
- [x] Verify `EnvVar` is `"COMPOZY_MODE"`

**Reference**: `_techspec.md` lines 426-434

---

### 3.2 Update Temporal Mode Registration
**File**: `pkg/config/definition/schema.go`

- [x] Verify `Default` is `""` (empty = inherit from global)
- [x] Update `Help` text: "Temporal deployment mode (memory/persistent/remote), inherits from global mode if unset"
- [x] Verify `CLIFlag` is `"temporal-mode"`
- [x] Verify `EnvVar` is `"TEMPORAL_MODE"`

**Reference**: `_techspec.md` lines 437-446

---

### 3.3 Update Redis Mode Registration
**File**: `pkg/config/definition/schema.go`

- [x] Verify `Default` is `""` (empty = inherit from global)
- [x] Update `Help` text: "Redis deployment mode (memory/persistent/distributed), inherits from global mode if unset"
- [x] Verify `CLIFlag` is `"redis-mode"`
- [x] Verify `EnvVar` is `"REDIS_MODE"`

**Reference**: `_techspec.md` lines 449-458

---

## Relevant Files

### Primary Files (Modified)
- `pkg/config/definition/schema.go` - Field registry

### Dependent Files (Reference Only)
- `pkg/config/resolver.go` - Mode constants (Task 1.0)
- `pkg/config/config.go` - Config struct (Task 2.0)
- `cli/help/global-flags.md` - Will be updated in Task 18.0

### Generated Files (May Need Refresh)
- CLI help text (auto-generated from registry)
- Environment variable documentation

---

## Deliverables

1. **Updated Global Mode Registration**
   - Default changed to "memory"
   - Help text explains all three modes
   - CLI flag and env var correct

2. **Updated Component Mode Registrations**
   - Temporal mode help text updated
   - Redis mode help text updated
   - Both inherit from global (empty default)

3. **Verified CLI Integration**
   - `compozy --help` shows correct mode help
   - Environment variables documented correctly

4. **Tests Pass**
   - Definition tests pass
   - No lint errors

---

## Tests

### Unit Tests to Verify
**File**: `pkg/config/definition/*_test.go`

Expected behavior:
- Mode field registered with correct default
- Mode field has correct help text
- CLI flags map to correct config paths
- Environment variables map correctly

### Validation Commands
```bash
# Must pass before completing task
go test ./pkg/config/definition -v
make lint

# Manual verification
compozy --help | grep -A 5 "mode"
# Should show updated help text with memory/persistent/distributed
```

---

## Success Criteria

- [x] Global mode field default changed to "memory"
- [x] Global mode help text updated for three modes
- [x] Temporal mode help text updated
- [x] Redis mode help text updated
- [x] All CLI flags correct
- [x] All environment variables correct
- [x] Code compiles without errors
- [x] No lint warnings or errors
- [x] Tests pass
- [x] CLI help shows correct information

---

## Dependencies

**Blocks:**
- Task 4.0 (Configuration Tests) - needs registry updates

**Depends On:**
- Task 1.0 (Mode Constants) - needs new mode definitions
- Task 2.0 (Configuration Validation) - needs struct updates

---

## Notes

- Registry changes affect CLI help output
- Default value change is BREAKING for alpha users
- Implementation details in `_techspec.md` lines 421-464
- Keep changes focused on schema.go only

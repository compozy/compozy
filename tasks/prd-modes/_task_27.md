# Task 27.0: Add Mode Selection to TUI Form

<task_context>
<phase>Phase 5: Template System</phase>
<priority>CRITICAL - First Impression</priority>
<complexity>Medium</complexity>
<estimated_duration>0.5 days</estimated_duration>
</task_context>

---

## Objective

Add a mode selection dropdown to the `compozy init` TUI form, allowing users to choose between memory, persistent, and distributed modes during project initialization.

**Impact**: CRITICAL - This is the user's first interaction with the mode system.

---

<critical>
**MANDATORY VALIDATION:**
- Run `go build ./cli` - MUST COMPILE
- Run `compozy init` - TUI form MUST show mode dropdown
- Select each mode - Generated project MUST use selected mode
- Run `make lint` - MUST BE CLEAN

**USER EXPERIENCE:**
- Mode dropdown appears AFTER template selection
- Default selection: "memory"
- Clear help text for each mode
- Visual indicators: 🚀 memory, 💾 persistent, 🏭 distributed
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

---

<requirements>

### TUI Form Changes

**Location**: `cli/cmd/init/components/project_form.go`

**New Field**: Mode dropdown
- Position: After template selection, before Docker toggle
- Options: memory (🚀), persistent (💾), distributed (🏭)
- Default: memory
- Help text: Clear use case guidance

**Docker Toggle Behavior**:
- Disabled for memory and persistent modes
- Enabled only for distributed mode
- Help text: "Distributed mode requires external services"

</requirements>

---

<research>
**Reference**: `tasks/prd-modes/TEMPLATE_SYSTEM_ANALYSIS.md`

**Current TUI Structure** (`cli/cmd/init/components/project_form.go`):
- Text fields: Name, Description, Version, Author, Author URL
- Dropdown: Template selection (currently only "basic")
- Toggle: Include Docker configuration

**Required Changes**:
1. Add mode dropdown field
2. Add mode help text
3. Conditional Docker toggle (disabled for memory/persistent)
4. Update form model to store mode selection
</research>

---

## Subtasks

### 27.1 Add Mode Field to Form Model
**File**: `cli/cmd/init/components/init_model.go`

- [ ] Add `Mode string` field to form model struct
- [ ] Initialize mode to "memory" (default)
- [ ] Add mode getter/setter methods
- [ ] Add mode to form data output

---

### 27.2 Create Mode Dropdown Component
**File**: `cli/cmd/init/components/project_form.go`

- [ ] Add mode dropdown after template selection
- [ ] Options: memory, persistent, distributed
- [ ] Default selected: memory
- [ ] Visual indicators: 🚀 memory, 💾 persistent, 🏭 distributed

**Help Text**:
```
Memory Mode (🚀):
- Zero dependencies, instant startup
- Perfect for tests and quick prototyping
- No persistence (data lost on restart)

Persistent Mode (💾):
- File-based storage, state preserved
- Ideal for local development
- Still zero external dependencies

Distributed Mode (🏭):
- External PostgreSQL, Redis, Temporal
- Production-ready, horizontal scaling
- Requires Docker or managed services
```

---

### 27.3 Conditional Docker Toggle
**File**: `cli/cmd/init/components/project_form.go`

- [ ] Disable Docker toggle when mode is memory or persistent
- [ ] Enable Docker toggle only when mode is distributed
- [ ] Update Docker toggle help text based on mode
- [ ] Gray out Docker toggle when disabled (visual feedback)

**Conditional Logic**:
```go
// Docker toggle only enabled for distributed mode
dockerToggle.Disabled = (mode != "distributed")

// Update help text
if mode == "distributed" {
    dockerToggle.Help = "Generate docker-compose.yaml for external services"
} else {
    dockerToggle.Help = "Docker not needed for embedded mode"
}
```

---

### 27.4 Update Form Rendering
**File**: `cli/cmd/init/components/project_form.go`

- [ ] Add mode dropdown to form layout
- [ ] Position after template, before Docker toggle
- [ ] Update form navigation (tab order)
- [ ] Add mode validation (must be one of three values)

---

### 27.5 Pass Mode to Template Generator
**File**: `cli/cmd/init/init.go`

- [ ] Extract mode from form data
- [ ] Pass mode to `GenerateOptions` struct (Task 28.0 will add this field)
- [ ] Log selected mode for debugging
- [ ] Validate mode before template generation

---

## Relevant Files

### Primary Files (Modified)
- `cli/cmd/init/components/project_form.go` - TUI form UI
- `cli/cmd/init/components/init_model.go` - Form data model
- `cli/cmd/init/init.go` - Command handler

### Dependent Files (Reference Only)
- `pkg/template/types.go` - Will be updated in Task 28.0
- `pkg/template/templates/basic/basic.go` - Will be updated in Task 29.0

---

## Deliverables

1. **Mode Dropdown Added**
   - Three options: memory, persistent, distributed
   - Default: memory
   - Visual indicators and help text

2. **Conditional Docker Toggle**
   - Disabled for memory/persistent
   - Enabled only for distributed
   - Clear visual feedback

3. **Form Model Updated**
   - Mode field added
   - Mode passed to template generator
   - Mode validated

4. **User Experience**
   - Clear mode selection guidance
   - Intuitive form flow
   - No confusion about options

---

## Tests

### Manual Testing
```bash
# Test mode selection
go build ./cli
./compozy init test-project

# Verify:
# 1. Mode dropdown appears after template selection
# 2. Default is "memory"
# 3. All three modes selectable
# 4. Docker toggle disabled for memory/persistent
# 5. Docker toggle enabled for distributed
# 6. Help text clear and accurate

# Test each mode
./compozy init memory-test --mode memory
./compozy init persistent-test --mode persistent
./compozy init distributed-test --mode distributed

# Verify generated compozy.yaml uses correct mode
```

### Validation Commands
```bash
# Must pass before completing task
go build ./cli
make lint

# Manual smoke test
./compozy init test-project
# Select mode, verify generation
```

---

## Success Criteria

- [x] Mode dropdown added to TUI form
- [x] Mode dropdown positioned correctly (after template, before Docker)
- [x] Default mode is "memory"
- [x] All three modes selectable
- [x] Help text clear for each mode
- [x] Visual indicators show mode characteristics
- [x] Docker toggle disabled for memory/persistent
- [x] Docker toggle enabled for distributed
- [x] Mode passed to template generator
- [x] Code compiles without errors
- [x] No lint warnings or errors

---

## Dependencies

**Blocks:**
- Task 29.0 (Mode-Aware Template Generation) - needs mode selection

**Depends On:**
- Phase 1 complete (mode constants defined)

**Parallel With:**
- Task 28.0 (Update Template System Types) - can run in parallel

---

## Notes

- This is the **first user touchpoint** for the mode system
- Clear, helpful UX is critical for adoption
- Default to memory mode emphasizes zero-dependency experience
- Implementation reference in `TEMPLATE_SYSTEM_ANALYSIS.md`
- Keep changes focused on TUI form only

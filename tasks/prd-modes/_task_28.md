# Task 28.0: Update Template System Types for Mode

<task_context>
<phase>Phase 5: Template System</phase>
<priority>HIGH</priority>
<complexity>Low</complexity>
<estimated_duration>0.5 days</estimated_duration>
</task_context>

---

## Objective

Add `Mode` field to `GenerateOptions` struct in the template system, enabling mode-aware template generation.

**Impact**: Enables template system to generate mode-specific configurations.

---

<critical>
**MANDATORY VALIDATION:**
- Run `go build ./pkg/template` - MUST COMPILE
- Run `make lint` on pkg/template - MUST BE CLEAN
- Run `go test ./pkg/template/... -v` - ALL TESTS MUST PASS

**BREAKING CHANGE:**
- `GenerateOptions` struct signature changes
- All template generators must handle mode field
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

---

<requirements>

### Type System Changes

**File**: `pkg/template/types.go`

**Add Mode Field**:
```go
type GenerateOptions struct {
    OutputDir      string
    Name           string
    Description    string
    Version        string
    Author         string
    AuthorURL      string
    IncludeDocker  bool
    Mode           string  // NEW: memory, persistent, or distributed
}
```

**Default Value**: "memory" (zero-dependency default)

</requirements>

---

<research>
**Reference**: `tasks/prd-modes/TEMPLATE_SYSTEM_ANALYSIS.md`

**Current Structure** (`pkg/template/types.go`):
- `Template` interface with `Generate(opts GenerateOptions) error` method
- `GenerateOptions` struct with project metadata fields
- Global registry for template registration

**Required Changes**:
1. Add Mode field to GenerateOptions
2. Update all template implementations to read Mode field
3. Validate mode value (memory/persistent/distributed)
</research>

---

## Subtasks

### 28.1 Add Mode Field to GenerateOptions
**File**: `pkg/template/types.go`

- [ ] Add `Mode string` field to `GenerateOptions` struct
- [ ] Add field comment documenting valid values
- [ ] Position after `IncludeDocker` field
- [ ] Update struct documentation

**Implementation**:
```go
// GenerateOptions contains configuration for project generation
type GenerateOptions struct {
    OutputDir     string // Target directory for generated files
    Name          string // Project name
    Description   string // Project description
    Version       string // Initial version (e.g., "0.1.0")
    Author        string // Author name
    AuthorURL     string // Author URL or email
    IncludeDocker bool   // Generate docker-compose.yaml
    Mode          string // Deployment mode: memory, persistent, or distributed
}
```

---

### 28.2 Add Mode Validation Function
**File**: `pkg/template/types.go`

- [ ] Add `ValidateMode(mode string) error` function
- [ ] Validate mode is one of: memory, persistent, distributed
- [ ] Return helpful error for invalid modes
- [ ] Suggest correction for "standalone" mode

**Implementation**:
```go
// ValidateMode checks if the provided mode is valid
func ValidateMode(mode string) error {
    validModes := []string{"memory", "persistent", "distributed"}

    for _, valid := range validModes {
        if mode == valid {
            return nil
        }
    }

    // Provide helpful error for "standalone"
    if mode == "standalone" {
        return fmt.Errorf("mode 'standalone' has been replaced. Use 'memory' (no persistence) or 'persistent' (with persistence)")
    }

    return fmt.Errorf("invalid mode '%s'. Must be one of: %s", mode, strings.Join(validModes, ", "))
}
```

---

### 28.3 Add Default Mode Constant
**File**: `pkg/template/types.go`

- [ ] Add `DefaultMode = "memory"` constant
- [ ] Use constant in validation and defaults
- [ ] Document constant purpose

**Implementation**:
```go
const (
    // DefaultMode is the default deployment mode for new projects
    DefaultMode = "memory"
)
```

---

### 28.4 Update Template Interface Documentation
**File**: `pkg/template/types.go`

- [ ] Update `Template` interface comments
- [ ] Document mode field requirement
- [ ] Add example usage

**Implementation**:
```go
// Template represents a project template that can generate files
// Implementations must:
// - Generate mode-appropriate compozy.yaml (memory/persistent/distributed)
// - Conditionally generate docker-compose.yaml (distributed mode only)
// - Create mode-specific README documentation
type Template interface {
    Name() string
    Description() string
    Generate(opts GenerateOptions) error
}
```

---

### 28.5 Update Service Layer
**File**: `pkg/template/service.go`

- [ ] Add mode validation in service layer
- [ ] Set default mode if not provided
- [ ] Log selected mode

**Implementation**:
```go
func (s *Service) Generate(name string, opts GenerateOptions) error {
    // Set default mode if not provided
    if opts.Mode == "" {
        opts.Mode = DefaultMode
    }

    // Validate mode
    if err := ValidateMode(opts.Mode); err != nil {
        return fmt.Errorf("invalid mode: %w", err)
    }

    // Log selected mode
    log.Info("Generating project with mode", "mode", opts.Mode, "template", name)

    // ... rest of implementation
}
```

---

## Relevant Files

### Primary Files (Modified)
- `pkg/template/types.go` - Add Mode field and validation
- `pkg/template/service.go` - Add mode validation and logging

### Dependent Files (Reference Only)
- `pkg/template/templates/basic/basic.go` - Will use Mode field (Task 29.0)
- `cli/cmd/init/init.go` - Will pass Mode from form (Task 27.0)

---

## Deliverables

1. **Mode Field Added**
   - GenerateOptions has Mode field
   - Mode field documented
   - Mode field validated

2. **Validation Function**
   - ValidateMode function added
   - Helpful error messages
   - "standalone" migration hint

3. **Default Mode**
   - DefaultMode constant added
   - Default applied when Mode empty
   - Default is "memory"

4. **Service Layer Updated**
   - Mode validated before generation
   - Default mode applied
   - Mode logged for debugging

---

## Tests

### Unit Tests to Add
**File**: `pkg/template/types_test.go`

```go
func TestValidateMode(t *testing.T) {
    tests := []struct {
        name    string
        mode    string
        wantErr bool
        errMsg  string
    }{
        {
            name:    "memory mode valid",
            mode:    "memory",
            wantErr: false,
        },
        {
            name:    "persistent mode valid",
            mode:    "persistent",
            wantErr: false,
        },
        {
            name:    "distributed mode valid",
            mode:    "distributed",
            wantErr: false,
        },
        {
            name:    "standalone rejected with hint",
            mode:    "standalone",
            wantErr: true,
            errMsg:  "has been replaced",
        },
        {
            name:    "invalid mode rejected",
            mode:    "invalid",
            wantErr: true,
            errMsg:  "invalid mode",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateMode(tt.mode)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateMode() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if err != nil && !strings.Contains(err.Error(), tt.errMsg) {
                t.Errorf("ValidateMode() error = %v, want message containing %v", err, tt.errMsg)
            }
        })
    }
}
```

### Validation Commands
```bash
# Must pass before completing task
go test ./pkg/template/... -v
make lint
go build ./pkg/template
```

---

## Success Criteria

- [x] Mode field added to GenerateOptions
- [x] ValidateMode function implemented
- [x] DefaultMode constant added
- [x] Service layer validates mode
- [x] Service layer applies default mode
- [x] Mode validation tests added
- [x] All tests pass
- [x] Code compiles without errors
- [x] No lint warnings or errors

---

## Dependencies

**Blocks:**
- Task 29.0 (Mode-Aware Template Generation) - needs Mode field

**Depends On:**
- Phase 1 complete (mode constants defined)

**Parallel With:**
- Task 27.0 (TUI Form) - can run in parallel

---

## Notes

- Simple structural change, low risk
- Foundation for mode-aware template generation
- Validation ensures type safety
- Implementation reference in `TEMPLATE_SYSTEM_ANALYSIS.md`
- Keep changes focused on types.go and service.go only

# Compozy Template System Analysis

## Current State Summary

### Template System Architecture

**Location**: `pkg/template/`
- **Types Definition**: `pkg/template/types.go` - Interface-based template system
- **Registry Pattern**: `pkg/template/registry.go` - Global singleton registry
- **Service Layer**: `pkg/template/service.go` - Service interface with singleton instance
- **Generator**: `pkg/template/generator.go` - Template rendering and file creation
- **Basic Template**: `pkg/template/templates/basic/basic.go` - Only existing template implementation

**Current Templates Available**: 1 (basic only)

### Basic Template Structure

**Location**: `pkg/template/templates/basic/`

**Files Generated**:
- `compozy.yaml` - Project config file
- `entrypoint.ts` - Runtime entry point (Bun)
- `workflows/main.yaml` - Example workflow
- `greeting_tool.ts` - Example tool
- `api.http` - API testing file
- `env.example` - Environment template
- `.gitignore` - Git ignore file
- `README.md` - Project README
- `docker-compose.yaml` - (Optional, when `--docker` flag used)

**Configuration Structure** (`projectConfig` struct in basic.go):
```go
type projectConfig struct {
	Name        string            // Project name
	Version     string            // Version
	Description string            // Description
	Author      *authorConfig     // Author info
	Workflows   []workflowRef     // Workflow references
	Models      []modelConfig     // LLM model config
	Runtime     *runtimeConfig    // Runtime config (type, entrypoint, perms)
	Autoload    *autoloadConfig   // Autoload settings
	Templates   map[string]string // Custom templates
}
```

### Current Mode Configuration in Templates

**In basic template** (`baseProjectConfig` function, lines 156-194):
```go
Models: []modelConfig{
	{
		Provider: "openai",
		Model:    "gpt-4.1-2025-04-14",
		APIKey:   "{{ .env.OPENAI_API_KEY }}",
	},
},
Runtime: &runtimeConfig{
	Type:       "bun",
	Entrypoint: "./entrypoint.ts",
	Permissions: []string{
		"--allow-read",
		"--allow-net",
		"--allow-write",
	},
},
Autoload: &autoloadConfig{
	Enabled: true,
	Strict:  true,
	Include: []string{
		"agents/*.yaml",
		"tools/*.yaml",
	},
	Exclude: []string{
		"**/*~",
		"**/*.bak",
		"**/*.tmp",
	},
},
```

**Key Finding**: No mode-specific configuration in template. Template generates generic project without mode selection.

### CLI/TUI Integration

**Location**: `cli/cmd/init/`

**Init Command**: `cli/cmd/init/init.go`
- Default template: `"basic"` (line 83)
- `--template` flag allows override (line 83)
- Docker support: `--docker` flag for docker-compose.yaml generation
- Two execution modes: JSON (non-interactive) and TUI (interactive)

**Project Form**: `cli/cmd/init/components/project_form.go`
- **Template Selection** (lines 68-72):
  ```go
  huh.NewSelect[string]().
      Title("Template").
      Description("Project template to use").
      Options(huh.NewOption("Basic", "basic")).  // ← ONLY "basic" option
      Value(&data.Template),
  ```
  
**Key Finding**: Template selection dropdown only shows "Basic" option. No mode selection in init flow.

**Init Model**: `cli/cmd/init/components/init_model.go`
- Handles form rendering and validation
- Displays header, collects form data, manages viewport
- No mode-specific questions or logic

**Key Finding**: Interactive form doesn't ask about mode at all.

### Docker Compose Configuration

**Location**: `pkg/template/templates/basic/docker-compose.yaml.tmpl`

**Services Configured** (when `--docker` flag used):
1. Redis (6379) - for caching
2. PostgreSQL (5432) - application database
3. PostgreSQL (5433) - Temporal database
4. Temporal (7233) - workflow engine
5. Temporal UI (8080) - web interface

**Key Finding**: Docker compose is **distributed mode only** - sets up external PostgreSQL and Temporal. No option for embedded services.

---

## Three-Mode System Requirements

Based on `tasks/prd-modes/_techspec.md`, the system needs:

### Mode Definitions

1. **memory** (NEW DEFAULT)
   - In-memory SQLite (no persistence)
   - Embedded Temporal
   - Embedded Redis (miniredis)
   - Use case: Development, testing, demos
   - No config file needed - works out of box

2. **persistent** (NEW)
   - File-based SQLite (with persistence)
   - Embedded Temporal
   - Embedded Redis with BadgerDB persistence
   - Use case: Local development with state preservation
   - Requires `compozy.yaml` with mode setting

3. **distributed** (EXISTING, renamed)
   - External PostgreSQL
   - External Temporal
   - External Redis
   - Use case: Production
   - Requires full infrastructure setup

### Configuration Points Needed

From tech spec, templates need to configure:
- Database driver selection (SQLite vs PostgreSQL)
- Temporal mode (embedded vs external)
- Cache/Redis mode (embedded vs external)
- Default generated workflows/examples
- Docker compose for distributed mode only

---

## Impact Analysis: What Needs Changing

### 1. Template System Changes

**Current**: 1 template (basic) with no mode awareness
**Needed**: 1 template or 3 templates with mode selection

**Options**:
- **Option A**: Single "basic" template that generates mode-specific config based on new GenerateOptions field
- **Option B**: Three templates: "basic-memory", "basic-persistent", "basic-distributed"
- **Recommended**: Option A - maintains simplicity, backward compatible at API level

**Files to Modify**:
- `pkg/template/types.go` - Add `Mode` field to `GenerateOptions`
- `pkg/template/templates/basic/basic.go` - Add mode-aware config generation
- `pkg/template/templates/basic/compozy.yaml.tmpl` - Make database/cache config mode-dependent
- `pkg/template/templates/basic/docker-compose.yaml.tmpl` - Only include for distributed mode

### 2. CLI/TUI Changes

**Current**: No mode selection in init flow
**Needed**: Mode selection in interactive form

**Files to Modify**:
- `cli/cmd/init/init.go` - Add mode option, pass to template
- `cli/cmd/init/components/project_form.go` - Add mode selection dropdown
- `cli/cmd/init/components/init_model.go` - Handle mode in form data

**New Field in ProjectFormData**:
```go
type ProjectFormData struct {
	// ... existing fields ...
	Mode string // "memory", "persistent", or "distributed"
}
```

### 3. Template Content Changes

**compozy.yaml.tmpl**:
- Conditional cache configuration based on mode
- Database driver selection based on mode
- Remove unnecessary Temporal config for embedded modes

**docker-compose.yaml.tmpl**:
- Only generate for distributed mode (or move to separate template)
- Update to use external services

**env.example.tmpl**:
- Different variables based on mode
- Only Temporal vars for distributed mode

### 4. Generated Project Differences

**Memory Mode**:
```yaml
# No explicit cache.adapter - uses default (memory)
# No explicit temporal.mode - uses default (embedded with :memory:)
database:
  driver: sqlite
  file: :memory:
```

**Persistent Mode**:
```yaml
database:
  driver: sqlite
  file: ./compozy.db
```

**Distributed Mode**:
```yaml
database:
  driver: postgres
  host: localhost
  port: 5432
cache:
  adapter: redis  # external
temporal:
  mode: remote
  address: localhost:7233
```

---

## Current Template Flow

```
CLI user runs: compozy init my-project
    ↓
init.go: prepareInitOptions()
    ↓
init.go: runInitTUI() or runInitJSON()
    ↓
components/init_model.go: NewInitModel(projectData)
    ↓
components/project_form.go: NewProjectForm(data)
    ↓
User selects: Name, Description, Version, Author, Template(="basic"), Docker(yes/no)
    ↓
ProjectFormData captured
    ↓
init.go: generateProjectStructure(opts)
    ↓
template.GetService().Generate("basic", generateOptions)
    ↓
registry.go: Get("basic") → basic.Template{}
    ↓
basic.go: GetProjectConfig() → generates compozy.yaml content
    ↓
generator.go: createFile() → renders templates to disk
```

---

## What Exists vs What's Missing

### Exists ✅
- Robust template registry system
- Template rendering with sprig functions
- TUI-based form for project initialization
- Docker compose option
- Basic template with multiple files

### Missing ❌
- Mode selection in init form
- Mode-aware template generation
- Database driver defaults based on mode
- Cache/temporal configuration by mode
- Mode-specific docker compose (only distributed)
- Mode documentation in generated README

---

## Files That Will Change

### Phase 1: Configuration (2 files - template system)
1. `pkg/template/types.go` - Add Mode to GenerateOptions
2. `pkg/template/templates/basic/basic.go` - Mode-aware config generation

### Phase 2: CLI/TUI (3 files)
3. `cli/cmd/init/init.go` - Pass mode to template
4. `cli/cmd/init/components/project_form.go` - Add mode selection field
5. `cli/cmd/init/components/init_model.go` - Handle mode field

### Phase 3: Templates (4 files)
6. `pkg/template/templates/basic/compozy.yaml.tmpl` - Database/cache config
7. `pkg/template/templates/basic/env.example.tmpl` - Mode-specific vars
8. `pkg/template/templates/basic/docker-compose.yaml.tmpl` - Conditionally included
9. `pkg/template/templates/basic/README.md.tmpl` - Mode documentation

### Phase 4: Testing (documentation only for now)
- Integration tests will be covered in separate phase (task 11.0 in PRD)

---

## Key Decisions to Make

### 1. Default Mode
**Decision**: memory mode
**Rationale**: Zero-dependency quickstart, fits dev/testing use case
**Impact**: Changes default generated config significantly

### 2. Mode Selection UI
**Decision**: Add dropdown after template selection
**Order**: Name → Description → Version → Author → URL → Template → **Mode** ← NEW → Docker → Bun
**Visibility**: Always shown (not conditional)

### 3. Docker Compose Behavior
**Current**: Generated when --docker flag used
**Change**: Only generate for distributed mode
**Impact**: Memory/persistent modes won't have docker-compose.yaml

### 4. Configuration in Generated YAML
**Decision**: Explicit mode settings in generated compozy.yaml
```yaml
mode: memory  # or persistent/distributed
```
**Rationale**: Clear intent, easier debugging, runtime can validate

---

## Risk Areas

### High Risk
1. **Template rendering changes**: If mode logic breaks, all new projects broken
2. **Docker compose conditional**: Need to verify docker-compose generation only for distributed
3. **Form field ordering**: Changes UX, need to validate in TUI

### Medium Risk
1. **Backward compatibility**: Old code expecting always-docker setup
2. **Test coverage**: New mode paths need test coverage
3. **Documentation**: User confusion if migration docs not clear

### Low Risk
1. **CLI flag changes**: No breaking flag changes, purely addition
2. **Registry changes**: Interface remains same, only options expanded

---

## Summary Table

| Component | Current | Needed | Status |
|-----------|---------|--------|--------|
| Templates | 1 (basic) | 1 (basic + mode-aware) | Needs mode logic |
| CLI Mode Selection | None | Dropdown in form | Needs UI |
| Config Generation | Generic | Mode-specific | Needs logic |
| Docker Compose | Always (if --docker) | Distributed only | Needs conditional |
| Database Config | Generic | Mode-dependent | Needs logic |
| Cache Config | Not in template | Mode-dependent | Needs addition |
| Documentation | Generic | Mode examples | Needs updates |

---

## Recommended Next Steps

1. **Update GenerateOptions** (5 min)
   - Add Mode field to pkg/template/types.go

2. **Update Basic Template** (1 hour)
   - Add mode-aware config in basic.go
   - Update compozy.yaml.tmpl with conditional logic
   - Update env.example.tmpl with mode-specific vars
   - Make docker-compose conditional

3. **Update CLI Form** (1.5 hours)
   - Add mode field to ProjectFormData
   - Add dropdown in project_form.go
   - Wire through init.go to GenerateOptions

4. **Testing** (2+ hours)
   - Verify each mode generates correct config
   - Test form with each mode selection
   - Validate docker-compose only in distributed

5. **Documentation** (1+ hours)
   - Update generated README.md template
   - Add mode notes to each generated project

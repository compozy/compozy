# Task 29.0: Make Template Generation Mode-Aware

## status: pending

<task_context>
<phase>Phase 5: Template System</phase>
<priority>CRITICAL - User Onboarding</priority>
<complexity>High</complexity>
<estimated_duration>1 day</estimated_duration>
</task_context>

---

## Objective

Update the "basic" template to generate mode-appropriate configuration files, with docker-compose.yaml only for distributed mode and mode-specific documentation.

**Impact**: CRITICAL - Ensures generated projects work out-of-the-box in selected mode.

---

<critical>
**MANDATORY VALIDATION:**
- Run `go build ./pkg/template` - MUST COMPILE
- Generate project in each mode - MUST WORK
- Run `make lint` - MUST BE CLEAN
- Run `make test` - MUST PASS

- **DO NOT RUN TUI COMMANDS OR BLOCK COMMANDS TO AVOID GET YOUR EXECUTION BLOCKED**

**USER EXPERIENCE:**
- Memory mode: Minimal config, no docker-compose, instant startup
- Persistent mode: File paths configured, no docker-compose, state persists
- Distributed mode: External services configured, docker-compose included
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

---

<requirements>

### Template File Changes

**Files to Update**:
1. `pkg/template/templates/basic/compozy.yaml.tmpl` - Mode-specific config
2. `pkg/template/templates/basic/docker-compose.yaml.tmpl` - Conditional generation
3. `pkg/template/templates/basic/README.md.tmpl` - Mode-specific docs
4. `pkg/template/templates/basic/env.example.tmpl` - Mode-specific env vars
5. `pkg/template/templates/basic/basic.go` - Generation logic

**Mode-Specific Behavior**:
- **Memory**: Minimal config, no docker-compose, quick start docs
- **Persistent**: File paths, no docker-compose, state preservation docs
- **Distributed**: External services, docker-compose, production docs

</requirements>

---

<research>
**Reference**: `tasks/prd-modes/TEMPLATE_SYSTEM_ANALYSIS.md`

**Current Template Structure**:
- 8 embedded files using `//go:embed`
- Template rendering with sprig functions
- Docker compose always generated when `IncludeDocker` is true

**Required Changes**:
1. Add mode-aware template logic to compozy.yaml.tmpl
2. Conditional docker-compose generation (distributed mode only)
3. Mode-specific README sections
4. Mode-specific environment variables
</research>

---

## Subtasks

### 29.1 Update compozy.yaml Template
**File**: `pkg/template/templates/basic/compozy.yaml.tmpl`

- [ ] Add mode-specific configuration sections
- [ ] Memory mode: Minimal config, explicit :memory: database
- [ ] Persistent mode: File paths for database and persistence
- [ ] Distributed mode: External service placeholders with env vars
- [ ] Add comments explaining mode-specific settings

**Implementation**:
```yaml
# {{ .Name }} - Generated with Compozy
name: {{ .Name }}
version: {{ .Version }}
description: {{ .Description }}

# Deployment Mode: {{ .Mode }}
{{- if eq .Mode "memory" }}
# Memory mode - zero dependencies, instant startup, no persistence
mode: memory

database:
  driver: sqlite
  url: ":memory:"

temporal:
  mode: memory
  namespace: {{ .Name }}-dev

redis:
  mode: memory
  # Embedded miniredis, no persistence

{{- else if eq .Mode "persistent" }}
# Persistent mode - file-based storage, state preserved
mode: persistent

database:
  driver: sqlite
  url: ./.compozy/{{ .Name }}.db

temporal:
  mode: persistent
  namespace: {{ .Name }}-dev
  standalone:
    database_file: ./.compozy/temporal.db

redis:
  mode: persistent
  standalone:
    persistence:
      enabled: true
      dir: ./.compozy/redis

{{- else if eq .Mode "distributed" }}
# Distributed mode - production deployment with external services
mode: distributed

database:
  driver: postgres
  url: ${COMPOZY_DATABASE_URL}
  pool:
    max_open_conns: 25
    max_idle_conns: 5

temporal:
  mode: remote
  host_port: ${TEMPORAL_HOST_PORT}
  namespace: ${TEMPORAL_NAMESPACE}

redis:
  mode: distributed
  distributed:
    addr: ${REDIS_ADDR}
    password: ${REDIS_PASSWORD}
    db: 0

{{- end }}

# Agent configuration (same for all modes)
agents:
  main:
    entrypoint: ./src/entrypoint.ts
    tools:
      - name: echo
        type: builtin

# Server configuration
server:
  host: localhost
  port: 8080
  log_level: info
```

---

### 29.2 Conditional Docker Compose Generation
**File**: `pkg/template/templates/basic/basic.go`

- [ ] Check if mode is "distributed" before generating docker-compose
- [ ] Skip docker-compose.yaml for memory and persistent modes
- [ ] Update file generation logic
- [ ] Log skipped files for transparency

**Implementation**:
```go
func (t *BasicTemplate) Generate(opts GenerateOptions) error {
    // ... existing file generation ...

    // Only generate docker-compose for distributed mode
    if opts.Mode == "distributed" && opts.IncludeDocker {
        dockerComposePath := filepath.Join(opts.OutputDir, "docker-compose.yaml")
        if err := t.renderTemplate("docker-compose.yaml.tmpl", dockerComposePath, opts); err != nil {
            return fmt.Errorf("failed to generate docker-compose.yaml: %w", err)
        }
        log.Info("Generated docker-compose.yaml for distributed mode")
    } else {
        log.Info("Skipping docker-compose.yaml (not needed for %s mode)", opts.Mode)
    }

    return nil
}
```

---

### 29.3 Update README Template
**File**: `pkg/template/templates/basic/README.md.tmpl`

- [ ] Add mode-specific quick start sections
- [ ] Memory mode: Instant startup instructions
- [ ] Persistent mode: State preservation notes
- [ ] Distributed mode: Docker setup instructions
- [ ] Add mode switching guide

**Implementation**:
```markdown
# {{ .Name }}

{{ .Description }}

**Mode:** {{ .Mode }}

## Quick Start

{{- if eq .Mode "memory" }}

### Memory Mode (Zero Dependencies)

Start the server instantly with no external dependencies:

\```bash
compozy start
\```

Server ready in <1 second!

**Note:** All data is stored in memory and lost on restart.

{{- else if eq .Mode "persistent" }}

### Persistent Mode (Local Development)

Start the server with state preservation:

\```bash
compozy start
\```

Data is saved to `./.compozy/` directory and persists between restarts.

{{- else if eq .Mode "distributed" }}

### Distributed Mode (Production)

Start external services first:

\```bash
# Start infrastructure
docker-compose up -d

# Start Compozy
export COMPOZY_DATABASE_URL="postgresql://..."
export TEMPORAL_HOST_PORT="localhost:7233"
export REDIS_ADDR="localhost:6379"
compozy start
\```

{{- end }}

## Switching Modes

To switch to a different mode:

1. Update `mode` in `compozy.yaml`
2. Restart the server: `compozy restart`

### Available Modes

- **memory**: Zero dependencies, fastest startup, no persistence
- **persistent**: File-based storage, state preserved
- **distributed**: External services, production-ready

## Development

[... rest of README ...]
```

---

### 29.4 Update Environment Variables Template
**File**: `pkg/template/templates/basic/env.example.tmpl`

- [ ] Add mode-specific environment variables
- [ ] Memory mode: Minimal env vars
- [ ] Persistent mode: Optional data directory overrides
- [ ] Distributed mode: Required external service connections

**Implementation**:
```bash
# {{ .Name }} - Environment Variables

# Global Configuration
COMPOZY_MODE={{ .Mode }}
COMPOZY_LOG_LEVEL=info
COMPOZY_SERVER_PORT=8080

{{- if eq .Mode "memory" }}

# Memory Mode - No additional configuration needed!
# All services embedded, all data in-memory

{{- else if eq .Mode "persistent" }}

# Persistent Mode - Optional overrides
# COMPOZY_DATABASE_URL=./.compozy/{{ .Name }}.db
# TEMPORAL_DATABASE_FILE=./.compozy/temporal.db
# REDIS_PERSISTENCE_DIR=./.compozy/redis

{{- else if eq .Mode "distributed" }}

# Distributed Mode - External Services (REQUIRED)
COMPOZY_DATABASE_URL=postgresql://user:password@localhost:5432/{{ .Name }}
TEMPORAL_HOST_PORT=localhost:7233
TEMPORAL_NAMESPACE={{ .Name }}-prod
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# Optional: TLS Configuration
# TEMPORAL_TLS_ENABLED=true
# REDIS_TLS_ENABLED=true

{{- end }}
```

---

### 29.5 Update .gitignore Template
**File**: `pkg/template/templates/basic/gitignore.tmpl`

- [ ] Add .compozy/ directory for persistent mode
- [ ] Mode-agnostic ignores (node_modules, .env)
- [ ] Keep minimal and clean

**Implementation**:
```gitignore
# Dependencies
node_modules/
bun.lockb

# Environment
.env
.env.local

{{- if or (eq .Mode "persistent") (eq .Mode "memory") }}

# Compozy data directory (persistent mode)
.compozy/

{{- end }}

# Build outputs
dist/
build/

# Logs
*.log
```

---

## Relevant Files

### Primary Files (Modified)
- `pkg/template/templates/basic/compozy.yaml.tmpl` - Mode-specific config
- `pkg/template/templates/basic/docker-compose.yaml.tmpl` - Conditional generation
- `pkg/template/templates/basic/README.md.tmpl` - Mode-specific docs
- `pkg/template/templates/basic/env.example.tmpl` - Mode-specific env vars
- `pkg/template/templates/basic/gitignore.tmpl` - Mode-specific ignores
- `pkg/template/templates/basic/basic.go` - Generation logic

### Dependent Files (Reference Only)
- `pkg/template/types.go` - Mode field (Task 28.0)
- `cli/cmd/init/init.go` - Mode selection (Task 27.0)

---

## Deliverables

1. **Mode-Specific compozy.yaml**
   - Memory mode: :memory: database, embedded services
   - Persistent mode: File paths, embedded services
   - Distributed mode: External services, env vars

2. **Conditional Docker Compose**
   - Generated only for distributed mode
   - Skipped for memory and persistent modes
   - Clear logging of generation decision

3. **Mode-Specific Documentation**
   - README quick start matches mode
   - Mode switching guide included
   - Environment variables documented

4. **Generated Projects Work**
   - Memory mode starts instantly
   - Persistent mode creates .compozy/ directory
   - Distributed mode includes docker-compose

---

## Tests

### Functional Testing
```bash
# Test memory mode generation
compozy init memory-test
cd memory-test
cat compozy.yaml  # Verify mode: memory, :memory: database
ls -la  # Verify NO docker-compose.yaml
compozy start  # Must start in <1 second
compozy stop

# Test persistent mode generation
compozy init persistent-test --mode persistent
cd persistent-test
cat compozy.yaml  # Verify mode: persistent, file paths
ls -la  # Verify NO docker-compose.yaml
compozy start  # Must create .compozy/ directory
ls -la .compozy/  # Verify files created
compozy stop

# Test distributed mode generation
compozy init distributed-test --mode distributed --docker
cd distributed-test
cat compozy.yaml  # Verify mode: distributed, env vars
ls -la  # Verify docker-compose.yaml EXISTS
cat docker-compose.yaml  # Verify PostgreSQL, Redis, Temporal
```

### Validation Commands
```bash
# Must pass before completing task
go test ./pkg/template/... -v
make lint
make test

# Manual validation
compozy init test-memory --mode memory
compozy init test-persistent --mode persistent
compozy init test-distributed --mode distributed --docker
```

---

## Success Criteria

- [x] compozy.yaml template has mode-specific sections
- [x] docker-compose generated ONLY for distributed mode
- [x] README has mode-specific quick start
- [x] Environment variables match mode requirements
- [x] .gitignore includes .compozy/ for persistent mode
- [x] Generated projects compile and run
- [x] Memory mode project starts in <1 second
- [x] Persistent mode creates .compozy/ directory
- [x] Distributed mode includes docker-compose.yaml
- [x] All tests pass
- [x] Code compiles without errors
- [x] No lint warnings or errors

---

## Dependencies

**Blocks:**
- Phase 6 (Final Validation) - needs working template generation

**Depends On:**
- Task 27.0 (TUI Form) - provides mode selection
- Task 28.0 (Template Types) - provides Mode field
- Phase 1 complete (mode constants)

---

## Notes

- This is the most complex task in Phase 5
- Tests all three modes end-to-end
- Generated projects are users' first experience
- Documentation in generated README is critical
- Implementation reference in `TEMPLATE_SYSTEM_ANALYSIS.md`
- Keep template logic clean and maintainable

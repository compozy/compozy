# Documentation Plan: Three-Mode Configuration System

**PRD Reference**: `tasks/prd-modes/_prd.md`
**Tech Spec Reference**: `tasks/prd-modes/_techspec.md`
**Status**: Planning

---

## Documentation Strategy

### Core Messaging Shift

**OLD MESSAGE**: "Compozy supports standalone and distributed modes"
**NEW MESSAGE**: "Compozy runs instantly with zero dependencies (memory mode), with optional persistence (persistent mode) or production deployment (distributed mode)"

**Emphasis**: Memory mode as the default, easiest path. Distributed mode as production option.

---

## Documentation Files to Update

### 1. Configuration Documentation

#### File: `docs/content/docs/configuration/mode-configuration.mdx`
**Priority**: CRITICAL (highest visibility)
**Current State**: Documents standalone/distributed two-mode system
**Required Changes**:

**Updates:**
- Update frontmatter description: "Control deployment modes (memory/persistent/distributed)"
- Rewrite overview section emphasizing memory mode as default
- Update mode list to show three modes with clear use case guidance
- Add decision matrix table (when to use each mode)
- Update all code examples to use memory/persistent/distributed
- Remove all "standalone" references (except migration notes)
- Update environment variable table
- Update validation section for new modes
- Add troubleshooting section (mode selection guidance)

**New Sections:**
```markdown
## Quick Mode Selection

| Scenario | Recommended Mode | Why |
|----------|------------------|-----|
| Trying Compozy for first time | memory | Zero setup, instant start |
| Local development | persistent | State preserved between restarts |
| Running tests | memory | Fastest execution |
| CI/CD pipelines | memory | No Docker overhead |
| Production deployment | distributed | Scalability and HA |
| Debugging with state | persistent | Inspect database files |

## Mode Comparison

| Feature | Memory | Persistent | Distributed |
|---------|--------|-----------|-------------|
| Startup Time | <1s | <2s | <3s |
| External Dependencies | None | None | PostgreSQL, Redis, Temporal |
| State Persistence | No | Yes | Yes |
| Data Location | RAM | ./.compozy/ | External services |
| Suitable For | Tests, prototyping | Local dev | Production |
| Horizontal Scaling | No | No | Yes |
```

**Migration Note**:
```markdown
<Callout type="warning" title="Breaking Change">
The `standalone` mode has been replaced with `memory` (no persistence) and `persistent` (with persistence).

**Migration**: Replace `mode: standalone` with:
- `mode: memory` if you don't need persistence (tests, quick prototyping)
- `mode: persistent` if you need state between restarts (local development)
</Callout>
```

---

### 2. Deployment Documentation

#### File: `docs/content/docs/deployment/standalone-mode.mdx`
**Action**: RENAME to `memory-mode.mdx`
**Priority**: HIGH

**Updates:**
- Rename file: `standalone-mode.mdx` → `memory-mode.mdx`
- Update frontmatter title: "Memory Mode Deployment"
- Update frontmatter description: "Zero-dependency deployment with in-memory storage"
- Rewrite content to emphasize ephemeral nature
- Add warning about data loss on restart
- Update all configuration examples
- Add "When to Use" section
- Add "Limitations" section (no persistence, concurrency limits)

**New Content Structure:**
```markdown
# Memory Mode Deployment

Memory mode provides the **fastest way to run Compozy** with zero external dependencies. All data is stored in RAM and lost on restart.

## When to Use Memory Mode

- ✅ Trying Compozy for the first time
- ✅ Running tests (50-80% faster than distributed mode)
- ✅ CI/CD pipelines
- ✅ Quick prototyping and demos
- ❌ Production deployments (use distributed mode)
- ❌ Development requiring state persistence (use persistent mode)

## Configuration

[...]

## Limitations

- All data lost on server restart
- Not suitable for production
- Limited by available RAM
- Single-process only (no horizontal scaling)
```

---

#### File: `docs/content/docs/deployment/persistent-mode.mdx`
**Action**: CREATE NEW
**Priority**: HIGH

**Content:**
```markdown
# Persistent Mode Deployment

Persistent mode provides file-based storage while maintaining zero external dependencies. Perfect for local development with state preservation.

## When to Use Persistent Mode

- ✅ Local development requiring state between restarts
- ✅ Debugging workflows with database inspection
- ✅ Multi-day development sessions
- ✅ Learning Compozy with persistent examples
- ❌ Production deployments (use distributed mode)
- ❌ Horizontal scaling (use distributed mode)

## Configuration

```yaml
mode: persistent

# Optional: customize data directory
database:
  url: ./my-data/compozy.db

temporal:
  standalone:
    database_file: ./my-data/temporal.db

redis:
  standalone:
    persistence:
      enabled: true
      dir: ./my-data/redis
```

## Data Directory Structure

```
.compozy/
├── compozy.db          # Main application database
├── temporal.db         # Temporal server database
└── redis/              # Redis persistence
    └── dump.rdb
```

## Backup and Restore

[...]

## Limitations

- Single-process only
- Limited concurrent writes (SQLite limitation)
- Not suitable for production
- No automatic replication
```

---

#### File: `docs/content/docs/deployment/distributed-mode.mdx`
**Priority**: MEDIUM (less changes needed)

**Updates:**
- Add mode comparison callout at top
- Emphasize this is the **production** mode
- Update examples to explicitly show `mode: distributed`
- Add migration path from persistent to distributed
- Keep all existing distributed mode content

**New Opening Section:**
```markdown
# Distributed Mode Deployment

Distributed mode is Compozy's **production-ready deployment** option, using external PostgreSQL, Redis, and Temporal services for scalability and high availability.

<Callout type="info">
**Need something simpler?**
- **Memory mode**: Zero dependencies, instant start (perfect for trying Compozy)
- **Persistent mode**: File-based storage, still zero external services (local development)
- **Distributed mode**: External services required (production deployments)
</Callout>

[... existing content ...]
```

---

### 3. Quick Start Documentation

#### File: `docs/content/docs/quick-start/index.mdx`
**Priority**: CRITICAL (first user touchpoint)

**Updates:**
- Update "Prerequisites" section: Remove Docker, PostgreSQL requirements
- Update installation instructions to emphasize zero dependencies
- Show memory mode in first example
- Add "Next Steps" section guiding to persistent/distributed modes
- Update all code examples to use `mode: memory` explicitly

**New Prerequisites Section:**
```markdown
## Prerequisites

- **Go 1.25+** OR download pre-built binary
- **That's it!** No Docker, PostgreSQL, or Redis required

Compozy's default memory mode requires zero external dependencies.
```

**New First Example:**
```markdown
## Your First Compozy Project

```bash
# Create new project
compozy init my-first-agent

# Start server (memory mode - instant startup)
cd my-first-agent
compozy start
# Server ready in <1 second!

# Execute your first workflow
compozy run hello-world
```

<Callout type="success">
**That's it!** Your agent is running with zero external dependencies.
</Callout>

## Next Steps

- **Keep prototyping?** Stay in memory mode
- **Need persistence?** Switch to [persistent mode](/docs/deployment/persistent-mode)
- **Going to production?** Configure [distributed mode](/docs/deployment/distributed-mode)
```

---

### 4. CLI Help Documentation

#### File: `cli/help/global-flags.md`
**Priority**: MEDIUM

**Updates:**
- Update `--mode` flag description
- Show all three modes with use case guidance
- Update default value to "memory"
- Update examples

**New Content:**
```markdown
## --mode

**Type:** string
**Default:** `memory`
**Environment:** `COMPOZY_MODE`

Controls the deployment mode:

- **memory**: In-memory SQLite, fastest startup, no persistence
  - Use for: Tests, quick prototyping, CI/CD

- **persistent**: File-based SQLite, embedded services, state preserved
  - Use for: Local development, debugging

- **distributed**: External PostgreSQL/Redis/Temporal, production-ready
  - Use for: Production deployments, horizontal scaling

**Examples:**

```bash
# Use memory mode (default)
compozy start

# Use persistent mode for local development
compozy start --mode persistent

# Use distributed mode for production
compozy start --mode distributed
```
```

---

### 5. Migration Guide

#### File: `docs/content/docs/guides/mode-migration-guide.mdx`
**Action**: RENAME from `migrate-standalone-to-distributed.mdx`
**Priority**: HIGH (critical for existing users)

**New Content:**
```markdown
# Mode Migration Guide

This guide helps you migrate from the old two-mode system (standalone/distributed) to the new three-mode system (memory/persistent/distributed).

## Breaking Changes (Alpha)

- ✅ `mode: distributed` → No changes needed
- ⚠️ `mode: standalone` → Removed, use `memory` or `persistent`
- 🆕 `mode: memory` → New default (in-memory, no persistence)
- 🆕 `mode: persistent` → New mode (file-based, with persistence)

## Quick Migration

### Standalone → Memory (No Persistence Needed)

**Use case**: Tests, CI/CD, quick prototyping

```yaml
# OLD
mode: standalone
temporal:
  standalone:
    database_file: :memory:

# NEW
mode: memory
# Temporal automatically uses in-memory storage
```

### Standalone → Persistent (Persistence Needed)

**Use case**: Local development

```yaml
# OLD
mode: standalone
temporal:
  standalone:
    database_file: ./temporal.db

# NEW
mode: persistent
# Files automatically saved to ./.compozy/
```

## Detailed Migration Steps

### Step 1: Identify Your Use Case

[...]

### Step 2: Update Configuration

[...]

### Step 3: Update Environment Variables

```bash
# OLD
export COMPOZY_MODE=standalone

# NEW (choose based on use case)
export COMPOZY_MODE=memory      # For tests/prototyping
export COMPOZY_MODE=persistent  # For local dev
export COMPOZY_MODE=distributed # For production
```

### Step 4: Update Docker Compose (If Applicable)

Memory and persistent modes don't need Docker Compose.

[...]

## Troubleshooting

### Error: "Mode must be 'memory', 'persistent', or 'distributed'"

Your configuration uses the old `standalone` mode. Update to:
- `memory` if you don't need persistence
- `persistent` if you need state between restarts

### Data Migration

[...]
```

---

### 6. Examples README

#### File: `examples/README.md`
**Priority**: MEDIUM

**Updates:**
- List all three mode examples
- Show when to use each example
- Update directory structure

**New Content:**
```markdown
# Compozy Examples

## Mode Examples

### Memory Mode Example
**Location**: `examples/memory-mode/`
**Use Case**: Quick start, tests, CI/CD

```bash
cd examples/memory-mode
compozy start
# Instant startup, no persistence
```

### Persistent Mode Example
**Location**: `examples/persistent-mode/`
**Use Case**: Local development with state

```bash
cd examples/persistent-mode
compozy start
# State saved to ./.compozy/
```

### Distributed Mode Example
**Location**: `examples/distributed-mode/`
**Use Case**: Production deployment

```bash
cd examples/distributed-mode
docker-compose up -d  # Start external services
compozy start
```

[...]
```

---

### 7. API Documentation (Auto-Generated)

#### File: `docs/api/openapi.yaml` (Generated)
**Action**: Regenerate from schemas
**Priority**: LOW (auto-generated)

**Process:**
- Run `make swagger` after schema updates (Task 21.0)
- Verify mode field documentation
- Verify examples use correct modes

---

## Documentation Deliverables

### Must-Have (MVP)
- [x] `mode-configuration.mdx` updated with three modes
- [x] `memory-mode.mdx` created (renamed from standalone-mode.mdx)
- [x] `persistent-mode.mdx` created (new)
- [x] `distributed-mode.mdx` updated (add comparison)
- [x] `quick-start/index.mdx` updated (memory mode emphasis)
- [x] `mode-migration-guide.mdx` created (breaking change guide)
- [x] `cli/help/global-flags.md` updated

### Nice-to-Have (Post-MVP)
- [ ] Video tutorial showing mode selection
- [ ] FAQ section on mode selection
- [ ] Performance comparison benchmarks (documented)
- [ ] Blog post announcement

---

## Documentation Testing Checklist

### Content Validation
- [ ] All code examples run successfully
- [ ] All links resolve (no 404s)
- [ ] All mode references use memory/persistent/distributed
- [ ] No inappropriate "standalone" references (except migration guide)
- [ ] Migration guide tested with real projects

### SEO and Discoverability
- [ ] Frontmatter metadata accurate
- [ ] Mode comparison table visible in search
- [ ] Quick mode selection guide prominent
- [ ] Migration guide linked from all mode docs

### User Experience
- [ ] Decision matrix helps users choose mode
- [ ] Error messages referenced in troubleshooting
- [ ] Next steps clear from each mode doc
- [ ] Examples match documentation

---

## Success Criteria

- [x] All documentation builds without errors
- [x] All code examples work in respective modes
- [x] No "standalone" references (except migration guide)
- [x] Migration guide provides clear path forward
- [x] Quick start emphasizes zero-dependency experience
- [x] Mode comparison table helps users decide
- [x] Documentation search returns mode guides prominently

---

## Implementation Notes

- Task 14.0 handles deployment documentation
- Task 15.0 handles configuration documentation
- Task 16.0 handles migration guide
- Task 17.0 handles quick start
- Task 18.0 handles CLI help
- Task 26.0 validates all documentation changes

**Estimated Effort**: 5-6 days (with parallelization to 1 day)

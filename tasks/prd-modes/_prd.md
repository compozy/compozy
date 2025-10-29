# Product Requirements Document: Three-Mode Configuration System

**Status**: Approved for Implementation
**Version**: 1.0
**Last Updated**: 2025-01-29
**Breaking Change**: Yes (acceptable in alpha)

---

## Overview

Replace Compozy's current two-mode system (standalone/distributed) with a clearer three-mode system that better reflects user intent and deployment scenarios:

- **memory** (NEW DEFAULT): Zero-dependency, in-memory operation for tests and quick prototyping
- **persistent**: File-based persistence for local development with state preservation
- **distributed**: Production-ready with external PostgreSQL, Redis, and Temporal

This change addresses the confusion around "standalone" terminology (which implies isolation but requires local services) and dramatically improves the developer experience with instant startup and 50-80% faster tests.

**Target Users:**
- Developers prototyping AI workflows (memory mode)
- Local development teams (persistent mode)
- DevOps teams deploying to production (distributed mode)

**Core Value**: "Just works" out of the box with zero external dependencies, while supporting production deployments when needed.

---

## Goals

### Primary Objectives
1. **Zero-dependency quickstart**: `compozy start` works immediately without Docker, PostgreSQL, or Redis
2. **50-80% faster test suite**: Eliminate testcontainers startup overhead by using in-memory SQLite
3. **Clearer intent-based naming**: Mode names reflect actual use cases (memory/persistent/distributed)
4. **Simplified onboarding**: New users can try Compozy in <1 minute

### Success Metrics
- Test suite execution time: 3-5 minutes → 45-90 seconds (60-70% improvement)
- Server startup (memory mode): <1 second (from cold start)
- Server startup (persistent mode): <2 seconds (from cold start)
- Time to first workflow execution: <10 seconds (down from 2-3 minutes with Docker)
- Developer satisfaction: "Easy to get started" feedback in user surveys

### Business Objectives
- Reduce friction in developer adoption (faster onboarding)
- Improve CI/CD pipeline performance (faster tests)
- Maintain production-grade distributed deployment support
- Demonstrate architectural flexibility and thoughtful design

---

## User Stories

### As a Developer Trying Compozy for the First Time
- I want to run `compozy init` and `compozy start` without installing PostgreSQL or Docker
- So that I can evaluate the framework in under 5 minutes
- **Acceptance**: Memory mode is the default, requires zero external dependencies

### As a Developer Running Tests
- I want tests to run 50-80% faster than before
- So that I can iterate quickly during development
- **Acceptance**: Test suite uses in-memory SQLite, no Docker containers

### As a Developer Working on a Feature
- I want my agent workflows to persist between server restarts
- So that I don't lose state during local development
- **Acceptance**: Persistent mode saves SQLite database to `./.compozy/` directory

### As a DevOps Engineer Deploying to Production
- I want to use external PostgreSQL, Redis, and Temporal services
- So that I can scale horizontally and maintain high availability
- **Acceptance**: Distributed mode supports all external services with proper configuration

### As a Documentation Reader
- I want clear guidance on which mode to use for my scenario
- So that I can make informed deployment decisions
- **Acceptance**: Documentation explains mode trade-offs with decision matrix

---

## Core Features

### Feature 1: Memory Mode (Default)

**What it does:**
- In-memory SQLite database (no persistence)
- Embedded miniredis cache (no persistence)
- Embedded Temporal server (ephemeral)
- Fastest startup and execution

**Why it's important:**
- Enables instant "try it out" experience
- Eliminates Docker/PostgreSQL as prerequisites
- Dramatically speeds up test suite
- Reduces CI/CD costs (no testcontainers overhead)

**Functional Requirements:**
1. Memory mode must be the default when `mode` is not specified
2. Server must start in <1 second (cold start)
3. All data lost on server restart (ephemeral by design)
4. No external dependencies required
5. SQLite uses `:memory:` connection string
6. Cache persistence explicitly disabled
7. Temporal uses in-memory SQLite

### Feature 2: Persistent Mode

**What it does:**
- File-based SQLite database (persists to `./.compozy/compozy.db`)
- Embedded Redis with BadgerDB persistence (persists to `./.compozy/redis/`)
- Embedded Temporal server with file-based SQLite (persists to `./.compozy/temporal.db`)
- State preserved across restarts

**Why it's important:**
- Supports local development with persistence
- No external services required
- Predictable state for debugging
- Still zero Docker/PostgreSQL dependency

**Functional Requirements:**
1. All data persists to `./.compozy/` directory by default
2. Server must start in <2 seconds (cold start)
3. Database file created automatically if missing
4. Redis persistence uses BadgerDB backend
5. Temporal database uses file-based SQLite
6. Graceful shutdown ensures data integrity
7. `.gitignore` should exclude `.compozy/` directory

### Feature 3: Distributed Mode

**What it does:**
- External PostgreSQL database (user-managed)
- External Redis cache (user-managed)
- External Temporal server (user-managed)
- Production-ready configuration

**Why it's important:**
- Horizontal scaling capability
- High availability support
- Separate state management
- Production best practices

**Functional Requirements:**
1. Requires external PostgreSQL, Redis, Temporal
2. Configuration validation enforces required connection details
3. Startup fails gracefully with actionable errors if services unavailable
4. TLS support for all external connections
5. Connection pooling and retry logic
6. Health checks for all external services
7. Metrics and monitoring integration

### Feature 4: Mode Selection in Templates

**What it does:**
- `compozy init` TUI form includes mode selection dropdown
- Generated `compozy.yaml` configures appropriate mode
- Docker compose only generated for distributed mode
- Mode-specific documentation in generated README

**Why it's important:**
- Users understand mode choices during project setup
- Generated configuration matches deployment intent
- Avoids confusion about infrastructure requirements

**Functional Requirements:**
1. TUI form has mode dropdown (memory/persistent/distributed)
2. Mode selected during init becomes default in compozy.yaml
3. Docker compose generation conditional on distributed mode
4. README explains chosen mode and how to change it
5. Environment variable examples match selected mode
6. `.gitignore` appropriate for selected mode

---

## User Experience

### User Personas

**Persona 1: Trial Developer (Sarah)**
- **Need**: Evaluate Compozy quickly without infrastructure setup
- **Journey**: Downloads binary → runs `compozy init` → chooses memory mode → runs `compozy start` → executes sample workflow
- **Time to value**: <5 minutes
- **Mode**: memory (default)

**Persona 2: Local Developer (Marcus)**
- **Need**: Develop AI agents with persistent state for debugging
- **Journey**: Initializes project → chooses persistent mode → develops workflows → state preserved between restarts
- **Time to value**: <10 minutes
- **Mode**: persistent

**Persona 3: DevOps Engineer (Priya)**
- **Need**: Deploy to production with external managed services
- **Journey**: Initializes project → chooses distributed mode → configures PostgreSQL/Redis/Temporal → deploys to Kubernetes
- **Time to value**: 1-2 hours (infrastructure setup time)
- **Mode**: distributed

### Key User Flows

**Flow 1: Quick Start (Memory Mode)**
```bash
# Install Compozy
brew install compozy  # or download binary

# Initialize project
compozy init
# TUI form: mode = "memory" (default)

# Start server
compozy start
# Output: "Server started in 0.8s (memory mode)"

# Execute workflow
compozy run my-workflow
# Works immediately, no external services needed
```

**Flow 2: Local Development (Persistent Mode)**
```bash
# Initialize with persistence
compozy init
# TUI form: mode = "persistent"

# Start server
compozy start
# Output: "Server started in 1.5s (persistent mode)"
# Output: "Database: ./.compozy/compozy.db"

# Develop workflows
# ... state persists ...

# Restart server
compozy restart
# State restored from ./.compozy/
```

**Flow 3: Production Deployment (Distributed Mode)**
```bash
# Initialize for production
compozy init
# TUI form: mode = "distributed"
# TUI form: include Docker = "yes"

# Configure external services
export COMPOZY_DATABASE_URL="postgresql://..."
export REDIS_ADDR="redis.prod:6379"
export TEMPORAL_HOST_PORT="temporal.prod:7233"

# Start server
compozy start
# Output: "Server started in 2.3s (distributed mode)"
# Output: "Connected to PostgreSQL"
# Output: "Connected to Redis"
# Output: "Connected to Temporal"
```

### UI/UX Considerations

**TUI Form Design:**
- Mode dropdown appears AFTER template selection, BEFORE Docker toggle
- Default selected: "memory"
- Help text for each mode explains use case
- Visual indicator: 🚀 memory, 💾 persistent, 🏭 distributed

**Error Messages:**
- Invalid mode: "Mode must be 'memory', 'persistent', or 'distributed'. Did you mean to use 'memory' instead of 'standalone'?"
- Distributed mode missing config: "Distributed mode requires PostgreSQL connection. Set COMPOZY_DATABASE_URL or configure database.url in compozy.yaml"
- pgvector with SQLite: "Vector search (pgvector) requires PostgreSQL. Use 'distributed' mode or remove vector search features."

**Accessibility:**
- CLI commands provide `--mode` flag for non-interactive use
- Environment variables support all configuration paths
- `compozy config show` displays effective mode and configuration
- Migration guide provides step-by-step instructions

---

## High-Level Technical Constraints

### Required Integrations
- SQLite driver (modernc.org/sqlite) - in-memory and file-based
- PostgreSQL driver (pgx) - distributed mode
- Miniredis (embedded Redis) - memory/persistent modes
- BadgerDB (Redis persistence backend) - persistent mode
- Temporal embedded server - memory/persistent modes

### Performance Targets
- Test suite: 50-80% faster than current (3-5 min → 45-90 sec)
- Server startup (memory): <1 second
- Server startup (persistent): <2 seconds
- Server startup (distributed): <3 seconds (network dependent)
- No performance regressions in distributed mode

### Data Sensitivity
- Memory mode: All data ephemeral, not suitable for sensitive data persistence
- Persistent mode: File-based storage, suitable for local development only
- Distributed mode: Production-grade, supports TLS and encryption at rest

### Non-Negotiable Requirements
- Breaking change acceptable (alpha version)
- Migration guide MUST be provided
- No backwards compatibility for "standalone" mode name
- All existing distributed mode functionality preserved

---

## Non-Goals (Out of Scope)

### Explicitly Excluded

1. **Hybrid mode combinations**: No support for "memory database + distributed cache" mixed configurations
2. **Automatic migration from standalone**: Users must manually update configuration (migration guide provided)
3. **In-place mode switching**: Changing modes requires server restart
4. **Cross-mode data migration**: No automated data export/import between modes
5. **Mode-specific features**: All features work across all modes (except pgvector which requires PostgreSQL)

### Future Considerations

1. **Read replicas**: Distributed mode read replica support (future)
2. **Sharding**: Database sharding strategies (future)
3. **Cloud-managed SQLite**: Integration with cloud SQLite providers (future)
4. **Hot reload**: Mode switching without restart (future)

### Boundaries and Limitations

- Memory mode data is ALWAYS ephemeral (by design)
- Persistent mode is NOT production-ready (local development only)
- SQLite has concurrency limitations (write serialization)
- Distributed mode requires external service management
- No downgrade path from distributed data to SQLite

---

## Phased Rollout Plan

### Phase 1: Core Configuration [CRITICAL] ✅ Planned
**Goal**: Configuration system supports three modes

**Deliverables:**
- Mode constants updated (memory/persistent/distributed)
- Default mode changed to memory
- Configuration validation enforces new modes
- Mode resolution logic handles three modes
- All configuration tests passing

**Success Criteria:**
- `make test` passes for config package
- `make lint` clean
- Mode selection validated correctly

**User-Facing Value**: N/A (infrastructure)

---

### Phase 2: Infrastructure Wiring [CRITICAL] ✅ Planned
**Goal**: Runtime correctly activates infrastructure per mode

**Deliverables:**
- Cache layer mode-aware (memory/persistent/distributed)
- Temporal wiring mode-aware
- Server logging shows correct mode
- Manual runtime validation complete

**Success Criteria:**
- Server starts successfully in all three modes
- Correct infrastructure activated per mode
- No regressions in distributed mode

**User-Facing Value**: Backend prepared for new modes

---

### Phase 3: Test Infrastructure [HIGH] ✅ Planned
**Goal**: Test suite 50-80% faster with memory mode

**Deliverables:**
- Test helpers default to SQLite memory mode
- Integration tests migrated to SQLite (except pgvector tests)
- Golden test files updated
- Performance benchmarked

**Success Criteria:**
- `make test` 50%+ faster than baseline
- All tests pass
- No flaky tests introduced

**User-Facing Value**: Faster CI/CD, better developer experience

---

### Phase 4: Documentation [HIGH] ✅ Planned
**Goal**: Users understand mode system and migration path

**Deliverables:**
- Mode configuration documentation updated
- Deployment guides for all three modes
- Migration guide from standalone to memory/persistent
- Quick start reflects memory mode default
- Examples for each mode

**Success Criteria:**
- All docs build without errors
- All code examples work
- Migration guide tested with real projects

**User-Facing Value**: Clear guidance on mode selection and migration

---

### Phase 5: Template System [NEW] 🆕 Proposed
**Goal**: `compozy init` generates mode-appropriate configuration

**Deliverables:**
- TUI form includes mode selection dropdown
- Generated compozy.yaml reflects selected mode
- Docker compose conditional on distributed mode
- Mode-specific README documentation
- Mode-specific environment variables

**Success Criteria:**
- All mode selections generate valid configurations
- Generated projects start successfully in chosen mode
- No Docker compose for memory/persistent modes

**User-Facing Value**: **CRITICAL** - First impression and onboarding experience

---

### Phase 6: Final Validation [CRITICAL] ✅ Planned
**Goal**: Ship-ready system validated end-to-end

**Deliverables:**
- Comprehensive testing across all modes
- Performance benchmarks documented
- Error messages validated
- Examples tested
- Documentation validated

**Success Criteria:**
- All tests pass
- Performance targets met
- No regressions
- Documentation accurate

**User-Facing Value**: Confidence in release quality

---

## Success Metrics

### User Engagement Metrics
- **Time to first workflow execution**: <10 seconds (memory mode)
- **Developer onboarding completion**: >80% complete quick start guide
- **Mode distribution**: 60% memory, 30% persistent, 10% distributed (estimated)
- **Docker compose generation**: Only for distributed mode users

### Performance Benchmarks
- **Test suite execution**: 45-90 seconds (down from 3-5 minutes, 60-70% improvement)
- **Server startup (memory)**: <1 second
- **Server startup (persistent)**: <2 seconds
- **Server startup (distributed)**: <3 seconds (network dependent)
- **CI/CD pipeline time**: 40-60% reduction (due to faster tests)

### Business Impact Indicators
- **GitHub stars increase**: 20-30% within 1 month (easier onboarding)
- **Issue reduction**: 50% fewer "setup issues" (zero dependencies)
- **Community growth**: 30% increase in Discord/community activity
- **Blog mentions**: "Compozy is easiest agentic framework to get started"

### Quality Attributes
- **Test coverage**: >80% (maintained)
- **Linter warnings**: Zero
- **Breaking change impact**: Documented in migration guide
- **Backward compatibility**: N/A (alpha version, breaking change acceptable)

---

## Risks and Mitigations

### User Adoption Risks

**Risk**: Existing users confused by "standalone" removal
- **Severity**: Medium
- **Mitigation**: Clear migration guide with find/replace instructions
- **Mitigation**: Helpful error messages suggesting memory mode
- **Mitigation**: Blog post explaining rationale

**Risk**: Users don't understand mode differences
- **Severity**: High
- **Mitigation**: Decision matrix in documentation
- **Mitigation**: Mode selection help text in TUI form
- **Mitigation**: Examples for each mode

### Market Competition Risks

**Risk**: Other frameworks already have zero-dependency setup
- **Severity**: Low
- **Mitigation**: Compozy's unique value is multi-mode flexibility
- **Mitigation**: Emphasize production-ready distributed mode

### Resource and Timeline Constraints

**Risk**: Template system (Phase 5) not originally planned
- **Severity**: High - impacts first impression
- **Mitigation**: Prioritize Phase 5 alongside Phase 4 (parallel work)
- **Mitigation**: Simple implementation (mode dropdown + conditional generation)
- **Estimate**: +1 day to timeline

**Risk**: 6-7 day estimate may be tight
- **Severity**: Medium
- **Mitigation**: Parallel execution plan (73% time savings)
- **Mitigation**: Clear task dependencies identified
- **Mitigation**: Breaking change acceptable, can iterate post-launch

---

## Open Questions

### Resolved ✅
- ~~Should we support mode switching without restart?~~ → No, out of scope
- ~~Should we migrate existing standalone data automatically?~~ → No, manual migration only
- ~~Should we keep "standalone" as an alias?~~ → No, clean break

### Remaining ❓

1. **Template System Priority**: Should Phase 5 (Template System) be mandatory for MVP?
   - **Impact**: HIGH - affects first impression and onboarding
   - **Recommendation**: Yes, prioritize Phase 5 alongside Phase 4

2. **`.compozy/` Directory Location**: Should persistent mode use `./.compozy/` or `~/.compozy/`?
   - **Current**: Project-local (`./.compozy/`)
   - **Alternative**: User-global (`~/.compozy/`)
   - **Recommendation**: Keep project-local, add `--data-dir` flag for customization (future)

3. **Mode Validation Strictness**: Should we warn on suboptimal configurations (e.g., memory mode in Dockerfile)?
   - **Recommendation**: Yes, add warnings in `compozy config diagnostics`

4. **Migration Tool**: Should we provide `compozy migrate standalone-to-memory` command?
   - **Recommendation**: No, manual migration sufficient for alpha (future consideration)

---

## Appendix

### Research Findings

**Current Pain Points** (from GitHub issues and Discord):
- "Docker setup too complex for trying out Compozy"
- "Test suite takes forever with testcontainers"
- "Why do I need PostgreSQL just to run Hello World?"
- "Standalone mode still requires Redis, confusing name"

**Competitive Analysis**:
- **Temporal**: Offers embedded server (Compozy already has this)
- **LangChain**: Zero dependencies for basic use (inspiration)
- **Prefect**: Defaults to SQLite, option for PostgreSQL (similar approach)

### Design Mockups

**TUI Form Flow:**
```
┌─────────────────────────────────────┐
│ Create New Compozy Project          │
├─────────────────────────────────────┤
│ Template: [basic ▼]                 │
│                                     │
│ Mode:     [memory ▼]                │ ← NEW FIELD
│           🚀 memory (default)       │
│           💾 persistent             │
│           🏭 distributed            │
│                                     │
│ Include Docker: [No]                │ ← Disabled for memory/persistent
└─────────────────────────────────────┘
```

### Reference Materials

- Technical Specification: `tasks/prd-modes/_techspec.md`
- Template Analysis: `tasks/prd-modes/TEMPLATE_SYSTEM_ANALYSIS.md`
- Current Mode Config Docs: `docs/content/docs/configuration/mode-configuration.mdx`
- Redis PRD: Reference for persistence patterns

---

## Planning Artifacts (Post-Approval)

These artifacts support implementation and should be maintained alongside this PRD:

- ✅ **Tech Spec**: `tasks/prd-modes/_techspec.md` (exists)
- ✅ **Tasks Summary**: `tasks/prd-modes/_tasks.md` (created)
- ✅ **Individual Tasks**: `tasks/prd-modes/_task_*.md` (26 tasks created)
- 🆕 **Docs Plan**: `tasks/prd-modes/_docs.md` (to be created)
- 🆕 **Examples Plan**: `tasks/prd-modes/_examples.md` (to be created)
- 🆕 **Tests Plan**: `tasks/prd-modes/_tests.md` (to be created)

---

**Status**: Ready for Implementation
**Next Steps**: Review Phase 5 (Template System) priority, then begin Phase 1 execution

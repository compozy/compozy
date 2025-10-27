# PRD: SQLite Database Backend Support

## Overview

Add SQLite as an alternative database backend to PostgreSQL for Compozy, enabling single-binary deployments and simplified development environments while maintaining PostgreSQL as the default and recommended option for production workloads.

## Problem Statement

Compozy currently requires PostgreSQL as a mandatory dependency, which creates barriers for:

1. **Development & Testing:** Developers need to run PostgreSQL locally or via Docker, adding complexity to onboarding
2. **Edge Deployments:** Embedded/IoT scenarios where running a separate database server is impractical
3. **Single-Binary Distribution:** Users wanting a truly self-contained binary without external database dependencies
4. **Quick Evaluation:** Potential users want to try Compozy without setting up infrastructure
5. **CI/CD Complexity:** Test environments require PostgreSQL setup, slowing down pipelines

## Goals

### Primary Goals

1. **Enable SQLite Backend:** Support SQLite 3.x as an alternative database driver alongside PostgreSQL
2. **Maintain PostgreSQL Default:** Keep PostgreSQL as the recommended production database
3. **Zero Breaking Changes:** Existing PostgreSQL configurations continue working unchanged
4. **Clear Guidance:** Document when to use SQLite vs PostgreSQL based on use case
5. **Vector DB Flexibility:** Mandate external vector database (Qdrant/Redis/Filesystem) when using SQLite

### Non-Goals

1. **Migration Tools:** Auto-migration from PostgreSQL to SQLite (manual export/import acceptable)
2. **Feature Parity:** SQLite doesn't need to match PostgreSQL's concurrency/performance characteristics
3. **Default Change:** PostgreSQL remains the default driver
4. **Backwards Compatibility:** No need to support upgrading old SQLite databases

## Success Metrics

### Quantitative Metrics

- **Developer Onboarding Time:** Reduce time-to-first-workflow from 15min → 5min (SQLite mode)
- **Binary Size:** Self-contained binary remains under 150MB
- **Test Suite Speed:** Integration tests 30% faster with in-memory SQLite
- **CI/CD Time:** Reduce test pipeline duration by 20%

### Qualitative Metrics

- Documentation clearly explains SQLite vs PostgreSQL trade-offs
- Zero production issues from accidental SQLite usage in high-concurrency scenarios
- Positive community feedback on simplified local development

## User Personas

### Persona 1: New Developer (Primary)

- **Need:** Quick local setup to evaluate Compozy
- **Pain:** Setting up PostgreSQL is a barrier to getting started
- **Benefit:** `compozy init` creates local SQLite database, ready immediately

### Persona 2: Edge Deployment Engineer

- **Need:** Deploy Compozy on embedded devices with limited resources
- **Pain:** Cannot run PostgreSQL on resource-constrained hardware
- **Benefit:** Single binary with SQLite embedded, no external dependencies

### Persona 3: CI/CD Pipeline Owner

- **Need:** Fast, reliable test execution
- **Pain:** PostgreSQL test containers slow down CI pipelines
- **Benefit:** In-memory SQLite for integration tests, faster execution

### Persona 4: Production Operator (Secondary - PostgreSQL)

- **Need:** High-concurrency, scalable production deployment
- **Current State:** Already using PostgreSQL
- **Impact:** Zero change; continues using PostgreSQL as recommended

## Requirements

### Functional Requirements

#### FR-1: Database Driver Selection

**Priority:** P0 (Must Have)

- **Requirement:** Support `database.driver` configuration field with values: `postgres` (default) or `sqlite`
- **Acceptance Criteria:**
  - Config validates driver value
  - PostgreSQL is default when field omitted
  - Invalid driver values produce clear error messages

#### FR-2: SQLite Driver Implementation

**Priority:** P0 (Must Have)

- **Requirement:** Implement complete repository pattern for SQLite backend
- **Acceptance Criteria:**
  - All workflow state operations work (create, read, update, list)
  - All task state operations work with hierarchical relationships
  - All auth operations work (users, API keys)
  - Transactions with proper isolation levels
  - Foreign key constraints enforced

#### FR-3: Database-Specific Configuration

**Priority:** P0 (Must Have)

- **Requirement:** Support driver-specific configuration options
- **PostgreSQL Config:** conn_string, host, port, user, password, dbname, ssl_mode
- **SQLite Config:** path (file path or `:memory:`)
- **Acceptance Criteria:**
  - PostgreSQL config unchanged
  - SQLite requires only `path` field
  - Configuration validation catches missing required fields per driver

#### FR-4: Migration Support

**Priority:** P0 (Must Have)

- **Requirement:** Database migrations work for both PostgreSQL and SQLite
- **Acceptance Criteria:**
  - Dual migration files (PostgreSQL SQL + SQLite SQL)
  - `compozy migrate up/down` works for both drivers
  - Migrations run automatically on server start (configurable)

#### FR-5: Vector Database Requirement for SQLite

**Priority:** P0 (Must Have)

- **Requirement:** When using SQLite, external vector database is mandatory for knowledge features
- **Acceptance Criteria:**
  - Startup validation fails if SQLite + pgvector provider configured
  - Startup validation passes if SQLite + (Qdrant OR Redis OR Filesystem) configured
  - Clear error message guiding users to configure external vector DB
  - Documentation explains vector DB requirement

### Non-Functional Requirements

#### NFR-1: Performance (SQLite)

**Priority:** P1 (Should Have)

- **Requirement:** SQLite performance sufficient for single-tenant, low-concurrency workloads
- **Targets:**
  - Read operations: <50ms p99 (comparable to PostgreSQL)
  - Write operations: <100ms p99 (acceptable degradation)
  - Concurrent workflows: Support 5-10 simultaneous executions
- **Acceptance Criteria:**
  - Benchmark tests demonstrate target performance
  - Documentation warns about concurrency limitations

#### NFR-2: Compatibility

**Priority:** P0 (Must Have)

- **Requirement:** Zero breaking changes to existing PostgreSQL deployments
- **Acceptance Criteria:**
  - All existing tests pass with PostgreSQL backend
  - Existing configuration files work unchanged
  - No schema migrations required for PostgreSQL users

#### NFR-3: Testing Coverage

**Priority:** P0 (Must Have)

- **Requirement:** Comprehensive test coverage for both database backends
- **Acceptance Criteria:**
  - Unit tests: 80%+ coverage for new SQLite code
  - Integration tests: All critical paths tested with both drivers
  - CI/CD: Matrix testing (PostgreSQL + SQLite)

#### NFR-4: Documentation Quality

**Priority:** P0 (Must Have)

- **Requirement:** Clear, comprehensive documentation for database selection
- **Acceptance Criteria:**
  - Decision matrix: When to use SQLite vs PostgreSQL
  - Configuration examples for both drivers
  - Vector DB configuration guide for SQLite
  - Performance characteristics comparison
  - Migration guide (if switching databases)

## Technical Constraints

### System Constraints

1. **Go Version:** Must support Go 1.25.2+ (current project version)
2. **SQLite Version:** Target SQLite 3.38.0+ (for JSON support)
3. **Driver Choice:** Use `modernc.org/sqlite` (pure Go, no CGO) or `github.com/mattn/go-sqlite3` (CGO, more mature)
4. **Vector Storage:** No native SQLite vector support - external DB required

### Architecture Constraints

1. **Repository Pattern:** Must maintain existing repository interfaces
2. **Clean Architecture:** Follow existing domain-driven design patterns
3. **Dependency Injection:** Context-first configuration and logging (no DI for config/logger)
4. **No Globals:** Zero global configuration state

### Database Feature Constraints

1. **Row Locking:** SQLite has DB-level locking only (vs PostgreSQL row-level)
2. **Concurrency:** SQLite write serialization acceptable for target use cases
3. **JSONB:** Map PostgreSQL JSONB to SQLite JSON functions
4. **Array Operations:** Convert PostgreSQL `ANY($1::type[])` to `IN (?, ?, ?)`

## Dependencies

### Internal Dependencies

- **Configuration System:** `pkg/config` - add database driver selection
- **Repository Interfaces:** `engine/workflow`, `engine/task`, `engine/auth` - must remain unchanged
- **Infra Layer:** `engine/infra/repo` - factory pattern to select implementation
- **Vector DB System:** `engine/knowledge/vectordb` - already supports multiple providers

### External Dependencies

**New:**
- `modernc.org/sqlite` OR `github.com/mattn/go-sqlite3` - SQLite driver
- Note: `github.com/pressly/goose/v3` already supports SQLite migrations

**Existing (No Changes):**
- `github.com/jackc/pgx/v5` - PostgreSQL (keep existing)
- `github.com/Masterminds/squirrel` - Query builder (database-agnostic, reuse)

## Risks & Mitigation

### Risk 1: Production SQLite Misuse

**Risk:** Users deploy SQLite in high-concurrency production scenarios

**Impact:** High - Performance degradation, database locks, poor user experience

**Likelihood:** Medium

**Mitigation:**
- Clear documentation with decision matrix
- Startup warnings when SQLite detected in production mode
- Performance benchmarks in documentation
- Default remains PostgreSQL

### Risk 2: Vector Search Quality Degradation

**Risk:** External vector DB performs worse than pgvector

**Impact:** Medium - Knowledge base features less effective

**Likelihood:** Low - Qdrant/Redis are mature solutions

**Mitigation:**
- Document vector DB options with performance characteristics
- Provide Qdrant as recommended external option
- Test suite validates all vector DB providers
- Keep pgvector as recommended for PostgreSQL

### Risk 3: Maintenance Burden

**Risk:** Maintaining two database implementations increases complexity

**Impact:** Medium - More code to maintain, test, debug

**Likelihood:** High

**Mitigation:**
- Shared test suite for both backends
- CI/CD matrix testing
- Clear code organization (separate packages)
- Comprehensive documentation for contributors

### Risk 4: Feature Divergence

**Risk:** New features only work with PostgreSQL

**Impact:** Low - SQLite users miss features

**Likelihood:** Medium

**Mitigation:**
- Document database-specific features
- Feature flags for PostgreSQL-only features
- Review process checks both implementations

## Implementation Phases

### Phase 1: Foundation (Weeks 1-3)

**Goal:** Core SQLite infrastructure and configuration

**Deliverables:**
- SQLite driver package (`engine/infra/sqlite`)
- Configuration support (`database.driver` field)
- Basic repository implementations (user auth)
- Migration system for SQLite
- Test infrastructure setup

**Success Criteria:**
- `compozy start --db-driver=sqlite --db-path=compozy.db` works
- User authentication (login, API keys) functional
- Basic tests passing

### Phase 2: Core Features (Weeks 4-6)

**Goal:** Workflow and task persistence

**Deliverables:**
- Workflow repository for SQLite
- Task repository for SQLite (hierarchical support)
- Transaction handling
- JSONB → JSON mapping
- Comprehensive tests

**Success Criteria:**
- Workflows execute end-to-end with SQLite
- Task hierarchy (parent/child) works
- State persistence across restarts
- Integration tests green

### Phase 3: Production Readiness (Weeks 7-9)

**Goal:** Polish, performance, documentation

**Deliverables:**
- Performance optimization
- Concurrent execution support
- Vector DB validation (SQLite mode)
- Complete documentation
- Migration tooling

**Success Criteria:**
- Performance benchmarks meet targets
- Documentation complete and reviewed
- All CI/CD tests passing
- Community preview feedback positive

### Phase 4: Release (Week 10)

**Goal:** Launch and communication

**Deliverables:**
- Release announcement
- Tutorial blog post
- Video walkthrough
- Community support plan

**Success Criteria:**
- Feature released in minor version
- Documentation published
- No critical bugs in first week

## Open Questions

1. **SQLite Driver Choice:** `modernc.org/sqlite` (pure Go) vs `github.com/mattn/go-sqlite3` (CGO)?
   - **Recommendation:** Start with `modernc.org/sqlite` (no CGO, easier cross-compilation)
   - **Backup:** Fall back to `go-sqlite3` if performance issues

2. **Default Vector DB for SQLite:** Which external vector DB to recommend?
   - **Options:** Qdrant (best features), Redis (simplest), Filesystem (development)
   - **Recommendation:** Qdrant for production, Filesystem for development

3. **Concurrency Limits:** Should we enforce max concurrent workflows with SQLite?
   - **Recommendation:** Document but don't enforce; let users decide

4. **Migration Path:** Support data migration between PostgreSQL ↔ SQLite?
   - **Recommendation:** Phase 2 feature - export/import JSON tooling

## Appendix

### Related Documents

- **Analysis:** `POSTGRES_ANALYSIS.md` - Comprehensive technical analysis
- **Architecture:** `.cursor/rules/architecture.mdc` - Project architecture patterns
- **Configuration:** `.cursor/rules/global-config.mdc` - Config management standards

### Reference Implementation

- **Vector DB Providers:** `engine/knowledge/vectordb/{qdrant,redis,filesystem}.go`
- **Repository Pattern:** `engine/infra/postgres/*.go`
- **Migration System:** `engine/infra/postgres/migrations/`

### Version History

| Version | Date       | Author      | Changes           |
| ------- | ---------- | ----------- | ----------------- |
| 1.0     | 2025-01-27 | AI Analysis | Initial PRD draft |

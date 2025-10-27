# SQLite Database Backend Support - Implementation Tasks

## Relevant Files

### Core Implementation Files

- `pkg/config/config.go` - Database driver configuration
- `engine/infra/sqlite/store.go` - SQLite connection management
- `engine/infra/sqlite/migrations.go` - Migration system
- `engine/infra/sqlite/migrations/*.sql` - SQLite schema definitions
- `engine/infra/sqlite/authrepo.go` - Authentication repository
- `engine/infra/sqlite/workflowrepo.go` - Workflow state repository
- `engine/infra/sqlite/taskrepo.go` - Task state repository
- `engine/infra/repo/provider.go` - Repository factory pattern

### Integration Points

- `engine/infra/server/dependencies.go` - Server initialization and validation
- `test/integration/database/multi_driver_test.go` - Cross-driver integration tests
- `test/helpers/database.go` - Test infrastructure

### Documentation Files

- `docs/content/docs/database/overview.mdx` - Database decision guide
- `docs/content/docs/database/postgresql.mdx` - PostgreSQL documentation
- `docs/content/docs/database/sqlite.mdx` - SQLite documentation
- `docs/content/docs/troubleshooting/database.mdx` - Troubleshooting guide
- `docs/content/docs/configuration/database.mdx` - Configuration reference
- `docs/content/docs/cli/start.mdx` - CLI documentation
- `docs/source.config.ts` - Navigation structure

### Examples

- `examples/database/sqlite-quickstart/` - SQLite quickstart example

## Tasks

- [x] 1.0 SQLite Foundation Infrastructure (L)
- [x] 2.0 Authentication Repository (SQLite) (M)
- [x] 3.0 Workflow Repository (SQLite) (M)
- [ ] 4.0 Task Repository & Factory Integration (L)
- [x] 5.0 Server Integration & Validation (M)
- [ ] 6.0 Multi-Driver Integration Tests (L)
- [ ] 7.0 Complete Database Documentation (L)
- [ ] 8.0 SQLite Quickstart Example (S)

Notes on sizing:

- S = Small (≤ half-day)
- M = Medium (1–2 days)
- L = Large (3+ days)

## Task Design Rules

- Each parent task is a closed deliverable: independently shippable and reviewable
- Do not split one deliverable across multiple parent tasks; avoid cross-task coupling
- Each parent task must include unit test subtasks derived from `_tests.md` for this feature
- Each generated `/_task_<num>.md` must contain explicit Deliverables and Tests sections

## Execution Plan

### Critical Path (Sequential)
```
1.0 → 2.0 → 3.0 → 4.0 → 5.0
```
**Duration:** ~12-18 days

### Parallel Tracks

**Track A (Documentation):** Can start Week 3
```
7.0
```

**Track B (Testing):** Can start after Task 5.0
```
6.0
```

**Track C (Example):** Can start Week 7
```
8.0
```

### Timeline
- **Weeks 1-2:** Foundation (Task 1.0)
- **Weeks 2-5:** Repositories (Tasks 2.0, 3.0, 4.0)
- **Weeks 5-6:** Integration (Task 5.0)
- **Week 6:** Testing (Task 6.0)
- **Weeks 3-7:** Documentation (Task 7.0, parallel)
- **Week 7:** Example (Task 8.0)

**Total Duration:** ~4-6 weeks

## Notes

- All runtime code MUST use `logger.FromContext(ctx)` and `config.FromContext(ctx)`
- Run `make fmt && make lint && make test` before marking any task as completed
- Use `t.Context()` in tests (never `context.Background()`)
- Follow repository pattern with interfaces (no leaking of driver-specific types)
- PostgreSQL remains default driver - zero breaking changes
- Vector DB validation: SQLite + pgvector must fail at startup

## Batch Plan (Grouped Commits)

- [x] Batch 1 — Foundation: 1.0
- [ ] Batch 2 — Repositories: 2.0, 3.0, 4.0
- [ ] Batch 3 — Integration: 5.0
- [ ] Batch 4 — Testing: 6.0
- [ ] Batch 5 — Documentation: 7.0
- [ ] Batch 6 — Example: 8.0

## Key Technical Decisions

**From Technical Specification:**
- **Driver Choice:** `modernc.org/sqlite` (pure Go, no CGO)
- **Architecture:** Hybrid dual-implementation (separate `postgres/` and `sqlite/` packages)
- **Vector Storage:** Mandatory external vector DB for SQLite (Qdrant/Redis/Filesystem)
- **Concurrency:** Document limitations (5-10 workflows recommended for SQLite)
- **SQL Syntax:** Separate migration files for PostgreSQL and SQLite
- **Locking:** Optimistic locking with version columns (SQLite has DB-level locking only)

## Success Criteria

- [ ] All tests pass: `make test`
- [ ] All linters pass: `make lint`
- [ ] PostgreSQL tests unchanged (zero regressions)
- [ ] SQLite tests achieve 80%+ coverage
- [ ] Integration tests pass for both drivers
- [ ] Documentation complete and reviewed
- [ ] Example runs successfully
- [ ] Vector DB validation enforced
- [ ] No breaking changes to existing configurations

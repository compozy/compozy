# Temporal Standalone Mode Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `engine/worker/embedded/config.go` - Embedded server configuration types and validation
- `engine/worker/embedded/server.go` - Server lifecycle management
- `engine/worker/embedded/builder.go` - Temporal config builder functions
- `engine/worker/embedded/namespace.go` - Namespace creation helper
- `engine/worker/embedded/ui.go` - Web UI server wrapper
- `pkg/config/config.go` - TemporalConfig.Mode and StandaloneConfig
- `pkg/config/definition/schema.go` - Configuration registry entries
- `pkg/config/provider.go` - Configuration defaults
- `engine/infra/server/dependencies.go` - Server lifecycle integration

### Integration Points

- `engine/worker/client.go` - Temporal client (unchanged, uses overridden HostPort)
- `engine/worker/mod.go` - Worker initialization (unchanged)

### Test Files

- `engine/worker/embedded/config_test.go` - Config validation tests
- `engine/worker/embedded/server_test.go` - Server lifecycle tests
- `engine/worker/embedded/namespace_test.go` - Namespace creation tests
- `engine/worker/embedded/ui_test.go` - UI server tests
- `pkg/config/config_test.go` - Config system tests
- `test/integration/temporal/standalone_test.go` - Core integration tests
- `test/integration/temporal/mode_switching_test.go` - Mode switching tests
- `test/integration/temporal/persistence_test.go` - Persistence tests
- `test/integration/temporal/errors_test.go` - Error handling tests
- `test/integration/temporal/startup_lifecycle_test.go` - Lifecycle tests

### Documentation Files

- `docs/content/docs/deployment/temporal-modes.mdx` - Mode selection guide
- `docs/content/docs/architecture/embedded-temporal.mdx` - Architecture deep-dive
- `docs/content/docs/configuration/temporal.mdx` - Configuration reference
- `docs/content/docs/quick-start/index.mdx` - Quick start guide
- `docs/content/docs/deployment/production.mdx` - Production deployment
- `docs/content/docs/cli/compozy-start.mdx` - CLI documentation
- `docs/content/docs/troubleshooting/temporal.mdx` - Troubleshooting guide

### Examples

- `examples/temporal-standalone/basic/` - Basic setup example
- `examples/temporal-standalone/persistent/` - File-based persistence
- `examples/temporal-standalone/custom-ports/` - Custom port configuration
- `examples/temporal-standalone/no-ui/` - UI disabled example
- `examples/temporal-standalone/debugging/` - Debugging with Web UI
- `examples/temporal-standalone/migration-from-remote/` - Mode migration guide
- `examples/temporal-standalone/integration-testing/` - Testing example

## Tasks

- [x] 1.0 Embedded Server Package Foundation (L - 3 days)
- [x] 2.0 Embedded Server Lifecycle (M - 2 days)
- [x] 3.0 Configuration System Extension (M - 2 days)
- [ ] 4.0 UI Server Implementation (M - 1-2 days)
- [ ] 5.0 Server Lifecycle Integration (M - 2 days)
- [ ] 6.0 Core Integration Tests (L - 3 days)
- [ ] 7.0 CLI & Schema Updates (S - half day)
- [ ] 8.0 Documentation (L - 2-3 days)
- [ ] 9.0 Examples (L - 2-3 days)
- [ ] 10.0 Advanced Integration Tests (M - 2 days)

## Task Sizing

- S = Small (≤ half-day)
- M = Medium (1–2 days)
- L = Large (3+ days)

## Task Design Rules

- Each parent task is a closed deliverable: independently shippable and reviewable
- Do not split one deliverable across multiple parent tasks; avoid cross-task coupling
- Each parent task must include unit test subtasks derived from `_tests.md` for this feature
- Each generated `/_task_<num>.md` must contain explicit Deliverables and Tests sections
- All runtime code MUST use `logger.FromContext(ctx)` and `config.FromContext(ctx)`
- Functions MUST be <50 lines per go-coding-standards.mdc
- Run `make lint && make test` before marking any task as completed

## Execution Plan

```
CRITICAL PATH (10 days):
┌─────────────────────────────────────────────────────────────┐
│ Phase 1: Foundation (3 days)                                │
│   1.0 Embedded Server Package Foundation [L]                │
│      ↓                                                       │
│ Phase 2: Parallel Core Development (2 days)                 │
│   ┌──────────────────┐    ┌──────────────────────┐         │
│   │ 2.0 Lifecycle [M]│    │ 3.0 Config System [M]│         │
│   └────────┬─────────┘    └──────────┬───────────┘         │
│            └────────────┬─────────────┘                     │
│                         ↓                                    │
│ Phase 3: Integration & UI (2 days)                          │
│   ┌──────────────────────────────────┐                     │
│   │ 5.0 Lifecycle Integration [M]    │                     │
│   └────────────┬─────────────────────┘                     │
│                ↓                                             │
│ Phase 4: Validation (3 days)                                │
│   6.0 Core Integration Tests [L]                            │
└─────────────────────────────────────────────────────────────┘

PARALLEL LANES (after respective dependencies):
┌─────────────────────────────────────────────────────────────┐
│ Lane A: 4.0 UI Server (starts after 2.0) [M, 1-2 days]     │
│ Lane B: 7.0 CLI & Schema (starts after 3.0) [S, 0.5 days]  │
│ Lane C: 8.0 Documentation (starts after 5.0) [L, 2-3 days] │
│ Lane D: 9.0 Examples (starts after 5.0) [L, 2-3 days]      │
│ Lane E: 10.0 Advanced Tests (starts after 5.0) [M, 2 days] │
└─────────────────────────────────────────────────────────────┘
```

**Timeline:**
- Critical Path: 10 days
- With Full Parallelization: ~10-11 days total
- Estimated Effort: ~20 developer-days
- Can be executed by 2-3 developers working in parallel lanes

**Key Dependencies:**
- 1.0 blocks everything (foundation)
- 2.0 and 3.0 can run in parallel after 1.0
- 5.0 requires both 2.0 and 3.0 complete
- 6.0, 8.0, 9.0, 10.0 all start after 5.0 (can run in parallel)
- 4.0 can start as soon as 2.0 completes
- 7.0 can start as soon as 3.0 completes

## Batch Plan (Grouped Commits)

- [x] **Batch 1 — Foundation:** 1.0
  - Complete embedded package with tests
  - Merge as single atomic commit
  
- [ ] **Batch 2 — Core Systems:** 2.0, 3.0
  - Server lifecycle + configuration system
  - Both required for integration
  - Can be separate PRs, both must merge before Batch 3
  
- [ ] **Batch 3 — Integration:** 4.0, 5.0
  - UI server + lifecycle integration
  - Complete end-to-end functionality
  - Merge together for working standalone mode
  
- [ ] **Batch 4 — Validation:** 6.0
  - Core integration tests
  - Validates Batch 3 works correctly
  - Required before Batch 6
  
- [ ] **Batch 5 — Polish:** 7.0, 10.0
  - CLI support + advanced tests
  - Quality improvements
  - Can merge independently
  
- [ ] **Batch 6 — Documentation:** 8.0, 9.0
  - Documentation + examples
  - Can be separate PRs
  - Should merge before feature release

## Success Criteria

- [ ] All unit tests pass (`make test`)
- [ ] All integration tests pass
- [ ] Linter passes (`make lint`)
- [ ] Workflows execute end-to-end in standalone mode
- [ ] Web UI accessible at http://localhost:8233
- [ ] File-based persistence works across restarts
- [ ] In-memory mode works for ephemeral development
- [ ] Documentation complete and accurate
- [ ] Examples all runnable and tested
- [ ] CLI flags functional
- [ ] Configuration schema updated
- [ ] Zero impact on existing remote mode functionality

## Implementation Notes

### Critical Requirements

1. **Use temporal.NewServer()**, NOT Temporalite (deprecated)
2. **Context-first patterns:** `logger.FromContext(ctx)`, `config.FromContext(ctx)`
3. **Function length:** Max 50 lines per function
4. **Error wrapping:** Use `fmt.Errorf("...: %w", err)`
5. **Resource cleanup:** Always defer cleanup functions
6. **Port configuration:** Default 7233-7236 for services, 8233 for UI

### Development Environment

- **Go Version:** 1.23+ (project uses 1.25.2)
- **Dependencies:** `go.temporal.io/server` (latest stable), `go.temporal.io/server/ui-server/v2`
- **Database:** SQLite (built into Go, no external deps)
- **Test Commands:**
  - Scoped tests: `gotestsum --format pkgname -- -race -parallel=4 ./engine/worker/embedded`
  - Scoped lint: `golangci-lint run --fix --allow-parallel-runners ./engine/worker/embedded/...`
  - Full validation: `make lint && make test` (run before completing tasks)

### Reference Implementation

- GitHub: https://github.com/abtinf/temporal-a-day/blob/main/001-all-in-one-hello/main.go
- Shows proper use of temporal.NewServer() with SQLite, UI server, and namespace creation

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Large dependency size | Document, acceptable for functionality gained |
| Port conflicts | Clear error messages, configurable ports (task 3.0) |
| SQLite corruption | WAL mode, good error messages (task 1.0) |
| Accidental production use | Validation checks, prominent warnings (tasks 5.0, 8.0) |
| Startup timeout | Configurable timeout, efficient polling (task 2.0) |

## Questions or Blockers?

If you encounter issues:
1. Check `_techspec.md` for detailed design
2. Check `_tests.md` for test requirements
3. Check `_docs.md` for documentation requirements
4. Check `.cursor/rules/` for coding standards
5. Review reference implementation link above

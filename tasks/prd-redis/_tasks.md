# Redis Standalone Mode Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `pkg/config/config.go` - Add global mode and RedisConfig structs
- `pkg/config/resolver.go` - Mode resolution logic and helper methods (NEW)
- `pkg/config/loader.go` - Mode validation rules
- `engine/infra/cache/miniredis_standalone.go` - MiniredisStandalone wrapper (NEW)
- `engine/infra/cache/snapshot_manager.go` - BadgerDB snapshot persistence (NEW)
- `engine/infra/cache/mod.go` - Mode-aware cache factory (UPDATE)

### Integration Points

- `engine/infra/server/dependencies.go` - Update Temporal factory to use resolver
- `engine/infra/server/mcp.go` - Update MCPProxy factory to use resolver
- `engine/memory/store/redis.go` - Memory store (no changes, verify compatibility)
- `engine/resources/redis_store.go` - Resource store (no changes, verify compatibility)
- `engine/infra/server/dependencies.go` - Streaming setup (no changes, verify compatibility)

### Documentation Files

- `docs/content/docs/deployment/standalone-mode.mdx` - Standalone deployment guide (NEW)
- `docs/content/docs/configuration/mode-configuration.mdx` - Mode configuration guide (NEW)
- `docs/content/docs/configuration/redis.mdx` - Redis configuration reference (NEW)
- `docs/content/docs/guides/migrate-standalone-to-distributed.mdx` - Migration guide (NEW)
- `docs/content/docs/deployment/distributed-mode.mdx` - Update with comparison (UPDATE)
- `docs/content/docs/getting-started/quickstart.mdx` - Add standalone quick start (UPDATE)

### Examples

- `examples/standalone/basic/*` - Minimal standalone deployment
- `examples/standalone/with-persistence/*` - Standalone with BadgerDB snapshots
- `examples/standalone/mixed-mode/*` - Hybrid deployment example
- `examples/standalone/edge-deployment/*` - Edge/IoT deployment
- `examples/standalone/migration-demo/*` - Migration walkthrough

## Tasks

- [x] 1.0 Global Mode Configuration & Resolver (M - 1-2 days)
- [x] 2.0 MiniredisStandalone Wrapper (S - ≤ half-day)
- [x] 3.0 Mode-Aware Cache Factory (S - ≤ half-day)
- [x] 4.0 Memory Store Integration (M - 1 day)
- [x] 5.0 Resource Store Integration (M - 1 day)
- [x] 6.0 Streaming & Pub/Sub Integration (M - 1 day)
- [ ] 7.0 Snapshot Manager Implementation (M - 1-2 days)
- [ ] 8.0 Persistence Integration Tests (M - 1 day)
- [ ] 9.0 End-to-End Workflow Tests (M - 1-2 days)
- [ ] 10.0 Contract Tests & Validation (M - 1 day)
- [ ] 11.0 Configuration Validation & CLI (S - ≤ half-day)
- [ ] 12.0 User Documentation (M - 1-2 days)
- [ ] 13.0 Examples & Runbooks (M - 1-2 days)

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

### Critical Path (5-7 days)
Task 1.0 → Task 2.0 → Task 3.0 → Task 4.0 → Task 9.0 → Task 10.0

### Parallel Execution Lanes

**Lane 1 (Config - 2 days):**
- Task 1.0 → Task 11.0

**Lane 2 (Core - 2-3 days):**
- Task 1.0 → Task 2.0 → Task 3.0

**Lane 3 (Memory Store - 3-4 days):**
- Task 3.0 → Task 4.0 → Task 9.0

**Lane 4 (Resource Store - 3-4 days):**
- Task 3.0 → Task 5.0 → Task 9.0

**Lane 5 (Streaming - 3-4 days):**
- Task 3.0 → Task 6.0 → Task 9.0

**Lane 6 (Persistence - 3-4 days):**
- Task 2.0 → Task 7.0 → Task 8.0 → Task 9.0

**Lane 7 (Validation - 1-2 days):**
- Task 9.0 → Task 10.0

**Lane 8 (Documentation - 1-2 days):**
- Task 3.0 → Task 12.0

**Lane 9 (Examples - 1-2 days):**
- Task 3.0 → Task 13.0

### Team Allocation Suggestion

**With 3 developers (5-7 days total):**

Developer 1 (Backend - Critical Path):
- Lane 1: Task 1.0 → Task 11.0
- Lane 2: Task 2.0 → Task 3.0
- Lane 7: Task 9.0 → Task 10.0

Developer 2 (Domain Integration):
- Lane 3: Task 4.0
- Lane 4: Task 5.0
- Lane 5: Task 6.0

Developer 3 (Persistence + Content):
- Lane 6: Task 7.0 → Task 8.0
- Lane 8: Task 12.0
- Lane 9: Task 13.0

**With 2 developers (7-10 days total):**
- Dev 1: Lanes 1, 2, 7
- Dev 2: Lanes 3, 4, 5, 6, 8, 9

Notes:
- All runtime code MUST use `logger.FromContext(ctx)` and `config.FromContext(ctx)`
- Run `make lint` and `make test` before marking any task as completed
- Each task includes its own tests - no separate testing phase
- Tasks 4, 5, 6 can run 100% in parallel after Task 3.0 completes
- Documentation and examples can start as soon as Task 3.0 completes

## Batch Plan (Grouped Commits)

### Batch 1 — Configuration Foundation
- [x] Task 1.0: Global Mode Configuration & Resolver
- [ ] Task 11.0: Configuration Validation & CLI

**Commit Message**: `feat(config): add global mode configuration with component inheritance`

**Why grouped**: Configuration foundation that enables all other work. Single logical feature.

---

### Batch 2 — Core Miniredis Integration
- [ ] Task 2.0: MiniredisStandalone Wrapper
- [ ] Task 3.0: Mode-Aware Cache Factory

**Commit Message**: `feat(cache): add miniredis standalone backend with mode-aware factory`

**Why grouped**: Core cache backend implementation. Completes the miniredis integration.

---

### Batch 3 — Domain Store Compatibility (Parallel)
- [ ] Task 4.0: Memory Store Integration
- [ ] Task 5.0: Resource Store Integration
- [ ] Task 6.0: Streaming & Pub/Sub Integration

**Commit Message**: `test(cache): verify miniredis compatibility across all domain stores`

**Why grouped**: All integration tests proving miniredis works with existing stores. Can be one commit or split into 3 if preferred.

---

### Batch 4 — Persistence Layer
- [ ] Task 7.0: Snapshot Manager Implementation
- [ ] Task 8.0: Persistence Integration Tests

**Commit Message**: `feat(cache): add BadgerDB snapshot persistence for standalone mode`

**Why grouped**: Complete persistence feature with tests. Single logical enhancement.

---

### Batch 5 — End-to-End Validation
- [ ] Task 9.0: End-to-End Workflow Tests
- [ ] Task 10.0: Contract Tests & Validation

**Commit Message**: `test(standalone): add comprehensive integration and contract tests`

**Why grouped**: Final validation suite. Ensures feature completeness.

---

### Batch 6 — Documentation
- [ ] Task 12.0: User Documentation

**Commit Message**: `docs: add standalone mode deployment and configuration guides`

**Why grouped**: All user-facing documentation in one commit.

---

### Batch 7 — Examples
- [ ] Task 13.0: Examples & Runbooks

**Commit Message**: `examples: add standalone mode example projects and runbooks`

**Why grouped**: All example projects together for easier review.

---

## Risk Mitigation

### High Priority Risks

1. **Miniredis Lua Script Compatibility**
   - **Risk**: Lua scripts may behave differently in miniredis
   - **Mitigation**: Task 4.0 explicitly tests all Lua scripts used by memory store
   - **Contingency**: Contract tests in Task 10.0 verify behavioral parity

2. **TxPipeline Atomicity**
   - **Risk**: Resource store relies on TxPipeline for atomic operations
   - **Mitigation**: Task 5.0 has dedicated tests for TxPipeline and optimistic locking
   - **Contingency**: Contract tests verify exact Redis behavior

3. **Snapshot Performance Impact**
   - **Risk**: Large snapshots may block operations
   - **Mitigation**: Task 7.0 uses background goroutines for non-blocking snapshots
   - **Validation**: Task 8.0 tests snapshot operations under load

### Medium Priority Risks

4. **Mode Configuration Confusion**
   - **Risk**: Users may misconfigure mode inheritance
   - **Mitigation**: Task 1.0 includes comprehensive validation; Task 12.0 provides clear docs
   - **Validation**: Task 11.0 adds helpful CLI diagnostics

5. **Data Loss Between Snapshots**
   - **Risk**: In-memory data lost if crash happens between snapshots
   - **Mitigation**: Documented in Task 12.0 as expected behavior; configurable intervals
   - **Validation**: Task 8.0 tests edge cases and recovery scenarios

## Success Metrics

- [ ] All 13 tasks completed and tests passing
- [ ] `make lint` passes with zero warnings
- [ ] `make test` passes with >80% coverage for new code
- [ ] All PRD acceptance criteria met
- [ ] Memory store, resource store, and streaming work identically with miniredis
- [ ] Documentation published and examples runnable
- [ ] Migration path validated and documented

## Dependencies

### External Libraries (Added)
- `github.com/alicebob/miniredis/v2` - In-memory Redis server
- `github.com/dgraph-io/badger/v4` - BadgerDB for snapshot persistence

### No Breaking Changes
- All existing Redis-based deployments continue to work
- Default mode is "distributed" for backward compatibility
- Consumer code requires ZERO changes

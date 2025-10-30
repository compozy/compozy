# Three-Mode Configuration System - Task Summary

**Status**: Ready for Implementation
**Breaking Change**: Yes (acceptable in alpha)
**Estimated Duration**: 7-8 days with parallelization
**Files Affected**: ~50 files across 7 phases

---

## Overview

Replace the current two-mode system (standalone/distributed) with a three-mode system:
- **memory** (NEW DEFAULT): In-memory SQLite + embedded services (no persistence)
- **persistent**: File-based SQLite + embedded services (with persistence)
- **distributed**: PostgreSQL + external services (production)

**Key Benefits**:
- 50-80% faster test suite (no testcontainers/Docker startup)
- Zero-dependency quickstart (`compozy start` just works)
- Clearer intent-based naming (mode matches use case)

**Technical Specification**: See `_techspec.md` for detailed implementation

---

## Task List (29 Tasks)

### Phase 1: Core Configuration [CRITICAL] 🔴

**Blocking**: All other work depends on Phase 1 completion

- [x] **1.0** Update Mode Constants & Defaults [M] - 1 day
- [x] **2.0** Update Configuration Validation [M] - 1 day
- [x] **3.0** Update Configuration Registry [M] - 1 day
- [ ] **4.0** Update Configuration Tests [L] - 2 days

**Duration**: 2 days (with parallelization)

---

### Phase 2: Infrastructure Wiring [HIGH] 🟡

**Dependencies**: Phase 1 complete

- [x] **5.0** Update Cache Layer [M] - 1 day
- [x] **6.0** Update Temporal Wiring [L] - 2 days
- [x] **7.0** Update Server Logging [S] - 0.5 days
- **8.0** Manual Runtime Validation [M] - 1 day

**Duration**: 1.5 days (with parallelization)
**Parallel Lanes**: 3 (tasks 5.0, 6.0, 7.0 can run concurrently)

---

### Phase 3: Test Infrastructure [HIGH] 🟡

**Dependencies**: Phase 1, Phase 2 complete

- [x] **9.0** Update Test Helpers [M] - 1 day
- [x] **10.0** Add Database Mode Helper [S] - 0.5 days
- **11.0** Audit & Migrate Integration Tests [XL] - 3 days
- [x] **12.0** Update Integration Test Helpers [M] - 1 day
- [x] **13.0** Update Golden Test Files [S] - 0.5 days

**Duration**: 2 days (with parallelization)
**Parallel Lanes**: 4 initially (9.0, 10.0, 12.0, 13.0 can start together)

---

### Phase 4: Documentation [MEDIUM] 🟢

**Dependencies**: Phase 1 complete (can run parallel with Phases 2-3)

- [x] **14.0** Update Deployment Documentation [L] - 2 days
- [x] **15.0** Update Configuration Documentation [M] - 1 day
- [x] **16.0** Create Migration Guide [L] - 2 days
- [x] **17.0** Update Quick Start [S] - 0.5 days
- [x] **18.0** Update CLI Help [S] - 0.5 days
- [x] **19.0** Create/Update Examples [M] - 1 day

**Duration**: 1 day (with parallelization)
**Parallel Lanes**: 5 (all tasks can run concurrently)

---

### Phase 5: Template System [CRITICAL] 🔴

**Dependencies**: Phase 1 complete (can run parallel with Phases 2-3-4)

- **27.0** Add Mode Selection to TUI Form [M] - 0.5 days
- **28.0** Update Template System Types for Mode [S] - 0.5 days
- **29.0** Make Template Generation Mode-Aware [L] - 1 day

**Duration**: 1 day (with parallelization)
**Parallel Lanes**: 2 (tasks 27.0 and 28.0 can run in parallel, then 29.0)

**CRITICAL**: First impression for new users, affects onboarding experience

---

### Phase 6: Schemas & Metadata [MEDIUM] 🟢

**Dependencies**: Phase 1 complete (can run parallel with Phases 2-3-4-5)

- [x] **20.0** Update JSON Schemas [S] - 0.5 days
- **21.0** Regenerate Generated Files [M] - 1 day

**Duration**: 1 day (sequential)

---

### Phase 7: Final Validation [CRITICAL] 🔴

**Dependencies**: ALL previous phases complete

- **22.0** Comprehensive Testing [L] - 2 days (BLOCKING)
- **23.0** Validate Examples [M] - 1 day
- **24.0** Performance Benchmarking [M] - 1 day
- **25.0** Error Message Validation [S] - 0.5 days
- **26.0** Documentation Validation [S] - 0.5 days

**Duration**: 1 day (with parallelization after task 22.0)
**Parallel Lanes**: 3 (after comprehensive tests complete)

---

## Execution Strategy

### Critical Path (6.5 days)

```
Phase 1: Core Config [2 days] - BLOCKING
    ↓
Phase 2: Infrastructure [1.5 days]
    ↓
Phase 3: Tests [2 days]
    ↓
Phase 6: Validation [1 day]
```

### Parallel Optimization

**Week 1 (Days 1-2): Foundation**
- All hands on Phase 1 (Core Config) - CRITICAL BLOCKING

**Week 1 (Days 3-5): Parallel Tracks**
- **Track A**: Phase 2 (Infrastructure) - 1 developer
- **Track B**: Phase 4 (Documentation) - 1 developer
- **Track C**: Phase 5 (Schemas) - 1 developer
- **Day 4-5**: Phase 2 completes → start Phase 3 (Tests)

**Week 2 (Day 6): Validation & Ship**
- Phase 6 (Final Validation) - All tracks converge
- Ship readiness verification

---

## Batch Plan (Git Commits)

### Batch 1: Core Configuration
**Tasks**: 1.0, 2.0, 3.0, 4.0
**Commit**: `feat(config): add memory/persistent/distributed modes`

### Batch 2: Infrastructure Wiring
**Tasks**: 5.0, 6.0, 7.0
**Commit**: `feat(infra): wire three-mode system to runtime`

### Batch 3: Test Helpers
**Tasks**: 9.0, 10.0, 12.0, 13.0
**Commit**: `test: migrate test infrastructure to memory mode`

### Batch 4: Test Migration
**Tasks**: 11.0
**Commit**: `test: migrate integration tests to SQLite`

### Batch 5: Documentation & Templates
**Tasks**: 14.0, 15.0, 16.0, 17.0, 18.0, 19.0, 27.0, 28.0, 29.0
**Commit**: `docs: update for three-mode system and template generation`

### Batch 6: Schemas & Validation
**Tasks**: 20.0, 21.0, 22.0-26.0
**Commit**: `chore: update schemas and validate ship readiness`

---

## Parallelization Summary

| Phase | Sequential | Parallel | Savings |
|-------|-----------|----------|---------|
| Phase 1 | 5 days | 2 days | 60% |
| Phase 2 | 3.5 days | 1.5 days | 57% |
| Phase 3 | 6 days | 2 days | 67% |
| Phase 4 | 5.5 days | 1 day | 82% |
| Phase 5 | 2 days | 1 day | 50% |
| Phase 6 | 1.5 days | 1 day | 33% |
| Phase 7 | 3 days | 1 day | 67% |
| **TOTAL** | **27.5 days** | **7.5 days** | **73%** |

---

## Success Metrics

### Performance Targets
- ✅ Test suite: 50-80% faster (3-5 min → 45-90 sec)
- ✅ Server startup (memory): <1 second
- ✅ Server startup (persistent): <2 seconds
- ✅ No regressions in distributed mode

### Quality Targets
- ✅ All tests pass (`make test`)
- ✅ Linter clean (`make lint`)
- ✅ Code coverage >80%
- ✅ All examples work in each mode

### Documentation Targets
- ✅ All mode references updated
- ✅ Migration guide complete
- ✅ No inappropriate "standalone" references
- ✅ API docs regenerated

---

## Risk Mitigation

### High-Risk Tasks
1. **Task 11.0** (Test Migration): XL size, touches many files
   - Mitigation: Break into sub-tasks per test suite

2. **Task 6.0** (Temporal Wiring): Complex runtime behavior
   - Mitigation: Extensive manual testing (Task 8.0)

3. **Task 22.0** (Comprehensive Testing): Blocks ship
   - Mitigation: Continuous testing throughout phases

### Breaking Change Management
- Alpha version = acceptable breakage
- Clear migration guide (Task 16.0)
- Helpful error messages (Task 25.0)
- Version bump in CHANGELOG

---

## Definition of Done

### Code Complete
- [ ] All ~40 files updated
- [ ] All tests passing (`make test`)
- [ ] Linter clean (`make lint`)
- [ ] No "standalone" references (except historical)

### Quality Complete
- [ ] Performance benchmarked (50%+ faster)
- [ ] Examples tested in each mode
- [ ] State persists in persistent mode
- [ ] Error messages helpful and clear

### Documentation Complete
- [ ] All mode references updated
- [ ] Migration guide written and tested
- [ ] CHANGELOG entry written
- [ ] API docs regenerated

### Ship Ready
- [ ] Smoke tests pass in all modes
- [ ] No regressions in distributed mode
- [ ] Team reviewed and approved
- [ ] Version bumped appropriately

---

**Next Steps**: Begin with Phase 1 (Core Configuration) - all developers focus here first.

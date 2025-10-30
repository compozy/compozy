## status: pending

<task_context>
<domain>performance</domain>
<type>benchmarking</type>
<scope>system_performance</scope>
<complexity>medium</complexity>
<dependencies>all_previous_phases</dependencies>
</task_context>

# Task 24.0: Performance Benchmarking

## Overview

Measure and validate performance improvements across all modes, particularly focusing on test suite execution speed and server startup times.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from `_techspec.md` Phase 6.3 before start
- **DEPENDENCIES:** Tasks 1.0-22.0 must be completed
- **TARGET:** 50-80% improvement in test suite execution time
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

<research>
# When you need information about benchmarking:
- use perplexity to find out about Go benchmarking best practices
- check how to measure startup times accurately
</research>

<requirements>
- Test suite 50%+ faster than baseline (3-5 min → 45-90 sec)
- Memory mode startup <1 second
- Persistent mode startup <2 seconds
- Distributed mode startup 5-15 seconds (external connections)
- Document all performance metrics
</requirements>

## Subtasks

- [ ] 24.1 Benchmark test suite execution time (before/after)
- [ ] 24.2 Measure memory mode server startup time
- [ ] 24.3 Measure persistent mode server startup time
- [ ] 24.4 Measure distributed mode server startup time
- [ ] 24.5 Document performance improvements
- [ ] 24.6 Verify 50%+ improvement target met

## Implementation Details

See `_techspec.md` Phase 6.3 for complete implementation details.

### Benchmarking Commands

**Test suite performance:**
```bash
# Baseline (if available - with testcontainers)
time make test
# Expected baseline: 2-5 minutes

# Current (with SQLite memory mode)
time make test
# Target: 30-90 seconds (50-80% faster)
```

**Server startup benchmarks:**
```bash
# Memory mode
time compozy start --timeout 10s
# Target: <1 second

# Persistent mode
time compozy start --mode persistent --timeout 10s
# Target: <2 seconds

# Distributed mode
time compozy start --mode distributed --timeout 30s
# Target: 5-15 seconds
```

### Relevant Files

**Performance-critical code:**
- `engine/infra/server/server.go`
- `engine/infra/server/dependencies.go`
- `engine/infra/cache/mod.go`
- `test/helpers/database.go`

### Dependent Files

All test infrastructure from Tasks 3.1-3.5

## Deliverables

- Performance benchmark report with before/after metrics
- Test suite execution time comparison
- Server startup time measurements for all modes
- Verification that 50%+ improvement target is met
- Performance documentation for users

## Tests

- Performance validation:
  - [ ] Test suite execution time measured and documented
  - [ ] 50%+ improvement in test suite speed achieved
  - [ ] Memory mode startup <1 second
  - [ ] Persistent mode startup <2 seconds
  - [ ] Distributed mode startup 5-15 seconds
  - [ ] No performance regressions in distributed mode
  - [ ] Memory usage is reasonable in all modes
  - [ ] Database query performance is acceptable

## Success Criteria

- ✅ Test suite 50-80% faster than baseline
- ✅ Memory mode: <1s startup
- ✅ Persistent mode: <2s startup
- ✅ Distributed mode: 5-15s startup (external services)
- ✅ Performance metrics documented
- ✅ No performance regressions in any mode

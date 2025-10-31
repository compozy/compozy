## status: pending

<task_context>
<domain>testing</domain>
<type>integration</type>
<scope>full_system</scope>
<complexity>high</complexity>
<dependencies>all_previous_phases</dependencies>
</task_context>

# Task 22.0: Comprehensive Testing

## Overview

Execute full test suite validation across all three modes, verify performance improvements, and ensure zero regressions. This is a critical validation gate before ship.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from `_techspec.md` Phase 6.1 before start
- **DEPENDENCIES:** All previous tasks (1.0-21.0) must be completed
- **BLOCKING:** This is a CRITICAL validation gate - must pass before ship
</critical>

<research>
# When you need information about testing:
- use perplexity to find out about Go testing best practices
- check existing test patterns in test/helpers/
</research>

<requirements>
- Full test suite passes (`make test`)
- Linter passes with zero warnings (`make lint`)
- Performance improvement of 50%+ in test execution time
- All three modes tested individually
- No regressions in any mode
</requirements>

## Subtasks

- [ ] 22.1 Clean build and full test suite execution
- [ ] 22.2 Linter validation (zero warnings)
- [ ] 22.3 Memory mode testing (default behavior)
- [ ] 22.4 Persistent mode testing (with state persistence)
- [ ] 22.5 Distributed mode testing (no regressions)
- [ ] 22.6 Performance benchmarking and validation

## Implementation Details

See `_techspec.md` Phase 6.1 for complete implementation details.

### Test Commands

**Full test suite:**
```bash
make clean
make build
make test  # Expected: All pass, 50%+ faster
```

**Linter:**
```bash
make lint  # Expected: Zero warnings
```

**Mode-specific testing:**
```bash
# Memory mode (default)
compozy start
compozy workflow run examples/hello-world.yaml

# Persistent mode
compozy start --mode persistent
# Restart to verify persistence
compozy start --mode persistent

# Distributed mode
docker-compose up -d postgres redis temporal
compozy start --mode distributed
```

### Relevant Files

**Test infrastructure:**
- `test/helpers/standalone.go`
- `test/helpers/database.go`
- `test/integration/*/`

**Core files to validate:**
- `pkg/config/resolver.go`
- `engine/infra/cache/mod.go`
- `engine/infra/server/dependencies.go`

### Dependent Files

All files from Tasks 1.0-21.0

## Deliverables

- Full test suite passing
- Linter clean (zero warnings)
- Performance benchmark report showing 50%+ improvement
- All three modes validated and working
- Test execution time documented (before/after)

## Tests

- Comprehensive validation:
  - [ ] Full test suite passes (`make test`)
  - [ ] No test failures or flaky tests
  - [ ] Linter clean (`make lint`)
  - [ ] Memory mode: server starts and executes workflows
  - [ ] Persistent mode: state persists across restarts
  - [ ] Distributed mode: connects to external services
  - [ ] Performance: test suite 50%+ faster than baseline
  - [ ] No regressions in code coverage
  - [ ] All pgvector tests explicitly use distributed mode

## Success Criteria

- ✅ All tests pass (`make test`)
- ✅ Linter clean (`make lint`)
- ✅ Test suite 50%+ faster (baseline: 3-5 min → target: 45-90 sec)
- ✅ All three modes work correctly
- ✅ No regressions in distributed mode
- ✅ Code coverage >80% for new code

# Temporal Standalone Mode - Task Breakdown

Complete task breakdown for implementing embedded Temporal server in Compozy.

## 📋 Planning Documents

- **`_techspec.md`** - Technical specification and implementation design
- **`_docs.md`** - Documentation plan (7 pages)
- **`_examples.md`** - Examples plan (7 projects)
- **`_tests.md`** - Testing plan (unit + integration)
- **`_tasks.md`** - Task summary with execution plan and dependencies

## 🎯 Task Files

### Foundation (3 days)
- **`_task_01.md`** - Embedded Server Package Foundation (L)
  - Creates `engine/worker/embedded/` package
  - Config types, validation, builders
  - Blocks all other tasks

### Core Development (2 days, parallel after Task 1)
- **`_task_02.md`** - Embedded Server Lifecycle (M)
  - Server Start/Stop implementation
  - Ready-state polling
  
- **`_task_03.md`** - Configuration System Extension (M)
  - Mode selection and standalone config
  - Registry entries and defaults

### Integration (2 days)
- **`_task_04.md`** - UI Server Implementation (M)
  - Optional Web UI wrapper
  - Can start after Task 2
  
- **`_task_05.md`** - Server Lifecycle Integration (M)
  - Dependencies.go integration
  - Requires Tasks 2 & 3

### Validation & Polish (3 days, parallel after Task 5)
- **`_task_06.md`** - Core Integration Tests (L)
  - Critical path validation
  
- **`_task_07.md`** - CLI & Schema Updates (S)
  - CLI flags and JSON schema

### Documentation (2-3 days, parallel after Task 5)
- **`_task_08.md`** - Documentation (L)
  - 7 documentation pages
  
- **`_task_09.md`** - Examples (L)
  - 7 example projects

### Advanced Testing (2 days, parallel after Task 5)
- **`_task_10.md`** - Advanced Integration Tests (M)
  - Error handling and edge cases

## 🚀 Quick Start

1. Read `_techspec.md` for implementation details
2. Start with `_task_01.md` (foundation)
3. Follow dependency order in `_tasks.md`
4. Reference `_tests.md` for test requirements
5. Use `_docs.md` and `_examples.md` for user-facing deliverables

## 📊 Execution Timeline

**Critical Path:** 10 days (1.0 → 2.0||3.0 → 5.0 → 6.0)
**Total Effort:** ~20 developer-days
**Parallelization:** Can be completed in 10-11 days with 2-3 developers

## 🎯 Success Criteria

- [ ] Workflows execute in standalone mode
- [ ] Web UI accessible at http://localhost:8233
- [ ] File-based persistence works
- [ ] In-memory mode works
- [ ] All tests pass (`make test`)
- [ ] No linter errors (`make lint`)
- [ ] Documentation complete
- [ ] Examples tested and working

## 🔗 Reference

- Reference implementation: https://github.com/abtinf/temporal-a-day/blob/main/001-all-in-one-hello/main.go
- Temporal server package: `go.temporal.io/server`
- UI server package: `go.temporal.io/server/ui-server/v2`

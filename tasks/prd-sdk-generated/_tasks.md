# SDK Code Generation Migration Task Summary

## Overview

Migrate all SDK packages from manual builder pattern to auto-generated functional options pattern. This eliminates 70-75% of boilerplate code and reduces maintenance overhead from 30-40 lines per field to 0 lines (just `go generate`).

**Status:** 50% Complete (agent + agentaction done, 9 packages + 9 examples remaining)

## Relevant Files

### Code Generation Infrastructure (Complete)

- `sdk2/internal/codegen/types.go` - Struct metadata types
- `sdk2/internal/codegen/parser.go` - AST parser for field discovery
- `sdk2/internal/codegen/generator.go` - Jennifer-based code generator
- `sdk2/internal/codegen/cmd/optionsgen/main.go` - CLI tool with -package flag

### Reference Implementations (Complete)

- `sdk2/agent/` - Agent package migration (14 fields, complex embedded structs)
- `sdk2/agentaction/` - Action package migration (12 fields, separate package pattern)
- `sdk2/MIGRATION_GUIDE.md` - Comprehensive migration documentation
- `sdk2/CODEGEN_COMPARISON.md` - Before/after metrics

### Packages to Migrate

#### Phase 1 - Foundations (5 packages)
- Reference: `sdk/model/builder.go` (257 LOC) → Create: `sdk2/model/`
- Reference: `sdk/schedule/builder.go` (174 LOC) → Create: `sdk2/schedule/`
- Reference: `sdk/mcp/builder.go` (117 LOC) → Create: `sdk2/mcp/`
- Reference: `sdk/runtime/builder.go` (150 LOC) → Create: `sdk2/runtime/`
- Reference: `sdk/memory/builder.go` (200 LOC) → Create: `sdk2/memory/`

#### Phase 2 - Components (4 packages)
- Reference: `sdk/tool/builder.go` (239 LOC) → Create: `sdk2/tool/`
- Reference: `sdk/schema/builder.go` (180 LOC) → Create: `sdk2/schema/` (special: new approach)
- Reference: `sdk/workflow/builder.go` (198 LOC) → Create: `sdk2/workflow/`
- Reference: `sdk/knowledge/builder.go` (250+ LOC) → Create: `sdk2/knowledge/` (4 different types)

#### Phase 3 - Complex Integration (2 packages)
- Reference: `sdk/task/builder.go` (300+ LOC) → Create: `sdk2/task/` (7+ task type variants)
- Reference: `sdk/project/builder.go` (460+ LOC) → Create: `sdk2/project/` (15+ fields, orchestrator)

### Examples to Update

- `sdk/cmd/01_simple_workflow/main.go`
- `sdk/cmd/02_parallel_tasks/main.go`
- `sdk/cmd/03_knowledge_rag/main.go`
- `sdk/cmd/04_memory_conversation/main.go`
- `sdk/cmd/06_runtime_native_tools/main.go`
- `sdk/cmd/07_scheduled_workflow/main.go`
- `sdk/cmd/08_signal_communication/main.go`
- `sdk/cmd/10_complete_project/main.go`
- `sdk/cmd/11_debugging/main.go`

## Tasks

### Phase 1: Foundations (Parallelizable)
- [x] [_task_1.md](_task_1.md) - 1.0 Migrate model package (S) - **COMPLETED**
- [x] [_task_2.md](_task_2.md) - 2.0 Migrate schedule package (S) - **COMPLETED**
- [x] [_task_3.md](_task_3.md) - 3.0 Migrate mcp package (S) - **COMPLETED**
- [x] [_task_4.md](_task_4.md) - 4.0 Migrate runtime package (S) - **COMPLETED**
- [x] [_task_5.md](_task_5.md) - 5.0 Migrate memory package (S) - **COMPLETED**

### Phase 2: Components (Sequential after Phase 1)
- [x] [_task_6.md](_task_6.md) - 6.0 Migrate tool package (M) - **COMPLETED**
- [x] [_task_7.md](_task_7.md) - 7.0 Migrate schema package (M) - **COMPLETED**
- [x] [_task_8.md](_task_8.md) - 8.0 Migrate workflow package (M) - **COMPLETED**
- [ ] 9.0 Migrate knowledge package (M)

### Phase 3: Complex Integration (Sequential after Phase 2)
- [ ] 10.0 Migrate task package (L)
- [ ] 11.0 Migrate project package (L)

### Phase 4: Examples (After Phase 1 complete)
- [ ] 12.0 Update example files (S)

Notes on sizing:
- S = Small (≤ 2 hours)
- M = Medium (2-4 hours)
- L = Large (4-8 hours)

## Task Design Rules

- Each parent task is a closed deliverable: independently shippable and reviewable
- Each task follows standard migration pattern: generate.go → go generate → constructor.go + tests → delete old files
- All tasks must pass `make lint` and `make test` before completion
- Greenfield approach: delete old builder files immediately, no backwards compatibility
- Use reference implementations (agent/agentaction) as templates

## Execution Plan

### Critical Path
1.0 → 2.0 → 3.0 → 4.0 → 5.0 → 6.0 → 7.0 → 8.0 → 9.0 → 10.0 → 11.0 → 12.0

### Parallel Opportunities

**Track A (Foundation):** Tasks 1.0-5.0 can run in parallel (independent packages)
**Track B (Examples):** Task 12.0 can start after 1.0-5.0 complete (doesn't need Phase 2/3)

### Dependency Graph

```
Phase 1 (Parallel):
├─ 1.0 model
├─ 2.0 schedule
├─ 3.0 mcp
├─ 4.0 runtime
└─ 5.0 memory
    ↓
Phase 2 (Sequential):
├─ 6.0 tool (needs model)
├─ 7.0 schema (needs model)
├─ 8.0 workflow (needs model)
└─ 9.0 knowledge (needs model)
    ↓
Phase 3 (Sequential):
├─ 10.0 task (needs workflow, tool)
└─ 11.0 project (needs all packages)
    ↓
Phase 4 (After Phase 1):
└─ 12.0 examples (needs model, workflow)
```

## Notes

- **SDK2 Parallel Development:** All work happens in `sdk2/` directory, `sdk/` remains untouched
- **No Deletions During Migration:** Do not delete anything from `sdk/` until sdk2 is complete
- **Greenfield in SDK2:** Build fresh in sdk2, no backwards compatibility with sdk
- **Package Naming:** Use separate packages if Option type conflicts (e.g., agentaction)
- **Validation Centralization:** Move all validation to constructor
- **Deep Copy Required:** Always clone configs before returning
- **Context First:** ctx must be first parameter in all constructors
- All runtime code MUST use `logger.FromContext(ctx)` and `config.FromContext(ctx)`
- Run `make fmt && make lint && make test` before marking any task as completed
- **SDK Cleanup:** After sdk2 complete, separately decide what to do with old sdk/

## Batch Plan (Grouped Commits)

- [ ] Batch 1 — Foundation Layer: 1.0, 2.0, 3.0, 4.0, 5.0 (Can be single PR or 5 separate PRs)
- [ ] Batch 2 — Component Layer: 6.0, 7.0, 8.0, 9.0 (Single PR recommended)
- [ ] Batch 3 — Integration Layer: 10.0, 11.0 (Separate PRs recommended due to size)
- [ ] Batch 4 — Examples Update: 12.0 (Single PR with all examples)

## Metrics

**Code Reduction:**
- Before: ~4,000-5,000 LOC manual builder code
- After: ~800-1,200 LOC (generated + constructors)
- Reduction: 70-75%

**Maintenance Impact:**
- Before: 30-40 lines per new engine field
- After: 0 lines (`go generate` handles it)
- ROI: Breaks even after ~2 months (3-5 field additions)

**Time Estimate:**
- Phase 1: 5 hours (1 hour if parallelized with 5 developers)
- Phase 2: 9 hours
- Phase 3: 12 hours
- Phase 4: 2 hours
- **Total: 28 hours single developer, ~10 hours with 3-person team**

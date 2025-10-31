# SDK v2 Compozy Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `sdk2/compozy/constructor.go` - New engine constructor and option wiring.
- `sdk2/compozy/types.go` - Shared request/response and mode definitions.
- `sdk2/compozy/engine.go` - Engine struct, lifecycle hooks, and client integration.
- `sdk2/compozy/mode.go` - Mode configuration objects and validation.
- `sdk2/compozy/loader.go` - YAML loading helpers and resource registrars.
- `sdk2/compozy/validation.go` - Dependency graph validation utilities.

### Integration Points

- `sdk2/internal/sdkcodegen/*` - Code generation specs and generators for options, execution, loading, registration.
- `sdk2/client` - HTTP transport used by engine execution methods.
- `engine/resources` & `engine/infra/*` - Resource store and infrastructure dependencies for mode wiring.
- `pkg/config`, `pkg/logger` - Context-based config and logging integration.

## Tasks

- [x] 1.0 Establish SDK foundation and constructor (M)
- [x] 2.0 Implement code generation pipeline for SDK resources (L)
- [x] 3.0 Build engine core lifecycle and client integration (L)
- [ ] 4.0 Deliver standalone and distributed mode orchestration (L)
- [ ] 5.0 Implement resource loading and validation layer (M)
- [ ] 6.0 Complete Test Coverage (M)

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

- Critical Path: 1.0 → 2.0 → 3.0 → 4.0 → 6.0
- Parallel Track A (after 3.0): 4.0 focuses on mode orchestration and infrastructure wiring
- Parallel Track B (after 3.0): 5.0 tackles resource loading, validation, and dependency graph; completes before 6.0
- Risk watch: embedded Temporal/Redis bootstrapping, generated code drift, and streaming client parity

Notes

- All runtime code MUST use `logger.FromContext(ctx)` and `config.FromContext(ctx)`
- Run `make fmt && make lint && make test` before marking any task as completed

## Batch Plan (Grouped Commits)

- [x] Batch 1 — Foundation & Codegen: 1.0, 2.0
- [ ] Batch 2 — Engine & Runtime: 3.0, 4.0, 5.0
- [ ] Batch 3 — Test Coverage: 6.0

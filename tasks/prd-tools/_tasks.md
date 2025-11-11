# [Feature] Tooling Refactor Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `engine/tool/config.go` - Extend tool configuration model with implementation modes and validation
- `engine/llm/service.go` - Register native Go tools and runtime-backed adapters
- `engine/llm/tool_registry.go` - Add adapters for native handlers and maintain execution flow
- `engine/runtime/bun_manager.go` - Wire generated entrypoint for inline code execution
- `sdk/tool/constructor.go` - Expose SDK options for native handler registration and inline code

### Integration Points

- `sdk/compozy/engine.go` - Initialize inline manager and attach to engine lifecycle
- `engine/resources/store.go` - Ensure tool watcher hooks subscribe for inline sync
- `engine/tool/router` - Validate REST payloads against new implementation field

### Documentation Files

- `sdk/tool/README.md` - Document Go-native tooling usage
- `docs/content/docs/core/tools/runtime-environment.mdx` - Update runtime tooling guidance

### Examples (if applicable)

- `sdk/examples/05_runtime_native_tools.go` - Expand to cover Go-native and inline TypeScript coexistence

## Tasks

- [x] 1.0 Go-Native Tooling Infrastructure (L)
- [x] 2.0 Inline TypeScript Execution Pipeline (L)
- [ ] 3.0 Validation, Testing, and Documentation Hardening (M)

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

- Critical Path: 1.0 → 2.0 → 3.0
- Parallel Track A (after 2.0): Targeted docs/SDK polish from 3.0 may begin once inline pipeline stabilizes
- Parallel Track B: None identified

Notes

- All runtime code MUST use `logger.FromContext(ctx)` and `config.FromContext(ctx)`
- Run `make fmt && make lint && make test` before marking any task as completed

## Batch Plan (Grouped Commits)

- [x] Batch 1 — Native Foundations: 1.0
- [ ] Batch 2 — Runtime & Quality: 2.0, 3.0

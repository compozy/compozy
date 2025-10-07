# [Feature] Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `path/to/file.go` - Description

### Integration Points

- `path/to/integration.go` - Description

### Documentation Files

- `docs/feature.md` - User documentation

### Examples (if applicable)

- `examples/feature/*` - Example configurations and assets

## Tasks

- [ ] 1.0 Parent Task Title (S/M/L)
- [ ] 2.0 Parent Task Title (S/M/L)
- [ ] 3.0 Parent Task Title (S/M/L)

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
- Parallel Track A (after X.Y): ...
- Parallel Track B (after X.Y): ...

Notes

- All runtime code MUST use `logger.FromContext(ctx)` and `config.FromContext(ctx)`
- Run `make fmt && make lint && make test` before marking any task as completed

## Batch Plan (Grouped Commits)

- [ ] Batch 1 — Name/Theme: 1.0, 2.0
- [ ] Batch 2 — Name/Theme: 3.0, 4.0

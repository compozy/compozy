# Memory Protocol

`cy-loop-tasks` reuses the existing `cy-workflow-memory` skill rather than
inventing a new memory format. Every iteration that touches code MUST update
memory **before** flipping any status field, mirroring step 5 of
`cy-execute-task`.

## Paths passed to cy-workflow-memory

| Param (cy-workflow-memory naming) | Value used by cy-loop-tasks |
|-----------------------------------|------------------------------|
| `workflow memory directory` | `.compozy/tasks/<slug>/memory/` |
| `shared workflow memory path` | `.compozy/tasks/<slug>/memory/MEMORY.md` |
| `current task memory path` | depends on phase — see below |

## Current-memory file per phase

| Phase + mode | `current task memory path` |
|--------------|------------------------------|
| Phase 0 (bootstrap) | none — only `MEMORY.md` is created |
| Phase B, mode=tasks | `.compozy/tasks/<slug>/memory/<task_NN>.md` (matches the `task_NN.md` being executed) |
| Phase B, mode=free | `.compozy/tasks/<slug>/memory/free-iter-<NNN>.md` (zero-padded 3 digits, equal to the `iteration` value on the checklist item just created by `update-state.py --add-progress`) |
| Phase C, qa_report | `.compozy/tasks/<slug>/memory/qa-report.md` |
| Phase C, qa_execution | `.compozy/tasks/<slug>/memory/qa-execution.md` |
| Phase D, peer review | `.compozy/tasks/<slug>/memory/peer-review.md` (one file; one `## Round <N>` section per round) |
| Phase E | none — Phase E only emits the done-signature |

## Lane ownership

In delegated iterations (frontend tasks/slices, `qa_report`), the herdr
worker updates the current memory file and any `MEMORY.md` promotions — the
paths travel in its delegation packet. The orchestrator verifies those
writes before flipping state. In local iterations the orchestrator writes
memory itself.

## Section schema (per current-memory file)

Reuse the canonical task-memory sections from `cy-workflow-memory`:

```markdown
# Task Memory: <name>

## Objective Snapshot
## Important Decisions
## Learnings
## Files / Surfaces
## Errors / Corrections
## Ready for Next Run
```

Phase-specific addenda (append after the canonical sections):

- **Phase B**: record scoped-validation commands, intermediate failures and
  repairs under `## Errors / Corrections`, and the final `cy-final-verify`
  PASS evidence under `## Ready for Next Run`. A final FAIL appears only with
  a proven external blocker.
- **Phase B mode=free**: add `## Slice Picked` (the exact text added to
  `progress.checklist[]`) and `## Acceptance Mapping` (which techspec
  acceptance criterion this slice advances).
- **Phase C**: add `## QA Artifacts Produced` (paths under `docs/qa/`).
- **Phase D**: each `## Round <N>` section records the verdict, the findings
  artifact path, and the remediated blockers/nits.

## Promotion rules (current → MEMORY.md)

A finding gets promoted to `MEMORY.md` `## Shared Decisions` or
`## Shared Learnings` only when **all three** are true (per
`cy-workflow-memory`):

1. Another iteration would need this info to avoid the same mistake.
2. It is durable across multiple iterations (not just this slice).
3. It is not already obvious from `_techspec.md` or the repo.

## Compaction

Soft caps (from `cy-workflow-memory`): `MEMORY.md` ~150 lines / 12KB,
per-iteration files ~200 lines / 16KB. When a file exceeds the cap, run
`cy-workflow-memory` compaction during the next Phase B iteration before
adding new content.

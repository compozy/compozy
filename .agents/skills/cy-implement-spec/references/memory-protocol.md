# Memory Protocol

`cy-implement-spec` reuses the existing `cy-workflow-memory` skill rather
than inventing a new memory format. Every iteration that touches code MUST
update memory **before** flipping any status field.

Memory carries extra weight in this loop: Phase B is long-running with no
task files to re-read, so after any context loss or compaction the agent
re-anchors from exactly three sources — the spec documents (contract),
`state.yaml` (progress), and these memory files (decisions). Anything worth
surviving a compaction goes here, not in conversation history.

## Paths passed to cy-workflow-memory

| Param (cy-workflow-memory naming) | Value used by cy-implement-spec |
|-----------------------------------|----------------------------------|
| `workflow memory directory` | `.compozy/tasks/<slug>/memory/` |
| `shared workflow memory path` | `.compozy/tasks/<slug>/memory/MEMORY.md` |
| `current task memory path` | depends on phase — see below |

## Current-memory file per phase

| Phase | `current task memory path` |
|-------|------------------------------|
| Phase 0 (bootstrap) | none — only `MEMORY.md` is created |
| Phase B | `.compozy/tasks/<slug>/memory/impl-iter-<NNN>.md` — `<NNN>` = `state.iteration + 1` at milestone start, zero-padded to three digits (stable through the iteration because exactly one `update-state.py` call closes it) |
| Phase C, qa_report | `.compozy/tasks/<slug>/memory/qa-report.md` |
| Phase C, qa_execution | `.compozy/tasks/<slug>/memory/qa-execution.md` |
| Phase D, peer review | `.compozy/tasks/<slug>/memory/peer-review.md` (one file; one `## Round <N>` section per round) |
| Phase E | none — Phase E only emits the done-signature |

## Lane ownership

In the delegated `qa_report` iteration, the herdr worker updates the current
memory file and any `MEMORY.md` promotions — the paths travel in its
delegation packet. The orchestrator verifies those writes before flipping
state. In every other iteration the orchestrator writes memory itself.

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

- **Phase B**: add `## Milestone` (the exact text passed to
  `commit-checkpoint.py --milestone`) and `## Criteria Advanced` — one line
  per criterion flipped to `met` this iteration, each naming its proof
  (command + result, test id, or artifact path). Record scoped-validation
  commands, intermediate failures and repairs under
  `## Errors / Corrections`, and the final `cy-final-verify` PASS evidence
  under `## Ready for Next Run`. A final FAIL appears only with a proven
  external blocker.
- **Phase C**: add `## QA Artifacts Produced` (paths under `docs/qa/`).
- **Phase D**: each `## Round <N>` section records the verdict, the findings
  artifact path, and the remediated blockers/nits.

## Promotion rules (current → MEMORY.md)

A finding gets promoted to `MEMORY.md` `## Shared Decisions` or
`## Shared Learnings` only when **all three** are true (per
`cy-workflow-memory`):

1. Another iteration would need this info to avoid the same mistake.
2. It is durable across multiple iterations (not just this milestone).
3. It is not already obvious from the spec documents or the repo.

## Compaction

Soft caps (from `cy-workflow-memory`): `MEMORY.md` ~150 lines / 12KB,
per-iteration files ~200 lines / 16KB. When a file exceeds the cap, run
`cy-workflow-memory` compaction during the next Phase B iteration before
adding new content.

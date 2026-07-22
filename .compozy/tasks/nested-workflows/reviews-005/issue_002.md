---
provider: manual
pr:
round: 5
round_created_at: 2026-07-22T21:45:58Z
status: resolved
file: internal/core/task_group_completion.go
line: 253
severity: high
author: claude-code
provider_ref:
---

# Issue 002: Completion ignores an invalid Task Group task manifest

## Review Comment

`taskCompletionEvidence` scans `task_*.md` files through `SnapshotTaskMeta`, but it never reads or validates the selected Task Group's `_tasks.md`. A directory containing completed task files can therefore pass both completion-evidence checks even when its v2 manifest has the wrong `workflow`, dangling or missing nodes, a cycle, or concurrent corruption. The bridge then records the lifecycle checkbox for a suite that the normal task runner cannot execute from its declared graph.

Load and validate the current manifest against the exact `<initiative>/TG-NNN` reference during every completion-evidence pass, and derive terminal state from the manifest-owned nodes rather than an unconstrained directory walk. Add pre-write and post-write mutation tests for malformed workflow identity, missing nodes, and graph corruption.

## Triage

- Decision: `VALID`
- Notes: `taskCompletionEvidence` calls `tasks.SnapshotTaskMeta`, which discovers every
  `task_*.md` in the directory without reading `_tasks.md`. The daemon's task runner instead
  uses `tasks.LoadValidatedTaskGraphManifest`, so completion can currently accept evidence
  that the executable workflow rejects (including a mismatched workflow identity, missing
  manifest-owned files, or an invalid graph). The completion bridge now loads and validates
  `_tasks.md` against the exact selected `<initiative>/TG-NNN` reference on every evidence
  pass and derives terminal state only from the validated manifest-owned task entries.
  Regression coverage mutates workflow identity, manifest-owned file presence, and graph
  cycles both before the durable write and after its pre-write validation, proving both
  evidence passes reject the completion. The repository verification pipeline passes with
  the repaired production path and fixtures.

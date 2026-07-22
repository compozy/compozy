---
provider: manual
pr:
round: 5
round_created_at: 2026-07-22T21:45:58Z
status: pending
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

- Decision: `UNREVIEWED`
- Notes:

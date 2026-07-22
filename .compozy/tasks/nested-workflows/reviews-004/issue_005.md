---
provider: manual
pr:
round: 4
round_created_at: 2026-07-22T19:59:29Z
status: resolved
file: internal/daemon/query_service.go
line: 685
severity: high
author: codex
provider_ref:
---

# Issue 005: Task-group plan excerpts bypass durable snapshots

## Review Comment

`resolveSpecificationReadTarget` correctly loads the parent initiative's artifact snapshots, and the other specification readers prefer those durable bodies. `readTaskGroupPlanExcerpt` ignores the `_task_groups.md` snapshot and directly reads and stats line 685's filesystem path. An archived task group whose archive directory is unavailable therefore fails even though its synchronized plan body is durable. It can also read a different filesystem generation from the one represented by the selected archived workflow row.

Build the excerpt from the parent snapshot when one exists, using the snapshot timestamp for `UpdatedAt`, and only fall back to the filesystem for active unsnapshotted state under the same rules as the other document readers. Add coverage for an archived task group after its archive directory is removed and for a filesystem plan that differs from the selected durable generation.

## Triage

- Decision: `VALID`
- Notes: `resolveSpecificationReadTarget` correctly replaces the task-group target with the exact parent workflow row and loads that row's durable artifact snapshots. `readTaskGroupPlanExcerpt` then ignored `snapshotsByPath`, read `_task_groups.md` from `rootDir`, and derived `UpdatedAt` with `os.Stat`. This made an archived task-group spec depend on archive-directory availability and could combine the selected DB generation with another filesystem generation. The fix reconstructs and parses the plan from the parent `_task_groups.md` snapshot when present, uses its `SourceMTime`, and retains the existing filesystem fallback only when no snapshot exists. Integration coverage removes an archived directory and replaces an archived filesystem plan with divergent content; both reads remain bound to the durable generation. Full `make verify` passed after the implementation.

---
provider: manual
pr:
round: 7
round_created_at: 2026-07-23T00:22:01Z
status: resolved
file: internal/core/archive.go
line: 464
severity: high
author: claude-code
provider_ref:
---

# Issue 002: Forced archive can mutate artifacts of a newly started run

## Review Comment

`archiveTaskGroupInitiative` checks the initiative hierarchy for active runs at line 451, then `forceArchiveInitiative` completes child task files and resolves review issues before refreshing the active-run state. A run can start after the first snapshot but before those writes. The second check at line 476 then rejects the archive, yet the forced task/review mutations are not rolled back, so the active run continues against artifacts that an unsuccessful archive operation silently completed or resolved. The database archive transaction's later active-run guard cannot protect these earlier filesystem writes.

Coordinate run start and forced archive mutation with one lifecycle exclusion or durable archive reservation that both paths enforce before touching artifacts. Do not expose forced file mutations until the no-active-run condition is held through the archive transition, or make the mutations transactional and reversible. Add a deterministic concurrency test that starts a child run through a seam between the first guard and `forceArchiveInitiative`, then asserts the archive fails without changing any task or review file.

## Triage

- Decision: `VALID`
- Root cause: `archiveTaskGroupInitiative` performs the forced task and review rewrites before its refreshed active-run check and before `MarkWorkflowHierarchyArchived` applies the transactional database guard. Every error after those rewrites returns without restoring the original files, so a child run inserted after the first guard makes the archive fail while leaving task and review artifacts resolved.
- Fix: snapshot the initiative child files before force mutation, capture the exact force-produced changes, and roll those changes back on every later archive error. Keep rollback conflict-safe by restoring only files that still match the force-produced bytes, so concurrent artifact edits are never overwritten. Commit the snapshot only after the hierarchy archive succeeds. Add a deterministic service-integration regression test that injects a real active child run through the force-mutation seam and verifies both task and review bytes remain unchanged.

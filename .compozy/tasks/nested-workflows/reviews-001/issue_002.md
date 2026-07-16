---
provider: manual
pr:
round: 1
round_created_at: 2026-07-16T07:23:44Z
status: resolved
file: internal/store/globaldb/migrations.go
line: 189
severity: high
author: claude-code
provider_ref:
---

# Issue 002: Child foreign keys prevent pruning removed initiatives

## Review Comment

The hierarchy migration adds `parent_workflow_id REFERENCES workflows(id)` with the default restrictive delete behavior. `PruneMissingActiveWorkflows` deliberately skips child rows and deletes a missing top-level workflow directly. For any initiative that still has package child rows, SQLite rejects that parent delete with `FOREIGN KEY constraint failed`, so a successful root sync cannot prune an initiative removed from disk.

The existing prune test covers ordinary workflows only and therefore misses the new hierarchy. Implement hierarchy-aware pruning in one transaction: evaluate active runs across the parent and children, preserve/skip the aggregate when any child is active, and otherwise delete children before the parent. An `ON DELETE CASCADE` migration can be part of the solution, but it still needs aggregate active-run protection so live child runs are not orphaned. Add a real-SQLite test for a missing initiative with children and for the active-child-run case.

## Triage

- Decision: `VALID`
- Root cause: migration v6 adds `workflows.parent_workflow_id` with SQLite's default `ON DELETE RESTRICT`. `PruneMissingActiveWorkflows` skips child rows but deletes a missing root directly, so an initiative with active child rows fails deletion with `FOREIGN KEY constraint failed`.
- Fix approach: extend the prune transaction in `sync.go` to treat an initiative and its children as one aggregate: count live runs for every member, skip the root when any member is active, otherwise delete children before the parent. `sync.go` and `sync_test.go` are necessary additions outside the review's tagged migration line because a cascade-only migration cannot preserve an aggregate that has an active child run.
- Verification: added real-SQLite regression coverage for both deletion of a missing initiative with children and retention when a child run is active. `make verify` passed after the final change.

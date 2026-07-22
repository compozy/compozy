---
provider: manual
pr:
round: 4
round_created_at: 2026-07-22T19:59:29Z
status: resolved
file: internal/core/taskgroups/completion.go
line: 352
severity: critical
author: codex
provider_ref:
---

# Issue 001: Completion rollback can overwrite a newer task-group plan

## Review Comment

`writeValidatedCompletion` atomically writes the completed plan, runs the post-write evidence validator, and unconditionally restores the bytes captured before the write when validation fails. The sidecar flock only coordinates callers that voluntarily use this store. A human editor, authoring skill, or other filesystem writer can replace `_task_groups.md` while the validator runs; the rollback at line 352 then overwrites that newer plan with stale bytes.

Make rollback compare-and-swap safe: restore the original only when the current file still exactly matches the completion rewrite written by this call. If the file changed, preserve the newer bytes and return `ErrCompletionConflict` with enough evidence for recovery. Add a deterministic regression test that mutates the plan during post-write validation and proves the external update survives.

## Triage

- Decision: `VALID`
- Notes: `writeValidatedCompletion` performed its second evidence validation after atomically writing the completion rewrite, but its failure path unconditionally rewrote the pre-operation bytes. The sidecar lock cannot prevent an external editor from replacing `_task_groups.md` during validation, so that rollback could destroy a newer plan. The fix now compares the current plan bytes with this call's exact completion rewrite before restoring the original; a mismatch preserves the current file and returns `ErrCompletionConflict` with the plan path and task-group context. A deterministic regression test replaces the plan from the second validator and proves that the replacement survives. Full `make verify` passed after the production and test changes.

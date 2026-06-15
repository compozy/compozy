---
provider: coderabbit
pr: "201"
round: 2
round_created_at: 2026-06-15T19:35:34.422493Z
status: resolved
file: internal/core/run/ui/view.go
line: 126
severity: major
author: coderabbitai[bot]
provider_ref: review:4500514537,nitpick_hash:0674b9aeddea
review_hash: 0674b9aeddea
source_review_id: "4500514537"
source_review_submitted_at: "2026-06-15T19:35:05Z"
---

# Issue 001: Handle canceled/crashed terminal states explicitly in runChipStatus.
## Review Comment

Line 133 can currently render `DONE` for non-success terminal outcomes (for example canceled/crashed paths with `failed == 0`). This misreports final run state in the workflow chip.

<!-- cr-comment:v1:dc68a670828a3cb4d4c620f6 -->

## Triage

- Decision: `valid`
- Root cause: `runChipStatus` derives the workflow-chip terminal label only from aggregate job counters. When a daemon snapshot or stream has a terminal run status such as `canceled` or `crashed` but no failed job count, `m.isRunComplete()` can evaluate true and render `DONE`.
- Fix approach: Preserve the daemon aggregate run status in the UI model, update it from remote snapshots and run-level terminal events, and have `runChipStatus` map `failed`, `canceled`, and `crashed` before falling back to job counters.
- Scope note: The status source is dropped in remote bootstrap/translation before `view.go` can read it, so the minimal production fix must touch `internal/core/run/ui/model.go`, `types.go`, `update.go`, `remote.go`, and `remote_test.go` in addition to the scoped `view.go`/`view_test.go`.
- Verification: Focused UI/daemon tests passed, touched-package tests passed, and full `rtk make verify` passed after centralizing repeated status labels for lint.

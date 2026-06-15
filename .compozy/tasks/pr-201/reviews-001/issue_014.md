---
provider: coderabbit
pr: "201"
round: 1
round_created_at: 2026-06-15T18:05:13.931425Z
status: resolved
file: internal/core/run/ui/multi_remote.go
line: 499
severity: minor
author: coderabbitai[bot]
provider_ref: review:4499860579,nitpick_hash:3e6f88fe9aa3
review_hash: 3e6f88fe9aa3
source_review_id: "4499860579"
source_review_submitted_at: "2026-06-15T18:04:32Z"
---

# Issue 014: Restart the spinner loop after child bootstrap.
## Review Comment

`handleChildBootstrap` can install a running child snapshot, but this branch returns `nil`, so a queue that had no spinner scheduled yet stays frozen until another child event or tab switch.

<!-- cr-comment:v1:fbe31c003ed770bec5c87649 -->

---

## Triage

- Decision: `valid`
- Root cause: `multiRunModel.Update` handles a child bootstrap by mutating the child tab and returning `nil`, so a newly running child may not start the queue-owned spinner loop until another event occurs.
- Fix approach: return `ensureSpinnerTick()` after child bootstrap. Regression coverage belongs in `internal/core/run/ui/multi_remote_test.go`, which is the canonical multi-run model suite and must be touched minimally outside the listed code files to validate this UI fix.

## Resolution

- Resolved by restarting the spinner loop after child bootstrap.
- Verification: `rtk make verify` exited 0 after the code changes.

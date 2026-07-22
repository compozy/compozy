---
provider: manual
pr:
round: 5
round_created_at: 2026-07-22T21:45:58Z
status: pending
file: internal/daemon/run_manager.go
line: 512
severity: high
author: claude-code
provider_ref:
---

# Issue 001: Dependency readiness can change after task-run preflight

## Review Comment

`StartTaskRun` evaluates Task Group readiness once, then passes the resulting `outOfOrderNeeded` value into `startRun`. Before the run row is created, `startRun` performs another scoped sync that re-resolves the current plan, but `sameExecutionScope` compares only paths and the public reference. A concurrent plan edit can therefore add or reopen a prerequisite after line 512 without changing those paths; the final sync accepts the newer plan and the daemon starts the Task Group without either satisfying the new dependency or recording an out-of-order authorization.

Bind preflight evidence to the plan checksum/generation and re-evaluate readiness at the final run-creation boundary. If the plan changed, reject the start or require a fresh `allow_out_of_order` decision. Add a deterministic test that mutates dependency completion through the final-sync seam after preflight.

## Triage

- Decision: `UNREVIEWED`
- Notes:

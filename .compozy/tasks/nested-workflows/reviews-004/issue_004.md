---
provider: manual
pr:
round: 4
round_created_at: 2026-07-22T19:59:29Z
status: resolved
file: internal/daemon/task_multi_summary.go
line: 70
severity: high
author: codex
provider_ref:
---

# Issue 004: Multi-run summaries count partially failed children as complete

## Review Comment

`classifyChildOutcome` treats a child as completed when its journal contains any `job.completed` event, or recovered when any stall also appears. It never reads `job.failed` and does not consult the child's terminal run result. A child containing multiple jobs can complete one job and fail a later one; its parent still reports that child as completed. If any job also stalled, the same failed child is reported as recovered. The parent exit status is computed separately, so operators receive contradictory terminal status and summary counts.

Classify from the settled child run outcome plus its complete job lifecycle, and only count completion/recovery when the child itself completed successfully. Include `job.failed` in the evidence or pass the already-available child terminal results into summary construction. Add cases for completion followed by failure and stall plus completion plus failure.

## Triage

- Decision: `VALID`
- Notes: `classifyChildOutcome` currently treats any `job.completed` event as
  proof that the entire child run completed. Multi-job children can emit
  `job.completed` for an earlier job and then `job.failed` for a later job, while
  the durable child run correctly settles as `failed`; a prior stall also causes
  that same failed child to be counted as recovered. The fix classifies from
  both the settled durable run status and the complete terminal job-event set,
  adding `job.failed` to the queried evidence. Completion and recovery will
  require a `completed` child status with no failed job event. Regression cases
  cover completion followed by failure and stall plus completion plus
  failure.
- Verification note: the first full `make verify` attempt reached 5,285 tests
  but hit the unrelated pre-existing
  `TestShutdownEscalatesFromSIGTERMToSIGKILL` startup race while waiting for its
  shell trap marker. The isolated race-enabled test passed 10 consecutive runs;
  the next full run passed all 5,285 tests. Its E2E phase then exposed an
  environment limit: the review worktree produces a 277-character Unix socket
  path, and Darwin rejects the bind with `invalid argument`. Using the supported
  `COMPOZY_HOME` override with a short temporary path preserves the test behavior;
  the isolated E2E suite then passed all 7 tests. The full repository gate then
  passed with `COMPOZY_HOME` scoped only to Bun through `BUNCMD`: frontend lint,
  type-check, tests, and build; Go formatting and zero-issue lint; all 5,285 Go
  tests with 7 intentional skips; Go and extension builds; and all 7 E2E tests.

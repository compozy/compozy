---
provider: manual
pr:
round: 1
round_created_at: 2026-07-23T18:09:25Z
status: resolved
file: internal/cli/daemon_commands.go
line: 716
severity: low
author: claude-code
provider_ref:
---

# Issue 007: `--parallel-limit` help text contradicts the flooring behavior

## Review Comment

The `--parallel-limit` flag help states the value "must be greater than 0"
(`internal/cli/daemon_commands.go:716`), but `resolveTaskRunMultipleParallelLimit`
(`:1270-1282`) silently floors an explicit `0` or `-1` to `1` — the intended
behavior verified by `UT-052`. The help text therefore misleads the user: a
`--parallel-limit 0` does not error as the help implies; it runs at limit 1.

Impact is minor (a documentation/behavior mismatch, no incorrect execution), but
the two statements should agree so a user is not surprised.

Suggested fix: update the flag help to say that a non-positive value is floored
to 1 (e.g. "values ≤ 0 are treated as 1"), matching `resolveTaskRunMultipleParallelLimit`.

## Triage

- Decision: `VALID`
- Root cause: The `--parallel-limit` flag help (`internal/cli/daemon_commands.go:716`)
  states the value "must be greater than 0", but `resolveTaskRunMultipleParallelLimit`
  (`:1270-1282`) floors an explicit non-positive flag value to 1
  (`if limit < 1 { return 1, nil }`) — the intended behavior verified by UT-052
  (`internal/cli/daemon_commands_test.go:1396,1413`). The `limit <= 0` error branch
  only fires for a config-derived value, never for the flag. So `--parallel-limit 0`
  runs at limit 1 rather than erroring as the help implies. Confirmed by reading
  both functions.
- Fix approach: Update the flag help to describe the flooring behavior
  ("a value of 0 or less is treated as 1") so the documented contract matches the
  resolver.
- Out-of-scope file touched: `internal/cli/testdata/tasks_run_help.golden:49`
  mirrors this help string verbatim and is asserted by an exact-match golden test
  (`TestTasksRunHelpMatchesGolden`, `internal/cli/root_test.go:741`). The golden
  line is updated to the minimum needed to keep `make test` green; no other
  testdata is changed.
- Notes: No execution behavior changes — this is a documentation-only correction.
- Verification: shared with issue_002 — `make fmt`/`make lint` clean and the full
  Go `-race` suite green (incl. `TestTasksRunHelpMatchesGolden` and
  `TestREADMETasksRunSnippetsMatchCLIHelp`, which assert the updated help text and
  golden). The only `make verify` failure was an unrelated environmental
  `frontend-e2e` daemon-readiness flake (see issue_002 Verification note).

---
provider: manual
pr:
round: 4
round_created_at: 2026-07-22T19:59:29Z
status: resolved
file: internal/core/run/executor/hooks.go
line: 132
severity: high
author: codex
provider_ref:
---

# Issue 003: Extension hooks silently lose the stall policy

## Review Comment

`model.RuntimeConfig` and the extension SDK expose `StallEnabled`, `StallTimeout`, `ChildStallTimeout`, `TerminalCommandTimeout`, and `StallRetries`, but `hookRuntimeConfig` omits all of them when converting the executor's resolved `config.Stall`. `applyHookRuntimeConfig` also ignores every returned stall field. A `run.pre_start` extension therefore observes zero or nil values instead of the active policy, and any attempt to disable or tune stall handling is silently discarded. The job may retry or park under a policy different from the hook's returned contract.

Round-trip the resolved stall policy through both adapter directions, preserving explicit `false` and `0` pointer values, then validate the resulting nested timeout invariant before execution. Add adapter and mutable-hook tests covering every stall field and explicit disable/zero-retry values.

## Triage

- Decision: `VALID`
- Notes: `runshared.NewConfig` resolves the public runtime stall fields into
  `Config.Stall`, but `hookRuntimeConfig` does not map that policy back to
  `model.RuntimeConfig`, and `applyHookRuntimeConfig` does not map returned
  fields into `Config.Stall`. Consequently, `run.pre_start` observes zero/nil
  stall values and all returned stall changes are discarded. The fix will map
  all five fields in both directions, use non-nil pointers so explicit `false`
  and retry `0` survive the round trip, and resolve the returned stall-only
  config through the model's canonical defaults/invariant enforcement before
  execution. Adapter and mutable-hook regression tests will cover every field,
  explicit disable/zero retries, and correction of `ChildTimeout <= IdleTimeout`.
- Verification note: The first `make verify` attempt reached 5,284 tests but an
  unrelated daemon test setup saw its `git config` subprocess terminate with a
  segmentation fault. The exact failing daemon test passed both cases on an
  immediate isolated race-enabled rerun; no unrelated production or test code
  was changed.
- Verification environment: This review worktree's absolute path exceeds the
  macOS Unix-socket path limit when Playwright uses its default nested daemon
  home. The exact seven-test E2E target passes with the supported `COMPOZY_HOME`
  override set to a fresh short directory under `/tmp`; the final full gate
  injects that override only into Make's `frontend-e2e` recipe so Go tests retain
  their normal isolated home behavior.
- Final verification: The complete `make verify` pipeline passed with frontend
  lint reporting zero warnings/errors, Go lint reporting zero issues, 5,284 Go
  tests passing under the race detector, all builds succeeding, and all seven
  Playwright E2E tests passing.

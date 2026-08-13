# BUG-20260813-native-claim-skips-run-start: Native worker claim does not start its run

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-isolated-task-loop-execution, per-run execution
- **Scenarios:** TA-task-per-run-worktree-isolation; TA-task-fanout-worktree-isolation
- **Found:** 2026-08-13 · **Report:** docs/qa/reports/2026-08-13-worktree-support.md

## Summary

The scheduler's native `compozy__task_run_claim_next` path claimed queued worker runs but never
started them. Per-run materialization therefore never ran, and the worker executed in its bootstrap
session at the workspace root instead of a dedicated checkout.

## Reproduction

- **Charter:** CH-worktree-fanout-exit-removal · **Tour:** Multi-Tab Tour
- **Environment:** macOS arm64, isolated runtime with two real Codex workers, en-US

1. Create a workspace task with a `per_run` worktree policy.
2. Fan it out to two designations owned by the live worker pool.
3. Inspect both runs, their sessions, and the worktree catalog after the workers claim them.

**Expected:** Every claim progresses through `starting` to `running`; each run binds to a distinct
session and worktree, and the bootstrap claim session performs no task work.
**Actual:** Both runs remained `claimed`, reported root-workspace sessions, and had no worktree row.

## Fix

- **Root cause:** Native claim returned the lease directly while the separate start/session-handoff
  transition existed only in the CLI path. The scheduler uses the native surface, so the runtime
  contract was split across two entry points.
- **Fix commit:** `e59a03b6`
- **Regression suites:** `internal/daemon/native_tool_autonomy_calls_test.go` and
  `internal/daemon/task_manager_integration_test.go`, including the real daemon per-run journey.

## Verification

- **Retested:** 2026-08-13 in
  `compozy-worktree-support-20260813-083057-155448-lab`.
- **Result:** Passed. Two real workers claimed two new runs, both reached `running`, and each bound
  to a distinct `session_id`, `worktree_id`, run branch, and checkout. Both agents independently
  verified their Git top-level and completed cleanly; HTTP and Web reads agreed afterward.
- **Evidence:**
  `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/native-handoff-fixed-task.json`;
  `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-native-handoff-fixed.png`.

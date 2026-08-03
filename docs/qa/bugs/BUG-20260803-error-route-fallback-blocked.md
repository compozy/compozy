# BUG-20260803-error-route-fallback-blocked: An authored error route stayed blocked behind its failed source

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Lea
- **Journey Step:** J-recover-loop-node-failure, route one declared failure to fallback
- **Scenarios:** LP-error-route-fallback; LP-on-error-notification-with-context
- **Found:** 2026-08-03 · **Report:** docs/qa/reports/2026-08-03-loop-node-lifecycle.md

## Summary

A node with `on_error.route: fallback` recorded the routed failure, but the authored fallback task
never ran. Task materialization also created a normal success dependency from the failed source to
the same fallback, so the task engine kept the recovery node blocked.

## Reproduction

- **Charter:** CH-author-loop-failure-contract · **Tour:** Feature Tour
- **Environment:** isolated macOS lab / desktop / wifi-fast / en-US

1. Publish a Loop whose primary node declares a payload failure and routes `on_error` to
   `fallback`.
2. Keep a separate success-only node downstream of the same primary node.
3. Run the Loop and inspect the durable node/task state.

**Expected:** The fallback runs once; the success-only branch is skipped.

**Actual before the fix:** The primary output said `error_routed:fallback`, but the fallback task
remained blocked by the failed primary task.

## Evidence

- Before fix: run `looprun-eea5294754535e3e`.
- Repaired run: `looprun-ce061b65a4695127`.
- Browser: `/Users/pedronauck/dev/qa-labs/compozy-loop-node-lifecycle-20260803-191237-281307-lab/qa-artifacts/qa/evidence/task13/13-route-fallback-done.png`.

## Fix

- **Root cause:** Task dependency materialization did not distinguish an authored error-route edge
  from a normal success edge.
- **Fix:** Skip the success dependency only when the source node's declared error route targets that
  node; retain every ordinary success-path dependency.
- **Fix commit:** pending Task 13 checkpoint
- **Regression test:** the existing route case in
  `TestCoordinatorRunnerShouldApplyNodeFailurePrecedence` now proves only the success branch owns a
  task dependency on the primary node.

## Verification

- **Retested:** 2026-08-03 through the public run and Web detail surfaces.
- **Result:** Pass. The fallback returned `{"status":"recovered"}` once, the primary recorded
  `payload_declared` plus `routed`, the success-only branch was `branch_skipped`, and the run ended
  `done`.

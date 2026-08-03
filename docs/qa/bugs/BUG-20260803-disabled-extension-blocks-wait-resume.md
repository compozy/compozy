# BUG-20260803-disabled-extension-blocks-wait-resume: Disabling a later extension blocked an open wait from resuming

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-04 resume a waiting Loop lane
- **Scenarios:** LP-live-run-survives-extension-disable
- **Found:** 2026-08-03 · **Report:** docs/qa/reports/2026-08-03-loop-node-lifecycle-task08.md

## Summary

An operator could not resume an already-running Loop wait after disabling an extension used only by the next node. The resume request returned an internal server error and left the lane waiting, even though the run had already persisted the definition it was meant to execute.

## Reproduction

- **Charter:** CH-operator-lifecycle-ui · **Tour:** Feature Tour
- **Environment:** isolated macOS lab / desktop / wifi-fast / en-US

1. Publish a Loop whose first node is an operator wait and whose next node uses an extension tool.
2. Start the Loop and wait until the first node is waiting.
3. Disable the extension through the public CLI.
4. Resume the waiting node through the public HTTP route.

**Expected:** Resume accepts the wait payload from the run's pinned definition; the later unavailable action reports its own lifecycle failure.

**Actual before the fix:** Resume returned HTTP 500 and the wait remained open.

## Evidence

- Repaired public response: `/Users/pedronauck/dev/qa-labs/compozy-loop-operator-lifecycle-ui-20260803-044343-123901-lab/qa-artifacts/qa/screenshots/task08/29-disabled-extension-resume.json`.
- The same run's fresh status moved beyond `release`, proving the 200 response was committed rather than optimistic.

## Fix

- **Root cause:** `ResumeNodeWait` recompiled mutable current configuration through `GetConfigSnapshot`. Disabling the extension changed that catalog, so an in-flight run could no longer cross an unrelated wait boundary even though its executed definition digest was durable.
- **Fix commit:** Task 08 checkpoint
- **Regression test:** `internal/loop/service_test.go` keeps the invariant in `TestServiceControlMethodsShouldPreserveStatusContracts`: wait admission uses the lifecycle values from the pinned executed-definition snapshot, not the current catalog.

## Verification

- **Retested:** 2026-08-03, same persona/journey · **Report:** docs/qa/reports/2026-08-03-loop-node-lifecycle-task08.md
- **Result:** Pass. With `dev-cycle` still disabled, the rebuilt daemon returned HTTP 200 and consumed the open wait.

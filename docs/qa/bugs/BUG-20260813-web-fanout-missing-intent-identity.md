# BUG-20260813-web-fanout-missing-intent-identity: Web fan-out omits required intent identity

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-isolated-task-loop-execution, fan-out
- **Scenarios:** TA-worktree-web-fanout-isolation
- **Found:** 2026-08-13 · **Report:** docs/qa/reports/2026-08-13-worktree-support.md

## Summary

Submitting the Web fan-out dialog sent no idempotency identity. The daemon correctly refused the
request, so no designated run could be created from the UI.

## Reproduction

- **Charter:** CH-worktree-fanout-exit-removal · **Tour:** Multi-Tab Tour
- **Environment:** macOS arm64, isolated runtime, en-US

1. Open a ready task and choose **Fan out runs…**.
2. Enter two assignment lines and enable per-run worktrees.
3. Submit the dialog.

**Expected:** The client supplies one stable intent identity, the daemon returns two accepted runs,
and retrying the same unchanged submission reuses that identity.
**Actual:** The daemon returned `designations[0].idempotency_key is required` and no run was created.

## Fix

- **Root cause:** The dialog assembled designation briefs but did not own a request-level intent
  identity. It therefore could neither satisfy the contract nor preserve retry identity.
- **Fix commit:** `207bc4a7`
- **Regression suite:** `web/src/systems/tasks/components/__tests__/task-fanout-dialog.test.tsx`
  and `web/src/systems/tasks/stores/__tests__/task-fanout-store.test.ts`

## Verification

- **Retested:** 2026-08-13 in
  `compozy-worktree-support-20260813-083057-155448-lab`.
- **Result:** Passed. The browser request returned HTTP `201`, displayed two queued run outcomes,
  and the independent HTTP read showed two distinct keys sharing one request identity.
- **Evidence:**
  `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-fanout-accepted.png`;
  `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-fanout-accepted-fixed.png`;
  `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/web-fanout-fixed-task.json`.

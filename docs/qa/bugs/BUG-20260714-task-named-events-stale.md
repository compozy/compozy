# BUG-20260714-task-named-events-stale: Persisted Task events leave the open detail stale

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-complete-task-tree, observe externally controlled Task state
- **Scenarios:** TA-016
- **Found:** 2026-07-14 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** Consolidated implementation peer review

## Summary

The Task detail EventSource listened to only part of the durable Task stream vocabulary. Transactional events such as pause/resume, block lifecycle, auto-enqueue, hallucination guards, and wake delivery were persisted and published under named SSE events, but the Web hook had no matching listeners. An open Task could therefore remain stale until a reload even though the daemon and stream were correct.

## Reproduction

1. Open a Task detail page and keep it mounted.
2. Pause the Task through the UDS-backed CLI so the Browser receives only the live stream update.
3. Observe whether the detail renders the paused state and reason without reload.
4. Resume the Task through the same agent-operable surface and observe the inverse transition.

**Expected:** Every persisted named Task event wakes the owning detail cache; pause and resume appear immediately without reload.
**Actual:** Event families missing from `TASK_STREAM_EVENT_TYPES` never reached the Web handler, so their durable state could remain stale.

## Fix

- **Root cause:** `EventSource` routes named frames only to matching `addEventListener` registrations. The Web hook's static vocabulary omitted persisted transactional event families.
- **Correction:** The listener inventory now includes pause/resume, block creation/clear/expiry, auto-enqueue, hallucination blocked/suspected, and wake delivered/suppressed events in addition to the existing Task lifecycle events.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** `web/src/systems/tasks/hooks/__tests__/use-task-stream.test.tsx` asserts the complete persisted listener inventory and proves representative named events invalidate the Task detail cache.

## Verification

- The focused root-Turbo Web test passed 16/16.
- Browser Task `task-5a7465009a4f277a` remained open while UDS-backed `agh task pause` emitted sequence 385. The page rendered `Paused`, the exact reason, and `Resume` in 380 ms without reload.
- UDS-backed `agh task resume` emitted sequence 386. The same page removed the paused reason and restored `Pause` in 322 ms without reload.
- The Task's pending run was canceled through the UI and the named Delete modal then removed the Task with `Task deleted.`

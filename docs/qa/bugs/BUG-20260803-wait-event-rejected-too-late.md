# BUG-20260803-wait-event-rejected-too-late: An unsupported wait event failed only after the Loop started

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-improve-loop-with-feedback, step 1
- **Scenarios:** LP-wait-event-catalog-validation
- **Found:** 2026-08-03 · **Report:** docs/qa/reports/2026-08-03-loop-node-lifecycle-task08.md

## Summary

A Loop builder could publish a wait for an event that the runtime can never consume. The mistake appeared only after starting the Loop, turning an authoring problem into a failed run instead of an exact validation diagnostic.

## Reproduction

- **Charter:** CH-operator-lifecycle-ui · **Tour:** Feature Tour
- **Environment:** isolated macOS lab / CLI / wifi-fast / en-US

1. Author a control wait whose event kind is `qa.acknowledged`.
2. Validate or publish the definition through the public CLI.
3. Start the accepted Loop.

**Expected:** Validation rejects the unsupported kind before publication or execution and points to the wait node.

**Actual before the fix:** Publication succeeded; execution later failed because the executed watch-event contract set had changed.

## Evidence

- Repaired CLI validation: `/Users/pedronauck/dev/qa-labs/compozy-loop-operator-lifecycle-ui-20260803-044343-123901-lab/qa-artifacts/qa/screenshots/task08/28-invalid-wait-event-validation.json`.
- The structured result names `acknowledgement`, `watch_events_kind_unknown`, and the unsupported event kind.

## Fix

- **Root cause:** The wait linter checked the event discriminator shape but did not compare the authored event kind with the hook catalog and supported watch-event set.
- **Fix commit:** Task 08 checkpoint
- **Regression test:** `internal/loop/linter_test.go` owns the invariant in `Should reject a wait event outside the hook catalog before execution`.

## Verification

- **Retested:** 2026-08-03, same builder flow · **Report:** docs/qa/reports/2026-08-03-loop-node-lifecycle-task08.md
- **Result:** Pass. `compozy loop validate` returned `valid: false` with the catalog-specific node diagnostic; no run was required.

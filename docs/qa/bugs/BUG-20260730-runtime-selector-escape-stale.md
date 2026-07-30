# BUG-20260730-runtime-selector-escape-stale: Closed runtime selector stayed keyboard-visible

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Sol
- **Journey Step:** J-17 choose runtime, step 2
- **Scenarios:** RT-068
- **Found:** 2026-07-30 · **Report:** docs/qa/reports/2026-07-28-untested-full.md

## Summary

Sol could close the runtime selector with Escape while its exit animation left the popup accessible long enough to intercept the next keyboard action.

## Reproduction

- **Charter:** CH-runtime-selector-keyboard · **Tour:** Keyboard Tour
- **Environment:** isolated current-source Web / browser automation / desktop / en-US

1. Open the onboarding runtime selector.
2. Press Escape and immediately inspect accessible popup state.
3. Press Enter from the restored trigger focus.

**Expected:** No dialog/listbox remains accessible during exit; focus is on the trigger and Enter reopens it.
**Actual:** The exiting popup remained interactive until animation unmount.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-web-settled-popover-20260730-071400-207447-lab/qa-artifacts/qa/evidence/web-settled-popover/runtime-selector-open.png`
- `/Users/pedronauck/dev/qa-labs/compozy-web-settled-popover-20260730-071400-207447-lab/qa-artifacts/qa/evidence/web-settled-popover/onboarding.png`

## Fix

- **Root cause:** Exit-preserved popup DOM was neither inert nor hidden from accessibility APIs.
- **Fix commit:** 9904270
- **Regression test:** `packages/ui/src/components/__tests__/popover.test.tsx`

## Verification

- **Retested:** 2026-07-30, same persona/journey · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Result:** Escape immediately removed the dialog/listbox from the accessibility tree, restored trigger focus, and Enter reopened it.

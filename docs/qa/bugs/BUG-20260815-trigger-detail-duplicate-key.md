# BUG-20260815-trigger-detail-duplicate-key: Trigger detail reports duplicate UI identity

- **Status:** verified
- **Impact (user-side):** Cosmetic
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Bruno
- **Journey Step:** J-24 Triage work and manage automation at scale, step 5
- **Scenarios:** ET-web-trigger-detail-rule-page
- **Found:** 2026-08-15 · **Report:** docs/qa/reports/2026-08-15-triggers-ui.md

## Summary

Bruno could read and operate the trigger detail, but every detail render reported that two UI children shared one identity. The visible flow still completed, while React warned that future updates could duplicate or omit one of those surfaces.

## Reproduction

- **Charter:** CH-trigger-detail-rule-page · **Tour:** Feature Tour
- **Environment:** desktop / 1280x720 / wifi-fast / en-US

1. Select the `triggers-ui` workspace and open Triggers from the Dock.
2. Search for `Deploy`, open `Deploy webhook`, and toggle it off and on across reloads.
3. Read the browser console after the detail rerenders.

**Expected:** The detail rerenders without React reconciliation errors.
**Actual:** React repeatedly reports that `trg-684b2d3ed23e7196` is used as the key for two sibling children.

## Evidence

- `docs/qa/evidence/2026-08-15-triggers-ui/webhook-detail.png`
- Browser console captured during the session; a fresh detail reload reproduced the same warning.

## Fix

- **Root cause:** The delete confirmation and Inspect sheet were sibling elements keyed with the same raw trigger id. React therefore received two children with identical identities even though they owned different overlay state.
- **Fix commit:** the commit containing this verified report
- **Regression test:** `web/src/systems/automation/components/__tests__/trigger-detail-panel.test.tsx`

## Verification

- **Retested:** 2026-08-15 in a fresh browser session against the isolated lab on Web port 4177
- **Result:** Pass — the trigger detail and Inspect sheet rerendered without duplicate-key or other application console errors.

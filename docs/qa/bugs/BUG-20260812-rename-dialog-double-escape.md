# BUG-20260812-rename-dialog-double-escape: Keyboard users needed two Escapes to close session rename

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-rename-session, step 1
- **Scenarios:** RT-session-rename-durable
- **Found:** 2026-08-12 · **Report:** docs/qa/reports/2026-08-12-pr-351-review-round-1.md

## Summary

After opening Rename session from More actions, a keyboard user had to press Escape twice because the closing menu consumed the first key press behind the visible dialog.

## Reproduction

- **Charter:** CH-rename-session-parity · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US, isolated local daemon and Web dev server

1. Open a stopped user session.
2. Open More actions and choose Rename session.
3. Press Escape once after the dialog appears.

**Expected:** The visible rename dialog closes with one Escape.
**Actual:** The hidden overflow menu consumed the first Escape; a second Escape was required.

## Evidence

- `docs/qa/evidence/2026-08-12-pr-351-review-round-1/CH-rename-session-parity-single-escape.png`
- The fresh browser replay showed no Rename session dialog after one Escape.

## Fix

- **Root cause:** The dialog opened in the menu-item click before Base UI completed the overflow menu's close transition, leaving both layers active.
- **Fix commit:** review-round-1 commit (this commit)
- **Regression test:** `web/src/systems/session/hooks/__tests__/use-session-topbar-slot.test.tsx` — `Should close overflow before opening rename so one Escape dismisses the dialog`

## Verification

- **Retested:** 2026-08-12, Dora / J-rename-session · **Report:** docs/qa/reports/2026-08-12-pr-351-review-round-1.md
- **Result:** Rename opens after the overflow transition completes, and one Escape dismisses the dialog.

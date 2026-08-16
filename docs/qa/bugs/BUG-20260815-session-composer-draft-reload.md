# BUG-20260815-session-composer-draft-reload: Reload loses the session draft

- **Status:** fixed
- **Impact (user-side):** Data-Loss
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-17, refresh or return by deep link
- **Scenarios:** ET-web-session-composer-text-entry
- **Found:** 2026-08-15 · **Report:** docs/qa/reports/2026-08-15-session-attachments-pr-412-final.md

## Summary

The composer preserved exact Unicode and repeated spaces while mounted, but a full browser reload erased the unsent draft.

## Reproduction

- **Charter:** CH-session-composer-text-entry · **Tour:** Feature Tour
- **Environment:** macOS arm64, isolated daemon and local Web bundle, en-US

1. Type Revisão 😊 antes   do lançamento in a session composer.
2. Open and close the next-prompt runtime selector.
3. Reload the deep-linked session.

**Expected:** The exact draft remains.
**Actual:** The session store existed only in memory, so reload recreated it with an empty draft map.

## Fix

- **Root cause:** Session drafts had no browser persistence owner.
- **Fix:** Persist only non-empty session drafts in a versioned local-storage envelope, validate rehydrated values, and keep first-prompt handoffs and Goal feedback memory-only.
- **Regression suite:** web/src/systems/session/stores/__tests__/session-store.test.ts

## Verification

- **Retested:** 2026-08-15 in session-attachments-pr-412-final-20260815-195219-431614.
- **Result:** Passed. Unicode, repeated spaces, selector interaction, full reload, and deep-link return preserved the exact draft.
- **Evidence:** docs/qa/evidence/2026-08-15-session-attachments-pr-412-final/26-composer-draft-after-reload.png.

# BUG-20260815-session-window-runtime-selector-loop: Opening a session loops forever

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Théo
- **Journey Step:** J-session-attachments, open the target session
- **Scenarios:** ET-session-attachment-model-gate; ET-session-attachment-picker
- **Found:** 2026-08-15 · **Report:** docs/qa/reports/2026-08-15-session-attachments-pr-412-final.md

## Summary

Opening a session caused React to repeat external-store renders until the session surface failed with a maximum update depth error.

## Reproduction

- **Charter:** CH-session-attachments · **Tour:** Network Tour
- **Environment:** macOS arm64, isolated daemon and local Web bundle, en-US

1. Open an existing session from the attachment QA workspace.
2. Wait for the runtime selector and composer to mount.

**Expected:** The session renders once and remains interactive.
**Actual:** The runtime snapshot selector returned a new object on every read, so React treated every read as a store change and entered an infinite render loop.

## Fix

- **Root cause:** useSessionWindowController selected a derived runtime object without a comparator.
- **Fix:** Compare derived runtime snapshots shallowly before notifying React.
- **Regression suite:** web/src/systems/os/apps/session/__tests__/use-session-window-controller.test.tsx

## Verification

- **Retested:** 2026-08-15 in session-attachments-pr-412-final-20260815-195219-431614.
- **Result:** Passed. The session opened normally and remained stable through live capability updates, attachment sends, and reloads.
- **Evidence:** docs/qa/evidence/2026-08-15-session-attachments-pr-412-final/02-image-ready-no-false-warning.png.

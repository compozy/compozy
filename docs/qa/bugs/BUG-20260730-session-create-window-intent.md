# BUG-20260730-session-create-window-intent: Created session route lost before window hydration

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P0
- **Persona Affected:** Dora
- **Journey Step:** J-17, create the session and arrive at its composer
- **Scenarios:** ET-web-session-prompt-runtime-and-create-navigation; MS-web-session-simple-advanced-launch; RT-063; RT-new-session-fast-feedback
- **Found:** 2026-07-30 · **Report:** docs/qa/reports/2026-07-30-session-runtime-selector.md

## Summary

The first session created after onboarding changed the browser URL to the new session but showed no
session window. Reloading materialized the window; later creates after desktop hydration appeared
normally.

## Reproduction

- **Charter:** CH-session-launch-composer-handoff · **Tour:** Feature Tour
- **Environment:** isolated current-source Web / desktop / en-US

1. Complete onboarding in a fresh browser and isolated Compozy home.
2. Open Start session, choose `general`, and submit.
3. Observe the canonical session URL before the Window Manager finishes hydration.

**Expected:** The returned session route remains pending until its window is materialized.
**Actual:** The URL changed, but the desktop showed no session window until reload.

## Evidence

- Before fix: `docs/qa/evidence/2026-07-30-session-runtime-selector/03-after-create.png`
- Retest: `docs/qa/evidence/2026-07-30-session-runtime-selector/04-session-open-after-create.png`

## Fix

- **Root cause:** Route reconciliation discarded the session intent when the Window Manager was not
  yet live and again when an accepted open completed without applying. No authoritative transition
  retained enough information to retry it.
- **Fix:** Keep the intent pending until hydration is live and restore it after an unapplied completion;
  the next authoritative state reconciles it.
- **Fix commit:** Working tree; this QA remediation is not committed separately.
- **Regression test:** `web/src/systems/os/lib/__tests__/routing-coordinator.test.ts` owns session-route
  retention across hydration and unapplied lifecycle completion.

## Verification

- **Retested:** 2026-07-30 in the same isolated browser and daemon after hot reload.
- **Result:** Pass — closing all session windows and creating another session materialized its composer
  in 1.2 seconds without reload. Web lint, typecheck, and all 4,089 tests passed.

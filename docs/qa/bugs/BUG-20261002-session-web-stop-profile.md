# BUG-20261002-session-web-stop-profile: Web cannot stop a session in a named profile

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Théo
- **Journey Step:** J-13, stop the selected session while retaining its transcript
- **Scenarios:** RT-013
- **Found:** 2026-10-02 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Théo can create a Studio session, complete useful work, and read its transcript in two browser
tabs, but Stop session reports that the same session does not exist. Reloading does not help.

## Reproduction

- **Charter:** CH-browser-multitab-stream-continuity · **Tour:** Multi-Tab Tour
- **Environment:** macOS, isolated Chrome profile, three production-served documents, local daemon

1. Select profile `studio` in workspace Studio Operations and send a first prompt to the real
   Codex agent. The session `sess-80997493cb1ab1db` writes `founder-launch-email.md`.
2. Open the same session in another tab, leaving a third document on the desktop.
3. In the first tab, choose More actions → Stop session.
4. Reload the first document and repeat the action.

**Expected:** The selected session stops, every tab converges, and persisted history remains readable.
**Actual:** Both Web attempts return 404. The fresh document displays
`Session not found: sess-80997493cb1ab1db`; independent reads still show the session active.

## Evidence

- `docs/qa/evidence/2026-10-02-untested/multitab-stop-fresh-retry.json`
- `docs/qa/evidence/2026-10-02-untested/multitab-stop-retry-network.json`
- `docs/qa/evidence/2026-10-02-untested/multitab-stop-not-found.png`
- `docs/qa/evidence/2026-10-02-untested/multitab-failed-stop-inspect-studio.json`
- `docs/qa/evidence/2026-10-02-untested/multitab-stop-scoped-uds-diagnostic.json`:
  the same public stop route with `profile=studio` succeeds over UDS (legacy bodyless 204).

The earlier CLI-specific `BUG-20260826-session-cli-profile-scope` has a different owning
boundary and remains verified. The multi-tab transport bug is also separate: this request
reaches the daemon immediately, whereas the historical connection-starvation request stalled.

## Fix

- **Root cause:** The Web stop adapter omits the profile query. Its hook also reconstructs
  workspace ownership from the current view instead of carrying the selected session's owner.
- **Fix commit:** `514de24d6`.
- **Regression test:** Existing `session-api.test.ts` owns profile serialization at fetch I/O;
  existing `use-session-actions.test.tsx` owns target selection and sequential mixed-owner batches;
  existing page-controls tests preserve stop/retry lifecycle assertions as the mutation input changes.

## Verification

Retested in the production-served browser through the same three-document charter. Stop sends
`profile=studio`, returns 202, and reaches Stopped in 2.256 seconds. The peer document converges
before activation; a refreshed Studio view retains the transcript. HTTP and UDS return identical
history, every prior event ID and field survives, and the active catalog excludes the session.
Both an ordinary stop and a second stop after a model-selection recovery attempt pass. The latter
provider attempt is rejected by the external backend; it is not counted as completed agent work.

Eight focused assertions fail before the Web repair; all 150 tests in the existing three Web suites
pass afterward. The existing OpenAPI session-profile case also fails before the selector is declared
and passes after regeneration. Root build/typecheck, React Doctor 100/100, and all affected `make gate`
lanes pass, including 6,898 Web tests. Evidence: `web-stop-*`, `gate-web-stop-owner.log`,
`multitab-fixed-stop-result.json`, and `multitab-fixed-replay-summary.json` in the current evidence folder.

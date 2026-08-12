# BUG-20260811-session-timeline-lifecycle-warning: Virtual timeline measurement triggers a nested React lifecycle update

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Rafa; Théo
- **Journey Step:** J-14 read a long transcript; J-11 return to a live session
- **Scenarios:** RT-047; RT-023
- **Found:** 2026-08-11 · **Report:** `docs/qa/reports/2026-08-11-frontend-performance.md`

## Summary

Mounting a real session timeline produced React's `flushSync was called from inside a lifecycle method` browser-console error twice. The transcript remained visible, but the virtualizer was forcing a synchronous React update while React was already committing layout work.

## Reproduction

- **Charter:** CH-021 · **Tour:** Garbage Tour
- **Environment:** Chromium / desktop / Wi-Fi-fast / en-US; isolated local daemon; real Codex session `sess-d2a00039e61f9b04`.

1. Open a session with transcript entries through the Web OS shell.
2. Let the variable-height virtual timeline mount and measure its viewport and rows.
3. Observe the browser console during the initial layout commit.

**Expected:** Timeline measurement completes without forcing a nested React lifecycle update.
**Actual:** React reports `flushSync was called from inside a lifecycle method` twice during virtualizer measurement.

## Evidence

- `docs/qa/evidence/2026-08-11-frontend-performance/live-session-before-interrupt.png`
- `docs/qa/evidence/2026-08-11-frontend-performance/session-reconnect-clean.png`
- `docs/qa/evidence/2026-08-11-frontend-performance/session-reconnect-clean.har`
- Browser replay after the fix produced no page errors or lifecycle warnings; the session stream resumed at cursor `44` with epoch/generation fences intact.

## Fix

- **Root cause:** `@tanstack/react-virtual` 3.14.9 defaults `useFlushSync` to `true`. A synchronous virtualizer notification emitted from its layout update called `flushSync(rerender)` while React 19 was already committing the timeline. The session timeline now opts out of that adapter behavior and lets React schedule the update normally.
- **Fix commit:** `7d80c60`.
- **Regression test:** A jsdom candidate was rejected because it stayed green after removing the production fix; it cannot reproduce browser layout and `ResizeObserver` timing. `docs/qa/automation-backlog/session-timeline-react-lifecycle.md` records the canonical browser-console coverage instead of freezing a false unit invariant.

## Verification

- **Retested:** 2026-08-11, same session route and real browser · **Report:** `docs/qa/reports/2026-08-11-frontend-performance.md`
- **Result:** A fresh route reload and a hidden/offline/visible reconnect mounted and measured the timeline with no page errors and no `flushSync` warning. The transcript remained readable and the reconnect opened one session EventSource at cursor `44`.

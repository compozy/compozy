# BUG-20260729-session-window-cross-tab-focus: A session selected in another tab stays hidden behind an unrelated window

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Théo
- **Journey Step:** J-13 Follow a live run, step 4
- **Scenarios:** RT-013; RT-015
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md

## Summary

A returning operator opened the same live session in multiple tabs. One tab showed the requested
session, while another remained on Marketplace even after the operator selected the session from the
Sessions catalog. The requested session existed and received lifecycle updates behind the unrelated
window, but it was not brought to the foreground until a reload.

## Reproduction

- **Charter:** CH-016 · **Tour:** Multi-Tab Tour
- **Environment:** desktop / 1440×900 / wifi-fast / en-US

1. Create an idle, attachable session through the public API.
2. Open `/agents/qa-hook-agent/sessions/<sid>` in two browser tabs.
3. In the tab showing Marketplace, open Sessions and select the newly created session.
4. Observe that Marketplace remains the foreground window even though the session window is mounted.
5. Stop the session in the other tab, then reload the affected tab.

**Expected:** Opening or selecting the session brings its window to the foreground in every tab, and
the second tab can observe the stop without an unrelated window obscuring it.

**Actual:** The session window remained behind Marketplace after both direct entry and explicit
selection. Reloading the canonical URL finally brought the stopped session to the foreground.

## Evidence

- `qa/evidence/046-session-lifecycle-browser/rt013-before-stop-a.png`
- `qa/evidence/046-session-lifecycle-browser/rt013-before-stop-b.png`
- `qa/evidence/046-session-lifecycle-browser/rt013-stop-a-session-selected.png`
- `qa/evidence/046-session-lifecycle-browser/rt013-after-stop-c-reload.png`
- Independent API and UDS reads confirmed the session changed to `stopped` and retained its transcript.

## Fix

- **Root cause:** Explicit route reconciliation issued focus against stale Window Manager topology and
  did not refresh/retry the rejected mutation. Separately, retained Home/Session windows kept their
  live transports when unfocused, minimized, or outside the active desktop because mount state was
  incorrectly acting as live-data ownership. The staged correction reconciles stale topology and
  grants per-window live ownership only to the focused, non-minimized window on the active desktop.
  Literal-tab replay then exposed the remaining cross-document transport gap tracked as the regression
  of `BUG-20260713-first-prompt-optimistic-stuck`; therefore this fix is not yet terminal.
- **Fix commit:**
- **Regression test:** `web/src/systems/os/lib/__tests__/routing-coordinator.test.ts`;
  `web/src/systems/os/hooks/__tests__/window-manager-runtime.test.ts`;
  `web/src/systems/os/apps/dashboard/__tests__/dashboard-window.test.tsx`;
  `web/src/systems/os/apps/session/__tests__/session-window.test.tsx`;
  `web/src/systems/dashboard/__tests__/use-home-dashboard.test.tsx`;
  `web/src/systems/session/components/__tests__/session-chat-runtime-provider.test.tsx`

## Verification

- **Retested:** 2026-07-29, isolated tabs and then two literal tabs in one browser
- **Result:** partial/fail — both literal tabs brought the requested session to the foreground, but the
  lifecycle stop remained queued until the background tab closed. RT-013 remains failed pending the
  structural document-visibility ownership TechSpec and a fresh original-persona replay.

## Re-found (2026-07-29) — RT-015

Théo opened a fresh attachable session through the canonical Web route in an isolated browser. The
URL remained correct, but Home stayed in the foreground with `Live layout disconnected` and no resume
control. One clean browser retry reproduced the same result. HTTP, UDS, and CLI attach operations all
passed against independent fixtures, isolating the failure to the document/window transport boundary.

- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/047-session-delete-attach`
- **Report:** `docs/qa/reports/2026-07-28-untested-full.md`

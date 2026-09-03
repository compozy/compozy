# BUG-20260902-background-window-stream-starvation: Background windows block desktop commands

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Théo
- **Journey Step:** J-organize-tabbed-work, dock activation and reload recovery
- **Scenarios:** ET-web-window-routing-lifecycle
- **Found:** 2026-09-02 · **Report:** docs/qa/reports/2026-09-01-integrated-terminal-review-r2.md

## Summary

Three retained Session windows kept their transcript streams open while covered by another window.
Together with the document's global streams, they exhausted Chrome's HTTP/1 connection pool. The
next window-manager command remained locally pending and never reached the daemon, so Dock and
traffic-light actions appeared inert.

## Reproduction

- **Charter:** CH-terminal-window-tabs-canary · **Tour:** Interrupt Tour
- **Environment:** desktop / wifi-fast / en-US / isolated local lab

1. Restore a desktop containing three Session windows, several Terminal windows, and Home.
2. Focus Home, then activate Terminal from the Dock or minimize Home.
3. Compare the browser's command state, the window-manager client view, and the daemon request log.

**Expected:** The action reaches the daemon, settles, and updates focus or minimized state.
**Actual:** `window.open` or `window.focus` remains pending, no matching request reaches the daemon,
and every later window command queues behind it.

## Evidence

- Browser client `web-d87b85d7-128d-473f-bafd-a808c8c88528` remained on Home with local
  `window.open` pending at layout revision 18 while the daemon received no later command request.
- Fresh client `web-6c459ca6-6fcc-4d0f-8f5d-6a90704e82cc` reproduced the same zero-request stall on
  its first `window.focus` command.
- The browser console showed one live transcript stream per covered Session window before the fix.

## Fix

- **Root cause:** `useWindowLiveDataEnabled` treated every non-minimized window on the active desktop
  as live, even when another floating window owned focus. Retained background Session windows
  therefore opened independent long-lived transcript connections.
- **Resolution:** The selector now grants live-data ownership to a bounded pair: the focused window
  and the most recently focused eligible background window. Additional retained windows stay mounted
  without holding long-lived connections and re-enter the pair when focus recency changes.
- **Fix commit:** pending remediation batch
- **Regression test:** `web/src/systems/os/hooks/__tests__/os-interaction-hooks.test.tsx`

## Verification

- **Retested:** 2026-09-02 in the same isolated lab with fresh browser client
  `web-58fe86d9-7eaa-4882-b2ca-d03314145fe9`.
- **Result:** Passed. The bounded pair preserved one visible background live-data owner while Terminal
  Dock activation settled, Home minimized and restored with the correct successor, URL and focused
  instance stayed aligned, and reload preserved the ten-window topology.
- **Evidence:**
  `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-review-r2-20260902-020216-937662-lab/qa-artifacts/qa/screenshots/theo-routing4-home-restored.png`

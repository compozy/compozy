# BUG-20260813-retry-leaves-blank-route: Retry leaves Home blank after the daemon returns

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Cora
- **Journey Step:** J-operate-home-dashboard, step 1
- **Scenarios:** RT-home-dashboard-zones
- **Found:** 2026-08-13 · **Report:** docs/qa/reports/2026-08-13-pr-368-coderabbit.md
- **Origin:**

## Summary

After Home explains that workspace data is unavailable and offers Retry, clicking Retry after the daemon returns removes the error page but renders nothing. A fresh browser session can load the same Home route, so the visible recovery action is what fails.

## Reproduction

- **Charter:** CH-home-preload-recovery · **Tour:** Network Tour
- **Environment:** laptop / desktop viewport / wifi-fast / en-US; isolated daemon and Web dev server

1. Open Home with a live daemon and confirm its truthful workspace-scoped zones.
2. Stop the daemon through the public daemon command and reload Home.
3. Confirm the root route error page reports the workspace failure and offers Retry.
4. Restart the same daemon and click Retry.

**Expected:** Home reloads its route data and renders the connected dashboard.
**Actual:** The error page disappears and the browser renders an empty page indefinitely.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-pr-368-coderabbit-20260813-051821-831054-lab/qa-artifacts/qa/screenshots/home-daemon-unavailable.png`
- A second browser session opened the same URL after restart and rendered the shell, proving the daemon and Web server were reachable.

## Fix

- **Root cause:** The root Retry forced every route match into a pending state after clearing the error, but the root route has no pending fallback. The app-shell preload also let a failed workspace catalog reject navigation instead of retaining the query error for the mounted consumer.
- **Fix commit:** `a97e07f`
- **Regression test:** `web/src/routes/__tests__/-__root.test.tsx`; `web/src/routes/_app/__tests__/-route-preloading.integration.test.tsx`

## Verification

- **Retested:** 2026-08-13 in the same isolated daemon/Web lab.
- **Result:** Pass. With the daemon unavailable the onboarding boundary remained visible; after restart, Retry rendered the project-scoped Home dashboard. Evidence: `/Users/pedronauck/dev/qa-labs/compozy-pr-368-coderabbit-20260813-051821-831054-lab/qa-artifacts/qa/screenshots/home-retry-recovered.png`.

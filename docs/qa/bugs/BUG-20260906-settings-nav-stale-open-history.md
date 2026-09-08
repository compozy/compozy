# BUG-20260906-settings-nav-stale-open-history: A Settings section click is lost while the launcher's open is still pending

- **Status:** fixed locally — unchanged real daemon E2E-014 journey passes 5/5 locally (served `web/dist`; the fixture ignores a dist override) and CI on d897e90f5 is green for Profiles E2E-014 and all 47 shard-3 tests
- **Impact:** Navigation
- **Severity:** Major · **Priority:** P1
- **Scenarios:** ET-profile-web-settings-lifecycle-dialogs
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

Web E2E shard 3 on head 500a93afe failed `profiles.spec.ts` E2E-014 at the archived disclosure: after archiving the active profile and reloading at `/settings/profiles`, the Settings launcher click and the Profiles section click left the window at General for the full 20s wait. The retained Playwright trace shows the order: the launcher's `window.navigate` to General was projected optimistically at 55.99s but the daemon answered at 56.47s (321ms on the runner); the Profiles click landed in that gap while the browser URL still read `/settings/profiles`. TanStack Router treats a navigation to the current location as a no-op, so the route sync-controller never reported it, and at 56.47s the launcher's deferred history write pushed `/settings/general` and reconciled the window back to General. No request for Profiles ever reached the daemon.

Two production repairs in the routing coordinator, the sole URL↔window-manager bridge: a user intent token lets a newer navigation (an explicit user navigation or a location the coordinator did not write) supersede a pending open's deferred history write, and `userNavigate` queues its own reconciliation so a same-location gesture still reaches the window manager. The Settings section links now hand plain clicks to the shell through that path while modified clicks keep native link behavior. Rejected commands still write no history, and the `href`, `aria-current`, and design of the links are unchanged.

The failure needs the runner's slow command round trip: three pre-fix and five post-fix local runs of the unchanged journey pass within seconds each, so the CI trace is the reproduction record. The coordinator's canonical suite gained the three race cases, the nav link gained a component test, and the full web Vitest lane, typecheck, and lint pass.

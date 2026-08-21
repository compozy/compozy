# QA Run Report — 2026-08-20 — window-management-regressions

- **Scope:** Window opening, focus, tab activation, background route rendering, live shortcut rows, and multi-client conflict recovery.
- **Cadence tier:** targeted
- **Build:** `01313427` + working tree
- **Environment:** isolated Compozy home and daemon on `127.0.0.1:49723`; isolated Web server
- **Started:** 2026-08-21T02:08:52-03:00 · **Status:** pass

## Personas

- Bruno operates the desktop, Settings, Knowledge, and the keyboard shortcut reference.
- Théo supplies a competing browser client to advance topology revisions during semantic app opening.

## Result matrix

| Charter | Journey / Scenario | Tour | Status |
|---|---|---|---|
| CH-untested-068-operate-desktop-shell-bruno | J-operate-desktop-shell / ET-web-window-routing-lifecycle | Feature Tour | Pass |
| CH-untested-068-operate-desktop-shell-bruno | J-operate-desktop-shell / ET-web-desktop-shell-lifecycle | Feature Tour | Pass |
| CH-untested-068-operate-desktop-shell-bruno | J-marketplace-acquisition / ET-web-catalog-navigation | Adjacent canary | Pass |
| CH-window-tabs-supervisor-recovery | J-administer-window-manager / ET-window-manager-multi-client | Interrupt Tour | Pass |
| CH-herdr-keyboard-navigation | J-operate-desktop-shell / ET-live-shortcut-cheatsheet | Feature Tour | Pass |

## Evidence

- Settings stayed mounted on Layouts while focus and the browser URL moved through Loops and Knowledge. Knowledge rendered normally and the browser reported no page error: `/Users/pedronauck/dev/qa-labs/compozy-window-management-regressions-20260821-020852-370190-lab/qa-artifacts/evidence/settings-knowledge-background-route.png`.
- Merge all produced one six-member stack. Selecting Knowledge changed the active tab and URL with a `200` command response: `/Users/pedronauck/dev/qa-labs/compozy-window-management-regressions-20260821-020852-370190-lab/qa-artifacts/evidence/grouped-tabs-knowledge-active.png`.
- Two browsers opened Jobs concurrently. Théo observed `409 → 200`, then opened Triggers with another `200`; neither browser reported a page error: `/Users/pedronauck/dev/qa-labs/compozy-window-management-regressions-20260821-020852-370190-lab/qa-artifacts/evidence/two-client-conflict-recovery.txt`.
- The live keyboard reference contained exactly one `Switch to desktop` range row: `/Users/pedronauck/dev/qa-labs/compozy-window-management-regressions-20260821-020852-370190-lab/qa-artifacts/evidence/keyboard-shortcuts-unique.png`.
- The dock and palette exposed the canonical Bridges and Sandbox labels; Sandbox opened as a real window.
- Focused Turbo evidence: 49 tests passed across the four canonical suites; Web lint finished with zero warnings/errors; Web typecheck completed successfully.
- Cleanup evidence: `/Users/pedronauck/dev/qa-labs/compozy-window-management-regressions-20260821-020852-370190-lab/qa-artifacts/qa/teardown.json` records `"clean": true` with no survivors.

## Runtime errors observed

One expected `409 Conflict` was induced by the two-client disruption probe. The stale client refreshed and its guarded focus retry returned `200`; its next independent command also returned `200`. No Compozy-owned browser error, duplicate-key warning, route invariant, maximum-update-depth error, or frozen command queue occurred in the verdict window.

## Compozy Impact Audit

- Native tools: no impact; checked the window-manager command descriptor path and no tool ID, schema, digest, risk flag, or capability gate changed.
- Extensibility and hooks: no impact; checked command registry consumption, extension shortcut rows, hooks, bridge SDKs, and window-manager config lifecycle. The fixes only stabilize existing Web projections and supported rebase metadata.
- Workspace data isolation: workspace-scoped topology and client-scoped focus remain unchanged; checked workspace/client bindings through the Web runtime command path and refresh/retry flow.
- Official Compozy skill: no impact; checked `skills/compozy/` and no public command, tool ID, hook event, capability, resource, or task semantic changed.

## Final status

**Verdict: PASS.** The isolated persona walk covered background route rendering, Knowledge stability, live shortcut uniqueness, semantic app labels, multi-client conflict recovery, window grouping, tab activation, reload persistence, and teardown.

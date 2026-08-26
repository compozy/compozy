# BUG-20260826-onboarding-pulse-hides-empty-home: First-run setup activity hides the empty Home state

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Lea
- **Journey Step:** J-operate-home-dashboard, first Home open
- **Scenarios:** RT-home-zero-inventory-first-run
- **Found:** 2026-08-26 · **Report:** `docs/qa/reports/2026-08-26-pr-484-global-desktop.md`

## Summary

Lea finished setup without a project or any agent work, but Home opened with seven zero-filled panels instead of the promised first-run state with three clear ways to start.

## Reproduction

- **Charter:** CH-home-zero-inventory-first-start · **Tour:** Feature Tour
- **Environment:** laptop / 1280×800 / wifi-fast / en-US

1. Start CompozyOS with a fresh isolated home and no workspaces.
2. Select a runtime and finish setup through Skip without adding or removing a project.
3. Open Home from the Dock, then refresh.

**Expected:** Home shows “No agent work yet” and the three real start actions.
**Actual:** Home shows all seven populated-dashboard zones with zero values before and after refresh.

## Evidence

- `docs/qa/evidence/2026-08-26-pr-484-global-desktop/CH-home-zero-inventory-first-start-no-empty-state.png`
- Independent CLI and API reads showed an empty workspace catalog and no runs, tasks, attention, or live work; `GET /api/observe/overview` nevertheless reported two pulse events immediately after onboarding.

## Fix

- **Root cause:** Overview pulse aggregation counted every global event row, including onboarding configuration events whose `session_id` is empty, as agent activity.
- **Fix commit:** pending
- **Regression test:** `internal/store/globaldb/global_db_observe_overview_test.go` — `TestObserveOverviewEventAndSessionAggregates` proves empty-session configuration events never enter a workspace or all-workspaces pulse.

## Verification

- **Retested:** 2026-08-26, Lea, fresh isolated home through the original onboarding and Home journey.
- **Result:** Pass in the current working tree. Home showed “No agent work yet” with exactly three actions before and after refresh; public HTTP and CLI overview reads both reported zero session events and no busiest bucket. Commit metadata remains pending.
- **Evidence:** `docs/qa/evidence/2026-08-26-pr-484-global-desktop/CH-home-zero-inventory-first-start-fixed.png`

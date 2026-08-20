# BUG-20260820-global-home-deleted-onboarding: Removing the suggested Home folder disables the desktop

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Lea
- **Journey Step:** J-19 onboarding default model, step 2
- **Scenarios:** RT-onboarding-setup-panel-over-shell; RT-home-zero-inventory-first-run
- **Found:** 2026-08-20 · **Report:** `docs/qa/reports/2026-08-20-ui-normies-retry.md`

## Summary

Lea could remove the preselected Home folder while choosing to start in Global. The setup then completed into an empty desktop where Dock apps received focus but did not open, so she could not reach Home or start work.

## Reproduction

- **Charter:** CH-untested-019-19-lea · **Tour:** Feature Tour
- **Environment:** laptop / 1280×800 and 768×800 / wifi-fast / en-US

1. Start CompozyOS with a fresh isolated home and open the Web surface.
2. Choose the default Codex model and continue to workspace setup.
3. Remove the preselected Home folder, then finish setup in Global.
4. Select Home from the Dock.

**Expected:** The operator-home registration remains internal to Global, and Home opens with its honest zero-inventory state.
**Actual:** Onboarding deleted the operator-home registration. The desktop remained blank and Home did nothing after click, reload, and a second click.

## Evidence

- `docs/qa/evidence/2026-08-20-ui-normies-retry/home-768-after-dock-click.png`
- Runtime log: the daemon registered `ws_cabcd37bde1f4fcb` for `/Users/pedronauck`, then Web issued `DELETE /api/workspaces/:workspace_id` with status 204.
- Independent API read after reload: `GET /api/workspaces` returned an empty catalog and the deleted workspace's window-manager read returned `window_manager_workspace_not_found`.

## Fix

- **Root cause:** `useOnboardingWorkspaces` seeded every daemon registration into the selectable project draft and resolved removal against the unpartitioned catalog. It did not apply the existing operator-home partition used by the rest of the workspace system.
- **Fix commit:** `e520f3fe`.
- **Regression test:** `web/src/systems/onboarding/hooks/__tests__/use-onboarding-workspaces.test.tsx` — three new operator-home cases failed before the production fix and the canonical suite now passes 11/11.

## Verification

- **Retested:** not run; skipped by explicit user instruction before the replacement Web surface started.
- **Result:** scoped suite/lint/typecheck evidence passed, but behavioral verification remains unclaimed.

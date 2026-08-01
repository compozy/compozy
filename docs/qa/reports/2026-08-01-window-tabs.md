# QA Run Report — 2026-08-01 — window-tabs

- **Scope:** Window-tab lifecycle across the desktop UI, CLI, HTTP, UDS, native tools, hooks, configuration, layout persistence, and adjacent Home/pager canaries
- **Cadence tier:** full
- **Build:** `d196f3a7` · **Environment:** isolated local daemon and production Web build; main lab `compozy-consumer-saas-growth-20260801-085219-264358`, focused config retest lab `compozy-window-tabs-live-apply-status-retest-20260801-115716-306628`
- **Completed:** 2026-08-01T12:00:04Z · **Status:** complete

## Personas

| Persona | Base | Sessions |
|---|---|---|
| Bruno | Keyboard-heavy desktop operator | CH-window-tabs-keyboard-flow |
| Théo | Multi-agent supervisor recovering interrupted work | CH-window-tabs-supervisor-recovery |
| Ada | Autonomous structured-surface operator | CH-window-tabs-agent-parity |
| Cora | Home observability operator | CH-window-tabs-home-canary |

## Session Matrix & Results

| # | Scenario | Persona | Status | Issue |
|---|---|---|---|---|
| 1 | ET-window-tab-deck-lifecycle | Bruno | Pass | |
| 2 | ET-window-tab-multi-instance | Bruno | Pass | |
| 3 | ET-window-tab-palette-search | Bruno | Pass | |
| 4 | ET-window-tab-navigation-stack | Bruno | Pass | |
| 5 | ET-window-tab-strip-relocation | Bruno | Pass | |
| 6 | ET-web-window-routing-lifecycle | Bruno | Pass | |
| 7 | ET-web-dock-default-window-size | Bruno | Pass | |
| 8 | ET-window-manager-layout-gestures | Bruno | Pass | |
| 9 | ET-window-manager-drop-swap | Bruno | Pass | |
| 10 | ET-window-tab-close-reopen | Théo | Pass | |
| 11 | ET-window-tab-pinning | Théo | Pass | |
| 12 | ET-window-manager-multi-client | Théo | Pass | |
| 13 | ET-web-desktop-shell-lifecycle | Théo | Pass | |
| 14 | RT-desktop-pager-overview | Théo | Pass | |
| 15 | ET-window-tab-agent-parity | Ada | Pass | |
| 16 | ET-window-tab-v3-discard | Ada | Pass | |
| 17 | ET-window-manager-public-parity | Ada | Pass | |
| 18 | ET-window-manager-hooks-resources | Ada | Pass | |
| 19 | ET-window-manager-layout-recovery | Ada | Pass | |
| 20 | MS-configure-window-manager | Ada | Fixed and retested | BUG-20260801-window-manager-live-config-drift |
| 21 | MS-layout-profile-cli-roundtrip | Ada | Pass | |
| 22 | RT-home-usage-window-persistence | Cora | Pass | |

## Session Debriefs

- **Keyboard flow:** Created and grouped independent app instances, used Command-T and Command-K, reordered and tore out tabs, navigated deep routes, reloaded the browser, and dragged/resized the Network frame. Active members, routes, and placements remained stable.
- **Supervisor recovery:** Exercised close/reopen, pin protection, multiple desktops, pager switching, independent client presentation, and recovery from a focused desktop. Shared topology and client-local focus remained separated.
- **Agent parity:** CLI, HTTP, UDS, native tools, layout profiles, resources, and the hook catalog agreed on the same revisioned topology. Layout v3 round-tripped; v2 was rejected; apply/undo/redo and workspace-scoped resources behaved atomically.
- **Home canary:** The 30-day usage selection and expanded System row survived reload with truthful retention/cost presentation.

## What Was Fixed

- **BUG-20260801-window-manager-live-config-drift:** `compozy config set window_manager.*` previously routed through the generic reload path. Unrelated restart-required drift could block the live mutation, and a later section apply could hide that drift from `compozy status`. The CLI now uses the canonical Window Manager Settings endpoint, the Settings service projects only the live section over the active snapshot, and status compares desired and active hashes.
- Focused replay proved `defaults.provider=claude` remained pending while `window_manager.nav_stack_limit=1` applied at generation 1. Status reported `warn / pending_restart`, apply history retained different desired/active hashes, and two route pushes left exactly one navigation entry.

## Evidence

- Browser captures: `docs/qa/evidence/2026-08-01-window-tabs/`
- Config replay: `/Users/pedronauck/dev/qa-labs/compozy-window-tabs-live-apply-status-retest-20260801-115716-306628-lab/qa-artifacts/qa/evidence/`
- Main teardown: `/Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260801-085219-264358-lab/qa-artifacts/qa/teardown.json` (`clean: true`)
- Retest teardown: `/Users/pedronauck/dev/qa-labs/compozy-window-tabs-live-apply-status-retest-20260801-115716-306628-lab/qa-artifacts/qa/teardown.json` (`clean: true`)
- Automated support: full runtime E2E and full Web E2E passed before the focused config replay; focused Settings/API/CLI regression suites passed under `-race`; affected Go lint returned zero issues.

## Runtime Errors Observed

- The first main-lab daemon was stale and was rebuilt before behavior verdicts.
- A whole floating tab deck swaps as one arrangement unit. The CLI result and canonical reducer suite confirmed this is the intended topology contract, not a defect.
- The generic real-scenario evidence auditor is designed for autonomous multi-agent playbooks and rejected this focused retest lab's empty collaboration fields; no synthetic actor or provider evidence was added.

## Human Verifications Needed

None.

## Final Status

- **Exit gate:** final repository-wide gate remains pending and is tracked by the implementation plan; it is not represented as QA evidence here.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 1 fixed · Cosmetic 0
- **Coverage:** 22 / 22 scenarios settled
- **Verdict:** pass — all in-scope journeys completed, the discovered live-config regression was fixed and replayed, and every QA-owned process was torn down cleanly.

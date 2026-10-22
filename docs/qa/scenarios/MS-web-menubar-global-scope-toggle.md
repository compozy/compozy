---
id: MS-web-menubar-global-scope-toggle
area: MS
title: Menubar globe owns All projects; the switcher focuses a project or User
persona: Bruno
journey: J-operate-desktop-shell
expected: A 28px Globe toggle sits between the CompozyOS mark and the destination chip, outside `role="menubar"`; its accessible name is "All projects". Pressed means All projects (chip "All projects" with a stacked mark, tooltip "Back to {project | User}"); unpressed means the focus — a project (chip shows its name) or User (chip "User", `~`). The switcher lists All projects, User, then projects, with one ✓ on the current destination; picking User focuses User and binds the `user` desktop; picking a project or worktree turns All projects off. Turning the globe off returns to the last focus (project or User); a deleted project resolves to User, never to another project. With zero projects the globe is never locked: off means User. While the workspace catalog is still loading the globe waits (`aria-disabled`) and claims nothing. ⇧⌘G toggles the same control and is skipped on editable targets. A polite live region announces "All projects on/off". The v4 storage key upgrades schema v1 → v2 losslessly (Global → All projects focused on the remembered project or User; workspace → that project); per-window worktree picks survive leaving and returning to a project.
entry_points: web desktop menubar; ⇧⌘G; command palette Turn on/off Global scope
qa_status: untested
bug_ids: BUG-20261003-background-windows-forget-global; BUG-20260906-stopped-history-schema-upgrade
fix_status: fixed
retest_status: untested
fix_commits: 62b58628b62a03048967a91543323d3cb427bb7b; 3f53932aa35300c32e92bb6a84d469e1d89f367c; 7a9d15e2f
evidence: docs/qa/evidence/2026-10-02-untested/scope-legacy-crossdoc-bruno-confirm.json; docs/qa/evidence/2026-10-02-untested/scope-legacy-history-delivery-gate-retry.json; docs/qa/evidence/2026-10-02-untested/scope-legacy-history-evidence-audit-final.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-menubar-menu-set; ET-web-command-palette-shortcuts; MS-web-workspace-lists-hide-home; MS-global-scope-no-workspace-work; ET-profile-switcher-restore
---

story: As a builder I flip one menubar globe to work across every project folder, and I always know which destination create dialogs will use.

2026-09-10 CI regression: In Global with no remembered project, keep the resolved desktop
when background automation registers a workspace or polling reorders the catalog. Reload must
restore the same desktop and route, without setting a workspace data destination. Selecting a
project still changes desktops; deleting the retained project permits fallback. A Global desktop
opened before any project exists stays on the reserved partition until an explicit selection.
The v4 browser storage key upgrades its envelope from version 0 to 1 without losing scope,
selected project, or per-window worktrees. The workspace hook suite owns catalog/rehydration
coverage; `jobs-hardening.spec.ts` exercises the real background-job and reload journey.

Change impact audit (`docs/_memory/change-impact.md`): the browser retains only a desktop
partition ID. Workspace Query data and daemon window-manager partitions remain authoritative;
Global data reads stay unscoped and profile binding is unchanged. Native tool IDs, CLI/HTTP/UDS
contracts, hooks, extension SDKs, configuration, and `skills/compozy/` are unaffected because
their operations and inputs do not change. Web impact is limited to workspace resolution and
browser preference persistence; this scenario and `TA-web-automations-listing` own documentation.

Introduced 2026-08-12 by menubar-owned Global scope. Persist key `compozy:active-workspace:v3` stores `scope` plus `selectedWorkspaceId`. Empty v3 hydrates as Global. `$HOME` is not a UI row.

2026-08-23 qa-impact (Profiles): **reset from `blocked-verify` to `untested`** — the behavior this
row describes changed twice. Phase 0 redefines Global as the across-workspaces *view* whose
creations are no-workspace work rather than work in a pseudo-workspace, and the menubar gained the
profile switcher on the right side immediately before Settings while the leading cluster
(mark → globe → chip) is unchanged. Re-walk the toggle's own contract with a second profile
present, and confirm the globe and the profile switcher are independent axes that compose
visibly — turning Global on must not change the acting profile and switching profiles must not
change the workspace breadth. What Global now *means* for the data is owned by
`MS-global-scope-no-workspace-work`; the switcher itself is owned by `ET-profile-switcher-restore`.
The prior evidence paths predate both changes and are historical.

src: web/src/systems/os/components/global-scope-toggle.tsx; web/src/systems/os/components/desktop-menubar.tsx; web/src/systems/os/components/os-menubar.tsx; web/src/systems/os/components/menubar/workspace-menu.tsx; web/src/systems/workspace/stores/active-workspace-store.ts; web/src/systems/workspace/lib/active-workspace.ts

2026-08-20: Deletion notice removed from the workspace menu. The chip stays Global when a remembered folder is gone; the menu lists remaining folders with no info or warning banner. Walk this cycle: blocked-verify — workspace-menu unit suite asserts no note/alert in the menu; an isolated QA lab with a live daemon was not started.

2026-08-17: Global-on info notice removed from the workspace menu and the workspaces overview. The chip, globe, and live region already name the mode; the deletion notice stays when a remembered folder is gone. Walk this cycle: blocked-verify — workspace-menu and workspaces-overview unit suites assert the notice is gone and picking still works; an isolated QA lab with a live daemon was not started.

2026-08-13: Globe moved between the mark and the workspace chip (leading cluster). Expected placement updated; previous captures that show a trailing globe need a re-capture. Walk this cycle: blocked-verify — unit tests for the toggle passed; an isolated QA lab with a live daemon was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.

2026-08-13: labeled Switch replaced by a 28px Globe Toggle (`aria-pressed`, accent when on). Expected control updated; previous Storybook captures (VC-01–VC-04) show the Switch and need a re-capture.

2026-08-12 fix pass: honest locked tooltips (pick vs add), keyboard-reachable tooltip, visible disabled affordance, announce-on-change live region, and loading-vs-deleted separation landed after the visual capture; the walk below predates them.

2026-08-12 walk: blocked-verify. This implementation cycle captured Storybook visual-contract evidence (`.compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-01`–`VC-04`) and unit/typecheck coverage. An isolated QA lab with a live daemon (`COMPOZY_HOME`, production-parity web) was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.

2026-08-13 re-walk: Bruno switched from project `tmp` to Global through the globe, confirmed the project stayed available, then used the command palette action "Switch to tmp turns Global scope off". Refresh preserved `tmp`; the project menu never exposed the operator-home registration.

QA 2026-10-03: a background document in a project overwrites the saved Global scope when its
window catalog changes. The foreground still displays Global until reload, which then restores
the other project's scope/profile. The new bug owns the repair; the full charter stays Pending.
Evidence: automation-global-detail-config-trigger-reload-state.json/.png and
engineering automation-scope-persistence-two-document-{setup,observed}.json.

The bounded repair is verified by a fresh two-document Bruno replay: background window additions
and removals no longer erase Global on refresh. The existing persistence suite passes both
regressions; keyboard/palette toggles, editable-target suppression, profile independence and
390px compact leading controls also pass. Full scenario status remains untested until the
pending-resolution, zero/no-remembered selection and Global-session deep-link legs are complete.
Evidence: workspace-background-scope-bruno-ended.json and its inspected final/compact screenshots.

QA 2026-10-04: a retained scoped Loop run immediately reverses an explicit Global choice. The background-window bug is reopened for the route-adoption path. Catalog concurrency and runtime-off controls have current live evidence, but this scenario remains Pending for repair and the remaining charter legs.

QA 2026-10-04 repaired replay: the focused real-daemon regression and fresh Bruno session now
preserve explicit Global selection with a scoped Loop run retained, through Home/Jobs navigation,
failed Job dispatch and reload. Explicit foreground Loop navigation still adopts its known owner.
Evidence: catalog-loop-scope-green-corrected.json and catalog-recovery-replay-global.json in this
cycle's evidence directory. The bug is verified; the full scenario retains its pending-resolution,
zero/no-remembered-selection and session-deep-link obligations.

QA 2026-10-04 remaining legs: fresh Bruno sessions confirm the zero-project and unremembered-
project reasons, keyboard reachability, unchecked project menu and project restoration after
Global/reload. With the real workspace request delayed, cold entry mounts no scope controls and
the shortcut cannot change the eventual project. The historical Global-session link remains
Pending pending a real pre-profile fixture; no current session creation bypass is used.
Evidence: scope-completion-zero-toggle.json, scope-completion-project-restore.json and
scope-completion-catalog-latency-bruno-ended.json under this cycle evidence directory.

QA 2026-10-04: a genuine released home-owned history upgraded to the current daemon cannot
resolve the desktop or session owner. The retained-history upgrade bug is reopened; current
scope-control repairs remain verified, while this historical link path requires repair and
fresh replay. See the dated report and scope-legacy evidence.

QA 2026-10-04 verified: the genuine beta.19 history upgrades losslessly and opens read-only through
both links after Global confirmation. Decline, reload, remembered-project restoration, a concurrent
project document and editable-target shortcut suppression all pass. HTTP/UDS/CLI preserve the
original three lifecycle events and physical database owner. Current affected gates and strict
lab evidence audit pass; both exact-manifest Global labs have clean teardown receipts. The fixture
has no authored provider narrative, and no such rendering claim is made. See the final Global
closure in the cycle report for the complete earlier scope-control legs and evidence.

2026-10-21 qa-impact (user-scope-work, web destination model): **reset to `untested`**. The binary Global/workspace scope became the destination model (breadth + focus with User). Re-walk the switcher order, User focus and its desktop, the zero-project globe, the v1→v2 upgrade of a real browser profile, and two windows on different worktrees surviving project → User → project → reload (E-01). Owned by the trailing QA pair of the user-scope-work / all-projects delivery.


2026-10-21 qa-impact (all-projects AW3): Loops windows no longer adopt their route workspace. A `?workspace=` Loop run (bound window) renders in place and never changes the destination, so the 2026-10-04 retained-Loop-run reversal path is gone by design rather than guarded; `loops.spec.ts` now asserts the globe stays on through a bound Loop run permalink, the aggregate Runs and catalog, and a reload. Remains `untested` for the trailing QA pair.

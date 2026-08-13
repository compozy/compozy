---
id: MS-web-workspace-add-directory-browser
area: MS
title: Add workspace picks a root by browsing and registers once on submit
persona: Dora
journey: J-operate-workspace-context
expected: Add workspace opens as a two-pane extra-wide dialog. The left pane is a filesystem browser (home/up toolbar, Locations row, mono current path, "Use this folder", hover-reveal row picks) that chooses the root; there is no plain path input and no one-click global-default / home-folder card. Picking a root only updates the draft — it must not register a workspace — and it autofills the display name from the folder name until the operator types their own. The right pane carries optional session defaults: default agent, sandbox profile, and additional directories as removable chips. Exactly one `POST /api/workspaces` is issued when the footer primary is pressed, carrying `root_dir` plus any of `name`, `add_dirs`, `default_agent`, and `sandbox_ref` that are set. A failed registration reports inline and keeps every entered value. Below 980px the two panes collapse to one column with session defaults stacked underneath. The browser's reading, empty, and permission-error states are all visible. First-run onboarding uses the same browser; folders are optional and Skip starts in Global scope without calling `POST /api/workspaces/resolve` for `$HOME`.
entry_points: web desktop shell → Add workspace; web workspaces overview → New workspace; web first-run onboarding
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/web-onboarding-status.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/web-onboarding-complete.json
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: MS-web-entity-modal-shell
---

story: As a person running agent work I point Compozy at a real folder by browsing to it, review the defaults sessions will inherit, and commit once — without a half-registered workspace appearing while I am still looking around.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §4.3 and F7), task_02, implemented 2026-07-25. Before this change the dialog offered a plain absolute-path input and called `POST /api/workspaces/resolve`; the onboarding wizard's directory browser registered a workspace the moment a folder was picked.

`POST /api/workspaces` (`createWorkspace`) is now wired from the web client for the first time. `$HOME` is registered by the daemon, not by a UI card; Add workspace never offers a global-default row.

src: web/src/systems/workspace/components/workspace-setup.tsx; web/src/systems/workspace/components/workspace-setup-location-pane.tsx; web/src/systems/workspace/components/workspace-setup-defaults-pane.tsx; web/src/systems/workspace/hooks/use-workspace-setup-content.ts; web/src/systems/onboarding/components/directory-browser.tsx

inventory: Needs QA

2026-08-11 qa-impact: the picker gained a Locations row sourced from the daemon's filesystem roots, making paths outside the operator home discoverable without manual typing. Reset for targeted browser re-walk.

2026-08-11 retest: passed. The Locations row opened `/`, the operator browsed to the isolated QA project, selected it, and one submit registered and activated the workspace.

2026-08-12 qa-impact: menubar-owned Global scope deleted the one-click global-default card. Add workspace is project folders only; first-run Skip starts in Global without `resolve` for `$HOME`. Reset to untested.

2026-08-12 walk: blocked-verify. This implementation cycle captured Storybook visual-contract evidence (`.compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-01`–`VC-04`) and unit/typecheck coverage. An isolated QA lab with a live daemon (`COMPOZY_HOME`, production-parity web) was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.

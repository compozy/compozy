---
id: MS-web-workspace-add-directory-browser
area: MS
title: Add workspace picks a root by browsing and registers once on submit
persona: Dora
journey:
expected: Add workspace opens as a two-pane extra-wide dialog. The left pane keeps the one-click global-default workspace card, then a filesystem browser (home/up toolbar, mono current path, "Use this folder", hover-reveal row picks) that chooses the root; there is no plain path input anywhere. Picking a root only updates the draft — it must not register a workspace — and it autofills the display name from the folder name until the operator types their own. The right pane carries optional session defaults: default agent, sandbox profile, and additional directories as removable chips. Exactly one `POST /api/workspaces` is issued when the footer primary is pressed, carrying `root_dir` plus any of `name`, `add_dirs`, `default_agent`, and `sandbox_ref` that are set. A failed registration reports inline and keeps every entered value. Below 980px the two panes collapse to one column with session defaults stacked underneath. The browser's reading, empty, and permission-error states are all visible. The first-run onboarding page renders the same panes and the same single-create submit.
entry_points: web desktop shell → Add workspace; web workspaces overview → New workspace; web first-run onboarding
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_02/VC-03
last_report:
overlaps: MS-web-entity-modal-shell
---

story: As an operator I point AGH at a real folder by browsing to it, review the defaults sessions will inherit, and commit once — without a half-registered workspace appearing while I am still looking around.

Introduced by the modal redesign (`.compozy/tasks/modals-redesign/`, `_techspec.md` §4.3 and F7), task_02, implemented 2026-07-25. Before this change the dialog offered a plain absolute-path input and called `POST /api/workspaces/resolve`; the onboarding wizard's directory browser registered a workspace the moment a folder was picked.

`POST /api/workspaces` (`createWorkspace`) is now wired from the web client for the first time. `resolve` remains only behind the global-default card, which is a get-or-create for the home directory.

src: web/src/systems/workspace/components/workspace-setup.tsx; web/src/systems/workspace/components/workspace-setup-location-pane.tsx; web/src/systems/workspace/components/workspace-setup-defaults-pane.tsx; web/src/systems/workspace/hooks/use-workspace-setup-content.ts; web/src/systems/onboarding/components/directory-browser.tsx

inventory: Needs QA

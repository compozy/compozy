---
id: MS-web-workspace-add-directory-browser
area: MS
title: Add workspace picks a root by browsing and registers once on submit
persona: Dora
journey: J-operate-workspace-context
expected: Add project opens in Simple mode as a single-pane dialog holding only the filesystem browser (home/up toolbar, Locations row, mono current path, "Use this folder", always-visible row picks at reduced emphasis) that chooses the root; there is no plain path input and no one-click global-default / home-folder card. Picking a root only updates the draft — it must not register a workspace — and it autofills the display name from the folder name until the operator types their own. Switching to Advanced widens the dialog to two panes; the right pane, "Defaults for new sessions", carries the optional default agent and "Other folders agents can read" as removable chips. Exactly one `POST /api/workspaces` is issued when the footer primary is pressed, carrying `root_dir` plus any of `name`, `add_dirs`, and `default_agent` that are set. A failed registration reports inline and keeps every entered value. Below 980px the Advanced panes collapse to one column with the defaults stacked underneath. The browser's reading, empty, and permission-error states are all visible. First-run onboarding uses the same browser; folders are optional and "Skip — use my home folder" starts in Global scope without calling `POST /api/workspaces/resolve` for `$HOME`.
entry_points: web desktop shell → Add project…; web workspaces overview → New project; web first-run onboarding
qa_status: pass
bug_ids: BUG-20261002-directory-error-traps-navigation; BUG-20261002-onboarding-skip-keeps-project
fix_status: fixed
retest_status: pass
fix_commits: ebfb89518
evidence: docs/qa/evidence/2026-10-02-untested/directory-recovery-real-replay.json; docs/qa/evidence/2026-10-02-untested/workspace-browser-fixed-draft-catalog.json; docs/qa/evidence/2026-10-02-untested/workspace-browser-fixed-advanced-layout.json; docs/qa/evidence/2026-10-02-untested/workspace-browser-single-registration.json; docs/qa/evidence/2026-10-02-untested/workspace-browser-created-http.json; docs/qa/evidence/2026-10-02-untested/workspace-browser-created-uds.json; docs/qa/evidence/2026-10-02-untested/workspace-browser-created-refresh.json; docs/qa/evidence/2026-10-02-untested/onboarding-skip-scope-fixed-replay.json; docs/qa/evidence/2026-10-02-untested/onboarding-normal-finish-canary.json
last_report: docs/qa/reports/2026-10-02-untested.md
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

2026-08-20 qa-impact: density cleanup removed the Location and Session defaults helper paragraphs, the empty-root hint, the default-agent helper, and the footer registration one-liner. Display name default and extra-roots copy moved behind HelpTip. Reset to untested.

2026-08-23 qa-impact (Profiles): the browser must not offer the home directory as a registrable
root, and registration of it is now refused by the daemon rather than only omitted from the UI.
Already `untested`, so no reset was needed. Add one attempt to reach the home folder through the
browser and confirm the refusal is honest and leaves the draft intact. The daemon-side rule is
owned by `RT-home-workspace-not-registrable`.


2026-09-27 scope update: current coverage follows the surviving product surfaces; a fresh walk is required.

QA impact 2026-09-28 (ui-normie-pass): the dialog reads "Add project" and opens in Simple mode (browser
only); the session defaults pane moved behind Advanced, and its chips use the shared
`CommandSelectChip`. Not walked.

QA 2026-10-02: Failed at permission-error recovery. Home and Up are disabled and Locations disappears
after entering an unreadable child. A fresh-page retry reproduces it; closing/reopening alone retains
the failed path. No workspace was created. Remaining draft/defaults/submit legs await the repair replay.

QA 2026-10-02 repair replay: Permission recovery now passes in Add project and onboarding. Draft
selection makes no registration; automatic/custom names, removable extra-folder chips, default agent,
1512px/960px Advanced layout, home refusal with all values preserved, abandonment, and one valid
create POST were verified. HTTP/UDS and refresh retain the exact new Research notes workspace.
The populated-catalog onboarding Skip then exposed a separate failure: the previous project remains
selected instead of Global, confirmed by a fresh retry. This scenario remains fail/pending repair.

QA 2026-10-02 completed replay: Skip now selects persistent Global with the three-project catalog,
without registering/resolving Home. Leaving Global restores Research notes. Normal Finish setup
preserves the previous project, and UDS confirms the three complete records are unchanged. The
permission, draft, layout, failure, abandonment, single-submit, refresh, and onboarding legs are
complete. The canonical suites and production build pass; see the report for gate and commit evidence.

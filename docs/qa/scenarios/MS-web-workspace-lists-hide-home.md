---
id: MS-web-workspace-lists-hide-home
area: MS
title: Workspace lists hide the operator home row
persona: Bruno
journey: J-operate-workspace-context
expected: The project menu and Projects picker (also opened through the command palette) show project folders only, with the same identities in every profile. `$HOME` cannot be registered and never appears as a named row, pin, or Home badge. While Global is on the chip reads Global (`~`) and the picker does not mark any project as current. A real project selection survives refresh; cancelling leaves the current scope intact.
entry_points: web project menu; ⌘K Workspace picker; Projects picker (⌘⇧O); Add project
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-10-02-untested/workspace-visibility-all-profiles.json; docs/qa/evidence/2026-10-02-untested/workspace-visibility-project-selection-and-task-entry.json; docs/qa/evidence/2026-10-02-untested/workspace-visibility-catalog-after.json; docs/qa/evidence/2026-10-02-untested/workspace-visibility-global-picker.png; docs/qa/evidence/2026-10-02-untested/workspace-visibility-entrypoint-reconciliation.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: MS-web-menubar-global-scope-toggle; MS-web-workspace-add-directory-browser
---

story: As a builder I pick among project folders. My home directory holds user-wide resources, not a workspace I switch to.

Introduced 2026-08-12. `useActiveWorkspace().workspaces` is the project list; `registeredWorkspaces` remains the full catalog for session streams.

2026-08-21 qa-impact: daemon boot no longer creates an operator-home workspace, and explicit or automatic registration of `$HOME` is refused. Reset to untested; task 13 owns the live walk.

src: web/src/systems/workspace/lib/project-workspaces.ts; web/src/systems/workspace/lib/active-workspace.ts; web/src/systems/os/components/menubar/workspace-menu.tsx; web/src/systems/workspace/components/workspace-command-select.tsx

2026-08-12 walk: blocked-verify. This implementation cycle captured Storybook visual-contract evidence (`.compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-01`–`VC-04`) and unit/typecheck coverage. An isolated QA lab with a live daemon (`COMPOZY_HOME`, production-parity web) was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.

2026-08-23 qa-impact (Profiles): phase 0 completes the rule this row asserts — the home directory is
no longer auto-registered at boot and registration of it is refused at the daemon, not merely hidden
in the UI. Already `untested`, so no reset was needed; add one check that the workspace list renders
identically in every profile (workspaces are machine-global and no profile owns, hides, or
duplicates one). The daemon and structured-surface half of the rule is owned by
`RT-home-workspace-not-registrable`.

2026-10-02 current-entry reconciliation: `OsWorkspacesOverview` is the visible Projects picker,
not a separate route. Commit `8040723c9` removed the mounted `WorkspaceCommandSelect` consumers
from creation forms; the component remains an internal story/test surface. Creation destination
statements are covered by `MS-web-create-destination-derived`, not an unmounted selector claim.
This reconciliation happened after the persona walk ended and changes no production behavior.

2026-10-02 walk: Bruno compared all three named project identities across the project menu and
the picker opened from both menu and command palette in all five active profiles. Global showed
`~ Global` with no current marker; cancellation retained Research notes; selecting Editorial exited
Global and persisted after reload. HTTP before and UDS after returned the same full catalog.
The real Home-registration refusal and retained draft are reused from this cycle's completed
`MS-web-workspace-add-directory-browser` walk. No new registration or agent task was created.

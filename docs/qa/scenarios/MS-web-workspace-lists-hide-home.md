---
id: MS-web-workspace-lists-hide-home
area: MS
title: Workspace lists hide the operator home row
persona: Bruno
journey: J-operate-workspace-context
expected: The workspace menu, command palette workspace list, workspaces overview, and workspace command select show project folders only. `$HOME` never appears as a named row, pin, or Home badge. While Global is on the chip reads Global (`~`); the daemon may still register operator home for session binding. Overview does not mark a Current pill when Global is on (`activeWorkspaceId` is null).
entry_points: web workspace menu; ⌘K workspace rows; web Workspaces overview; Add workspace / command select
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-02
last_report:
overlaps: MS-web-menubar-global-scope-toggle; MS-web-workspace-add-directory-browser
---

story: As a builder I pick among project folders. Home is how Global sessions bind, not a workspace I switch to.

Introduced 2026-08-12. `useActiveWorkspace().workspaces` is the project list; `registeredWorkspaces` remains the full catalog for session streams.

src: web/src/systems/workspace/lib/project-workspaces.ts; web/src/systems/workspace/lib/active-workspace.ts; web/src/systems/os/components/menubar/workspace-menu.tsx; web/src/systems/workspace/components/workspace-command-select.tsx

2026-08-12 walk: blocked-verify. This implementation cycle captured Storybook visual-contract evidence (`.compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-01`–`VC-04`) and unit/typecheck coverage. An isolated QA lab with a live daemon (`COMPOZY_HOME`, production-parity web) was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.

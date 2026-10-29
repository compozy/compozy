---
id: MS-global-scope-no-workspace-work
area: MS
title: Focus User for work outside any project; All projects is the view across them
persona: Lea
journey: J-scope-global-across-workspaces
expected: User is a destination like a project: focusing it (switcher row "User") lists only User-scope items on every surface — sessions and terminals and Loops through `@user`, tasks and automations through `scope=global`, Home through `@user` — never an aggregate, on its own `user` desktop. All projects (the globe) is the across-projects view. With zero project folders the globe is not locked and turning it off focuses User. Work created outside a project has no workspace and is owned by the acting profile; the session catalog and its live stream apply the boundary on the server and send nothing while the destination is indeterminate; user-layer resources stay visible in every destination.
entry_points: web first run with zero folders; menubar globe toggle; shared creation surfaces while Global is on; compozy session list; GET /api/sessions and /api/sessions/catalog-stream over HTTP and UDS
qa_status: untested
bug_ids: BUG-20261004-task-execution-switches-unnamed; BUG-20261004-global-agents-project-gate
fix_status: fixed
retest_status: untested
fix_commits: 60ddd98e12e10731a7f98d8dedca20ff88f2e459
evidence: docs/qa/evidence/2026-10-02-untested/global-work-fixed-lea-ended.json;docs/qa/evidence/2026-10-02-untested/global-work-stream-boundary-verified.json;docs/qa/evidence/2026-10-02-untested/global-agents-fixed-lea-ended.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: MS-web-menubar-global-scope-toggle; RT-home-workspace-not-registrable; RT-web-session-all-workspaces; ET-profile-stream-isolation
---

Minted by Profiles task 12 (planning) for the task_01 QA-impact flag: phase 0 turns Global into the
across-workspaces view whose creations are no-workspace work, and moves session-catalog filtering
server-side as the enforcement point every profile read later reuses.
`MS-web-menubar-global-scope-toggle` owns the toggle control itself; this row owns what the view
means for the data. Task 13 owns the walk, the evidence, and the verdict.

Walk:

1. Finish first run with zero project folders and confirm the operator lands on a working desktop in
   the Global view with no full-page workspace gate.
2. With Global on, create work from a shared creation surface and confirm the stored item carries no
   workspace and is owned by the acting profile — check the same item through the web listing, the
   CLI, HTTP, and UDS.
3. Confirm that item is reported as no-workspace work rather than as work inside a hidden or implied
   folder, and that it survives a daemon restart with the same reading.
4. Open the session catalog and its live stream, then verify with a proxy or request log that the
   narrowing happens in the daemon's response — the client must never receive rows it then hides.
5. Force an indeterminate scope on the stream and confirm it sends nothing rather than everything.
6. Drop the stream, reconnect with a replay cursor, and confirm the same boundary holds across
   initial state, live updates, and replay.
7. Place a user-layer resource and confirm it resolves in Global and in every registered workspace.
8. Turn Global off with a remembered project present, and again with none, and confirm the restore
   and the honest stays-on reason respectively.

Expected evidence: first-run capture, the created item's stored representation on all four surfaces,
request-level proof of server-side filtering, the indeterminate-scope response, paired reconnect and
replay frames, and the user-layer resolution in two workspaces.


QA closure 2026-10-05: all planned legs above pass. Fix 60ddd98e12e10731a7f98d8dedca20ff88f2e459
passes the warning-free affected gate and matches its recorded tree. This scenario is
Fixed for the current cycle; earlier pending notes record the progression of the walk.

2026-10-21 qa-impact (user-scope-work, web destination model): **reset to `untested`**. Global split into All projects (breadth) and User (a focus). Re-walk User focus with zero projects and with projects present across Sessions, Tasks, Loops, Automations, Terminal and Home; daemon `@user` acceptance lands with the user-scope-work backend slices. Owned by the trailing QA pair.


Upgrade checks (U4): seed a stopped fixture with a `COMPOZY_HOME` registration, two profiles,
sessions/folders, tasks, Loop state, terminal recordings/spill artifacts and profile layouts. After
upgrade, verify User-only ownership, readable retained files and unchanged session owner stamps.
Interrupt journal import, then restart and confirm no duplicate imports. Keep an existing User
layout and confirm the source remains exportable. A source with worktrees or a Loop-target automation
must remain registered. Add an unknown referencing table and confirm the pre-delete assertion retains
the registration. Check renamed `global` selectors through CLI/HTTP/native and reserved WM behavior.
The U4 worker verifies SQLite, blob/restart, layout and dispatch fixtures; the full interface walk
remains pending integration QA.

2026-10-21 qa-impact (docs slice lab run, `docs/qa/reports/2026-10-21-all-projects-user-scope.md`):
the CLI leg `compozy session new --user` → `session prompt` → daemon restart → `session prompt`
failed on the integrated branch (reply-watch sender scope; CLI route without `workspace_id`; prompt
admission without workspace) and was fixed with regression coverage. Re-walk that CLI leg and the
HTTP `POST /api/workspaces/@user/sessions/<id>/prompt` leg in the trailing QA pair.


2026-10-21 backend gap follow-up: scope inputs for task creation/catalog and automation creation/catalog accept `user` and `@user`, persisting/emitting `global`. Scope remains immutable on updates. Include `compozy logs --workspace @user --type workspace.access_granted` in the User audit leg; it must exclude project rows and other profiles. Focused SQLite and native-dispatch integration coverage verifies the backend boundary; the complete browser scenario remains pending integration QA.

2026-10-21 qa-impact (all-projects AW2): Tasks and Home under All projects now read the explicit
aggregates (`all_workspaces=true` on tasks, task dashboard, and overview; `scope=all` on the scheduler
backlog); User focus keeps `scope=global` for tasks and `@user` for Home. Re-walk the User half with a
projectless task and confirm it never appears in a project's listing and shows the "User" badge under
All projects.

2026-10-28 terminal regression (#1008): in All projects, choose User in Terminal's creation
picker, attach and run a command, then create a second tab and cancel/confirm group close. Repeat
from the User desktop. Verify both terminals belong to `@user` and close a missing-terminal window
without a project-selection error. The acting profile supplies the launch environment. This focused
terminal walk does not replace the remaining multi-surface charter above.

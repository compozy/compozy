---
id: MS-global-scope-no-workspace-work
area: MS
title: Treat Global as a view and file its work without a workspace
persona: Lea
journey: J-scope-global-across-workspaces
expected: Global reads as a view across the registered project folders rather than a workspace; work created while it is on has no workspace, is owned by the acting profile, and reads back that way on every surface; the session catalog and its live stream apply the workspace boundary on the server and send nothing when scope is indeterminate; user-layer resources stay visible in every workspace and in Global alike.
entry_points: web first run with zero folders; menubar globe toggle; shared creation surfaces while Global is on; compozy session list; GET /api/sessions and /api/sessions/catalog-stream over HTTP and UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
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

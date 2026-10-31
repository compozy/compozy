---
id: ET-terminal-browser-lifecycle
area: ET
title: Use persistent terminals from the browser
persona: Marina
journey: J-operate-integrated-terminal
expected: Reload preserves terminal tabs and their processes. Window Close confirms running terminal termination before removing the window; Cancel preserves both; Stop leaves the window open; exited or missing terminals close directly; failures retain retryable windows. Group close confirms exactly its affected terminals and normal history remains retained. Under All projects the dock's Terminal item, with no Terminal window open, opens the Terminal launcher dialog instead of a window: New terminal rows for every project, each followed by its ready managed worktrees owned by the acting profile (read once from `/api/worktrees?all_workspaces=true`), plus User (the remembered project preselected, so Enter opens it) and a Running group read from `/api/terminals?all_workspaces=true` with project badges, kept live by the selection-bound aggregate stream; exited terminals are not listed. Opening a row lands in a Terminal window bound to that terminal's scope (a worktree row starts the shell in that worktree's checkout, bound to its project), and an unbound Terminal window (dock menu placement, restored layout) shows the same launcher in place and binds itself on open; a bound window keeps its binding across terminal switches. The dock live mark and bell terminal rows follow the same breadth.
entry_points: Web dock Terminal app; /terminal; /terminal/{terminal_id}
qa_status: untested
bug_ids: BUG-20260906-hidden-terminal-pane-minimum-vote
fix_status: none
retest_status: untested
fix_commits:
evidence: .compozy/tasks/sessions-stability/memory/terminal-retention-ci.md; /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-review-r2-20260902-020216-937662-lab/qa-artifacts/qa/screenshots/bruno-two-terminals-after-reload.png; /Users/pedronauck/dev/qa-labs/compozy-terminal-shared-control-20260904-204013-041114-lab/qa-artifacts/window-canary-after-reload.png; docs/qa/reports/2026-09-04-terminal-shared-control.md
last_report: docs/qa/reports/2026-09-04-terminal-shared-control.md
overlaps:
---

QA impact 2026-09-04: the Terminal app removed controller badges/actions and now accepts input as soon
as its interactive stream connects. Reset for a focused window lifecycle and shared-input re-walk.

qa-impact: 2026-09-01 deep-review round 2 changed terminal route retargeting, reconnect settlement,
window close handling, and browser recovery ownership. Reset for a focused public-surface re-walk.

2026-09-01 re-walk: passed. Bruno opened two browser terminals, executed a real shell command,
reloaded with both tabs intact, closed the window without ending either process, reopened the newest
running terminal from the dock, and closed an already-ended terminal without an error notification.
Independent `compozy terminal get|list` reads matched the rendered state throughout.

Flagged by integrated-terminal task 06; reset 2026-08-31 by the window-native
UX rework (internal tab strip removed; id-less route resolves itself; close is
idempotent). The prior pass predates that surface.

Walk:

1. Click the Terminal dock item in a project with no terminals; a working terminal opens with no launcher or empty-state click in between.
2. Use the head's New terminal; a second terminal joins the frame as an OS window tab; switch between both deck tabs and reload the browser.
3. Close a running Terminal window. Cancel the accessible confirmation and verify its process and window remain intact through CLI and refresh.
4. Confirm Close; verify the process exits before the managed window disappears. Reopen the browser and confirm it stays closed; inspect retained journal/recording history through the normal surfaces.
5. Stop a running terminal from its header; verify the window remains on its exit state, then close it without another running warning.
6. Close a group containing two terminals and an unrelated app; verify one confirmation lists only the affected running terminals. Cancel preserves the group; confirm closes the group without terminating terminals outside it.
7. Exercise the keyboard/window menu and close-other/right gestures. Pinned tabs remain protected by their existing scope rules.
8. Lose the connection while closing; verify visible failure feedback, a retained window, and a successful fresh retry after reconnect.
9. Confirm the route, title, dock badge, and another attached viewer remain truthful throughout.
   Under All projects, create and attach in a project and in User, run a command, then open a
   second tab from each bound window. Cancel and confirm group close without switching desktops;
   verify the target processes through the public terminal API. A bound window referencing a
   missing terminal closes without an error or confirmation. Repeat the User desktop canary.
10. Hold a terminal creation response, switch workspace or profile, and release it. The original
    owner's catalog retains the terminal; the destination desktop is not retargeted. Returning to
    the original scope must not revive the obsolete navigation. Its initiating tab offers
    **Open terminal**; activating it shows the process already created without adding another
    terminal. A creation without a scope change still opens its terminal normally.

QA impact 2026-09-10: issue #594 intentionally replaces the old operator view-only close behavior. Historical passes below describe the previous contract; this round is tracked in `docs/qa/reports/2026-09-10-issue-594-terminal-close.md`.

2026-09-04 targeted re-walk: passed. The Terminal app opened a second terminal, switched between OS
window tabs, survived minimize and reload with both instances restored, and closed one window without
ending the original terminal. A new browser session reattached to the running terminal with shared input.

QA re-walk 2026-09-10: PASS for the changed close contract. Production-bundle E2Es cover running cancel/confirm, grouped reload/history, disconnect feedback, Stop, and exited close. Manual isolated-browser checks cover keyboard/window-menu close, mixed-app groups, close-other/right targeting, shared viewers, and unchanged native view-only close. Scope and evidence: `docs/qa/reports/2026-09-10-issue-594-terminal-close.md`. Unchanged steps retain their earlier evidence.

Post-merge creation-scope verification is recorded in
`docs/qa/reports/2026-09-10-merged-pr-ci-review-remediation.md`; earlier browser evidence
does not independently prove the newly added pending-response step.

All-projects AW4 (2026-10-21): reset to untested. Walk with terminals in two projects and User under All projects: list with badges; open one in place (chrome badge, no project switch); New terminal in another project; close a terminal and see the list update live; switch profile and see the list re-read. PR screenshots: aggregate terminal list; New terminal project picker.


Issue #1034 bound-window walk (2026-10-29): run an isolated PTY that prints a
recognizable line and exits. Reopen its bound window while retention is active.
The retained output and exit outcome must remain readable, with zero attach-ticket
requests. A terminal that exits after a running catalog observation still uses
the protocol's existing 409 race handling. Verify a running terminal still
attaches normally. This does not change the launcher or aggregate catalog.
See the [gone-resource report](../reports/2026-10-29-gone-resource-requests.md).

Targeted re-walk 2026-10-29: PASS for the #1034 request-lifecycle steps above.
The broader historical scenario is not re-certified by this bounded walk.

Issue #1034 review regression: catalog exit may arrive before final stream frames.
Keep the existing attachment until its own exit; final bytes must remain visible.
Reconnect on a streamed exited pane must not fetch an unused retained snapshot.
The delayed-real-frame browser canary and canonical window suite cover this path.

2026-10-29 qa-impact (#1025): the All projects list window became the Terminal launcher. Walk:
with running terminals in a project and User plus one exited terminal, click the dock Terminal item
with no Terminal window open; confirm a dialog (not a window) opens with the remembered project
preselected, the Running group lists only the running terminals with badges, and the exited one is
absent. Type part of a project name and press Enter; confirm a bound window opens on a new
terminal. Reopen the launcher, arrow to a Running row, press Enter; confirm it re-attaches without
creating a terminal. Use the dock menu's Open in new window; confirm the window shows the launcher
in place. Choose a project whose folder was removed; confirm the toast names the missing folder
(typed `invalid_cwd`) instead of "Internal Server Error". Project desktops keep the direct open.
Isolated-lab evidence for this walk is linked from the PR.

2026-10-30 qa-impact (#1026): the launcher lists each project's ready managed worktrees and opens
a terminal in one. Walk: in a git project with one ready worktree of the acting profile, one pending
or removed worktree, and one owned by another profile, open the launcher under All projects;
confirm only the ready same-profile worktree appears under its project, select it, and confirm the
new terminal's `pwd` and `compozy terminal get` cwd are the worktree path while the window binds to
the project. Then `compozy terminal open --worktree <id> --detach -o json` and native
`compozy__terminal_open {"worktree": "<id>"}` start in the same checkout; the same id against another
workspace fails with typed `worktree_not_found`, the pending one with `worktree_not_ready`. While
the worktree terminal runs, `compozy worktree remove` refuses with `worktree_operation_in_progress`;
after the terminal exits the removal succeeds.

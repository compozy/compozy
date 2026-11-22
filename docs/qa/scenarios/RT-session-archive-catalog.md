---
id: RT-session-archive-catalog
area: RT
title: Archive and recover a stopped session without deleting history
persona: Théo
journey: J-archive-session-without-deleting
expected: Archiving is a durable workspace-scoped marker allowed only for stopped sessions; default lists exclude archived rows, archived-only and inclusive filters return exact pages, direct reads preserve history, prompt/resume/attach reject archived sessions, and unarchive restores the same session across HTTP, UDS, CLI, native tools, extension Host API, SSE, and fresh daemon reads.
entry_points: Web session catalog; CLI session archive/unarchive/list; HTTP+UDS archive routes; native tools; extension Host API
qa_status: untested
bug_ids: BUG-20260805-session-archive-sdk-missing;BUG-20260805-archived-detail-unarchive-missing;BUG-20261122-archive-stopped-denial-code-docs
fix_status: fixed
retest_status: pass
fix_commits: e40dc76;12e4be26d
evidence: /home/jilles/dev/qa-labs/compozy-archive-stopped-1940-20261122-185609-503208-lab/qa-artifacts/evidence/02-archive-alpha.json;/home/jilles/dev/qa-labs/compozy-archive-stopped-1940-20261122-185609-503208-lab/qa-artifacts/evidence/04-archive-alpha-subagent.json;/home/jilles/dev/qa-labs/compozy-archive-stopped-1940-20261122-185609-503208-lab/qa-artifacts/evidence/06-usage-errors.txt;/home/jilles/dev/qa-labs/compozy-archive-stopped-1940-20261122-185609-503208-lab/qa-artifacts/evidence/07-profile-flag.txt;/home/jilles/dev/qa-labs/compozy-archive-stopped-1940-20261122-185609-503208-lab/qa-artifacts/evidence/09-tool-all-denied.json
last_report: docs/qa/reports/2026-11-22-archive-stopped-1940.md
overlaps: RT-011; RT-042; RT-session-delete-owned-history
---

Archive must never delete or rewrite session-owned metadata, events, transcript, ledger, permissions,
or token statistics. Repeated archive and unarchive calls are idempotent. Cross-workspace requests
must not reveal or mutate the target session.

QA completion 2026-08-05: Cora and Ada completed the archive round trip through Web, CLI, HTTP,
UDS, native tools, and a live extension. The marker survived daemon restart, archived history stayed
readable, write guards held, and a second workspace received only not-found responses.

SQLite audit 2026-10-09: Archive and reopen a session whose event ledger contains a large archived prefix and retained active tail. Preserve transcript identities, content, cursor groups, archive-cut crossing refusals, and generation across upgrade and repeated reopen. A current projection no-op open must remain readable while another connection holds the writer. Existing sessiondb archive/projection/migration suites own these checks.

See [SQLite performance and contention evidence](../reports/2026-10-09-sqlite-performance.md)
for exact verification and limits. These backend checks do not replace or promote the
scenario's historical browser/provider verdict.

## Profile-bound session regression (#783)

From All profiles, operate on a stopped session owned by a nondefault profile. Rename,
archive, and restore it without switching the selected profile; inspect its deep link
and subagent roster. Confirm the request uses the row owner for mutations, the roster
returns successfully, and a terminal 404/410 does not cause recurring control polls.
A failed cold roster page must stop automatic pagination until an explicit retry.
Record the session slice separately from this scenario's broader coverage in
`docs/qa/reports/2026-10-17-issue-783-profile-sessions.md`.

## Archive a running session in one action (#908)

Archive a running session from the row "…" menu and from the right-click menu: the item reads
"Stop and archive session", one click ends with the session stopped and archived, and the live
list updates without a reload. Select a mix of running and stopped sessions and bulk-archive
them: the selection menu reads "Stop and archive", every session ends stopped and archived, and a
session whose stop fails stays unarchived with its own error while the rest succeed. Through the
CLI, `compozy session archive <id>` still refuses a live session with `409`, and
`compozy session archive <id> --stop` stops and archives it; HTTP/UDS `?stop=true` and native
tool `stop: true` behave the same.

## Archiving the open session clears its window (#1110)

Archive the session open in a window from its rail row (Archive, or Stop and archive while it runs):
the window drops it and shows "No session selected", with no "Session not found" toast, exactly as
after Delete. Every window showing that session does the same, and so does a bulk archive holding
it. With the session open, run `compozy session archive <id>` (or `compozy__session_archive`): the
window clears on the catalog reread, also without a toast. Archive another session: the window and
its focus stay. Open an archived session from the Archive shelf, including one you just archived in
this tab: it stays open, read-only; Unarchive there and archive it again from elsewhere and the
window clears. Move to Later does not clear the window: a parked session stays readable and its
composer is one way back (RT-session-later).

Request-ownership regression (#1384): delay the local archive HTTP response after
its catalog wake reports the session archived. Only the initiating window must
remain mounted until the response settles, so automatic window retirement cannot
abort its request. Repeat from a subagent window's sidebar while archiving its
parent, both with and without the optional child tree: subagents follow their
parent automatically. Other windows showing archived targets retire on the
catalog reread without waiting for this request. After the response, retire the
initiating window without a missing-session toast. The canonical SessionWindow
and session-actions suites cover this ordering and keep cancellation on genuine
hook unmount; E2E-PROC-002 owns the real browser receipt.

## Native error parity — 2026-10-28

Native archival refusals (#1012): a subagent child returns `subagent_archive_follows_parent` through both CLI and native tools. A runtime-detached but lifecycle-active session returns `session_archive_requires_stopped`. After stopping, native archive succeeds. Verify retained state after each refusal.

The [targeted report](../reports/2026-10-28-native-tool-errors.md) records the real isolated-daemon re-walk, automated evidence, and limits. Broader scenario status is unchanged.

## Child sessions — #1130

Create a parent with a stopped child and a live grandchild (`compozy session new --parent <id>`),
plus a session continued from the parent. `compozy session archive <parent>` leaves every child
active and listed under the top level. `compozy session archive <parent> --stop --with-children`
stops the grandchild and archives the tree, while the continued session stays. A native
`compozy__session_archive` with `with_children: true` from inside the tree is denied.
`GET .../sessions/<parent>/descendants` lists the tree whatever the catalog filter or page.
A stop that fails archives nothing and reports `session_archive_stop_failed`. Unarchiving the parent
leaves non-subagent children archived; restore them one by one. In the web, archive shows a
confirmation only when the target has children, with the checkbox checked and the count.

## Clean up archived history (#1717)

Fill one project's archive with threads archived 3, 10, 20 and 40 days ago. Include child
sessions, subagents, one child that is still live (unarchived) under an archived parent, and one
always-on agent's archived main session. In the sessions sidebar open Archived. The footer shows
**Clean up history…** and the archived count; the Active and Later views show no footer verb.

1. Open the dialog. Everything, Child sessions and Subagents are checked, and the last two are
   locked. The range is 7+ days ago. The headline count equals the Delete button's count and
   matches `compozy session cleanup -o json` for the same scope. The 3-day thread is not counted,
   and the preview says the live child stays.
2. Switch to Child sessions only and 30+ days ago: the count drops, and the main sessions stay
   after the run. Pick Before a date and Any time: the count follows. With nothing checked the
   preview asks to choose something and Delete stays off.
3. Under All projects with more than 100 sessions selected, the preview breaks the count down
   per project, and Delete stays off until the exact count is typed (separators optional).
4. Confirm, then close the dialog right away. The footer shows `Deleting… X of N`, rows disappear
   live, and the run ends in a "Deleted N sessions from …" toast. The live child, its subtree,
   the 3-day thread, the always-on main session (never counted in the preview) and every
   worktree remain.
5. Make one thread fail mid-run (for example, hold its session directory open read-only): the run
   ends under "Some sessions weren't deleted", listing that archived session, and **Try again**
   retries only that thread.
   Also hold a read-only history lease indefinitely: the delete must fail within its reader-wait
   budget, record the failure against that thread's root, retain its remaining sessions, and keep
   deleting other roots. The run must settle as partial (or failed if nothing was deleted), and a
   new run must be admissible. Release the lease before retrying the failed root. The canonical
   Manager and cleanup service suites cover the timeout and per-root continuation respectively;
   verification for this change is CI-only, with the manual walk pending.
6. Start a large run and press **Stop**: it ends after the deletes in flight, and what was deleted
   stays deleted. Restore one archived thread while a run is going: it is skipped, not deleted.
7. Through the CLI, `compozy session cleanup` only previews. `--yes` starts the run;
   `cleanup status` and `cleanup stop` follow it, and a second `--yes` while one runs returns
   `409`. Over HTTP an agent identity gets `403`. `compozy__session_cleanup` previews without
   approval, and with `dry_run: false` it asks for approval before it deletes.

Follow-up regression walk (#1741): remove only `meta.json` from stopped archived history with an
event database and an attachment, then delete it through both ordinary deletion and archived cleanup.
Neither history files nor attachments remain. Interrupt after staging: recovery restores the directory
when the catalog still owns it and finishes removal after catalog deletion, using the retained deletion
owner receipt. A wrong-profile/workspace cleanup request still cannot remove it. Archived deletion
sets any retained subagent child activity to stopped without changing its outcome. Stall one accepted
delete: its 30-second deadline fails that thread while the run can continue with other roots.
CI owns this verification; the manual walk is pending.

## Archive every stopped thread (#1924)

Fill one project with stopped threads (some with child sessions and subagents) plus one thread of
each skip kind: a running session, a stopped thread with an unresolved interaction (waiting on
you), queued input, a pinned session, a session in Later, and an always-on agent's main session. A
live session with a pending permission counts as `running`, since `running` is classified first;
stopping a session cancels its pending permission, so the thread becomes eligible. Repeat a smaller set in a
second project.

1. `compozy session archive --stopped --workspace <project> --dry-run` prints
   `Would archive N threads (M sessions) in <project>.` and
   `Skipped K threads: …` with one count per reason, largest first. Nothing is archived afterwards.
2. Without `--dry-run` the same counts print with `Archived`; every member of each archived thread
   is archived, and the skipped threads are untouched. `-o json` prints the HTTP response unchanged;
   unarchiving its `archived[].restore_ids` in order restores every thread, subagents included
   (each restores with its parent; unarchiving a subagent directly is refused).
3. Run it from the project directory without `--workspace`: it targets that project.
   `--all-workspaces` covers both projects and prints `in all projects.`; archived projects are left
   out.
4. `--stopped` with a session ID, or with `--stop`, `--with-children`, `--terminate-processes`, or
   `--keep-processes`, exits `2` without a request. Make one thread's archive fail: the summary lists
   it under `Failed`, the others archive, and the command exits `1`.
5. From an agent session, `compozy__session_archive_stopped` with `dry_run: true` runs without
   approval and reports the caller's own thread as skipped `caller`; without `dry_run` it archives
   and never archives the caller's thread. `all_workspaces: true` from a restricted agent returns
   `tool_denied` with reason code `workspace_access_denied`.

QA 2026-11-22 (PR #1940 at `db3f92bd5`, isolated lab, mock ACP agents): steps 1, 2, 3 and 5 pass, and
the global `--profile` flag archives only that profile's threads in a shared project. Step 4's usage
errors exit `2`; its per-thread failure is not reachable through public surfaces and stays owned by the
store and session suites. `waiting_on_you` was not reachable from a public seed (stopping a session
cancels its permission; a restart left no orphaned interaction); the store suite owns it. Step 5 found
the documented denial code wrong (BUG-20261122-archive-stopped-denial-code-docs, fixed in `12e4be26d`).
The other sections of this scenario were not re-walked, so `qa_status` stays `untested`. Report:
[2026-11-22-archive-stopped-1940](../reports/2026-11-22-archive-stopped-1940.md).

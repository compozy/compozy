---
id: RT-session-archive-catalog
area: RT
title: Archive and recover a stopped session without deleting history
persona: Théo
journey: J-archive-session-without-deleting
expected: Archiving is a durable workspace-scoped marker allowed only for stopped sessions; default lists exclude archived rows, archived-only and inclusive filters return exact pages, direct reads preserve history, prompt/resume/attach reject archived sessions, and unarchive restores the same session across HTTP, UDS, CLI, native tools, extension Host API, SSE, and fresh daemon reads.
entry_points: Web session catalog; CLI session archive/unarchive/list; HTTP+UDS archive routes; native tools; extension Host API
qa_status: pass
bug_ids: BUG-20260805-session-archive-sdk-missing;BUG-20260805-archived-detail-unarchive-missing
fix_status: fixed
retest_status: pass
fix_commits: e40dc76
evidence: /Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/journey-log.jsonl;/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/extension-host-api.json;/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/daemon-restart.json
last_report: docs/qa/reports/2026-08-04-session-archive.md
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

## Native error parity — 2026-10-28

Native archival refusals (#1012): a subagent child returns `subagent_archive_follows_parent` through both CLI and native tools. A runtime-detached but lifecycle-active session returns `session_archive_requires_stopped`. After stopping, native archive succeeds. Verify retained state after each refusal.

The [targeted report](../reports/2026-10-28-native-tool-errors.md) records the real isolated-daemon re-walk, automated evidence, and limits. Broader scenario status is unchanged.

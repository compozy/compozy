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

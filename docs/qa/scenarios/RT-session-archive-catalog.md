---
id: RT-session-archive-catalog
area: RT
title: Archive and recover a stopped session without deleting history
persona: Théo
journey: J-archive-session-without-deleting
expected: Archiving is a durable workspace-scoped marker allowed only for stopped sessions; default lists exclude archived rows, archived-only and inclusive filters return exact pages, direct reads preserve history, prompt/resume/attach reject archived sessions, and unarchive restores the same session across HTTP, UDS, CLI, native tools, extension Host API, SSE, and fresh daemon reads.
entry_points: Web session catalog; CLI session archive/unarchive/list; HTTP+UDS archive routes; native tools; extension Host API
qa_status: untested
bug_ids: BUG-20260805-session-archive-sdk-missing
fix_status: pending
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-011;RT-042;RT-session-delete-owned-history
---

Archive must never delete or rewrite session-owned metadata, events, transcript, ledger, permissions,
or token statistics. Repeated archive and unarchive calls are idempotent. Cross-workspace requests
must not reveal or mutate the target session.

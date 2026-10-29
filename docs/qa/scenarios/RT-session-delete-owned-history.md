---
id: RT-session-delete-owned-history
area: RT
title: Delete a stopped session with owned runtime history
persona: Bruno
journey: J-11
expected: Confirming deletion of a stopped session removes its catalog row, transcript/history, permission log, token statistics, and other session-owned rows atomically while preserving every other session.
entry_points: Web session Delete session modal; HTTP session DELETE; global session catalog
qa_status: blocked-verify
bug_ids: BUG-20260714-session-delete-history-fk
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260714-194637-422214-lab/qa-artifacts/qa/session-delete/; /Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260714-194637-422214-lab/qa-artifacts/qa/screenshots/session-delete-fresh-web.png;/Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: RT-034; RT-035; TA-task-create-async-activation
---

Exercise a real provider session with tool permission and token history; an empty synthetic session does not prove the ownership boundary.

2026-07-14 historical pre-rebase retest: the original failing task-role session `sess-12b2c865a27ecc72` survived that branch's v1 → v2 migration with five permission rows, then deleted successfully through the UI. Fresh HTTP read returned 404 and direct counts for its session, permission, and token rows were all zero. The merged stream carries the cascade in v3.

2026-07-14 final lifecycle retest: real Cursor/Grok user session `sess-6a4f5db74d195230` stored one authored prompt and response, stopped cleanly, and deleted through the named confirmation modal with `Session deleted.` The canonical manager suite also proved pre-commit directory rollback and post-commit tombstone retry semantics.

QA impact 2026-07-14: the API fallback was removed and daemon boot now requires the atomic workspace-removal preparer. Reset pending final-worktree deletion and foreign-session preservation evidence.

2026-07-14 final-worktree control: public detail, history, transcript, usage, event, and catalog reads stopped exposing the deleted session, while the neighboring session retained its history and usage. Retest promoted to pass.

## Stopped Goal owner with an unbound checkpoint

In an isolated fixture, retain a session-origin Goal Run with a supported idle checkpoint whose
session binding has not been established, and a stopped origin session. Delete that origin through
the session lifecycle surface. Expect successful deletion, absence from session reads, a canceled
Run with retained checkpoint/audit history, and an unchanged neighboring session. The cancellation
projection uses the persisted origin identity; the checkpoint remains unbound. Repeat cancellation
without creating duplicate projection events. Never alter an operator's live database to seed this case.

The store turn-runtime suite owns the exact empty-binding regression. The managed runtime suite
owns deletion through the real Manager and SQLite. GitHub CI runs both with race detection; the
existing Goal command E2E covers public stop/remove/clear/replace with ACP subprocesses.
The issue-specific automated verdict, exact commit and CI job receipts are recorded in
[PR #647 — How you verified it](https://github.com/compozy/compozy/pull/647).
That record covers this stopped/unbound deletion slice; the historical evidence above is separate
and is not promoted to a new full manual-scenario pass.

## Live session with delegated descendants

In an isolated daemon, create a parent session that delegates a child which delegates a
grandchild holding an active turn. Delete the parent through the workspace-scoped HTTP route;
repeat with the child as the deletion target. Each request must complete, retained descendants
must be stopped, and a subsequent session must reach active state and delete successfully
through UDS without restarting the daemon.

The owning suite is `TestSubagentRootHostedMCPDaemonIntegration` in
`internal/daemon/subagent_integration_test.go`. It runs the real daemon, SQLite, hosted MCP,
HTTP/UDS, and ACP fixture subprocesses. Before the fix, parent deletion exhausted the request's
60-second deadline: deletion held the global lifecycle mutex while synchronous subagent
settlement tried to acquire it again. Session creation uses the same mutex. The fix settles
execution and Goals before entering the globally serialized history-removal phase, while
retaining the conversation-operation lock for the target session.

The incident log at 2026-10-21 13:23:48 -03:00 recorded a canceled deletion waiting for that
session's conversation operation. Shutdown began at 13:26:50; the next boot classified
interrupted sessions as `agent_crashed`. Those recovery labels do not independently establish
a provider crash. The original process has no retained goroutine dump; the deterministic
regression establishes the deadlock path and its repair, rather than every historical interleaving.

Automated retest 2026-10-21: both deletion variants and the existing hosted-MCP delegation/cancel
cases passed with `go test -tags=integration ./internal/daemon -run '^TestSubagentRootHostedMCPDaemonIntegration$' -count=1 -timeout=240s -v`.
The targeted `TestManagerDelete` suite also passed with race detection.
These receipts cover this live-tree slice; they do not promote the historical manual scenarios
above to a new full-scenario pass.


QA impact 2026-10-10 (verification pending): On native Windows, remove a stopped session with attachment files and nested directories after querying its history and again after a daemon restart. Confirm no Access is denied error, no remaining catalog row, and no staged/committed tombstone leak. Exercise catalog-failure rollback and interrupted tombstone recovery. Owning suite: TestManagerDelete, including pooled-reader quiescence, rollback, and recovery cases.

Automated retest 2026-10-10: The startup/rooted-write/deletion step in [native Windows CI](https://github.com/compozy/compozy/actions/runs/38112607881/job/114392299915) passed at `3cabffd82`. It covers stopped-session deletion, reader quiescence, catalog-failure rollback, interrupted recovery for all catalog states, and bound removal of files plus nested directories without leftover private directories. Cleanup retains mandatory directory flushes and checks the identity of its writable handle. This is owning-suite native evidence, not a full manual provider-session walk.


Issue #1034 Web boundary walk (2026-10-29): open the session being deleted and
hold delivery of the real DELETE response. During the pending mutation, inspect
network traffic: outline/search, scoped detail, and presence must remain stopped.
Abort the DELETE before it reaches the daemon and verify rollback resumes reads
and presence. Then complete the real deletion and verify no request restarts for
the removed session. A presence lease 404 still reacquires a lease for a surviving
session; it must not retire the session window. This targeted walk does not
replace the descendant/provider lifecycle scenarios above. See the
[gone-resource report](../reports/2026-10-29-gone-resource-requests.md).

Targeted re-walk 2026-10-29: PASS for the #1034 request-lifecycle steps above.
The broader historical scenario is not re-certified by this bounded walk.

Issue #1034 presence ownership regression: switching a watcher from A to a
deleting B releases A's lease and does not acquire B's. The canonical presence
hook suite owns this exact interleaving; the real deletion/rollback re-walk
confirms the unchanged public lease lifecycle.

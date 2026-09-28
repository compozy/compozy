---
id: RT-session-derive-retry
area: RT
title: Retrying a continue returns the recorded outcome and never creates a second session
persona: Rafa
journey: J-15-operate-session-via-cli-api
expected: Repeating compozy session continue (or POST …/continue, or compozy__session_continue) with the same idempotency key and the same request returns the recorded outcome with replayed true (HTTP 200, CLI "Replayed  yes") even after the source gained new turns; after the child is deleted it returns the outcome with child_deleted true and creates nothing; the same key with a different request returns idempotency_conflict; stale fences return session_fence_conflict; a first message admitted but never dispatched before a restart is dispatched exactly once by the retry.
entry_points: compozy session continue <id> --agent <name> --idempotency-key <key> [--message …] [-o json]; POST /api/workspaces/{workspace_id}/sessions/{session_id}/continue; compozy session delete <child>; compozy session list -o json; compozy logs --session <child> --type session.derived -o json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-cli-session-continue
---

Planning 2026-09-28 (session-continue-fork task_03): new behavior.

1. `compozy session continue <src> --agent claude-code --idempotency-key idem-qa-1 --message "go" -o json`
   returns `201`-equivalent output with `derived.replayed: false`; note the child id.
2. Prompt the source once more, then repeat step 1 verbatim: the output shows the same child id,
   `Replayed  yes`, and the recorded counts from step 1 (not the new source state). `compozy session
   list -o json` shows one child; `compozy logs --type session.derived` shows one event.
3. Repeat with the same key but `--agent codex`: `idempotency_conflict` (409), nothing created.
4. Pass stale `--expected-*` fences with a new key: `session_fence_conflict` (409), nothing created.
5. `compozy session delete <child>`, then repeat step 1: the recorded outcome returns with
   `child_deleted: true`, no `session`, and no new session is listed.
6. Restart window: stop the daemon right after a continue with `--message` returns (before the first
   turn runs), start it, and repeat the same command: the message is dispatched exactly once.

Automated evidence at authoring time: session manager derive receipt/idempotency cases and the HTTP
replay transport cases (`TestContinueSessionHandler`). task_07/08 own the walk.

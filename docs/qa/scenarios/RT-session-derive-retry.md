---
id: RT-session-derive-retry
area: RT
title: Retrying a continue returns the recorded outcome and never creates a second session
persona: Rafa
journey: J-15-operate-session-via-cli-api
expected: Repeating compozy session continue (or POST …/continue, or compozy__session_continue) with the same idempotency key and the same request returns the recorded outcome with replayed true (HTTP 200, CLI "Replayed  yes") even after the source gained new turns; after the child is deleted it returns the outcome with child_deleted true and creates nothing; the same key with a different request returns idempotency_conflict; stale fences return session_fence_conflict; a first message admitted but never dispatched before a restart is dispatched exactly once by the retry; a retry after the source was deleted still replays; a failure after the child was committed names it as child_session_id.
entry_points: compozy session continue <id> --agent <name> --idempotency-key <key> [--message …] [-o json]; POST /api/workspaces/{workspace_id}/sessions/{session_id}/continue; compozy session delete <child>; compozy session list -o json; compozy logs --session <child> --type session.derived -o json
qa_status: pass
bug_ids: BUG-20260928-derive-replay-deleted-child-origin-lost
fix_status: fixed
retest_status: pass
fix_commits: uncommitted (task_08 part B1)
evidence: docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/retry-deleted-source.txt; docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/http-committed-child.txt; docs/qa/evidence/2026-09-28-session-continue-fork-b1/retry-1.json; docs/qa/evidence/2026-09-28-session-continue-fork-b1/retry-2.txt; docs/qa/evidence/2026-09-28-session-continue-fork-b1/retry-3-4.txt; docs/qa/evidence/2026-09-28-session-continue-fork-b1/retry-5b.txt; docs/qa/evidence/2026-09-28-session-continue-fork-b1/retry-6g-events-after-restart.txt; docs/qa/evidence/2026-09-28-session-continue-fork-b1/retry-6h-events-after-retry.txt
last_report: docs/qa/reports/2026-09-29-session-continue-fork-r1-rewalk.md
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
7. Deleted source (review round 1 #10): continue A→B with key K, delete B, delete A, then repeat the
   same command from A's workspace: the recorded outcome returns with `replayed: true`,
   `child_deleted: true` (HTTP `200`), not `404`. The derive and preview routes never rewrite the
   source's metadata (no repairing read).
8. Post-commit failure (review round 1 #9): continue with `--message` onto an agent whose runtime
   cannot authenticate: `422` carries `child_session_id`; the CLI prints the session id and the key to
   rerun with; the web dialog shows **Open new session** and opens that child.
9. Round 2 (review round 2 #1–#3): after step 8's refusal, change the agent in the web dialog and
   submit again: the `idempotency_conflict` refusal shows and **Open new session** stays offered and
   opens the first child. A model refusal (`model_unavailable`) prints `model_unavailable: …` in the
   CLI and carries `"code": "model_unavailable"` over HTTP. Continue a stopped source whose
   `meta.json` still claims a live process through the CLI without fences: its `meta.json` bytes are
   unchanged afterward. Retest owed on the next walk (automated: `TestDeriveCommitBoundaries`,
   `TestContinueSessionHandler`, `TestSessionDeriveCommandDaemonFailures`,
   `TestDaemonE2ESessionContinueCLI`, `session-continue-dialog.test.tsx`).

Automated evidence at authoring time: session manager derive receipt/idempotency cases and the HTTP
replay transport cases (`TestContinueSessionHandler`). task_07/08 own the walk.

## 2026-09-28 walk (task_08 part B1) — FIXED

Théo, Interrupt Tour, isolated lab `compozy-session-continue-fork-b1-20260929-010817-219689-lab`, acpmock `retry-source` → `retry-target` (lab fixture), plus real Claude for one restart attempt.
1. `continue --idempotency-key idem-qa-1 --message "go now"` → child, `replayed false`, the child answered. 2. After a new source turn the
same command returned the same child, `Replayed yes`, the recorded `2 messages · 0.3 KiB`; one child listed, one `session.derived`.
3. Same key, `--agent beta` → `idempotency_conflict` (CLI 65, HTTP 409). 4. Stale fences, new key → `session_fence_conflict`, nothing created.
5. `session remove <child>` then the same command → `Child deleted yes`, no session created — but the origin agent printed `--`
(BUG-20260928-derive-replay-deleted-child-origin-lost, fixed: the receipt records `origin_agent_name`; re-walk prints `(retry-source)`).
6. `--message` returns only after the child's first bind dispatched the message (a 6 s slow-start agent made the call return after
6 s), so the "admitted but never dispatched" crash window is not reachable from the CLI; it stays covered by `TestDeriveCommitBoundaries`.
Walked instead: `kill -9` of the daemon right after the return (turn in flight), restart, same command → `replayed true`, and the
child has exactly one `user_message` and one `session/prompt` in the driver diagnostics (the interrupted turn is recorded as
`agent_crashed`, not re-run).

## 2026-09-29 re-walk (review round 1) — PASS

Théo, lab `…-r1-rewalk-…`, acpmock. Step 7 (deleted source): `continue A --agent beta --idempotency-key qa-r1-delsrc` → child B; `session remove B`, `session remove A` (`session status A` now 404). The same command from A's workspace (cwd resolution) printed `Replayed yes` and `Child deleted yes` with origin `alpha`. `-o json` has `replayed: true`, `child_deleted: true`, and HTTP `POST …/continue` answers `200` with the same outcome. Nothing was created. Preview of the deleted source answers `404 session_not_found`. The derive and preview routes leave the source `meta.json` sha unchanged. Step 8: HTTP `422` carries `child_session_id`. The CLI prints the child id and the rerun key. The Web dialog shows **Open new session** and opens the child (after BUG-20260929-derive-refusal-below-fold was fixed, the offer is scrolled into view). "A committed child survives a post-commit failure" stays owned by `TestDeriveCommitBoundaries` and cannot be reached from the CLI. Evidence: `docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/retry-deleted-source.txt`, `docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/http-committed-child.txt`.

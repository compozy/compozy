# BUG-20260713-task-role-session-never-starts: Task-role workers connect but never receive their first turn

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-complete-task-tree, release Ready children to an agent pool
- **Scenarios:** TA-task-role-session-activation; TA-parent-rollup-completion; LP-task-rollup-wakes-loop
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** AGH-71 integrated replay

## Summary

When a queued run starves, AGH creates and connects a real Cursor task-role session with the correct task/run prompt overlay, but never sends an initial prompt or starts a turn. The run remains queued and unbound through repeated escalation cycles. After 15 minutes the idle system session is reaped by TTL and the Web renders it only as `failed`, with a disabled composer and no actionable failure details.

Two independent AGH-71 child runs reproduced the same behavior. This blocks real agent claim/completion, parent rollup, and the downstream Loop wake even after Web owner clearing was fixed.

## Reproduction

- **Charter:** CH-task-tree-loop-rollup · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US; isolated daemon; live Cursor provider configured for Grok 4.5.

1. Create a Ready workspace task owned by Agent / pool `general` and enqueue its run.
2. Wait for scheduler starvation recovery to create the task-role session.
3. Open the agent's Sessions tab and the new `task-role:general:coord-run-*` session.
4. Inspect the task run after repeated scheduler cycles and then after the session TTL.

**Expected:** Once the Cursor ACP transport connects, AGH starts a turn using the task-role prompt overlay. The worker calls `agh task next --wait -o json`, claims and attaches the queued run, and completes or fails it through the lease contract. If startup fails, the run and session surface an actionable typed cause promptly.
**Actual:** Session `sess-fbc0f0f9edf012ea` connected in about 4.2 seconds and logged `daemon: starvation worker spawned`, but its ledger contains only `session.post_create`; no prompt, agent turn, or tool call exists. Run `run-0dc2db2a608bf620` stayed queued/unbound, and the session ended only as `spawn_reaper:ttl_expired`. Session `sess-c6d3ba0f7edea93b` reproduced the same outcome for the second child.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-task-role-session-never-starts.dom.txt`
- `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-108e1613c829/runtime/sessions/ws_06366aad69887872/sess-fbc0f0f9edf012ea/ledger.jsonl`
- `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-108e1613c829/runtime/sessions/sess-fbc0f0f9edf012ea/meta.json`
- `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-108e1613c829/runtime/logs/agh.log` around `2026-07-13T02:57:12-03:00`.
- Tasks `task-f6638f9897b1b0f8` / `task-a090a4e5ba779d61`; runs `run-0dc2db2a608bf620` / `run-1bbcc49e5cc31c45`.

## Fix

- **Root cause:** The task-role runtime treated `SessionManager.Create` as activation. Create completed subprocess/ACP/session setup and persisted `session.post_create`, but `creation_profile.prompt_overlay` was only system context; no real prompt consumed it, so the worker had no turn and could not self-claim.
- **Correction:** After creating or reusing a task-role session, the daemon sends one correlated synthetic prompt for the task/run. In-flight `(session_id, run_id)` dispatches coalesce, event streams are drained under daemon shutdown ownership, and a synchronous first-prompt failure stops a newly created session with a typed failed cause so scheduler retry remains possible. Claim/attach remains owned by `agh task next`.
- **Fix commit:** pending
- **Regression test:** `internal/daemon/task_role_runtime_test.go` covers enqueue/recovery and starvation prompts, metadata correlation, in-flight coalescing, failed-start cleanup/retry, joined cleanup errors, and Cursor explicit/default model projection.

## Verification

- **Retested:** 2026-07-13T10:58Z → 2026-07-13T10:59Z with live Cursor/Grok 4.5 after Web Recover queued `run-be2c1d6592e2c043`.
- New system session `sess-1e9a13013651c8b0` reached a real first turn instead of idling after `session.post_create`. Its visible transcript identified the exact task, run, channel, and `agh task next --wait -o json` claim path in 21 seconds.
- Evidence: `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-task-role-synthetic-turn-fixed-agent-mode-blocked.dom.txt`.
- Full claim/completion, AGH-71 parent rollup, and Loop wake remain blocked by the separately tracked Cursor mode defect: the live worker reports Ask mode even though AGH session capabilities project Agent mode. That downstream blocker does not reproduce this bug's former zero-turn/TTL-only failure.
- **Final live acceptance:** Cursor Agent mode is now available. Sessions `sess-0bb0f23ac1414396`, `sess-64f9badf5a65dd2f`, `sess-23117c8e3aad8ea6`, and `sess-d84ebf495d1547f6` each received the correlated activation, claimed their exact run, performed the read-only contract, and completed once. No session idled at `session.post_create`, expired by TTL, or received a duplicate initial turn.

---
id: RT-parked-child-idle-ttl
area: RT
title: Run the idle clock only while a child is parked
persona: Bruno
journey: J-message-a-running-agent
expected: The idle clock arms only on park and suspends the moment a call or message arrives, a session with an open call is never reaped, and expiry terminalizes every queued message with that reason before the target is finalized.
entry_points: compozy call reviewer "Park after completion" --idle-ttl 30s; compozy config set calls.idle_ttl 30s; compozy message send ses_01JBD8G2MZTX "Wake and continue"; compozy call ses_01JBD8G2MZTX "Check the tests too"; compozy session status ses_01JBD8G2MZTX; HTTP and UDS GET /api/workspaces/{workspace_id}/calls/{call_id} and assert idle_expires_at
qa_status: pass
bug_ids: BUG-20260826-parked-child-idle-clock
fix_status: fixed
retest_status: pass
fix_commits: 22e982fc6; 76dcc3d5a
evidence: /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/ttl-public-retest.md; /Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/scenario-walk-matrix.md
last_report: docs/qa/reports/2026-08-26-agent-comms.md
overlaps: RT-agent-call-follow-up; RT-agent-mailbox-send-list; RT-session-stop-subtree
---

A parked child is a resource with a clock on it, and the whole value of parking depends on that
clock being honest. Three things must hold.

**The clock arms only on park.** While a call is in flight `idle_expires_at` is null; it becomes a
timestamp when the child parks. Set a deliberately short `--idle-ttl`, give the child work that
outlasts it, and confirm the working child is never clock-reaped — the reaper must skip any session
with a call in `queued` or `running`.

**Contact suspends it immediately.** Send a message to a parked child, or call its session id, and
confirm the clock stops on contact rather than at the next boundary, and that park state clears only
after a successful wake — not optimistically at the attempt.

**Expiry is loud, not silent.** Let a parked child with queued messages actually expire. Every queued
message must terminalize `failed` naming the expiry reason **before** the target is finalized, so a
later `compozy message list` explains what happened instead of showing an eternally queued row. Then
call the expired session id and confirm `call_target_expired` points at calling the agent fresh.

Also confirm the exclusions: operator-caller sessions are outside reaping, liveness caps, targeting
and revival entirely, and a drain produces the same terminalization with the drain reason attached.

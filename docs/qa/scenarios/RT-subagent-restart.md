---
id: RT-subagent-restart
area: RT
title: Subagent results and wakes survive a daemon restart
persona: Dora
journey: J-automatic-runtime-recovery
expected: A subagent that settles while the daemon is down is finalized at boot and wakes its parent exactly once; a wake already queued before shutdown is not duplicated; a daemon killed mid child turn recovers the child through ordinary session recovery and the row finalizes and wakes once when the child settles; boot recovery fails delegations stuck in queued for more than 2 minutes with "delegation interrupted", re-admits a missing first prompt once, re-offers open wakes whose input is missing, claims pending rows of idle parents, and stops subagent sessions that have no record, logging each action as subagent.recovered with its reason; results stay readable through the native tool, CLI, and HTTP/UDS after restart.
entry_points: compozy daemon stop/start (or kill -9 of the daemon process); compozy session subagents <session-id>; compozy session subagents show <subagent-id> --json; compozy__subagent_status; daemon log (subagent.recovered)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-subagent-delegate; RT-session-spawn-wake
---

Spec: `.compozy/tasks/subagents/_spec.md` Delivery and lifecycle rule 11 (Recover) and ADR-002.
Automated owners: IT-011, IT-012, IT-031 (acpmock, restarted daemon over the same DB). This scenario
is the real-lab walk.

1. **Settles while down.** Delegate a long task from a Claude parent. Stop the daemon cleanly while the
   child runs; let the provider process finish (or replay with acpmock a child that settles before the
   daemon is back). Start the daemon. Expect the row `completed` with its result, exactly one wake turn
   on the parent, and `subagent.recovered{reason=finalized}` in the log.
2. **Wake queued before shutdown.** Let a child settle while the parent is mid-turn so the wake is
   queued, then stop the daemon before the parent's turn ends. After restart, the parent receives that
   wake once (no duplicate input row with the same wake id).
3. **Killed mid child turn.** `kill -9` the daemon while the child is streaming. After restart the
   child session recovers through normal session recovery; when it settles, the row finalizes and wakes
   the parent once.
4. **Stuck delegation.** Interrupt a delegation between reserve and child start (acpmock boot delay +
   kill). After restart, a `queued` row older than 2 minutes is `failed` with `delegation interrupted`,
   any partly created child is stopped, and `subagent.recovered{reason=stale_reserved}` is logged.
5. **Orphan session.** A `spawn_role = "subagent"` session with no record (seeded) is stopped at boot
   with `reason=orphan_session_stopped`.
6. **Reads after restart.** `compozy session subagents <parent>`, `show --json`, and
   `compozy__subagent_status` return the same records and results as before the restart; the Web card
   for each subagent shows its final status after reload.

QA impact 2026-10-08 (subagents): new in this change; no prior verdict.

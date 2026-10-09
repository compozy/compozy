---
id: RT-subagent-restart
area: RT
title: Subagent results and wakes survive a daemon restart
persona: Dora
journey: J-automatic-runtime-recovery
expected: A subagent that settles while the daemon is down is finalized at boot and wakes its parent exactly once; a wake already queued before shutdown is not duplicated; a daemon killed mid child turn recovers the child through ordinary session recovery and the row finalizes and retains one wake when the child settles; a parent classified as a dead process runtime remains read-only under the existing attachment contract; boot recovery fails delegations stuck in queued for more than 2 minutes with "delegation interrupted", re-admits a missing first prompt once, re-offers open wakes whose input is missing, claims pending rows of idle parents, and stops subagent sessions that have no record, logging each action as subagent.recovered with its reason; results stay readable through the native tool, CLI, and HTTP/UDS after restart.
entry_points: compozy daemon stop/start (or kill -9 of the daemon process); compozy session subagents <session-id>; compozy session subagents show <subagent-id> --json; compozy__subagent_status; daemon log (subagent.recovered)
qa_status: fail
bug_ids: BUG-20261009-subagent-daemon-stop-cancels; BUG-20261009-subagent-crash-no-wake
fix_status: pending
retest_status:
fix_commits:
evidence: .compozy/tasks/subagents/orchestration/screens/pr/
last_report: docs/qa/reports/2026-10-09-subagents.md
overlaps: RT-subagent-delegate; RT-session-spawn-wake
---

Spec: `.compozy/tasks/subagents/_spec.md` Delivery and lifecycle rule 11 (Recover) and ADR-002.
Automated owners: IT-011, IT-012, IT-031 (acpmock, restarted daemon over the same DB). This scenario
is the real-lab walk.

1. **Settles while down.** Delegate a long task from a Claude parent. Stop the daemon cleanly while the
   child runs; let the provider process finish (or replay with acpmock a child that settles before the
   daemon is back). Start the daemon. Expect the row `completed` with its result and exactly one
   durable wake waiting for explicit parent resume (no implicit resume), then one wake turn.
   Recovery logs `subagent.recovered{reason=child_reconciled}`.
2. **Wake queued before shutdown.** Let a child settle while the parent is mid-turn so the wake is
   queued, then stop the daemon before the parent's turn ends. After restart and explicit parent
   resume, the parent receives that wake once (no duplicate input row with the same wake id).
3. **Killed mid child turn.** `kill -9` the daemon while the child is streaming. After restart the
   child session recovers through normal session recovery; when it settles, the row finalizes and
   retains one durable wake. A parent classified as a dead process runtime remains read-only under
   the existing attachment contract; do not bypass that gate to deliver the wake. A resumable
   shutdown parent receives its wake after explicit resume. Explicit user stop still disposes it.
4. **Stuck delegation.** Interrupt a delegation between reserve and child start (acpmock boot delay +
   kill). After restart, a `queued` row older than 2 minutes is `failed` with `delegation interrupted`,
   any partly created child is stopped, and `subagent.recovered{reason=stale_reserved}` is logged.
5. **Orphan session.** A `spawn_role = "subagent"` session with no record (seeded) is stopped at boot
   with `reason=orphan_session_stopped`.
6. **Reads after restart.** `compozy session subagents <parent>`, `show --json`, and
   `compozy__subagent_status` return the same records and results as before the restart; the Web card
   for each subagent shows its final status after reload.

QA impact 2026-10-08 (subagents): new in this change; no prior verdict.

Automated restart evidence (2026-10-09): `internal/daemon/subagent_integration_test.go`
uses fresh daemons and Managers over the same home/DB. `TestSubagentSettledRestartDaemonIntegration`
replays an unobserved real-ACP completed-child checkpoint and reopens twice, asserting stable
settlement/wake identities and one delivery on explicit resume. `TestSubagentCrashRestartDaemonIntegration`
kills the daemon-hosting process mid child turn and verifies ordinary crash repair and one retained wake.
`TestSubagentFirstAdmissionRestartDaemonIntegration` reopens a linked child with `pending_task`
and no admission, verifies one readmission/result, and proves the reaper respects a shutdown parent.
These tests cover daemon/storage/ACP boundaries; public CLI/HTTP/Web journey validation remains with tail QA.

QA walk 2026-10-09: reads survive restart; a clean daemon stop cancels the running child (no wake); kill -9 fails the row and disposes it with no wake. Steps 2, 4, 5 not walked. Verdict: fail. Report: `docs/qa/reports/2026-10-09-subagents.md`.

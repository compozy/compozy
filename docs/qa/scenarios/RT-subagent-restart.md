---
id: RT-subagent-restart
area: RT
title: Subagent results and wakes survive a daemon restart
persona: Dora
journey: J-automatic-runtime-recovery
expected: A clean daemon stop is never a cancel: a running subagent is not settled canceled or disposed by shutdown, its child session is resumed after restart like any other session, and the row finalizes and wakes its parent exactly once when the child settles (a child that cannot be resumed settles interrupted with its wake kept for the parent); a subagent that settles while the daemon is down is finalized at boot and wakes its parent exactly once; a wake already queued before shutdown is not duplicated; a daemon killed mid child turn (kill -9) settles the running row failed with its wake kept for a resumable parent, while a parent recovered from SIGKILL stays read-only under the existing attachment contract; boot recovery fails delegations stuck in queued for more than 2 minutes with "delegation interrupted", re-admits a missing first prompt once, re-offers open wakes whose input is missing, claims pending rows of idle parents, and stops subagent sessions that have no record, logging each action as subagent.recovered with its reason; results stay readable through the native tool, CLI, and HTTP/UDS after restart.
entry_points: compozy daemon stop/start (or kill -9 of the daemon process); compozy session subagents <session-id>; compozy session subagents show <subagent-id> --json; compozy__subagent_status; daemon log (subagent.recovered)
qa_status: pass
bug_ids: BUG-20261009-subagent-daemon-stop-cancels; BUG-20261009-subagent-crash-no-wake
fix_status: fixed
retest_status: pass
fix_commits: 5b33ec550
evidence: .compozy/tasks/subagents/orchestration/screens/pr/
last_report: docs/qa/reports/2026-10-09-subagents-r2.md
overlaps: RT-subagent-delegate; RT-session-spawn-wake
---

Spec: `.compozy/tasks/subagents/_spec.md` Delivery and lifecycle rule 11 (Recover) and ADR-002.
Automated owners: IT-011, IT-012, IT-031 (acpmock, restarted daemon over the same DB). This scenario
is the real-lab walk.

1. **Clean stop mid child turn.** Delegate a long task from a Claude parent. `compozy daemon stop`
   while the child runs. Shutdown is not a user cancel: the row must not become `canceled` or
   `disposed`. Start the daemon. The child session is resumed or recovered like any other session,
   and when it settles the row finalizes (`completed` with its result) and the parent gets exactly one
   wake turn. If the child cannot be resumed after the clean restart, the row settles `interrupted`
   with its wake kept (delivery not `disposed`), and the parent is woken when it resumes. A child that
   settles while the daemon is down (acpmock replay) is finalized at boot with
   `subagent.recovered{reason=child_reconciled}` and wakes the parent once.
2. **Wake queued before shutdown.** Let a child settle while the parent is mid-turn so the wake is
   queued, then stop the daemon before the parent's turn ends. After restart and explicit parent
   resume, the parent receives that wake once (no duplicate input row with the same wake id).
3. **Killed mid child turn.** `kill -9` the daemon while the child is streaming. After restart the
   running row settles `failed` (the child crashed with the daemon), recovery logs
   `subagent.recovered{reason=child_reconciled}`, and the row's wake is kept for a resumable parent
   (delivery not `disposed`). A parent that was itself recovered from SIGKILL stays read-only under
   the existing attachment contract, so no wake turn runs on it; do not bypass that gate. The result
   and error stay readable through `compozy__subagent_status`, the CLI, and HTTP. Explicit user stop
   still disposes the wake.
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

2026-10-09 fix round 1 (docs): steps 1 and 3 now state the controller decision for D-06/D-07 (clean
stop is not a cancel; crash settles `failed` with the wake kept). Verdict stays `fail` until the re-walk.

Re-walk 2026-10-09 (stock 980d51fbe): a clean stop kept the row `running`, resumed the child, and delivered one wake after the parent was resumed (a wake turn cut by shutdown was re-offered once). kill -9 settled the row `failed` with its wake kept and the parent read-only. Reads survived restarts. Steps 4 and 5 need acpmock seeding and stay with IT-031. Verdict: pass. Report: `docs/qa/reports/2026-10-09-subagents-r2.md`.

SQLite audit 2026-10-09: Seed pending deliveries for multiple parents, settle one parent, and verify that only its successors are consumed. Boot recovery must still discover every parent. Paginate tied creation timestamps within one workspace and parent without duplicates or skipped rows; foreign rows remain inaccessible. Existing store/session/daemon subagent recovery suites own the backend checks.

See [SQLite performance and contention evidence](../reports/2026-10-09-sqlite-performance.md)
for exact verification and limits. These backend checks do not replace or promote the
scenario's historical browser/provider verdict.

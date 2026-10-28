---
id: RT-subagent-restart
area: RT
title: Subagent results and wakes survive a daemon restart
persona: Dora
journey: J-automatic-runtime-recovery
expected: Eligible subagent turns resume once after requested restart or daemon crash, preserve partial results and durable wakes, and finalize through the subagent owner; explicit daemon or session stops never auto-resume, and skipped recovery retains inspectable results.
entry_points: compozy daemon restart; isolated daemon SIGKILL; compozy daemon stop/start; compozy session subagents; compozy__subagent_status; HTTP/UDS session reads
qa_status: untested
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

1. **Requested restart mid child turn.** Delegate a long task and request `compozy daemon restart`.
   The subagent owner resumes an eligible child once using the shared continuation safety checks;
   the ordinary-session boot pass must not claim it too. Partial text remains, and the completed
   child finalizes its row and wakes the parent exactly once. Pending approvals, changed bindings,
   or exhausted restart attempts require review instead of blind continuation.
2. **Wake queued before restart.** Let a child settle while the parent works, then restart with
   the wake queued. The parent continues its interrupted work first and receives the existing wake
   once afterward. Repeat another restart and confirm the durable wake identity is not duplicated.
3. **Killed mid child turn and explicit stop.** SIGKILL only the isolated daemon. After boot, eligible
   children and ordinary parents continue without a false `agent_crashed` failure. Recorded real
   provider failures remain failures. Repeat with `compozy daemon stop`: explicit stop cancels work
   and start does not auto-resume it. Explicit parent stop still disposes its wake.
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

QA impact 2026-10-23 (#939): requested restart and SIGKILL now share safe continuation; explicit daemon stop is cancellation. Earlier verdicts below describe the previous contract and are historical. Re-walk before delivery.

Targeted automated verification 2026-10-23 (#939): the changed runtime contracts passed in the
canonical session/daemon suites and isolated real-process probes. Restart/stop detail and persisted
events were independently read over HTTP and UDS after replacement and a third boot; no false failed
badge or duplicate continuation was observed. See
[the scoped report and public readback evidence](../reports/2026-10-23-issue-939-restart-continuation.md).
The full persona/Web/CLI journey remains untested; this automated evidence does not replace that verdict.

2026-10-27 update continuity regression: start concurrent work and a delegated child before
app Update. After native installation and runtime replacement, the same session IDs and partial
text remain; eligible parents and children continue without a manual prompt or a false failure.
The child completes and delivers its durable result once. An explicitly stopped control remains
stopped, and a subsequent restart does not duplicate continuation or child delivery. A failed
bundle install leaves the live daemon untouched; failure to request coordinated restart never
falls back to user-stop signaling. Existing opt-out, pending-interaction and binding-change gates
remain in force. E2E-016 owns the app/daemon handoff; session/daemon suites own recovery safety.
Evidence and execution limits: [continuity report](../reports/2026-10-27-update-session-continuity.md).

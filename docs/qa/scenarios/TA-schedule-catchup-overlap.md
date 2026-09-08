---
id: TA-schedule-catchup-overlap
area: TA
title: Recover one scheduled fire without overlap
persona: Bruno
journey: J-24
expected: Restart downtime under run_once_on_catchup dispatches one latest missed fire; skip_missed beyond grace and an overlapping active run persist canceled history rows with misfire_grace_exceeded or self_overlap, and the next cycle remains eligible.
entry_points: Web automation job form and run history; automation CLI/HTTP/UDS/native tools; daemon restart
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-ta-replay-20260730-062156-531636-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: TA-055; TA-063
---

story: As an autonomy operator, I configure a recurring job to recover once after downtime and can explain every suppressed fire from durable history.

Added by the Hermes comparison D2 implementation. Flag only; the next QA cycle owns execution.

Phase C planning 2026-07-19: persona normalized to Bruno, journey reference normalized to J-24;
settles US-005 AC-1..AC-3 (D2, ADR-007/ADR-010) — TA-daemon-lifecycle-command-guard owns AC-4.

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Downtime window timestamps and the single catch-up run row under `run_once_on_catchup` (durable
  cursor advanced once).
- The grace-aware skip reason under `skip_missed` and the `self_overlap` skip reason in job
  history, with the next cycle normal.
- The claim-CAS at-most-once check (no double-fire) across restart.

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-005-schedules-recover-once-never-overlap-never-target-the-daemon

QA impact 2026-09-06 sessions-stability task_05 (walk owned by final task_10): occupy the shared
automation concurrency gate when both a one-shot and recurring job become due. Inspect the durable
deferral and original fire/run IDs. Release capacity and repeat across daemon restart: each original
fire runs once, then later missed times follow the configured catch-up policy. Change or disable
a deferred schedule and confirm the old reservation is canceled and cannot be revived by a stale retry.
Also inspect scheduler counters: skipped/coalesced/rate-limited heartbeat attempts do not increment
successful sends or arm cooldown. Sustained known capacity waiting advances bounded escalation to
one canonical capacity event and needs_attention; recover through the existing task-run surface
and confirm claimability after capacity returns.

QA 2026-09-06 — sessions-stability selected scope: PASS for selected capacity/restart branches: the original deferred one-shot and recurring run IDs completed attempt1 after restart, then the recurring schedule was disabled. Known task capacity advanced one canonical event and needs_attention at four configured cycles; public recovery produced linked attempt2 and task next claimed it. Exhaustive edit/disable/counter cases reuse current task05 integration evidence; prior unrelated catch-up branches retain their historical scope. Evidence: docs/qa/reports/2026-09-06-sessions-stability.md.

---
id: TA-schedule-catchup-overlap
area: TA
title: Recover one scheduled fire without overlap
persona: Bruno
journey: J-24
expected: Restart downtime under run_once_on_catchup dispatches one latest missed fire; skip_missed beyond grace and an overlapping active run persist canceled history rows with misfire_grace_exceeded or self_overlap, and the next cycle remains eligible.
entry_points: Web automation job form and run history; automation CLI/HTTP/UDS/native tools; daemon restart
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-055, TA-063
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

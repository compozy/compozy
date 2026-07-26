---
id: TA-daemon-lifecycle-command-guard
area: TA
title: Reject daemon lifecycle commands before scheduling
persona: Bruno
journey: J-24
expected: Creating a dynamic automation job with a command-shaped AGH daemon restart, stop, or kill instruction fails with the stable blocked class and persists no job; prose in non-command fields remains valid.
entry_points: automation CLI; HTTP/UDS POST /api/automation/jobs; agh__automation_jobs_create
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-055, TA-schedule-catchup-overlap
---

story: As an autonomy operator, I cannot schedule a job that terminates its own daemon and creates a supervisor restart loop.

Added by the Hermes comparison lifecycle-guard implementation. Flag only; the next QA cycle owns execution.

Phase C planning 2026-07-19: persona normalized to Bruno, journey normalized to J-24; settles
US-005 AC-4/EC-1 (ADR-010 §1). The optional operator bypass was NOT adopted (one unconditional
creation-seam validator — recorded shared decision).

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Rejected creation attempts (CLI and `agh__automation_jobs_create`) with the deterministic
  blocked-class error naming the command class; nothing persisted.
- The accepted prose-mention run (false-positive guard: command-shaped regex, not prose scanning).

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-005-schedules-recover-once-never-overlap-never-target-the-daemon

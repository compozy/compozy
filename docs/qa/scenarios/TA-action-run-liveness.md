---
id: TA-action-run-liveness
area: TA
title: Bound action runs by deadline and observable progress
persona: Ada
journey: J-bound-runaway-work
expected: An action without a node timeout inherits the configured deadline, active tools and fresh activity avoid false idle failures, a wedged action is canceled with node_timeout or no_progress with its lease freed and the loop advancing, and a timeout consumes the shared O1 attempt budget instead of reclaiming forever.
entry_points: `agh config set task.orchestration.action_run_timeout`; `agh task inspect <run-id> -o json`; task-run listing over CLI/HTTP/UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-019; TA-022; TA-023; TA-workspace-run-capacity; TA-lease-recovery-attempt-budget
---

Added by the Hermes comparison recovery/liveness milestone. Exercise both inherited and explicit
node deadlines, observable ACP activity, an active long-running tool, and a genuinely idle action.
Repeated lease-expiry attempt budgeting is owned by TA-lease-recovery-attempt-budget (US-012); the
two couple only through the shared budget a timeout consumes.

Flag only: the later Hermes comparison QA cycle owns execution and evidence.

Phase C planning 2026-07-19 (corrected same day): linked to J-bound-runaway-work; settles US-015
ONLY (O4 wall-clock + progress-aware liveness, Safety Invariant 25; UT-105–110, IT-038). An
initial planning fold of US-012 into this file was reversed — _tests.md assigns O1 and O4 to
distinct invariant owners (UT-102–104/IT-035 vs UT-105–110/IT-038), so US-012 has its own
scenario, TA-lease-recovery-attempt-budget.

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- The wedged fixture's terminal reason (`node_timeout`/`no_progress`) with the loop advancing and
  the freed lease.
- The healthy-long-run survival log (active in-tool run untouched at the idle window; inherited
  vs explicit deadlines both honored).
- The timed-out run consuming the O1 attempt budget (never reclaim-forever) — budget exhaustion
  itself is proven in TA-lease-recovery-attempt-budget.

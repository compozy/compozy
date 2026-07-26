---
id: TA-lease-recovery-attempt-budget
area: TA
title: Bound crash-looping recovery by the attempt budget
persona: Ada
journey: J-bound-runaway-work
expected: A run claimed then abandoned repeatedly consumes the durable attempt/recovery budget on every lease-expiry requeue and terminalizes to needs_attention with lease_recovery_exhausted at max_attempts, carrying a forensic reason that distinguishes crash-loop from ordinary failure while the token-fenced snapshot CAS and normal release-requeue semantics stay intact.
entry_points: agh task next --wait -o json; POST /api/agent/tasks/claim-next; agh task inspect <run-id> -o json; scheduler recovery sweep
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-action-run-liveness; TA-workspace-run-capacity
---

Queue one worker run and crash-loop it: claim, kill the worker pre-heartbeat, let the lease expire,
and let the recovery sweep reclaim — repeatedly through the same run row. Each recovery must
increment the durable `recovery_count`, requeue only while `attempt + recovery_count <
max_attempts`, and terminalize the row to `needs_attention` with `lease_recovery_exhausted` once
the bound is hit. Confirm the terminal reason is distinguishable from ordinary failure, the
token-fenced snapshot CAS in expired-lease requeue rejects stale owner tuples, and normal
`ReleaseRunLease` requeue semantics are unchanged.

Minted by the hermes-comparison Phase C planning cycle for US-012 (TechSpec §3.10 O1, Safety
Invariant 22; UT-102–104, IT-035). Distinct invariant owner from TA-action-run-liveness (US-015 O4
liveness): the two couple only through the shared budget — a timeout consumes the same bound this
scenario proves.

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Repeated kill-claim cycle timestamps through the same run row with the incrementing durable
  recovery budget.
- The terminal row showing `needs_attention` + `lease_recovery_exhausted` at max_attempts,
  distinguishable from ordinary failure.
- The recovery-sweep log showing the bound (no further requeue after exhaustion) and the fencing
  rejection of a stale owner tuple.

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-012-crash-loops-are-bounded

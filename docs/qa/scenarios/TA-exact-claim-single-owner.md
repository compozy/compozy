---
id: TA-exact-claim-single-owner
area: TA
title: Exactly one owner wins a contested exact claim
persona: Ada
journey: J-bound-runaway-work
expected: Two concurrent exact ClaimNextRun calls naming the same queued RunID converge to exactly one owner; the loser receives the typed no-claimable-run outcome, never a false success, and an exact claim on an already-claimed or running run returns a typed error without overwriting ownership.
entry_points: POST /api/agent/tasks/claim-next; agh task next --wait -o json; agh__task_run_claim_next
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-workspace-run-capacity; TA-action-run-liveness
---

Queue one worker run, then race two concurrent exact claims (`ClaimCriteria.RunID`) against it from
two agent clients. Confirm one succeeds and one receives the typed no-claimable-run outcome, and
that the winning owner's lease is the only ownership row. Repeat against an already-claimed and a
running run and require typed errors with no ownership overwrite. Exact and next-work selection must
share the same guarded queued-status CAS — fencing strength cannot diverge (Safety Invariant 24);
non-claim `UpdateTaskRun` mutations remain unchanged.

Minted by the hermes-comparison Phase C planning cycle for US-014 (TechSpec §3.10 O3): no existing
scenario owned the contested exact-claim invariant — TA-workspace-run-capacity owns the capacity
race, not same-RunID ownership.

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Race transcript of both concurrent claim commands with their structured responses (one success,
  one typed no-claimable-run).
- Run listing after the race showing a single owner tuple (claimed_by, claimed_at, lease).
- Typed-error captures for exact claims against claimed and running runs.

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-014-exactly-one-manual-claim-wins

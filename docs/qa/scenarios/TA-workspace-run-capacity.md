---
id: TA-workspace-run-capacity
area: TA
title: Defer and drain workspace runs at the active-run limit
persona: Ada
journey: J-operate-bounded-task-capacity
expected: A full workspace returns typed capacity deferral while preserving queued work, other workspaces plus global and Network wake work remain claimable, and the deferred run claims when capacity opens.
entry_points: `compozy config set task.orchestration.max_active_runs_per_workspace`; `compozy task next --wait -o json`; `POST /api/agent/tasks/claim-next`; `compozy__task_run_claim_next`
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-ta-replay-20260730-062156-531636-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: TA-049; TA-044; TA-024
---

Added by the Hermes comparison claim/cap milestone. The limit counts claimed, starting, and running
worker/coordinator runs with live leases in the selected run's workspace. `0` disables the bound.

Flag only: the later Hermes comparison QA cycle owns execution and evidence.

Phase C planning 2026-07-19: settles US-016 AC-1..AC-3 (O5, Safety Invariant 26) —
TA-task-wake-dedup owns EC-1.

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Fan-out saturation run showing bounded admission waves and the typed capacity deferral.
- Deferred-then-drained run rows (durably enqueued, claimed as capacity frees, attempt unchanged).
- The one-slot concurrency race admitting exactly one claim, and the workspace-B isolation probe.

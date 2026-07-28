# L-003 — `task_runs` is the single durable work queue

**Class:** Architecture / Autonomy
**Date discovered:** 2026-04-25
**Evidence sources:** Architecture review records forbid duplication.

## Context

Designing the autonomy MVP, several alternatives proposed a parallel scheduler-owned queue ("scheduler claims, then asks task service to assign"). All were rejected.

## Root cause

Two durable queues for the same work creates two sources of truth for ownership state. Any divergence (lease vs. claim, sweep vs. heartbeat, recovery vs. boot) becomes a race. The existing `task_runs` table already carries `status`, `attempt`, `idempotency_key`, `origin_kind/ref` — adding state via columns and side tables is strictly cheaper than adding a parallel table.

## Rule

> `task_runs` is the single durable work queue. Do not introduce a parallel queue or actor table. Add new ownership/state via columns + side tables on `task_runs`.

## Lease invariants (operationalization)

1. Exactly one active claim token per non-terminal run.
2. Heartbeat/complete/fail/release compare run owner + claim token.
3. Stale/late after recovery fails explicitly (no silent reassignment).
4. Sweep + heartbeat serialize via SQLite tx (`BEGIN IMMEDIATE`).
5. Boot recovery runs BEFORE the scheduler accepts wake/claim traffic.
6. Lease extension is bounded by config.
7. One active lease per session in MVP.

## Side-table strategy

- Capability matching = exact-match rows in `task_run_required_capabilities` and `task_run_preferred_capabilities`.
- Coordination channels: `coordination_channel_id` column on `task_runs`.
- Permission narrowing data: side table indexed by `task_run_id`.
- **Never** stuff dynamic ownership/match state into a JSON metadata blob.

## Source

Analysis corpus: docs/\_memory/analysis/analysis_compozy_tasks.md (lesson 1).

# L-004 — Manual operator paths converge with autonomous on the same primitives

**Class:** Architecture / Autonomy
**Date discovered:** 2026-04-25
**Evidence sources:** Repeated spec and implementation review findings.

## Context

Early autonomy drafts treated user-driven flows and agent-spawned flows as separate code paths. Pedro pushed back: "autonomy is additive, never replacement."

## Root cause

Splitting manual and autonomous into "user mode" and "agent mode" creates two implementations of every safety primitive (claim, lease, heartbeat, complete, fail, release, narrow). Inevitably they drift. Operators end up with weaker invariants than agents (or vice versa), and the system loses the property that operator and agent flows can interleave safely.

## Rule

> Manual operator paths and autonomous paths converge on the same primitives. User-created tasks, automation-created tasks, coordinator-created tasks, and agent-spawned child tasks all use the same task/run model and the same claim-token/lease/heartbeat/complete/fail/release rules.

## Operationalization

- **Task creation alone NEVER enqueues claimable work or starts the coordinator.** Publish/start/approval is the run-enqueue boundary that triggers coordinator bootstrap.
- **Operator commands are identity-explicit; agent commands are identity-implicit.** Operator endpoints MUST NOT infer agent identity from environment variables.
- **No separate manual/autonomous/coordinator queues.** All converge on `task_runs` with `actor_kind` differentiating origin.
- **E2E coverage MUST include both manual-first bookends:**
  1. `user create → publish → coordinated execution`
  2. `user-start session → direct prompt without coordinator`
- **Operator UI must visually distinguish creation vs. publish/approval vs. run enqueue vs. coordinator spawn.**

## Source

Analysis corpus: docs/\_memory/analysis/analysis_compozy_tasks.md and docs/\_memory/analysis/analysis_existing_surfaces.md.

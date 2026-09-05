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

- Task creation does not enqueue work; publish/start/approval is the boundary that triggers execution. Persist explicit `actor_kind`; operator identity is explicit and agent identity follows its authenticated context, never environment-variable inference. Both use the same execution queue.
- Manual and autonomous paths share `task_runs` and the same ownership primitives.
- When those boundaries change, verify the affected manual-first and direct-prompt paths in the existing E2E suite. UI changes distinguish creation, approval, enqueue, and coordinator state; unrelated tasks do not repeat both journeys.

## Source

Analysis corpus: docs/\_memory/analysis/analysis_compozy_tasks.md and docs/\_memory/analysis/analysis_existing_surfaces.md.

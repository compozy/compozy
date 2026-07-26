# L-005 — Authoritative primitives are exclusive

**Class:** Architecture
**Date discovered:** 2026-04-25; reinforced 2026-04-26
**Evidence sources:** Implementation review and global runs analysis.

## Context

The mechanical scheduler in `internal/scheduler` was tempted to claim runs directly during sweeps and recoveries. That would have collapsed the agent-pull model into a daemon-push model and given two components authority over the same state transition.

## Root cause

When two components can perform the same authoritative state transition (claim, spawn, migrate, narrow), they will eventually disagree. Either you serialize them (introducing locks, latency, and complexity), or you accept two sources of truth (introducing races and recovery bugs). The clean answer is to pick one authority per transition and force everything else to _observe and notify_ rather than _act_.

## Rule

> When an authoritative primitive owns a state transition (`task.Service.ClaimNextRun`, `Spawn`, `EnsureMigration`), no peer package may replicate the transition. Wake/observe/sweep are allowed; claim/own is not.

## Examples (canonical authorities in AGH)

| Transition                    | Authority                            | Allowed peers                                                  |
| ----------------------------- | ------------------------------------ | -------------------------------------------------------------- |
| Claim a `task_run`            | `task.Service.ClaimNextRun`          | `internal/scheduler` may wake idle agents; never claims itself |
| Spawn a child session         | Daemon-managed safe-spawn API        | Coordinator submits requests; the daemon decides               |
| Apply a schema migration      | `internal/store` migrations registry | `EnsureSchema` is forbidden for column changes                 |
| Mutate session terminal state | Session manager                      | Channels, hooks, observability emit events but cannot mutate   |
| Approve / publish a task      | Operator + manual API surface        | Coordinators receive enqueues; do not auto-approve             |

## Operationalization

- **Scheduler can wake and sweep, but cannot claim.** `internal/scheduler` issues `scheduler.wake.count`/`no_match`/`lease_sweep.count`/`error` metrics — never `task.run.claim.success`.
- **Hooks can deny/narrow/annotate but cannot bypass safety invariants** (claim tokens, leases, TTL, lineage, spawn caps, permission narrowing).
- **Coordination channels are NEVER an ownership/status authority.** Channel `status`/`result` messages cannot mutate ownership/terminal state.

## Anti-pattern

- Adding a "scheduler.\*" hook taxonomy that lets external code claim runs.
- Letting the coordinator bypass `ClaimNextRun` through a "fast-path" for trusted runs.
- Allowing the network layer to write terminal state via channel messages.

## Source

Analysis corpus: docs/\_memory/analysis/analysis_global_runs.md lesson L4.

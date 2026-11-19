# L-042 — Ephemeral worktrees need reviewed, pushed checkpoints before heavy validation

**Class:** Workflow / Recovery
**Date discovered:** 2026-11-19, during draft PR [#1849](https://github.com/compozy/compozy/pull/1849)
**Evidence sources:** [PR #1849](https://github.com/compozy/compozy/pull/1849) at `69e4c805` (plan-only commit); observations reported in the 2026-11-19 implementation session; follow-up inspection of the replacement workspace.

## Incident

The first implementation attempt for #1848 ran in a linked worktree at
`compozy-sparse/.worktrees/daily-spend-guardrails`, under a temporary
executor. The session reported uncommitted changes to
Go runtime, SQLite, API, CLI, Web, SDK, docs, and tests while an independent
review and test execution were in progress. The reported scope and compilation
timeline are session-reported observations, not independently recoverable
artifacts.

The session observed concurrent Go test compilations, a compiler returning
`signal: killed`, and then an executor transport disconnection. A replacement
workspace no longer contained that worktree. A remote check confirmed that
the PR still pointed at `69e4c805`, the original plan-only commit, and no
recoverable implementation was found on the checked refs. The compiler kill
is consistent with resource pressure; it does not prove why the executor reset.

The confirmed recovery gap is narrower and independent of that uncertainty:
the uncommitted implementation was not backed by a pushed Git checkpoint
when its temporary workspace became unavailable.

## Rule

> When implementation lives in a temporary executor, use small commits and,
> where publication is authorized, push them to a recoverable branch before
> launching a potentially destabilizing test lane. Do not accumulate an
> entire multi-surface feature solely in an uncommitted worktree.

Checkpoint review and publication follow the owning workflow and operator
requirements. A checkpoint is not a delivery claim: unverified or blocked
tests remain explicitly pending.

## Application to #1849

1. Work on the existing fork branch in a dedicated Git worktree, leaving the
   checkout used to manage it clean.
2. Split work into narrowly reviewable changes, each with focused tests and
   focused lint, independent Astra 6.2 review **before each commit**, and
   push to the same authorized **draft** branch.
3. Serialize memory-heavy Go compilation and race/E2E lanes; do not run
   concurrent Go package compilations in the memory-constrained executor.
   Use `go test -p 1` for local focused checks. CI remains responsible for
   repository-wide gates; requested local race/E2E checks run separately.
4. Keep explicit verification evidence with the current commit or PR body.
   Before claiming completion, confirm the pushed SHA and all required check
   conclusions on that exact SHA.

This supplements [L-030](L-030-dual-verify-capacity-collapse.md) on local
capacity collapse and [L-037](L-037-vertical-slices-gates-travel-with-the-slice.md)
on verification traveling with each slice. It does not relax any existing
quality gate or authorize a PR to leave draft early.

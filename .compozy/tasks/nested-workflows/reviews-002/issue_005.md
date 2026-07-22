---
provider: manual
pr:
round: 2
round_created_at: 2026-07-22T15:39:03Z
status: resolved
file: skills/cy-create-techspec/references/techspec-template.md
line: 51
severity: high
author: claude-code
provider_ref:
---

# Issue 005: API changes can publish without a rollout strategy

## Review Comment

The TechSpec impact table records affected components and a free-form action, but it does not require contract-diff analysis, enumerate active consumers, or select a rollout strategy. A backend task can therefore change an existing API while consumer updates remain outside its task or dependency graph.

When an existing contract changes, derive known consumers from repository analysis and require one explicit strategy: atomic consumer updates, backward compatibility, versioning, content negotiation, feature flag, or temporary adapter. Consumer verification must belong to the same task or a coordinated dependency. Any temporary compatibility behavior needs an owner, cleanup condition, and removal task. Block generation whenever an active consumer exists and no rollout strategy is defined.

## Triage

- Decision: `VALID`
- Notes: `cy-create-techspec` delegates Impact Analysis requirements to this
  template, but the current table captures only a component, impact type,
  free-form risk description, and action. It never requires the contract delta,
  repository-derived active consumers, or a coordinated rollout strategy.
  `cy-create-tasks` can preserve declared dependencies, but it cannot infer
  consumer work omitted from the TechSpec, so an existing API contract can be
  changed without consumer updates in the task graph. Add an explicit contract
  change analysis that inventories active consumers, requires one supported
  rollout strategy, assigns consumer verification to the same task or a declared
  dependency, records cleanup ownership for temporary compatibility behavior,
  and blocks TechSpec generation when an active consumer lacks a strategy. Add a
  focused bundle test that protects these required template instructions.
- Verification: The focused regression test failed against the old template and
  passes with the fix. The first in-place `make verify` run reached Playwright
  after all earlier checks passed, then failed because this review worktree's
  absolute path exceeds Darwin's Unix-socket path limit (`bind: invalid
  argument`). Re-running the same source from a short physical path passed the
  full pipeline: 5,187 Go tests with 7 intentional skips, zero lint issues,
  extension verification, build, and all 7 Playwright tests.

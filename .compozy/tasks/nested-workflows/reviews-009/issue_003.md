---
provider: manual
pr:
round: 9
round_created_at: 2026-07-24T20:14:29Z
status: resolved
file: internal/core/worktree/review_isolation.go
line: 1302
severity: medium
author: claude-code
provider_ref:
---

# Issue 003: Artifact overlays retain files deleted from the live workflow

## Review Comment

`OverlayTree` walks only the source tree and creates or truncates entries that
still exist there. It never removes entries present only in the destination.
The new task-group launch path calls it over both the complete initiative tree
and the operational adapter
(`internal/daemon/task_multi_artifacts.go:75,86`), even though an allocated Git
worktree can already contain tracked task artifacts from its base commit.

If the live workspace deleted or renamed an artifact after that base commit,
the destination's old copy survives the overlay. A child can consequently see
and execute a deleted `task_NN.md`, consume a stale specification or ADR, or
pick up obsolete review/memory state. The behavior contradicts the callers'
mirror semantics and is especially dangerous for group runs because the whole
initiative tree is copied as execution context.

Make the validated destination an exact mirror: populate a clean temporary
directory and replace it safely, remove the destination before copying when its
containment and ownership are proven, or explicitly delete destination entries
that are absent from the source. Preserve the current symlink and special-file
guards. Add a regression test whose destination contains a stale
`task_02.md` absent from the source and assert that it is gone after mirroring.

## Triage

- Decision: `VALID`
- Notes: `OverlayTree` walks only `source`, so it has no branch that observes or
  removes destination-only paths. The task-group launch callers use this
  operation as a mirror over worktrees that can contain tracked artifacts from
  the base commit, allowing deleted or renamed workflow files to survive.
  Build the replacement tree in a clean sibling temporary directory, preserving
  source symlink and special-file rejection, then swap the validated tree into
  place with rollback if installation fails. Add a regression test proving a
  stale destination-only `task_02.md` is removed, plus a rejection case proving
  invalid source entries do not destroy the existing destination.
- Verification blocker: `make verify` reaches Go lint after all frontend checks
  and formatting pass, then fails on pre-existing
  `internal/cli/daemon_commands.go:1958` because
  `resolveTaskPresentationMode` has cyclomatic complexity 16 (limit 15). That
  file is unchanged from `HEAD`; scoped worktree lint reports zero issues.
  Running the remaining Go race suite separately also encounters pre-existing
  run databases under `~/.compozy/runs` with schema version 4 while this branch
  expects version 3. The changed worktree package passes all 38 race tests.

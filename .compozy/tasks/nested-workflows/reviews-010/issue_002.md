---
provider: manual
pr:
round: 10
round_created_at: 2026-07-25T01:52:47Z
status: resolved
file: internal/core/worktree/review_isolation.go
line: 660
severity: critical
author: claude-code
provider_ref:
---

# Issue 002: Three-way merge follows symlinks outside the workspace

## Review Comment

The three-way fallback validates patched names with `safeWorkspacePath`, but
that helper only rejects lexical `..` escapes. `readFileIfExists` then uses
`os.Stat` and `os.ReadFile`, and `mergeSourcePathThreeWay` passes `oursAbs`
directly to `git merge-file` before calling `os.Chmod`. All of those operations
follow symlinks.

This fallback is reachable when strict patch application fails after a prior
batch or concurrent edit drifted a shared path. If that path, or one of its
parent components, is now a symlink, the merge reads and rewrites its target
outside the source workspace. A first batch can also turn a shared regular file
into a symlink and make a later overlapping batch enter this path. Snapshot and
rollback code explicitly supports symlinks, so the path is not rejected earlier.
The result is an out-of-workspace arbitrary file read/write and mode change,
followed by rollback logic that may remove and recreate only the link while the
external target remains modified.

Use `os.Lstat` and reject symlinks and non-regular components before any merge
input is opened, or merge blob contents only in private temporary regular files
and install the result with a no-follow, root-contained operation. Validate
every path component, not only the final entry. Add a regression where the
source path drifts to a symlink before fallback and assert that integration
fails without reading, changing, or chmodding the external target.

## Triage

- Decision: `VALID`
- Notes: `safeWorkspacePath` provides lexical containment only. During drift
  detection and three-way fallback, `sourcePathMatchesBaseline`,
  `batchAlreadyIntegrated`, and `readFileIfExists` can follow a symlinked path
  component. `mergeSourcePathThreeWay` then passes the source and isolated
  paths directly to `git merge-file`, which rewrites the source operand in
  place, and follows with `os.Chmod`; both operations can therefore modify an
  out-of-workspace target. The fix rejects symlink and non-directory parent
  components through rooted filesystem handles, merges only private regular
  temp files, and atomically installs clean results through the source-root
  handle. Regression coverage replaces both the final source path and a parent
  component with symlinks before fallback and verifies integration fails while
  external content and mode remain unchanged. The complete `make verify` gate
  passed after the production fix and regression tests. Full-repository lint
  also exposed
  a pre-existing cyclomatic-complexity violation in
  `internal/cli/daemon_commands.go`; the minimum behavior-preserving extraction
  is included outside the listed code file because the mandatory `make verify`
  gate cannot pass without it.

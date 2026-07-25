---
provider: manual
pr:
round: 11
round_created_at: 2026-07-25T03:30:01Z
status: resolved
file: internal/daemon/task_multi.go
line: 2835
severity: critical
author: claude-code
provider_ref:
---

# Issue 001: Artifact reconciliation follows a symlinked task-group root

## Review Comment

`taskMultiGroupArtifactSyncPaths` proves only that the child operational path
string equals the expected remapped path. It then calls `requireDirectory`,
which uses `os.Stat` and follows a symlink. `captureTaskMultiArtifactTree`
subsequently calls `os.OpenRoot(root)`; `os.OpenRoot` also follows symbolic
links in the root name. The `WalkDir` symlink guard therefore sees the target
directory as `.` and never observes that the task-group root itself is a
symlink.

A completed child can remove its operational directory and replace it with a
symlink to a sibling task group's directory or another accessible directory.
When the canonical parent is unchanged, the three-way merge treats that entire
target tree as the child's edits. The CAS installer then replaces the selected
canonical task group's tasks, reviews, and memory with those unrelated files,
reports the child successful, and removes its worktree. Because these workflow
artifacts are gitignored, the overwritten state may be unrecoverable.

Open the operational directory relative to a trusted child-workspace root,
reject a symlink in the final component, and capture through the verified
directory handle. Verify that the opened directory has the same identity as the
expected remapped path before reconciliation; apply the same rooted identity
check to the canonical destination before rename/install. Add an integration
test that replaces the child operational root with a symlink to a sibling tree
and asserts synchronization fails, preserves the worktree, and leaves both
canonical task groups byte-for-byte unchanged.

## Triage

- Decision: `VALID`
- Notes: `taskMultiGroupArtifactSyncPaths` compares only cleaned path strings and
  then calls `requireDirectory`, whose `os.Stat` follows the final symlink.
  `captureTaskMultiArtifactTree` subsequently calls `os.OpenRoot` with that same
  path, which also follows a symlink in the root name. The walk therefore treats
  the symlink target as the artifact-tree root and cannot detect the substituted
  final component. The canonical CAS path has the same identity gap between
  capture and rename. Fix by opening each task-group root relative to its trusted
  workspace root, rejecting a symlink in the operational directory's final
  component, comparing the opened directory identity with the rooted expected
  path, capturing through that verified handle, and requiring the same verified
  canonical identity before CAS rename/install. Add an integration regression
  test that substitutes a sibling task-group symlink and proves failure,
  worktree preservation, and byte-for-byte preservation of both canonical
  groups.

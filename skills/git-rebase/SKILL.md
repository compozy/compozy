---
name: git-rebase
description: Resolve Git merge and rebase conflicts conservatively, preserving both sides' intent, staging only understood resolutions, and leaving a git-clean conflict state.
---

# Git Rebase Conflict Resolution

Use this skill when Compozy asks you to resolve conflicts in an integration
worktree. The goal is a clean merge result, not a clever shortcut.

## Core Rules

1. Understand every conflicted hunk before editing it.
2. Preserve important behavior from both sides whenever possible.
3. Prefer the smallest readable merge that keeps the code idiomatic for the
   affected language and project.
4. Do not delete tests, weaken assertions, suppress lint, swallow errors, or
   otherwise hide a failing invariant.
5. Do not commit. Compozy owns the final squash commit.
6. Do not leave conflict markers in any file.
7. If a conflict is unsafe or unclear, leave it unresolved so Compozy can abort
   and roll back honestly.

## Required Workflow

1. Inspect the conflicted files listed in the prompt.
2. For each hunk, identify what the integration branch changed and what the
   incoming task changed.
3. Edit the file so both sides' required behavior is represented.
4. Run only language-specific formatting commands that are clearly required for
   the files you edited and are safe for this repository.
5. Stage resolved files with `git add`.
6. Check `git status --porcelain`; no unmerged entries may remain.
7. Report what was resolved and any files that remain unsafe.

## Resolution Guidance

- For Go files, keep error wrapping with `fmt.Errorf("context: %w", err)`.
- For Go files, preserve `context.Context` propagation and cancellation behavior.
- Preserve synchronization ownership; do not introduce unmanaged background
  work.
- Keep tests focused on behavior and invariants, not implementation details.
- When both sides add cases to a table test, combine the cases unless they prove
  the same invariant twice.
- When both sides alter an interface, update every implementation instead of
  guessing from the conflicted file alone.

## Fail-Honestly Criteria

Stop and leave the conflict unresolved when:

- you cannot tell which side owns the invariant,
- resolving would require deleting behavior from either side without evidence,
- conflict markers remain, or
- a binary/generated file conflict cannot be validated safely.

Compozy will roll back the integration branch when resolution is exhausted, so
an honest unresolved conflict is safer than a speculative broken merge.

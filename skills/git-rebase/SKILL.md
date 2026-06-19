---
name: git-rebase
description: Resolve Git merge and rebase conflicts for Go projects conservatively, preserving both sides' intent, staging only understood resolutions, and validating with make verify.
---

# Git Rebase Conflict Resolution for Go Projects

Use this skill when Compozy asks you to resolve conflicts in an integration
worktree. The goal is a clean, building merge result, not a clever shortcut.

## Core Rules

1. Understand every conflicted hunk before editing it.
2. Preserve important behavior from both sides whenever possible.
3. Prefer the smallest readable merge that keeps the code idiomatic Go.
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
4. Run `gofmt` where Go files changed.
5. Stage resolved files with `git add`.
6. Check `git status --porcelain`; no unmerged entries may remain.
7. Run `make verify`.
8. Report what was resolved and any files that remain unsafe.

## Go Resolution Guidance

- Keep error wrapping with `fmt.Errorf("context: %w", err)`.
- Preserve `context.Context` propagation and cancellation behavior.
- Preserve synchronization ownership; do not introduce fire-and-forget
  goroutines.
- Keep tests focused on behavior and invariants, not implementation details.
- When both sides add cases to a table test, combine the cases unless they prove
  the same invariant twice.
- When both sides alter an interface, update every implementation and compile
  with `make verify` instead of guessing.

## Fail-Honestly Criteria

Stop and leave the conflict unresolved when:

- you cannot tell which side owns the invariant,
- resolving would require deleting behavior from either side without evidence,
- the resulting code does not pass `make verify`,
- conflict markers remain, or
- a binary/generated file conflict cannot be validated safely.

Compozy will roll back the integration branch when resolution is exhausted, so
an honest unresolved conflict is safer than a speculative broken merge.

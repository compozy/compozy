---
name: git-rebase
description: "Rebase branches and resolve cross-commit conflicts while preserving changes. Excludes merge-commit workflows, cherry-picking, and repository setup."
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---

# Git Rebase

Carry out the requested rebase while preserving both sides' intended behavior and unrelated work. A rebase request does not by itself authorize squashing, dropping commits, changing global Git settings, or publishing a rewritten branch.

1. Inspect status, current branch, target, and any in-progress rebase. Reuse a known target and current fetch evidence. Preserve dirty work; do not auto-stash, reset, checkout, clean, or stage unrelated paths. Use an isolated checkout when needed.
2. Before starting a new rebase, record HEAD and keep a backup ref when rewriting valuable history. `bash <skill-dir>/scripts/pre-rebase-backup.sh` creates a backup for a clean branch without modifying tracked files. An existing verified backup needs no duplicate.
3. Rebase onto the requested target directly by default. Use interactive/squash/reorder operations only when the user requested that history change; commit count is not a reason to squash. Consult `references/strategies.md` for an actual strategy decision.
4. When conflicts occur, inspect the replayed commit, base, and both sides. During rebase, `ours` is the target plus commits already replayed; `theirs` is the commit being replayed. Trace changed APIs and preserve intended security, state, and compatibility contracts rather than accepting one side wholesale.
5. Resolve the affected files, inspect their diff, stage only those resolved paths, then continue. Reuse valid observations for repeated conflicts. Do not add merge-history comments to production code or run a full suite after every file.
6. At the completed rebase, inspect the resulting patch series/diff and run checks for affected behavior plus the project's required delivery gate. Tests that expose a regression require a production fix, not weaker assertions.
7. Publish only when already authorized. Use `--force-with-lease` for a rewritten remote branch, and follow the project's current-head CI policy. If the lease fails, inspect the new remote commits; do not replace it with unconditional force.

Use `references/resolution-patterns.md` for semantic conflicts and `references/troubleshooting.md` for diagnosed operational failures. Helpers live under this skill's actual `scripts/` path; `analyze-conflicts.sh`/`validate-merge.sh` are optional aids after checking their repository assumptions. They do not replace the owning tests or gate. Recovery that discards work still needs the repository's explicit permission; a backup ref is evidence, not permission to reset.

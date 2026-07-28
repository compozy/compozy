---
name: coderabbit-review-commit
description: Review exactly one commit with the CodeRabbit CLI in an isolated throwaway worktree.
disable-model-invocation: true
---

# CodeRabbit Review of a Single Commit

Review **exactly one commit** — including a non-tip one — with the CodeRabbit CLI. The CLI only diffs `base → HEAD`, so this skill pins the target commit in a **throwaway git worktree** (detached `HEAD = C`) and reviews `base = C~1 → HEAD = C`. The current branch and working tree are never touched; the worktree is force-removed on every exit path.

## Procedure

**Step 1: Resolve the target commit**
1. Default to `HEAD~1` (the penultimate commit) unless the user names a commit-ish or SHA.
2. Confirm it resolves: `git rev-parse --short <target>`.

*Done when:* a concrete short SHA is in hand.

**Step 2: Run the review in the background**
1. Execute the bundled helper (**mutating:** creates a temp worktree + log file; never alters the branch), backgrounded — the review takes minutes:
   `bash <coderabbit-review-commit-dir>/scripts/review-commit.sh <target>`
2. The helper tees to `<repo>/.coderabbit/reviews/<short-sha>.log` and cleans its worktree on exit.
3. Override with env only when needed: `CR_REVIEW_OUT` (log dir), `CR_CONFIG` (config file), `CR_REVIEW_TYPE` (default `committed`).

*Done when:* the helper is running and its log path is reported.

**Step 3: Report the outcome**
1. On completion, surface the log path and the CodeRabbit findings.

*Done when:* the user has the findings and the log location.

## Error Handling
- `coderabbit` not in PATH → helper exits 127; the user must install and auth the CLI (`coderabbit auth login`).
- Target is a root commit (no parent) → helper exits 1; there is no base to diff.
- `Too many files!` (commit exceeds CodeRabbit's 300-file cap) → review a subdirectory (`coderabbit review --dir <path>`) or split the commit.

---
name: cy-fix-reviews
description: Executes provider-agnostic PR review remediation using existing review round files under .compozy/tasks/<name>/reviews-NNN/. Use when resolving batched review issues, updating issue/grouped markdown files, implementing fixes, and verifying the result. Do not use for PRD task execution, review export/fetch, or generic coding tasks without review issue files.
---

# Fix Reviews

Execute the review remediation workflow in a strict sequence. The review files already exist and define the full scope for the run.

## Required Inputs

- The scoped issue files listed in `<batch_issue_files>`.
- The PRD review round directory and `_meta.md`.
- The repository verification workflow required by `cy-final-verify`.

## Workflow

1. Read and triage the scoped issue files.
   - Read every listed issue file completely before editing code.
   - Update each issue file frontmatter `status` from `pending` to `valid` or `invalid`.
   - Record concrete technical reasoning in `## Triage`.

2. Fix valid issues completely.
   - Implement production-quality fixes for every `valid` issue in scope.
   - Add or update tests when behavior changes or regressions are possible.
   - Keep the scope constrained to the listed issue files and their necessary code changes.

3. Close out issue files correctly.
   - For a `valid` issue, set frontmatter `status: resolved` only after the code and verification are done.
   - For an `invalid` issue, document why it is invalid and then set frontmatter `status: resolved` once the analysis is complete.
   - Update grouped review trackers only when the prompt explicitly enables them.

4. Verify before completion.
   - Use `cy-final-verify` before any completion claim or automatic commit.
   - Run the repository’s real verification commands; do not stop at partial checks.
   - Leave the diff ready for manual review unless the prompt explicitly enables auto-commit.

## Critical Rules

- Do not fetch or export reviews inside this workflow. `fetch-reviews` already produced the round files.
- Do not call provider-specific scripts or `gh` mutations. Compozy resolves provider threads after the batch succeeds.
- Do not modify issue files outside the scoped batch.
- Do not mark an issue `resolved` before the underlying work and verification are actually complete.

---
name: cy-fix-reviews
description: Executes provider-agnostic PR review remediation using existing review round files under .compozy/tasks/<name>/reviews-NNN/. Use when resolving batched review issues, updating issue markdown files, implementing fixes, and verifying the result. Do not use for PRD task execution, review export/fetch, or generic coding tasks without review issue files.
---

# Fix Reviews

Execute the review remediation workflow in a strict sequence. The review files already exist and define the full scope for the run.

## Required Inputs

- The scoped issue files listed in `<batch_issue_files>`.
- The PRD review round directory and issue-file frontmatter.
- The repository verification workflow required by `cy-final-verify`.

## Workflow

1. Gather round context.
   - Read the scoped issue file frontmatter to understand the provider, round number, and issue status/severity. If multiple issue files are in scope, verify their `provider`, `pr`, `round`, and `round_created_at` values agree.
   - Read `<batch_scope>` to identify the PRD name, review round, code files in scope, and conditional flags such as auto-commit.

2. Read and triage the scoped issue files.
   - Read every listed issue file completely before editing code.
   - Update each issue file frontmatter `status` from `pending` to `valid` or `invalid`.
   - Record concrete technical reasoning in `## Triage`: state why the issue is valid or invalid, identify the root cause if valid, and outline the intended fix approach.

3. Fix valid issues completely.
   - Fix issues in severity order: critical first, then high, medium, low. This ensures the most impactful fixes land even if the batch is interrupted.
   - Implement production-quality fixes for every `valid` issue in scope.
   - Add or update tests when behavior changes or regressions are possible. Test file edits are always in scope when they validate a fix.
   - Keep code changes constrained to the files listed in `<batch_scope>` code files. If a fix absolutely requires touching a file not listed there, limit the change to the minimum needed and document why in the issue file's `## Triage` section.
   - Do not refactor, clean up, or improve code that is unrelated to the issues being fixed.

4. Close out issue files correctly.
   - For a `valid` issue, set frontmatter `status: resolved` only after the code and verification are done.
   - For an `invalid` issue, document why it is invalid and then set frontmatter `status: resolved` once the analysis is complete.

5. Verify before completion.
   - Use `cy-final-verify` before any completion claim or automatic commit.
   - Run the repository’s real verification commands; do not stop at partial checks.
   - If verification fails, fix the failing checks in the code you changed. Do not revert your fixes to pass verification -- find the root cause of the failure and address it. If the failure is in pre-existing code unrelated to your changes, document it in the relevant issue file’s `## Triage` section and proceed. If two fixes conflict with each other and verification cannot pass after two attempts, document the conflict in both issue files and report the situation rather than looping indefinitely.
   - If all issues in the batch are invalid and no code was changed, skip the commit step entirely -- do not create an empty commit. Still run verification to confirm no regressions.
   - Leave the diff ready for manual review unless `<batch_scope>` shows "Automatic commits: enabled".

## Critical Rules

- Do not fetch or export reviews inside this workflow. `compozy reviews fetch` already produced the round files.
- Do not call provider-specific scripts or `gh` mutations. Compozy resolves provider threads after the batch succeeds.
- Do not modify issue files outside the scoped batch.
- Do not mark an issue `resolved` before the underlying work and verification are actually complete.

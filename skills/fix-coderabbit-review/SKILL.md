---
name: fix-coderabbit-review
description: Executes CodeRabbit PR review remediation using either an existing batch of exported issue files or a PR number that must be exported from GitHub first. Use when resolving CodeRabbit review comments, updating review tracking files, and resolving GitHub review threads. Do not use for generic bug fixing, PRD task execution, or review workflows that are not based on CodeRabbit issue markdown.
---

# Fix CodeRabbit Review

Execute the CodeRabbit remediation workflow in a strict sequence. Use existing exported issue files when the caller already supplied them. Otherwise, export the PR review first.

Run commands from the repository root.

## Required Inputs

- PR number.
- Either explicit batch issue files or permission to export the PR into `ai-docs/reviews-pr-<PR>/`.
- `gh` authenticated for the target repository.
- `python3`.

## Workflow

1. Determine execution mode.
   - If the caller provided explicit batch issue files, treat those files as the entire scope and skip export unless one of them is missing.
   - Otherwise, run `python3 scripts/export_coderabbit_review.py --pr <PR_NUMBER> --hide-resolved`.
   - Read `ai-docs/reviews-pr-<PR_NUMBER>/_summary.md` and the scoped issue files before changing code.

2. Triage each scoped issue.
   - Restate the requested change in technical terms.
   - Validate it against the current codebase and tests.
   - Record `VALID` or `INVALID` reasoning directly in the issue file.
   - Treat ambiguous or incorrect suggestions as `INVALID` and document the rationale instead of forcing a bad change.

3. Fix each valid issue completely.
   - Apply production-quality code changes.
   - Add or update tests for every valid issue.
   - Update the issue file status to `RESOLVED ✓` only after the fix and evidence are complete.
   - If the caller included grouped tracker files, update only the grouped files that correspond to touched code.

4. Verify before completion.
   - Use the installed `verification-before-completion` skill before any completion claim or commit.
   - Run the repository's real verification commands for the touched code and the final integrated diff.
   - Reject stale output and partial checks.

5. Resolve review threads.
   - When the caller provided a numeric batch range, run `bash scripts/resolve_pr_issues.sh --pr-dir ai-docs/reviews-pr-<PR_NUMBER> --from <START> --to <END>`.
   - When the caller provided explicit issue files without a clean range, resolve only the threads referenced by those files.
   - Re-open `_summary.md` and confirm the unresolved count reflects the current state.

6. Commit only when the caller or looper explicitly enabled it.
   - Create exactly one local commit after clean verification if automatic commit mode is enabled.
   - Never push automatically.
   - Keep tracking-only files out of the commit unless the caller explicitly requires them.

## Error Handling

- If `python3 scripts/export_coderabbit_review.py` fails, stop and report the exact `gh` or repository-context error instead of guessing.
- If a scoped issue file is missing, regenerate the export or stop and report the missing path before editing code.
- If a review thread cannot be resolved, keep the affected issue unresolved and report the exact thread failure.
- If an issue file has no thread ID, leave the resolution state unchanged and note the missing metadata in the report.

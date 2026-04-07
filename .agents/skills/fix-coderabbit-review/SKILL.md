---
name: fix-coderabbit-review
description: End-to-end remediation workflow for PR review feedback by PR number. Use when Codex must export CodeRabbit issues for a PR, fix every issue completely, commit all fixes in a single commit, and resolve GitHub review threads afterward.
---

# Fix CodeRabbit Review

Execute PR review remediation in a strict sequence: export issues, fix all issues, commit once, resolve threads.

Core principle: evaluate each review item technically before changing code. Do not apply suggestions blindly.

## Required Inputs

- PR number (integer).
- `GITHUB_TOKEN` in environment.
- `gh` authenticated for the target repository.

## Workflow

1. Export review issues for the PR.
2. Triage and validate every unresolved issue.
3. Fix every validated issue completely.
4. Commit all remediation changes together in one commit.
5. Resolve review threads and mark exported issues resolved.

Run commands from repository root.

## Step 1: Export CodeRabbit Issues

Use bundled exporter script:

```bash
PR_NUMBER=<pr-number>
uv run .claude/skills/fix-coderabbit-review/scripts/pr_review.py "$PR_NUMBER" --hide-resolved
```

Read generated files in:

- `ai-docs/reviews-pr-<PR_NUMBER>/_summary.md`
- `ai-docs/reviews-pr-<PR_NUMBER>/issues/*.md`

If `_summary.md` shows unresolved issues, continue until all are addressed in code.

## Step 2: Triage and Validate Issues

Review each unresolved issue file before implementation.

For each issue:

1. Restate the concrete technical change required.
2. Verify against current codebase behavior and tests.
3. Decide one of:
   - `VALID`: suggestion is correct and should be implemented.
   - `INVALID`: suggestion is incorrect or not actionable for this codebase.

Rules:

- If an issue is ambiguous, treat it as `INVALID`, document rationale and assumptions in the issue file, and continue execution.
- If an issue is `INVALID`, document technical rationale directly in the issue file and do not force a bad change.
- Prefer technical evidence over agreement language.

## Step 3: Fix Issues Completely

Process all unresolved issue files under `ai-docs/reviews-pr-<PR_NUMBER>/issues/`.

- Implement production-quality fixes in source code.
- Add or update tests so each fix is verified.
- Execute fixes one issue at a time and validate each before moving to the next.
- Do not leave partial fixes.
- Do not skip issues unless they are invalid; for invalid issues, keep rationale in the issue file.

After finishing all issue fixes, run full verification before commit:

```bash
pnpm run lint
pnpm run typecheck
pnpm run test
```

## Step 4: Commit All Fixes Together

Create exactly one remediation commit containing all related code, test, and issue-tracking changes.

```bash
git add -A
git commit -m "fix(repo): resolve PR #<PR_NUMBER> review issues"
```

Do not split fixes across multiple commits.

## Step 5: Resolve Review Threads

After the single commit is created, resolve exported issue threads.

Determine the issue range from issue files (example: `001` to `018`) and run:

```bash
uv run .claude/skills/fix-coderabbit-review/scripts/resolve_pr_issues.py \
  --pr-dir ai-docs/reviews-pr-<PR_NUMBER> \
  --from <first-issue-number> \
  --to <last-issue-number>
```

Re-open `ai-docs/reviews-pr-<PR_NUMBER>/_summary.md` and confirm unresolved count is `0`.

If thread resolution fails for specific IDs, keep those issues marked unresolved and report exact failures instead of claiming completion.

## Output Contract

Deliver:

- One commit containing all remediation changes.
- Per-issue technical disposition (`VALID` or `INVALID`) recorded during triage.
- Updated `ai-docs/reviews-pr-<PR_NUMBER>/issues/*.md` statuses.
- Updated `ai-docs/reviews-pr-<PR_NUMBER>/_summary.md` counts/checklist.
- Resolved review threads for addressed issues.

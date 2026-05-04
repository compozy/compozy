---
status: resolved
file: packages/ui/src/tokens.css
line: 172
severity: minor
author: coderabbitai[bot]
provider_ref: review:4148025019,nitpick_hash:5802fadfa48e
review_hash: 5802fadfa48e
source_review_id: "4148025019"
source_review_submitted_at: "2026-04-21T13:30:56Z"
---

# Issue 027: Remove .stylelintrc.json or fix it for Tailwind at-rules.
## Review Comment

The `.stylelintrc.json` config extends `stylelint-config-standard-scss`, which enables `at-rule-no-unknown` by default. This would flag `@theme` as unknown in IDEs with Stylelint extensions, despite being valid Tailwind syntax. Since Stylelint isn't installed or used in this project's CI pipeline (linting is handled by oxlint/oxfmt), either remove the unused config file or add `ignoreAtRules: ["theme"]` to suppress the warning for IDE users.

## Triage

- Decision: `invalid`
- Notes:
  - The review is stale relative to the current branch. There is no `.stylelintrc.json` or other `.stylelintrc*` file in the repository, and there is no stylelint task to remove or adjust.
  - Root cause of the comment: it assumes repository state that is not present in this worktree.
  - Resolution path: no code change is needed for this batch.

## Resolution

- Closed as invalid. No `.stylelintrc*` file exists in the current repository, so there is no in-scope config to remove or fix.
- Verification:
- `make verify`

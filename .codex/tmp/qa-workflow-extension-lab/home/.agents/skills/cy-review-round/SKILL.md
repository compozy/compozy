---
name: cy-review-round
description: Performs a comprehensive code review of a PRD implementation and generates a review round directory with issue files compatible with cy-fix-reviews. Use when reviewing implemented PRD tasks, creating a manual review round without an external provider, or performing a quality audit of code changes. Do not use for fetching reviews from external providers, fixing existing review issues, executing PRD tasks, or editing source code.
---

# Review Round

Perform a structured code review of a PRD implementation and produce a review round directory that the `cy-fix-reviews` workflow can process.

## Required Inputs

- Feature name identifying the `.compozy/tasks/<name>/` directory.
- Optional: specific files or directories to scope the review.

## Workflow

1. Determine the review round directory.
   - Derive the PRD directory from the feature name: `.compozy/tasks/<name>/`.
   - Verify the PRD directory exists. If it does not, stop and report the missing directory.
   - List existing `reviews-NNN/` subdirectories to determine the next round number. If none exist, use round 1.
   - If prior review rounds exist, read their issue files to build a list of already-known issues. The current round must only contain NEW issues not already tracked in prior rounds. Do not re-flag issues that are pending, valid, or resolved in earlier rounds.
   - Determine the review round directory path: `.compozy/tasks/<name>/reviews-NNN/` with the round number zero-padded to 3 digits. Do NOT create it yet — wait until step 4 confirms there are issues to write. This avoids leaving empty directories when the review finds no issues.

2. Identify the review scope.
   - Read `_prd.md`, `_techspec.md`, and `_tasks.md` from the PRD directory to understand what was implemented and why.
   - Read ADRs from `.compozy/tasks/<name>/adrs/` for architectural decision context.
   - If `_prd.md` and `_techspec.md` are both missing, warn that the review will lack requirements context but proceed with a code-quality-only review.
   - If the user provided specific files or directories, scope the review to those paths.
   - If no explicit scope was provided, run `git diff main...HEAD --name-only` to discover all files created or modified on the current branch. If the diff is empty or unhelpful, ask the user to specify files.
   - Spawn an Agent tool call to explore the identified files, their imports, and their dependencies to build a map of the implementation.

3. Perform the code review.
   - Read `references/review-criteria.md` for severity definitions and evaluation areas.
   - **Prioritize the review scope.** If the scope contains more than 15 files, triage before deep-reading: identify the core implementation files (new packages, new exported APIs, files with the most additions) and review those in full first. Review remaining files (tests, minor edits, config changes) for obvious issues only. This prevents shallow reviews spread across too many files.
   - Read every file in the prioritized scope completely before forming conclusions.
   - **Requirements validation**: If `_prd.md` or `_techspec.md` were available in step 2, cross-check the implementation against every stated requirement, acceptance criterion, and architectural decision. Flag any requirement that is missing, partially implemented, or implemented differently than specified. These are correctness issues — assign severity based on the gap's impact (critical if a core feature is missing, high if behavior deviates from spec, medium if an edge case from the spec is unhandled).
   - Evaluate each file against the nine evaluation areas: Security, Correctness, Concurrency, Performance and Scalability, Error Handling, Code Quality and Maintainability, Testing, Architecture, and Operations.
   - Identify issues in severity order: critical first, then high, medium, and low.
   - For each issue record: the file path relative to the repository root, the approximate line number, the severity level, a concise title (max 72 characters), and a detailed review comment describing the problem and a suggested fix.
   - **Deduplicate before writing.** If the same pattern (e.g., missing nil check, missing error wrap) appears in multiple files, create one issue for the most representative instance and list the other affected files in its Review Comment. Do not create N identical issues for N files exhibiting the same root cause. One issue per distinct problem, not per occurrence.
   - **Verify before flagging.** Before creating an issue, check whether the pattern is intentional: look for adjacent comments explaining the choice, ADR references, or test coverage that validates the behavior. If code looks suspicious but has a clear justification (e.g., `// nolint: intentionally ignoring close error on read-only file`), do not create an issue. Only flag patterns that are genuinely problematic, not merely unconventional.
   - Skip issues that linters or formatters already catch. Run `make lint` first to filter these out.
   - **Focus on signal, not volume.** Aim for fewer, higher-quality issues rather than an exhaustive list. If you find more than 20 issues, re-evaluate: keep all critical and high issues, but prune medium and low issues to only the most impactful. A review with 8 precise issues is more useful than one with 30 that includes marginal concerns.
   - Also note well-implemented aspects of the code. These observations inform the summary but do not produce issue files.
   - If no issues are found after a thorough review, report that the implementation looks clean and skip steps 4 through 6. Do not create the review round directory.

4. Generate issue files.
   - Create the review round directory determined in step 1.
   - Read `references/issue-template.md` for the canonical format.
   - For each issue identified in step 3, create an `issue_NNN.md` file in the review round directory.
   - Issue numbering starts at `001` and increments sequentially.
   - Each file must use this exact structure:

     ```
     ---
     provider: manual
     pr:
     round: <N>
     round_created_at: <UTC timestamp in RFC3339 format>
     status: pending
     file: path/to/file.go
     line: 42
     severity: high
     author: claude-code
     provider_ref:
     ---

     # Issue NNN: <title>

     ## Review Comment

     <detailed review body>

     ## Triage

     - Decision: `UNREVIEWED`
     - Notes:
     ```

   - The `<author>` field must be `claude-code`.
   - The `provider_ref` field must be empty.
   - The `provider` field must be `manual`.
   - The `pr` field is empty for manual reviews. If the user provides a PR number, include it.
   - The `round` field must match the directory number as an integer (not zero-padded).
   - The `round_created_at` field must use the same current UTC RFC3339 timestamp in every issue in this round.
   - The `severity` field must be exactly one of: `critical`, `high`, `medium`, `low`.

5. Summarize and present the review.
   - Print a summary listing:
     - **Merge recommendation**: If any critical or high issues exist, state "Needs fixes before merge" with the blocking issues. If only medium/low issues exist, state "Safe to merge with follow-ups." If no issues, state "Clean — ready to merge."
     - Total issues found, broken down by severity (critical, high, medium, low).
     - The review round directory path.
     - The full list of generated issue file names.
     - Well-implemented aspects observed during the review.
   - Suggest running `compozy reviews fix <name>` to process the review round.

6. Verify before completion.
   - Use installed `cy-final-verify` before claiming the review round is complete.
   - Read back each generated issue file and verify the frontmatter parses correctly.
   - Verify every issue file in the round has matching `provider`, `pr`, `round`, and `round_created_at` values.
   - Confirm the review round directory follows the `reviews-NNN` naming convention.

## Critical Rules

- Do not fix the issues found. This skill only identifies and documents issues. The `cy-fix-reviews` workflow handles remediation.
- Do not create issue files for problems that linters or formatters already catch.
- Every issue file must have valid YAML frontmatter parseable by `prompt.ParseReviewContext()`.
- Do not create or maintain review `_meta.md`; round metadata lives in each issue file frontmatter.
- Do not create empty review rounds. If no issues are found, report a clean review and do not create the round directory.
- Do not modify any source code files. This is a review-only skill.
- Do not call provider-specific scripts or `gh` mutations.

## Error Handling

- If the PRD directory does not exist, stop and report the missing directory.
- If no files can be identified for review and the user did not provide explicit paths, ask the user to specify files.
- If both `_prd.md` and `_techspec.md` are missing, warn about the lack of requirements context but proceed with code-quality-only review.
- If the review round directory cannot be created, stop and report the filesystem error.
- If writing an issue file fails, stop and report which file could not be written.
- If `make lint` fails to run (build errors, missing tools), note the failure in the summary and proceed with the review. Do not skip the review because linting failed — just acknowledge that linter-overlap filtering could not be applied.

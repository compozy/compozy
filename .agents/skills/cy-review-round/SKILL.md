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
   - Create the review round directory: `.compozy/tasks/<name>/reviews-NNN/` with the round number zero-padded to 3 digits.

2. Identify the review scope.
   - Read `_prd.md`, `_techspec.md`, and `_tasks.md` from the PRD directory to understand what was implemented and why.
   - Read ADRs from `.compozy/tasks/<name>/adrs/` for architectural decision context.
   - If `_prd.md` and `_techspec.md` are both missing, warn that the review will lack requirements context but proceed with a code-quality-only review.
   - If the user provided specific files or directories, scope the review to those paths.
   - If no explicit scope was provided, run `git diff main...HEAD --name-only` to discover all files created or modified on the current branch. If the diff is empty or unhelpful, ask the user to specify files.
   - Spawn an Agent tool call to explore the identified files, their imports, and their dependencies to build a map of the implementation.

3. Perform the code review.
   - Read `references/review-criteria.md` for severity definitions and evaluation areas.
   - Read every file in scope completely before forming conclusions.
   - Evaluate each file against the nine evaluation areas: Security, Correctness, Concurrency, Performance and Scalability, Error Handling, Code Quality and Maintainability, Testing, Architecture, and Operations.
   - Identify issues in severity order: critical first, then high, medium, and low.
   - For each issue record: the file path relative to the repository root, the approximate line number, the severity level, a concise title (max 72 characters), and a detailed review comment describing the problem and a suggested fix.
   - Skip issues that linters or formatters already catch. Run `make lint` first to filter these out.
   - Also note well-implemented aspects of the code. These observations inform the summary but do not produce issue files.
   - If no issues are found after a thorough review, report that the implementation looks clean and skip steps 4 through 6. Do not create the review round directory.

4. Generate issue files.
   - Read `references/issue-template.md` for the canonical format.
   - For each issue identified in step 3, create an `issue_NNN.md` file in the review round directory.
   - Issue numbering starts at `001` and increments sequentially.
   - Each file must use this exact structure:

     ```
     ---
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
   - The `severity` field must be exactly one of: `critical`, `high`, `medium`, `low`.

5. Generate the round metadata file.
   - Write `_meta.md` in the review round directory with this format:

     ```
     ---
     provider: manual
     pr:
     round: <N>
     created_at: <UTC timestamp in RFC3339 format>
     ---

     ## Summary
     - Total: <number of issue files>
     - Resolved: 0
     - Unresolved: <same as Total>
     ```

   - The `provider` field must be `manual`.
   - The `pr` field is empty for manual reviews. If the user provides a PR number, include it.
   - The `round` field must match the directory number as an integer (not zero-padded).
   - The `created_at` field must use the current UTC time in RFC3339 format.
   - The counts must accurately reflect the number of issue files written.

6. Summarize and present the review.
   - Print a summary listing:
     - Total issues found, broken down by severity (critical, high, medium, low).
     - The review round directory path.
     - The full list of generated issue file names.
     - Well-implemented aspects observed during the review.
   - Suggest running `compozy fix-reviews --name <name>` to process the review round.

7. Verify before completion.
   - Use installed `cy-final-verify` before claiming the review round is complete.
   - Read back each generated issue file and verify the frontmatter parses correctly.
   - Verify `_meta.md` has correct counts matching the actual number of issue files.
   - Confirm the review round directory follows the `reviews-NNN` naming convention.

## Critical Rules

- Do not fix the issues found. This skill only identifies and documents issues. The `cy-fix-reviews` workflow handles remediation.
- Do not create issue files for problems that linters or formatters already catch.
- Every issue file must have valid YAML frontmatter parseable by `prompt.ParseReviewContext()`.
- The `_meta.md` must be parseable by `reviews.ReadRoundMeta()`.
- Do not create empty review rounds. If no issues are found, report a clean review and do not create the round directory.
- Do not modify any source code files. This is a review-only skill.
- Do not call provider-specific scripts or `gh` mutations.

## Error Handling

- If the PRD directory does not exist, stop and report the missing directory.
- If no files can be identified for review and the user did not provide explicit paths, ask the user to specify files.
- If both `_prd.md` and `_techspec.md` are missing, warn about the lack of requirements context but proceed with code-quality-only review.
- If the review round directory cannot be created, stop and report the filesystem error.
- If writing an issue file fails, stop and report which file could not be written.

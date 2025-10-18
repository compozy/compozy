# Transform Review Issues

This directory contains scripts to transform ai-docs/reviews/\* files into the format expected by `solve_issues.go` (the same format that `pr-review.ts` generates).

## Problem

The ai-docs/reviews files have a different format than what `solve_issues.go` expects:

**Current format (ai-docs/reviews):**

```markdown
---
title: "Issue Title"
status: "pending"
---

**Location:** `engine/path/to/file.go`
```

**Expected format (pr-review.ts output):**

```markdown
# Issue 1 - Review Thread Comment

**File:** `engine/path/to/file.go:1`
**Date:** 2025-01-18 10:30:00 UTC
**Status:** - [ ] UNRESOLVED

## Body

...
```

## Solution

Use `transform-review-issues.ts` to convert the files:

### Usage

```bash
# Transform monitoring issues
bun scripts/transform-review-issues.ts monitoring

# Transform performance issues
bun scripts/transform-review-issues.ts performance

# Specify custom output directory
bun scripts/transform-review-issues.ts monitoring --output-dir ai-docs/reviews-pr-monitoring/issues
```

### What it does

1. Reads all `.md` files from `ai-docs/reviews/<category>/`
2. Extracts:
   - Code file path from `**Location:**` field
   - Title, priority, status from YAML frontmatter
   - Full body content
3. Transforms to pr-review.ts format with:
   - `**File:** \`path:line\`` format (required by solve_issues.go)
   - `**Status:** - [ ] UNRESOLVED` format (required by solve_issues.go)
   - Descriptive filenames: `001-runtime-bun_manager-runtime-tool-execution-metrics.md`
     - Format: `{number}-{file-slug}-{title-slug}.md`
     - Makes issues easy to identify without opening them
4. Creates `_summary.md` with issue listing
5. Outputs to `ai-docs/reviews-pr-manual/issues/`

### After transformation

Process the transformed issues with solve_issues.go:

```bash
# Single file at a time
go run scripts/issues/solve_issues.go --pr manual --issues-dir ai-docs/reviews-pr-manual/issues

# Batch processing (3 files per batch, 2 batches in parallel)
go run scripts/issues/solve_issues.go \
  --pr manual \
  --issues-dir ai-docs/reviews-pr-manual/issues \
  --batch-size 3 \
  --concurrent 2

# Use Claude instead of Codex
go run scripts/issues/solve_issues.go \
  --pr manual \
  --issues-dir ai-docs/reviews-pr-manual/issues \
  --ide claude

# Specify model
go run scripts/issues/solve_issues.go \
  --pr manual \
  --issues-dir ai-docs/reviews-pr-manual/issues \
  --ide codex \
  --model gpt-5

# Dry run (generate prompts only, don't execute)
go run scripts/issues/solve_issues.go \
  --pr manual \
  --issues-dir ai-docs/reviews-pr-manual/issues \
  --dry-run
```

## Example Workflow

```bash
# 1. Transform monitoring issues
bun scripts/transform-review-issues.ts monitoring

# 2. Review the transformed files
ls -la ai-docs/reviews-pr-manual/issues/

# 3. Process with solve_issues.go (dry run first)
go run scripts/issues/solve_issues.go \
  --pr manual \
  --issues-dir ai-docs/reviews-pr-manual/issues \
  --dry-run

# 4. Review generated prompts
ls -la .tmp/codex-prompts/pr-manual/

# 5. Run for real with batching
go run scripts/issues/solve_issues.go \
  --pr manual \
  --issues-dir ai-docs/reviews-pr-manual/issues \
  --batch-size 3 \
  --concurrent 2 \
  --ide codex \
  --model gpt-5
```

## Key Format Requirements

The `solve_issues.go` script expects:

1. **File path with line number**: `**File:** \`path/to/file.go:123\``
   - Must be in backticks
   - Must include `:line` (can be `:1` for generic issues)
   - This is how files are grouped for batching

2. **Status format**: `**Status:** - [ ] UNRESOLVED` or `**Status:** - [x] RESOLVED ✓`
   - Used to filter already resolved issues
   - Checked with regex patterns

3. **Numbered filenames**: `001-issue.md`, `002-issue.md`, etc.
   - Sequential numbering for deterministic ordering

4. **\_summary.md file**: Lists all issues with checkboxes
   - Updated by solve_issues.go as issues are resolved

## Categories

Current categories in ai-docs/reviews/:

- `monitoring` - 26 files (metrics, observability)
- `performance` - 16 files (optimizations, bottlenecks)

## Notes

- The script skips `refactoring.md` files
- If no `**Location:**` field is found, defaults to `unknown_file.go`
- All issues start as UNRESOLVED (status will be updated by solve_issues.go)
- Line numbers default to `:1` since the original reviews don't specify exact lines
- The transformed format is compatible with both `codex` and `claude` IDE tools

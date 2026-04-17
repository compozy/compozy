---
status: resolved
file: internal/cli/task_runtime_form.go
line: 364
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4130406042,nitpick_hash:07a3304ab632
review_hash: 07a3304ab632
source_review_id: "4130406042"
source_review_submitted_at: "2026-04-17T16:20:53Z"
---

# Issue 005: Redundant trimming logic in formatBaseTaskRuntime.
## Review Comment

Lines 368-374 contain redundant operations: `label` is already set from `state.ide` and trimmed, then checked and re-trimmed unnecessarily.

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. `formatBaseTaskRuntime` trims `state.ide` multiple times before appending it to the display parts.
  - Root cause: the label normalization was written in a defensive style that now repeats equivalent whitespace handling.
  - Intended fix: collapse the repeated trimming into a single normalization step without changing the rendered output.
  - Resolution: the redundant `state.ide` trimming path was collapsed to a single normalization step.

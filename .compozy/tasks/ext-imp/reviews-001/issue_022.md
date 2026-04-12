---
status: resolved
file: internal/core/run/exec/hooks.go
line: 57
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093952670,nitpick_hash:d34c02491600
review_hash: d34c02491600
source_review_id: "4093952670"
source_review_submitted_at: "2026-04-11T16:18:34Z"
---

# Issue 022: Wrap hook dispatch errors with call-site context.
## Review Comment

Returning raw errors here loses which hook failed (`run.pre_start` vs `prompt.post_build`) during triage.

As per coding guidelines: Prefer explicit error returns with wrapped context using `fmt.Errorf("context: %w", err)`.

Also applies to: 91-93

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. `applyExecRunPreStartHook` and `applyExecPromptPostBuildHook` return raw dispatch errors, which loses which hook failed.
  - Root cause: the call sites do not wrap hook dispatch failures with hook-specific context.
  - Intended fix: wrap each dispatch error with the hook name so troubleshooting shows the failing phase directly.
  - Resolution: both mutable exec hook dispatch paths now wrap errors with hook-specific context.

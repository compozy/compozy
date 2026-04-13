---
status: resolved
file: internal/core/run/executor/result_test.go
line: 293
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093952670,nitpick_hash:be20b4edad5a
review_hash: be20b4edad5a
source_review_id: "4093952670"
source_review_submitted_at: "2026-04-11T16:18:34Z"
---

# Issue 023: Manual mutex unlock before each t.Fatalf is error-prone.
## Review Comment

The repeated `captureExecuteStreamsMu.Unlock()` calls before each `t.Fatalf` are necessary to avoid deadlock but add maintenance burden. Consider extracting the inner loop body into a helper that can use `defer` for cleanup.

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. The JSON-output loop in `TestEmitExecutionResultKeepsWorkflowJSONModesQuietOnStdout` manually unlocks `captureExecuteStreamsMu` before each `t.Fatalf`, which is easy to break during later edits.
  - Root cause: stream capture setup/cleanup is duplicated inline instead of being isolated behind a helper with deferred cleanup.
  - Intended fix: extract the stdout-capture sequence into a test helper so the mutex and stdout restoration are always released via `defer`.
  - Resolution: extracted stdout capture into a helper that restores stdout and unlocks via deferred cleanup.

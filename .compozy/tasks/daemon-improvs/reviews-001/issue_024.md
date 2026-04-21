---
status: resolved
file: internal/logger/logger_test.go
line: 12
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148016854,nitpick_hash:080dca09d210
review_hash: 080dca09d210
source_review_id: "4148016854"
source_review_submitted_at: "2026-04-21T13:29:50Z"
---

# Issue 024: Add t.Parallel() to enable concurrent test execution.
## Review Comment

These tests are independent (each uses `t.TempDir()` for isolation) and should run in parallel per coding guidelines.

As per coding guidelines: "Use `t.Parallel()` for independent subtests".

Also applies to: 47-47, 77-77, 107-107

## Triage

- Decision: `invalid`
- Reasoning: the review assumes every test in `internal/logger/logger_test.go` is isolated, but the foreground and detached install tests mutate the process-wide default logger through `slog.SetDefault()`. Running those tests in parallel would race on shared global state and make the suite flaky.
- Resolution note: no code change is needed for this review item; keeping these logger-installation tests serial is the correct behavior.
- Verification: `go test ./internal/logger` and `make verify`

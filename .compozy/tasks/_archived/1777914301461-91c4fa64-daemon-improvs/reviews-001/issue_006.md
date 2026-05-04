---
status: resolved
file: internal/api/contract/sse.go
line: 78
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148016854,nitpick_hash:8b267a824efd
review_hash: 8b267a824efd
source_review_id: "4148016854"
source_review_submitted_at: "2026-04-21T13:29:50Z"
---

# Issue 006: Split the parse failure from the zero-sequence validation.
## Review Comment

`err != nil || sequence == 0` drops the underlying `ParseUint` error, so callers lose the `%w` chain here. Keep the parse failure wrapped and handle `0` as a separate validation error.

As per coding guidelines, "Prefer explicit error returns with wrapped context using `fmt.Errorf("context: %w", err)`".

## Triage

- Decision: `valid`
- Root cause: `ParseCursor` currently folds `strconv.ParseUint` failures and zero-sequence validation into one branch, which discards the wrapped parse error and makes cursor diagnostics less precise.
- Fix approach: preserve the `%w` chain for parse failures and handle `sequence == 0` as a separate validation error.
- Resolution: `ParseCursor` now wraps `ParseUint` failures and validates `sequence == 0` separately.
- Regression coverage: `TestCursorFormattingParsingAndOrderingRemainStable` now asserts that invalid sequence errors preserve a `strconv.NumError`, and `go test ./internal/api/client ./internal/api/contract ./internal/api/core ./internal/api/httpapi` passed.
- Verification: `make verify` passed after the final edit with `2534` tests, `2` skipped daemon helper-process tests, and a successful `go build ./cmd/compozy`.

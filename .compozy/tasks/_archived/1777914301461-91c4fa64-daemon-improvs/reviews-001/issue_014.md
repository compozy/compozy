---
status: resolved
file: internal/cli/daemon_exec_test_helpers_test.go
line: 68
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148016854,nitpick_hash:fb0b571d51b0
review_hash: fb0b571d51b0
source_review_id: "4148016854"
source_review_submitted_at: "2026-04-21T13:29:50Z"
---

# Issue 014: Consider logging or commenting on ignored shutdown error.
## Review Comment

Per coding guidelines, errors should not be ignored with `_` without justification. While test cleanup errors are often non-critical, a brief comment or `t.Logf` would clarify intent:

As per coding guidelines: "NEVER ignore errors with `_` — every error must be handled or have a written justification".

## Triage

- Decision: `valid`
- Root cause: the cleanup path ignores `manager.Shutdown(...)` errors, which hides teardown failures in the daemon-backed CLI test helper.
- Fix approach: keep the cleanup in `t.Cleanup`, but report shutdown failures explicitly with the test handle instead of dropping them.
- Resolution: the cleanup now reports `RunManager.Shutdown()` failures with `t.Errorf(...)` instead of discarding them.
- Regression coverage: `go test ./internal/cli ./internal/core/run/journal ./internal/daemon` passed after the cleanup update.
- Verification: `make verify` passed after the final edit with `2534` tests, `2` skipped daemon helper-process tests, and a successful `go build ./cmd/compozy`.

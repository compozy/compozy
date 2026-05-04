---
status: resolved
file: internal/api/core/handlers_contract_test.go
line: 191
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4149408235,nitpick_hash:3d960091697e
review_hash: 3d960091697e
source_review_id: "4149408235"
source_review_submitted_at: "2026-04-21T16:46:20Z"
---

# Issue 001: Missing t.Parallel() call.
## Review Comment

This test should include `t.Parallel()` for consistency with other tests in this file.

## Triage

- Decision: `valid`
- Notes:
  - The current top-level contract test is isolated and already uses only per-test state (`newFakeRunStream`, `httptest.NewServer`, local channels), so it can safely follow the same `t.Parallel()` pattern used by the other top-level tests in this file.
  - Root cause: this test was added without the parallel marker even though the surrounding file defaults to parallel top-level cases.
  - Fix approach: add `t.Parallel()` at the start of `TestStreamRunEmitsCanonicalEventHeartbeatAndOverflowPayloads` while keeping the existing per-test setup unchanged.
  - Resolution: `TestStreamRunEmitsCanonicalEventHeartbeatAndOverflowPayloads` now declares `t.Parallel()`, matching the file’s established top-level contract-test pattern.
  - Verification: `go test ./internal/api/core -run 'TestStreamRunEmitsCanonicalEventHeartbeatAndOverflowPayloads|TestDaemonHealthReturnsCanonicalEnvelopeForReadyAndDegradedStates|TestRunStartEndpointsReturnCanonicalRunEnvelopes|TestTransportErrorsUseCanonicalCodeAndRequestIDFields' -count=1` passed. `make verify` also passed with `2548` tests, `2` skipped helper-process tests, and a successful `go build ./cmd/compozy`.

---
status: resolved
file: internal/api/core/handlers_contract_test.go
line: 59
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148862187,nitpick_hash:ea2fabbf0f3c
review_hash: ea2fabbf0f3c
source_review_id: "4148862187"
source_review_submitted_at: "2026-04-21T15:19:50Z"
---

# Issue 001: Loop variable capture pattern is redundant in Go 1.22+.
## Review Comment

The `tc := tc` pattern on lines 60 and 161 was necessary before Go 1.22 to capture loop variables for parallel subtests. Since Go 1.22+, loop variables are per-iteration by default, making this pattern unnecessary. Consider removing for cleaner code if the project targets Go 1.22+.

Also applies to: 160-163

## Triage

- Decision: `valid`
- Root cause: `internal/api/core/handlers_contract_test.go` still uses `tc := tc` loop-variable rebinding in table-driven subtests even though this module targets `go 1.26.1`, where range variables are already per-iteration.
- Plan: remove the redundant rebinding in the affected loops and keep the subtest bodies unchanged.
- Resolution: removed the redundant loop-variable rebinding from all three table-driven loops in `internal/api/core/handlers_contract_test.go`, including the extra matching occurrence later in the file.
- Regression coverage: the existing canonical handler contract tests still exercise the same ready/degraded, run-start, and stream envelope paths; this change only removes obsolete capture noise.
- Verification: `go test ./internal/api/core -run 'Test(DaemonHealthReturnsCanonicalEnvelopeForReadyAndDegradedStates|RunStartEndpointsReturnCanonicalRunEnvelopes|StreamRunEmitsCanonicalEventHeartbeatAndOverflowPayloads)$' -count=1` passed. `make verify` then passed with `2544` tests and `2` skipped helper-process tests.

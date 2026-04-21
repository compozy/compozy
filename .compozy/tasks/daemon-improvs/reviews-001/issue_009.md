---
status: resolved
file: internal/api/core/handlers_contract_test.go
line: 398
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148016854,nitpick_hash:d4b344e5e955
review_hash: d4b344e5e955
source_review_id: "4148016854"
source_review_submitted_at: "2026-04-21T13:29:50Z"
---

# Issue 009: Extract the SSE frame parser into a shared test helper.
## Review Comment

This frame type and parser logic are now duplicated across multiple new test files. Keeping separate copies in sync will get brittle the next time the SSE contract changes.

As per coding guidelines, "Check for shared test utilities usage to avoid duplication".

## Triage

- Decision: `valid`
- Root cause: near-identical SSE frame parsers now exist in `internal/api/contract/contract_integration_test.go`, `internal/api/core/handlers_contract_test.go`, and `internal/api/httpapi/transport_integration_test.go`. That duplication already caused the packages to diverge in scanner ownership and makes future contract changes brittle.
- Fix approach: extract the shared frame parser into a small test-helper package and update the affected tests to reuse it. This requires one additional helper file outside the listed code-file scope because the three tests live in different packages.
- Resolution: added the shared helper `internal/api/testutil/sse.go` and switched the contract, core, and httpapi SSE tests to reuse it.
- Regression coverage: `go test ./internal/api/client ./internal/api/contract ./internal/api/core ./internal/api/httpapi` and `go test -tags integration ./internal/api/contract` both passed after the shared-helper migration.
- Verification: `make verify` passed after the final edit with `2534` tests, `2` skipped daemon helper-process tests, and a successful `go build ./cmd/compozy`.

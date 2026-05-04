---
status: resolved
file: internal/api/core/handlers_contract_test.go
line: 232
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4149167497,nitpick_hash:7a46ba6aefcf
review_hash: 7a46ba6aefcf
source_review_id: "4149167497"
source_review_submitted_at: "2026-04-21T16:03:49Z"
---

# Issue 002: Consider adding a timeout to the HTTP client for test reliability.
## Review Comment

The `http.DefaultClient` has no timeout configured. If the server hangs before sending headers, the test will block indefinitely. While `ReadSSEFramesUntil` has a timeout for reading frames, the initial connection has no protection.

## Triage

- Decision: `VALID`
- Reasoning: `TestStreamRunEmitsCanonicalEventHeartbeatAndOverflowPayloads` currently uses an HTTP client with no connection timeout, so a hang before response headers would block the test indefinitely.
- Root cause: The test uses `http.DefaultClient.Do(request)` instead of a client with an explicit deadline.
- Resolution plan: Use the test server's client with a bounded timeout before issuing the stream request.

## Resolution

- Replaced `http.DefaultClient` with the `httptest.Server` client and set a `5s` timeout before opening the SSE stream in `internal/api/core/handlers_contract_test.go`.

## Verification

- `go test ./internal/api/core ./internal/daemon ./internal/logger -count=1`
- `make verify`

---
status: resolved
file: internal/api/httpapi/transport_integration_test.go
line: 1851
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58go79,comment:PRRC_kwDORy7nkc651UMb
---

# Issue 010: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Stop the SSE scanner goroutine when the helper exits early.**

`readSSEFramesUntil` returns as soon as `stop(frames)` is true, but the scanner goroutine keeps blocking on `body` because this helper only owns an `io.Reader` and has no cancellation path. Repeated uses can leak goroutines and make these SSE tests flaky under `-race`.  


As per coding guidelines, "NEVER fire-and-forget goroutines — every goroutine must have explicit ownership and shutdown handling".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/httpapi/transport_integration_test.go` around lines 1789 - 1851,
readSSEFramesUntil currently takes an io.Reader so when we return early
(stop(frames) == true) the scanner goroutine keeps blocking on body and leaks;
change the function to accept an io.ReadCloser (or add a separate io.Closer
param), and ensure you Close() the body when exiting early so the scanner
unblocks; update the goroutine/cleanup to still send scanner.Err() to errCh and
close linesCh as before, and call body.Close() (or defer a close on normal
return) immediately before returning from readSSEFramesUntil to ensure the
scanner goroutine terminates.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5db983a7-a2cf-4801-8b6f-2aa2e3a5e394 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `readSSEFramesUntil` starts a scanner goroutine around an `io.Reader` and returns as soon as the stop condition is met, but it has no way to stop the goroutine or unblock the reader. That leaks goroutines across repeated SSE transport tests.
- Fix approach: route the parser through a helper that owns an `io.ReadCloser`, closes it on early termination or timeout, and waits for the scanner goroutine to exit before returning.
- Resolution: the shared `ReadSSEFramesUntil` helper now owns the stream body, closes it on early stop or timeout, and drains the scanner exit before returning; the httpapi transport tests now use that helper.
- Regression coverage: `go test ./internal/api/client ./internal/api/contract ./internal/api/core ./internal/api/httpapi` passed after the leak fix.
- Verification: `make verify` passed after the final edit with `2534` tests, `2` skipped daemon helper-process tests, and a successful `go build ./cmd/compozy`.

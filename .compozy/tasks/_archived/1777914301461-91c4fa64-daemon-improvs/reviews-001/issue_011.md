---
status: resolved
file: internal/api/httpapi/transport_integration_test.go
line: 1893
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58go8A,comment:PRRC_kwDORy7nkc651UMf
---

# Issue 011: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Handle the `ReadAll` failure in the helper's error path.**

The response-body read is ignored here, so a short read or transport failure gets silently dropped right when the helper is building the failure message.  


<details>
<summary>Suggested fix</summary>

```diff
 	if response.StatusCode != http.StatusOK {
 		defer response.Body.Close()
-		body, _ := io.ReadAll(response.Body)
-		t.Fatalf("status = %d, want 200; body=%s", response.StatusCode, body)
+		body, readErr := io.ReadAll(response.Body)
+		if readErr != nil {
+			t.Fatalf("ReadAll(%s) error = %v", rawURL, readErr)
+		}
+		t.Fatalf("status = %d, want 200; body=%s", response.StatusCode, body)
 	}
```
</details>

As per coding guidelines, "NEVER ignore errors with `_` — every error must be handled or have a written justification".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/httpapi/transport_integration_test.go` around lines 1889 - 1893,
The test helper currently ignores the error returned by io.ReadAll when building
the failure message for a non-OK response; update the error path in the block
that checks response.StatusCode (the code using response, io.ReadAll and
t.Fatalf) to properly handle the ReadAll error by checking and including it in
the failure output (e.g., if io.ReadAll returns an error, include that error
string in the t.Fatalf message alongside or instead of the body content), ensure
response.Body is deferred closed before reading, and avoid using the blank
identifier for the read error.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5db983a7-a2cf-4801-8b6f-2aa2e3a5e394 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: the helper discards `io.ReadAll` errors while building the failure message for non-OK stream responses, so a truncated read or transport failure disappears at exactly the point where the test should surface it.
- Fix approach: read the body with an explicit error check and include a targeted fatal message when that read fails.
- Resolution: `mustStreamRequest` now handles `io.ReadAll` errors explicitly before composing the failure message.
- Regression coverage: `go test ./internal/api/client ./internal/api/contract ./internal/api/core ./internal/api/httpapi` passed after the helper update.
- Verification: `make verify` passed after the final edit with `2534` tests, `2` skipped daemon helper-process tests, and a successful `go build ./cmd/compozy`.

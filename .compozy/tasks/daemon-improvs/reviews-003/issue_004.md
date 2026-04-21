---
status: resolved
file: internal/daemon/boot_integration_test.go
line: 391
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58jii8,comment:PRRC_kwDORy7nkc655NST
---

# Issue 004: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Handle the ignored helper errors explicitly.**

These new helpers still discard `Kill`, `Wait`, `ReadInfo`, and `ReadFile` failures with `_`. That will hide the root cause when daemon startup/shutdown flakes in CI. Please either surface those errors in the fatal path or add an explicit `errors.Is(...)` justification for the expected cases.

As per coding guidelines, "NEVER ignore errors with `_` — every error must be handled or have a written justification".


Also applies to: 472-473, 506-506, 550-550

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/boot_integration_test.go` around lines 388 - 391, The cleanup
and helper calls currently ignore errors (e.g., the calls to cmd.Process.Kill(),
cmd.Process.Wait(), ReadInfo, ReadFile) using `_`, which hides failures; update
each occurrence so you capture the error (err := cmd.Process.Kill(), err :=
cmd.Process.Wait(), etc.) and handle it explicitly — for unexpected failures
call t.Fatalf or t.Errorf to surface the root cause in CI, and for any genuinely
expected/benign errors use an explicit errors.Is(...) check with a short comment
justifying why that specific error can be ignored; apply this change to the
cleanup block around cmd.Process.Kill()/Wait and to the ReadInfo/ReadFile helper
usages mentioned so no error is discarded without handling or justification.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:82e27b84-9af7-416f-b268-4f6848c55952 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Reasoning: The helper cleanup and timeout-diagnostic paths currently discard `Kill`, `Wait`, `ReadInfo`, and `ReadFile` errors, which would hide the real failure mode when daemon process orchestration flakes in CI.
- Root cause: Several test helpers use `_ =` or `_, _ =` in cleanup and diagnostic paths instead of surfacing unexpected errors or including them in the failure output.
- Resolution plan: Handle unexpected process cleanup failures explicitly and include info/log read failures in the fatal diagnostic message instead of discarding them.

## Resolution

- Added explicit `signal`, `kill`, and `wait` helper handling for test processes in `internal/daemon/boot_integration_test.go`.
- Replaced ignored info/log reads in failure paths with diagnostic helpers that include the read error text in the fatal message.
- Removed the remaining ignored `Kill()`/`ReadFile()` paths called out by the review in the scoped helper code.

## Verification

- `go test ./internal/api/core ./internal/daemon ./internal/logger -count=1`
- `make verify`

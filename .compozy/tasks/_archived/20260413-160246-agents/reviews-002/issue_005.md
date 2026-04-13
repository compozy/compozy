---
status: resolved
file: internal/core/run/exec/prompt_exec.go
line: 86
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56QRk9,comment:PRRC_kwDORy7nkc62z8Sa
---

# Issue 005: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Don’t hide `completeTurn` failures behind the exec error.**

If `executeExecJob` fails and `state.completeTurn(result)` also fails, this branch returns only `result.err`. That drops the terminal-state persistence failure, so callers never learn that the child run may not have been recorded as completed.



Based on learnings "Stress-test logic and failure modes, not just happy path conclusions".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/exec/prompt_exec.go` around lines 81 - 86, The current
logic calls executeExecJob and then state.completeTurn(result) but if both
completeTurn and result.err are non-nil it returns only result.err, hiding the
persistence failure; change the control flow in the block around executeExecJob
/ state.completeTurn so you capture the error returned by state.completeTurn
(e.g., errComplete := state.completeTurn(result)) and if errComplete != nil
return buildPreparedPromptResult(state, result), errComplete (before returning
result.err), otherwise proceed to return result.err if present or the successful
value; this ensures state.completeTurn failures (persistence/terminal-state
errors) are surfaced instead of being masked by result.err.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:e61192d9-7c66-438c-8efd-0a27424736ab -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - `ExecutePreparedPrompt` currently ignores `state.completeTurn(result)` failures whenever `result.err` is also non-nil, so nested exec persistence/emission failures can be masked by the child execution error.
  - The nearby `finalizeExecResult` path already preserves both failure sources, which confirms this branch is the inconsistent one.
  - Intended fix: capture the completion error, return it when it is the only failure, and join it with `result.err` when both occur; add a regression test that forces a failed exec plus a failed completion write.
  - Resolution: updated `internal/core/run/exec/prompt_exec.go` to preserve `completeTurn` failures even when the child exec itself fails, joining distinct errors so neither root cause is lost; added regression coverage in `internal/core/run/exec/exec_test.go`.
  - Verification: `go test ./internal/core/agents/... ./internal/core/run/exec/...` and `make verify`.

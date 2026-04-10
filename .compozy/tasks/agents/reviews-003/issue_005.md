---
status: resolved
file: internal/core/run/exec/prompt_exec.go
line: 67
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56QZsY,comment:PRRC_kwDORy7nkc620G-3
---

# Issue 005: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
cat -n internal/core/run/exec/prompt_exec.go | sed -n '55,80p'
```

Repository: compozy/compozy

Length of output: 1079

---

🏁 Script executed:

```shell
cat -n internal/core/run/exec/prompt_exec.go | head -100
```

Repository: compozy/compozy

Length of output: 3923

---

🏁 Script executed:

```shell
cat -n internal/core/run/exec/prompt_exec.go | sed -n '81,89p'
```

Repository: compozy/compozy

Length of output: 555

---



</details>

**Preserve both MCP-builder and completion failures; inconsistent with the exec-job error handling pattern.**

The condition at line 64 returns only `completeErr` when both `err` and `completeErr` exist and differ, masking the root MCP builder failure. Lines 82–86 handle the analogous exec-job scenario correctly using `errors.Join()` to preserve both errors. Apply the same pattern here.

<details>
<summary>Proposed fix</summary>

```diff
-			if completeErr := state.completeTurn(failure); completeErr != nil && !errors.Is(completeErr, err) {
-				return buildPreparedPromptResult(state, failure), completeErr
-			}
-			return buildPreparedPromptResult(state, failure), err
+			if completeErr := state.completeTurn(failure); completeErr != nil {
+				if !errors.Is(completeErr, err) {
+					return buildPreparedPromptResult(state, failure), errors.Join(err, completeErr)
+				}
+				return buildPreparedPromptResult(state, failure), completeErr
+			}
+			return buildPreparedPromptResult(state, failure), err
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/exec/prompt_exec.go` around lines 64 - 67, The current
branch in prompt_exec.go returns only completeErr when both err (the MCP-builder
failure) and completeErr (the completion failure from state.completeTurn) exist
and differ, which masks the original err; change the return to preserve both
errors using errors.Join(err, completeErr) (or errors.Join(completeErr, err)) so
callers see both failures, keeping the same returned prepared result from
buildPreparedPromptResult(state, failure) and referencing state.completeTurn,
buildPreparedPromptResult, err and completeErr to locate the code to update.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:3bf551e7-5f49-4b28-8f83-cabd39a9c4c5 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes:
  - The MCP-builder failure branch in `ExecutePreparedPrompt` currently differs from the main exec completion path in `finalizeExecResult`.
  - When `buildMCPServers` fails and `state.completeTurn(failure)` also fails with a different error, the current code returns only the completion error and drops the original builder error.
  - Root cause: the branch short-circuits to `completeErr` instead of preserving both failures the way the exec-job path already does.
  - Fix approach: join the builder error and completion error with `errors.Join` and add a regression test that forces both failures.
  - Resolved in `internal/core/run/exec/prompt_exec.go` and covered by `TestExecutePreparedPromptReturnsBuilderAndCompletionFailure` in `internal/core/run/exec/exec_test.go`.
  - Verified by `go test ./internal/core/run/exec -count=1` and the final `make verify` pass.

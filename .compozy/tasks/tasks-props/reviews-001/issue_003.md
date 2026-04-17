---
status: resolved
file: internal/cli/state.go
line: 74
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc57ypzA,comment:PRRC_kwDORy7nkc644Msj
---

# Issue 003: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Defaulting retries to 2 changes failure semantics for non-idempotent jobs.**

These executions can already have modified files before an ACP timeout or execution-stage failure is reported. Making retries opt-out means the CLI will now rerun those jobs by default, which can duplicate edits or follow-up actions in the workspace.

<details>
<summary>Suggested fix</summary>

```diff
-const defaultMaxRetries = 2
+const defaultMaxRetries = 0
```
</details>



Also applies to: 239-244

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/state.go` around lines 73 - 74, The defaultMaxRetries constant
currently set to 2 causes non-idempotent jobs to be retried by default; change
defaultMaxRetries to 0 (no retries) and ensure retry behavior is opt-in via the
existing CLI retry flag or configuration (update any code paths referencing
defaultMaxRetries such as where retries are parsed/applied and any help text or
tests); specifically update the constant defaultMaxRetries and adjust related
logic that merges defaults into actual retry settings so retries only occur when
the user explicitly enables them (also update any unit tests or docs that assume
a default of 2).
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:7bb75d9d-dbd3-41c6-89da-03415801a6e9 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. `defaultMaxRetries` is currently `2`, and the executor marks generic execution failures and activity timeouts as retryable.
  - Root cause: making retries the built-in default changes workflow semantics from single-attempt to implicit re-execution even though review/PRD jobs are not guaranteed to be idempotent after partial workspace edits.
  - Intended fix: restore opt-in retry behavior by setting the built-in CLI default back to `0` while keeping explicit `--max-retries` and config overrides intact.
  - Resolution: the built-in retry default is back to `0`, the CLI regression test now asserts the opt-in default, and the CLI help golden files were updated to reflect Cobra no longer printing a non-zero default for this flag.

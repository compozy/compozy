---
status: resolved
file: internal/core/agents/agents.go
line: 159
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56QRku,comment:PRRC_kwDORy7nkc62z8SK
---

# Issue 001: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Preserve the full `Problem` when resolving a known-invalid agent.**

This wraps `problem.Err`, so callers lose the scope/source details carried by `Problem.Error()`. Wrap `problem` here instead so the richer diagnostics survive while unwrapping still works.

<details>
<summary>Suggested fix</summary>

```diff
-			return ResolvedAgent{}, fmt.Errorf("resolve agent %q: %w", normalized, problem.Err)
+			return ResolvedAgent{}, fmt.Errorf("resolve agent %q: %w", normalized, problem)
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	for _, problem := range c.Problems {
		if problem.Name == normalized {
			return ResolvedAgent{}, fmt.Errorf("resolve agent %q: %w", normalized, problem)
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agents/agents.go` around lines 157 - 159, The current loop
returns fmt.Errorf("resolve agent %q: %w", normalized, problem.Err) which
unwraps only the inner error and loses the richer Problem diagnostics; change
the wrap to use the whole Problem value (e.g., fmt.Errorf("resolve agent %q:
%w", normalized, problem)) so callers retain Problem.Error() context and
unwrapping still works; update the return in the loop that iterates c.Problems
(where problem.Name == normalized) to wrap problem instead of problem.Err.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:e61192d9-7c66-438c-8efd-0a27424736ab -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - `Catalog.Resolve` currently wraps `problem.Err`, which preserves `errors.Is(...)` checks but discards the richer `Problem.Error()` text that carries the agent name and source scope.
  - `Problem` already implements both `error` and `Unwrap()`, so wrapping the full `problem` preserves the detailed message while keeping the underlying validation error available to callers.
  - Intended fix: wrap `problem` instead of `problem.Err`, then extend agent resolution coverage to assert the error still unwraps correctly and retains the contextual message.
  - Resolution: updated `internal/core/agents/agents.go` to wrap the full `Problem`, and extended `internal/core/agents/agents_test.go` so the malformed override path now asserts the contextual `planner (workspace)` message while `errors.Is(..., ErrMalformedFrontmatter)` still passes.
  - Verification: `go test ./internal/core/agents/... ./internal/core/run/exec/...` and `make verify`.

# Issues for `engine/llm/orchestrator/request_builder.go`

## Issue 13 - Review Thread Comment

**File:** `engine/llm/orchestrator/request_builder.go:29`
**Date:** 2025-10-20 03:07:21 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🧹 Nitpick_ | _🔵 Trivial_

**Consider making tool-suggestion cap configurable.**

maxToolSuggestions=3 is reasonable but tunable; expose via settings/config with a default to avoid hardcoding.

As per coding guidelines

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In engine/llm/orchestrator/request_builder.go at line 29, the hardcoded constant
maxToolSuggestions = 3 should be made configurable: replace the constant with a
configurable field (e.g., part of the orchestrator/request builder config or
settings struct) that defaults to 3, read from configuration or an environment
variable during initialization, validate the value (must be >=0 and reasonable
upper bound), and update any constructors/initializers and callers to use the
config field instead of the constant; also update related tests and any
documentation to reflect the new configurable parameter and default.
```

</details>

<!-- fingerprinting:phantom:medusa:chinchilla -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5eiyQC`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5eiyQC
```

---
*Generated from PR review - CodeRabbit AI*

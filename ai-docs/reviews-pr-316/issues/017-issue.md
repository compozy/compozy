# Issue 17 - Review Thread Comment

**File:** `sdk/compozy/lifecycle.go:176`
**Date:** 2025-11-01 01:57:03 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🛠️ Refactor suggestion_ | _🟠 Major_

**Replace the hardcoded shutdown timeout**

Line 168 hardcodes a `5 * time.Second` timeout inside runtime code, which violates our “no magic numbers” rule. Please promote this to a named constant (or, even better, pull it from configuration alongside the other server timeouts) and use the constant here for clarity and maintainability. As per coding guidelines.

```diff
-	if state.server != nil {
-		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
+	if state.server != nil {
+		shutdownCtx, cancel := context.WithTimeout(ctx, defaultHTTPShutdownTimeout)
```
Don’t forget to declare `defaultHTTPShutdownTimeout` near the other HTTP defaults with a meaningful name and type. 


> Committable suggestion skipped: line range outside the PR's diff.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In sdk/compozy/lifecycle.go around lines 168 to 176, the shutdown uses a
hardcoded 5*time.Second which violates the no-magic-numbers rule; declare a
named constant (e.g. defaultHTTPShutdownTimeout time.Duration) alongside the
other HTTP default constants (or expose it via existing server configuration if
available) and replace the inline 5*time.Second with that constant (or read from
the config variable) when creating shutdownCtx; ensure the constant has a
descriptive name and appropriate type and is used consistently for HTTP shutdown
timeouts.
```

</details>

<!-- fingerprinting:phantom:medusa:sabertoothed -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gLa2q`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gLa2q
```

---
*Generated from PR review - CodeRabbit AI*

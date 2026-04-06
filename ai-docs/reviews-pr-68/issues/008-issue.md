# Issue 8 - Review Thread Comment

**File:** `pkg/compozy/runs/watch.go:209`
**Date:** 2026-04-06 11:28:05 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** VALID

**Rationale:** `sendSetupError` mantém o envio não bloqueante, mas agora registra via `slog` quando o buffer já está cheio e um erro de setup seria descartado silenciosamente.

## Body

_⚠️ Potential issue_ | _🟡 Minor_

**`sendSetupError` may silently drop errors if channel buffer is full.**

The `default` case in the select allows the function to return without blocking, but if the error channel (capacity 4) is somehow full, the setup error would be silently dropped. This is unlikely in practice since setup errors occur before the main loop starts, but consider logging dropped errors.


<details>
<summary>🛡️ Suggested improvement</summary>

```diff
 func sendSetupError(dst chan<- error, err error) {
 	if err == nil {
 		return
 	}
 	select {
 	case dst <- err:
 	default:
+		// Channel full - this shouldn't happen during setup but log if it does
+		slog.Error("failed to send setup error", "error", err)
 	}
 }
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func sendSetupError(dst chan<- error, err error) {
	if err == nil {
		return
	}
	select {
	case dst <- err:
	default:
		// Channel full - this shouldn't happen during setup but log if it does
		slog.Error("failed to send setup error", "error", err)
	}
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@pkg/compozy/runs/watch.go` around lines 201 - 209, sendSetupError currently
swallows an error when the dst channel is full due to the select default; update
sendSetupError to detect when the non-blocking send fails and emit a log (using
the package logger or log.Printf) that includes the dropped error and context
(e.g., "dropped setup error sending to dst chan"). Keep the non-blocking
behavior but ensure the else/default branch logs the error details so dropped
errors are visible for debugging; reference the sendSetupError function and the
dst channel in your change.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:2974b5e6-6986-4720-8bc5-83199c8c0f48 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55C9uw`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55C9uw
```

---
*Generated from PR review - CodeRabbit AI*

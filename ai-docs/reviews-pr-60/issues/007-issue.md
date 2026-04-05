# Issue 7 - Review Thread Comment

**File:** `internal/core/agent/registry.go:356`
**Date:** 2026-04-05 19:22:18 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** VALID

**Rationale:** The broader runtime stack normalizes `""` to text output, but `validateRuntimeOutputFormat()` currently rejects that zero value before defaults are applied. Aligning validation with the runtime default removes a stricter-than-runtime mismatch for unset callers.

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Treat the zero-value output format as text.**

The rest of the runtime already treats `""` as text, and `internal/cli/root.go` still builds non-exec configs with the zero value when no exec-only format is set. Rejecting `""` here makes validation stricter than the execution stack and will fail any caller that passes the field through unset.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/registry.go` around lines 341 - 356, The validation in
validateRuntimeOutputFormat incorrectly rejects the zero-value output format;
treat the empty string as equivalent to model.OutputFormatText by accepting ""
in the allowed cases (or normalize cfg.OutputFormat = model.OutputFormatText
before validating). Update validateRuntimeOutputFormat (referencing
cfg.OutputFormat and cfg.Mode) so "" is considered valid and the existing
exec-mode check continues to compare against model.OutputFormatText (or the
normalized value).
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5606214d-8719-43d5-98a2-064a6cd78ca1 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc547tHk`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc547tHk
```

---
*Generated from PR review - CodeRabbit AI*

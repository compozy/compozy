---
status: resolved
file: internal/core/agent/registry_validate.go
line: 156
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc55LZWH,comment:PRRC_kwDORy7nkc61XmQt
---

# Issue 009: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Count `ResolvedPromptText` as a fallback prompt source.**

`resolveExecPromptText` in `internal/core/run/exec/exec.go` already accepts `ResolvedPromptText` first, but `runtimePromptSourceCount` ignores it. A caller that normalizes stdin/argv into `ResolvedPromptText` and clears the raw source fields will fail validation even though exec can run the request.


<details>
<summary>💡 Suggested fix</summary>

```diff
 func runtimePromptSourceCount(cfg *model.RuntimeConfig) int {
 	sources := 0
 	if strings.TrimSpace(cfg.PromptText) != "" {
 		sources++
 	}
 	if strings.TrimSpace(cfg.PromptFile) != "" {
 		sources++
 	}
 	if cfg.ReadPromptStdin {
 		sources++
 	}
+	if sources == 0 && strings.TrimSpace(cfg.ResolvedPromptText) != "" {
+		sources = 1
+	}
 	return sources
 }
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/registry_validate.go` around lines 145 - 156, The
runtimePromptSourceCount function currently counts PromptText, PromptFile, and
ReadPromptStdin but omits ResolvedPromptText, causing valid requests normalized
into ResolvedPromptText to fail validation; update runtimePromptSourceCount(cfg
*model.RuntimeConfig) to also check strings.TrimSpace(cfg.ResolvedPromptText) !=
"" and increment sources when present (treat it as a fallback prompt source), so
the function aligns with resolveExecPromptText which prefers ResolvedPromptText.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:51576401-e056-421f-904c-06271e28cefe -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  `resolveExecPromptText` already accepts `ResolvedPromptText` as the highest-priority prompt source, but `runtimePromptSourceCount` ignores it, so normalized exec configs can fail validation unnecessarily. The fix is to count `ResolvedPromptText` as the effective source when the raw prompt fields are absent, keeping validation aligned with execution.
  Resolution: implemented and verified with the focused Go test run plus `make verify`.

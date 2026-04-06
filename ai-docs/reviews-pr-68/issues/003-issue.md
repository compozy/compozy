# Issue 3 - Review Thread Comment

**File:** `internal/core/run/result.go:121`
**Date:** 2026-04-06 11:28:05 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** VALID

**Rationale:** `emitExecutionResult` passou a tratar `json` e `raw-json` separadamente. O artefato em disco continua identado, mas `raw-json` agora escreve o payload compacto em stdout; há teste cobrindo esse caminho.

## Body

_⚠️ Potential issue_ | _🔴 Critical_

**Handle `raw-json` separately.**

`model.OutputFormatRawJSON` is still a supported exec format, but this guard now treats every non-`json` mode like text and returns before stdout is written. `--output-format raw-json` will emit no result payload at all.

<details>
<summary>💡 Proposed fix</summary>

```diff
-	if cfg.outputFormat != model.OutputFormatJSON {
-		return nil
-	}
-	if _, err := fmt.Fprintf(os.Stdout, "%s\n", payload); err != nil {
+	stdoutPayload := payload
+	switch cfg.outputFormat {
+	case model.OutputFormatJSON:
+	case model.OutputFormatRawJSON:
+		stdoutPayload, err = json.Marshal(result)
+		if err != nil {
+			return fmt.Errorf("marshal raw exec result: %w", err)
+		}
+	default:
+		return nil
+	}
+	if _, err := fmt.Fprintf(os.Stdout, "%s\n", stdoutPayload); err != nil {
 		return fmt.Errorf("write exec result stdout: %w", err)
 	}
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/result.go` around lines 119 - 121, The current early-return
treats anything not equal to model.OutputFormatJSON as text and skips writing
stdout, which drops --output-format raw-json; update the conditional so it only
returns early for formats that are neither model.OutputFormatJSON nor
model.OutputFormatRawJSON (i.e., allow processing to continue when
cfg.outputFormat == model.OutputFormatJSON or cfg.outputFormat ==
model.OutputFormatRawJSON). Locate the check using cfg.outputFormat in result.go
(around the if cfg.outputFormat != model.OutputFormatJSON block) and adjust it
to explicitly handle model.OutputFormatRawJSON as a JSON-path that should not
return early.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:30c6a342-10dd-4312-82fe-961af791fe0d -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55C9uU`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55C9uU
```

---
*Generated from PR review - CodeRabbit AI*

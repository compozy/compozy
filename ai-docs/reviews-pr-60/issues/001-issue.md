# Issue 1 - Review Thread Comment

**File:** `internal/cli/root_test.go:569`
**Date:** 2026-04-05 16:33:47 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** VALID

**Rationale:** The cited table-driven cases currently use plain labels even though this repo’s test convention requires `t.Run("Should ...")` style names. Renaming the cases is a localized fix with no behavioral risk.

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Use `Should...` subtest names in the table-driven exec prompt cases.**

The subtests are table-driven, but current names (`"positional prompt"`, etc.) don’t follow the required `t.Run("Should...")` convention.

<details>
<summary>Suggested rename</summary>

```diff
-		{
-			name:           "positional prompt",
+		{
+			name:           "Should resolve prompt from positional argument",
 			args:           []string{"Summarize the repo"},
 			wantPromptText: "Summarize the repo",
 			wantResolved:   "Summarize the repo",
 		},
 		{
-			name:           "prompt file",
+			name:           "Should resolve prompt from --prompt-file",
 			promptFile:     promptPath,
 			wantPromptFile: promptPath,
 			wantResolved:   "Prompt from file\n",
 		},
 		{
-			name:                "stdin prompt",
+			name:                "Should resolve prompt from stdin",
 			stdin:               strings.NewReader("Prompt from stdin\n"),
 			wantReadPromptStdin: true,
 			wantResolved:        "Prompt from stdin\n",
 		},
```
</details>


As per coding guidelines, `**/*_test.go`: "MUST use t.Run("Should...") pattern for ALL test cases".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/root_test.go` around lines 559 - 563, The subtests in the
table-driven tests call t.Run(tc.name, ...) with names like "positional prompt"
that don't follow the required "Should..." convention; update the test case
names in the cases table (the elements used by the loop over cases) so each
tc.name begins with "Should ..." (e.g., "Should handle positional prompt"), or
replace the t.Run call to format a "Should ..." prefix
(t.Run(fmt.Sprintf("Should %s", tc.name), ...)) so all subtest names conform to
the t.Run("Should...") pattern referenced by the test harness; locate the table
of test cases and the t.Run invocation inside the loop to apply the change
(cases, tc.name, t.Run).
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:6f24ca4c-62f7-4b50-b32b-781dee6ae407 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc547MWH`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc547MWH
```

---
*Generated from PR review - CodeRabbit AI*

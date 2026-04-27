# Issue 1 - Review Thread Comment

**File:** `internal/api/core/handlers_service_errors_test.go:367`
**Date:** 2026-04-27 14:49:00 UTC
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟡 Minor_

**Rename this test case to follow the required `Should...` pattern**

Please rename the case label (e.g., `"Should return validation_error for wrapped task parse failures"`) to match the enforced test naming convention.

<details>
<summary>Suggested change</summary>

```diff
-			"sync validation error",
+			"Should return validation_error for wrapped task parse failures",
```

</details>
As per coding guidelines, "MUST use t.Run(\"Should...\") pattern for ALL test cases".

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
			"Should return validation_error for wrapped task parse failures",
			&core.HandlerConfig{Sync: &errorSyncService{
				err: tasks.WrapParseError("/tmp/task_01.md", tasks.ErrV1TaskMetadata),
			}},
			http.MethodPost,
			"/api/sync",
			`{"workspace":"ws-1","workflow_slug":"daemon"}`,
			http.StatusUnprocessableEntity,
			"validation_error",
		},
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/core/handlers_service_errors_test.go` around lines 358 - 367,
Rename the test case label string "sync validation error" in the table-driven
tests (the entry that uses &core.HandlerConfig{Sync: &errorSyncService{...}} and
asserts http.StatusUnprocessableEntity with "validation_error") to follow the
test naming convention by starting with "Should", e.g. "Should return
validation_error for wrapped task parse failures", so that the t.Run invocation
uses the required `t.Run("Should...")` pattern for this case.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:b27f5408-b987-4945-8b9c-5076daecce81 -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: `VALID`
- Rationale: o repositório exige nomes de casos `Should...` nos `t.Run` table-driven. O caso adicionado ficou inconsistente com esse padrão.

## Resolve

Thread ID: `PRRT_kwDORy7nkc593g9h`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc593g9h
```

---

_Generated from PR review - CodeRabbit AI_

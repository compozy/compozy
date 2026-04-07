# Issue 4 - Review Thread Comment

**File:** `internal/core/migration/workflow_target_test.go:40`
**Date:** 2026-04-07 11:44:37 America/Sao_Paulo
**Status:** - [x] ADDRESSED

## Technical Disposition

`VALID` - the workflow-name validation path was string-matched in the test, which is inconsistent with the package guidance. The fix introduces `ErrInvalidWorkflowName`, wraps it from `normalizeWorkflowName`, and asserts with `errors.Is` in `internal/core/migration/workflow_target_test.go`.

The comment's extra note about `migrate_test.go` did not apply to the same validation path, so only the workflow-name case was changed.

## Body

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
cd internal/core/migration && pwd && ls -la
```

Repository: compozy/compozy

Length of output: 491

---

🏁 Script executed:

```shell
cat -n internal/core/migration/workflow_target_test.go | head -60
```

Repository: compozy/compozy

Length of output: 1450

---

🏁 Script executed:

```shell
cat -n internal/core/migration/workflow_target.go | grep -A 10 -B 5 "single active workflow"
```

Repository: compozy/compozy

Length of output: 426

---

🏁 Script executed:

```shell
rg "ErrWorkflow|ErrorWorkflow|Workflow.*Error" internal/core/migration/ -A 2 -B 2
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
rg "errors\.Is|errors\.As" internal/core/migration/ -l
```

Repository: compozy/compozy

Length of output: 93

---

🏁 Script executed:

```shell
rg "ErrorContains|ErrorAs" --type go -l | head -20
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
cat -n internal/core/migration/migrate_test.go | grep -A 3 -B 1 "ErrorContains\|errors\.Is\|errors\.As\|err\.Error()" | head -40
```

Repository: compozy/compozy

Length of output: 438

---

🏁 Script executed:

```shell
fd -name "assert*" -o -name "*helper*" -o -name "*util*" internal/ | grep -i test
```

Repository: compozy/compozy

Length of output: 289

---

🏁 Script executed:

```shell
rg "var.*Error.*=" internal/core/migration/ -B 2 -A 2
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
rg "var \(" internal/ --type go -A 5 | grep -A 5 "Error"
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
rg "errors\." internal/ --type go | grep -v "errors.New\|errors.Is\|errors.As\|errors.Unwrap" | head -20
```

Repository: compozy/compozy

Length of output: 770

---

🏁 Script executed:

```shell
cat -n internal/core/migration/migrate_test.go | sed -n '525,565p'
```

Repository: compozy/compozy

Length of output: 1688

---

🏁 Script executed:

```shell
cat -n internal/core/migration/workflow_target.go
```

Repository: compozy/compozy

Length of output: 3403

---



</details>

**Use typed error matching instead of string matching.**

Line 39 uses `strings.Contains(err.Error(), ...)` which violates the coding guidelines: "Use `errors.Is()` and `errors.As()` for error matching; do not compare error strings." To fix this, define a sentinel error for the invalid workflow name validation failure in `normalizeWorkflowName` (line 105), wrap the error with `fmt.Errorf("%w", err)`, and assert with `errors.Is()` in the test instead of string matching.

Note: This same pattern appears in `migrate_test.go` at lines 530 and 560, indicating a codebase-wide refactoring opportunity.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/migration/workflow_target_test.go` around lines 39 - 40,
Replace string-based error checks by introducing a sentinel error (e.g.,
ErrInvalidWorkflowName) in the package, have normalizeWorkflowName wrap/return
that sentinel (use fmt.Errorf("%w: %s", ErrInvalidWorkflowName, details) or
similar) so callers can detect the failure, and update the test in
workflow_target_test.go to assert with errors.Is(err, ErrInvalidWorkflowName)
instead of strings.Contains(err.Error(), ...); apply the same
replace-for-string-match change in the other tests noted in migrate_test.go that
check the same validation.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:8a265f76-06eb-4087-817e-00119a58516d -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55T4Lp`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55T4Lp
```

---
*Generated from PR review - CodeRabbit AI*

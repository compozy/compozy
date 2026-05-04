---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/core/workspace/config_validate.go
line: 194
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22EG,comment:PRRC_kwDORy7nkc68_V6a
---

# Issue 009: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
cat -n internal/core/workspace/config_validate.go | head -250 | tail -100
```

Repository: compozy/compozy

Length of output: 4005

---

🏁 Script executed:

```shell
# Get the isEnabled helper definition
rg -B 3 -A 10 "func isEnabled" internal/core/workspace/config_validate.go
```

Repository: compozy/compozy

Length of output: 311

---

🏁 Script executed:

```shell
# Search for all uses of isEnabled in the file
rg -n "isEnabled" internal/core/workspace/config_validate.go
```

Repository: compozy/compozy

Length of output: 302

---

🏁 Script executed:

```shell
# Get broader context around the validateWatchReviews function
rg -B 5 -A 30 "validateWatchReviews" internal/core/workspace/config_validate.go
```

Repository: compozy/compozy

Length of output: 2320

---



</details>

**Consolidate `defaults.auto_commit` nil handling with `isEnabled` helper.**

The current check only rejects explicit `false` values for `defaults.auto_commit`, allowing omitted (nil) values to pass validation. Since the requirement is that `watch_reviews.auto_push` requires `auto_commit` to be true, and omitted values default to false at runtime, the nil case should also be rejected. Use `!isEnabled(defaults.AutoCommit, false)` to treat nil and false consistently with the helper used elsewhere in the file.

<details>
<summary>Suggested change</summary>

```diff
-	if isEnabled(cfg.AutoPush, false) && defaults.AutoCommit != nil && !*defaults.AutoCommit {
+	if isEnabled(cfg.AutoPush, false) && !isEnabled(defaults.AutoCommit, false) {
 		return fmt.Errorf(
 			"%s requires %s to be true",
 			configFieldName(scope, "watch_reviews.auto_push"),
 			configFieldName(scope, "defaults.auto_commit"),
 		)
 	}
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	if isEnabled(cfg.AutoPush, false) && !isEnabled(defaults.AutoCommit, false) {
		return fmt.Errorf(
			"%s requires %s to be true",
			configFieldName(scope, "watch_reviews.auto_push"),
			configFieldName(scope, "defaults.auto_commit"),
		)
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/workspace/config_validate.go` around lines 189 - 194, The
validation currently only rejects explicit false for defaults.AutoCommit but
lets nil pass; update the conditional that guards the error in Validate (where
cfg.AutoPush and defaults.AutoCommit are checked) to use the isEnabled helper
consistently: replace the check "defaults.AutoCommit != nil &&
!*defaults.AutoCommit" with "!isEnabled(defaults.AutoCommit, false)" so nil and
false are treated the same; keep the existing fmt.Errorf call that uses
configFieldName(scope, "watch_reviews.auto_push") and configFieldName(scope,
"defaults.auto_commit") unchanged.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:2805fd45-79e2-44b1-9974-bb5124532ac5 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `watch_reviews.auto_push` currently rejects only explicit `defaults.auto_commit = false`, but omitted `defaults.auto_commit` also behaves as false at runtime and should be rejected consistently.
- Fix plan: Switch the validation to the shared `isEnabled(..., false)` helper and add a regression case in `internal/core/workspace/config_test.go`, which is outside the listed file scope but is the minimal existing test surface for this behavior.

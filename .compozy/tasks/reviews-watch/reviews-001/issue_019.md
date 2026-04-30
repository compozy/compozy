---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/daemon/review_watch_test.go
line: 967
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22Ea,comment:PRRC_kwDORy7nkc68_V61
---

# Issue 019: _⚠️ Potential issue_ | _🔴 Critical_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🔴 Critical_ | _⚡ Quick win_

**Guard `cfg` before first dereference in the execute helper.**

`resolveIssues(...)` dereferences `cfg` internally before the nil-check here, so a nil config would panic instead of returning the intended error.
 

<details>
<summary>Fix nil-check ordering</summary>

```diff
 return func(ctx context.Context, preparation *model.SolvePreparation, cfg *model.RuntimeConfig) error {
+	if cfg == nil {
+		return errors.New("runtime config is required")
+	}
 	if err := resolveIssues(ctx, preparation, cfg); err != nil {
 		return err
 	}
-	if cfg == nil {
-		return errors.New("runtime config is required")
-	}
 	reviewsDir := cfg.ReviewsDir
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	return func(ctx context.Context, preparation *model.SolvePreparation, cfg *model.RuntimeConfig) error {
		if cfg == nil {
			return errors.New("runtime config is required")
		}
		if err := resolveIssues(ctx, preparation, cfg); err != nil {
			return err
		}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/review_watch_test.go` around lines 961 - 967, In the execute
helper function, move the nil-check for cfg (model.RuntimeConfig) to before
calling resolveIssues so you don't dereference a nil pointer; i.e., first return
errors.New("runtime config is required") when cfg == nil, then call
resolveIssues(ctx, preparation, cfg) and handle its error, referencing the
anonymous execute func and the resolveIssues call to locate the change.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:fd59a763-64a2-4d66-ae65-58d4eb6327b4 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: The helper used by the temp-git review-watch test calls `resolveIssues(ctx, preparation, cfg)` before checking whether `cfg` is nil, so a nil config would panic instead of returning the intended validation error.
- Fix plan: Move the nil guard before `resolveIssues` and keep the helper behavior otherwise unchanged.

---
provider: coderabbit
pr: "133"
round: 2
round_created_at: 2026-04-30T21:47:34.803875Z
status: resolved
file: internal/store/globaldb/sync.go
line: 254
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-3Yf5,comment:PRRC_kwDORy7nkc69AEIT
---

# Issue 012: _⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_

**Wrap `ListWorkflows` failures with prune context.**

Line 253 returns the error unwrapped, which drops operation context at call sites.

<details>
<summary>💡 Suggested fix</summary>

```diff
  workflows, err := g.ListWorkflows(ctx, ListWorkflowsOptions{WorkspaceID: trimmedWorkspaceID})
  if err != nil {
- 	return WorkflowPruneResult{}, err
+ 	return WorkflowPruneResult{}, fmt.Errorf(
+ 		"globaldb: list workflows for prune workspace %q: %w",
+ 		trimmedWorkspaceID,
+ 		err,
+ 	)
  }
```
</details>

 

As per coding guidelines, "Prefer explicit error returns with wrapped context using `fmt.Errorf("context: %w", err)`".

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	workflows, err := g.ListWorkflows(ctx, ListWorkflowsOptions{WorkspaceID: trimmedWorkspaceID})
	if err != nil {
		return WorkflowPruneResult{}, fmt.Errorf(
			"globaldb: list workflows for prune workspace %q: %w",
			trimmedWorkspaceID,
			err,
		)
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/store/globaldb/sync.go` around lines 251 - 254, Wrap the error
returned by g.ListWorkflows with pruning context before returning so callers
retain operation information; replace the current bare return of err in the
WorkflowPruneResult path (call to g.ListWorkflows with
ListWorkflowsOptions{WorkspaceID: trimmedWorkspaceID}) with a wrapped error
using fmt.Errorf, e.g. include text like "failed to list workflows for prune
(workspace=<trimmedWorkspaceID>): %w" to preserve the original error.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:fac9bd05-da51-494b-b380-cc1fbefdcd70 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `UNREVIEWED`
- Notes:

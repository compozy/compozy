---
provider: coderabbit
pr: "201"
round: 1
round_created_at: 2026-06-15T18:05:13.931425Z
status: resolved
file: internal/daemon/run_snapshot.go
line: 191
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6Jqhq6,comment:PRRC_kwDORy7nkc7LlP4L
---

# Issue 021: _⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_

**Remove the metadata gate before task identity extraction.**

`jobQueuedTaskNumber` returns `0` when both `TaskTitle` and `TaskType` are empty, even if `CodeFile`/`CodeFiles` contain a valid identity (for example `task_15`). That drops valid task numbers for sparse queued payloads.

<details>
<summary>💡 Proposed fix</summary>

```diff
 func jobQueuedTaskNumber(payload kinds.JobQueuedPayload) int {
 	if payload.TaskNumber > 0 {
 		return payload.TaskNumber
 	}
-	if strings.TrimSpace(payload.TaskTitle) == "" && strings.TrimSpace(payload.TaskType) == "" {
-		return 0
-	}
 	if number := tasks.ExtractTaskIdentityNumber(payload.CodeFile); number > 0 {
 		return number
 	}
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func jobQueuedTaskNumber(payload kinds.JobQueuedPayload) int {
	if payload.TaskNumber > 0 {
		return payload.TaskNumber
	}
	if number := tasks.ExtractTaskIdentityNumber(payload.CodeFile); number > 0 {
		return number
	}
	for _, codeFile := range payload.CodeFiles {
		if number := tasks.ExtractTaskIdentityNumber(codeFile); number > 0 {
			return number
		}
	}
	return 0
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/daemon/run_snapshot.go` around lines 175 - 191, The
jobQueuedTaskNumber function returns 0 prematurely when both TaskTitle and
TaskType are empty, preventing it from attempting to extract task identity
numbers from CodeFile and CodeFiles. Remove the early return gate that checks if
both TaskTitle and TaskType are empty strings (the condition that returns 0
before the ExtractTaskIdentityNumber calls). This allows the function to proceed
and extract valid task numbers from CodeFile and CodeFiles even when the
metadata fields are absent.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk -->

<!-- cr-comment:v1:0a3c32313ce719572d58a01d -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Root cause: `jobQueuedTaskNumber` exits before checking `CodeFile` and `CodeFiles` whenever `TaskTitle` and `TaskType` are empty. Sparse queued payloads can still contain canonical task identities such as `task_15` in code-file fields, so the snapshot summary can lose a valid task number.
- Fix approach: remove the metadata gate and keep the existing precedence: explicit `TaskNumber`, then identity extraction from `CodeFile`, then `CodeFiles`.
- Test coverage: add a snapshot-builder regression proving a queued event with no title/type still derives the task number from sparse code-file metadata.

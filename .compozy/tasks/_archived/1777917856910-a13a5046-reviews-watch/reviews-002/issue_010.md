---
provider: coderabbit
pr: "133"
round: 2
round_created_at: 2026-04-30T21:47:34.803875Z
status: resolved
file: internal/store/globaldb/read_queries_test.go
line: 240
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-3Yfx,comment:PRRC_kwDORy7nkc69AEII
---

# Issue 010: _⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_

**Assert map key presence before validating zero counts.**

The resolved-workflow count assertion can pass even if the key is missing, because a missing map entry yields zero values.

<details>
<summary>Suggested adjustment</summary>

```diff
-	if got := counts[pending.Workflow.ID]; got.Total != 2 || got.Completed != 1 || got.Pending != 1 {
+	gotPending, ok := counts[pending.Workflow.ID]
+	if !ok {
+		t.Fatalf("missing pending workflow counts for %q", pending.Workflow.ID)
+	}
+	if gotPending.Total != 2 || gotPending.Completed != 1 || gotPending.Pending != 1 {
-		t.Fatalf("pending task counts = %#v, want total=2 completed=1 pending=1", got)
+		t.Fatalf("pending task counts = %#v, want total=2 completed=1 pending=1", gotPending)
 	}
-	if got := counts[resolved.Workflow.ID]; got.Total != 0 || got.Completed != 0 || got.Pending != 0 {
-		t.Fatalf("resolved task counts = %#v, want zeros", got)
+	gotResolved, ok := counts[resolved.Workflow.ID]
+	if !ok {
+		t.Fatalf("missing resolved workflow counts for %q", resolved.Workflow.ID)
+	}
+	if gotResolved.Total != 0 || gotResolved.Completed != 0 || gotResolved.Pending != 0 {
+		t.Fatalf("resolved task counts = %#v, want zeros", gotResolved)
 	}
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	gotPending, ok := counts[pending.Workflow.ID]
	if !ok {
		t.Fatalf("missing pending workflow counts for %q", pending.Workflow.ID)
	}
	if gotPending.Total != 2 || gotPending.Completed != 1 || gotPending.Pending != 1 {
		t.Fatalf("pending task counts = %#v, want total=2 completed=1 pending=1", gotPending)
	}
	gotResolved, ok := counts[resolved.Workflow.ID]
	if !ok {
		t.Fatalf("missing resolved workflow counts for %q", resolved.Workflow.ID)
	}
	if gotResolved.Total != 0 || gotResolved.Completed != 0 || gotResolved.Pending != 0 {
		t.Fatalf("resolved task counts = %#v, want zeros", gotResolved)
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/store/globaldb/read_queries_test.go` around lines 235 - 240, The
test is asserting zero counts for resolved.Workflow.ID but doesn't verify the
map actually contains that key, so a missing entry would falsely pass; update
the assertion in read_queries_test.go to first check presence (e.g., using the
comma-ok pattern: _, ok := counts[resolved.Workflow.ID]) and fail if !ok, then
proceed to validate counts[resolved.Workflow.ID].Total/Completed/Pending are all
zero to ensure the key exists and values are correct.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:d0f092d4-f42b-498e-943a-5993321058fa -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `UNREVIEWED`
- Notes:

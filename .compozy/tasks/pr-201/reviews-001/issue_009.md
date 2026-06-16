---
provider: coderabbit
pr: "201"
round: 1
round_created_at: 2026-06-15T18:05:13.931425Z
status: resolved
file: internal/core/plan/prepare_test.go
line: 290
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6JqhqN,comment:PRRC_kwDORy7nkc7LlP3S
---

# Issue 009: _⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_

**Avoid zero-value map lookup in the task-number assertion.**

`wantTaskNumbers[job.CodeFiles[0]]` returns `0` for unknown keys, which can accidentally pass for unexpected task IDs when `job.TaskNumber` is also `0`. Assert key presence before comparing.




<details>
<summary>Suggested fix</summary>

```diff
-		if got, want := job.TaskNumber, wantTaskNumbers[job.CodeFiles[0]]; got != want {
-			t.Fatalf("expected task number %d for %s, got %d", want, job.CodeFiles[0], got)
-		}
+		taskID := job.CodeFiles[0]
+		want, ok := wantTaskNumbers[taskID]
+		if !ok {
+			t.Fatalf("unexpected task id %q in prepared jobs", taskID)
+		}
+		if got := job.TaskNumber; got != want {
+			t.Fatalf("expected task number %d for %s, got %d", want, taskID, got)
+		}
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/core/plan/prepare_test.go` around lines 288 - 290, The comparison
using wantTaskNumbers[job.CodeFiles[0]] relies on implicit zero-value semantics
for missing keys, which can mask bugs when job.TaskNumber is also 0. Instead,
use the two-value form of map lookup to explicitly verify that the key exists in
the wantTaskNumbers map before asserting the task number. Check that
job.CodeFiles[0] is present as a key in wantTaskNumbers, and only compare values
when the key exists, otherwise fail the test with a clear message indicating the
missing key.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk -->

<!-- cr-comment:v1:fbc0d535e878f19ed2121f5d -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: the task-number assertion reads `wantTaskNumbers[job.CodeFiles[0]]` without checking key presence, so an unexpected task key can silently compare against the zero value.
- Fix approach: use the two-value map lookup and fail with a clear unexpected-task message before comparing the number.

## Resolution

- Resolved with explicit expectation-map validation in the test.
- Verification: `rtk make verify` exited 0 after the code changes.

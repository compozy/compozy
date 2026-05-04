---
provider: coderabbit
pr: "133"
round: 3
round_created_at: 2026-04-30T21:50:44.830324Z
status: resolved
file: internal/daemon/review_watch_git_test.go
line: 88
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-3Yfl,comment:PRRC_kwDORy7nkc69AEH9
---

# Issue 007: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Strengthen error-path assertions to validate the expected failure reason**

These checks only assert non-nil errors. Please assert the specific error content/type (e.g., expected validation message) so tests fail if the wrong path returns an error.

<details>
<summary>Suggested tightening</summary>

```diff
 if err := git.Push(context.Background(), "/repo", "", "feature"); err == nil {
 	t.Fatal("Push() error = nil, want missing target error")
 }
+if err := git.Push(context.Background(), "/repo", "", "feature"); err == nil ||
+	!strings.Contains(err.Error(), "review watch git push requires remote and branch") {
+	t.Fatalf("Push() error = %v, want missing target validation error", err)
+}
```
</details>

 

As per coding guidelines, "MUST have specific error assertions (ErrorContains, ErrorAs)".


Also applies to: 174-185, 225-237

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/review_watch_git_test.go` around lines 86 - 88, Replace the
generic non-nil assertion for the git.Push failure with a specific error
assertion that verifies the expected failure reason (e.g., use
require.ErrorContains or require.ErrorAs / errors.Is to assert the "missing
target" validation message or the concrete error type returned by git.Push);
update the assertion around the git.Push(context.Background(), "/repo", "",
"feature") call to check the specific error text/type and apply the same
tightening to the other similar assertions in this test file (the blocks that
currently only assert err != nil) so tests fail if a different error path is
taken.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:a02def14-ace0-4527-9531-2aec99eb5414 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: several error-path tests in `review_watch_git_test.go` only check for non-nil errors, which does not prove the intended validation or wrapped failure path executed.
- Fix approach: assert stable error content or wrapped sentinel identity for the affected push/state validation paths so the tests pin the expected behavior.
- Resolution: strengthened the git validation assertions and verified the batch with targeted tests plus `make verify`.

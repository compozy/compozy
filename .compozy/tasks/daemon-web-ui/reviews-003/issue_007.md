---
status: resolved
file: internal/daemon/query_helpers_test.go
line: 190
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58j9CL,comment:PRRC_kwDORy7nkc655wcT
---

# Issue 007: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Strengthen the empty-path assertion to avoid false positives.**

On Line 188, this subtest only checks for a non-nil error, so unrelated failures would still pass. Assert the expected error content (or sentinel) explicitly.

<details>
<summary>Suggested change</summary>

```diff
-		if _, err := readMarkdownDir(" "); err == nil {
-			t.Fatal("readMarkdownDir(empty) error = nil, want non-nil")
-		}
+		if _, err := readMarkdownDir(" "); err == nil ||
+			!strings.Contains(err.Error(), "markdown directory is required") {
+			t.Fatalf("readMarkdownDir(empty) error = %v, want markdown directory required", err)
+		}
```
</details>


As per coding guidelines, "MUST have specific error assertions (ErrorContains, ErrorAs)".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/query_helpers_test.go` around lines 185 - 190, Test currently
only checks that readMarkdownDir(" ") returns a non-nil error which can mask
unrelated failures; update the subtest ("Should reject empty markdown directory
paths") to assert a specific error value or message using errors.Is or
t.ErrorContains (e.g., compare against a sentinel ErrEmptyPath if defined, or
assert the error string contains "empty" or "path" so the failure is explicit).
Locate the call to readMarkdownDir in the test and replace the generic err==nil
check with a concrete assertion (errors.Is(err, ErrEmptyPath) or
t.ErrorContains(err, "empty") depending on which sentinel/message exists) to
ensure the test fails only for unexpected errors.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:1e172c4c-97c9-4b45-a610-076679c62973 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
- The empty-path branch in `TestQueryHelperDirectoryAndStatusBranches` only checks for `err != nil`, so a wrong error still satisfies the test.
- Root cause: the regression test is weaker than the helper contract even though `readMarkdownDir()` emits a specific `daemon: markdown directory is required` error for blank paths.
- Fix plan: tighten the subtest to assert the expected error content so unrelated failures do not pass.
- Implemented: the empty-path subtest now asserts the expected `markdown directory is required` message substring.
- Verification:
- `go test ./internal/daemon -run 'TestQueryHelper(ErrorsAndDocumentTitles|DirectoryAndStatusBranches)$' -count=1`
- `make verify`

---
status: resolved
file: internal/daemon/query_helpers_test.go
line: 122
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58j9Bz,comment:PRRC_kwDORy7nkc655wb3
---

# Issue 006: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Test-case label and expected value are inconsistent.**

The case says “title-cased filename” but expects `"design notes"` (lowercase). Please align name vs expectation so intent is unambiguous.

<details>
<summary>Suggested change (label-only)</summary>

```diff
-			name: "Should fall back to a title-cased filename",
+			name: "Should fall back to a normalized filename title",
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
			name: "Should fall back to a normalized filename title",
			path: "design_notes.md",
			kind: "doc",
			body: "no heading",
			want: "design notes",
		},
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/query_helpers_test.go` around lines 117 - 122, The test case
in query_helpers_test.go has a mismatched label vs expectation: the test case
struct with name "Should fall back to a title-cased filename" expects want
"design notes" (lowercase). Update either the test name or the expected value so
they match intent — e.g., change the name to "Should fall back to a lowercased
filename" or change want to "Design Notes" — by editing the test case object
(the struct containing name/path/kind/body/want) so the name and want are
consistent.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:1e172c4c-97c9-4b45-a610-076679c62973 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
- The test case name says "title-cased filename", but `documentTitle()` intentionally falls back to `strings.ReplaceAll(base, "_", " ")`, which yields `design notes`.
- Root cause: the assertion expectation matches the implementation, but the test label no longer describes the actual fallback behavior.
- Fix plan: rename the subtest case so it explicitly describes the normalized filename fallback without changing the production helper semantics.
- Implemented: renamed the case to `Should fall back to the normalized filename`, leaving `documentTitle()` behavior unchanged.
- Verification:
- `go test ./internal/daemon -run 'TestQueryHelper(ErrorsAndDocumentTitles|DirectoryAndStatusBranches)$' -count=1`
- `make verify`

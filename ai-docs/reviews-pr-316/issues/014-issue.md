# Issue 14 - Review Thread Comment

**File:** `sdk/compozy/codegen/generator_test.go:31`
**Date:** 2025-10-31 14:57:18 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🧹 Nitpick_ | _🔵 Trivial_

**Consider the maintainability of hard-coded hash validation.**

This test validates generated files by comparing SHA-256 hashes. While this ensures generated code stability, it will fail and require manual hash updates whenever the codegen logic or templates change intentionally.



Consider one of these approaches:
1. **Keep as-is if intentional**: If the goal is to force explicit review of all generated code changes, document this in a comment.
2. **Alternative approach**: Test the generated code's behavior (e.g., that options correctly set fields) rather than its exact bytes.
3. **Semi-automated updates**: Add a make/mage target that regenerates and updates these hashes when codegen changes are intentional.

Example comment to add if keeping this approach:

```diff
 func TestGeneratedFilesHashes(t *testing.T) {
+	// This test validates that generated files remain stable.
+	// When codegen intentionally changes, update hashes by running:
+	// go test -v -run TestGeneratedFilesHashes and copying the "got" hashes.
 	files := map[string]string{
```

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In sdk/compozy/codegen/generator_test.go around lines 11 to 31, the test uses
hard-coded SHA-256 hashes which will break whenever codegen or templates change;
either (A) retain the strict byte-level checks but add a clear comment
explaining this is intentional to force manual review (and document the workflow
for updating hashes), or (B) replace the hash assertions with behavior-level
tests that validate generated code semantics (e.g., compile the generated
package and run/reflection tests that ensure options set fields and exported
symbols exist), or (C) keep the hashes but add a test helper/make/mage target
that regenerates and writes updated hashes to this test file so maintainers can
semi-automate intentional updates — implement one of these approaches and update
the test accordingly.
```

</details>

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gJFEm`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gJFEm
```

---
*Generated from PR review - CodeRabbit AI*

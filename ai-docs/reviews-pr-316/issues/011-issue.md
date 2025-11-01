# Issue 11 - Review Thread Comment

**File:** `sdk/compozy/codegen/generator_test.go:34`
**Date:** 2025-11-01 01:57:02 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Refactor to use `t.Run()` subtests per coding guidelines.**

The test iterates over multiple files but doesn't use subtests. Per the coding guidelines, all tests must use `t.Run("Should describe behavior", ...)` subtests for each behavior being validated.

Using subtests will:
- Allow all files to be validated even if one fails
- Provide clearer test output showing which specific file failed
- Enable parallel execution if needed
- Align with project testing patterns



As per coding guidelines.

Apply this refactor:

```diff
 func TestGeneratedFilesHashes(t *testing.T) {
 	// This test locks generated outputs to make intentional template changes explicit.
 	// When updates are expected, run `go test -run TestGeneratedFilesHashes -v`
 	// and refresh the hashes from the failure output.
 	files := map[string]string{
 		"options_generated.go":   "c827fddefb3ca3a92e9148f83b8ddab434033e3a0b015877ae5686c5312a5a60",
 		"engine_execution.go":    "4a398c36ef0d122a0fa10e4b2deaa4948d524ccaf809264b3ded3ca8ebaa32da",
 		"engine_loading.go":      "4c67e1323d10f6ebf6d270e4838ae57058fd70488d5930f968e6e79be7bf4058",
 		"engine_registration.go": "d560f5c93bbe9557050fe3066df8a2e855cf0bfee20d7275b844c8d07b877596",
 	}
 	root := filepath.Clean("..")
 	for name, expected := range files {
-		path := filepath.Join(root, name)
-		data, err := os.ReadFile(path)
-		if err != nil {
-			t.Fatalf("read %s: %v", name, err)
-		}
-		sum := sha256.Sum256(data)
-		hash := hex.EncodeToString(sum[:])
-		if hash != expected {
-			t.Fatalf("unexpected hash for %s: got %s want %s", name, hash, expected)
-		}
+		t.Run("Should validate hash for "+name, func(t *testing.T) {
+			path := filepath.Join(root, name)
+			data, err := os.ReadFile(path)
+			if err != nil {
+				t.Fatalf("read %s: %v", name, err)
+			}
+			sum := sha256.Sum256(data)
+			hash := hex.EncodeToString(sum[:])
+			if hash != expected {
+				t.Fatalf("unexpected hash for %s: got %s want %s", name, hash, expected)
+			}
+		})
 	}
 }
```

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In sdk/compozy/codegen/generator_test.go around lines 11 to 34, the table-driven
loop checks multiple files but does not use t.Run subtests as required; refactor
by replacing the direct loop body with a t.Run(fmt.Sprintf("file=%s", name),
func(t *testing.T) { ... }) for each entry, copy loop variables into locals
(e.g., n, exp := name, expected) to avoid closure capture bugs, move the
file-read, hash compute and assertion into that subtest, and optionally call
t.Parallel() inside the subtest if parallelization is desired.
```

</details>

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gLa2g`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gLa2g
```

---
*Generated from PR review - CodeRabbit AI*

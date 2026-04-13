---
status: resolved
file: internal/core/provider/coderabbit/nitpicks_test.go
line: 16
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56V4Zi,comment:PRRC_kwDORy7nkc627G2z
---

# Issue 005: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
set -euo pipefail

python - <<'PY'
import re
from pathlib import Path

path = Path("internal/core/provider/coderabbit/nitpicks_test.go")
src = path.read_text()

for m in re.finditer(r'^func\s+(Test\w+)\(t \*testing\.T\)\s*\{', src, re.M):
    name = m.group(1)
    i = m.end()
    depth = 1
    while i < len(src) and depth:
        if src[i] == '{':
            depth += 1
        elif src[i] == '}':
            depth -= 1
        i += 1
    body = src[m.end():i-1]
    print(f"{name}: {'HAS t.Run(\"Should\")' if 't.Run(\"Should' in body else 'MISSING t.Run(\"Should\")'}")
PY
```

Repository: compozy/compozy

Length of output: 367

---

🏁 Script executed:

```shell
#!/bin/bash
set -euo pipefail

python - <<'PY'
import re
from pathlib import Path

path = Path("internal/core/provider/coderabbit/nitpicks_test.go")
src = path.read_text()

for m in re.finditer(r'^func\s+(Test\w+)\(t \*testing\.T\)\s*\{', src, re.M):
    name = m.group(1)
    i = m.end()
    depth = 1
    while i < len(src) and depth:
        if src[i] == '{':
            depth += 1
        elif src[i] == '}':
            depth -= 1
        i += 1
    body = src[m.end():i-1]
    has_should = 't.Run("Should' in body
    status = "HAS t.Run(\"Should\")" if has_should else "MISSING t.Run(\"Should\")"
    print(f"{name}: {status}")
PY
```

Repository: compozy/compozy

Length of output: 484

---

🏁 Script executed:

```shell
cat -n internal/core/provider/coderabbit/nitpicks_test.go | head -100
```

Repository: compozy/compozy

Length of output: 3901

---



</details>

**Wrap these test cases in `t.Run("Should ...")` subtests to meet repo test conventions.**

Four test functions execute assertions directly at the top level without subtest wrappers. The repo test policy requires the `t.Run("Should...")` pattern for all test cases.

<details>
<summary>♻️ Suggested pattern</summary>

```diff
 func TestParseReviewBodyCommentItemsDeduplicatesHashesAndKeepsNewestReview(t *testing.T) {
 	t.Parallel()
+	t.Run("Should deduplicate by hash and keep newest review", func(t *testing.T) {
+		t.Parallel()

-	sharedTitle := "Prefer reusing existing stop-reason helper..."
+		sharedTitle := "Prefer reusing existing stop-reason helper..."
 	// ... test body ...
-	if queryItem.SourceReviewID != "4090314487" {
-		t.Fatalf("expected newest review id to win, got %q", queryItem.SourceReviewID)
+		if queryItem.SourceReviewID != "4090314487" {
+			t.Fatalf("expected newest review id to win, got %q", queryItem.SourceReviewID)
+		}
+	})
 }
```
</details>

Affected functions: `TestParseReviewBodyCommentItemsDeduplicatesHashesAndKeepsNewestReview`, `TestParseReviewBodyCommentItemsRecognizesMinorAndMajorCategories`, `TestFetchReviewsSkipsPullRequestReviewsWhenNitpicksDisabled`, `TestFetchReviewsIncludesReviewBodyCommentsWhenRequested`.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/provider/coderabbit/nitpicks_test.go` around lines 13 - 16,
Each of the four tests must be converted to use a t.Run("Should ...") subtest
wrapper; for each function named
(TestParseReviewBodyCommentItemsDeduplicatesHashesAndKeepsNewestReview,
TestParseReviewBodyCommentItemsRecognizesMinorAndMajorCategories,
TestFetchReviewsSkipsPullRequestReviewsWhenNitpicksDisabled,
TestFetchReviewsIncludesReviewBodyCommentsWhenRequested) move the current
top-level assertions into a t.Run call with a descriptive "Should ..." name and
run the existing body inside the subtest, calling t.Parallel() inside the
subtest if parallelization is desired; keep all existing setup and assertions
but relocate them into the anonymous func(t *testing.T) passed to t.Run.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:8ade0401-438f-4b37-be5e-ab58a701cf08 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Notes:
  - These tests already exercise single behaviors directly and are not masking a correctness problem.
  - Repository guidance prefers subtests as a default pattern, but it does not require every standalone test to be wrapped in `t.Run("Should ...")`; the current file already mixes both styles.
  - No code change is planned because this is style churn without additional coverage or bug prevention.

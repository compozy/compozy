---
status: resolved
file: internal/cli/run.go
line: 103
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc55LZVU,comment:PRRC_kwDORy7nkc61XmPz
---

# Issue 001: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
# Read the file around the specified lines to verify the issue
head -120 internal/cli/run.go | tail -40
```

Repository: compozy/compozy

Length of output: 861

---

🏁 Script executed:

```shell
# Get broader context to understand the function
sed -n '80,110p' internal/cli/run.go
```

Repository: compozy/compozy

Length of output: 694

---

🏁 Script executed:

```shell
# Check the function signature and full context
sed -n '1,120p' internal/cli/run.go | grep -A 50 "func.*fetchReviews"
```

Repository: compozy/compozy

Length of output: 1237

---



</details>

**Handle the summary write failure.**

The `fmt.Fprintf` call at lines 95–103 explicitly ignores both return values with `_, _`. If `cmd.OutOrStdout()` is closed or broken, the write fails silently and the function still returns `nil` (success), even though the output was never delivered.

<details>
<summary>Suggested fix</summary>

```diff
-	_, _ = fmt.Fprintf(
-		cmd.OutOrStdout(),
-		"Fetched %d review issues from %s for PR %s into %s (round %03d)\n",
-		result.Total,
-		result.Provider,
-		result.PR,
-		result.ReviewsDir,
-		result.Round,
-	)
-	return nil
+	if _, err := fmt.Fprintf(
+		cmd.OutOrStdout(),
+		"Fetched %d review issues from %s for PR %s into %s (round %03d)\n",
+		result.Total,
+		result.Provider,
+		result.PR,
+		result.ReviewsDir,
+		result.Round,
+	); err != nil {
+		return fmt.Errorf("write fetch summary: %w", err)
+	}
+	return nil
```
</details>

This violates the coding guideline: "Never ignore errors with `_` in Go — every error must be handled or have a written justification."

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/run.go` around lines 95 - 103, Replace the ignored return values
from the fmt.Fprintf call that writes the summary (the call using
cmd.OutOrStdout() with
result.Total/result.Provider/result.PR/result.ReviewsDir/result.Round) with
proper error handling: capture the returned n, err := fmt.Fprintf(...), check if
err != nil and return or wrap that error (or log and return) so the function
does not silently succeed when stdout is closed or the write fails.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:6c29911c-ba13-4d74-ad6e-790b2357b234 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  The current `fetchReviews` path ignores the summary write error with `_, _ = fmt.Fprintf(...)`, so a broken stdout still returns success after the fetch completes. This is a real error-handling gap in `internal/cli/run.go`, and the fix is to return a wrapped write error from the existing summary print.
  Resolution: implemented and verified with the focused Go test run plus `make verify`.

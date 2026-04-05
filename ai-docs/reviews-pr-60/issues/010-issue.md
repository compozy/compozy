# Issue 10 - Review Thread Comment

**File:** `internal/core/run/exec_flow.go:1056`
**Date:** 2026-04-05 19:22:18 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** VALID

**Rationale:** Go’s layout syntax treats the trailing `000000000` in the current format string as literal characters, so the generated IDs only vary at second granularity. The formatter needs a real nanosecond suffix to avoid collisions inside the same second.

## Body

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🌐 Web query:

`In Go's time.Format layout syntax, does "20060102-150405-000000000" print nanoseconds, or are those trailing zeros literal without a decimal point?`

💡 Result:

In Go's time.Format layout syntax, the trailing "000000000" in "20060102-150405-000000000" are literal characters, not nanoseconds. Nanoseconds (fractional seconds) require a decimal point (or comma) prefix followed by zeros or nines, e.g., ".000000000" for fixed 9-digit nanoseconds or ".999999999" for variable with trailing zeros removed. Without the decimal point, sequences of digits like "000000000" are treated as literal text to be printed verbatim, not interpreted as time components.

Citations:

- 1: https://go.dev/src/time/format.go
- 2: https://pkg.go.dev/time@go1.20.7
- 3: https://pkg.go.dev/time@go1.25.5
- 4: https://stackoverflow.com/questions/66770720/how-can-i-parse-a-time-in-go-with-no-period-separating-seconds-and-milliseconds

---



</details>

**Run ID generation can collide within the same second.**

Line 1055 uses `Format("20060102-150405-000000000")`. In Go's time layout syntax, fractional seconds require a decimal point prefix (e.g., `.000000000`). Without it, the trailing zeros are literal characters, not nanosecond formatting. This reduces ID uniqueness to second-level granularity, causing collisions if multiple IDs are generated within the same second.

<details>
<summary>Proposed fix</summary>

```diff
 func buildExecRunID() string {
-	return fmt.Sprintf("exec-%s", time.Now().UTC().Format("20060102-150405-000000000"))
+	now := time.Now().UTC()
+	return fmt.Sprintf("exec-%s-%09d", now.Format("20060102-150405"), now.Nanosecond())
 }
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func buildExecRunID() string {
	now := time.Now().UTC()
	return fmt.Sprintf("exec-%s-%09d", now.Format("20060102-150405"), now.Nanosecond())
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/exec_flow.go` around lines 1054 - 1056, The run ID
generator buildExecRunID currently uses time.Format("20060102-150405-000000000")
so the trailing zeros are literal and IDs collide within the same second; update
buildExecRunID to include fractional seconds by using the correct Go layout
(e.g., "20060102-150405.000000000") or alternatively append
time.Now().UTC().UnixNano() (or a short hex/base36 of it) to the ID so multiple
IDs generated in the same second are unique; modify the buildExecRunID function
and its fmt.Sprintf call accordingly.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:443cb2f9-9289-4e80-a9c6-9308c8d22d24 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc547tHo`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc547tHo
```

---
*Generated from PR review - CodeRabbit AI*

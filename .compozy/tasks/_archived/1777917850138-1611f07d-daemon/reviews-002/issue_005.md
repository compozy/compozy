---
status: resolved
file: internal/api/core/sse_test.go
line: 135
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579yyG,comment:PRRC_kwDORy7nkc65HZWW
---

# Issue 005: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Assert the overflow resume cursor too.**

`core.OverflowMessage` includes the last cursor as part of the payload, but this case only checks `run_id` and `reason`. If `cursor` is dropped from overflow frames, this test still passes even though clients lose the resume point after an overflow.

<details>
<summary>Suggested assertion</summary>

```diff
 		{
 			name:    "overflow",
 			message: core.OverflowMessage("run-1", cursor, timestamp.Add(2*time.Second), "slow consumer"),
 			want: []string{
 				"event: overflow",
 				`"run_id":"run-1"`,
+				`"cursor":"` + core.FormatCursor(cursor.Timestamp, cursor.Sequence) + `"`,
 				`"reason":"slow consumer"`,
 			},
 		},
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
		{
			name:    "overflow",
			message: core.OverflowMessage("run-1", cursor, timestamp.Add(2*time.Second), "slow consumer"),
			want: []string{
				"event: overflow",
				`"run_id":"run-1"`,
				`"cursor":"` + core.FormatCursor(cursor.Timestamp, cursor.Sequence) + `"`,
				`"reason":"slow consumer"`,
			},
		},
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/core/sse_test.go` around lines 128 - 135, The "overflow" test
case for core.OverflowMessage is missing an assertion that the payload includes
the resume cursor; add a check that the SSE frame contains the cursor value
produced earlier (the second argument passed to core.OverflowMessage, e.g.,
cursor.String() or its serialized form) so the test verifies the resume cursor
is present alongside "run_id" and "reason" in the want expectations for the
"overflow" case.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:34d8a7c1-5aaf-4e7e-85b1-924950384777 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: the overflow SSE test asserts `run_id` and `reason` but not the serialized resume cursor, so a regression that drops `cursor` from overflow frames would not fail the test.
- Fix plan: extend the overflow expectations to require the formatted cursor alongside the existing fields.
- Resolution: `internal/api/core/sse_test.go` now asserts the overflow cursor explicitly. Verification also exposed a real SSE transport hardening gap, so `internal/api/core/sse.go` now rejects short writes and the live stream handler test was narrowed to frame-emission coverage while payload content remains asserted in `sse_test.go`.

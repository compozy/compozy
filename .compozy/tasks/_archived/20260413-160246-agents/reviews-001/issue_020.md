---
status: resolved
file: pkg/compozy/events/kinds/reusable_agent.go
line: 53
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5uE,comment:PRRC_kwDORy7nkc62zc81
---

# Issue 020: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Drop `omitempty` from state fields whose zero values are meaningful.**

`nested_depth: 0`, `max_nested_depth`, `success: false`, and `blocked: false` are real lifecycle states, but the current tags erase them from the JSON payload. That makes downstream event consumers guess whether the field was intentionally set or never populated.

<details>
<summary>Suggested contract fix</summary>

```diff
-	NestedDepth       int                         `json:"nested_depth,omitempty"`
-	MaxNestedDepth    int                         `json:"max_nested_depth,omitempty"`
+	NestedDepth       int                         `json:"nested_depth"`
+	MaxNestedDepth    int                         `json:"max_nested_depth"`
 	OutputRunID       string                      `json:"output_run_id,omitempty"`
-	Success           bool                        `json:"success,omitempty"`
-	Blocked           bool                        `json:"blocked,omitempty"`
+	Success           bool                        `json:"success"`
+	Blocked           bool                        `json:"blocked"`
 	BlockedReason     ReusableAgentBlockedReason  `json:"blocked_reason,omitempty"`
```
</details>


Based on learnings "Prioritize system boundaries and ownership - ensure clear ownership and API contracts between system components".

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	NestedDepth       int                         `json:"nested_depth"`
	MaxNestedDepth    int                         `json:"max_nested_depth"`
	OutputRunID       string                      `json:"output_run_id,omitempty"`
	Success           bool                        `json:"success"`
	Blocked           bool                        `json:"blocked"`
	BlockedReason     ReusableAgentBlockedReason  `json:"blocked_reason,omitempty"`
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@pkg/compozy/events/kinds/reusable_agent.go` around lines 48 - 53, The JSON
tags on the ReusableAgent event struct currently use `omitempty` for fields
whose zero values are meaningful; remove `omitempty` from NestedDepth,
MaxNestedDepth, Success, and Blocked (fields named NestedDepth, MaxNestedDepth,
Success, Blocked in reusable_agent.go) so zero/false values are preserved in
serialized payloads; update the struct tags to plain `json:"nested_depth"`,
`json:"max_nested_depth"`, `json:"success"`, and `json:"blocked"` and run
tests/serialization checks to ensure downstream consumers receive explicit
zero/false values.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:d740b4bc-0bac-4faf-9dba-d2618b9a24f6 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `omitempty` was suppressing meaningful zero/false lifecycle fields such as `nested_depth: 0`, `max_nested_depth: 0`, `success: false`, and `blocked: false`.
- Fix: Removed `omitempty` from those fields and added serialization coverage to prove the zero/false values remain present in JSON payloads.
- Evidence: `go test ./pkg/compozy/events/...`

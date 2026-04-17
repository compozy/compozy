---
status: resolved
file: pkg/compozy/events/event_test.go
line: 142
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc57ypzc,comment:PRRC_kwDORy7nkc644MtH
---

# Issue 012: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Rename new subtest cases to the required `Should...` format.**

The added case names (`"job queued"`, `"job started"`) violate the enforced test naming rule.

<details>
<summary>Suggested rename</summary>

```diff
-			name: "job queued",
+			name: "Should round-trip job queued payload with runtime fields",
...
-			name: "job started",
+			name: "Should round-trip job started payload with runtime fields",
```
</details>


As per coding guidelines, "MUST use t.Run(\"Should...\") pattern for ALL test cases".


Also applies to: 159-160

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@pkg/compozy/events/event_test.go` around lines 141 - 142, Rename the failing
subtest names that use plain strings to the required t.Run("Should...") pattern:
update the subtest names currently written as "job queued" and "job started"
(and the other case at lines ~159-160) to descriptive "Should..." names (e.g.,
"Should queue job" and "Should start job") wherever t.Run is invoked in the
event tests that construct kinds.JobQueuedPayload / kinds.JobStartedPayload so
they comply with the enforced naming rule.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:42b35b9b-175e-4036-827f-7ad498ee62ed -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. The newly added event round-trip cases use plain names like `"job queued"` and `"job started"` even though the table feeds directly into `t.Run(tc.name, ...)`.
  - Root cause: the new runtime-payload cases were added with descriptive strings but not the required `Should...` convention enforced in this repository.
  - Intended fix: rename the new case labels to descriptive `Should...` names while leaving the test behavior unchanged.
  - Resolution: the new event round-trip case names now use descriptive `Should...` labels.

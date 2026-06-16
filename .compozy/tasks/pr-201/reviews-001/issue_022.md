---
provider: coderabbit
pr: "201"
round: 1
round_created_at: 2026-06-15T18:05:13.931425Z
status: resolved
file: internal/store/rundb/run_db.go
line: 1613
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6JqhrB,comment:PRRC_kwDORy7nkc7LlP4Z
---

# Issue 022: _⚠️ Potential issue_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_ | _⚡ Quick win_

**Pause/resume projections can be keyed under empty `job_id` when `Index == 0`.**

The new pause/resume projections derive IDs with `jobIDFromIndex(payload.Index, "")`. That helper returns an empty string for index `0`, so these events can upsert/read under `job_id=""` instead of the queued job key, breaking state continuity.

<details>
<summary>💡 Proposed fix</summary>

```diff
 func jobIDFromIndex(index int, safeName string) string {
 	if trimmed := strings.TrimSpace(safeName); trimmed != "" {
 		return trimmed
 	}
-	if index > 0 {
+	if index >= 0 {
 		return fmt.Sprintf("job-%03d", index)
 	}
 	return ""
 }
```
</details>

   


Also applies to: 1695-1708

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/store/rundb/run_db.go` around lines 1591 - 1613, The three functions
projectJobPausingState, projectJobPausedState, and projectJobResumedState all
call jobIDFromIndex(payload.Index, "") with an empty string as the second
argument, which results in an empty job_id when Index equals 0, breaking state
continuity. Fix each function by providing a non-empty fallback value to the
second parameter of jobIDFromIndex instead of an empty string, ensuring these
pause/resume events are correctly keyed under the proper job ID regardless of
the Index value. Apply the same fix to any other similar functions in the
indicated line range that have the same pattern.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk -->

<!-- cr-comment:v1:c6e2534f235166f8be6b7834 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Root cause: `jobIDFromIndex` treats `0` as missing, but job indexes are zero-based across run events. Lifecycle events without `SafeName`, including pause/resume events, can therefore project under `job_id=""` instead of the canonical `job-000` key and split persisted state from the queued row.
- Fix approach: correct the central helper so every non-negative index maps to `job-%03d` when no safe name is provided; keep negative indexes as invalid/empty.
- Test coverage: add a run DB projection regression proving an index-zero queued job and following pause/resume lifecycle events update one `job-000` row rather than creating an empty-key row.

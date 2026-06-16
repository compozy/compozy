---
provider: coderabbit
pr: "201"
round: 1
round_created_at: 2026-06-15T18:05:13.931425Z
status: resolved
file: internal/core/model/job_control.go
line: 168
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6Jqhpy,comment:PRRC_kwDORy7nkc7LlP2v
---

# Issue 006: _⚠️ Potential issue_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_ | _⚡ Quick win_

**Enforce request identity in normalized job-control responses.**

`completeJobControlResponse` only fills `RunID`/`JobID` when empty, so a controller can return mismatched IDs and leak inconsistent API contract values back to callers. Normalize these fields unconditionally from the request.

<details>
<summary>Proposed fix</summary>

```diff
 func completeJobControlResponse(resp JobControlResponse, req JobControlRequest) JobControlResponse {
-	if resp.RunID == "" {
-		resp.RunID = req.RunID
-	}
-	if resp.JobID == "" {
-		resp.JobID = req.JobID
-	}
+	resp.RunID = req.RunID
+	resp.JobID = req.JobID
 	if req.Index >= 0 {
 		resp.Index = req.Index
 	} else if resp.Index < 0 {
 		resp.Index = req.Index
 	}
 	return resp
 }
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/core/model/job_control.go` around lines 157 - 168, In the
completeJobControlResponse function, the RunID and JobID fields are
conditionally assigned only when empty, which allows mismatched values to
propagate. Remove the conditional checks for empty values and unconditionally
assign resp.RunID = req.RunID and resp.JobID = req.JobID to ensure the response
always contains the normalized request identities, enforcing the API contract
regardless of what the controller initially returned.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk -->

<!-- cr-comment:v1:f51d47fabe17b411ea042c9b -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `completeJobControlResponse` only backfills request `RunID`/`JobID` when the controller leaves them empty, allowing a controller to return mismatched identifiers to API callers.
- Fix approach: normalize response identity unconditionally from the request and add regression coverage that a mismatched controller response is overwritten.

## Resolution

- Resolved by normalizing controller response identity from the validated request.
- Verification: `rtk make verify` exited 0 after the code changes.

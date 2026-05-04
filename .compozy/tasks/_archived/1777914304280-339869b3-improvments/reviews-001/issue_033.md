---
status: resolved
file: internal/daemon/run_transcript.go
line: 149
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUlq,comment:PRRC_kwDORy7nkc68K-Qr
---

# Issue 033: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Keep aggregated session metadata in sync with the newest revision.**

`result.Revision` is promoted to the latest job snapshot, but `Session.Status` and `Session.CurrentModeID` are still first-write-wins. If a later job carries a newer revision, the transcript can return stale session metadata alongside a newer revision number.

<details>
<summary>💡 Proposed fix</summary>

```diff
-		if session.Revision > result.Revision {
-			result.Revision = session.Revision
-		}
+		newerRevision := session.Revision > result.Revision
+		if newerRevision {
+			result.Revision = session.Revision
+		}
 		result.Plan.PendingCount += session.Plan.PendingCount
 		result.Plan.RunningCount += session.Plan.RunningCount
 		result.Plan.DoneCount += session.Plan.DoneCount
 		result.Plan.Entries = append(result.Plan.Entries, session.Plan.Entries...)
-		if result.Session.Status == "" {
+		if newerRevision || result.Session.Status == "" {
 			result.Session.Status = session.Session.Status
 		}
-		if result.Session.CurrentModeID == "" {
+		if newerRevision || result.Session.CurrentModeID == "" {
 			result.Session.CurrentModeID = session.Session.CurrentModeID
 		}
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
		session := job.Summary.Session
		newerRevision := session.Revision > result.Revision
		if newerRevision {
			result.Revision = session.Revision
		}
		result.Plan.PendingCount += session.Plan.PendingCount
		result.Plan.RunningCount += session.Plan.RunningCount
		result.Plan.DoneCount += session.Plan.DoneCount
		result.Plan.Entries = append(result.Plan.Entries, session.Plan.Entries...)
		if newerRevision || result.Session.Status == "" {
			result.Session.Status = session.Session.Status
		}
		if newerRevision || result.Session.CurrentModeID == "" {
			result.Session.CurrentModeID = session.Session.CurrentModeID
		}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/run_transcript.go` around lines 136 - 149, The code updates
result.Revision to the newest job snapshot but leaves result.Session.Status and
result.Session.CurrentModeID as first-write-wins; modify the aggregation in
run_transcript.go so that when session.Revision > result.Revision (i.e., the
job.Summary.Session is newer) you also overwrite result.Session.Status and
result.Session.CurrentModeID from job.Summary.Session (and any other
session-level metadata that must reflect the newest snapshot) instead of
preserving the earlier values; ensure the overwrite happens in the same branch
where result.Revision is promoted.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:4021176c-ee24-47e4-a6f3-a97e632a8084 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: Confirmed transcript aggregation promoted `Revision` from newer job snapshots but left session status and mode as first-write-wins. Updated aggregation to refresh session metadata when a newer revision is observed and added a regression test.

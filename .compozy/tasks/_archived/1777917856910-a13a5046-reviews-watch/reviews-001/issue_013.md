---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/daemon/review_watch.go
line: 1048
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4208391746,nitpick_hash:92b70afd568a
review_hash: 92b70afd568a
source_review_id: "4208391746"
source_review_submitted_at: "2026-04-30T20:37:25Z"
---

# Issue 013: Simplify redundant reservation check.
## Review Comment

Line 1055 checks both `runID != ""` and `keyReserved(...)`. Since `keyReserved` already checks if the key exists in the map, and an empty string value is used for reservations (line 1069), the `runID != ""` check is redundant for the reservation case (though useful for the "already active with run" case).

The logic is correct but slightly confusing. Consider a small refactor for clarity:

```diff
- if runID := strings.TrimSpace(m.activeReviewWatches[key]); runID != "" || keyReserved(m.activeReviewWatches, key) {
+ if existingRunID, reserved := m.activeReviewWatches[key]; reserved {
return apicore.NewProblem(
http.StatusConflict,
"review_watch_already_active",
"review watch is already active for this workspace, provider, and PR",
map[string]any{
"workspace_id": key.WorkspaceID,
"provider": key.Provider,
"pr": key.PR,
- "run_id": runID,
+ "run_id": existingRunID,
},
nil,
)
}
```

## Triage

- Decision: `invalid`
- Reasoning: The current reservation logic is behaviorally correct. It intentionally reports the active watch conflict both when a slot is reserved with a blank run id and when a concrete run id is already attached, and the existing code makes that distinction explicit at the call site.
- Why no fix: The suggested rewrite is a readability preference only; it does not address a defect, contract violation, or diagnosability gap worth changing in this scoped remediation batch.

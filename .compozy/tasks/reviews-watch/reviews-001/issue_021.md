---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/daemon/task_transport_service.go
line: 77
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22Eg,comment:PRRC_kwDORy7nkc68_V6-
---

# Issue 021: _⚠️ Potential issue_ | _🟠 Major_ | _🏗️ Heavy lift_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_ | _🏗️ Heavy lift_

**This turns workflow listing into an N+1 query path.**

For every workflow row, the loop now does a `ListTaskItems` call plus another archive-eligibility lookup. On larger workspaces this endpoint becomes O(n) database round-trips and will slow down noticeably. Consider folding task counts and archive eligibility into the initial list query, or fetching both in bulk before the loop.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/task_transport_service.go` around lines 67 - 77, The current
loop in task_transport_service.go causes N+1 DB calls by calling
s.globalDB.ListTaskItems and attachWorkflowArchiveEligibility for each workflow
row; fix this by fetching task rows and archive-eligibility in bulk before the
loop (e.g., add or use a DB method like ListTaskItemsForWorkflowIDs(ctx, ids) to
get all task rows keyed by workflow ID and a bulk
attachWorkflowArchiveEligibilityBulk(ctx, db, rows) or a single query that
returns archive eligibility per workflow), then iterate rows and call
transportWorkflowSummaryWithTaskCounts(row,
summarizeTaskRows(bulkTaskMap[row.ID])) and set the precomputed eligibility from
the bulk result instead of per-row DB calls.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:96e4d5e8-33bf-4fb0-b3f8-c84c4be01780 -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: `ListWorkflows` currently performs one `ListTaskItems` query and one archive-eligibility query per workflow row, so the reviewer is correct about the N+1 path. I will replace the per-row reads with bulk task-count and archive-eligibility lookups, which requires a minimal `globaldb` read-path extension beyond the originally listed file scope.
- Resolution: Replaced the per-workflow calls with bulk `globaldb` task-count and archive-eligibility reads, then verified the updated transport path with focused tests and the full `make verify` pipeline.

---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/daemon/task_transport_service.go
line: 103
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22Ek,comment:PRRC_kwDORy7nkc68_V7D
---

# Issue 022: _⚠️ Potential issue_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_ | _⚡ Quick win_

**Populate the derived workflow fields on the single-workflow path too.**

`ListWorkflows` now returns the task-count/start-block metadata via `transportWorkflowSummaryWithTaskCounts`, but `GetWorkflow` still builds the summary with `transportWorkflowSummary`, so those derived fields stay empty here. This makes the single-item endpoint disagree with the list endpoint for the same workflow.


<details>
<summary>Suggested fix</summary>

```diff
+ taskRows, err := s.globalDB.ListTaskItems(ctx, row.ID)
+ if err != nil {
+ 	return apicore.WorkflowSummary{}, err
+ }
- summary := transportWorkflowSummary(row)
+ summary := transportWorkflowSummaryWithTaskCounts(row, summarizeTaskRows(taskRows))
  if err := attachWorkflowArchiveEligibility(ctx, s.globalDB, row, &summary); err != nil {
  	return apicore.WorkflowSummary{}, err
  }
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	taskRows, err := s.globalDB.ListTaskItems(ctx, row.ID)
	if err != nil {
		return apicore.WorkflowSummary{}, err
	}
	summary := transportWorkflowSummaryWithTaskCounts(row, summarizeTaskRows(taskRows))
	if err := attachWorkflowArchiveEligibility(ctx, s.globalDB, row, &summary); err != nil {
		return apicore.WorkflowSummary{}, err
	}
	return summary, nil
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/task_transport_service.go` around lines 99 - 103, GetWorkflow
builds the summary using transportWorkflowSummary which omits the derived
task-count/start-block fields; replace that call with
transportWorkflowSummaryWithTaskCounts (and keep the existing
attachWorkflowArchiveEligibility call) so the single-workflow path populates the
same derived fields as ListWorkflows; update the code in
task_transport_service.go to create summary via
transportWorkflowSummaryWithTaskCounts(ctx, row, s.globalDB or same params used
by the list path) and then call attachWorkflowArchiveEligibility(ctx,
s.globalDB, row, &summary) before returning.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:96e4d5e8-33bf-4fb0-b3f8-c84c4be01780 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: `GetWorkflow` still uses `transportWorkflowSummary`, so its `TaskCounts`, `CanStartRun`, and `StartBlockReason` fields diverge from `ListWorkflows`. I will load the workflow task rows on the single-item path and build the summary with `transportWorkflowSummaryWithTaskCounts`, then preserve the existing archive-eligibility attachment.
- Resolution: Updated `GetWorkflow` to build the derived task summary and added assertions so the single-workflow response now matches the list path; verified with focused daemon tests and `make verify`.

---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/daemon/query_service.go
line: 165
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22EO,comment:PRRC_kwDORy7nkc68_V6l
---

# Issue 011: _⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_

**Use the counted summary builder here as well.**

`counts` is already available, but `transportWorkflowSummary(workflow)` drops the derived `task_counts` and start-block fields that `buildWorkflowCard` now includes. The top-level `TaskCounts` payload softens this, but the embedded `Workflow` becomes inconsistent with the rest of the API surface.


<details>
<summary>Suggested fix</summary>

```diff
- summary := transportWorkflowSummary(workflow)
+ summary := transportWorkflowSummaryWithTaskCounts(workflow, counts)
  summary.ArchiveEligible = &archiveEligible
  summary.ArchiveReason = archiveReason
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/query_service.go` around lines 159 - 165, The embedded
Workflow summary is built with transportWorkflowSummary(workflow), which omits
derived task_counts and start-block fields; instead call the counted summary
builder (buildWorkflowCard or the function that accepts both workflow and
counts) to construct the summary using the existing counts variable, then set
summary.ArchiveEligible = &archiveEligible and summary.ArchiveReason =
archiveReason before returning the WorkflowOverviewPayload so the embedded
Workflow matches the top-level TaskCounts payload.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:96e4d5e8-33bf-4fb0-b3f8-c84c4be01780 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `WorkflowOverview` builds its embedded `Workflow` summary without the derived `TaskCounts`, `CanStartRun`, and `StartBlockReason` fields even though those counts are already available, leaving the payload internally inconsistent.
- Fix plan: Build the embedded summary with `transportWorkflowSummaryWithTaskCounts` and extend the existing workflow overview tests outside the listed scope because they live in `internal/daemon/query_service_test.go` and `internal/daemon/transport_read_models_test.go`.

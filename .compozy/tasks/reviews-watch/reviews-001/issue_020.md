---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/daemon/run_manager.go
line: 764
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4208391746,nitpick_hash:be07bc5e7130
review_hash: be07bc5e7130
source_review_id: "4208391746"
source_review_submitted_at: "2026-04-30T20:37:25Z"
---

# Issue 020: Confusing fallback error handling in rejectCompletedTaskWorkflow.
## Review Comment

The control flow at lines 794-798 has subtle behavior: when `taskRows` is empty, it returns `nil` (allowing the run), but when `taskRows` is non-empty, it returns the original `SnapshotTaskMeta` error. This masking could confuse callers since the returned error references task metadata inspection but the actual check was done via DB rows.

Consider returning a clearer error when falling back to DB succeeds but metadata inspection initially failed:

```diff
if len(taskRows) == 0 {
return nil
}
- return fmt.Errorf("inspect task metadata for %s: %w", strings.TrimSpace(workflowSlug), err)
+ // DB shows tasks exist but metadata snapshot failed - proceed cautiously
+ return nil
```

Alternatively, if the metadata error should still block execution when tasks exist, document this intent clearly.

## Triage

- Decision: `invalid`
- Reasoning: The fallback is intentionally asymmetric. If metadata snapshotting fails but no DB task rows exist, the workflow can proceed; if DB rows do exist, surfacing the original metadata inspection error preserves a real filesystem/state inconsistency instead of silently allowing execution.
- Why no fix: Returning `nil` when DB rows exist would mask corrupt or unsynced task metadata and weaken the run-start safety boundary.

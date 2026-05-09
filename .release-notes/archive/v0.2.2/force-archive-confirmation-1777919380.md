---
title: Force-confirmation when archiving non-terminal workflows
type: feature
---

Archiving a workflow that still has open work no longer silently succeeds. The daemon now returns a typed `workflow_force_required` error when the target workflow has non-terminal tasks or unresolved review issues, and the dashboard surfaces it as an inline confirmation dialog so you can either resolve the open items first or explicitly archive with `force = true`.

### What changed

- `internal/core/archive.go` introduces `ErrWorkflowForceRequired` and a structured `WorkflowArchiveForceRequiredError` that reports task and review counts:

  ```go
  type WorkflowArchiveForceRequiredError struct {
      WorkspaceID      string
      WorkflowID       string
      Slug             string
      Reason           string
      TaskTotal        int
      TaskNonTerminal  int
      ReviewTotal      int
      ReviewUnresolved int
  }
  ```

- The daemon HTTP API maps that error to `code: "workflow_force_required"` with a 409 response, so frontends can detect it without parsing strings.
- `model.ArchiveConfig.Force` and the kernel `WorkflowArchiveCommand.Force` field now flow end-to-end, so a retry with `force=true` bypasses the gate.
- The web archive flow (`web/src/routes/_app/workflows.tsx` + `web/src/systems/workflows/adapters/workflows-api.ts`) catches the typed error, opens an alert dialog with task/review counts, and re-issues the archive call with `force: true` if you confirm.

### Web UI

A new `AlertDialog` primitive in `@compozy/ui` powers the confirmation. The flow is:

1. Click _Archive_ on a workflow with open tasks or reviews.
2. The daemon returns `workflow_force_required` with counts (e.g. `task_non_terminal: 2`, `review_unresolved: 1`).
3. The UI opens a confirmation dialog explaining what will be archived anyway.
4. Confirm → the same archive request is retried with `force: true`; the response includes `forced: true` and the counts that were overridden.

### API shape

```jsonc
// Without force, when state is open:
HTTP 409
{
  "code": "workflow_force_required",
  "message": "workflow \"my-feature\" requires force archive confirmation: ...",
  "details": {
    "task_total": 5,
    "task_non_terminal": 2,
    "review_total": 4,
    "review_unresolved": 1
  }
}

// Retry with force = true:
{
  "archived": true,
  "forced": true,
  "completed_tasks": 5,
  "resolved_review_issues": 4
}
```

Workflows whose state is already clean continue to archive on the first call with no prompt — the gate only fires when there is genuinely open work.

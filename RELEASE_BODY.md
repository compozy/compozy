## 0.2.2 - 2026-05-05

### 🎉 Features

- Add qa extension (#138)
### 🐛 Bug Fixes

- Workspace register (#140)- Workspace discover path
### 📚 Documentation

- Update- Release notes
### 📦 Build System

- Release tool

### Release Notes

#### Features

##### Force-confirmation when archiving non-terminal workflows
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

##### Built-in QA workflow extension
Compozy now ships a built-in `cy-qa-workflow` extension that automatically attaches QA-planning and QA-execution tasks to any PRD-driven workflow, with curated runtimes per task. The extension lives at `extensions/cy-qa-workflow/` and follows the same on-disk contract as user extensions, so it can be customized or replaced project-by-project.

When enabled, every `compozy tasks run <slug>` over a PRD-mode workflow ends up with two extra tasks at the tail of `_tasks.md`:

| Task                                                   | Purpose                                                                                                                                | Type   | Complexity |
| ------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------- | ------ | ---------- |
| `<Workflow> QA plan and regression artifacts`          | Generates feature-level test plans, execution-ready test cases, and regression suites under `.compozy/tasks/<workflow>/qa/`            | `docs` | `high`     |
| `<Workflow> QA execution and operator-flow validation` | Executes the generated plan, files bug reports for confirmed failures, fixes root causes, and finishes only after `make verify` passes | `test` | `critical` |

The execution task depends on the report task; the report task depends on every other implementation task in the workflow, so QA always runs last.

### Curated runtimes

The extension also pins per-task runtimes via the new `plan.pre_resolve_task_runtime` hook so each QA task runs on the IDE/model best suited to it — no manual `--task-runtime` needed:

| QA task      | IDE      | Model     | Reasoning effort |
| ------------ | -------- | --------- | ---------------- |
| QA report    | `claude` | `opus`    | `xhigh`          |
| QA execution | `codex`  | `gpt-5.5` | `xhigh`          |

Override on a per-run basis with `--task-runtime`, or per-project via `[[tasks.run.task_runtime_rules]]`.

### Prompt augmentation

`cy-qa-workflow` also patches the agent session at create time:

- The QA execution prompt is prefixed with `/goal …` so the agent enters goal-driven mode and only finishes after `make verify` passes.
- The QA report prompt sets `CLAUDE_CODE_EFFORT_LEVEL=xhigh` in the session env to lift Claude's effort ceiling for plan generation.

### Manifest

```toml
# extensions/cy-qa-workflow/extension.toml
[extension]
name = "cy-qa-workflow"
version = "0.1.0"
description = "Adds Compozy QA report and QA execution tasks to workflow runs"
min_compozy_version = "0.1.10"

[subprocess]
command = "go"
args = ["run", "."]

[security]
capabilities = ["plan.mutate", "agent.mutate", "tasks.read", "tasks.create"]

[[hooks]]
event = "plan.pre_discover"
required = true

[[hooks]]
event = "plan.pre_resolve_task_runtime"
required = true

[[hooks]]
event = "agent.pre_session_create"
required = true
```

### Idempotency

- Tasks are detected by HTML markers (`<!-- compozy-qa-workflow:qa-report -->` / `<!-- compozy-qa-workflow:qa-execution -->`) plus title/type heuristics, so re-running the workflow does not duplicate them.
- `update_index = true` is set on the new `host.tasks.create` request, so the entries appear in `_tasks.md` in the right order on first run.

### SDK additions used by the extension

- `TaskCreateRequest.UpdateIndex` (`update_index` in JSON / TS) — when `true`, the host appends the created task to `_tasks.md`. Documented in `docs/extensibility/host-api-reference.md`.
- `TaskFrontmatter.Dependencies` — extensions can now seed task dependencies directly when creating a task.
- `SessionRequest` / `ResumeSessionRequest` now use a stable readable JSON contract (prompts are plain strings, not base64), matching the runtime-side ACP contract used by hook payloads and patches.

#### Fixes

##### Workspace register/resolve path fixes
Two long-standing workspace-discovery papercuts are fixed. `compozy workspaces register` and `resolve` now accept relative paths the same way every other Compozy command does, and workspace auto-discovery no longer treats the home-scoped `~/.compozy/` runtime directory as a project-local workspace marker.

Closes #139.

### Relative paths now work for `register` / `resolve`

Before, the API client sent paths through unchanged after `strings.TrimSpace`. A relative path like `.` or `./my-project` was forwarded to the daemon as-is, where it resolved against the daemon's working directory instead of the caller's, producing confusing "workspace not found" errors or registering the wrong directory.

The client now normalizes the argument before sending it:

```go
// internal/api/client/operator.go
func normalizeWorkspacePathArg(path string) (string, error) {
    trimmed := strings.TrimSpace(path)
    if trimmed == "" {
        return "", nil
    }
    if filepath.IsAbs(trimmed) {
        return filepath.Clean(trimmed), nil
    }
    absolutePath, err := filepath.Abs(trimmed)
    if err != nil {
        return "", fmt.Errorf("resolve workspace path %q: %w", path, err)
    }
    return filepath.Clean(absolutePath), nil
}
```

This normalization runs for both `RegisterWorkspace` and `ResolveWorkspace`, so:

```bash
cd ~/code/my-feature
compozy workspaces register .            # now registers /Users/you/code/my-feature
compozy workspaces resolve ./sub-project  # resolves against the caller's CWD
```

### `~/.compozy/` is no longer auto-detected as a workspace

`discoverWorkspaceRootFromStart` walks up the filesystem looking for a `.compozy/` marker directory. When `compozy` was invoked from anywhere under `$HOME` that did not contain its own `.compozy/`, the walk would eventually find `~/.compozy/` — the home-scoped daemon runtime root — and register the user's home directory (or some ancestor) as a workspace.

The discovery loop now resolves the global Compozy marker once and skips it during the walk, so only project-local `.compozy/` directories are treated as workspace roots:

```go
// internal/core/workspace/config.go
globalMarkerDir, hasGlobalMarker := discoverGlobalWorkspaceMarkerDir()
// ...
if err == nil && info.IsDir() {
    // The home-scoped Compozy directory stores global runtime/config state.
    // It must not redefine arbitrary paths under HOME as local workspaces.
    if !hasGlobalMarker || !sameWorkspaceMarkerDir(candidate, globalMarkerDir) {
        return current, nil
    }
}
```

Comparison is symlink-aware (`filepath.EvalSymlinks` on both sides), so installs that symlink `~/.compozy/` are still correctly excluded.

### Coverage

New tests pin the behavior end-to-end:

- `internal/api/client/client_transport_test.go` — relative paths are normalized before transport.
- `internal/cli/operator_commands_integration_test.go` — `register` / `resolve` from a relative CWD produce absolute paths in the registry.
- `internal/core/workspace/config_test.go` — discovery skips `~/.compozy/` even when started from `$HOME`.
- `internal/store/globaldb/registry_test.go` — registry insert/lookup is consistent with the normalized paths.
# Task Memory: task_02.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Migrate `internal/core/run` execution and logging from subprocess/stdout parsing to ACP `Client`/`Session` lifecycle with typed `SessionUpdate` handling.
- Preserve runtime lifecycle behavior (`queued` through `succeeded/failed/retrying/canceled`) while keeping post-success hooks unchanged.
- Result: completed with clean `make verify` and `internal/core/run` coverage at 80.4%.

## Important Decisions
- Keep ACP SDK types confined to `internal/core/agent`; `internal/core/run` consumes only repo-local `agent.Client`, `agent.Session`, and `model` content types.
- Bridge typed ACP content into the existing UI by storing raw `[]model.ContentBlock` on each `uiJob` and deriving fallback text buffers from those blocks until task_03 introduces richer rendering.
- Preserve existing system-prompt behavior during the ACP migration by prepending `systemPrompt` text into the session request prompt via `composeSessionPrompt(...)`.
- Extend `agent.Session` with `ID()` so runtime `slog` entries can correlate agent/session lifecycle without leaking SDK structs.

## Learnings
- `github.com/coder/acp-go-sdk@v0.6.3` does not expose usage totals on live session updates; the new handler supports repo-local `model.Usage` when present, but live usage reporting remains adapter-limited for now.
- Package-level client injection (`newAgentClient`) is useful for unit coverage, but those tests cannot run in parallel because they share the replacement seam.
- The helper-process ACP integration path is stable for end-to-end execution tests by installing a temporary `codex-acp` shim on `PATH` and replaying session scenarios through stdio.

## Files / Surfaces
- `internal/core/agent/session.go`
- `internal/core/run/command_io.go`
- `internal/core/run/execution.go`
- `internal/core/run/logging.go`
- `internal/core/run/types.go`
- `internal/core/run/ui_model.go`
- `internal/core/run/ui_model_test.go`
- `internal/core/run/ui_update.go`
- `internal/core/run/ui_update_test.go`
- `internal/core/run/ui_view.go`
- `internal/core/run/ui_view_test.go`
- `internal/core/run/logging_test.go`
- `internal/core/run/execution_acp_test.go`
- `internal/core/run/execution_acp_integration_test.go`

## Errors / Corrections
- The first post-implementation `make verify` failed on `gocyclo` and `revive`; the fix was to split `renderContentBlock(...)` into per-block helpers and move `context.Context` to the first parameter position in `handleSessionCompletion(...)`.
- A leftover helper name still contained the legacy `TokenUsage` identifier after the type migration; renaming it ensured the run package no longer referenced the removed symbol at all.

## Ready for Next Run
- Task_02 is complete. Task_03 should build richer UI rendering on top of `uiJob.blocks`, remove the remaining fallback-oriented log presentation glue where appropriate, and decide whether ACP usage needs additional adapter support beyond the current SDK surface.

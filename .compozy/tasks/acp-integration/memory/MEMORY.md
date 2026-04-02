# Workflow Memory

Keep only durable, cross-task context here. Do not duplicate facts that are obvious from the repository, PRD documents, or git history.

## Current State
- Tasks 01, 02, and 03 are complete and verified.

## Shared Decisions
- The ACP registry concept from the techspec is implemented as exported `agent.Spec` instead of `AgentSpec` to satisfy the repo's zero-lint `revive` stutter rule.
- `internal/core/run` now depends only on `agent.Client` / `agent.Session` and repo-local content types; ACP SDK imports remain confined to `internal/core/agent/`.
- The TUI now renders directly from `uiJob.blocks` in `internal/core/run/ui_view.go`; the legacy line buffers are no longer the primary viewport source.
- The final ACP cleanup moved shared shell preview helpers into `internal/core/agent/registry.go` and removed the legacy `internal/core/agent/ide.go` driver layer entirely.

## Shared Learnings
- `github.com/coder/acp-go-sdk` is pinned at `v0.6.3`.
- ACP SDK imports are currently confined to `internal/core/agent/`.
- The current ACP SDK surface still does not provide live usage totals on session updates; `internal/core/run` supports repo-local `model.Usage`, but live usage metrics remain zero unless future adapter/SDK work populates them.
- `agent.Session` now exposes `ID()` so `internal/core/run` can emit structured session lifecycle logs with agent/session correlation.

## Open Risks
- None currently.

## Handoffs
- The new ACP foundations live in `internal/core/model/content.go` and `internal/core/agent/{client,session,registry}.go`.
- The mock ACP helper in `internal/core/agent` already covers full stdio lifecycle, prompt errors, and cancellation paths for follow-on runtime work.
- The ACP runtime execution/logging pipeline now lives in `internal/core/run/{command_io,execution,logging,types}.go`, with coverage from `execution_acp_test.go`, `execution_acp_integration_test.go`, and the updated UI/logging tests.
- The rich block renderer and usage-summary plumbing now live in `internal/core/run/{ui_view,ui_update,ui_styles}.go`, with mixed-block and summary coverage in the UI tests.

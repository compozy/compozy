---
status: completed
title: UI Adaptation & Legacy Cleanup
type: frontend
complexity: high
dependencies:
    - task_02
---

# Task 3: UI Adaptation & Legacy Cleanup

## Overview

Adapt the Bubble Tea TUI to render typed ACP content blocks (text, diff, tool_use, terminal_output) instead of raw text lines, and remove all legacy code that has been replaced by the ACP agent layer and execution pipeline from tasks 1 and 2. This completes the ACP integration by delivering rich UI rendering and a clean codebase with no dead code.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST update `uiJob` struct (types.go:90) to store typed content blocks instead of (or alongside) `lineBuffer` for stdout/stderr
- MUST update `jobUpdateMsg` (replacing `jobLogUpdateMsg`) handler in `ui_update.go` to process `[]model.ContentBlock`
- MUST update `usageUpdateMsg` (replacing `tokenUsageUpdateMsg`) handler to use `model.Usage`
- MUST update `buildViewportContent()` (ui_view.go:561) to render content blocks by type: text as-is, diff with syntax highlighting, tool_use with name/input formatting, terminal_output with command/output/exit code
- MUST implement a fallback text renderer for unknown `ContentBlockType` values — degrade gracefully to raw JSON text
- MUST update `renderSummaryTokenBox()` (ui_view.go:120) to display `model.Usage` fields (InputTokens, OutputTokens, CacheReads, CacheWrites, TotalTokens)
- MUST update `buildMetaCard()` (ui_view.go:396) to display `model.Usage` instead of old `TokenUsage`
- MUST update `uiModel.aggregateUsage` field to use `*model.Usage` instead of `*TokenUsage`
- MUST delete `internal/core/agent/ide.go` entirely (452 lines) — replaced by registry.go from task_01
- MUST delete `internal/core/agent/ide_test.go` entirely — replaced by new registry tests from task_01
- MUST remove `ClaudeMessage` struct from `types.go` (line 167)
- MUST remove old `TokenUsage` struct from `types.go` (line 145) — replaced by `model.Usage`
- MUST remove `jsonFormatter` (logging.go:259), `activityMonitor` (logging.go:184), and `uiLogTap` (logging.go:205) — replaced by UpdateHandler in task_02
- MUST remove `lineFilterWriter` (logging.go:108) if no longer referenced
- MUST remove `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS` env var injection from `command_io.go` (line 130) if not already done in task_02
- MUST remove IDE-type branching for `formatsJSON` in tap builders if not already done in task_02
- MUST ensure zero dead code — no unused types, functions, or imports remain
</requirements>

## Subtasks

- [x] 3.1 Update `uiJob` struct and related types to support typed content blocks and `model.Usage`
- [x] 3.2 Update UI message handlers in `ui_update.go` for new `jobUpdateMsg` and `usageUpdateMsg` types
- [x] 3.3 Rewrite `buildViewportContent()` to render per-block-type content (text, diff, tool_use, terminal_output, image placeholder, unknown fallback)
- [x] 3.4 Update summary views (`renderSummaryTokenBox`, `buildMetaCard`) for `model.Usage` fields
- [x] 3.5 Delete legacy files: `ide.go`, `ide_test.go`; remove `ClaudeMessage`, old `TokenUsage`, `jsonFormatter`, `activityMonitor`, `uiLogTap`, `lineFilterWriter` from run/ package
- [x] 3.6 Verify zero dead code — no unused imports, types, or functions remain across all modified packages
- [x] 3.7 Write unit tests for all new UI rendering logic

## Implementation Details

The TechSpec §UI Adaptation describes the target rendering approach. The TechSpec §Data Models defines the message type changes. ADR-004 mandates that all ACP content block types are exposed to the UI layer for rich rendering.

### Relevant Files

- `internal/core/run/ui_model.go` — `uiModel` struct (line 16) with `aggregateUsage *TokenUsage`, `uiController` (line 38)
- `internal/core/run/ui_view.go` — `buildViewportContent()` (line 561), `renderSummaryTokenBox()` (line 120), `buildMetaCard()` (line 396), `renderSummaryView()` (line 22)
- `internal/core/run/ui_update.go` — `Update()` dispatcher (line 9), `handleJobLogUpdate()` (line 252), `handleTokenUsageUpdate()` (line 257), `refreshViewportContent()` (line 143)
- `internal/core/run/ui_styles.go` — Existing styles (line 42-54) — may need new styles for diff highlighting, tool call formatting
- `internal/core/run/types.go` — `uiJob` (line 90), `TokenUsage` (line 145), `ClaudeMessage` (line 167), `jobLogUpdateMsg` (line 132), `tokenUsageUpdateMsg` (line 136)
- `internal/core/run/logging.go` — `jsonFormatter` (line 259), `activityMonitor` (line 184), `uiLogTap` (line 205), `lineFilterWriter` (line 108)
- `internal/core/agent/ide.go` — Entire file (452 lines) to delete
- `internal/core/agent/ide_test.go` — Entire file (195 lines) to delete

### Dependent Files

- `internal/core/model/content.go` — Consumed: `ContentBlock`, `ContentBlockType`, `SessionUpdate`, `Usage` (from task_01)
- `internal/core/agent/registry.go` — Must exist before `ide.go` is deleted (from task_01)
- `internal/core/run/execution.go` — Must already use ACP session (from task_02)

### Related ADRs

- [ADR-004: Redesigned Content Model with Typed ACP Content Blocks](../adrs/adr-004.md) — All block types rendered in UI; unknown types fall back to raw text
- [ADR-001: Full Replacement of Legacy Agent Drivers with ACP](../adrs/adr-001.md) — Legacy ide.go, ClaudeMessage, and activity monitor removed entirely

## Deliverables

- Updated `ui_model.go` with `model.Usage` aggregate field
- Updated `ui_update.go` with handlers for new message types
- Updated `ui_view.go` with per-block-type rendering and updated summary/meta views
- Updated `types.go` with legacy types removed and new message types
- Updated `ui_styles.go` with styles for diff blocks, tool calls, terminal output if needed
- Deleted `ide.go` and `ide_test.go`
- Cleaned `logging.go` — removed `jsonFormatter`, `activityMonitor`, `uiLogTap`, `lineFilterWriter`
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests for UI rendering pipeline **(REQUIRED)**

## Tests

- Unit tests:
  - [x] buildViewportContent: renders text blocks as plain text
  - [x] buildViewportContent: renders diff blocks with file path header and diff content
  - [x] buildViewportContent: renders tool_use blocks with tool name and formatted input
  - [x] buildViewportContent: renders terminal_output blocks with command, output, and exit code
  - [x] buildViewportContent: renders unknown block type as raw JSON fallback
  - [x] buildViewportContent: renders mixed block types in correct order
  - [x] renderSummaryTokenBox: displays all model.Usage fields correctly (InputTokens, OutputTokens, CacheReads, CacheWrites, TotalTokens)
  - [x] buildMetaCard: displays model.Usage token counts
  - [x] handleJobUpdate: processes jobUpdateMsg and refreshes viewport
  - [x] handleUsageUpdate: accumulates model.Usage per-job and aggregate
  - [x] uiJob stores and retrieves typed content blocks correctly
- Integration tests:
  - [x] Full UI message flow: jobUpdateMsg with mixed content blocks → viewport renders all block types
  - [x] Summary view: after job completion, token usage displays correctly from model.Usage
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- `make verify` passes with zero lint issues
- `ide.go` and `ide_test.go` deleted — no references remain
- Zero references to `ClaudeMessage`, old `TokenUsage`, `jsonFormatter`, `activityMonitor`, `uiLogTap` in codebase
- All ACP content block types render correctly in TUI
- Unknown block types degrade gracefully to raw text
- Token usage summary displays all `model.Usage` fields
- No dead code: `go vet`, `golangci-lint` report zero unused symbols

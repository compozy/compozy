---
status: pending
domain: TUI
type: Feature Implementation
scope: Full
complexity: medium
dependencies:
  - task_01
---

# Task 5: TUI Timeline Header — Title, Type Badge, Provider·Model

## Overview

Transform the session timeline panel header from the static `session.timeline` label into a dynamic, per-task banner: task title (uppercase) + `[type]` badge on the header line, with `provider · model` right-aligned on the meta row opposite the `N entries · selected M/N` counter. Falls back to the original `SESSION.TIMELINE` string when the running task has no title. This change makes it immediately visible which task is running and which provider+model is handling it.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST extend `uiJob` (`internal/core/run/types.go:93-125`) with `taskTitle string` and `taskType string` fields
- MUST extend `jobQueuedMsg` (`internal/core/run/types.go:163-173`) with `TaskTitle string` and `TaskType string` fields
- MUST wire the new fields from the parsed `model.TaskEntry` (available in the job-setup path in `internal/core/run/execution.go`) into `jobQueuedMsg` emission
- MUST render the timeline panel header with the task title in uppercase followed by a `[type]` badge when both are present (see TechSpec "TUI layout" preview)
- MUST fall back to the current `session.timeline` label when `taskTitle` is empty
- MUST render `provider · model` right-aligned on the same row as the entries/selection counter, truncating the left-side counter if the row is narrow
- MUST NOT break existing sidebar / main pane layouts — all width math must respect `timelineMinWidth` (44) and existing panel padding
- SHOULD reuse an existing Lipgloss style for the badge (e.g., a muted background chip with colorSuccess/colorWarning accent) — no new color constants unless absolutely necessary
</requirements>

## Subtasks
- [ ] 5.1 Extend `uiJob` and `jobQueuedMsg` with `taskTitle` / `taskType` fields.
- [ ] 5.2 Populate the new message fields at the job-setup site in `internal/core/run/execution.go` from the parsed `TaskEntry`.
- [ ] 5.3 Update `renderTimelinePanel` (`internal/core/run/ui_view.go:473-496`) to render the title+badge header when `taskTitle != ""`, else the existing `session.timeline` label.
- [ ] 5.4 Update `timelineMeta` (`ui_view.go:498-516`) to build a two-sided row: existing counter on the left, `provider · model` right-aligned.
- [ ] 5.5 Add a small helper for right-aligning the provider/model fragment inside a width-bounded row (reuse any existing alignment helpers if present).
- [ ] 5.6 Extend the timeline rendering tests with table-driven cases covering the fallback, the badge, the right-alignment truncation, and narrow widths.

## Implementation Details

The job pipeline already has access to the parsed `TaskEntry` during job setup — wire `entry.Title` and `entry.TaskType` into the new `jobQueuedMsg` fields at the emission site. The UI model (`uiModel`) receives the message and stores the values on the matching `uiJob`.

In `renderTimelinePanel`, introduce a `timelineHeaderLabel(job *uiJob) string` helper that returns either `"session.timeline"` (existing behavior) or `strings.ToUpper(title) + "  [" + taskType + "]"`. When only `taskTitle` is set and `taskType` is empty, skip the badge. When only `taskType` is set with empty `taskTitle`, still render `session.timeline` (title is the primary signal).

For the two-sided meta row, compute the available width as `contentWidth = panelContentWidth(timelineWidth)`, render the left counter as before, compute the right fragment `"<provider> · <model>"`, and pad between them with spaces so the right fragment sits at the right edge. When the combined width exceeds `contentWidth`, truncate the left counter first (it's more verbose).

The provider and model values live on `config` (`internal/core/run/types.go:249-301`: `ide`, `model`). The timeline panel has `m` (the `uiModel`) which holds the config; expose `m.cfg.ide` / `m.cfg.model` in the meta row. Map `ide` → display name for the provider (it's the CLI flag name, e.g., "claude", "codex").

Refer to TechSpec "TUI layout" and the ADR-003/ADR-001 preview in the approved answer for the exact rendered shape.

### Relevant Files
- `internal/core/run/types.go` (lines 93-125 for `uiJob`, 163-173 for `jobQueuedMsg`) — add `taskTitle` / `taskType` fields.
- `internal/core/run/execution.go` — wire `TaskEntry.Title` / `.TaskType` into `jobQueuedMsg` at emission.
- `internal/core/run/ui_view.go` (lines 473-523) — update `renderTimelinePanel` and `timelineMeta`.
- `internal/core/run/ui_view_test.go` or equivalent timeline test file — extend with rendering cases.
- `internal/core/run/ui_styles.go` — reuse styles; may add one chip style if needed.

### Dependent Files
- `internal/core/run/ui_model.go` — the `jobQueuedMsg` handler must copy the new fields onto `uiJob`.
- `internal/core/run/session_view_model.go` — no change, but verify the snapshot flow is unchanged.
- `internal/core/prompt/common.go` — `TaskEntry.Title` populated in task_01 is the upstream data source.

### Related ADRs
- [ADR-001: Task Metadata Schema v2](adrs/adr-001.md) — Title is canonical from frontmatter and the direct source for this TUI change.

## Deliverables
- `uiJob` + `jobQueuedMsg` extended with `taskTitle` / `taskType`.
- Timeline panel header shows dynamic title + type badge + right-aligned `provider · model`.
- Fallback to `SESSION.TIMELINE` preserved.
- Unit tests with 80%+ coverage **(REQUIRED)**.
- Integration test rendering the timeline with real snapshot data **(REQUIRED)**.

## Tests
- Unit tests:
  - [ ] `timelineHeaderLabel` with empty `taskTitle` returns `"session.timeline"` (lowercase tech label).
  - [ ] `timelineHeaderLabel` with `taskTitle="acp agent layer"` and `taskType="backend"` returns `"ACP AGENT LAYER  [backend]"`.
  - [ ] `timelineHeaderLabel` with `taskTitle="acp agent layer"` and empty `taskType` returns `"ACP AGENT LAYER"` (no badge).
  - [ ] The two-sided meta row pads so that `"claude · sonnet-4.5"` sits at the right edge for a given width.
  - [ ] When the combined left+right length exceeds the width, the left counter is truncated (not the provider/model).
  - [ ] `renderTimelinePanel` with a job whose `taskTitle` is empty still renders exactly the current layout (regression guard).
  - [ ] `renderTimelinePanel` with title+type+provider+model renders a three-line header (title, meta, blank) plus the transcript viewport.
- Integration tests:
  - [ ] Build a `uiJob` with a snapshot of 3 entries, set `taskTitle/taskType/provider/model`, render the panel, and golden-compare the output at width 80.
  - [ ] Render the same panel at width 44 (min) and assert no panic, no truncation of the provider/model fragment.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` passes (fmt + lint + test + build with zero issues)
- Running `compozy start` on a v2 task file renders the task title in the timeline header (manual smoke-test)
- The provider/model fragment is visible and right-aligned at all terminal widths from 80 to 200 columns

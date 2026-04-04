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
- MUST extend `model.Job` (`internal/core/model/model.go:208-214`) with `TaskTitle string` and `TaskType string` fields so parsed task metadata can flow from `plan.prepare` into the runner
- MUST extend the internal `job` struct (`internal/core/run/types.go:271-282`) with `taskTitle string` and `taskType string` fields and copy them from `model.Job` inside `newJobs()` (at or near `internal/core/run/types.go:316`)
- MUST extend `uiJob` (`internal/core/run/types.go:93-125`) with `taskTitle string` and `taskType string` fields
- MUST extend `jobQueuedMsg` (`internal/core/run/types.go:163-173`) with `TaskTitle string` and `TaskType string` fields and populate them in the emitter at `internal/core/run/ui_model.go:214-224` (inside `newUIController`)
- MUST populate `model.Job.TaskTitle`/`.TaskType` from `model.TaskEntry.Title`/`.TaskType` at the plan-preparation site — likely `internal/core/plan/prepare.go` (around lines 156-165) where `model.Job` instances are constructed
- MUST render the timeline panel header with the task title in uppercase followed by a `[type]` badge when both are present (see TechSpec "TUI layout" preview)
- MUST fall back to the current `session.timeline` label when `taskTitle` is empty
- MUST render `provider · model` right-aligned on the same row as the entries/selection counter, truncating the left-side counter if the row is narrow
- MUST use `agent.DisplayName(ide string) string` from `internal/core/agent/registry.go:318-324` to convert the runtime `ide` flag to a human-readable provider name; do NOT introduce a second mapping
- MUST source the runtime `ide` + `model` fields from the `config` struct (`internal/core/run/types.go:299-301`) — these are the `compozy start` runtime values, NOT the `review provider` field (`internal/core/run/types.go:246-249`) used for PR review flows
- MUST NOT break existing sidebar / main pane layouts — all width math must respect `timelineMinWidth` (44) and existing panel padding
- SHOULD reuse an existing Lipgloss style for the badge (e.g., a muted background chip with colorSuccess/colorWarning accent) — no new color constants unless absolutely necessary
</requirements>

## Subtasks
- [ ] 5.1 Extend `model.Job` (`internal/core/model/model.go:208-214`) with `TaskTitle` / `TaskType` so task metadata flows end-to-end from plan preparation into the runner.
- [ ] 5.2 Populate `model.Job.TaskTitle`/`.TaskType` from `model.TaskEntry` in `internal/core/plan/prepare.go` (around the `model.Job` construction at lines 156-165).
- [ ] 5.3 Extend the internal `job` struct (`internal/core/run/types.go:271-282`) and the `newJobs(src []model.Job)` converter (`types.go:316`) to copy the new fields.
- [ ] 5.4 Extend `uiJob` and `jobQueuedMsg` with `taskTitle` / `taskType` fields; populate `jobQueuedMsg` at the emission site `internal/core/run/ui_model.go:214-224` and copy into `uiJob` inside the message handler.
- [ ] 5.5 Update `renderTimelinePanel` (`internal/core/run/ui_view.go:473-496`) to render the title+badge header when `taskTitle != ""`, else the existing `session.timeline` label.
- [ ] 5.6 Update `timelineMeta` (`ui_view.go:498-516`) to build a two-sided row: existing counter on the left, `agent.DisplayName(ide) · model` right-aligned.
- [ ] 5.7 Extend the timeline rendering tests (`internal/core/run/ui_view_test.go:142-166,476-479` + `ui_update_test.go:263-301`) with table-driven cases covering the fallback, the badge, the right-alignment truncation, and narrow widths.

## Implementation Details

The task metadata must travel a four-stage pipeline: (1) `plan.prepare.go` parses task files and constructs `model.Job`s — extend `model.Job` with `TaskTitle`/`TaskType` and set them from the parsed `model.TaskEntry` here; (2) `run.newJobs` at `types.go:316` converts `[]model.Job → []job` (the internal runner struct at `types.go:271-282`) — extend this converter to copy the fields; (3) `newUIController` at `ui_model.go:214-224` iterates `jobs` and emits a `jobQueuedMsg` per job — extend the message and emitter to carry the fields; (4) the UI handler `handleJobQueued` (exercised in `ui_update_test.go:267`) copies the message fields into the matching `uiJob`. Without extending all four structs the data cannot reach the renderer.

In `renderTimelinePanel`, introduce a `timelineHeaderLabel(job *uiJob) string` helper that returns either `"session.timeline"` (existing behavior) or `strings.ToUpper(title) + "  [" + taskType + "]"`. When only `taskTitle` is set and `taskType` is empty, skip the badge. When only `taskType` is set with empty `taskTitle`, still render `session.timeline` (title is the primary signal).

For the two-sided meta row, compute the available width as `contentWidth = panelContentWidth(timelineWidth)`, render the left counter as before, compute the right fragment `"<provider> · <model>"`, and pad between them with spaces so the right fragment sits at the right edge. When the combined width exceeds `contentWidth`, truncate the left counter first (it's more verbose).

The provider and model values live on `config` (`internal/core/run/types.go:299-301`: `ide`, `model`). The timeline panel has `m` (the `uiModel`) which holds the config; expose `m.cfg.ide` / `m.cfg.model` in the meta row. Convert `ide` → display name using the existing `agent.DisplayName(ide)` at `internal/core/agent/registry.go:318-324`, which already maps `"claude"→"Claude"`, `"codex"→"Codex"`, `"droid"→"Droid"`, `"cursor"→"Cursor"`, `"opencode"→"OpenCode"`, `"pi"→"Pi"`, `"gemini"→"Gemini"`. Do NOT use `config.provider` (line 249) — that field is the PR review provider for `fix-reviews` / `fetch-reviews` workflows, not the `compozy start` runtime.

Refer to TechSpec "TUI layout" and the ADR-003/ADR-001 preview in the approved answer for the exact rendered shape.

### Relevant Files
- `internal/core/model/model.go` (lines 208-214, `type Job struct`) — add `TaskTitle`, `TaskType` fields.
- `internal/core/plan/prepare.go` (lines 156-165 — the `model.Job` construction site) — populate the new fields from `model.TaskEntry`.
- `internal/core/run/types.go` (lines 93-125 for `uiJob`, 163-173 for `jobQueuedMsg`, 271-282 for internal `job`, 316 for `newJobs`) — add fields and copy them through the converter.
- `internal/core/run/ui_model.go` (lines 214-224) — populate `jobQueuedMsg.TaskTitle`/`.TaskType` in `newUIController`.
- `internal/core/run/ui_view.go` (lines 473-523) — update `renderTimelinePanel` and `timelineMeta`.
- `internal/core/run/ui_view_test.go` (lines 142-166, 476-479) + `ui_update_test.go` (lines 263-301) — extend with rendering + `handleJobQueued` cases.
- `internal/core/agent/registry.go` (lines 318-324) — reuse `DisplayName(ide string) string`.
- `internal/core/run/ui_styles.go` — reuse styles; may add one chip style if needed.

### Dependent Files
- `internal/core/run/ui_model.go` — the `handleJobQueued` handler (exercised at `ui_update_test.go:267`) must copy the new fields onto `uiJob`.
- `internal/core/run/session_view_model.go` — no change, but verify the snapshot flow is unchanged.
- `internal/core/prompt/common.go` — `TaskEntry.Title` populated in task_01 is the upstream data source.
- Any producer of `model.Job` outside `plan/prepare.go` (grep for `model.Job{`) must also populate the new fields or explicitly leave them empty.

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

---
status: completed
title: "Phase 1: File-level splits"
type: refactor
complexity: high
dependencies:
  - task_01
---

# Task 02: Phase 1: File-level splits

## Overview

Split 12 oversized files into focused, single-responsibility files within their existing packages. These are safe refactors with no import path changes and no API changes -- only file reorganization. This dramatically reduces cognitive load and merge-conflict surface, preparing the codebase for the package-level splits in Phase 3. The `prompt/common.go` cleanup is intentionally deferred to Task 03 because that file still contains parsing logic that must move first.

<critical>
- ALWAYS READ the TechSpec (20260406-summary.md) and detailed reports before starting
- REFERENCE the individual analysis reports for exact split recommendations with line ranges
- FOCUS ON "WHAT" — move code between files, do not refactor logic
- MINIMIZE CODE — pure file moves, no behavior changes
- TESTS REQUIRED — run `make verify` after each split to catch breakage early
</critical>

<requirements>
- MUST split `internal/cli/root.go` (1,190 lines) into 5 focused files: `commands.go`, `commands_simple.go`, `dispatch_adapters.go`, `state.go`, `run.go` — keeping root.go for `NewRootCommand` and constants (G1-F1)
- MUST split `internal/core/run/execution.go` (1,900 lines) into: `lifecycle.go`, `shutdown.go`, `runner.go`, `session_exec.go`, `review_hooks.go` (G3-F2)
- MUST split `internal/core/run/types.go` (380 lines) into: `config.go`, `exec_types.go`, `ui_types.go`, `shutdown_types.go` (G3-F7)
- MUST split `internal/core/run/ui_view.go` (1,034 lines) into: `ui_sidebar.go`, `ui_timeline.go`, `ui_summary.go` (G3-F9)
- MUST split `internal/core/agent/session.go` (728 lines) into: `session.go` (lifecycle) + `acp_convert.go` (ACP-to-model conversion) (G3-F5)
- MUST split `internal/core/agent/tool_call_format.go` (732 lines) into: `tool_call_name.go` (name resolution) + `tool_call_input.go` (input normalization) (G3-F6)
- MUST split `internal/core/model/model.go` (369 lines) into: `constants.go`, `runtime_config.go`, `workspace_paths.go`, `artifacts.go`, `task_review.go`, `preparation.go` (G2-F3)
- MUST defer any `internal/core/prompt/common.go` split until Task 03 relocates parsing out of the file; do not restructure that file in this phase (G4-F06)
- MUST split `internal/core/agent/registry.go` (868 lines) into: `registry_specs.go`, `registry_validate.go`, `registry_launch.go` (G3-F12)
- MUST rename `internal/core/run/logging.go` to `session_handler.go` and extract `render_blocks.go` and `buffers.go` (G3-F8)
- MUST split `internal/core/workspace/config.go` (438 lines) into: `config_types.go`, `config_validate.go`, `config.go` (G4-F14)
- MUST split `pkg/compozy/runs/run.go` (451 lines) into: `run.go`, `replay.go`, `status.go` (G5-F08)
- MUST split `pkg/compozy/events/kinds/session.go` (449 lines) into: `content_block.go` + `session.go` (G5-F14)
- MUST NOT change any exported or unexported APIs, function signatures, or behavior
- MUST NOT change import paths — all splits are within the same package
- MUST pass `make verify` with zero issues after all splits
</requirements>

## Subtasks

- [x] 2.1 Split `model/model.go` into 6 focused files (foundational — other packages import model)
- [x] 2.2 Split `cli/root.go` into 5 focused files
- [x] 2.3 Split `run/execution.go` into 5 focused files
- [x] 2.4 Split `run/types.go` into 4 files and rename `logging.go` to `session_handler.go` with extractions
- [x] 2.5 Split `run/ui_view.go` into 3 panel files
- [x] 2.6 Split `agent/session.go`, `agent/tool_call_format.go`, and `agent/registry.go`
- [x] 2.7 Split remaining files: `workspace/config.go`, `events/kinds/session.go`, `pkg/runs/run.go`

## Implementation Details

Every split follows the same mechanical pattern: create new file with the same `package` declaration, move functions/types/vars from the source file, verify compilation. No logic changes. See the individual analysis reports for exact line ranges per split. `prompt/common.go` is intentionally excluded here and should be cleaned up only after Task 03 relocates parsing.

### Relevant Files

- `internal/cli/root.go` — 1,190 lines, 6+ responsibilities (see 20260406-cli-entry.md F1)
- `internal/core/run/execution.go` — 1,900 lines, split map in 20260406-agent-run.md F2
- `internal/core/run/types.go` — 380 lines, 4 domain mixtures
- `internal/core/run/ui_view.go` — 1,034 lines, 3 panel renderers
- `internal/core/run/logging.go` — 605 lines, misnamed (actually session handler + renderers)
- `internal/core/agent/session.go` — 728 lines, lifecycle + ACP conversion mixed
- `internal/core/agent/tool_call_format.go` — 732 lines, name + input normalization mixed
- `internal/core/agent/registry.go` — 868 lines, specs + validation + launch mixed
- `internal/core/model/model.go` — 369 lines, 5 unrelated domains
- `internal/core/workspace/config.go` — 438 lines, types + validation + discovery
- `pkg/compozy/runs/run.go` — 451 lines, loading + replay + status
- `pkg/compozy/events/kinds/session.go` — 449 lines, content blocks + session payloads

### Dependent Files

- All test files for the above packages — must continue to compile and pass
- No external callers are affected since no APIs change

## Deliverables

- 12 oversized files split into focused files
- Zero behavior changes
- All existing tests passing unchanged
- `make verify` passes with zero issues **(REQUIRED)**

## Tests

- Unit tests:
  - [x] All existing tests in `internal/cli/` pass after root.go split
  - [x] All existing tests in `internal/core/run/` pass after execution.go, types.go, ui_view.go, logging.go splits
  - [x] All existing tests in `internal/core/agent/` pass after session.go, tool_call_format.go, registry.go splits
  - [x] All existing tests in `internal/core/model/` pass after model.go split
  - [x] All existing tests in `internal/core/workspace/` pass after config.go split
  - [x] All existing tests in `pkg/compozy/runs/` pass after run.go split
  - [x] All existing tests in `pkg/compozy/events/` pass after session.go split
- Integration tests:
  - [x] `make verify` passes (fmt + lint + test + build)
- All tests must pass

## Success Criteria

- All tests passing
- `make verify` exits 0
- No single file exceeds 500 lines (except files not targeted by this task)
- Zero API or behavior changes — this is a pure file reorganization

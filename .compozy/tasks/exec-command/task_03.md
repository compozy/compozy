---
status: pending
title: Exec Command, Prompt-Backed Preparation, and Structured Output
type: backend
complexity: critical
dependencies:
  - task_01
  - task_02
---

# Exec Command, Prompt-Backed Preparation, and Structured Output

## Overview

Deliver the actual `compozy exec` feature on top of the shared runtime and artifact foundations. This task adds the CLI surface, turns ad hoc prompt sources into normal prepared jobs, and extends the executor so `exec` can run in either human-oriented text mode or machine-readable JSON mode without creating a parallel execution path.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST add a `compozy exec` command that accepts a positional prompt, `--prompt-file`, or `stdin` with explicit ambiguity checks
- MUST build ad hoc prompt-backed jobs through the shared planning pipeline instead of bypassing planning or execution
- MUST keep `exec` limited to a single prepared job unless a later spec explicitly expands batching
- MUST support `--format text|json`, where JSON mode suppresses the human/TUI presentation path while still using the shared executor
- MUST emit stable machine-readable run results that include status, usage, and artifact paths, including `result.json` for JSON-mode runs
- MUST preserve existing `start` and `fix-reviews` execution behavior while extending the executor for `exec`
</requirements>

## Subtasks
- [ ] 3.1 Add `newExecCommand()` and command-state plumbing for prompt source, output format, and workspace defaults
- [ ] 3.2 Implement prompt-source resolution across positional input, `--prompt-file`, and `stdin`, with explicit validation for ambiguous combinations
- [ ] 3.3 Extend planning to build one ad hoc job and run metadata from prompt-backed inputs using the shared artifact layout
- [ ] 3.4 Extend the shared executor to support `text` and `json` output modes without duplicating ACP runtime logic
- [ ] 3.5 Persist JSON-mode run results and expose stable result payloads for automation
- [ ] 3.6 Add unit, CLI, and execution integration tests for prompt sources, JSON mode, and failure behavior

## Implementation Details

Use the TechSpec sections "Component Overview", "Prompt source resolution rules", "Data flow", and "Structured Output" as the implementation contract. This task should consume the config surface from task 01 and the artifact layout from task 02, then deliver the end-to-end feature without reopening those boundaries.

### Relevant Files
- `internal/cli/root.go` — Command wiring, flags, examples, and run entry points live here
- `internal/cli/root_test.go` — CLI behavior, validation, and command execution patterns should be extended here
- `internal/core/plan/input.go` — Prompt-backed input resolution should be added alongside existing directory-backed resolution
- `internal/core/plan/prepare.go` — Ad hoc job construction should be integrated into shared preparation
- `internal/core/run/execution.go` — Shared execution flow must support text vs JSON completion behavior
- `internal/core/run/types.go` — Executor config and per-job runtime metadata should carry output-format and result-path information
- `internal/core/run/command_io.go` — Session startup and log/result writing must remain consistent in JSON mode

### Dependent Files
- `internal/cli/testdata/start_help.golden` — Existing help snapshot patterns should guide new exec help coverage
- `internal/core/run/execution_acp_test.go` — Shared execution behavior for retries, dry-run, and logs should gain `exec` coverage
- `internal/core/run/execution_acp_integration_test.go` — ACP integration coverage should assert JSON mode, UI suppression, and result persistence

### Related ADRs
- [ADR-001: Model Ad Hoc Execution as a First-Class Runtime Mode](../adrs/adr-001.md) — Requires `exec` to ride the shared planner and executor
- [ADR-002: Replace `.tmp/codex-prompts` with Workspace-Scoped Run Artifacts](../adrs/adr-002.md) — Requires `exec` to use the shared run layout
- [ADR-003: Support Multi-Source Prompt Input and Structured Output for `compozy exec`](../adrs/adr-003.md) — Defines prompt-source rules, JSON output, and CLI precedence

## Deliverables
- `compozy exec` CLI command with prompt-source and format validation
- Shared planning support for prompt-backed ad hoc runs
- Shared executor support for `text` and `json` output behavior
- Persisted JSON results for automation under `.compozy/runs/<run-id>/result.json`
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests covering CLI, planning, and ACP execution behavior **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Positional prompt, `--prompt-file`, and `stdin` resolve correctly and ambiguous combinations fail with descriptive errors
  - [ ] `exec` config defaults merge correctly with explicit flags and workspace defaults
  - [ ] Prompt-backed planning produces exactly one prepared job with prompt and log artifacts under the shared run directory
  - [ ] JSON result payloads include run status, job summaries, usage, and artifact paths
- Integration tests:
  - [ ] `compozy exec "prompt"` runs through the shared pipeline in text mode and writes prompt/log artifacts under `.compozy/runs/`
  - [ ] `compozy exec --prompt-file <path> --format json` emits machine-readable output, persists `result.json`, and suppresses the human/TUI path
  - [ ] `compozy exec` using `stdin` works end-to-end without breaking existing `start` or `fix-reviews` execution behavior
  - [ ] Runtime failures in JSON mode still produce structured results and usable artifact paths for debugging
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `compozy exec` works with positional prompt, prompt file, and `stdin`
- `exec` runs through the shared planning and execution stack instead of a parallel ACP path
- JSON mode emits stable machine-readable results and suppresses human/TUI presentation correctly
- Existing execution modes continue to pass their regression coverage unchanged

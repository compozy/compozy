## status: completed

<task_context>
  <domain>Agent, Prompt</domain>
  <type>Refactor</type>
  <scope>Full</scope>
  <complexity>medium</complexity>
  <dependencies>none</dependencies>
</task_context>

# Task 4: Agent Command Refactor + Mode-Specific System Prompts

## Overview
Refactor the Claude Code command construction to drop headless flags (`--print`, `--output-format stream-json`, `--verbose`) and add `--system-prompt` for interactive terminal mode. Create mode-specific system prompt builders that include behavioral instructions, skill references, and the job-done signaling protocol (curl command) per execution mode (PRD tasks, fix reviews). This task changes how Claude Code is invoked and what instructions it receives.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST remove `--print`, `--output-format stream-json`, and `--verbose` flags from Claude command construction
- MUST replace `--append-system-prompt` with `--system-prompt` for full system prompt control
- MUST keep `--permission-mode bypassPermissions`, `--dangerously-skip-permissions`, `--model`, `--add-dir` flags
- MUST create BuildSystemPrompt(mode, jobID, serverPort) function that returns mode-specific prompts
- Every system prompt MUST include the job-done curl instruction with correct port and job ID
- MUST add SignalPort field to RuntimeConfig with default value of 9877
- MUST update buildClaudeCommand() shell preview to reflect new flags
- MUST keep existing agent specs for codex, droid, cursor unchanged (only Claude changes)
</requirements>

## Subtasks
- [x] 4.1 Create `internal/looper/prompt/system.go` with BuildSystemPrompt() and mode-specific prompt content
- [x] 4.2 Refactor `claudeCommand()` in agent/ide.go to accept systemPrompt parameter and use --system-prompt flag
- [x] 4.3 Refactor `buildClaudeCommand()` shell preview to match new command structure
- [x] 4.4 Update `agent.Command()` public API to pass system prompt through from RuntimeConfig
- [x] 4.5 Add SignalPort field to RuntimeConfig in model.go (if not done in task_02)
- [x] 4.6 Write unit tests for system prompt builder and updated command construction

## Implementation Details
Modify `internal/looper/agent/ide.go` — the `claudeCommand()` function currently builds the command with `--print --output-format stream-json`. These flags must be removed and `--system-prompt` added. The `spec` struct for Claude may need a new field or the `commandFunc` signature may need to change to accept the system prompt.

Create `internal/looper/prompt/system.go` with prompt content per mode. The job-done instruction is appended to every mode's prompt as the final section.

Reference the TechSpec "Agent Command Changes" and "System Prompt Structure" sections. Reference ADR-004 for the design rationale.

### Relevant Files
- `internal/looper/prompt/system.go` — NEW: Mode-specific system prompt builders
- `internal/looper/agent/ide.go` — MODIFY: claudeCommand(), buildClaudeCommand(), Command()

### Dependent Files
- `internal/looper/agent/ide_test.go` — Update existing tests for new command structure
- `internal/looper/run/command_io.go` — Currently calls agent.Command(); will be replaced in task_05
- `internal/looper/prompt/common.go` — Existing prompt builders, system.go follows same patterns

### Related ADRs
- [ADR-004: Mode-Specific System Prompts](adrs/adr-004.md) — Defines why --system-prompt over --append-system-prompt or CLAUDE.md

## Deliverables
- `internal/looper/prompt/system.go` with complete system prompt builder
- Refactored `internal/looper/agent/ide.go` with updated Claude command construction
- Updated tests in `ide_test.go`
- Unit tests with 80%+ coverage **(REQUIRED)**

## Tests
- Unit tests:
  - [x] BuildSystemPrompt for ExecutionModePRDTasks includes PRD-specific instructions and job-done curl
  - [x] BuildSystemPrompt for ExecutionModePRReview includes review-specific instructions and job-done curl
  - [x] BuildSystemPrompt includes correct port number and job ID in curl command
  - [x] claudeCommand() does NOT include --print, --output-format, or --verbose flags
  - [x] claudeCommand() includes --system-prompt with the built prompt
  - [x] claudeCommand() still includes --model, --permission-mode, --dangerously-skip-permissions, --add-dir
  - [x] buildClaudeCommand() shell preview reflects new flag structure
  - [x] Command() for non-Claude IDEs (codex, droid, cursor) remains unchanged
  - [x] agent.Command() passes system prompt correctly for Claude IDE
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` passes (fmt + lint + test + build)
- Claude command runs in interactive mode (no --print)
- System prompt includes mode-specific content and job-done signaling
- Non-Claude IDE commands are unaffected

## status: completed

<task_context>
  <domain>Runtime, Input</domain>
  <type>Feature Implementation</type>
  <scope>Full</scope>
  <complexity>medium</complexity>
  <dependencies>task_01</dependencies>
</task_context>

# Task 3: Input Layer (Key Translation + Composer Simulation + Readiness Detection)

## Overview
Create the three input-handling components that enable communication between the Bubbletea UI and PTY terminals: key translation (converting tea.KeyMsg to terminal escape bytes), composer simulation (sending the initial task prompt via keystroke injection), and readiness detection (polling the VT emulator screen to detect when Claude Code's composer is ready for input). Together these enable the looper to automatically deliver task prompts and allow users to interact directly with running agents.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- Key translation MUST support: Enter, Tab, Backspace, arrow keys, Ctrl+C, Ctrl+D, Esc, and printable runes
- Composer simulation MUST normalize line endings, use Ctrl+J (0x0a) for newlines within the composer, and Enter (0x0d) to submit
- Readiness detection MUST poll VT emulator screen every 200ms looking for composer prompt indicators
- Readiness detection MUST have a 15-second timeout fallback (send prompt anyway if detection fails)
- Readiness detection MUST respect context cancellation
- Composer message MUST be short (file path reference), not the full prompt content
</requirements>

## Subtasks
- [x] 3.1 Create `internal/looper/run/keytranslate.go` with translateKey() function covering all supported key types
- [x] 3.2 Create `internal/looper/run/composer.go` with sendComposerInput() function implementing keystroke injection
- [x] 3.3 Create `internal/looper/run/readiness.go` with waitForReady() and detectComposerReady() functions
- [x] 3.4 Write unit tests for key translation (all key types, edge cases)
- [x] 3.5 Write unit tests for composer simulation (single-line, multi-line, special characters)
- [x] 3.6 Write unit tests for readiness detection (ready screen, loading screen, timeout, cancellation)

## Implementation Details
Three new files in `internal/looper/run/`. Key translation maps Bubbletea key messages to raw terminal escape sequences. Composer simulation writes to the PTY using the Terminal.WriteInput() method from task_01. Readiness detection uses Terminal.Render() (via SafeEmulator.String()) to poll screen content.

Reference the TechSpec "Composer Simulation", "Readiness Detection", and "Key Forwarding" sections. Reference ADR-003 for the composer simulation rationale.

### Relevant Files
- `internal/looper/run/keytranslate.go` — NEW: Key translation
- `internal/looper/run/composer.go` — NEW: Composer simulation
- `internal/looper/run/readiness.go` — NEW: Readiness detection

### Dependent Files
- `internal/looper/run/terminal.go` — Uses Terminal.WriteInput() and Terminal.Render() (from task_01)
- `internal/looper/run/ui_update.go` — Will use translateKey() in task_05

### Related ADRs
- [ADR-003: Composer Simulation for Initial Prompt](adrs/adr-003.md) — Defines why composer simulation over --prompt flag or stdin pipe

## Deliverables
- `internal/looper/run/keytranslate.go` with complete key translation
- `internal/looper/run/composer.go` with composer simulation
- `internal/looper/run/readiness.go` with readiness detection
- Unit tests with 80%+ coverage **(REQUIRED)**

## Tests
- Unit tests:
  - [x] translateKey: Enter → 0x0d, Tab → 0x09, Backspace → 0x7f
  - [x] translateKey: Arrow keys → correct escape sequences (e.g., Up → \x1b[A)
  - [x] translateKey: Ctrl+C → 0x03, Ctrl+D → 0x04, Esc → 0x1b
  - [x] translateKey: Printable runes → UTF-8 bytes
  - [x] translateKey: Unknown key type → nil (no bytes sent)
  - [x] sendComposerInput: Single-line message sends text + Enter
  - [x] sendComposerInput: Multi-line message uses Ctrl+J between lines and Enter at end
  - [x] sendComposerInput: Normalizes \r\n and \r to \n
  - [x] sendComposerInput: Handles special characters (quotes, backticks, JSON)
  - [x] detectComposerReady: Screen with ">" at last line returns true
  - [x] detectComposerReady: Screen with "What can I help" returns true
  - [x] detectComposerReady: Empty screen returns false
  - [x] detectComposerReady: Loading screen without prompt indicator returns false
  - [x] waitForReady: Returns nil when composer detected ready within timeout
  - [x] waitForReady: Returns nil (fallback) after 15s timeout
  - [x] waitForReady: Returns error on context cancellation
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` passes (fmt + lint + test + build)
- Key translation covers all standard terminal key types
- Composer can send multi-line text to a PTY process
- Readiness detection correctly identifies Claude Code's composer prompt

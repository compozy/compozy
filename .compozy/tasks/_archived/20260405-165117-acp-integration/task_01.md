---
status: completed
title: ACP Agent Layer & Content Model
type: backend
complexity: high
---

# Task 1: ACP Agent Layer & Content Model

## Overview

Define the ACP content model types, vendor the `coder/acp-go-sdk` dependency, implement `Client` and `Session` interfaces that wrap the SDK over stdio, build the `AgentSpec` registry replacing the current `specs` map, and create a mock ACP server test helper for integration testing. This task lays the entire foundation that tasks 2 and 3 build upon.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST define all ACP content block types (text, tool_use, tool_result, diff, terminal_output, image) as typed Go structs
- MUST define `SessionUpdate`, `SessionStatus`, `Usage`, and `ContentBlock` types in `internal/core/model/content.go`
- MUST vendor `coder/acp-go-sdk` via `go get` — never hand-edit `go.mod`
- MUST implement `Client` interface that spawns agent binary and wires JSON-RPC 2.0 over stdin/stdout via the SDK
- MUST implement `Session` interface that streams `SessionUpdate` via a channel and exposes `Done()`/`Err()` for lifecycle
- MUST wrap SDK types behind Compozy's own interfaces — SDK types must NOT leak beyond `internal/core/agent/`
- MUST define `AgentSpec` struct and registry entries for all 7 agents (Claude, Codex, Droid, Cursor, OpenCode, Pi, Gemini)
- MUST preserve existing exported function signatures: `ValidateRuntimeConfig()`, `EnsureAvailable()`, `DisplayName()`, `BuildShellCommandString()`
- MUST build a mock ACP server test helper that emits configurable `SessionUpdate` sequences over stdio for integration tests
- MUST use `context.Context` for all lifecycle management — no fire-and-forget goroutines
</requirements>

## Subtasks

- [x] 1.1 Create `internal/core/model/content.go` with all ACP content block types, `SessionUpdate`, `SessionStatus`, `Usage` with `Add()` method
- [x] 1.2 Vendor `coder/acp-go-sdk` via `go get github.com/coder/acp-go-sdk@<latest-stable>`
- [x] 1.3 Create `internal/core/agent/client.go` implementing the `Client` interface — spawn agent binary, wire JSON-RPC 2.0 over stdio, create sessions
- [x] 1.4 Create `internal/core/agent/session.go` implementing the `Session` interface — stream `SessionUpdate` from SDK notifications, expose `Done()`/`Err()`
- [x] 1.5 Create `internal/core/agent/registry.go` with `AgentSpec` struct and entries for all 7 agents, plus ported `ValidateRuntimeConfig()`, `EnsureAvailable()`, `DisplayName()`, `BuildShellCommandString()` functions
- [x] 1.6 Create mock ACP server test helper (in `internal/core/agent/` test files or `internal/testutil/`) for integration testing
- [x] 1.7 Write unit and integration tests for all new code

## Implementation Details

The TechSpec §Core Interfaces defines the `Client`, `Session`, `SessionRequest`, `AgentSpec`, and `UpdateHandler` interface shapes. The TechSpec §Data Models defines all content block types and their fields. The TechSpec §Agent Registry Entries defines all 7 agent bootstrap configurations.

### Files to Create

- `internal/core/model/content.go` — ContentBlockType enum, ContentBlock, TextBlock, ToolUseBlock, ToolResultBlock, DiffBlock, TerminalOutputBlock, SessionStatus, SessionUpdate, Usage
- `internal/core/agent/client.go` — Client interface and implementation wrapping acp-go-sdk
- `internal/core/agent/session.go` — Session interface and implementation
- `internal/core/agent/registry.go` — AgentSpec struct, registry map, ValidateRuntimeConfig, EnsureAvailable, DisplayName, BuildShellCommandString

### Relevant Files

- `internal/core/agent/ide.go` — Current `spec` struct (line 15), `specs` map (line 31), all exported functions (lines 104-183) — being replaced by registry.go
- `internal/core/agent/ide_test.go` — Current test patterns (12 tests, parallel subtests, string assertions) — reference for new test style
- `internal/core/model/model.go` — IDE constants (lines 11-16), DefaultModel constants (lines 17-21), RuntimeConfig (lines 37-61) — preserved, referenced by registry
- `internal/core/run/types.go` — Current `TokenUsage` (line 145) and `ClaudeMessage` (line 167) — being replaced by model.Usage and model.SessionUpdate
- `go.mod` — Add `coder/acp-go-sdk` dependency

### Dependent Files

- `internal/core/run/execution.go` — Will consume `Client`/`Session` in task_02
- `internal/core/run/logging.go` — Will consume `SessionUpdate`/`ContentBlock` in task_02
- `internal/core/run/ui_model.go` — Will consume `ContentBlock` in task_03
- `internal/core/run/ui_view.go` — Will render typed content blocks in task_03

### Related ADRs

- [ADR-001: Full Replacement of Legacy Agent Drivers with ACP](../adrs/adr-001.md) — All agents communicate exclusively via ACP, no legacy fallback
- [ADR-002: Vendor coder/acp-go-sdk as ACP Client Foundation](../adrs/adr-002.md) — SDK wrapped behind our own interfaces, never leaked beyond agent package
- [ADR-003: Subprocess-over-Stdio for ACP Transport](../adrs/adr-003.md) — Spawn agent binary, JSON-RPC 2.0 over stdin/stdout
- [ADR-004: Redesigned Content Model with Typed ACP Content Blocks](../adrs/adr-004.md) — ACP-native content model, not mapped back to Claude types

## Deliverables

- `internal/core/model/content.go` with all ACP types and `Usage.Add()` method
- `internal/core/agent/client.go` with `Client` interface and implementation
- `internal/core/agent/session.go` with `Session` interface and implementation
- `internal/core/agent/registry.go` with `AgentSpec` registry for 7 agents and ported validation/utility functions
- Mock ACP server test helper for integration tests
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests verifying client spawn → session creation → update streaming → completion **(REQUIRED)**

## Tests

- Unit tests:
  - [x] Content block serialization: each block type round-trips through JSON correctly
  - [x] Content block deserialization: valid JSON produces correct typed structs
  - [x] Content block deserialization: malformed JSON returns descriptive error
  - [x] Usage.Add(): accumulates all fields correctly across multiple calls
  - [x] AgentSpec registry: all 7 agents produce correct binary names and bootstrap args
  - [x] AgentSpec registry: unknown IDE returns error
  - [x] ValidateRuntimeConfig: accepts all valid IDE values including opencode and pi
  - [x] ValidateRuntimeConfig: rejects PRD-tasks mode with batch size > 1
  - [x] ValidateRuntimeConfig: rejects invalid retry config
  - [x] EnsureAvailable: returns error when binary not on PATH
  - [x] DisplayName: returns correct display names for all agents
  - [x] BuildShellCommandString: produces correct shell preview for each agent
  - [x] Client.CreateSession: sends correct JSON-RPC request via mock stdio
  - [x] Session.Updates: streams SessionUpdate from mock responses
  - [x] Session.Done: closes after completion notification
  - [x] Session.Err: returns error from failed session
  - [x] Client.Close: terminates subprocess gracefully
- Integration tests:
  - [x] Full lifecycle: client spawn → session creation → update streaming → completion via mock ACP server
  - [x] Error handling: mock server sends error → client propagates structured error
  - [x] Context cancellation: cancelled context terminates session and process
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- `make verify` passes with zero lint issues
- SDK types do not appear in any package outside `internal/core/agent/`
- All 7 agents have registry entries with correct binary and bootstrap args
- Mock ACP server can replay configurable update sequences for integration tests

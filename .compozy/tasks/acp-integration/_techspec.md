# TechSpec: ACP (Agent Client Protocol) Integration

## Executive Summary

This specification describes the replacement of Compozy's custom per-agent CLI driver system with ACP (Agent Client Protocol), a JSON-RPC 2.0 protocol over stdio that standardizes communication between clients and coding agents. The current architecture requires hand-coded `commandFunc`/`shellPreviewFunc` implementations for each of 6 agents, with Claude-specific output parsing and heuristic timeout detection. ACP normalizes all agent communication into typed session updates with structured content blocks (text, tool_use, diff, terminal_output, etc.), eliminating per-agent branching throughout the codebase.

The implementation vendors `coder/acp-go-sdk` (Apache 2.0) as the protocol foundation, redesigns the content model to expose ACP's typed content blocks to the UI, and replaces the activity-monitor timeout with ACP's session lifecycle events. All 6 current agents (Claude, Codex, Droid, Cursor, OpenCode, Pi) have ACP adapters and will be migrated in a single release.

## System Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────────┐
│  CLI Layer (internal/cli/)                                   │
│  Unchanged — --ide flag selects agent, passes to core.Config │
└──────────────────────────┬──────────────────────────────────┘
                           │ core.Config
┌──────────────────────────▼──────────────────────────────────┐
│  Core API (internal/core/api.go)                             │
│  Config.runtime() builds RuntimeConfig, calls plan.Prepare   │
│  then run.Execute — unchanged public surface                 │
└──────────────────────────┬──────────────────────────────────┘
                           │ []model.Job + *model.RuntimeConfig
┌──────────────────────────▼──────────────────────────────────┐
│  Execution Pipeline (internal/core/run/)                     │
│                                                              │
│  execution.go  — Creates ACP Client per job instead of       │
│                  exec.Cmd. Session lifecycle replaces         │
│                  process watchdog.                            │
│  logging.go    — Processes SessionUpdate notifications.      │
│                  Replaces jsonFormatter + activityMonitor.    │
│  ui_model.go   — Receives typed ContentBlocks via uiCh.      │
│  ui_view.go    — Renders ContentBlocks by type (text, diff,  │
│                  tool_use, terminal_output).                  │
└──────────────────────────┬──────────────────────────────────┘
                           │ agent.Client / agent.Session
┌──────────────────────────▼──────────────────────────────────┐
│  ACP Agent Layer (internal/core/agent/)                       │
│                                                              │
│  registry.go  — AgentSpec map (replaces spec map in ide.go). │
│                 Bootstrap config: binary, args, env vars.     │
│  client.go    — ACP Client interface. Spawns agent binary,   │
│                 wires JSON-RPC 2.0 over stdio via SDK.        │
│  session.go   — ACP Session interface. Streams updates,      │
│                 exposes Done()/Err() for lifecycle.            │
│  validation.go — ValidateRuntimeConfig (preserved logic).    │
└──────────────────────────┬──────────────────────────────────┘
                           │ JSON-RPC 2.0 over stdin/stdout
┌──────────────────────────▼──────────────────────────────────┐
│  Agent Process (subprocess)                                   │
│  claude-agent-acp | codex-acp | gemini | opencode |          │
│  cursor-acp | droid                                           │
│  Speaks ACP protocol, emits SessionUpdate notifications       │
└─────────────────────────────────────────────────────────────┘
```

**Data flow**: CLI collects config → `core.Run()` prepares jobs → `run.Execute()` iterates jobs → for each job, `agent.NewClient()` spawns the agent subprocess → `client.CreateSession()` sends the prompt via ACP → `session.Updates()` streams `SessionUpdate` notifications → logging processes content blocks + usage → UI renders typed blocks → `session.Done()` signals completion.

## Implementation Design

### Core Interfaces

**ACP Client** — manages agent process lifecycle and session creation:

```go
// Client manages an ACP agent subprocess and creates sessions.
type Client interface {
    // CreateSession starts a new ACP session with the given prompt.
    CreateSession(ctx context.Context, req SessionRequest) (Session, error)
    // Close terminates the agent subprocess.
    Close() error
}

// Session represents an active ACP session streaming updates.
type Session interface {
    // Updates returns a channel of session update notifications.
    Updates() <-chan model.SessionUpdate
    // Done returns a channel that closes when the session completes.
    Done() <-chan struct{}
    // Err returns the session error, if any. Only valid after Done.
    Err() error
}

// SessionRequest contains the parameters for creating a session.
type SessionRequest struct {
    Prompt       []byte
    WorkingDir   string
    Model        string
    ExtraEnv     map[string]string
}
```

**Agent Registry** — replaces the `specs` map with ACP-native agent definitions:

```go
// AgentSpec defines how to bootstrap an ACP-compatible agent.
type AgentSpec struct {
    ID            string
    DisplayName   string
    DefaultModel  string
    Binary        string
    BootstrapArgs func(model, reasoningEffort string) []string
    EnvVars       map[string]string
}
```

**Content Block Consumer** — interface for processing typed updates:

```go
// UpdateHandler processes ACP session updates for logging and UI.
type UpdateHandler interface {
    HandleUpdate(update model.SessionUpdate)
    HandleCompletion(usage model.Usage, err error)
}
```

### Data Models

**New types in `internal/core/model/content.go`**:

```go
type ContentBlockType string

const (
    BlockText           ContentBlockType = "text"
    BlockToolUse        ContentBlockType = "tool_use"
    BlockToolResult     ContentBlockType = "tool_result"
    BlockDiff           ContentBlockType = "diff"
    BlockTerminalOutput ContentBlockType = "terminal_output"
    BlockImage          ContentBlockType = "image"
)

type ContentBlock struct {
    Type ContentBlockType
    Data json.RawMessage
}

type TextBlock struct {
    Text string
}

type ToolUseBlock struct {
    ID    string
    Name  string
    Input json.RawMessage
}

type ToolResultBlock struct {
    ToolUseID string
    Content   string
    IsError   bool
}

type DiffBlock struct {
    FilePath string
    Diff     string
}

type TerminalOutputBlock struct {
    Command  string
    Output   string
    ExitCode int
}

type SessionStatus string

const (
    StatusRunning   SessionStatus = "running"
    StatusCompleted SessionStatus = "completed"
    StatusFailed    SessionStatus = "failed"
)

type SessionUpdate struct {
    Blocks []ContentBlock
    Usage  Usage
    Status SessionStatus
}

type Usage struct {
    InputTokens  int
    OutputTokens int
    TotalTokens  int
    CacheReads   int
    CacheWrites  int
}

func (u *Usage) Add(other Usage) {
    u.InputTokens += other.InputTokens
    u.OutputTokens += other.OutputTokens
    u.TotalTokens += other.TotalTokens
    u.CacheReads += other.CacheReads
    u.CacheWrites += other.CacheWrites
}
```

**Removed types**:
- `ClaudeMessage` (logging.go)
- `TokenUsage` (types.go) — replaced by `model.Usage`
- `activityMonitor` (logging.go) — replaced by ACP session lifecycle

**Modified types**:
- `jobLogUpdateMsg` → `jobUpdateMsg` carrying `[]model.ContentBlock` instead of `[]string`
- `tokenUsageUpdateMsg` → `usageUpdateMsg` carrying `model.Usage`

### Agent Registry Entries

All 6 agents defined with their ACP adapter binaries and bootstrap args:

| Agent | Binary | Key Bootstrap Args |
|-------|--------|-------------------|
| Claude | `claude-agent-acp` | `--model <model>` |
| Codex | `codex-acp` | `--model <model>`, `--reasoning-effort <level>` |
| Droid | `droid` | ACP mode native, `--model <model>`, `--reasoning-effort <level>` |
| Cursor | `cursor-acp` | `--model <model>` |
| OpenCode | `opencode` | ACP mode native, `--model <model>`, `--thinking <level>` |
| Pi | `pi` | ACP mode native, `--model <model>`, `--thinking <level>` |
| Gemini | `gemini` | `--experimental-acp`, `--model <model>` |

Note: Agents with "ACP mode native" support ACP directly via a flag or default mode. Agents with separate adapter binaries (e.g., `claude-agent-acp`, `codex-acp`) are ACP wrappers around the original CLI.

## Impact Analysis

| Component | Impact Type | Description and Risk | Required Action |
|-----------|-------------|---------------------|-----------------|
| `internal/core/agent/ide.go` | Replaced | `spec` map, `Command()`, all build/command funcs removed. High impact — core driver logic. | Replace with `registry.go`, `client.go`, `session.go` |
| `internal/core/agent/ide_helpers.go` | Removed | Helper functions for IDE availability checks. Low risk. | Absorb relevant validation into `registry.go` and `client.go` |
| `internal/core/agent/ide_test.go` | Replaced | Tests for command construction. Medium impact. | Replace with ACP client tests using mock stdio |
| `internal/core/model/model.go` | Modified | IDE constants preserved. `RuntimeConfig` unchanged. Low risk. | No structural changes needed |
| `internal/core/model/content.go` | New | ACP content block types, session update, usage model. | New file |
| `internal/core/run/execution.go` | Modified | `createIDECommand()` → ACP Client/Session. `executeJobWithTimeout()` uses `session.Done()`. High impact. | Rewrite job execution to use ACP session lifecycle |
| `internal/core/run/logging.go` | Modified | `jsonFormatter`, `activityMonitor` removed. Replaced by `UpdateHandler`. High impact. | Rewrite output processing pipeline |
| `internal/core/run/types.go` | Modified | `TokenUsage`, `ClaudeMessage` removed. Medium impact. | Remove, replaced by `model.Usage` and `model.SessionUpdate` |
| `internal/core/run/ui_model.go` | Modified | Message types change to carry `ContentBlock`. Medium impact. | Update message types and model state |
| `internal/core/run/ui_view.go` | Modified | Render typed content blocks. Medium impact. | Add per-block-type rendering logic |
| `internal/core/api.go` | Unchanged | Public API surface (`Config`, `Run`, `Prepare`) unchanged. No risk. | None |
| `internal/cli/root.go` | Unchanged | CLI flags and form collection unchanged. No risk. | None |
| `go.mod` | Modified | Add `coder/acp-go-sdk` dependency. Low risk. | `go get github.com/coder/acp-go-sdk@<version>` |

## Testing Approach

### Unit Tests

- **ACP Client**: Mock `io.ReadWriter` to simulate JSON-RPC 2.0 exchanges. Test session creation, update streaming, error handling, and graceful close.
- **Agent Registry**: Test that all 6 agent specs produce correct binary names and bootstrap args. Test validation of unknown IDE values.
- **Content Block Parsing**: Test deserialization of each content block type (text, tool_use, tool_result, diff, terminal_output, image) from JSON.
- **Usage Aggregation**: Test `Usage.Add()` across multiple updates and concurrent access.
- **UpdateHandler**: Test that content blocks are correctly routed to log files and UI channel.

### Integration Tests

- **Mock ACP Server**: Build a test binary that speaks ACP protocol over stdio, emitting configurable sequences of session updates. Use this to test the full pipeline: client spawn → session creation → update streaming → completion.
- **Execution Pipeline**: Test `run.Execute()` with mock ACP server binary, verifying job lifecycle (queued → running → succeeded/failed), retry logic with ACP error codes, and timeout via context cancellation.
- **UI Message Flow**: Test that typed content blocks flow through `uiCh` and render correctly in the Bubble Tea model.
- **Test data**: Captured ACP session transcripts from real agent interactions, replayed by mock server.

## Development Sequencing

### Build Order

1. **Content Model** (`internal/core/model/content.go`) — no dependencies. Define `ContentBlock`, `SessionUpdate`, `Usage`, and all block types. Write unit tests for serialization/deserialization.

2. **ACP Client + Session** (`internal/core/agent/client.go`, `session.go`) — depends on step 1 and vendored SDK. Implement `Client` and `Session` interfaces wrapping `coder/acp-go-sdk`. Write unit tests with mock stdio.

3. **Agent Registry** (`internal/core/agent/registry.go`) — depends on step 2. Define `AgentSpec` entries for all 6 agents. Implement `ValidateRuntimeConfig()`, `EnsureAvailable()`, `DisplayName()`, `BuildShellCommandString()`. Write unit tests.

4. **Mock ACP Server** (test helper) — depends on steps 1-2. Build a test binary that speaks ACP protocol for integration testing. Not shipped in production binary.

5. **Execution Pipeline** (`internal/core/run/execution.go`) — depends on steps 2-3. Replace `createIDECommand()` with ACP client creation. Replace process watchdog with `session.Done()` + `ctx.Done()`. Update retry logic to use ACP error codes.

6. **Logging Pipeline** (`internal/core/run/logging.go`) — depends on steps 1, 5. Replace `jsonFormatter` with `UpdateHandler` that processes `SessionUpdate`. Replace `activityMonitor`. Route content blocks to file + UI channel.

7. **UI Adaptation** (`internal/core/run/ui_model.go`, `ui_view.go`) — depends on step 6. Update message types to carry `[]ContentBlock`. Add per-block-type rendering (text, diff with highlighting, tool calls, terminal output).

8. **Remove Legacy Code** — depends on steps 3-7. Delete `ide.go`, `ide_helpers.go`, `ClaudeMessage`, old `TokenUsage`. Clean up IDE-type branching in logging.

### Technical Dependencies

- **`coder/acp-go-sdk`**: Must be vendored before step 2. Install via `go get`.
- **ACP adapter binaries**: Not required for development (mock server used for testing), but required for production use. Each agent's ACP adapter must be available on PATH.
- **Bubble Tea / Lipgloss**: Already dependencies. UI rendering of typed content blocks uses existing framework capabilities.

## Monitoring and Observability

- **Structured logging via `slog`**: Log ACP session lifecycle events (session.created, session.update, session.completed, session.error) with agent ID, session ID, and duration fields.
- **Usage metrics**: Aggregate `Usage` (input/output tokens, cache reads/writes) per job and per run, displayed in summary output.
- **Error classification**: ACP error codes logged with structured fields for debugging adapter issues.
- **Content block counts**: Log count of each content block type per session for observability into agent behavior patterns.

## Technical Considerations

### Key Decisions

See Architecture Decision Records section below for full details on each decision.

### Known Risks

1. **ACP adapter maturity varies across agents**
   - Likelihood: Medium (Cursor and Pi adapters are newer)
   - Mitigation: Test each adapter in isolation. Document minimum adapter versions. The mock ACP server ensures our client works correctly regardless of adapter quality.

2. **`coder/acp-go-sdk` is young**
   - Likelihood: Low-medium (active development, backed by Coder)
   - Mitigation: Pin version, wrap behind interfaces, maintain ability to fork.

3. **All-or-nothing release**
   - Likelihood: N/A (design choice, not a risk event)
   - Mitigation: Comprehensive test suite with mock ACP server. Real agent smoke tests before release. Feature branch with full CI validation.

4. **UI complexity increase from typed content blocks**
   - Likelihood: Low (Bubble Tea handles variable-height content well)
   - Mitigation: Unknown block types fall back to raw text rendering. Incremental UI polish after functional correctness.

## Architecture Decision Records

ADRs documenting key decisions made during technical design:

- [ADR-001: Full Replacement of Legacy Agent Drivers with ACP](adrs/adr-001.md) — ACP fully replaces the current per-agent CLI driver system; no legacy fallback.
- [ADR-002: Vendor coder/acp-go-sdk as ACP Client Foundation](adrs/adr-002.md) — Use Coder's Go SDK for JSON-RPC 2.0 + ACP protocol, wrapped behind our own interfaces.
- [ADR-003: Subprocess-over-Stdio for ACP Transport](adrs/adr-003.md) — Spawn agent binaries as child processes, speak ACP over stdin/stdout. Local-first, no sidecars.
- [ADR-004: Redesigned Content Model with Typed ACP Content Blocks](adrs/adr-004.md) — New content model exposes all ACP block types (text, tool_use, diff, terminal_output, image) to the UI layer.
- [ADR-005: ACP Session Lifecycle for Timeout and Retry Management](adrs/adr-005.md) — Replace heuristic activity monitor with ACP session lifecycle events for timeout and retry decisions.

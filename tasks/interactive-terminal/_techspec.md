# Technical Specification: Interactive Terminal Embedding via PTY + VT Emulator

## Executive Summary

Replace the current headless execution model (`--print --output-format stream-json`) with an interactive PTY-based terminal embedding that renders live Claude Code TUI sessions inside Looper's Bubbletea UI. Each job spawns Claude Code in a pseudo-terminal, feeds its output through `charmbracelet/x/vt` (a virtual terminal emulator), and renders the result in a Bubbletea viewport. Users can observe agent work in real-time and interact directly with any running instance. Job completion is signaled explicitly by the agent via an HTTP endpoint (`looper job done --id <id>`) served by a local Fiber server, replacing the JSON-based exit code detection. System prompts are mode-specific (PRD tasks, fix reviews, etc.) and the initial task prompt is a short composer message pointing to the full prompt file in `.tmp/`.

Key trade-offs: dropping stream-json removes structured token usage parsing (to be replaced by hooks or API-level telemetry in the future), and `charmbracelet/x/vt` is pre-v1 (no tagged releases, potential breaking changes). The gain is a fundamentally richer execution experience — full observability, live interaction, and deterministic job signaling.

## System Architecture

### Component Overview

```
CLI Layer (unchanged entry point)
  looper start / looper fix-reviews
       |
       v
Execution Layer (refactored)
  internal/looper/run/
    execution.go       → Refactored: PTY-based job execution, Fiber server lifecycle
    terminal.go        → NEW: PTY + VT emulator wrapper component
    signal_server.go   → NEW: Fiber HTTP server for agent-to-looper signaling
    command_io.go      → REMOVED: stream-json taps replaced by PTY
    logging.go         → SIMPLIFIED: remove jsonFormatter, keep activityMonitor
       |
       v
Agent Layer (refactored)
  internal/looper/agent/
    ide.go             → Refactored: Claude runs interactive (no --print/--output-format)
       |
       v
Prompt Layer (modified)
  internal/looper/prompt/
    system.go          → NEW: mode-specific system prompts (PRD tasks, fix reviews)
    review.go          → Modified: system prompt includes job-done instruction
    task.go            → Modified: system prompt includes job-done instruction
       |
       v
UI Layer (refactored)
  internal/looper/run/
    ui_model.go        → Refactored: terminal viewport replaces log viewport
    ui_update.go       → Refactored: PTY output messages, key forwarding to active terminal
    ui_view.go         → Refactored: render VT emulator screen in main pane
    ui_messages.go     → NEW: PTY-specific message types
```

### Data Flow

```
Job Start:
  Execute() → startSignalServer(:9877)
           → for each job:
               xpty.NewPty(w, h) → exec.Command("claude", "--system-prompt", ...) → pty.Start(cmd)
               vt.NewSafeEmulator(w, h)
               goroutine: pty.Read() → emu.Write() → uiCh <- terminalOutputMsg
               goroutine: emu.Read() → pty.Write()  (response forwarding)

User Interaction:
  Bubbletea KeyMsg → if job selected & focused:
                       translate key → pty.Write(keyBytes)
                     else:
                       handle navigation (up/down/tab)

Job Completion:
  Agent executes: curl -X POST localhost:9877/job/done --data '{"id":"batch-001"}'
  → Fiber handler receives → uiCh <- jobDoneSignalMsg{ID: "batch-001"}
  → UI marks job complete, switches view to next pending job
  → PTY stays alive for user to revisit

Readiness Detection:
  After pty.Start():
    goroutine polls emu.String() every 200ms
    → detects composer prompt pattern ("> " or cursor at input line)
    → sends readyMsg → composer simulation sends initial prompt
    → fallback: 15s timeout → send anyway
```

## Implementation Design

### Core Interfaces

```go
// internal/looper/run/terminal.go

// Terminal wraps a PTY process and VT emulator for a single job.
type Terminal struct {
    emu    *vt.SafeEmulator
    pty    xpty.Pty
    cmd    *exec.Cmd
    width  int
    height int
    jobID  string
    alive  bool
    mu     sync.RWMutex
}

func NewTerminal(width, height int, jobID string) *Terminal
func (t *Terminal) Start(cmd *exec.Cmd) error
func (t *Terminal) Render() string
func (t *Terminal) WriteInput(p []byte) error
func (t *Terminal) Resize(w, h int)
func (t *Terminal) IsAlive() bool
func (t *Terminal) Close() error
```

```go
// internal/looper/run/signal_server.go

// SignalServer receives job lifecycle signals from agents via HTTP.
type SignalServer struct {
    app      *fiber.App
    eventCh  chan<- SignalEvent
    port     int
    listener net.Listener
}

type SignalEvent struct {
    Type  string // "done", "status", "error"
    JobID string
    Data  map[string]string
}

func NewSignalServer(port int, eventCh chan<- SignalEvent) *SignalServer
func (s *SignalServer) Start() error
func (s *SignalServer) Shutdown(ctx context.Context) error
func (s *SignalServer) Port() int
```

```go
// internal/looper/prompt/system.go

// SystemPrompt builds mode-specific system prompts that include
// job signaling instructions and skill references.
func BuildSystemPrompt(mode model.ExecutionMode, jobID string, serverPort int) string
```

### Data Models

**Terminal message types** (new `ui_messages.go`):

```go
type terminalOutputMsg struct {
    Index int    // job index
    Data  []byte // raw PTY output (already written to emulator)
}

type terminalReadyMsg struct {
    Index int
}

type jobDoneSignalMsg struct {
    JobID string
}

type composerSendMsg struct {
    Index   int
    Message string
}
```

**RuntimeConfig changes** (`model/model.go`):

```go
type RuntimeConfig struct {
    // existing fields...
    SignalPort int // Port for the local Fiber signal server (default: 9877)
}
```

### Composer Simulation

The composer simulation sends the initial prompt to Claude Code after detecting readiness:

```go
// internal/looper/run/composer.go

func sendComposerInput(pty xpty.Pty, message string) error {
    // Normalize line endings
    normalized := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(message)
    for _, line := range strings.Split(normalized, "\n") {
        // Send literal text
        pty.Write([]byte(line))
        // Ctrl+J for newline in composer (doesn't submit)
        pty.Write([]byte{0x0a})
    }
    // Enter to submit
    pty.Write([]byte{0x0d})
    return nil
}
```

The initial prompt is short, pointing to the full prompt file:

```
Read and execute the task described in /absolute/path/to/.tmp/batch-001.md
```

### System Prompt Structure

Each mode gets a tailored `--system-prompt`:

```go
func BuildSystemPrompt(mode model.ExecutionMode, jobID string, serverPort int) string {
    base := fmt.Sprintf(`When you have completed the task, you MUST run this command:
curl -s -X POST http://localhost:%d/job/done -H 'Content-Type: application/json' -d '{"id":"%s"}'
This signals the orchestrator that your work is finished. Do NOT skip this step.`, serverPort, jobID)

    switch mode {
    case model.ExecutionModePRDTasks:
        return prdTaskSystemPrompt + "\n\n" + base
    case model.ExecutionModePRReview:
        return reviewSystemPrompt + "\n\n" + base
    }
    return base
}
```

### Agent Command Changes

Claude Code command construction changes from headless to interactive:

```go
// Before (headless):
// claude --print --output-format stream-json --verbose --model opus ...

// After (interactive):
// claude --model opus --system-prompt "..." --dangerously-skip-permissions --add-dir ...
func claudeCommand(ctx context.Context, model string, addDirs []string, systemPrompt string) *exec.Cmd {
    args := []string{
        "--model", model,
        "--system-prompt", systemPrompt,
        "--permission-mode", "bypassPermissions",
        "--dangerously-skip-permissions",
    }
    args = appendAddDirs(args, addDirs)
    return exec.CommandContext(ctx, "claude", args...)
}
```

Removed flags: `--print`, `--output-format stream-json`, `--verbose`, `--append-system-prompt`.
New flag: `--system-prompt` (replaces `--append-system-prompt`, full system prompt).

### Readiness Detection

```go
// internal/looper/run/readiness.go

const (
    readinessPollInterval = 200 * time.Millisecond
    readinessTimeout      = 15 * time.Second
)

// waitForReady polls the VT emulator screen until it detects
// the Claude Code composer is ready for input.
func waitForReady(ctx context.Context, emu *vt.SafeEmulator) error {
    deadline := time.After(readinessTimeout)
    ticker := time.NewTicker(readinessPollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            screen := emu.String()
            if detectComposerReady(screen) {
                return nil
            }
        case <-deadline:
            return nil // fallback: send anyway after timeout
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}

func detectComposerReady(screen string) bool {
    lines := strings.Split(screen, "\n")
    for i := len(lines) - 1; i >= 0; i-- {
        trimmed := strings.TrimSpace(lines[i])
        if trimmed == "" {
            continue
        }
        // Claude Code composer indicators
        return strings.HasPrefix(trimmed, ">") ||
            strings.Contains(trimmed, "What can I help") ||
            strings.Contains(trimmed, "Type your")
    }
    return false
}
```

### Key Forwarding

```go
// internal/looper/run/keytranslate.go

// translateKey converts a Bubbletea KeyMsg to terminal escape bytes.
func translateKey(msg tea.KeyMsg) []byte {
    switch msg.Type {
    case tea.KeyEnter:
        return []byte{0x0d}
    case tea.KeyTab:
        return []byte{0x09}
    case tea.KeyBackspace:
        return []byte{0x7f}
    case tea.KeyUp:
        return []byte{0x1b, '[', 'A'}
    case tea.KeyDown:
        return []byte{0x1b, '[', 'B'}
    case tea.KeyRight:
        return []byte{0x1b, '[', 'C'}
    case tea.KeyLeft:
        return []byte{0x1b, '[', 'D'}
    case tea.KeyCtrlC:
        return []byte{0x03}
    case tea.KeyCtrlD:
        return []byte{0x04}
    case tea.KeyEsc:
        return []byte{0x1b}
    case tea.KeyRunes:
        return []byte(string(msg.Runes))
    }
    return nil
}
```

### UI Model Changes

The `uiModel` gains terminal references and interaction mode:

```go
type interactionMode int

const (
    modeNavigate  interactionMode = iota // arrow keys navigate sidebar
    modeTerminal                         // keys forwarded to active PTY
)

type uiModel struct {
    // existing fields (jobs, sidebar, viewport, etc.)...
    terminals   []*Terminal       // one per job
    mode        interactionMode   // navigate vs terminal input
    signalCh    <-chan SignalEvent // from Fiber server
}
```

Key handling in `Update()`:

```
modeNavigate:
  up/down   → select job in sidebar
  Enter     → switch to modeTerminal for selected job
  q         → quit

modeTerminal:
  Esc       → switch back to modeNavigate
  all other → forward to active job's PTY via translateKey()
```

### Signal Server Endpoints

```
POST /job/done    {"id": "batch-001"}
  → Marks job as completed, triggers view switch to next pending job

POST /job/status  {"id": "batch-001", "status": "working on tests"}
  → Optional: updates job status text in sidebar (future use)

GET  /health
  → Returns 200 OK (for agent connectivity check)
```

## Integration Points

### Fiber HTTP Server

- Binds to `localhost:<port>` (default 9877, configurable via `--signal-port`)
- Started before any jobs launch
- Shutdown gracefully after all jobs complete or on SIGINT/SIGTERM
- No authentication required (localhost-only, ephemeral)

### Claude Code CLI

- Invoked in interactive mode (no `--print`)
- System prompt injected via `--system-prompt` flag
- Permissions bypassed via `--dangerously-skip-permissions`
- Additional directories via `--add-dir`
- Agent teams enabled via `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1` env var

### PTY / VT Libraries

- `github.com/charmbracelet/x/xpty` — cross-platform PTY management
- `github.com/charmbracelet/x/vt` — virtual terminal emulator (SafeEmulator for thread safety)
- Both are pre-v1; pinned to specific pseudo-versions

## Impact Analysis

| Component                           | Impact Type    | Description and Risk                                                         | Required Action                                     |
| ----------------------------------- | -------------- | ---------------------------------------------------------------------------- | --------------------------------------------------- |
| `internal/looper/run/execution.go`  | Major refactor | PTY-based execution replaces exec.Command+pipe. High risk.                   | Rewrite job execution to use Terminal wrapper       |
| `internal/looper/run/command_io.go` | Removed        | stream-json IO taps no longer needed. Low risk.                              | Delete file                                         |
| `internal/looper/run/logging.go`    | Simplified     | Remove jsonFormatter, uiLogTap. Keep activityMonitor, lineRing. Medium risk. | Remove JSON-specific code, keep monitoring          |
| `internal/looper/run/ui_model.go`   | Major refactor | Terminal viewport replaces log viewport. High risk.                          | Add Terminal refs, interaction mode, signal channel |
| `internal/looper/run/ui_update.go`  | Major refactor | Key forwarding, PTY messages, signal handling. High risk.                    | Rewrite Update() for terminal interaction           |
| `internal/looper/run/ui_view.go`    | Major refactor | Render VT emulator screen instead of log lines. High risk.                   | Rewrite main pane rendering                         |
| `internal/looper/agent/ide.go`      | Modified       | Claude command drops --print, gains --system-prompt. Medium risk.            | Update claudeCommand() and buildClaudeCommand()     |
| `internal/looper/prompt/`           | New files      | System prompt builders per mode. Low risk.                                   | Add system.go with mode-specific prompts            |
| `internal/looper/model/model.go`    | Modified       | Add SignalPort field. Low risk.                                              | Add field to RuntimeConfig                          |
| `go.mod`                            | New deps       | fiber, x/vt, x/xpty. Medium risk (pre-v1 deps).                              | go get new dependencies                             |

## Testing Approach

### Unit Tests

- **Terminal**: test Start/Render/WriteInput/Resize/Close lifecycle; test concurrent Read+Render via SafeEmulator
- **SignalServer**: test endpoint responses, event channel delivery, graceful shutdown
- **Readiness detection**: test detectComposerReady() with various screen states (loading, ready, error)
- **Composer simulation**: test sendComposerInput() with single-line, multi-line, special characters
- **Key translation**: test translateKey() for all supported key types (arrows, ctrl sequences, runes)
- **System prompt builder**: test BuildSystemPrompt() for each mode includes job-done instruction and mode-specific content

### Integration Tests

- **Terminal + PTY**: spawn a simple command (e.g., `echo hello`) in PTY, verify VT emulator captures output
- **Signal flow**: start SignalServer, POST /job/done, verify event arrives on channel
- **Readiness + Composer**: spawn a mock process that prints "> ", verify readiness detected and prompt sent
- **UI model**: test modeNavigate → modeTerminal transitions, key forwarding behavior

## Development Sequencing

### Build Order

1. **Terminal wrapper** (`terminal.go`) — PTY + VT emulator lifecycle. Foundation for everything else. No existing code depends on it.

2. **Signal server** (`signal_server.go`) — Fiber HTTP server with /job/done endpoint. Independent of Terminal, can be developed in parallel with step 1.

3. **Key translation + Composer simulation** (`keytranslate.go`, `composer.go`) — Input handling utilities. Depends on Terminal for testing.

4. **Readiness detection** (`readiness.go`) — VT screen polling. Depends on Terminal.

5. **System prompt builder** (`prompt/system.go`) — Mode-specific prompts with job-done instructions. Independent, can parallelize with steps 1-4.

6. **Agent command refactor** (`agent/ide.go`) — Drop --print, add --system-prompt. Depends on step 5 for prompt content.

7. **UI refactor** (`ui_model.go`, `ui_update.go`, `ui_view.go`, `ui_messages.go`) — Terminal rendering, key forwarding, signal handling. Depends on steps 1-4.

8. **Execution pipeline refactor** (`execution.go`) — Wire Terminal + SignalServer + UI together. Depends on all previous steps.

9. **Logging cleanup** (`logging.go`, `command_io.go`) — Remove stream-json code. After step 8 confirms everything works.

10. **Integration testing + polish** — End-to-end flow validation, edge cases.

### Technical Dependencies

- `github.com/charmbracelet/x/vt` (pseudo-version, pinned)
- `github.com/charmbracelet/x/xpty` (pseudo-version, pinned)
- `github.com/gofiber/fiber/v3` (new dependency)
- Claude Code CLI must be installed and available on PATH

## Monitoring and Observability

- **Job lifecycle**: `slog.Info` for job start, ready detection, prompt sent, job-done signal received, job completion
- **PTY errors**: `slog.Error` for PTY read/write failures, emulator errors
- **Signal server**: `slog.Info` for incoming requests, `slog.Warn` for unknown job IDs
- **Readiness**: `slog.Info` when composer detected ready, `slog.Warn` on timeout fallback
- **Activity monitor**: existing timeout detection remains for safety net (agent crashes without sending done signal)

## Technical Considerations

### Key Decisions

1. **PTY + VT emulator over headless stream-json** — Enables real-time observability and direct interaction with running agents. Trade-off: loses structured token usage data, gains full TUI rendering and user intervention capability. Token usage can be recovered later via Claude Code hooks or API telemetry.

2. **Fiber HTTP for job signaling over file watchers or Unix sockets** — HTTP is debuggable (`curl`), proven in the agh POC, and requires no platform-specific code. Trade-off: opens a TCP port on localhost, but it's ephemeral and local-only.

3. **Composer simulation over --prompt flag** — The `--prompt`/`-p` flag implies `--print` (headless mode), which defeats the purpose of interactive terminal embedding. Composer simulation keeps the TUI alive. Trade-off: requires readiness detection and timing logic, but VT screen polling makes this deterministic.

4. **Mode-specific --system-prompt over --append-system-prompt** — Full system prompt gives complete control over agent behavior per execution mode. Each mode (PRD tasks, fix reviews) gets tailored instructions including the job-done signaling protocol.

5. **charmbracelet/x/vt over hinshun/vt10x** — Better ecosystem integration (Charm team), actively developed, supports SafeEmulator for thread safety, damage tracking, and scrollback. Trade-off: pre-v1 with no tagged releases, but already used by 5+ production projects including tuios (2.6k stars).

6. **SafeEmulator (thread-safe) over standard Emulator** — PTY read goroutine calls Write() while Bubbletea render calls Render() concurrently. SafeEmulator uses RWMutex internally. No performance concern for single-terminal rendering.

7. **Completed job PTYs stay alive** — Users can navigate back to completed jobs and see the full terminal history (scrollback). PTYs are only killed on looper exit. Trade-off: memory usage for long-running sessions with many jobs, mitigated by VT scrollback limits.

### Known Risks

1. **`charmbracelet/x/vt` breaking changes** — Pre-v1 library with no stability guarantees. Mitigation: pin to specific pseudo-version in go.mod, wrap in Terminal abstraction to isolate API surface.

2. **Claude Code TUI rendering complexity** — Claude Code uses Ink (React for terminal) with complex ANSI sequences, spinners, box drawing. The VT emulator may not render all sequences perfectly. Mitigation: `x/vt` handles full VT220+ sequences; test with real Claude Code sessions early.

3. **Readiness detection false positives** — The composer prompt pattern ("> ") could appear in loading text or other output. Mitigation: check last non-empty line only, combine with minimum elapsed time (e.g., 2s after PTY start), fallback to 15s timeout.

4. **Agent forgets to send done signal** — LLM might not always execute the curl command. Mitigation: existing activity timeout serves as safety net; system prompt uses strong language ("MUST", "Do NOT skip").

5. **Port conflicts** — Default port 9877 could be in use. Mitigation: configurable via `--signal-port` flag; server reports clear error on bind failure.

6. **Large prompt files** — Composer simulation for very long messages could be slow (character-by-character). Mitigation: prompt sent via composer is short (one-liner pointing to `.tmp/` file); full task content is read by the agent from file.

## Architecture Decision Records

- [ADR-001: PTY + VT Emulator for Terminal Embedding](adrs/adr-001.md) — Use xpty + charmbracelet/x/vt SafeEmulator to embed interactive Claude Code sessions in Bubbletea viewports
- [ADR-002: Fiber HTTP Server for Job Signaling](adrs/adr-002.md) — Local Fiber HTTP server replaces exit-code-based job completion detection with explicit agent-to-orchestrator signaling
- [ADR-003: Composer Simulation for Initial Prompt](adrs/adr-003.md) — Send initial task prompt via PTY keystroke simulation after VT screen readiness detection, keeping Claude Code in interactive TUI mode
- [ADR-004: Mode-Specific System Prompts](adrs/adr-004.md) — Use --system-prompt flag with mode-tailored prompts that include job-done signaling protocol and execution-specific instructions

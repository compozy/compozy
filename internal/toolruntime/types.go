// Package toolruntime tracks long-running tool subprocess ownership and scoped interrupts.
package toolruntime

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

const (
	maxCommandLength = 256
	maxArgs          = 32
)

var (
	// ErrProcessNotFound reports that an interrupt scope matched no active process records.
	ErrProcessNotFound = errors.New("toolruntime: process not found")
	// ErrOwnershipValidationFailed reports that a recovered PID no longer matches
	// the stored process ownership evidence.
	ErrOwnershipValidationFailed = errors.New("toolruntime: process ownership validation failed")
)

// ProcessState is the durable lifecycle state for a tracked process.
type ProcessState string

const (
	ProcessStateRunning      ProcessState = "running"
	ProcessStateInterrupting ProcessState = "interrupting"
	ProcessStateInterrupted  ProcessState = "interrupted"
	ProcessStateCompleted    ProcessState = "completed"
	ProcessStateFailed       ProcessState = "failed"
	ProcessStateStale        ProcessState = "stale"
)

// ProcessSource identifies the AGH subsystem that launched a process.
type ProcessSource string

const (
	ProcessSourceACPAgent        ProcessSource = "acp_agent"
	ProcessSourceACPTerminal     ProcessSource = "acp_terminal"
	ProcessSourceSandboxTerminal ProcessSource = "sandbox_terminal"
	ProcessSourceHook            ProcessSource = "hook"
	ProcessSourceExtension       ProcessSource = "extension"
	ProcessSourceSubprocess      ProcessSource = "subprocess"
)

// ProcessOwner captures stable owner IDs used for scoped interrupts.
type ProcessOwner struct {
	SessionID     string
	TurnID        string
	ToolCallID    string
	TerminalID    string
	ExtensionName string
	HookName      string
	SandboxID     string
}

// ProcessRecord is the checkpointed process ownership record.
type ProcessRecord struct {
	ID             string
	Source         ProcessSource
	Owner          ProcessOwner
	PID            int
	ProcessGroupID int
	Command        string
	Args           []string
	Cwd            string
	StartedAt      time.Time
	StartedByPID   int
	State          ProcessState
	ExitCode       *int
	Error          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

// ProcessCheckpoint mutates checkpointable fields for a tracked record.
type ProcessCheckpoint struct {
	Owner          *ProcessOwner
	PID            *int
	ProcessGroupID *int
	StartedAt      *time.Time
	State          ProcessState
	Error          string
	UpdatedAt      time.Time
}

// ProcessCompletion captures the terminal outcome for a tracked process.
type ProcessCompletion struct {
	ExitCode *int
	Err      error
	Error    string
}

// ProcessQuery filters persisted process records.
type ProcessQuery struct {
	IDs    []string
	States []ProcessState
	Scope  InterruptScope
	Limit  int
}

// InterruptScope targets one process, tool call, turn, session, hook, or extension.
type InterruptScope struct {
	ProcessID     string
	SessionID     string
	TurnID        string
	ToolCallID    string
	TerminalID    string
	ExtensionName string
	HookName      string
	Source        ProcessSource
	Reason        string
}

// IsZero reports whether the scope carries any selector.
func (s InterruptScope) IsZero() bool {
	normalized := s.Normalize()
	return normalized.ProcessID == "" &&
		normalized.SessionID == "" &&
		normalized.TurnID == "" &&
		normalized.ToolCallID == "" &&
		normalized.TerminalID == "" &&
		normalized.ExtensionName == "" &&
		normalized.HookName == "" &&
		normalized.Source == ""
}

// Normalize trims every string selector in the scope.
func (s InterruptScope) Normalize() InterruptScope {
	return InterruptScope{
		ProcessID:     strings.TrimSpace(s.ProcessID),
		SessionID:     strings.TrimSpace(s.SessionID),
		TurnID:        strings.TrimSpace(s.TurnID),
		ToolCallID:    strings.TrimSpace(s.ToolCallID),
		TerminalID:    strings.TrimSpace(s.TerminalID),
		ExtensionName: strings.TrimSpace(s.ExtensionName),
		HookName:      strings.TrimSpace(s.HookName),
		Source:        ProcessSource(strings.TrimSpace(string(s.Source))),
		Reason:        trimBounded(s.Reason),
	}
}

// ProcessStateUpdate is the storage-level state mutation for a record.
type ProcessStateUpdate struct {
	ID          string
	State       ProcessState
	ExitCode    *int
	Error       string
	UpdatedAt   time.Time
	CompletedAt *time.Time
}

// Store is the durable persistence boundary consumed by Registry.
type Store interface {
	UpsertProcessRecord(ctx context.Context, record ProcessRecord) error
	UpdateProcessRecordState(ctx context.Context, update ProcessStateUpdate) error
	ListProcessRecords(ctx context.Context, query ProcessQuery) ([]ProcessRecord, error)
}

func normalizeRecord(record ProcessRecord, now time.Time, daemonPID int) ProcessRecord {
	record.ID = strings.TrimSpace(record.ID)
	record.Source = ProcessSource(strings.TrimSpace(string(record.Source)))
	record.Owner = normalizeOwner(record.Owner)
	record.Command = trimBounded(record.Command)
	record.Args = sanitizeArgs(record.Args)
	record.Cwd = trimBounded(record.Cwd)
	record.Error = trimBounded(record.Error)
	if record.ProcessGroupID < 0 {
		record.ProcessGroupID = 0
	}
	if record.PID < 0 {
		record.PID = 0
	}
	if record.State == "" {
		record.State = ProcessStateRunning
	}
	if record.StartedByPID <= 0 {
		record.StartedByPID = daemonPID
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = now
	}
	if record.StartedAt.IsZero() && record.PID <= 0 {
		record.StartedAt = now
	}
	return record
}

func normalizeOwner(owner ProcessOwner) ProcessOwner {
	return ProcessOwner{
		SessionID:     strings.TrimSpace(owner.SessionID),
		TurnID:        strings.TrimSpace(owner.TurnID),
		ToolCallID:    strings.TrimSpace(owner.ToolCallID),
		TerminalID:    strings.TrimSpace(owner.TerminalID),
		ExtensionName: strings.TrimSpace(owner.ExtensionName),
		HookName:      strings.TrimSpace(owner.HookName),
		SandboxID:     strings.TrimSpace(owner.SandboxID),
	}
}

func validateRecord(record ProcessRecord) error {
	if strings.TrimSpace(record.ID) == "" {
		return errors.New("toolruntime: process id is required")
	}
	if err := record.Source.Validate(); err != nil {
		return err
	}
	if err := record.State.Validate(); err != nil {
		return err
	}
	return nil
}

// Validate ensures the source is known.
func (s ProcessSource) Validate() error {
	switch s {
	case ProcessSourceACPAgent,
		ProcessSourceACPTerminal,
		ProcessSourceSandboxTerminal,
		ProcessSourceHook,
		ProcessSourceExtension,
		ProcessSourceSubprocess:
		return nil
	default:
		return fmt.Errorf("toolruntime: invalid process source %q", s)
	}
}

// Validate ensures the state is known.
func (s ProcessState) Validate() error {
	switch s {
	case ProcessStateRunning,
		ProcessStateInterrupting,
		ProcessStateInterrupted,
		ProcessStateCompleted,
		ProcessStateFailed,
		ProcessStateStale:
		return nil
	default:
		return fmt.Errorf("toolruntime: invalid process state %q", s)
	}
}

func activeStates() []ProcessState {
	return []ProcessState{ProcessStateRunning, ProcessStateInterrupting}
}

func terminalStates() []ProcessState {
	return []ProcessState{
		ProcessStateInterrupted,
		ProcessStateCompleted,
		ProcessStateFailed,
		ProcessStateStale,
	}
}

func isTerminalState(state ProcessState) bool {
	return slices.Contains(terminalStates(), state)
}

func trimBounded(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= maxCommandLength {
		return trimmed
	}
	return trimmed[:maxCommandLength]
}

func sanitizeArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	limit := min(len(args), maxArgs)
	sanitized := make([]string, 0, limit)
	redactNext := false
	for idx := range limit {
		arg := strings.TrimSpace(args[idx])
		if redactNext {
			sanitized = append(sanitized, "[redacted]")
			redactNext = false
			continue
		}
		lower := strings.ToLower(arg)
		if sensitiveFlag(lower) {
			if strings.Contains(arg, "=") {
				key, _, _ := strings.Cut(arg, "=")
				sanitized = append(sanitized, trimBounded(key+"=[redacted]"))
			} else {
				sanitized = append(sanitized, trimBounded(arg))
				redactNext = true
			}
			continue
		}
		if sensitiveInline(lower) {
			sanitized = append(sanitized, "[redacted]")
			continue
		}
		sanitized = append(sanitized, trimBounded(arg))
	}
	if len(args) > maxArgs {
		sanitized = append(sanitized, "[truncated]")
	}
	return sanitized
}

func sensitiveFlag(lower string) bool {
	if !strings.HasPrefix(lower, "-") {
		return false
	}
	return strings.Contains(lower, "token") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "password") ||
		strings.Contains(lower, "passwd") ||
		strings.Contains(lower, "api-key") ||
		strings.Contains(lower, "apikey") ||
		strings.Contains(lower, "client-key") ||
		strings.Contains(lower, "client_secret") ||
		strings.Contains(lower, "authorization")
}

func sensitiveInline(lower string) bool {
	return strings.HasPrefix(lower, "bearer ") ||
		strings.HasPrefix(lower, "token=") ||
		strings.HasPrefix(lower, "password=") ||
		strings.HasPrefix(lower, "secret=") ||
		strings.HasPrefix(lower, "authorization=")
}

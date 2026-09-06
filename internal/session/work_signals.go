package session

import (
	"context"
	"time"
)

// WorkSignalKind names the authoritative sources allowed to defer inactivity stops.
type WorkSignalKind string

const (
	WorkSignalAgentProgress WorkSignalKind = "agent_progress"
	WorkSignalToolRunning   WorkSignalKind = "tool_running"
	WorkSignalActiveChild   WorkSignalKind = "active_child"
	WorkSignalLoopRun       WorkSignalKind = "loop_run"
	WorkSignalTaskLease     WorkSignalKind = "task_lease"
	WorkSignalScheduledWait WorkSignalKind = "scheduled_wait"
)

// WorkSignal is ephemeral evidence of work; it never enters session persistence.
type WorkSignal struct {
	Kind  WorkSignalKind `json:"kind"`
	Since time.Time      `json:"since"`
	Ref   string         `json:"ref,omitempty"`

	ValidUntil     time.Time `json:"-"`
	StaleAttention bool      `json:"-"`
}

// SignalSource inspects one authoritative subsystem without renewing its evidence.
type SignalSource interface {
	Signals(context.Context, string) ([]WorkSignal, error)
}

// SignalSourceState makes absence, stale evidence and failed inspections explicit.
type SignalSourceState struct {
	Kind  WorkSignalKind `json:"kind"`
	State string         `json:"state"`
	Ref   string         `json:"ref,omitempty"`
	Error string         `json:"error,omitempty"`
}

// QuietWarning records one quiet episode and its optional inactivity-stop deadline.
type QuietWarning struct {
	QuietSince time.Time  `json:"quiet_since"`
	WarnedAt   time.Time  `json:"warned_at"`
	StopAt     *time.Time `json:"stop_at"`
}

// SupervisionState is the session's last authoritative work inspection.
type SupervisionState struct {
	WorkSignals  []WorkSignal        `json:"work_signals"`
	Sources      []SignalSourceState `json:"sources"`
	QuietWarning *QuietWarning       `json:"quiet_warning"`
}

// CloneSupervisionState isolates runtime read models and preserves empty arrays.
func CloneSupervisionState(state *SupervisionState) *SupervisionState {
	cloned := &SupervisionState{WorkSignals: []WorkSignal{}, Sources: []SignalSourceState{}}
	if state == nil {
		return cloned
	}
	cloned.WorkSignals = append(cloned.WorkSignals, state.WorkSignals...)
	cloned.Sources = append(cloned.Sources, state.Sources...)
	if state.QuietWarning != nil {
		warning := *state.QuietWarning
		warning.StopAt = cloneTimePointer(warning.StopAt)
		cloned.QuietWarning = &warning
	}
	return cloned
}

// WorkSignalKindValues defines the closed vocabulary shared with public schemas.
func WorkSignalKindValues() []string {
	return []string{
		string(WorkSignalAgentProgress), string(WorkSignalToolRunning), string(WorkSignalActiveChild),
		string(WorkSignalLoopRun), string(WorkSignalTaskLease), string(WorkSignalScheduledWait),
	}
}

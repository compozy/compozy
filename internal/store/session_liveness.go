package store

import (
	"fmt"
	"strings"
	"time"
)

const (
	// SessionStallStateDetected marks a session whose ACP activity timed out
	// while the subprocess still appeared alive during recovery.
	SessionStallStateDetected = "stalled"

	// SessionStallReasonActivityTimeout reports that no ACP activity was observed
	// within the configured supervision window.
	SessionStallReasonActivityTimeout = "activity_timeout"

	// SessionStallReasonPromptDeadlineExceeded reports that the prompt-level
	// supervision deadline elapsed before completion.
	SessionStallReasonPromptDeadlineExceeded = "prompt_deadline_exceeded"

	// SessionStallReasonProcessUnhealthy reports that subprocess health checks
	// failed while a prompt was still active.
	SessionStallReasonProcessUnhealthy = "process_unhealthy"
)

// SessionLivenessMeta is the persisted runtime supervision state for one
// ACP-backed session.
type SessionLivenessMeta struct {
	SubprocessPID       int                  `json:"subprocess_pid,omitempty"`
	SubprocessStartedAt *time.Time           `json:"subprocess_started_at,omitempty"`
	LastUpdateAt        *time.Time           `json:"last_update_at,omitempty"`
	StallState          string               `json:"stall_state,omitempty"`
	StallReason         string               `json:"stall_reason,omitempty"`
	Activity            *SessionActivityMeta `json:"activity,omitempty"`
}

// SessionActivityMeta is the persisted prompt/runtime activity snapshot for one
// ACP-backed session.
type SessionActivityMeta struct {
	TurnID             string     `json:"turn_id,omitempty"`
	TurnSource         string     `json:"turn_source,omitempty"`
	TurnStartedAt      *time.Time `json:"turn_started_at,omitempty"`
	LastActivityAt     *time.Time `json:"last_activity_at,omitempty"`
	LastActivityKind   string     `json:"last_activity_kind,omitempty"`
	LastActivityDetail string     `json:"last_activity_detail,omitempty"`
	CurrentTool        string     `json:"current_tool,omitempty"`
	ToolCallID         string     `json:"tool_call_id,omitempty"`
	LastProgressAt     *time.Time `json:"last_progress_at,omitempty"`
	IterationCurrent   int        `json:"iteration_current,omitempty"`
	IterationMax       int        `json:"iteration_max,omitempty"`
	IdleSeconds        int64      `json:"idle_seconds,omitempty"`
}

// Validate ensures the liveness payload remains internally consistent.
func (m *SessionLivenessMeta) Validate() error {
	if m == nil {
		return nil
	}
	if m.SubprocessPID < 0 {
		return fmt.Errorf("store: session subprocess pid must be zero or positive: %d", m.SubprocessPID)
	}
	switch strings.TrimSpace(m.StallState) {
	case "", SessionStallStateDetected:
	default:
		return fmt.Errorf("store: invalid session stall state %q", m.StallState)
	}
	if strings.TrimSpace(m.StallState) != "" && strings.TrimSpace(m.StallReason) == "" {
		return fmt.Errorf("store: session stall reason required when stall state is set")
	}
	if strings.TrimSpace(m.StallReason) != "" && strings.TrimSpace(m.StallState) == "" {
		return fmt.Errorf("store: session stall reason requires a stall state")
	}
	if err := m.Activity.Validate(); err != nil {
		return err
	}
	return nil
}

// Validate ensures the activity payload remains internally consistent.
func (m *SessionActivityMeta) Validate() error {
	if m == nil {
		return nil
	}
	if m.IterationCurrent < 0 {
		return fmt.Errorf("store: session activity iteration current must be zero or positive: %d", m.IterationCurrent)
	}
	if m.IterationMax < 0 {
		return fmt.Errorf("store: session activity iteration max must be zero or positive: %d", m.IterationMax)
	}
	if m.IdleSeconds < 0 {
		return fmt.Errorf("store: session activity idle seconds must be zero or positive: %d", m.IdleSeconds)
	}
	return nil
}

// CloneSessionLivenessMeta returns a deep copy of the liveness payload.
func CloneSessionLivenessMeta(meta *SessionLivenessMeta) *SessionLivenessMeta {
	if meta == nil {
		return nil
	}

	cloned := &SessionLivenessMeta{
		SubprocessPID: meta.SubprocessPID,
		StallState:    strings.TrimSpace(meta.StallState),
		StallReason:   strings.TrimSpace(meta.StallReason),
		Activity:      CloneSessionActivityMeta(meta.Activity),
	}
	if meta.SubprocessStartedAt != nil {
		startedAt := meta.SubprocessStartedAt.UTC()
		cloned.SubprocessStartedAt = &startedAt
	}
	if meta.LastUpdateAt != nil {
		lastUpdateAt := meta.LastUpdateAt.UTC()
		cloned.LastUpdateAt = &lastUpdateAt
	}
	return cloned
}

// CloneSessionActivityMeta returns a deep copy of the runtime activity payload.
func CloneSessionActivityMeta(meta *SessionActivityMeta) *SessionActivityMeta {
	if meta == nil {
		return nil
	}

	cloned := &SessionActivityMeta{
		TurnID:             strings.TrimSpace(meta.TurnID),
		TurnSource:         strings.TrimSpace(meta.TurnSource),
		LastActivityKind:   strings.TrimSpace(meta.LastActivityKind),
		LastActivityDetail: strings.TrimSpace(meta.LastActivityDetail),
		CurrentTool:        strings.TrimSpace(meta.CurrentTool),
		ToolCallID:         strings.TrimSpace(meta.ToolCallID),
		IterationCurrent:   meta.IterationCurrent,
		IterationMax:       meta.IterationMax,
		IdleSeconds:        meta.IdleSeconds,
	}
	if meta.TurnStartedAt != nil {
		turnStartedAt := meta.TurnStartedAt.UTC()
		cloned.TurnStartedAt = &turnStartedAt
	}
	if meta.LastActivityAt != nil {
		lastActivityAt := meta.LastActivityAt.UTC()
		cloned.LastActivityAt = &lastActivityAt
	}
	if meta.LastProgressAt != nil {
		lastProgressAt := meta.LastProgressAt.UTC()
		cloned.LastProgressAt = &lastProgressAt
	}
	return cloned
}

// SessionActivityIdleSeconds reports the age of the last recorded runtime
// activity relative to now.
func SessionActivityIdleSeconds(meta *SessionActivityMeta, now time.Time) int64 {
	if meta == nil || meta.LastActivityAt == nil || meta.LastActivityAt.IsZero() || now.IsZero() {
		return 0
	}
	idle := now.UTC().Sub(meta.LastActivityAt.UTC())
	if idle < 0 {
		return 0
	}
	return int64(idle.Seconds())
}

package hooks

import (
	"strings"
	"time"
)

// TerminalContext carries the mandatory owner and correlation keys for terminal hooks.
type TerminalContext struct {
	WorkspaceID string    `json:"workspace_id"`
	ProfileID   string    `json:"profile_id"`
	TerminalID  string    `json:"terminal_id,omitempty"`
	ActorKind   string    `json:"actor_kind"`
	ActorID     string    `json:"actor_id"`
	SessionID   string    `json:"session_id,omitempty"`
	RunID       string    `json:"run_id,omitempty"`
	At          time.Time `json:"at"`
}

// HookProfileID returns the durable owner used to isolate profile-scoped declarations.
func (c TerminalContext) HookProfileID() string { return strings.TrimSpace(c.ProfileID) }

func (c TerminalContext) hookTerminalContext() TerminalContext { return c }

type TerminalExit struct {
	Cause  string  `json:"cause"`
	Code   *int    `json:"code,omitempty"`
	Signal *string `json:"signal,omitempty"`
}

type TerminalOpenedPayload struct {
	PayloadBase
	TerminalContext
	Mode  string `json:"mode"`
	Cwd   string `json:"cwd"`
	Title string `json:"title,omitempty"`
}

type TerminalClosedPayload struct {
	PayloadBase
	TerminalContext
	Exit   TerminalExit `json:"exit"`
	Reason string       `json:"reason"`
}

type TerminalLeaseChangedPayload struct {
	PayloadBase
	TerminalContext
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason"`
}

type TerminalCommandStartedPayload struct {
	PayloadBase
	TerminalContext
	CommandID  string `json:"command_id"`
	Command    string `json:"command"`
	Cwd        string `json:"cwd"`
	DetectedBy string `json:"detected_by"`
}

type TerminalCommandFinishedPayload struct {
	PayloadBase
	TerminalContext
	CommandID  string  `json:"command_id"`
	ExitCode   *int    `json:"exit_code,omitempty"`
	Signal     *string `json:"signal,omitempty"`
	ExitCause  string  `json:"exit_cause"`
	DurationMS int64   `json:"duration_ms"`
	DetectedBy string  `json:"detected_by"`
	Approval   string  `json:"approval"`
}

type TerminalInputRequestedPayload struct {
	PayloadBase
	TerminalContext
	RequestID string `json:"request_id"`
	Reason    string `json:"reason"`
	Redacted  bool   `json:"redacted"`
}

type TerminalInputProvidedPayload struct {
	PayloadBase
	TerminalContext
	RequestID string `json:"request_id"`
	Redacted  bool   `json:"redacted"`
	Length    int    `json:"length"`
	Outcome   string `json:"outcome"`
}

type TerminalRecordingStartedPayload struct {
	PayloadBase
	TerminalContext
	RecordingID string `json:"recording_id"`
}

type TerminalRecordingStoppedPayload struct {
	PayloadBase
	TerminalContext
	RecordingID string `json:"recording_id"`
	Digest      string `json:"digest"`
	Bytes       int64  `json:"bytes"`
	Reason      string `json:"reason"`
	Truncated   bool   `json:"truncated"`
}

type TerminalSubscriberEvictedPayload struct {
	PayloadBase
	TerminalContext
	Flow   string `json:"flow"`
	Reason string `json:"reason"`
}

type TerminalLimitRejectedPayload struct {
	PayloadBase
	TerminalContext
	Limit   string `json:"limit"`
	Current int    `json:"current"`
	Max     int    `json:"max"`
}

type TerminalObservationPatch struct{}

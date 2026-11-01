// Package monitor owns daemon-native session monitors: durable-for-the-daemon
// watches that wake their owning agent session when a terminal exits, a
// terminal prints a pattern, or a timer elapses. Provider-internal background
// mechanisms (Claude Code Monitor/run_in_background, Codex background
// terminals) cannot start a Compozy turn, so the daemon owns the wake.
package monitor

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// Kind selects what a monitor watches.
type Kind string

const (
	// KindTerminal watches one integrated terminal for exit or an output match.
	KindTerminal Kind = "terminal"
	// KindTimer fires once after a delay.
	KindTimer Kind = "timer"
)

// Status is the lifecycle state of one monitor.
type Status string

const (
	StatusActive Status = "active"
	// StatusDelivering means the monitor fired and its wake is retrying a transient refusal.
	StatusDelivering Status = "delivering"
	// StatusFired means the wake was admitted to the owning session's input queue.
	StatusFired    Status = "fired"
	StatusCanceled Status = "canceled"
	// StatusUndelivered means the monitor fired but its owning session could not be woken.
	StatusUndelivered Status = "undelivered"
	// StatusInterrupted means a daemon restart ended the watch and the owner could not be
	// resumed, so no wake was sent. Reason says why.
	StatusInterrupted Status = "interrupted"
)

// Terminal wait conditions accepted by terminal monitors.
const (
	UntilExit  = "exit"
	UntilMatch = "match"
)

// Outcome reasons reported to the woken agent.
const (
	ReasonExit           = "exit"
	ReasonMatch          = "match"
	ReasonTimeout        = "timeout"
	ReasonStalled        = "stalled"
	ReasonTimerElapsed   = "timer_elapsed"
	ReasonTerminalFailed = "terminal_unavailable"
	// ReasonSessionStopped marks a monitor canceled because its owner was stopped on purpose.
	ReasonSessionStopped = "session_stopped"
	// ReasonOwnerPrefix prefixes the restart-eligibility reason that kept an owner from resuming.
	ReasonOwnerPrefix = "owner_"
)

// Bounds keep monitors cheap and prevent runaway wake loops.
const (
	MaxDescriptionRunes    = 200
	MaxPatternRunes        = 512
	MaxActivePerSession    = 8
	MaxActivePerTerminal   = 4
	MaxActiveTotal         = 256
	DefaultTerminalTimeout = 30 * time.Minute
	MaxTerminalTimeout     = 6 * time.Hour
	MinTimerDelay          = 5 * time.Second
	MaxTimerDelay          = 24 * time.Hour
	// Finished monitors stay listable for inspection, bounded per session and in time.
	retainedFinishedPerSession = 16
	finishedRetention          = time.Hour
	sweepInterval              = time.Minute
	// A transient wake refusal (full input queue, admission pressure) retries with
	// backoff inside this window before the monitor is marked undelivered.
	deliveryRetryWindow  = 15 * time.Minute
	deliveryRetryInitial = 2 * time.Second
	deliveryRetryMax     = time.Minute
)

var (
	// ErrInvalid reports a monitor spec that fails validation.
	ErrInvalid = errors.New("monitor: invalid spec")
	// ErrLimit reports that the owning session already has the maximum active monitors.
	ErrLimit = errors.New("monitor: active monitor limit reached")
	// ErrNotFound reports an unknown monitor id for the session.
	ErrNotFound = errors.New("monitor: not found")
	// ErrStopped reports that the service is shutting down.
	ErrStopped = errors.New("monitor: service stopped")
	// ErrSessionGone reports a permanent wake refusal: the owning session is stopped or removed.
	ErrSessionGone = errors.New("monitor: owning session is not active")
)

// Spec describes one monitor request from an agent session.
type Spec struct {
	WorkspaceID string
	ProfileID   string
	SessionID   string
	Kind        Kind
	Description string
	TerminalID  string
	Until       string
	Pattern     string
	// Timeout bounds terminal monitors; on expiry the monitor still fires with ReasonTimeout.
	Timeout time.Duration
	// Delay is the timer monitor duration.
	Delay time.Duration
}

// Monitor is the observable state of one monitor.
type Monitor struct {
	ID          string    `json:"monitor_id"`
	SessionID   string    `json:"session_id"`
	Kind        Kind      `json:"kind"`
	Description string    `json:"description"`
	TerminalID  string    `json:"terminal_id,omitempty"`
	Until       string    `json:"until,omitempty"`
	Pattern     string    `json:"pattern,omitempty"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	DeadlineAt  time.Time `json:"deadline_at"`
	FiredAt     time.Time `json:"fired_at,omitzero"`
	Reason      string    `json:"reason,omitempty"`
	ExitCode    *int      `json:"exit_code,omitempty"`
	// Reused marks a Create that returned an already-active identical terminal
	// monitor instead of registering a duplicate wake. It is not part of the wire shape.
	Reused bool `json:"-"`
}

// Outcome is what a fired monitor observed.
type Outcome struct {
	Reason   string
	ExitCode *int
	// Output is untrusted terminal text; empty for timers.
	Output string
	// OutputLost marks a wake re-delivered after a restart: the terminal output it carried was not retained.
	OutputLost bool
}

func normalizeSpec(spec Spec) (Spec, error) {
	spec.WorkspaceID = strings.TrimSpace(spec.WorkspaceID)
	spec.ProfileID = strings.TrimSpace(spec.ProfileID)
	spec.SessionID = strings.TrimSpace(spec.SessionID)
	spec.Description = strings.TrimSpace(spec.Description)
	spec.TerminalID = strings.TrimSpace(spec.TerminalID)
	spec.Until = strings.TrimSpace(spec.Until)
	if spec.SessionID == "" || spec.WorkspaceID == "" {
		return Spec{}, invalid("session and workspace are required")
	}
	if spec.Description == "" || len([]rune(spec.Description)) > MaxDescriptionRunes {
		return Spec{}, invalid("description must be 1-200 characters")
	}
	switch spec.Kind {
	case KindTerminal:
		return normalizeTerminalSpec(spec)
	case KindTimer:
		if spec.TerminalID != "" || spec.Pattern != "" || spec.Until != "" {
			return Spec{}, invalid("timer monitors accept only delay_seconds")
		}
		if spec.Delay < MinTimerDelay || spec.Delay > MaxTimerDelay {
			return Spec{}, invalid("timer delay must be between 5 seconds and 24 hours")
		}
		spec.Timeout = 0
		return spec, nil
	default:
		return Spec{}, invalid("kind must be terminal or timer")
	}
}

func normalizeTerminalSpec(spec Spec) (Spec, error) {
	if spec.TerminalID == "" || spec.ProfileID == "" {
		return Spec{}, invalid("terminal monitors require terminal_id and a profile")
	}
	if spec.Delay != 0 {
		return Spec{}, invalid("terminal monitors do not accept delay_seconds")
	}
	if spec.Until == "" {
		spec.Until = UntilExit
	}
	switch spec.Until {
	case UntilExit:
		if spec.Pattern != "" {
			return Spec{}, invalid("pattern requires until=match")
		}
	case UntilMatch:
		if strings.TrimSpace(spec.Pattern) == "" || len([]rune(spec.Pattern)) > MaxPatternRunes {
			return Spec{}, invalid("until=match requires a pattern of at most 512 characters")
		}
		if _, err := regexp.Compile(spec.Pattern); err != nil {
			return Spec{}, invalid("pattern is not a valid regular expression")
		}
	default:
		return Spec{}, invalid("until must be exit or match")
	}
	if spec.Timeout == 0 {
		spec.Timeout = DefaultTerminalTimeout
	}
	if spec.Timeout < 0 || spec.Timeout > MaxTerminalTimeout {
		return Spec{}, invalid("terminal timeout must be at most 6 hours")
	}
	return spec, nil
}

func invalid(message string) error {
	return errors.Join(ErrInvalid, errors.New(message))
}

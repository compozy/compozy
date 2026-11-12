// Package monitor owns daemon-native session monitors: durable-for-the-daemon
// watches that wake their owning agent session when a terminal exits, a
// terminal prints a pattern, a timer elapses, other sessions reach a state, or a
// pull request changes. Only a pull_request monitor fires more than once: it stays
// armed across wakes. Provider-internal background
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
	// KindPullRequest watches a branch's pull request through the forge and fires on each meaningful change.
	KindPullRequest Kind = "pull_request"
	// KindSession watches other sessions until any or all of them reach a state.
	KindSession Kind = "session"
)

// Status is the lifecycle state of one monitor.
type Status string

const (
	StatusActive Status = "active"
	// StatusDelivering means the monitor fired and its wake is retrying a transient refusal.
	StatusDelivering Status = "delivering"
	// StatusFired means the final wake was admitted to the owning session's input queue. A pull_request
	// monitor returns to active after every other admitted wake.
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

// Session wait conditions accepted by session monitors.
const (
	UntilAny = "any"
	UntilAll = "all"
)

// Outcome reasons reported to the woken agent.
const (
	ReasonExit           = "exit"
	ReasonMatch          = "match"
	ReasonTimeout        = "timeout"
	ReasonStalled        = "stalled"
	ReasonTimerElapsed   = "timer_elapsed"
	ReasonTerminalFailed = "terminal_unavailable"
	// ReasonConditionMet marks a session monitor whose any or all condition held.
	ReasonConditionMet = "condition_met"
	// ReasonSessionStopped marks a monitor canceled because its owner was stopped on purpose.
	ReasonSessionStopped = "session_stopped"
	// ReasonCancelRequested marks a monitor its owner canceled explicitly; no wake was lost.
	ReasonCancelRequested = "cancel_requested"
	// ReasonOwnerPrefix prefixes the restart-eligibility reason that kept an owner from resuming.
	ReasonOwnerPrefix = "owner_"
	// Pull request wake reasons. A timed-out pull_request monitor reports ReasonTimeout.
	ReasonPullRequestChanged = "pull_request_changed"
	ReasonPullRequestMerged  = "pull_request_merged"
	ReasonPullRequestClosed  = "pull_request_closed"
	ReasonForgeUnavailable   = "forge_unavailable"
	ReasonFireCapReached     = "fire_cap_reached"
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
	// DefaultPullRequestTimeout and MaxPullRequestTimeout bound a pull_request watch.
	DefaultPullRequestTimeout = 6 * time.Hour
	MaxPullRequestTimeout     = 24 * time.Hour
	MinTimerDelay             = 5 * time.Second
	MaxTimerDelay             = 24 * time.Hour
	MaxSessionTargets         = 32
	DefaultSessionTimeout     = 2 * time.Hour
	MaxSessionTimeout         = 6 * time.Hour
	// Finished monitors stay listable for inspection, bounded per session and in time.
	retainedFinishedPerSession = 16
	finishedRetention          = time.Hour
	sweepInterval              = time.Minute
	// A transient wake refusal (full input queue, admission pressure) retries with
	// backoff inside this window before the monitor is marked undelivered.
	deliveryRetryWindow  = 15 * time.Minute
	deliveryRetryInitial = 2 * time.Second
	deliveryRetryMax     = time.Minute
	// Wakes pending for one session coalesce into a single queue row, outside the input capacity pools.
	// Past MaxCoalescedWakeBytes a wake keeps its outcome and drops its output tail; past
	// MaxCoalescedWakes it stays delivering and retries like any other refusal, until the row is
	// delivered or deliveryRetryWindow leaves it undelivered.
	CoalescedWakeKey      = "monitor"
	MaxCoalescedWakes     = 64
	MaxCoalescedWakeBytes = 64 << 10
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
	// Branch selects a pull_request monitor's pull request; empty means the session's branch.
	Branch string
	// Timeout bounds terminal and pull_request monitors; on expiry the monitor still fires with ReasonTimeout.
	Timeout time.Duration
	// Delay is the timer monitor duration.
	Delay time.Duration
	// Targets are the sessions a session monitor watches.
	Targets []string
	// States are the target states that satisfy a session monitor. Empty selects the default set.
	States []string
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
	Branch      string    `json:"branch,omitempty"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	DeadlineAt  time.Time `json:"deadline_at"`
	FiredAt     time.Time `json:"fired_at,omitzero"`
	Reason      string    `json:"reason,omitempty"`
	ExitCode    *int      `json:"exit_code,omitempty"`
	// FireCount and LastFiredAt track a repeating monitor's wakes; FiredAt stays the moment it ended.
	FireCount   int       `json:"fire_count,omitempty"`
	LastFiredAt time.Time `json:"last_fired_at,omitzero"`
	// TargetSessionIDs and States define a session monitor.
	TargetSessionIDs []string `json:"target_session_ids,omitempty"`
	States           []string `json:"states,omitempty"`
	// Targets is each target's state when a session monitor fired. It is stored with the monitor, so
	// a wake re-delivered after a restart reports what fired it.
	Targets []TargetStatus `json:"targets,omitempty"`
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
	// Targets is each target's state for a session monitor.
	Targets []TargetStatus
}

func normalizeSpec(spec Spec) (Spec, error) {
	spec.WorkspaceID = strings.TrimSpace(spec.WorkspaceID)
	spec.ProfileID = strings.TrimSpace(spec.ProfileID)
	spec.SessionID = strings.TrimSpace(spec.SessionID)
	spec.Description = strings.TrimSpace(spec.Description)
	spec.TerminalID = strings.TrimSpace(spec.TerminalID)
	spec.Until = strings.TrimSpace(spec.Until)
	spec.Branch = strings.TrimSpace(spec.Branch)
	if spec.Kind != KindSession && (len(spec.Targets) != 0 || len(spec.States) != 0) {
		return Spec{}, invalid("target_session_ids and states require kind session")
	}
	if spec.SessionID == "" || spec.WorkspaceID == "" {
		return Spec{}, invalid("session and workspace are required")
	}
	if spec.Description == "" || len([]rune(spec.Description)) > MaxDescriptionRunes {
		return Spec{}, invalid("description must be 1-200 characters")
	}
	switch spec.Kind {
	case KindTerminal:
		return normalizeTerminalSpec(spec)
	case KindPullRequest:
		return normalizePullRequestSpec(spec)
	case KindTimer:
		if spec.TerminalID != "" || spec.Pattern != "" || spec.Until != "" || spec.Branch != "" {
			return Spec{}, invalid("timer monitors accept only delay_seconds")
		}
		if spec.Delay < MinTimerDelay || spec.Delay > MaxTimerDelay {
			return Spec{}, invalid("timer delay must be between 5 seconds and 24 hours")
		}
		spec.Timeout = 0
		return spec, nil
	case KindSession:
		return normalizeSessionSpec(spec)
	default:
		return Spec{}, invalid("kind must be terminal, timer, session or pull_request")
	}
}

func normalizePullRequestSpec(spec Spec) (Spec, error) {
	if spec.TerminalID != "" || spec.Pattern != "" || spec.Until != "" || spec.Delay != 0 {
		return Spec{}, invalid("pull_request monitors accept only branch and timeout_seconds")
	}
	if spec.Timeout == 0 {
		spec.Timeout = DefaultPullRequestTimeout
	}
	if spec.Timeout < time.Millisecond || spec.Timeout > MaxPullRequestTimeout {
		return Spec{}, invalid("pull_request timeout must be between 1 millisecond and 24 hours")
	}
	return spec, nil
}

func normalizeTerminalSpec(spec Spec) (Spec, error) {
	if spec.TerminalID == "" || spec.ProfileID == "" {
		return Spec{}, invalid("terminal monitors require terminal_id and a profile")
	}
	if spec.Branch != "" {
		return Spec{}, invalid("terminal monitors do not accept branch")
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
	if spec.Timeout < time.Millisecond || spec.Timeout > MaxTerminalTimeout {
		return Spec{}, invalid("terminal timeout must be between 1 millisecond and 6 hours")
	}
	return spec, nil
}

func invalid(message string) error {
	return errors.Join(ErrInvalid, errors.New(message))
}

// InvalidSpec reports a refused monitor request whose static message guides the caller to correct it.
func InvalidSpec(message string) error { return invalid(message) }

// observable hides the legacy cancellation timestamp retained for durable history pruning.
func (m Monitor) observable() Monitor {
	if m.Status == StatusCanceled {
		m.FiredAt = time.Time{}
	}
	return m
}

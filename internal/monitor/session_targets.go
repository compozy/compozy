package monitor

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
)

// Target states reported for a session monitor. Failed, stopped and gone are final: a target in one
// of them can never change on its own, so it always counts toward the condition.
const (
	TargetStateIdle            = "idle"
	TargetStateDone            = "done"
	TargetStateWaitingForInput = "waiting-for-input"
	TargetStateWaitingForAuth  = "waiting-for-auth"
	TargetStateNeedsAttention  = "needs-attention"
	TargetStateHung            = "hung"
	TargetStateUnhealthy       = "unhealthy"
	TargetStateStopped         = "stopped"
	TargetStateFailed          = "failed"
	// TargetStateGone marks a target that was deleted or superseded.
	TargetStateGone = "gone"
	// TargetStateUnknown marks a target whose state could not be read; it never satisfies a condition.
	TargetStateUnknown = "unknown"
)

// targetSweepInterval re-reads targets between change notifications. Those notifications are best
// effort, so the sweep is what guarantees a monitor cannot miss a target reaching the condition.
const targetSweepInterval = 30 * time.Second

// errWatchQuiesced ends a session watch without an outcome; the monitor stays active for the next boot.
var errWatchQuiesced = errors.New("monitor: session watches are quiesced")

// TargetStatus is the state of one watched session.
type TargetStatus struct {
	SessionID string `json:"session_id"`
	Name      string `json:"name,omitempty"`
	State     string `json:"state"`
	// Matched reports that this target counts toward the monitor's condition.
	Matched bool `json:"matched"`
}

// SessionTargets reads and follows the sessions a monitor watches. The daemon implements it so this
// package never depends on the session manager.
type SessionTargets interface {
	// ReadTargets returns the current state of every target in spec. A target that cannot be read
	// is reported with TargetStateUnknown.
	ReadTargets(ctx context.Context, spec Spec) []TargetStatus
	// TargetChanges returns a best-effort signal for changes to spec's targets and a function that
	// stops it. The channel closes if the signal ends; the sweep then keeps the monitor correct.
	TargetChanges(ctx context.Context, spec Spec) (<-chan struct{}, func(), error)
}

func defaultTargetStates() []string {
	return []string{
		TargetStateFailed, TargetStateIdle, TargetStateNeedsAttention, TargetStateStopped,
		TargetStateWaitingForAuth, TargetStateWaitingForInput,
	}
}

func selectableTargetState(state string) bool {
	switch state {
	case TargetStateIdle, TargetStateWaitingForInput, TargetStateWaitingForAuth, TargetStateNeedsAttention,
		TargetStateHung, TargetStateUnhealthy, TargetStateStopped, TargetStateFailed:
		return true
	default:
		return false
	}
}

func normalizeSessionSpec(spec Spec) (Spec, error) {
	if spec.TerminalID != "" || spec.Pattern != "" || spec.Delay != 0 {
		return Spec{}, invalid("session monitors accept only target_session_ids, until, states and timeout_seconds")
	}
	spec.Targets = sortedUnique(spec.Targets)
	if len(spec.Targets) == 0 || len(spec.Targets) > MaxSessionTargets {
		return Spec{}, invalid("session monitors require 1-32 target_session_ids")
	}
	if slices.Contains(spec.Targets, spec.SessionID) {
		return Spec{}, invalid("a session monitor cannot target its own session")
	}
	if spec.Until == "" {
		spec.Until = UntilAll
	}
	if spec.Until != UntilAll && spec.Until != UntilAny {
		return Spec{}, invalid("session monitors require until to be any or all")
	}
	spec.States = sortedUnique(spec.States)
	if len(spec.States) == 0 {
		spec.States = defaultTargetStates()
	}
	if !allSelectable(spec.States) {
		return Spec{}, invalid("states must be idle, waiting-for-input, waiting-for-auth, needs-attention, " +
			"hung, unhealthy, stopped or failed")
	}
	if spec.Timeout == 0 {
		spec.Timeout = DefaultSessionTimeout
	}
	if spec.Timeout < time.Millisecond || spec.Timeout > MaxSessionTimeout {
		return Spec{}, invalid("session timeout must be between 1 millisecond and 6 hours")
	}
	return spec, nil
}

func allSelectable(states []string) bool {
	for _, state := range states {
		if !selectableTargetState(state) {
			return false
		}
	}
	return true
}

func sortedUnique(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	slices.Sort(result)
	return slices.Compact(result)
}

// sessionKey identifies a session monitor's condition for duplicate detection.
func sessionKey(spec Spec) string {
	if spec.Kind != KindSession {
		return ""
	}
	return strings.Join(spec.Targets, ",") + "\x00" + strings.Join(spec.States, ",")
}

// targetMatched reports whether status counts toward a condition over states. A session with open
// work of its own reports "waiting", which deliberately does not count as idle: it is not finished.
func targetMatched(status TargetStatus, states []string) bool {
	switch status.State {
	case TargetStateStopped, TargetStateFailed, TargetStateGone:
		return true
	case TargetStateDone:
		return slices.Contains(states, TargetStateIdle)
	default:
		return slices.Contains(states, status.State)
	}
}

// evaluateTargets marks each status and reports whether the condition holds.
func evaluateTargets(spec Spec, statuses []TargetStatus) ([]TargetStatus, bool) {
	marked := make([]TargetStatus, len(statuses))
	matched := 0
	for index, status := range statuses {
		status.Matched = targetMatched(status, spec.States)
		if status.Matched {
			matched++
		}
		marked[index] = status
	}
	if spec.Until == UntilAny {
		return marked, matched > 0
	}
	return marked, len(spec.Targets) > 0 && matched == len(spec.Targets)
}

// sessionWatch builds the blocking watch of one session monitor.
func (s *Service) sessionWatch(spec Spec) Watch {
	return func(ctx context.Context) (Outcome, error) {
		changes, stop, err := s.targets.TargetChanges(ctx, spec)
		if err != nil {
			// The sweep alone still reaches every condition, only later.
			s.logLifecycle("monitor.target_changes_unavailable", Monitor{SessionID: spec.SessionID}, "sweep_only")
			changes, stop = nil, func() {}
		}
		defer stop()
		timeout := time.NewTimer(spec.Timeout)
		defer timeout.Stop()
		sweep := time.NewTicker(targetSweepInterval)
		defer sweep.Stop()
		return s.followTargets(ctx, spec, changes, timeout.C, sweep.C)
	}
}

// followTargets subscribes before its first read, so a change between the two re-evaluates the targets.
func (s *Service) followTargets(
	ctx context.Context, spec Spec, changes <-chan struct{}, timeout, sweep <-chan time.Time,
) (Outcome, error) {
	for {
		marked, held, err := s.readTargets(ctx, spec)
		if err != nil {
			return Outcome{}, err
		}
		if held {
			return Outcome{Reason: ReasonConditionMet, Targets: marked}, nil
		}
		select {
		case <-ctx.Done():
			return Outcome{}, ctx.Err()
		case <-s.quiesced:
			return Outcome{}, errWatchQuiesced
		case <-timeout:
			return Outcome{Reason: ReasonTimeout, Targets: marked}, nil
		case <-sweep:
		case _, open := <-changes:
			if !open {
				changes = nil
			}
		}
	}
}

// readTargets discards a read that overlaps quiescing: it may have observed a target the daemon itself
// is stopping, which says nothing about that target's work.
func (s *Service) readTargets(ctx context.Context, spec Spec) ([]TargetStatus, bool, error) {
	statuses := s.targets.ReadTargets(ctx, spec)
	select {
	case <-s.quiesced:
		return nil, false, errWatchQuiesced
	default:
	}
	marked, held := evaluateTargets(spec, statuses)
	return marked, held, nil
}

// QuiesceSessionWatches stops evaluating session monitors. The daemon calls it before stopping sessions
// for a shutdown, so a target stopped by the shutdown cannot fire a monitor. Monitors stay active and
// durable, and the next boot re-evaluates them.
func (s *Service) QuiesceSessionWatches() {
	s.quiesceOnce.Do(func() { close(s.quiesced) })
}

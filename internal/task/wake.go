package task

import (
	"context"
	"fmt"
	"strings"
)

const (
	// WakeReasonTerminal wakes the creator after a run reaches a terminal state.
	WakeReasonTerminal WakeReason = "terminal"
	// WakeReasonBlocked wakes the creator after a task block is created.
	WakeReasonBlocked WakeReason = "blocked"
	// WakeReasonNeedsAttention wakes the creator after the task escalates to needs_attention.
	WakeReasonNeedsAttention WakeReason = "needs_attention"
)

// WakeReason identifies the transition class that woke a task creator.
type WakeReason string

// WakeEvent is the task-domain wake payload handed to the daemon bridge.
type WakeEvent struct {
	WakeEventID string     `json:"wake_event_id"`
	WorkspaceID string     `json:"workspace_id,omitempty"`
	TaskID      string     `json:"task_id"`
	RunID       string     `json:"run_id,omitempty"`
	Reason      WakeReason `json:"reason"`
	Summary     string     `json:"summary"`
}

// WakeNotifier delivers one wake event to a creator session.
type WakeNotifier interface {
	WakeCreator(ctx context.Context, creatorSessionID string, ev WakeEvent) error
}

type noopWakeNotifier struct{}

func defaultWakeNotifier(notifier WakeNotifier) WakeNotifier {
	if notifier != nil {
		return notifier
	}
	return noopWakeNotifier{}
}

func (noopWakeNotifier) WakeCreator(context.Context, string, WakeEvent) error {
	return nil
}

// Normalize returns the canonical wake reason value.
func (r WakeReason) Normalize() WakeReason {
	return WakeReason(strings.ToLower(strings.TrimSpace(string(r))))
}

// Validate reports whether the wake reason is supported.
func (r WakeReason) Validate(path string) error {
	switch r.Normalize() {
	case WakeReasonTerminal, WakeReasonBlocked, WakeReasonNeedsAttention:
		return nil
	default:
		return fmt.Errorf("%w: invalid %s: %q", ErrValidation, path, r)
	}
}

// Normalize returns a trimmed wake event with bounded redacted summary text.
func (e WakeEvent) Normalize() WakeEvent {
	return WakeEvent{
		WakeEventID: strings.TrimSpace(e.WakeEventID),
		WorkspaceID: strings.TrimSpace(e.WorkspaceID),
		TaskID:      strings.TrimSpace(e.TaskID),
		RunID:       strings.TrimSpace(e.RunID),
		Reason:      e.Reason.Normalize(),
		Summary:     boundWakeSummary(e.Summary),
	}
}

// Validate reports whether the wake event has the minimum delivery identity.
func (e WakeEvent) Validate() error {
	normalized := e.Normalize()
	if normalized.WakeEventID == "" {
		return fmt.Errorf("%w: wake_event_id is required", ErrValidation)
	}
	if normalized.TaskID == "" {
		return fmt.Errorf("%w: wake_event.task_id is required", ErrValidation)
	}
	if err := normalized.Reason.Validate("wake_event.reason"); err != nil {
		return err
	}
	if normalized.Summary == "" {
		return fmt.Errorf("%w: wake_event.summary is required", ErrValidation)
	}
	return nil
}

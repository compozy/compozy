package task

import (
	"fmt"
	"slices"
)

// TerminalRunHistoryImport is the explicit bootstrap-only boundary for one
// already-finished historical run. Runtime execution must create queued work
// through the execution transaction instead.
type TerminalRunHistoryImport struct {
	run   Run
	actor ActorContext
}

// TerminalRunHistoryStatuses lists the run statuses a history import accepts.
func TerminalRunHistoryStatuses() []RunStatus {
	return []RunStatus{TaskRunStatusCompleted, TaskRunStatusFailed, TaskRunStatusCanceled}
}

// StatusForTerminalRun reports the task projection a terminal run requires.
func StatusForTerminalRun(status RunStatus) (Status, bool) {
	switch status.Normalize() {
	case TaskRunStatusCompleted:
		return TaskStatusCompleted, true
	case TaskRunStatusFailed:
		return TaskStatusFailed, true
	case TaskRunStatusCanceled:
		return TaskStatusCanceled, true
	default:
		return "", false
	}
}

// NewTerminalRunHistoryImport validates and isolates one finished history
// snapshot before it reaches persistence.
func NewTerminalRunHistoryImport(run Run, actor ActorContext) (TerminalRunHistoryImport, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return TerminalRunHistoryImport{}, err
	}
	if !slices.Contains(TerminalRunHistoryStatuses(), run.Status.Normalize()) {
		return TerminalRunHistoryImport{}, fmt.Errorf(
			"%w: history import requires a terminal run status, got %q",
			ErrInvalidStatusTransition,
			run.Status,
		)
	}
	if !run.IsTaskAnchored() {
		return TerminalRunHistoryImport{}, fmt.Errorf(
			"%w: history import requires a task-anchored run",
			ErrValidation,
		)
	}
	if run.EndedAt.IsZero() {
		return TerminalRunHistoryImport{}, fmt.Errorf("%w: history import requires ended_at", ErrValidation)
	}
	if run.QueuedAt.IsZero() {
		return TerminalRunHistoryImport{}, fmt.Errorf("%w: history import requires queued_at", ErrValidation)
	}
	if (!run.StartedAt.IsZero() && run.StartedAt.Before(run.QueuedAt)) || run.EndedAt.Before(run.QueuedAt) ||
		(!run.StartedAt.IsZero() && run.EndedAt.Before(run.StartedAt)) {
		return TerminalRunHistoryImport{}, fmt.Errorf("%w: history import timestamps are out of order", ErrValidation)
	}
	if err := run.Validate(); err != nil {
		return TerminalRunHistoryImport{}, err
	}
	return TerminalRunHistoryImport{run: cloneImportedRun(run), actor: actor}, nil
}

// Run reports an isolated copy of the finished historical snapshot.
func (c *TerminalRunHistoryImport) Run() Run { return cloneImportedRun(c.run) }

// Actor reports the authority responsible for the import.
func (c *TerminalRunHistoryImport) Actor() ActorContext { return c.actor }

func cloneImportedRun(run Run) Run {
	cloned := run
	if run.ClaimedBy != nil {
		claimedBy := *run.ClaimedBy
		cloned.ClaimedBy = &claimedBy
	}
	if run.RunNetworkState != nil {
		networkState := *run.RunNetworkState
		cloned.RunNetworkState = &networkState
	}
	if run.RunWorktreeState != nil {
		worktreeState := *run.RunWorktreeState
		cloned.RunWorktreeState = &worktreeState
	}
	cloned.RequiredCapabilities = slices.Clone(run.RequiredCapabilities)
	cloned.PreferredCapabilities = slices.Clone(run.PreferredCapabilities)
	if run.Review != nil {
		review := *run.Review
		review.MissingWork = cloneRawJSON(run.Review.MissingWork)
		cloned.Review = &review
	}
	cloned.Metadata = cloneRawJSON(run.Metadata)
	cloned.Result = cloneRawJSON(run.Result)
	return cloned
}

package task

import (
	"fmt"

	"slices"
)

// CompletedRunHistoryImport is the explicit bootstrap-only boundary for one
// already-completed historical run. Runtime execution must create queued work
// through the execution transaction instead.
type CompletedRunHistoryImport struct {
	run   Run
	actor ActorContext
}

// NewCompletedRunHistoryImport validates and isolates one completed history
// snapshot before it reaches persistence.
func NewCompletedRunHistoryImport(run Run, actor ActorContext) (CompletedRunHistoryImport, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return CompletedRunHistoryImport{}, err
	}
	if run.Status.Normalize() != TaskRunStatusCompleted {
		return CompletedRunHistoryImport{}, fmt.Errorf(
			"%w: completed history import requires completed status",
			ErrInvalidStatusTransition,
		)
	}
	if !run.IsTaskAnchored() {
		return CompletedRunHistoryImport{}, fmt.Errorf(
			"%w: completed history import requires a task-anchored run",
			ErrValidation,
		)
	}
	if err := run.Validate(); err != nil {
		return CompletedRunHistoryImport{}, err
	}
	return CompletedRunHistoryImport{run: cloneImportedRun(run), actor: actor}, nil
}

// Run reports an isolated copy of the completed historical snapshot.
func (c *CompletedRunHistoryImport) Run() Run { return cloneImportedRun(c.run) }

// Actor reports the authority responsible for the import.
func (c *CompletedRunHistoryImport) Actor() ActorContext { return c.actor }

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

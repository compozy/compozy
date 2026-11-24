package loop

import (
	"context"
	"time"
)

type RunListAttention struct {
	Kind  string
	Count int
	Since time.Time
}

type RunListSummary struct {
	RunID     RunID
	Attention *RunListAttention
	Progress  StepProgress
	Forks     []ForkRef
}

type RunListSummaryReader interface {
	ListLoopRunSummaries(context.Context, WorkspaceID, []RunID) (map[RunID]RunListSummary, error)
}

// RunTerminalCause is the durable cause recorded by the settlement status event.
type RunTerminalCause struct {
	Status     Status
	Cause      TransitionCause
	ReasonCode string
	GateID     NodeID
	NodeID     NodeID
	Message    string
	At         time.Time
}

// RunTerminalCauseReader reads settled terminal causes for run list and status projections.
type RunTerminalCauseReader interface {
	ListLoopRunTerminalCauses(context.Context, WorkspaceID, []RunID) (map[RunID]RunTerminalCause, error)
}

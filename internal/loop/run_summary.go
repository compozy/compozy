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

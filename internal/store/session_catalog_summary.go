package store

import "context"

// SessionStatusSummary contains exact counts across the selected durable population.
type SessionStatusSummary struct {
	Total    int
	Active   int
	ByStatus map[string]int
	ByBadge  map[string]int
}

// SessionStatusRowsReader reads the ownership/attention fields needed for metadata counts.
type SessionStatusRowsReader interface {
	SessionStatusRows(context.Context, ReadScope) ([]SessionInfo, error)
}

// SessionFailureSummary contains exact visible failure counts and at most ten recent failures.
type SessionFailureSummary struct {
	Total  int
	ByKind map[FailureKind]int
	Recent []SessionInfo
}

type SessionFailureSummaryReader interface {
	SessionFailureSummary(context.Context, ReadScope) (SessionFailureSummary, error)
}

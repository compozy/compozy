package store

import "context"

// EventSummaryStore manages persisted observability event summaries.
type EventSummaryStore interface {
	WriteEventSummary(ctx context.Context, summary EventSummary) error
	ListEventSummaries(ctx context.Context, query EventSummaryQuery) ([]EventSummary, error)
}

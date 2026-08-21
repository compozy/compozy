package loop

import "context"

// RunReadService exposes computed, workspace-scoped run projections.
type RunReadService interface {
	NodeRoster(context.Context, string, RunID, RosterQuery) (RosterPage, error)
	Briefing(context.Context, string, RunID) (Briefing, error)
	Timeline(context.Context, string, RunID, TimelineQuery) (TimelinePage, error)
}

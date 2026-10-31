package store

import "context"

// SessionCatalogFacets is exact metadata across an eligible catalog population.
// Search and selected badge are excluded so the chip counts remain scope counts.
type SessionCatalogFacets struct {
	Board             SessionBoardCounts
	All               int
	NeedsYou          int
	Working           int
	Finished          int
	TerminalApprovals int
	Idle              int
}

// WorkspaceSessionCatalogFacets supplies each registered group's exact counts.
type WorkspaceSessionCatalogFacets struct {
	WorkspaceID string
	Facets      SessionCatalogFacets
}

type ProfileSessionCatalogFacets struct {
	ProfileID string
	Facets    SessionCatalogFacets
}

// SessionCatalogFacetResult aggregates durable metadata without hydrating rows.
type SessionCatalogFacetResult struct {
	TerminalApprovals []TerminalApprovalCount
	Facets            SessionCatalogFacets
	ByWorkspace       []WorkspaceSessionCatalogFacets
	ByProfile         []ProfileSessionCatalogFacets
	// Later counts the scope's sessions parked in Later, whatever the read's
	// Later filter, so one facets read serves both the list and its Later toggle.
	Later int
}

// TerminalApprovalCount retains the exact workspace/profile intersection for shell badges.
type TerminalApprovalCount struct {
	WorkspaceID string
	ProfileID   string
	Count       int
}

// SessionCatalogFacetReader is the metadata-only catalog count boundary.
type SessionCatalogFacetReader interface {
	SessionCatalogFacets(context.Context, SessionCatalogPageQuery) (SessionCatalogFacetResult, error)
}

// SessionBoardCounts counts sessions in each derived board column.
type SessionBoardCounts struct {
	NeedsYou int
	Working  int
	Waiting  int
	InReview int
	Done     int
}

// Total counts the sessions across every column.
func (counts SessionBoardCounts) Total() int {
	return counts.NeedsYou + counts.Working + counts.Waiting + counts.InReview + counts.Done
}

func (counts *SessionBoardCounts) Add(column string, count int) {
	switch column {
	case SessionBoardNeedsYou:
		counts.NeedsYou += count
	case SessionBoardWorking:
		counts.Working += count
	case SessionBoardWaiting:
		counts.Waiting += count
	case SessionBoardInReview:
		counts.InReview += count
	case SessionBoardDone:
		counts.Done += count
	}
}

const (
	SessionBoardNeedsYou = "needs_you"
	SessionBoardWorking  = "working"
	SessionBoardWaiting  = "waiting"
	SessionBoardInReview = "in_review"
	SessionBoardDone     = "done"
)

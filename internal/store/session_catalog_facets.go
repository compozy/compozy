package store

import "context"

// SessionCatalogFacets is exact metadata across an eligible catalog population.
// Search and selected badge are excluded so the chip counts remain scope counts.
type SessionCatalogFacets struct {
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

// SessionCatalogFacetResult aggregates durable metadata without hydrating rows.
type SessionCatalogFacetResult struct {
	Facets      SessionCatalogFacets
	ByWorkspace []WorkspaceSessionCatalogFacets
}

// SessionCatalogFacetReader is the metadata-only catalog count boundary.
type SessionCatalogFacetReader interface {
	SessionCatalogFacets(context.Context, SessionCatalogPageQuery) (SessionCatalogFacetResult, error)
}

package store

import "context"

// SessionCatalogRelationsQuery scopes page relationship metadata independently of row filters.
type SessionCatalogRelationsQuery struct {
	DefaultBranches map[string][]string
	WorkspaceID     string
	WorkspaceIDs    []string
	ReadScope       ReadScope
	SessionIDs      []string
	ParentIDs       []string
	ActiveIDs       []string
	// ContinuedSourceIDs are the sources of the page's continued sessions; each resolves
	// to the parent its continuation chain started under (ContinuedTreeParents).
	ContinuedSourceIDs []string
	RepositoryKeys     map[string]string
}

type SessionCatalogRelations struct {
	Children     map[string]SessionBoardCounts
	ParentTitles map[string]string
	// ContinuedTreeParents maps a continued session's source to the hierarchy parent of
	// the first session in its continue chain; a chain that began top-level is absent.
	ContinuedTreeParents map[string]string
}

type SessionCatalogRelationReader interface {
	SessionCatalogRelations(context.Context, SessionCatalogRelationsQuery) (SessionCatalogRelations, error)
}

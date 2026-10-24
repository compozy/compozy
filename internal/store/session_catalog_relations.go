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
	RepositoryKeys  map[string]string
}

type SessionCatalogRelations struct {
	Children     map[string]SessionBoardCounts
	ParentTitles map[string]string
}

type SessionCatalogRelationReader interface {
	SessionCatalogRelations(context.Context, SessionCatalogRelationsQuery) (SessionCatalogRelations, error)
}

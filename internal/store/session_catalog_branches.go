package store

import "context"

// SessionBranchKey identifies one branch without hydrating its session history.
type SessionBranchKey struct{ WorkspaceID, Branch string }

type SessionCatalogBranchReader interface {
	SessionCatalogBranches(context.Context, SessionCatalogPageQuery) ([]SessionBranchKey, error)
}

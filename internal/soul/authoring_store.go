package soul

import "context"

// AuthoringStore is the persistence boundary used by managed authoring.
type AuthoringStore interface {
	UpsertSoulSnapshot(ctx context.Context, snapshot Snapshot) (Snapshot, error)
	AppendSoulRevision(ctx context.Context, revision Revision) (Revision, error)
	ListSoulRevisions(ctx context.Context, query RevisionListQuery) ([]Revision, error)
	FindSoulRevisionForRollback(ctx context.Context, query RollbackLookup) (Revision, error)
	DeleteSoulAgentHistory(ctx context.Context, workspaceID string, agentName string, sourcePath string) error
}

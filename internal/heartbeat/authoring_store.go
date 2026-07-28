package heartbeat

import "context"

// AuthoringStore is the persistence boundary used by managed Heartbeat authoring.
type AuthoringStore interface {
	UpsertHeartbeatSnapshot(ctx context.Context, snapshot Snapshot) (Snapshot, error)
	AppendHeartbeatRevision(ctx context.Context, revision Revision) (Revision, error)
	ListHeartbeatRevisions(ctx context.Context, query RevisionListQuery) ([]Revision, error)
	FindHeartbeatRevisionForRollback(ctx context.Context, query RollbackLookup) (Revision, error)
	FindHeartbeatSnapshotByDigest(
		ctx context.Context,
		workspaceID string,
		agentName string,
		digest string,
	) (Snapshot, bool, error)
	DeleteHeartbeatAgentHistory(ctx context.Context, workspaceID string, agentName string, sourcePath string) error
}

package globaldb

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func soulSnapshotFromGenerated(row sqlcgen.AgentSoulSnapshot) (soul.Snapshot, error) {
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return soul.Snapshot{}, fmt.Errorf("store: parse soul snapshot created_at: %w", err)
	}
	snapshot := soul.Snapshot{
		ID: row.ID, WorkspaceID: row.WorkspaceID, AgentName: row.AgentName,
		SourcePath: row.SourcePath, Digest: row.Digest, ProfileJSON: json.RawMessage(row.ProfileJson),
		Body: row.Body, Truncated: row.Truncated != 0, CreatedAt: createdAt,
	}
	if err := snapshot.Validate(); err != nil {
		return soul.Snapshot{}, err
	}
	return snapshot.Normalize(), nil
}

func soulRevisionFromGenerated(row sqlcgen.AgentSoulRevision) (soul.Revision, error) {
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return soul.Revision{}, fmt.Errorf("store: parse soul revision created_at: %w", err)
	}
	revision := soul.Revision{
		ID: row.ID, WorkspaceID: row.WorkspaceID, AgentName: row.AgentName,
		SourcePath: row.SourcePath, Action: soul.RevisionAction(row.Action),
		PreviousDigest: row.PreviousDigest, NewDigest: row.NewDigest, Body: row.Body,
		DiagnosticsJSON: json.RawMessage(row.DiagnosticsJson), ActorKind: row.ActorKind,
		ActorID: row.ActorID, OriginKind: row.OriginKind, OriginRef: row.OriginRef,
		CreatedAt: createdAt,
	}
	if err := revision.Validate(); err != nil {
		return soul.Revision{}, err
	}
	return revision.Normalize(), nil
}

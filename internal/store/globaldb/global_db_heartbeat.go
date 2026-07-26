package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

const heartbeatOrderByCreatedDesc = " ORDER BY created_at DESC, id DESC"

// UpsertHeartbeatSnapshot inserts a resolved Heartbeat snapshot or reuses the existing row for its digest.
func (g *HeartbeatRepo) UpsertHeartbeatSnapshot(
	ctx context.Context,
	snapshot heartbeat.Snapshot,
) (heartbeat.Snapshot, error) {
	if err := g.checkReady(ctx, "upsert heartbeat snapshot"); err != nil {
		return heartbeat.Snapshot{}, err
	}

	normalized := snapshot.Normalize()
	if normalized.ID == "" {
		normalized.ID = store.NewID("hb")
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if err := normalized.Validate(); err != nil {
		return heartbeat.Snapshot{}, err
	}

	existing, ok, err := g.FindHeartbeatSnapshotByDigest(
		ctx,
		normalized.WorkspaceID,
		normalized.AgentName,
		normalized.Digest,
	)
	if err != nil {
		return heartbeat.Snapshot{}, err
	}
	if ok {
		return existing, nil
	}

	affected, err := g.queries.InsertHeartbeatSnapshot(ctx, sqlcgen.InsertHeartbeatSnapshotParams{
		ID: normalized.ID, WorkspaceID: normalized.WorkspaceID, AgentName: normalized.AgentName,
		SourcePath: normalized.SourcePath, SchemaVersion: int64(normalized.SchemaVersion), Digest: normalized.Digest,
		ConfigDigest: normalized.ConfigDigest, Body: normalized.Body,
		FrontmatterJson: string(normalized.FrontmatterJSON), ResolvedJson: string(normalized.ResolvedJSON),
		DiagnosticsJson: string(normalized.DiagnosticsJSON), CreatedAt: store.FormatTimestamp(normalized.CreatedAt),
	})
	if err != nil {
		return heartbeat.Snapshot{}, fmt.Errorf("store: insert heartbeat snapshot %q: %w", normalized.ID, err)
	}
	if affected == 0 {
		existing, ok, err := g.FindHeartbeatSnapshotByDigest(
			ctx,
			normalized.WorkspaceID,
			normalized.AgentName,
			normalized.Digest,
		)
		if err != nil {
			return heartbeat.Snapshot{}, err
		}
		if !ok {
			return heartbeat.Snapshot{}, fmt.Errorf(
				"store: reload heartbeat snapshot digest %q: %w",
				normalized.Digest,
				heartbeat.ErrSnapshotNotFound,
			)
		}
		return existing, nil
	}
	return normalized, nil
}

// GetHeartbeatSnapshot returns a persisted Heartbeat snapshot by id.
func (g *HeartbeatRepo) GetHeartbeatSnapshot(ctx context.Context, id string) (heartbeat.Snapshot, error) {
	if err := g.checkReady(ctx, "get heartbeat snapshot"); err != nil {
		return heartbeat.Snapshot{}, err
	}
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return heartbeat.Snapshot{}, fmt.Errorf("%w: id is required", heartbeat.ErrInvalidSnapshot)
	}

	row, err := g.queries.GetHeartbeatSnapshot(ctx, trimmedID)
	if errors.Is(err, sql.ErrNoRows) {
		return heartbeat.Snapshot{}, fmt.Errorf(
			"store: heartbeat snapshot %q: %w",
			trimmedID,
			heartbeat.ErrSnapshotNotFound,
		)
	}
	if err != nil {
		return heartbeat.Snapshot{}, err
	}
	return heartbeatSnapshotFromGenerated(row)
}

// FindHeartbeatSnapshotByDigest returns the snapshot matching an agent digest.
func (g *HeartbeatRepo) FindHeartbeatSnapshotByDigest(
	ctx context.Context,
	workspaceID string,
	agentName string,
	digest string,
) (heartbeat.Snapshot, bool, error) {
	if err := g.checkReady(ctx, "find heartbeat snapshot by digest"); err != nil {
		return heartbeat.Snapshot{}, false, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	agentName = strings.TrimSpace(agentName)
	digest = strings.TrimSpace(digest)
	if workspaceID == "" || agentName == "" || digest == "" {
		return heartbeat.Snapshot{}, false, fmt.Errorf(
			"%w: workspace id, agent name, and digest are required",
			heartbeat.ErrInvalidSnapshot,
		)
	}

	row, err := g.queries.GetHeartbeatSnapshotByDigest(ctx, sqlcgen.GetHeartbeatSnapshotByDigestParams{
		WorkspaceID: workspaceID, AgentName: agentName, Digest: digest,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return heartbeat.Snapshot{}, false, nil
	}
	if err != nil {
		return heartbeat.Snapshot{}, false, err
	}
	snapshot, err := heartbeatSnapshotFromGenerated(row)
	return snapshot, err == nil, err
}

// GetLatestValidHeartbeatSnapshot returns the newest persisted valid Heartbeat policy for an agent.
func (g *HeartbeatRepo) GetLatestValidHeartbeatSnapshot(
	ctx context.Context,
	workspaceID string,
	agentName string,
) (heartbeat.Snapshot, error) {
	snapshots, err := g.ListHeartbeatSnapshots(ctx, heartbeat.SnapshotListQuery{
		WorkspaceID: workspaceID,
		AgentName:   agentName,
	})
	if err != nil {
		return heartbeat.Snapshot{}, err
	}
	for _, snapshot := range snapshots {
		envelope, err := snapshot.ResolvedEnvelope()
		if err != nil {
			return heartbeat.Snapshot{}, err
		}
		if envelope.Valid {
			return snapshot, nil
		}
	}
	return heartbeat.Snapshot{}, fmt.Errorf(
		"store: latest valid heartbeat snapshot for %q/%q: %w",
		strings.TrimSpace(workspaceID),
		strings.TrimSpace(agentName),
		heartbeat.ErrSnapshotNotFound,
	)
}

// ListHeartbeatSnapshots lists persisted Heartbeat snapshots in newest-first order.
func (g *HeartbeatRepo) ListHeartbeatSnapshots(
	ctx context.Context,
	query heartbeat.SnapshotListQuery,
) (snapshots []heartbeat.Snapshot, err error) {
	if err := g.checkReady(ctx, "list heartbeat snapshots"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	// dynamic-sql: optional workspace/agent/digest filters and the caller limit change the statement shape.
	sqlQuery := `SELECT id, workspace_id, agent_name, source_path, schema_version, digest, config_digest,
			body, frontmatter_json, resolved_json, diagnostics_json, created_at
		FROM agent_heartbeat_snapshots`
	where, args := store.BuildClauses(
		store.StringClause("workspace_id", query.WorkspaceID),
		store.StringClause("agent_name", query.AgentName),
		store.StringClause("digest", query.Digest),
	)
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += heartbeatOrderByCreatedDesc
	sqlQuery, args = store.AppendLimit(sqlQuery, args, query.Limit)

	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query heartbeat snapshots: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close heartbeat snapshot rows: %w", closeErr))
		}
	}()

	snapshots = make([]heartbeat.Snapshot, 0)
	for rows.Next() {
		snapshot, scanErr := scanHeartbeatSnapshot(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		snapshots = append(snapshots, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate heartbeat snapshots: %w", err)
	}
	return snapshots, nil
}

// AppendHeartbeatRevision appends one managed Heartbeat authoring revision row.
func (g *HeartbeatRepo) AppendHeartbeatRevision(
	ctx context.Context,
	revision heartbeat.Revision,
) (heartbeat.Revision, error) {
	if err := g.checkReady(ctx, "append heartbeat revision"); err != nil {
		return heartbeat.Revision{}, err
	}

	normalized := revision.Normalize()
	if normalized.ID == "" {
		normalized.ID = store.NewID("hrev")
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if err := normalized.Validate(); err != nil {
		return heartbeat.Revision{}, err
	}

	err := g.queries.InsertHeartbeatRevision(ctx, sqlcgen.InsertHeartbeatRevisionParams{
		ID:          normalized.ID,
		WorkspaceID: normalized.WorkspaceID,
		AgentName:   normalized.AgentName,
		SourcePath:  normalized.SourcePath,
		Operation:   string(normalized.Operation),
		PreviousDigest: nullableHeartbeatString(
			normalized.PreviousDigest,
		),
		NewDigest: nullableHeartbeatString(normalized.NewDigest),
		NewSnapshotID: nullableHeartbeatString(
			normalized.NewSnapshotID,
		),
		Body:      nullableHeartbeatText(normalized.Body),
		ActorKind: string(normalized.ActorKind),
		ActorID:   normalized.ActorID,
		CreatedAt: store.FormatTimestamp(normalized.CreatedAt),
	})
	if err != nil {
		return heartbeat.Revision{}, fmt.Errorf("store: append heartbeat revision %q: %w", normalized.ID, err)
	}
	return normalized, nil
}

// GetHeartbeatRevision returns a managed Heartbeat authoring revision by id.
func (g *HeartbeatRepo) GetHeartbeatRevision(ctx context.Context, id string) (heartbeat.Revision, error) {
	if err := g.checkReady(ctx, "get heartbeat revision"); err != nil {
		return heartbeat.Revision{}, err
	}
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return heartbeat.Revision{}, fmt.Errorf("%w: id is required", heartbeat.ErrInvalidRevision)
	}

	row, err := g.queries.GetHeartbeatRevision(ctx, trimmedID)
	if errors.Is(err, sql.ErrNoRows) {
		return heartbeat.Revision{}, fmt.Errorf(
			"store: heartbeat revision %q: %w",
			trimmedID,
			heartbeat.ErrRevisionNotFound,
		)
	}
	if err != nil {
		return heartbeat.Revision{}, err
	}
	return heartbeatRevisionFromGenerated(row)
}

// ListHeartbeatRevisions lists managed Heartbeat authoring revisions in newest-first order.
func (g *HeartbeatRepo) ListHeartbeatRevisions(
	ctx context.Context,
	query heartbeat.RevisionListQuery,
) (revisions []heartbeat.Revision, err error) {
	if err := g.checkReady(ctx, "list heartbeat revisions"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	// dynamic-sql: optional workspace/agent/operation filters and the caller limit change the statement shape.
	sqlQuery := `SELECT id, workspace_id, agent_name, source_path, operation, previous_digest, new_digest,
			new_snapshot_id, body, actor_kind, actor_id, created_at
		FROM agent_heartbeat_revisions`
	where, args := store.BuildClauses(
		store.StringClause("workspace_id", query.WorkspaceID),
		store.StringClause("agent_name", query.AgentName),
		store.StringClause("operation", string(query.Operation)),
	)
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += heartbeatOrderByCreatedDesc
	sqlQuery, args = store.AppendLimit(sqlQuery, args, query.Limit)

	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query heartbeat revisions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close heartbeat revision rows: %w", closeErr))
		}
	}()

	revisions = make([]heartbeat.Revision, 0)
	for rows.Next() {
		revision, scanErr := scanHeartbeatRevision(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		revisions = append(revisions, revision)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate heartbeat revisions: %w", err)
	}
	return revisions, nil
}

// FindHeartbeatRevisionForRollback returns the revision body selected for a managed rollback.
func (g *HeartbeatRepo) FindHeartbeatRevisionForRollback(
	ctx context.Context,
	query heartbeat.RollbackLookup,
) (heartbeat.Revision, error) {
	if err := g.checkReady(ctx, "find heartbeat rollback revision"); err != nil {
		return heartbeat.Revision{}, err
	}
	if err := query.Validate(); err != nil {
		return heartbeat.Revision{}, err
	}

	row, err := g.queries.GetHeartbeatRevisionForRollback(ctx, sqlcgen.GetHeartbeatRevisionForRollbackParams{
		WorkspaceID: strings.TrimSpace(query.WorkspaceID), AgentName: strings.TrimSpace(query.AgentName),
		ID: strings.TrimSpace(query.RevisionID),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return heartbeat.Revision{}, fmt.Errorf(
			"store: heartbeat rollback revision %q: %w",
			query.RevisionID,
			heartbeat.ErrRevisionNotFound,
		)
	}
	if err != nil {
		return heartbeat.Revision{}, err
	}
	return heartbeatRevisionFromGenerated(row)
}

// UpsertSessionHealth stores the latest metadata-only health row for one session.

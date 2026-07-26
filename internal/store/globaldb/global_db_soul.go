package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

// UpsertSoulSnapshot inserts a resolved Soul snapshot or reuses the existing row for its digest.
func (g *SoulRepo) UpsertSoulSnapshot(ctx context.Context, snapshot soul.Snapshot) (soul.Snapshot, error) {
	if err := g.checkReady(ctx, "upsert soul snapshot"); err != nil {
		return soul.Snapshot{}, err
	}

	normalized := snapshot.Normalize()
	if normalized.ID == "" {
		normalized.ID = store.NewID("soul")
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if err := normalized.Validate(); err != nil {
		return soul.Snapshot{}, err
	}

	existing, ok, err := g.FindSoulSnapshotByDigest(
		ctx,
		normalized.WorkspaceID,
		normalized.AgentName,
		normalized.Digest,
	)
	if err != nil {
		return soul.Snapshot{}, err
	}
	if ok {
		return existing, nil
	}

	affected, err := g.queries.InsertSoulSnapshot(ctx, sqlcgen.InsertSoulSnapshotParams{
		ID: normalized.ID, WorkspaceID: normalized.WorkspaceID, AgentName: normalized.AgentName,
		SourcePath: normalized.SourcePath, Digest: normalized.Digest, ProfileJson: string(normalized.ProfileJSON),
		Body: normalized.Body, Truncated: int64(soulBoolToInt(normalized.Truncated)),
		CreatedAt: store.FormatTimestamp(normalized.CreatedAt),
	})
	if err != nil {
		return soul.Snapshot{}, fmt.Errorf("store: insert soul snapshot %q: %w", normalized.ID, err)
	}
	if affected == 0 {
		existing, ok, err := g.FindSoulSnapshotByDigest(
			ctx,
			normalized.WorkspaceID,
			normalized.AgentName,
			normalized.Digest,
		)
		if err != nil {
			return soul.Snapshot{}, err
		}
		if !ok {
			return soul.Snapshot{}, fmt.Errorf(
				"store: reload soul snapshot digest %q: %w",
				normalized.Digest,
				soul.ErrSnapshotNotFound,
			)
		}
		return existing, nil
	}
	return normalized, nil
}

// GetSoulSnapshot returns a persisted Soul snapshot by id.
func (g *SoulRepo) GetSoulSnapshot(ctx context.Context, id string) (soul.Snapshot, error) {
	if err := g.checkReady(ctx, "get soul snapshot"); err != nil {
		return soul.Snapshot{}, err
	}
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return soul.Snapshot{}, fmt.Errorf("%w: id is required", soul.ErrInvalidSnapshot)
	}

	row, err := g.queries.GetSoulSnapshot(ctx, trimmedID)
	if errors.Is(err, sql.ErrNoRows) {
		return soul.Snapshot{}, fmt.Errorf("store: soul snapshot %q: %w", trimmedID, soul.ErrSnapshotNotFound)
	}
	if err != nil {
		return soul.Snapshot{}, err
	}
	return soulSnapshotFromGenerated(row)
}

// FindSoulSnapshotByDigest returns the snapshot matching an agent digest.
func (g *SoulRepo) FindSoulSnapshotByDigest(
	ctx context.Context,
	workspaceID string,
	agentName string,
	digest string,
) (soul.Snapshot, bool, error) {
	if err := g.checkReady(ctx, "find soul snapshot by digest"); err != nil {
		return soul.Snapshot{}, false, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	agentName = strings.TrimSpace(agentName)
	digest = strings.TrimSpace(digest)
	if workspaceID == "" || agentName == "" || digest == "" {
		return soul.Snapshot{}, false, fmt.Errorf(
			"%w: workspace id, agent name, and digest are required",
			soul.ErrInvalidSnapshot,
		)
	}

	row, err := g.queries.GetSoulSnapshotByDigest(ctx, sqlcgen.GetSoulSnapshotByDigestParams{
		WorkspaceID: workspaceID, AgentName: agentName, Digest: digest,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return soul.Snapshot{}, false, nil
	}
	if err != nil {
		return soul.Snapshot{}, false, err
	}
	snapshot, err := soulSnapshotFromGenerated(row)
	return snapshot, err == nil, err
}

// ListSoulSnapshots lists persisted Soul snapshots in newest-first order.
func (g *SoulRepo) ListSoulSnapshots(
	ctx context.Context,
	query soul.SnapshotListQuery,
) (snapshots []soul.Snapshot, err error) {
	if err := g.checkReady(ctx, "list soul snapshots"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	// dynamic-sql: optional workspace/agent/digest filters and the caller limit change the statement shape.
	sqlQuery := `SELECT id, workspace_id, agent_name, source_path, digest, profile_json, body, truncated, created_at
		FROM agent_soul_snapshots`
	where, args := store.BuildClauses(
		store.StringClause("workspace_id", query.WorkspaceID),
		store.StringClause("agent_name", query.AgentName),
		store.StringClause("digest", query.Digest),
	)
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += " ORDER BY created_at DESC, id DESC"
	sqlQuery, args = store.AppendLimit(sqlQuery, args, query.Limit)

	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query soul snapshots: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close soul snapshot rows: %w", closeErr))
		}
	}()

	snapshots = make([]soul.Snapshot, 0)
	for rows.Next() {
		snapshot, scanErr := scanSoulSnapshot(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		snapshots = append(snapshots, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate soul snapshots: %w", err)
	}
	return snapshots, nil
}

// AppendSoulRevision appends one managed authoring revision row.
func (g *SoulRepo) AppendSoulRevision(ctx context.Context, revision soul.Revision) (soul.Revision, error) {
	if err := g.checkReady(ctx, "append soul revision"); err != nil {
		return soul.Revision{}, err
	}

	normalized := revision.Normalize()
	if normalized.ID == "" {
		normalized.ID = store.NewID("srev")
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if err := normalized.Validate(); err != nil {
		return soul.Revision{}, err
	}

	err := g.queries.InsertSoulRevision(ctx, sqlcgen.InsertSoulRevisionParams{
		ID: normalized.ID, WorkspaceID: normalized.WorkspaceID, AgentName: normalized.AgentName,
		SourcePath: normalized.SourcePath, Action: string(normalized.Action),
		PreviousDigest: normalized.PreviousDigest, NewDigest: normalized.NewDigest, Body: normalized.Body,
		DiagnosticsJson: string(normalized.DiagnosticsJSON), ActorKind: normalized.ActorKind,
		ActorID: normalized.ActorID, OriginKind: normalized.OriginKind, OriginRef: normalized.OriginRef,
		CreatedAt: store.FormatTimestamp(normalized.CreatedAt),
	})
	if err != nil {
		return soul.Revision{}, fmt.Errorf("store: append soul revision %q: %w", normalized.ID, err)
	}
	return normalized, nil
}

// GetSoulRevision returns a managed authoring revision by id.
func (g *SoulRepo) GetSoulRevision(ctx context.Context, id string) (soul.Revision, error) {
	if err := g.checkReady(ctx, "get soul revision"); err != nil {
		return soul.Revision{}, err
	}
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return soul.Revision{}, fmt.Errorf("%w: id is required", soul.ErrInvalidRevision)
	}

	row, err := g.queries.GetSoulRevision(ctx, trimmedID)
	if errors.Is(err, sql.ErrNoRows) {
		return soul.Revision{}, fmt.Errorf("store: soul revision %q: %w", trimmedID, soul.ErrRevisionNotFound)
	}
	if err != nil {
		return soul.Revision{}, err
	}
	return soulRevisionFromGenerated(row)
}

// ListSoulRevisions lists managed authoring revisions in newest-first order.
func (g *SoulRepo) ListSoulRevisions(
	ctx context.Context,
	query soul.RevisionListQuery,
) (revisions []soul.Revision, err error) {
	if err := g.checkReady(ctx, "list soul revisions"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	// dynamic-sql: optional workspace/agent/action filters and the caller limit change the statement shape.
	sqlQuery := `SELECT id, workspace_id, agent_name, source_path, action, previous_digest, new_digest,
			body, diagnostics_json, actor_kind, actor_id, origin_kind, origin_ref, created_at
		FROM agent_soul_revisions`
	where, args := store.BuildClauses(
		store.StringClause("workspace_id", query.WorkspaceID),
		store.StringClause("agent_name", query.AgentName),
		store.StringClause("action", string(query.Action)),
	)
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += " ORDER BY created_at DESC, id DESC"
	sqlQuery, args = store.AppendLimit(sqlQuery, args, query.Limit)

	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query soul revisions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close soul revision rows: %w", closeErr))
		}
	}()

	revisions = make([]soul.Revision, 0)
	for rows.Next() {
		revision, scanErr := scanSoulRevision(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		revisions = append(revisions, revision)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate soul revisions: %w", err)
	}
	return revisions, nil
}

// FindSoulRevisionForRollback returns the revision body selected for a managed rollback.
func (g *SoulRepo) FindSoulRevisionForRollback(
	ctx context.Context,
	query soul.RollbackLookup,
) (soul.Revision, error) {
	if err := g.checkReady(ctx, "find soul rollback revision"); err != nil {
		return soul.Revision{}, err
	}
	if err := query.Validate(); err != nil {
		return soul.Revision{}, err
	}

	row, err := g.queries.GetSoulRevisionForRollback(ctx, sqlcgen.GetSoulRevisionForRollbackParams{
		WorkspaceID: strings.TrimSpace(query.WorkspaceID), AgentName: strings.TrimSpace(query.AgentName),
		ID: strings.TrimSpace(query.RevisionID),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return soul.Revision{}, fmt.Errorf(
			"store: soul rollback revision %q: %w",
			query.RevisionID,
			soul.ErrRevisionNotFound,
		)
	}
	if err != nil {
		return soul.Revision{}, err
	}
	return soulRevisionFromGenerated(row)
}

// UpdateSessionSoulSnapshot updates only the Soul provenance fields on a session row.
func (g *SoulRepo) UpdateSessionSoulSnapshot(ctx context.Context, update store.SessionSoulSnapshotUpdate) error {
	if err := g.checkReady(ctx, "update session soul snapshot"); err != nil {
		return err
	}
	if err := update.Validate(); err != nil {
		return err
	}

	updatedAt := update.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = g.now()
	}
	affected, err := g.queries.UpdateSessionSoulSnapshot(ctx, sqlcgen.UpdateSessionSoulSnapshotParams{
		SoulSnapshotID: nullableSessionString(update.SoulSnapshotID), SoulDigest: strings.TrimSpace(update.SoulDigest),
		ParentSoulDigest: strings.TrimSpace(update.ParentSoulDigest), UpdatedAt: store.FormatTimestamp(updatedAt),
		ID: strings.TrimSpace(update.ID),
	})
	if err != nil {
		return fmt.Errorf("store: update session soul snapshot %q: %w", update.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("store: session %q not found", update.ID)
	}
	return nil
}

func scanSoulSnapshot(scanner rowScanner) (soul.Snapshot, error) {
	var (
		snapshot    soul.Snapshot
		profileJSON string
		truncated   int
		createdRaw  string
	)
	if err := scanner.Scan(
		&snapshot.ID,
		&snapshot.WorkspaceID,
		&snapshot.AgentName,
		&snapshot.SourcePath,
		&snapshot.Digest,
		&profileJSON,
		&snapshot.Body,
		&truncated,
		&createdRaw,
	); err != nil {
		return soul.Snapshot{}, err
	}
	createdAt, err := store.ParseTimestamp(createdRaw)
	if err != nil {
		return soul.Snapshot{}, fmt.Errorf("store: parse soul snapshot created_at: %w", err)
	}
	snapshot.ProfileJSON = json.RawMessage(profileJSON)
	snapshot.Truncated = truncated != 0
	snapshot.CreatedAt = createdAt
	if err := snapshot.Validate(); err != nil {
		return soul.Snapshot{}, err
	}
	return snapshot.Normalize(), nil
}

func scanSoulRevision(scanner rowScanner) (soul.Revision, error) {
	var (
		revision       soul.Revision
		action         string
		diagnosticsRaw string
		createdRaw     string
	)
	if err := scanner.Scan(
		&revision.ID,
		&revision.WorkspaceID,
		&revision.AgentName,
		&revision.SourcePath,
		&action,
		&revision.PreviousDigest,
		&revision.NewDigest,
		&revision.Body,
		&diagnosticsRaw,
		&revision.ActorKind,
		&revision.ActorID,
		&revision.OriginKind,
		&revision.OriginRef,
		&createdRaw,
	); err != nil {
		return soul.Revision{}, err
	}
	createdAt, err := store.ParseTimestamp(createdRaw)
	if err != nil {
		return soul.Revision{}, fmt.Errorf("store: parse soul revision created_at: %w", err)
	}
	revision.Action = soul.RevisionAction(action)
	revision.DiagnosticsJSON = json.RawMessage(diagnosticsRaw)
	revision.CreatedAt = createdAt
	if err := revision.Validate(); err != nil {
		return soul.Revision{}, err
	}
	return revision.Normalize(), nil
}

func soulBoolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

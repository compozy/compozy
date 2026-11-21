package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// Derivation receipts outlive their side chat, so surviving children keep the same visibility.
const sessionSideHiddenIDs = `WITH RECURSIVE side_family(id, workspace_id, profile_id) AS (
 SELECT id, workspace_id, profile_id FROM sessions WHERE lineage_kind = 'side'
 UNION
 SELECT receipt.child_session_id, receipt.workspace_id, receipt.profile_id
 FROM session_derivations receipt WHERE receipt.kind = 'side'
   AND NOT EXISTS (SELECT 1 FROM sessions existing WHERE existing.id = receipt.child_session_id)
 UNION
 SELECT child.id, child.workspace_id, child.profile_id
 FROM sessions child JOIN side_family parent ON child.parent_session_id = parent.id
   AND child.workspace_id = parent.workspace_id AND child.profile_id = parent.profile_id
 WHERE child.lineage_kind NOT IN ('continue', 'fork')
) SELECT id FROM side_family`
const sessionSideVisible = `id NOT IN (` + sessionSideHiddenIDs + `)`

// SessionSideAncestry reads a scoped hierarchy edge when its metadata is unavailable.
func (g *SessionRepo) SessionSideAncestry(
	ctx context.Context, sessionID, workspaceID, profileID string,
) (*store.SessionLineage, error) {
	if err := g.checkReady(ctx, "read side ancestry"); err != nil {
		return nil, err
	}
	row, err := g.readQueries.GetSessionSideAncestry(ctx, sqlcgen.GetSessionSideAncestryParams{
		SessionID: sessionID, WorkspaceID: workspaceID, ProfileID: profileID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: read side ancestry: %w", err)
	}
	return &store.SessionLineage{
		Kind: store.LineageKind(row.LineageKind), ParentSessionID: row.ParentSessionID.String,
	}, nil
}

func (g *SessionRepo) ListSessionSides(
	ctx context.Context, workspaceID, sourceID string, archived bool,
) (result []store.SessionInfo, err error) {
	if err := g.checkReady(ctx, "list side chats"); err != nil {
		return nil, err
	}
	// dynamic-sql: reuse the catalog's canonical row mapping with an optional archive filter.
	query := sessionInfoSelectQuery + " WHERE workspace_id = ? AND parent_session_id = ? AND lineage_kind = 'side'"
	if !archived {
		query += " AND archived_at IS NULL"
	}
	query += " ORDER BY updated_at DESC, id DESC"
	rows, err := g.readDB.QueryContext(ctx, query, strings.TrimSpace(workspaceID), strings.TrimSpace(sourceID))
	if err != nil {
		return nil, fmt.Errorf("store: list side chats: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close side chat list: %w", closeErr))
		}
	}()
	result = []store.SessionInfo{}
	for rows.Next() {
		info, scanErr := scanSessionInfo(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("store: scan side chat list: %w", scanErr)
		}
		result = append(result, info)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate side chat list: %w", err)
	}
	return result, nil
}

func guardSideSource(ctx context.Context, exec globalSQLExecutor, receipt store.SessionDerivationReceipt) error {
	if receipt.Kind != store.LineageKindSide {
		return nil
	}
	source, err := sessionInWorkspace(ctx, exec, receipt.WorkspaceID, receipt.SourceSessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return store.ErrSessionNotFound
	}
	if err != nil {
		return err
	}
	if source.ArchivedAt != nil {
		return store.ErrSessionArchived
	}
	if source.ProfileID != receipt.ProfileID || source.SessionType != "user" ||
		(source.Lineage != nil &&
			(source.Lineage.Kind == store.LineageKindSide || source.Lineage.Kind == store.LineageKindSpawn)) {
		return fmt.Errorf("store: invalid side chat source")
	}
	return nil
}

var _ store.SessionSideStore = (*SessionRepo)(nil)

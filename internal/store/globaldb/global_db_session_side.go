package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// The recursive population also hides descendants left by older or internal writers.
const sessionSideHiddenIDs = `WITH RECURSIVE side_family(id) AS (
 SELECT id FROM sessions WHERE lineage_kind = 'side'
 UNION SELECT child.id FROM sessions child JOIN side_family parent ON child.parent_session_id = parent.id
) SELECT id FROM side_family`
const sessionSideVisible = `id NOT IN (` + sessionSideHiddenIDs + `)`

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
	defer func() { err = errors.Join(err, rows.Close()) }()
	result = []store.SessionInfo{}
	for rows.Next() {
		info, scanErr := scanSessionInfo(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, info)
	}
	return result, rows.Err()
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

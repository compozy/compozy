package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// Folder members carry children within their workspace and profile. As in the
// list tree, continuing a carried child keeps it there, while continuing a
// foldered root or forking any member opens new top-level work.
const sessionCatalogFolderSeed = `WITH RECURSIVE folder_walk(member, carried) AS (
 SELECT p.session_id, 0 FROM session_placements AS p
 WHERE p.profile_id = ? AND p.workspace_id = ? AND p.folder_id IS NOT NULL AND p.session_id IS NOT NULL`

const sessionCatalogFolderChildren = `
 UNION
 SELECT child.id, 1 FROM sessions AS child
 JOIN folder_walk AS parent ON child.parent_session_id = parent.member
 WHERE child.workspace_id = ? AND child.profile_id = ?
 AND NOT EXISTS (SELECT 1 FROM session_placements AS own WHERE own.session_id = child.id)
 AND (child.lineage_kind NOT IN ('continue', 'fork')
 OR (child.lineage_kind = 'continue' AND parent.carried = 1))
) SELECT member FROM folder_walk`

func sessionCatalogFolderWalk(query store.SessionCatalogPageQuery) (string, []any) {
	workspaceID := strings.TrimSpace(query.WorkspaceID)
	if workspaceID == workspaceaccess.UserSelector {
		workspaceID = ""
	}
	profileID := strings.TrimSpace(query.ReadScope.ProfileID)
	seed := sessionCatalogFolderSeed
	args := []any{profileID, workspaceID}
	if query.Folder != store.SessionCatalogRootFolder {
		seed += " AND p.folder_id = ?"
		args = append(args, query.Folder)
	}
	return seed + sessionCatalogFolderChildren, append(args, workspaceID, profileID)
}

func appendSessionCatalogFolderFilter(
	where []string, args []any, query store.SessionCatalogPageQuery,
) ([]string, []any) {
	if query.Folder == "" {
		return where, args
	}
	walk, folderArgs := sessionCatalogFolderWalk(query)
	predicate := "id IN ("
	if query.Folder == store.SessionCatalogRootFolder {
		predicate = "id NOT IN ("
	}
	return append(where, predicate+walk+")"), append(args, folderArgs...)
}

// The same transaction and membership query partition both SQLite rows and
// active sessions whose runtime snapshots will replace their durable rows.
func readSessionCatalogFolderOverlay(
	ctx context.Context, tx *sql.Tx, query store.SessionCatalogPageQuery,
) (members map[string]struct{}, err error) {
	ids, err := json.Marshal(query.ExcludeIDs)
	if err != nil {
		return nil, fmt.Errorf("store: encode session folder overlay IDs: %w", err)
	}
	walk, args := sessionCatalogFolderWalk(query)
	rows, err := tx.QueryContext(ctx, walk+" WHERE member IN (SELECT CAST(value AS TEXT) FROM json_each(?))",
		append(args, string(ids))...)
	if err != nil {
		return nil, fmt.Errorf("store: query session folder overlay: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close session folder overlay: %w", closeErr))
		}
	}()
	members = make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: scan session folder overlay: %w", err)
		}
		members[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate session folder overlay: %w", err)
	}
	return members, nil
}

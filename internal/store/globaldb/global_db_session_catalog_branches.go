package globaldb

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// SessionCatalogBranches discovers lightweight refresh keys before the board column and page cut.
func (g *SessionRepo) SessionCatalogBranches(
	ctx context.Context,
	query store.SessionCatalogPageQuery,
) (_ []store.SessionBranchKey, err error) {
	if err := g.checkReady(ctx, "read catalog branches"); err != nil {
		return nil, err
	}
	query.Columns = nil
	if err := query.Validate(); err != nil {
		return nil, err
	}
	where, args, err := sessionCatalogPageFilters(query, store.FormatTimestamp(g.now()))
	if err != nil {
		return nil, err
	}
	where = append(where, "refresh_branch <> ''", "workspace_id <> ''")
	source := `(SELECT sessions.*, CASE WHEN COALESCE(sessions.worktree_id, '') <> '' THEN
 CASE WHEN w.state IN ('pending','ready') THEN COALESCE(w.branch,'') ELSE '' END
 ELSE sessions.branch END AS refresh_branch FROM sessions
 LEFT JOIN worktrees w ON w.workspace_id = sessions.workspace_id AND w.id = sessions.worktree_id) AS sessions`
	// dynamic-sql: reuse authorized catalog filters without loading session metadata or cutting a page.
	rows, err := g.db.QueryContext(
		ctx,
		store.AppendWhere("SELECT DISTINCT workspace_id, refresh_branch FROM "+source, where),
		args...)
	if err != nil {
		return nil, fmt.Errorf("store: query catalog branches: %w", err)
	}
	defer func() { joinCleanupError(&err, rows.Close()) }()
	keys := make([]store.SessionBranchKey, 0)
	for rows.Next() {
		var key store.SessionBranchKey
		if err := rows.Scan(&key.WorkspaceID, &key.Branch); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

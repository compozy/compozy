package globaldb

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// Pending interactions remain durable authority for live and historical rows.
// Active badge overlay exclusions therefore do not apply to these counters.
func (g *SessionRepo) addTerminalApprovalFacets(
	ctx context.Context,
	query store.SessionCatalogPageQuery,
	result *store.SessionCatalogFacetResult,
) (err error) {
	query.ExcludeIDs = nil
	where, args, err := sessionCatalogPageFilters(query, store.FormatTimestamp(g.now()))
	if err != nil {
		return err
	}
	eligible := store.AppendWhere("SELECT id, workspace_id FROM sessions", where)
	// dynamic-sql: eligible sessions share the owning catalog scope and visibility filters.
	var statement strings.Builder
	statement.WriteString(`SELECT eligible.workspace_id, COUNT(1)
 FROM session_pending_interactions AS interactions
 JOIN (`)
	statement.WriteString(eligible)
	statement.WriteString(`) AS eligible ON eligible.id = interactions.session_id
 WHERE interactions.kind = 'permission' AND interactions.status = 'pending'
 AND instr(COALESCE(json_extract(interactions.payload_json, '$.tool_id'), ''), 'compozy__terminal_') = 1
 GROUP BY eligible.workspace_id ORDER BY eligible.workspace_id`)

	rows, err := g.db.QueryContext(ctx, statement.String(), args...)
	if err != nil {
		return fmt.Errorf("store: query terminal approval facets: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close terminal approval facets: %w", closeErr))
		}
	}()
	for rows.Next() {
		var workspaceID string
		var count int
		if err := rows.Scan(&workspaceID, &count); err != nil {
			return fmt.Errorf("store: scan terminal approval facets: %w", err)
		}
		result.Facets.TerminalApprovals += count
		found := false
		for index := range result.ByWorkspace {
			if result.ByWorkspace[index].WorkspaceID == workspaceID {
				result.ByWorkspace[index].Facets.TerminalApprovals = count
				found = true
				break
			}
		}
		if !found {
			result.ByWorkspace = append(
				result.ByWorkspace,
				store.WorkspaceSessionCatalogFacets{
					WorkspaceID: workspaceID,
					Facets:      store.SessionCatalogFacets{TerminalApprovals: count},
				},
			)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: iterate terminal approval facets: %w", err)
	}
	sort.Slice(
		result.ByWorkspace,
		func(i, j int) bool { return result.ByWorkspace[i].WorkspaceID < result.ByWorkspace[j].WorkspaceID },
	)
	return nil
}

package globaldb

import (
	"context"
	"fmt"
	"slices"
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
	statement := sessionTerminalApprovalFacetStatement(where)

	// dynamic-sql: eligible sessions share the owning catalog scope and visibility filters.
	rows, err := g.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return fmt.Errorf("store: query terminal approval facets: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close terminal approval facets: %w", closeErr))
		}
	}()
	for rows.Next() {
		var workspaceID, profileID string
		var count int
		if err := rows.Scan(&workspaceID, &profileID, &count); err != nil {
			return fmt.Errorf("store: scan terminal approval facets: %w", err)
		}
		result.Facets.TerminalApprovals += count
		result.TerminalApprovals = append(result.TerminalApprovals, store.TerminalApprovalCount{
			WorkspaceID: workspaceID, ProfileID: profileID, Count: count,
		})
		profileFound := false
		for i := range result.ByProfile {
			if result.ByProfile[i].ProfileID == profileID {
				result.ByProfile[i].Facets.TerminalApprovals += count
				profileFound = true
				break
			}
		}
		if !profileFound {
			result.ByProfile = append(
				result.ByProfile,
				store.ProfileSessionCatalogFacets{
					ProfileID: profileID,
					Facets:    store.SessionCatalogFacets{TerminalApprovals: count},
				},
			)
		}
		found := false
		for index := range result.ByWorkspace {
			if result.ByWorkspace[index].WorkspaceID == workspaceID {
				result.ByWorkspace[index].Facets.TerminalApprovals += count
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
	slices.SortFunc(result.ByWorkspace, func(left, right store.WorkspaceSessionCatalogFacets) int {
		return strings.Compare(left.WorkspaceID, right.WorkspaceID)
	})
	slices.SortFunc(
		result.ByProfile,
		func(a, b store.ProfileSessionCatalogFacets) int { return strings.Compare(a.ProfileID, b.ProfileID) },
	)
	return nil
}

func sessionTerminalApprovalFacetStatement(where []string) string {
	eligible := store.AppendWhere("SELECT id, workspace_id, profile_id FROM sessions", where)
	var statement strings.Builder
	statement.WriteString(`SELECT eligible.workspace_id, eligible.profile_id, COUNT(1)
 FROM session_pending_interactions AS interactions
 JOIN (`)
	statement.WriteString(eligible)
	// The active-status predicate exposes the existing partial index to SQLite.
	statement.WriteString(`) AS eligible ON eligible.id = interactions.session_id
 WHERE interactions.kind = 'permission' AND interactions.status = 'pending'
 AND interactions.status IN ('pending', 'orphaned')
 AND instr(COALESCE(json_extract(interactions.payload_json, '$.tool_id'), ''), 'compozy__terminal_') = 1
 GROUP BY eligible.workspace_id, eligible.profile_id ORDER BY eligible.workspace_id`)
	return statement.String()
}

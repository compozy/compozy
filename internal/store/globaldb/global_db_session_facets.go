package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

const sessionCatalogFacetSelect = "SELECT workspace_id, " + sessionCatalogBadgeExpression + ", COUNT(1) FROM sessions"
const sessionCatalogFacetGrouping = " GROUP BY workspace_id, " +
	sessionCatalogBadgeExpression + " ORDER BY workspace_id"

// SessionCatalogFacets counts eligible durable badge and approval metadata.
// It never scans rich rows, per-session databases, or health/transcript payloads.
func (g *SessionRepo) SessionCatalogFacets(
	ctx context.Context,
	query store.SessionCatalogPageQuery,
) (result store.SessionCatalogFacetResult, err error) {
	if err := g.checkReady(ctx, "session catalog facets"); err != nil {
		return result, err
	}
	if err := query.Validate(); err != nil {
		return result, err
	}
	query.Badges = nil
	query.AttentionOnly = false
	query.Search = ""
	where, args, err := sessionCatalogPageFilters(query, store.FormatTimestamp(g.now()))
	if err != nil {
		return result, err
	}
	// dynamic-sql: catalog scope and exclusion slices share the owning page filters.
	var statement strings.Builder
	statement.WriteString(store.AppendWhere(sessionCatalogFacetSelect, where))
	statement.WriteString(sessionCatalogFacetGrouping)

	rows, err := g.db.QueryContext(ctx, statement.String(), args...)
	if err != nil {
		return result, fmt.Errorf("store: query session catalog facets: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close session catalog facets: %w", closeErr))
		}
	}()
	result.ByWorkspace = make([]store.WorkspaceSessionCatalogFacets, 0)
	for rows.Next() {
		var workspaceID, badge string
		var count int
		if err := rows.Scan(&workspaceID, &badge, &count); err != nil {
			return result, fmt.Errorf("store: scan session catalog facets: %w", err)
		}
		if len(result.ByWorkspace) == 0 || result.ByWorkspace[len(result.ByWorkspace)-1].WorkspaceID != workspaceID {
			result.ByWorkspace = append(
				result.ByWorkspace,
				store.WorkspaceSessionCatalogFacets{WorkspaceID: workspaceID},
			)
		}
		workspace := &result.ByWorkspace[len(result.ByWorkspace)-1]
		addCatalogBadgeCount(&workspace.Facets, badge, count)
		addCatalogBadgeCount(&result.Facets, badge, count)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("store: iterate session catalog facets: %w", err)
	}
	if err := g.addTerminalApprovalFacets(ctx, query, &result); err != nil {
		return result, err
	}
	return result, nil
}

func addCatalogBadgeCount(facets *store.SessionCatalogFacets, badge string, count int) {
	facets.All += count
	switch badge {
	case "waiting-for-auth", "waiting-for-input", "failed", "needs-attention":
		facets.NeedsYou += count
	case "running":
		facets.Working += count
	case "done":
		facets.Finished += count
	case "idle":
		facets.Idle += count
	}
}

var _ store.SessionCatalogFacetReader = (*SessionRepo)(nil)

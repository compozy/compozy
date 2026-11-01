package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

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
	query.Columns = nil
	query.AttentionOnly = false
	query.Search = ""
	where, args, err := sessionCatalogPageFilters(query, store.FormatTimestamp(g.now()))
	if err != nil {
		return result, err
	}
	// dynamic-sql: catalog scope and exclusion slices share the owning page filters.
	var statement strings.Builder
	source, sourceArgs, err := sessionCatalogBoardSource(query.RepositoryKeys, query.DefaultBranches)
	if err != nil {
		return result, err
	}
	boardCount, boardArgs, err := sessionCatalogBoardCount(query.BoardExcludedIDs)
	if err != nil {
		return result, err
	}
	selection := "SELECT workspace_id, profile_id, " + sessionCatalogBadgeExpression + ", " +
		sessionCatalogBoardColumnExpression + ", COUNT(1), " + boardCount + " FROM " + source
	statement.WriteString(store.AppendWhere(selection, where))
	statement.WriteString(" GROUP BY workspace_id, profile_id, 3, 4 ORDER BY workspace_id")
	args = append(append(boardArgs, sourceArgs...), args...)

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
	profiles := map[string]store.SessionCatalogFacets{}
	for rows.Next() {
		var workspaceID, profileID, badge, column string
		var count, boardCount int
		if err := rows.Scan(&workspaceID, &profileID, &badge, &column, &count, &boardCount); err != nil {
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
		workspace.Facets.Board.Add(column, boardCount)
		addCatalogBadgeCount(&result.Facets, badge, count)
		result.Facets.Board.Add(column, boardCount)
		facets := profiles[profileID]
		addCatalogBadgeCount(&facets, badge, count)
		facets.Board.Add(column, boardCount)
		profiles[profileID] = facets
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("store: iterate session catalog facets: %w", err)
	}
	result.ByProfile = make([]store.ProfileSessionCatalogFacets, 0, len(profiles))
	for id, facets := range profiles {
		result.ByProfile = append(result.ByProfile, store.ProfileSessionCatalogFacets{ProfileID: id, Facets: facets})
	}
	slices.SortFunc(
		result.ByProfile,
		func(a, b store.ProfileSessionCatalogFacets) int { return strings.Compare(a.ProfileID, b.ProfileID) },
	)
	if err := g.addTerminalApprovalFacets(ctx, query, &result); err != nil {
		return result, err
	}
	if result.Later, err = g.sessionCatalogLaterCount(ctx, query); err != nil {
		return result, err
	}
	return result, nil
}

// sessionCatalogLaterCount counts the scope's parked sessions with every other
// filter of the read, so the Later toggle needs no second facets request.
func (g *SessionRepo) sessionCatalogLaterCount(ctx context.Context, query store.SessionCatalogPageQuery) (int, error) {
	query.Later = store.SessionLaterOnly
	where, args, err := sessionCatalogPageFilters(query, store.FormatTimestamp(g.now()))
	if err != nil {
		return 0, err
	}
	// dynamic-sql: catalog scope and exclusion slices share the owning page filters.
	statement := store.AppendWhere("SELECT COUNT(1) FROM sessions", where)
	var count int
	if err := g.db.QueryRowContext(ctx, statement, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("store: count session catalog later: %w", err)
	}
	return count, nil
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

func sessionCatalogBoardCount(excludedIDs []string) (string, []any, error) {
	if len(excludedIDs) == 0 {
		return "COUNT(1)", nil, nil
	}
	encoded, err := json.Marshal(excludedIDs)
	if err != nil {
		return "", nil, err
	}
	return "SUM(CASE WHEN id IN (SELECT value FROM json_each(?)) THEN 0 ELSE 1 END)", []any{string(encoded)}, nil
}

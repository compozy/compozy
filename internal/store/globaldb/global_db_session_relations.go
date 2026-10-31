package globaldb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// SessionCatalogRelations reads direct-child aggregates and parent titles in one scoped query.
// Continued and forked sessions name their source as provenance, so they never count as its children.
func (g *SessionRepo) SessionCatalogRelations(
	ctx context.Context,
	query store.SessionCatalogRelationsQuery,
) (result store.SessionCatalogRelations, err error) {
	if err := g.checkReady(ctx, "session catalog relations"); err != nil {
		return result, err
	}
	if err := query.ReadScope.Validate(); err != nil {
		return result, err
	}
	result.Children = make(map[string]store.SessionBoardCounts)
	result.ParentTitles = make(map[string]string)
	if len(query.SessionIDs) == 0 {
		return result, nil
	}
	statement, args, err := sessionCatalogRelationsSQL(query, store.FormatTimestamp(g.now()))
	if err != nil {
		return result, err
	}
	// dynamic-sql: scoped page identifiers and active overlay exclusions share catalog filters.
	rows, err := g.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return result, fmt.Errorf("store: query session relations: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close session relations: %w", closeErr))
		}
	}()
	for rows.Next() {
		var id, kind, value string
		var count int
		if err := rows.Scan(&id, &kind, &value, &count); err != nil {
			return result, fmt.Errorf("store: scan session relations: %w", err)
		}
		if kind == "parent" {
			result.ParentTitles[id] = value
			continue
		}
		counts := result.Children[id]
		counts.Add(value, count)
		result.Children[id] = counts
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("store: iterate session relations: %w", err)
	}
	return result, nil
}

func sessionCatalogRelationsSQL(query store.SessionCatalogRelationsQuery, now string) (string, []any, error) {
	source, sourceArgs, err := sessionCatalogBoardSource(query.RepositoryKeys, query.DefaultBranches)
	if err != nil {
		return "", nil, err
	}
	children, err := json.Marshal(query.SessionIDs)
	if err != nil {
		return "", nil, fmt.Errorf("store: encode child relation ids: %w", err)
	}
	parents, err := json.Marshal(query.ParentIDs)
	if err != nil {
		return "", nil, fmt.Errorf("store: encode parent relation ids: %w", err)
	}
	where, args, err := sessionCatalogPageFilters(store.SessionCatalogPageQuery{
		ReadScope:         query.ReadScope,
		WorkspaceID:       query.WorkspaceID,
		WorkspaceIDs:      query.WorkspaceIDs,
		Archive:           store.SessionArchiveExclude,
		ExcludeIDs:        query.ActiveIDs,
		ExcludeSpawnRoles: []string{"auto-title"},
	}, now)
	if err != nil {
		return "", nil, err
	}
	where = append(where, sessionCatalogHierarchyChildExpression, "parent_session_id IN (SELECT value FROM json_each(?))")
	args = append(sourceArgs, args...)
	args = append(args, string(children))
	statement := store.AppendWhere(
		"SELECT parent_session_id, 'children', "+sessionCatalogBoardColumnExpression+", COUNT(1) FROM "+source,
		where,
	)
	statement += " GROUP BY parent_session_id, 3 UNION ALL "
	parentWhere, parentArgs := store.BuildClauses(
		store.ReadScopeClause("profile_id", query.ReadScope),
		store.SessionWorkspaceClause(query.WorkspaceID),
	)
	parentWhere, parentArgs, err = appendSessionWorkspaceIDsFilter(parentWhere, parentArgs, query.WorkspaceIDs)
	if err != nil {
		return "", nil, err
	}
	parentWhere = append(parentWhere, "id IN (SELECT value FROM json_each(?))")
	statement += store.AppendWhere(
		"SELECT id, 'parent', "+sessionCatalogDisplayTitleExpression+", 0 FROM sessions",
		parentWhere,
	)
	args = append(args, parentArgs...)
	args = append(args, string(parents))
	return statement, args, nil
}

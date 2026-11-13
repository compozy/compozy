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
	result.ContinuedTreeParents = make(map[string]string)
	if len(query.SessionIDs) == 0 {
		return result, nil
	}
	if err := g.readContinuedTreeParents(ctx, query.ContinuedSourceIDs, result.ContinuedTreeParents); err != nil {
		return result, err
	}
	statement, args, err := sessionCatalogRelationsSQL(query, store.FormatTimestamp(g.now()))
	if err != nil {
		return result, err
	}
	// dynamic-sql: scoped page identifiers and active overlay exclusions share catalog filters.
	rows, err := g.readDB.QueryContext(ctx, statement, args...)
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
		Later:             store.SessionLaterExclude,
		ExcludeIDs:        query.ActiveIDs,
		ExcludeSpawnRoles: []string{"auto-title"},
	}, now)
	if err != nil {
		return "", nil, err
	}
	where = append(
		where,
		sessionCatalogHierarchyChildExpression,
		"parent_session_id IN (SELECT value FROM json_each(?))",
	)
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

// sessionContinuationChainLimit bounds a continue chain walk so malformed cyclic lineage terminates.
const sessionContinuationChainLimit = 64

// sessionContinuedTreeParentsQuery walks each source back through its continue chain to the
// first session that is not a continuation, and returns that session's hierarchy parent. A
// continuation deleted from the catalog is stepped over through its durable derivation
// receipt, so removing an intermediate continuation does not split the thread. A chain that
// began top-level or at a fork yields no row: forks and top-level sessions name their source
// as provenance only (store.HierarchyParentSessionID).
var sessionContinuedTreeParentsQuery = fmt.Sprintf(`WITH RECURSIVE continuation_chain(source_id, current_id, hops) AS (
 SELECT value, value, 0 FROM json_each(?)
 UNION ALL
 SELECT chain.source_id, COALESCE(predecessor.parent_session_id, receipt.source_session_id), chain.hops + 1
 FROM continuation_chain AS chain
 LEFT JOIN sessions AS predecessor ON predecessor.id = chain.current_id
 LEFT JOIN session_derivations AS receipt
  ON predecessor.id IS NULL AND receipt.child_session_id = chain.current_id AND receipt.kind = '%[1]s'
 WHERE chain.hops < %[3]d
  AND ((predecessor.lineage_kind = '%[1]s' AND COALESCE(predecessor.parent_session_id, '') <> '')
   OR (predecessor.id IS NULL AND COALESCE(receipt.source_session_id, '') <> ''))
)
SELECT chain.source_id, origin.parent_session_id
FROM continuation_chain AS chain
JOIN sessions AS origin ON origin.id = chain.current_id
WHERE COALESCE(origin.parent_session_id, '') <> ''
 AND origin.lineage_kind NOT IN ('%[1]s', '%[2]s')`,
	store.LineageKindContinue, store.LineageKindFork, sessionContinuationChainLimit)

// readContinuedTreeParents resolves the tree position each continued source stands in for.
// Only identifiers cross the read: a parent outside the caller's page or scope stays unrendered.
func (g *SessionRepo) readContinuedTreeParents(
	ctx context.Context,
	sourceIDs []string,
	treeParents map[string]string,
) (err error) {
	if len(sourceIDs) == 0 {
		return nil
	}
	encoded, err := json.Marshal(sourceIDs)
	if err != nil {
		return fmt.Errorf("store: encode continued source ids: %w", err)
	}
	// dynamic-sql: a recursive lineage walk over a variable source set has no static sqlc shape.
	rows, err := g.readDB.QueryContext(ctx, sessionContinuedTreeParentsQuery, string(encoded))
	if err != nil {
		return fmt.Errorf("store: query continued tree parents: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close continued tree parents: %w", closeErr))
		}
	}()
	for rows.Next() {
		var sourceID, parentID string
		if err := rows.Scan(&sourceID, &parentID); err != nil {
			return fmt.Errorf("store: scan continued tree parents: %w", err)
		}
		treeParents[sourceID] = parentID
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: iterate continued tree parents: %w", err)
	}
	return nil
}

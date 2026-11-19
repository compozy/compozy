package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// sessionPinScopeCTE resolves every session a pin carries, with the outermost pin of its subtree
// (pin_root) and that pin's Pinned position. The walk follows the edges the session list nests
// by: a spawned, provenance or recovery child under its parent, and a continuation under the
// parent its continue chain began under, so a continuation of a carried session is carried while
// a continuation of the pin itself stands beside it. A fork is new top-level work and is never
// carried. A pin inside another pin's subtree is carried by the outer pin and is no root itself.
// Only an active pin carries, and only within its own workspace and profile: an archived or
// parked pin still lists as pinned itself, but it and a child spawned into another project never
// pull rows out of a list that cannot show the pin above them.
const sessionPinScopeCTE = `WITH RECURSIVE pin_walk(pin_seed, pin_member, pin_carried, pin_workspace, pin_profile,
 pin_active) AS (
 SELECT id, id, 0, workspace_id, profile_id, archived_at IS NULL AND parked_at IS NULL
 FROM sessions WHERE pinned_at IS NOT NULL
 UNION
 SELECT pin_walk.pin_seed, pin_child.id, 1, pin_walk.pin_workspace, pin_walk.pin_profile, 1
 FROM sessions AS pin_child
 JOIN pin_walk ON pin_child.parent_session_id = pin_walk.pin_member
 WHERE pin_walk.pin_active = 1
 AND pin_child.workspace_id = pin_walk.pin_workspace AND pin_child.profile_id = pin_walk.pin_profile
 AND (pin_child.lineage_kind NOT IN ('continue', 'fork')
 OR (pin_child.lineage_kind = 'continue' AND pin_walk.pin_carried = 1))
), pin_scope(pin_member, pin_root, pin_root_position) AS (
 SELECT DISTINCT pin_walk.pin_member, pin_walk.pin_seed, COALESCE(pin_root_row.pin_position, 0)
 FROM pin_walk JOIN sessions AS pin_root_row ON pin_root_row.id = pin_walk.pin_seed
 WHERE pin_walk.pin_seed NOT IN (
 SELECT carried.pin_member FROM pin_walk AS carried
 WHERE carried.pin_carried = 1 AND carried.pin_seed <> carried.pin_member)
) `

// sessionPinScopeMembers is the set of sessions a pin carries, pins included.
const sessionPinScopeMembers = "(" + sessionPinScopeCTE + "SELECT pin_member FROM pin_scope)"

// Pinned order is the operator's: the smallest position leads and a new pin takes the top. A
// page holds whole subtrees: the pin first, then what it carries by last activity.
const sessionCatalogPinnedOrderClause = ` ORDER BY pin_root_position ASC, pin_root ASC,
 CASE WHEN id = pin_root THEN 0 ELSE 1 END ASC,
 COALESCE(last_update_at, updated_at) DESC, updated_at DESC, created_at DESC, id DESC`

// The pinned cursor anchors a root, never a carried row: the next page starts at the next pin.
const sessionCatalogPinnedCursorClause = `(pin_root_position > ? OR (pin_root_position = ? AND pin_root > ?))`

// appendSessionPinnedFilter keeps or leaves out every session a pin carries, so a pinned
// session's subtree lists once, in Pinned, and returns to its place when the pin clears.
func appendSessionPinnedFilter(where []string, filter store.SessionPinnedFilter) []string {
	switch filter {
	case store.SessionPinnedExclude:
		return append(where, "id NOT IN "+sessionPinScopeMembers)
	case store.SessionPinnedOnly:
		return append(where, "id IN "+sessionPinScopeMembers)
	default:
		return where
	}
}

// sessionCatalogPinnedSource adds each row's pin root and position to the page source.
func sessionCatalogPinnedSource(source string) string {
	return "(" + sessionPinScopeCTE + `SELECT sessions.*, pin_scope.pin_root AS pin_root,
 pin_scope.pin_root_position AS pin_root_position
 FROM ` + source + ` JOIN pin_scope ON pin_scope.pin_member = sessions.id) AS sessions`
}

func sessionCatalogPinnedSort(query store.SessionCatalogPageQuery) bool {
	return strings.TrimSpace(query.Sort) == sessionCatalogSortPinned
}

// pageSessionCatalogPinnedRoots lists the pins with at least one matching row in their subtree,
// which a pinned page counts and pages by.
func pageSessionCatalogPinnedRoots(
	ctx context.Context,
	tx *sql.Tx,
	query store.SessionCatalogPageQuery,
	where []string,
	args []any,
) (roots []string, err error) {
	source, sourceArgs, err := sessionCatalogPageSource(query)
	if err != nil {
		return nil, err
	}
	sqlQuery := store.AppendWhere("SELECT DISTINCT pin_root FROM "+sessionCatalogPinnedSource(source), where)
	// dynamic-sql: page filters alter the pinned root population.
	rows, err := tx.QueryContext(ctx, sqlQuery, append(sourceArgs, args...)...)
	if err != nil {
		return nil, fmt.Errorf("store: count pinned session roots: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close pinned session roots: %w", closeErr))
		}
	}()
	for rows.Next() {
		var root string
		if err := rows.Scan(&root); err != nil {
			return nil, fmt.Errorf("store: scan pinned session root: %w", err)
		}
		roots = append(roots, root)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate pinned session roots: %w", err)
	}
	return roots, nil
}

// queryPinnedSessionCatalogRows reads one page of pins: the first Limit pins after the cursor
// whose subtree has a matching row, with every matching row of those subtrees. The limit counts
// pins, so a subtree never splits across pages.
func queryPinnedSessionCatalogRows(
	ctx context.Context,
	tx *sql.Tx,
	query store.SessionCatalogPageQuery,
	where []string,
	args []any,
) (sessions []store.SessionInfo, err error) {
	source, sourceArgs, err := sessionCatalogPageSource(query)
	if err != nil {
		return nil, err
	}
	pinned := sessionCatalogPinnedSource(source)
	rootWhere := append([]string(nil), where...)
	rootArgs := append(append([]any(nil), sourceArgs...), args...)
	if query.After != nil {
		rootWhere = append(rootWhere, sessionCatalogPinnedCursorClause)
		rootArgs = append(rootArgs, *query.After.PinPosition, *query.After.PinPosition, query.After.PinRootID)
	}
	rootQuery := store.AppendWhere("SELECT pin_root FROM "+pinned, rootWhere) +
		" GROUP BY pin_root, pin_root_position ORDER BY pin_root_position ASC, pin_root ASC LIMIT ?"
	rootArgs = append(rootArgs, query.Limit)

	selectQuery := strings.TrimSuffix(sessionInfoSelectQuery, "FROM sessions") +
		", pin_root, pin_root_position FROM " + pinned
	pageWhere := append(append([]string(nil), where...), "pin_root IN ("+rootQuery+")")
	sqlQuery := store.AppendWhere(selectQuery, pageWhere) + sessionCatalogPinnedOrderClause
	pageArgs := append(append(append([]any(nil), sourceArgs...), args...), rootArgs...)

	// dynamic-sql: filters, the root cursor and the root limit alter the pinned page structure.
	rows, err := tx.QueryContext(ctx, sqlQuery, pageArgs...)
	if err != nil {
		return nil, fmt.Errorf("store: query pinned session catalog page: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close pinned session catalog rows: %w", closeErr))
		}
	}()
	for rows.Next() {
		scope := store.SessionPinScope{}
		info, scanErr := scanSessionInfo(pinScopeRowScanner{rows: rows, scope: &scope})
		if scanErr != nil {
			return nil, scanErr
		}
		info.PinScope = &scope
		sessions = append(sessions, info)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate pinned session catalog page: %w", err)
	}
	return sessions, nil
}

// pinScopeRowScanner reads a session row followed by its pin root and position.
type pinScopeRowScanner struct {
	rows  *sql.Rows
	scope *store.SessionPinScope
}

func (s pinScopeRowScanner) Scan(dest ...any) error {
	return s.rows.Scan(append(dest, &s.scope.RootID, &s.scope.Position)...)
}

// SessionPinScopes reads the Pinned placement of each listed session a pin carries.
func (g *SessionRepo) SessionPinScopes(
	ctx context.Context,
	sessionIDs []string,
) (result map[string]store.SessionPinScope, err error) {
	if err := g.checkReady(ctx, "read session pin scopes"); err != nil {
		return nil, err
	}
	result = make(map[string]store.SessionPinScope)
	if len(sessionIDs) == 0 {
		return result, nil
	}
	encoded, err := subagentIDsJSON(sessionIDs)
	if err != nil {
		return nil, err
	}
	// dynamic-sql: the recursive pin scope is shared with the catalog filters, so it stays one constant.
	rows, err := g.readDB.QueryContext(ctx, sessionPinScopeCTE+`SELECT pin_member, pin_root, pin_root_position
 FROM pin_scope WHERE pin_member IN (SELECT CAST(value AS TEXT) FROM json_each(?))`, encoded)
	if err != nil {
		return nil, fmt.Errorf("store: read session pin scopes: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close session pin scopes: %w", closeErr))
		}
	}()
	for rows.Next() {
		var member string
		var scope store.SessionPinScope
		if err := rows.Scan(&member, &scope.RootID, &scope.Position); err != nil {
			return nil, fmt.Errorf("store: scan session pin scope: %w", err)
		}
		result[member] = scope
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate session pin scopes: %w", err)
	}
	return result, nil
}

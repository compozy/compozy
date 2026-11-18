package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

const sessionCatalogSortPinned = "pinned"

// Pinned order is pin order: the oldest pin leads and a new pin lands last, so
// rows already pinned never shift. The sort only reads pinned rows.
const sessionCatalogPinnedOrderClause = " ORDER BY pinned_at ASC, id ASC"

func sessionCatalogPinnedCursorClause(position store.SessionCatalogPosition) (string, []any) {
	pinned := store.FormatTimestamp(*position.PinnedAt)
	return `(pinned_at > ? OR (pinned_at = ? AND id > ?))`, []any{pinned, pinned, strings.TrimSpace(position.ID)}
}

func appendSessionPinnedFilter(where []string, filter store.SessionPinnedFilter) []string {
	switch filter {
	case store.SessionPinnedExclude:
		return append(where, "pinned_at IS NULL")
	case store.SessionPinnedOnly:
		return append(where, "pinned_at IS NOT NULL")
	default:
		return where
	}
}

// PinSession pins one listed session. A session pinned already keeps its pin
// time, so pinning again never moves it within the pinned order.
func (g *SessionRepo) PinSession(ctx context.Context, workspaceID string, sessionID string) (store.SessionInfo, error) {
	return g.setSessionPinned(ctx, workspaceID, sessionID, true)
}

// UnpinSession clears the pin of one session; it reports false when the session was not pinned.
func (g *SessionRepo) UnpinSession(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (store.SessionInfo, bool, error) {
	info, err := g.setSessionPinned(ctx, workspaceID, sessionID, false)
	if err != nil {
		return store.SessionInfo{}, false, err
	}
	return info, info.PinnedAt == nil, nil
}

func (g *SessionRepo) setSessionPinned(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	pinned bool,
) (store.SessionInfo, error) {
	if err := g.checkReady(ctx, "set session pin"); err != nil {
		return store.SessionInfo{}, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return store.SessionInfo{}, errors.New("store: session pin session id is required")
	}
	var result store.SessionInfo
	err := g.withImmediateTransaction(ctx, "set session pin", func(exec globalSQLExecutor) error {
		current, e := readWorkspaceSessionInfo(ctx, exec, workspaceID, sessionID)
		if e != nil {
			return e
		}
		q := sqlcgen.New(exec)
		if pinned {
			if e := guardSessionPinnable(ctx, q, &current); e != nil {
				return e
			}
			_, e = q.PinSession(ctx, sqlcgen.PinSessionParams{
				PinnedAt:    sql.NullString{String: store.FormatTimestamp(g.now()), Valid: true},
				WorkspaceID: workspaceID,
				ID:          sessionID,
			})
		} else {
			_, e = q.UnpinSession(ctx, sqlcgen.UnpinSessionParams{WorkspaceID: workspaceID, ID: sessionID})
		}
		if e != nil {
			return fmt.Errorf("store: set session pin %q: %w", sessionID, e)
		}
		result, e = readWorkspaceSessionInfo(ctx, exec, workspaceID, sessionID)
		return e
	})
	return result, err
}

// guardSessionPinnable refuses sessions the session list never shows as their
// own row: subagents live in their parent's roster, mains in their agent's avatar.
func guardSessionPinnable(ctx context.Context, q *sqlcgen.Queries, info *store.SessionInfo) error {
	if info.Lineage != nil && info.Lineage.SpawnRole == store.SubagentSpawnRole {
		return store.ErrSubagentPinFollowsParent
	}
	if err := guardInstanceMainArchive(ctx, q, info.ID); err != nil {
		if errors.Is(err, store.ErrSessionIsInstanceMain) {
			return store.ErrSessionPinInstanceMain
		}
		return err
	}
	return nil
}

// SessionPinnedAt reads the pin time of each listed session that is pinned.
func (g *SessionRepo) SessionPinnedAt(ctx context.Context, sessionIDs []string) (map[string]time.Time, error) {
	if err := g.checkReady(ctx, "read session pins"); err != nil {
		return nil, err
	}
	result := make(map[string]time.Time)
	if len(sessionIDs) == 0 {
		return result, nil
	}
	encoded, err := subagentIDsJSON(sessionIDs)
	if err != nil {
		return nil, err
	}
	rows, err := g.readQueries.ListSessionPinnedAt(ctx, encoded)
	if err != nil {
		return nil, fmt.Errorf("store: read session pins: %w", err)
	}
	for _, row := range rows {
		pinnedAt, err := store.ParseTimestamp(row.PinnedAt)
		if err != nil {
			return nil, fmt.Errorf("store: parse session pinned at %q: %w", row.ID, err)
		}
		result[row.ID] = pinnedAt
	}
	return result, nil
}

// sessionReorderKeepingPinnedSlots builds a container's new order: the listed
// sessions first, then the other unlisted members in their order, except that
// an unlisted pinned member stays at the index it holds now.
func sessionReorderKeepingPinnedSlots(
	ctx context.Context,
	q *sqlcgen.Queries,
	listedIDs []string,
	listed map[string]struct{},
	members []string,
) ([]string, error) {
	unlisted := make([]string, 0, len(members))
	for _, member := range members {
		if _, ok := listed[member]; !ok {
			unlisted = append(unlisted, member)
		}
	}
	pinned := map[string]struct{}{}
	if len(unlisted) > 0 {
		encoded, err := subagentIDsJSON(unlisted)
		if err != nil {
			return nil, err
		}
		rows, err := q.ListSessionPinnedAt(ctx, encoded)
		if err != nil {
			return nil, fmt.Errorf("store: read pinned container members: %w", err)
		}
		for _, row := range rows {
			pinned[row.ID] = struct{}{}
		}
	}
	reserved := make(map[int]string, len(pinned))
	for index, member := range members {
		if _, ok := pinned[member]; ok {
			reserved[index] = member
		}
	}
	rest := append([]string(nil), listedIDs...)
	for _, member := range unlisted {
		if _, ok := pinned[member]; !ok {
			rest = append(rest, member)
		}
	}
	// Every reserved index is below the new length: the order holds every member.
	order := make([]string, 0, len(rest)+len(reserved))
	for range cap(order) {
		if member, ok := reserved[len(order)]; ok {
			order = append(order, member)
			continue
		}
		order = append(order, rest[0])
		rest = rest[1:]
	}
	return order, nil
}

var _ store.SessionPinStore = (*SessionRepo)(nil)

package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

const sessionCatalogSortPinned = "pinned"

// PinSession pins one listed session at the top of the Pinned order. A session pinned already
// keeps its pin time and position, so pinning again never moves it.
func (g *SessionRepo) PinSession(ctx context.Context, workspaceID string, sessionID string) (store.SessionInfo, error) {
	info, _, err := g.setSessionPinned(ctx, workspaceID, sessionID, true)
	return info, err
}

// UnpinSession clears the pin of one session; it reports false when the session was not pinned.
func (g *SessionRepo) UnpinSession(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (store.SessionInfo, bool, error) {
	info, changed, err := g.setSessionPinned(ctx, workspaceID, sessionID, false)
	if err != nil {
		return store.SessionInfo{}, false, err
	}
	return info, changed, nil
}

func (g *SessionRepo) setSessionPinned(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	pinned bool,
) (store.SessionInfo, bool, error) {
	if err := g.checkReady(ctx, "set session pin"); err != nil {
		return store.SessionInfo{}, false, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return store.SessionInfo{}, false, errors.New("store: session pin session id is required")
	}
	var result store.SessionInfo
	var rows int64
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
			rows, e = q.PinSession(ctx, sqlcgen.PinSessionParams{
				PinnedAt:    sql.NullString{String: store.FormatTimestamp(g.now()), Valid: true},
				WorkspaceID: workspaceID,
				ID:          sessionID,
			})
		} else {
			rows, e = q.UnpinSession(ctx, sqlcgen.UnpinSessionParams{WorkspaceID: workspaceID, ID: sessionID})
		}
		if e != nil {
			return fmt.Errorf("store: set session pin %q: %w", sessionID, e)
		}
		result, e = readWorkspaceSessionInfo(ctx, exec, workspaceID, sessionID)
		return e
	})
	// Unpin matches only a pinned row, so its row count says whether a pin was cleared.
	return result, rows > 0, err
}

// ReorderPinnedSessions gives the listed pins the Pinned slots they hold now, in the listed order,
// so a partial list (one page, a search, one project of the aggregate) moves only its own pins and
// every other pin keeps its slot. Each listed session must be pinned and belong to the scope's
// workspace and profile; a pin cleared meanwhile reports a layout conflict.
func (g *SessionRepo) ReorderPinnedSessions(
	ctx context.Context,
	scope store.SessionLayoutScope,
	sessionIDs []string,
) error {
	if err := g.checkReady(ctx, "reorder pinned sessions"); err != nil {
		return err
	}
	scope, err := scope.Normalize()
	if err != nil {
		return err
	}
	sessionIDs, err = store.NormalizeSessionLayoutIDs(sessionIDs, store.MaxSessionLayoutOrder)
	if err != nil {
		return err
	}
	if len(sessionIDs) == 0 {
		return fmt.Errorf("%w: pinned order is empty", store.ErrSessionLayoutInvalid)
	}
	encoded, err := subagentIDsJSON(sessionIDs)
	if err != nil {
		return err
	}
	return g.withImmediateTransaction(ctx, "reorder pinned sessions", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		rows, e := q.ListLayoutPinPositions(ctx, sqlcgen.ListLayoutPinPositionsParams{
			WorkspaceID: scope.WorkspaceID, ProfileID: scope.ProfileID, SessionIds: encoded,
		})
		if e != nil {
			return fmt.Errorf("store: read pinned positions: %w", e)
		}
		current := make(map[string]int64, len(rows))
		for _, row := range rows {
			current[row.ID] = row.PinPosition
		}
		slots := make([]int64, 0, len(sessionIDs))
		for _, sessionID := range sessionIDs {
			position, ok := current[sessionID]
			if !ok {
				if e := requireSessionLayoutMember(ctx, q, scope, sessionID); e != nil {
					return e
				}
				return fmt.Errorf("%w: session %s is not pinned", store.ErrSessionLayoutConflict, sessionID)
			}
			slots = append(slots, position)
		}
		slices.Sort(slots)
		for index, sessionID := range sessionIDs {
			if current[sessionID] == slots[index] {
				continue
			}
			if _, e := q.SetSessionPinPosition(ctx, sqlcgen.SetSessionPinPositionParams{
				PinPosition: sql.NullInt64{Int64: slots[index], Valid: true},
				WorkspaceID: scope.WorkspaceID,
				ID:          sessionID,
			}); e != nil {
				return fmt.Errorf("store: set pinned position %q: %w", sessionID, e)
			}
		}
		return nil
	})
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

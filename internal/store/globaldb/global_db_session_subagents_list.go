package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/listcursor"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

type subagentPosition struct {
	CreatedAt string
	ID        string
}

func subagentIDsJSON(ids []string) (string, error) {
	if len(ids) == 0 {
		return "[]", nil
	}
	data, err := json.Marshal(ids)
	return string(data), err
}
func (g *SessionRepo) ListSubagents(ctx context.Context, q store.SubagentListQuery) (store.SubagentPage, error) {
	if err := g.checkReady(ctx, "list subagents"); err != nil {
		return store.SubagentPage{}, err
	}
	if strings.TrimSpace(q.WorkspaceID) == "" && strings.TrimSpace(q.ParentSessionID) == "" {
		return store.SubagentPage{}, errors.New("store: subagent workspace or parent is required")
	}
	if q.Limit > 200 {
		q.Limit = 200
	}
	if q.Limit <= 0 {
		q.Limit = 50
	}
	q.Origins = slices.Sorted(slices.Values(q.Origins))
	q.Statuses = slices.Sorted(slices.Values(q.Statuses))
	origins, err := subagentIDsJSON(q.Origins)
	if err != nil {
		return store.SubagentPage{}, err
	}
	statuses, err := subagentIDsJSON(q.Statuses)
	if err != nil {
		return store.SubagentPage{}, err
	}
	cursor := q.Cursor
	q.Cursor = ""
	fingerprint, err := listcursor.Fingerprint(q)
	if err != nil {
		return store.SubagentPage{}, err
	}
	var pos subagentPosition
	if cursor != "" {
		pos, err = listcursor.Decode[subagentPosition](cursor, 1, "subagents", fingerprint, 0)
		if err != nil {
			return store.SubagentPage{}, err
		}
		if pos.ID == "" {
			return store.SubagentPage{}, listcursor.ErrInvalid
		}
	}
	var rows []sqlcgen.SessionSubagent
	if q.ParentSessionID != "" {
		rows, err = g.queries.ListSubagentsByParent(ctx, sqlcgen.ListSubagentsByParentParams{
			Workspace: q.WorkspaceID, Parent: q.ParentSessionID, Origins: origins, Statuses: statuses,
			CursorID: pos.ID, CursorTime: pos.CreatedAt, PageLimit: int64(q.Limit) + 1,
		})
	} else {
		rows, err = g.queries.ListSubagentsByWorkspace(ctx, sqlcgen.ListSubagentsByWorkspaceParams{
			Workspace: q.WorkspaceID, Origins: origins, Statuses: statuses,
			CursorID: pos.ID, CursorTime: pos.CreatedAt, PageLimit: int64(q.Limit) + 1,
		})
	}
	if err != nil {
		return store.SubagentPage{}, err
	}
	out := store.SubagentPage{}
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		last := rows[len(rows)-1]
		out.NextCursor, err = listcursor.Encode(1, "subagents", fingerprint, subagentPosition{last.CreatedAt, last.ID})
		if err != nil {
			return out, err
		}
	}
	out.Items, err = subagentsFromSQL(rows)
	return out, err
}
func (g *SessionRepo) Summaries(ctx context.Context, parentIDs []string) (map[string]store.SubagentSummary, error) {
	if err := g.checkReady(ctx, "summarize subagents"); err != nil {
		return nil, err
	}
	ids, err := subagentIDsJSON(parentIDs)
	if err != nil {
		return nil, err
	}
	rows, err := g.queries.SubagentSummaries(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]store.SubagentSummary, len(rows))
	for _, row := range rows {
		live := int64(row.Live.Float64)
		failed := int64(row.Failed.Float64)
		attention := int64(row.Attention.Float64)
		v := store.SubagentSummary{
			Total:     int(row.Total),
			Live:      int(live),
			Failed:    int(failed),
			Attention: int(attention),
		}
		switch {
		case attention > 0:
			v.MostUrgent = "attention"
		case failed > 0:
			v.MostUrgent = "failed"
		case live > 0:
			v.MostUrgent = "running"
		}
		out[row.ParentSessionID] = v
	}
	return out, nil
}
func (g *SessionRepo) ListStaleReserved(ctx context.Context, olderThan time.Time) ([]store.SessionSubagent, error) {
	if err := g.checkReady(ctx, "list stale subagents"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListStaleReservedSubagents(ctx, store.FormatTimestamp(olderThan))
	if err != nil {
		return nil, err
	}
	return subagentsFromSQL(rows)
}
func (g *SessionRepo) ListUnfinalizedDelegated(ctx context.Context) ([]store.SessionSubagent, error) {
	if err := g.checkReady(ctx, "list unfinalized subagents"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListUnfinalizedDelegatedSubagents(ctx)
	if err != nil {
		return nil, err
	}
	return subagentsFromSQL(rows)
}
func (g *SessionRepo) ListPending(ctx context.Context, parentID string) ([]store.SessionSubagent, error) {
	if err := g.checkReady(ctx, "list pending subagents"); err != nil {
		return nil, err
	}
	var rows []sqlcgen.SessionSubagent
	var err error
	if parentID = strings.TrimSpace(parentID); parentID == "" {
		rows, err = g.queries.ListPendingSubagents(ctx)
	} else {
		rows, err = g.queries.ListPendingSubagentsByParent(ctx, parentID)
	}
	if err != nil {
		return nil, err
	}
	return subagentsFromSQL(rows)
}
func (g *SessionRepo) ListOrphanSubagentSessions(ctx context.Context) ([]string, error) {
	if err := g.checkReady(ctx, "list orphan subagent sessions"); err != nil {
		return nil, err
	}
	return g.queries.ListOrphanSubagentSessions(ctx)
}

func (g *SessionRepo) ListUnfinalizedNative(ctx context.Context) ([]store.SessionSubagent, error) {
	if err := g.checkReady(ctx, "list unfinalized native subagents"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListUnfinalizedNativeSubagents(ctx)
	if err != nil {
		return nil, err
	}
	return subagentsFromSQL(rows)
}

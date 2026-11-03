package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *SessionRepo) SaveSessionWorkLink(ctx context.Context, link store.SessionWorkLink, automatic bool) error {
	if err := g.checkReady(ctx, "save session work link"); err != nil {
		return err
	}
	var belongs bool
	const belongsQuery = "SELECT EXISTS (SELECT 1 FROM sessions WHERE id = ? AND workspace_id = ?)"
	if err := g.readDB.QueryRowContext(ctx, belongsQuery, link.SessionID, link.WorkspaceID).
		Scan(&belongs); err != nil {
		return fmt.Errorf("store: validate linked session: %w", err)
	}
	if !belongs {
		return fmt.Errorf("store: linked session does not belong to workspace")
	}
	var number sql.NullInt64
	if link.PRNumber > 0 {
		number = sql.NullInt64{Int64: int64(link.PRNumber), Valid: true}
	}
	params := sqlcgen.SaveSessionWorkLinkParams{
		SessionID: link.SessionID, WorkspaceID: link.WorkspaceID, RepoKey: link.Repository, Branch: link.Branch,
		PrNumber: number, WorktreeID: link.WorktreeID, WorktreeName: link.WorktreeName, Provenance: link.Provenance,
	}
	var err error
	if automatic {
		_, err = g.queries.SaveAutomaticSessionWorkLink(ctx, sqlcgen.SaveAutomaticSessionWorkLinkParams(params))
	} else {
		_, err = g.queries.SaveSessionWorkLink(ctx, params)
	}
	if err != nil {
		return fmt.Errorf("store: save session work link: %w", err)
	}
	return nil
}

func (g *SessionRepo) UnlinkSessionWork(ctx context.Context, link store.SessionWorkLink) error {
	if err := g.checkReady(ctx, "unlink session work"); err != nil {
		return err
	}
	_, err := g.queries.UnlinkSessionWork(
		ctx,
		sqlcgen.UnlinkSessionWorkParams{
			SessionID:    link.SessionID,
			WorkspaceID:  link.WorkspaceID,
			RepoKey:      link.Repository,
			Branch:       link.Branch,
			PrNumber:     sql.NullInt64{Int64: int64(link.PRNumber), Valid: link.PRNumber > 0},
			WorktreeID:   link.WorktreeID,
			WorktreeName: link.WorktreeName,
			Provenance:   link.Provenance,
		},
	)
	if err != nil {
		return fmt.Errorf("store: unlink session work: %w", err)
	}
	return nil
}

func (g *SessionRepo) SessionWorkLinks(ctx context.Context, ids []string) (map[string][]store.SessionWorkLink, error) {
	if err := g.checkReady(ctx, "read session work links"); err != nil {
		return nil, err
	}
	data, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	rows, err := g.readQueries.ReadSessionWorkLinks(ctx, string(data))
	if err != nil {
		return nil, fmt.Errorf("store: read session work links: %w", err)
	}
	out := make(map[string][]store.SessionWorkLink)
	for _, r := range rows {
		out[r.SessionID] = append(
			out[r.SessionID],
			store.SessionWorkLink{
				SessionID:       r.SessionID,
				WorkspaceID:     r.WorkspaceID,
				Repository:      r.RepoKey,
				Branch:          r.Branch,
				PRNumber:        int(r.PrNumber.Int64),
				WorktreeID:      r.WorktreeID,
				WorktreeName:    r.WorktreeName,
				Provenance:      r.Provenance,
				Unlinked:        r.Unlinked != 0,
				WorktreeRemoved: r.WorktreeRemoved != 0,
			},
		)
	}
	return out, nil
}

func (g *SessionRepo) SessionPendingWork(ctx context.Context, ids []string) (map[string]bool, error) {
	if err := g.checkReady(ctx, "read pending session work"); err != nil {
		return nil, err
	}
	data, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	rows, err := g.readQueries.ReadSessionPendingWork(ctx, string(data))
	if err != nil {
		return nil, fmt.Errorf("store: read pending session work: %w", err)
	}
	out := make(map[string]bool, len(rows))
	for _, id := range rows {
		out[id] = true
	}
	return out, nil
}

func (g *SessionRepo) SessionWorkFamily(ctx context.Context, ids []string) ([]store.SessionWorkMember, error) {
	if err := g.checkReady(ctx, "read session work family"); err != nil {
		return nil, err
	}
	data, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	rows, err := g.readQueries.ReadSessionWorkFamily(ctx, string(data))
	if err != nil {
		return nil, fmt.Errorf("store: read linked work family: %w", err)
	}
	out := make([]store.SessionWorkMember, 0, len(rows))
	for _, r := range rows {
		out = append(
			out,
			store.SessionWorkMember{
				RootID:       r.RootID,
				ID:           r.ID,
				WorkspaceID:  r.WorkspaceID,
				ProfileID:    r.ProfileID,
				Name:         r.Name.String,
				WorktreeID:   r.WorktreeID,
				WorktreeName: r.WorktreeName,
			},
		)
	}
	return out, nil
}

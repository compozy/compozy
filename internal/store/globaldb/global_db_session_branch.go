package globaldb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *SessionRepo) SessionBranches(ctx context.Context, ids []string) (map[string]string, error) {
	if err := g.checkReady(ctx, "read session branches"); err != nil {
		return nil, err
	}
	data, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	rows, err := g.queries.ReadSessionBranches(ctx, string(data))
	if err != nil {
		return nil, fmt.Errorf("store: read session branches: %w", err)
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		out[row.ID] = row.Branch
	}
	return out, nil
}

func (g *SessionRepo) SaveSessionBranch(ctx context.Context, workspaceID, id, branch string) error {
	if err := g.checkReady(ctx, "save session branch"); err != nil {
		return err
	}
	_, err := g.queries.SaveSessionBranch(
		ctx,
		sqlcgen.SaveSessionBranchParams{WorkspaceID: workspaceID, ID: id, Branch: branch},
	)
	if err != nil {
		return fmt.Errorf("store: save session branch: %w", err)
	}
	return nil
}

func (g *SessionRepo) SessionCatalogProfiles(ctx context.Context, workspaceID string) ([]string, error) {
	return g.queries.SessionCatalogProfiles(ctx, workspaceID)
}

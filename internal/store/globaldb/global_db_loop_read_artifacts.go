package globaldb

import (
	"context"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// ListAvailableLoopOutputRefs returns retained blob references owned by one workspace run.
func (g *LoopRepo) ListAvailableLoopOutputRefs(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) (map[string]bool, error) {
	if err := g.checkReady(ctx, "list available loop output refs"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListAvailableLoopOutputRefs(ctx, sqlcgen.ListAvailableLoopOutputRefsParams{
		WorkspaceID: string(workspaceID), LoopRunID: string(runID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list available loop output refs: %w", err)
	}
	available := make(map[string]bool, len(rows))
	for _, outputRef := range rows {
		available[outputRef] = true
	}
	return available, nil
}

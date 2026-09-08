package globaldb

import (
	"context"
	"database/sql"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// BackfillLoopWaitAdmissionDeadlines normalizes one page of released expiryless
// event waits. The executed definition and original creation time own the deadline.
func (g *LoopRepo) BackfillLoopWaitAdmissionDeadlines(ctx context.Context, limit int) (int, error) {
	if err := g.checkReady(ctx, "normalize Loop wait admission deadlines"); err != nil {
		return 0, err
	}
	if limit <= 0 {
		return 0, fmt.Errorf("store: wait deadline page limit must be positive")
	}
	rows, err := g.queries.ListExpirylessLoopEventWaits(ctx, int64(limit))
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		run, err := getLoopRunByIDWithExecutor(ctx, g.db, looppkg.RunID(row.LoopRunID))
		if err != nil {
			return 0, err
		}
		resolved, err := loadWaitExecutedDefinition(ctx, g.db, run)
		if err != nil {
			return 0, err
		}
		node, found := loopWaitNodeByRuntimeID(resolved.Definition.Graph, "", row.NodeID)
		if !found {
			return 0, fmt.Errorf("store: expiryless wait node %q is missing from executed definition", row.NodeID)
		}
		lifecycle, err := looppkg.ResolveNodeLifecycleConfig(node, nil, resolved.EffectiveConfig.Lifecycle)
		if err != nil {
			return 0, err
		}
		deadline := row.CreatedAt.UTC().Add(lifecycle.AdmissionHorizon)
		if _, err := g.queries.BackfillLoopEventWaitDeadline(ctx, sqlcgen.BackfillLoopEventWaitDeadlineParams{
			LoopRunID: row.LoopRunID, Generation: row.Generation, NodeID: row.NodeID, ItemIndex: row.ItemIndex,
			Deadline: sql.NullTime{Time: deadline, Valid: true},
		}); err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}

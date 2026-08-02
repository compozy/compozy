package globaldb

import (
	"context"
	"encoding/json"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// ListEffectOutbox returns workspace-scoped effect delivery rows for one run.
func (g *LoopRepo) ListEffectOutbox(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) ([]looppkg.EffectOutboxEntry, error) {
	if err := g.checkReady(ctx, "list effect outbox"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListLoopEffectOutbox(ctx, sqlcgen.ListLoopEffectOutboxParams{
		WorkspaceID: string(workspaceID),
		LoopRunID:   string(runID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list loop effect outbox: %w", err)
	}
	entries := make([]looppkg.EffectOutboxEntry, 0, len(rows))
	for _, row := range rows {
		state := looppkg.EffectOutboxState(row.State)
		if !state.Valid() {
			return nil, fmt.Errorf(
				"store: invalid loop effect outbox state %q: %w",
				row.State,
				looppkg.ErrValidation,
			)
		}
		entries = append(entries, looppkg.EffectOutboxEntry{
			LoopRunID:     looppkg.RunID(row.LoopRunID),
			DeliveryID:    row.DeliveryID,
			SourceEventID: row.SourceEventID,
			Trigger:       row.Trigger,
			Generation:    int(row.Generation),
			NodeID:        looppkg.NodeID(row.NodeID),
			ItemIndex:     int(row.ItemIndex),
			EntryIndex:    int(row.EntryIndex),
			Entry:         json.RawMessage(row.EntryJson),
			State:         state,
			Attempts:      int(row.Attempts),
			CreatedAt:     row.CreatedAt.UTC(),
			DeliveredAt:   loopTimePointer(row.DeliveredAt),
		})
	}
	return entries, nil
}

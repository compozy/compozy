package globaldb

import (
	"context"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// ListSessionLoopWork reads durable reconciliation evidence within one session scope.
func (g *LoopRepo) ListSessionLoopWork(
	ctx context.Context,
	scope store.ReadScope,
	workspaceID, sessionID string,
	ownerRunID looppkg.RunID,
) ([]looppkg.SessionWork, error) {
	if err := g.checkReady(ctx, "inspect session loop work"); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListSessionLoopWork(ctx, sqlcgen.ListSessionLoopWorkParams{
		WorkspaceID: workspaceID, AllProfiles: boolToInt64(scope.AllProfiles), ProfileID: scope.ProfileID,
		SessionID: nullableAutomationString(sessionID), OwnerRunID: string(ownerRunID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: inspect session loop work: %w", err)
	}
	result := make([]looppkg.SessionWork, 0, len(rows))
	for _, row := range rows {
		work := looppkg.SessionWork{
			RunID:          looppkg.RunID(row.ID),
			Status:         looppkg.Status(row.Status),
			NeedsAttention: row.NeedsAttention,
			OwnsGoal:       row.OwnsGoal != 0,
		}
		work.Since, err = store.ParseTimestamp(row.CreatedAt)
		if err != nil {
			return nil, err
		}
		if row.ReconciledAt != "" {
			work.ReconciledAt, err = store.ParseTimestamp(row.ReconciledAt)
			if err != nil {
				return nil, err
			}
		}
		if row.LeaseUntil != "" {
			work.LeaseUntil, err = store.ParseTimestamp(row.LeaseUntil)
			if err != nil {
				return nil, err
			}
		}
		work.Waits, err = g.ListNodeWaits(ctx, looppkg.WorkspaceID(workspaceID), work.RunID)
		if err != nil {
			return nil, err
		}
		result = append(result, work)
	}
	return result, nil
}

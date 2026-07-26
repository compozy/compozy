package globaldb

import (
	"context"
	"fmt"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

// RecordTokenUsage writes the session aggregate and the daily rollup in one
// transaction so a rollup failure cannot leave overview usage undercounting the
// tokens already committed to token_stats.
func (g *ObserveRepo) RecordTokenUsage(
	ctx context.Context,
	stats store.TokenStatsUpdate,
	daily store.TokenUsageDailyUpdate,
) (err error) {
	if err := g.checkReady(ctx, "record token usage"); err != nil {
		return err
	}
	if err := stats.Validate(); err != nil {
		return err
	}
	if err := daily.Validate(); err != nil {
		return err
	}

	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin record token usage: %w", err)
	}
	defer func() {
		joinCleanupError(&err, rollbackTx(tx, "record token usage"))
	}()

	queries := sqlcgen.New(tx)
	if statsErr := queries.UpsertTokenStats(ctx, g.tokenStatsParams(stats)); statsErr != nil {
		return fmt.Errorf("store: upsert token stats for session %q: %w", stats.SessionID, statsErr)
	}
	if dailyErr := queries.UpsertTokenUsageDaily(ctx, g.tokenUsageDailyParams(daily)); dailyErr != nil {
		return fmt.Errorf("store: upsert token usage daily for day %q: %w", daily.Day, dailyErr)
	}
	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("store: commit record token usage: %w", commitErr)
	}
	return nil
}

package globaldb

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
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
	statsParams, err := g.tokenStatsParams(stats)
	if err != nil {
		return err
	}

	return store.ExecuteWriteOperation(
		ctx,
		g.db,
		"record token usage",
		func(ctx context.Context, tx *store.WriteTx) error {
			queries := sqlcgen.New(tx)
			if statsErr := queries.UpsertTokenStats(ctx, statsParams); statsErr != nil {
				return fmt.Errorf("store: upsert token stats for session %q: %w", stats.SessionID, statsErr)
			}
			if dailyErr := queries.UpsertTokenUsageDaily(ctx, g.tokenUsageDailyParams(daily)); dailyErr != nil {
				return fmt.Errorf("store: upsert token usage daily for day %q: %w", daily.Day, dailyErr)
			}
			return nil
		},
	)
}

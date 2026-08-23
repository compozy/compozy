package globaldb

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// DeleteWorkspaceObservability removes one workspace's event summaries and token
// usage rollups. Bootstrap importers call it before re-seeding, because the daily
// usage rollup is additive and would otherwise double on every replace.
func (g *ObserveRepo) DeleteWorkspaceObservability(ctx context.Context, workspaceID string) error {
	if err := g.checkReady(ctx, "delete workspace observability"); err != nil {
		return err
	}
	trimmed := strings.TrimSpace(workspaceID)
	if trimmed == "" {
		return errors.New("store: workspace id is required")
	}
	return g.withImmediateTransaction(
		ctx,
		"delete workspace observability",
		func(exec globalSQLExecutor) error {
			if _, err := exec.ExecContext(
				ctx, `DELETE FROM event_summaries WHERE workspace_id = ?`, trimmed,
			); err != nil {
				return fmt.Errorf("store: delete event summaries for workspace %q: %w", trimmed, err)
			}
			if _, err := exec.ExecContext(
				ctx, `DELETE FROM token_usage_daily WHERE workspace_id = ?`, trimmed,
			); err != nil {
				return fmt.Errorf("store: delete token usage rollups for workspace %q: %w", trimmed, err)
			}
			return nil
		},
	)
}

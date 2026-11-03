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
	if _, err := g.telemetry.DeleteScope(ctx, "workspace", trimmed); err != nil {
		return fmt.Errorf("store: delete workspace event summaries: %w", err)
	}
	return g.withImmediateTransaction(ctx, "delete workspace usage rollups", func(exec globalSQLExecutor) error {
		_, err := exec.ExecContext(ctx, `DELETE FROM token_usage_daily WHERE workspace_id = ?`, trimmed)
		return err
	})
}

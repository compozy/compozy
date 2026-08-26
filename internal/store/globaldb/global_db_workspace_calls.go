package globaldb

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func terminalizeWorkspaceCalls(
	ctx context.Context,
	exec *store.WriteTx,
	workspaceID string,
	at time.Time,
) error {
	timestamp := store.FormatTimestamp(at)
	if _, err := exec.ExecContext(ctx, `UPDATE task_runs
		SET status = 'canceled', error = 'workspace removed', ended_at = ?, claim_token = NULL,
			claim_token_hash = NULL, lease_until = NULL, heartbeat_at = NULL,
			workspace_id = NULL, worktree_id = NULL
		WHERE id IN (
			SELECT activation.run_id FROM call_activation_runs activation
			JOIN calls call ON call.call_id = activation.call_id
			WHERE call.workspace_id = ? AND call.state IN ('queued', 'running')
		) AND status IN ('queued', 'claimed', 'starting', 'running')`, timestamp, workspaceID); err != nil {
		return fmt.Errorf("store: cancel call activations for removed workspace %q: %w", workspaceID, err)
	}
	if _, err := exec.ExecContext(ctx, `UPDATE calls
		SET state = 'failed', failure_code = 'call_workspace_removed',
			failure_detail = 'workspace removed', activation_run_id = NULL,
			settled_at = ?, updated_at = ?
		WHERE workspace_id = ? AND state IN ('queued', 'running')`, timestamp, timestamp, workspaceID); err != nil {
		return fmt.Errorf("store: terminalize calls for removed workspace %q: %w", workspaceID, err)
	}
	return nil
}

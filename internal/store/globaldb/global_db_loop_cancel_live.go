package globaldb

import (
	"context"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func nodeHasLiveCancellationOutputs(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	nodeID looppkg.NodeID,
) (bool, error) {
	var live int
	err := exec.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM loop_generation_outputs
		WHERE loop_run_id = ? AND node_id = ? AND status IN (`+liveCancelOutputStatuses+`)
	)`, runID, nodeID).Scan(&live)
	if err != nil {
		return false, fmt.Errorf("store: inspect live Loop node cancellation outputs: %w", err)
	}
	return live == 1, nil
}

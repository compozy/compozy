package globaldb

import (
	"context"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func (g *LoopRepo) ListAvailableLoopOutputRefs(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) (available map[string]bool, err error) {
	if err := g.checkReady(ctx, "list available loop output refs"); err != nil {
		return nil, err
	}
	rows, err := g.db.QueryContext(ctx, `
		SELECT DISTINCT output.output_ref
		FROM loop_generation_outputs AS output
		JOIN loop_runs AS run ON run.id = output.loop_run_id
		JOIN loop_output_blobs AS blob ON blob.output_ref = output.output_ref
		WHERE run.workspace_id = ?
		  AND output.loop_run_id = ?
		  AND output.output_ref <> ''`, workspaceID, runID)
	if err != nil {
		return nil, fmt.Errorf("store: list available loop output refs: %w", err)
	}
	defer func() {
		err = joinRowsCloseError(rows, err, "available loop output refs query")
	}()

	available = make(map[string]bool)
	for rows.Next() {
		var outputRef string
		if scanErr := rows.Scan(&outputRef); scanErr != nil {
			return nil, fmt.Errorf("store: scan available loop output ref: %w", scanErr)
		}
		available[outputRef] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate available loop output refs: %w", err)
	}
	return available, nil
}

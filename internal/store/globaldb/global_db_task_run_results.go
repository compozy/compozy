package globaldb

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func attachTaskRunResultDescriptors(
	ctx context.Context,
	exec taskSQLExecutor,
	runs []taskpkg.Run,
) (_ []taskpkg.Run, returnErr error) {
	if len(runs) == 0 {
		return runs, nil
	}
	runIndexes := make(map[string]int, len(runs))
	runIDs := make([]string, 0, len(runs))
	for index := range runs {
		result := runs[index].ResultValue()
		runs[index].SetResult(result)
		if runs[index].ResultByteCount() > 0 {
			continue
		}
		runIndexes[runs[index].ID] = index
		runIDs = append(runIDs, runs[index].ID)
	}
	if len(runIDs) == 0 {
		return runs, nil
	}
	placeholders, args := sqlInPlaceholders(runIDs)
	// dynamic-sql: the run-id batch changes the IN arity and prevents per-run descriptor queries.
	query := `SELECT DISTINCT output.task_run_id, output.output_ref, blob.byte_size
		FROM loop_generation_outputs AS output
		JOIN loop_output_blobs AS blob ON blob.output_ref = output.output_ref
		WHERE output.task_run_id IN (` + placeholders + `)
		  AND output.output_ref IS NOT NULL
		ORDER BY output.task_run_id ASC`
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query task run result descriptors: %w", err)
	}
	defer func() {
		returnErr = joinRowsCloseError(rows, returnErr, "task run result descriptor query")
	}()
	seen := make(map[string]struct{}, len(runIDs))
	for rows.Next() {
		var runID string
		var resultRef string
		var resultBytes int64
		if err := rows.Scan(&runID, &resultRef, &resultBytes); err != nil {
			return nil, fmt.Errorf("store: scan task run result descriptor: %w", err)
		}
		index, ok := runIndexes[runID]
		if !ok {
			return nil, fmt.Errorf("%w: result descriptor references unknown run", taskpkg.ErrTaskRunResultCorrupt)
		}
		if _, duplicated := seen[runID]; duplicated {
			return nil, fmt.Errorf("%w: multiple external results for run %q", taskpkg.ErrTaskRunResultCorrupt, runID)
		}
		seen[runID] = struct{}{}
		if !looppkg.OutputRefLooksContentAddressed(resultRef) || resultBytes <= 0 {
			return nil, fmt.Errorf("%w: invalid descriptor for run %q", taskpkg.ErrTaskRunResultCorrupt, runID)
		}
		runs[index].SetExternalResult(resultRef, resultBytes)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate task run result descriptors: %w", err)
	}
	return runs, nil
}

// ReadTaskRunResultPage returns one integrity-checked exact page from an externalized result.
func (g *TaskRunRepo) ReadTaskRunResultPage(
	ctx context.Context,
	runID string,
	offset int64,
	limit int64,
) (_ taskpkg.RunResultPage, returnErr error) {
	if err := g.checkReady(ctx, "read task run result page"); err != nil {
		return taskpkg.RunResultPage{}, err
	}
	trimmedRunID, err := requireTaskValue(runID, "task run id")
	if err != nil {
		return taskpkg.RunResultPage{}, err
	}
	if offset < 0 || limit < 0 || limit > taskpkg.MaxRunResultPageBytes {
		return taskpkg.RunResultPage{}, taskpkg.ErrTaskRunResultInvalidRange
	}
	rows, err := g.db.QueryContext(
		ctx,
		`SELECT DISTINCT output.output_ref, blob.payload_json, blob.byte_size
		 FROM loop_generation_outputs AS output
		 JOIN loop_output_blobs AS blob ON blob.output_ref = output.output_ref
		 WHERE output.task_run_id = ? AND output.output_ref IS NOT NULL
		 ORDER BY output.loop_run_id, output.generation, output.node_id, output.item_index`,
		trimmedRunID,
	)
	if err != nil {
		return taskpkg.RunResultPage{}, fmt.Errorf("store: query task run result: %w", err)
	}
	defer func() {
		returnErr = joinRowsCloseError(rows, returnErr, "task run result query")
	}()
	var resultRef string
	var payload string
	var resultBytes int64
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return taskpkg.RunResultPage{}, fmt.Errorf("store: iterate task run result: %w", err)
		}
		return taskpkg.RunResultPage{}, taskpkg.ErrTaskRunResultNotFound
	}
	if err := rows.Scan(&resultRef, &payload, &resultBytes); err != nil {
		return taskpkg.RunResultPage{}, fmt.Errorf("store: scan task run result: %w", err)
	}
	if rows.Next() {
		return taskpkg.RunResultPage{}, fmt.Errorf(
			"%w: multiple external results for run %q",
			taskpkg.ErrTaskRunResultCorrupt,
			trimmedRunID,
		)
	}
	if err := rows.Err(); err != nil {
		return taskpkg.RunResultPage{}, fmt.Errorf("store: iterate task run result: %w", err)
	}
	raw := []byte(payload)
	if resultBytes != int64(len(raw)) || looppkg.OutputRefForPayload(raw) != strings.TrimSpace(resultRef) {
		return taskpkg.RunResultPage{}, fmt.Errorf(
			"%w: digest or byte size mismatch for run %q",
			taskpkg.ErrTaskRunResultCorrupt,
			trimmedRunID,
		)
	}
	page, err := taskpkg.PageRunResult(trimmedRunID, resultRef, raw, offset, limit)
	if err != nil {
		return taskpkg.RunResultPage{}, err
	}
	return page, nil
}

package globaldb

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"

	automation "github.com/compozy/compozy/internal/automation/model"
)

func (g *AutomationRepo) LatestRunsByOwner(
	ctx context.Context, owner automation.RunOwnerKind, ids []string,
) (map[string]automation.Run, error) {
	if err := g.checkReady(ctx, "list latest automation runs"); err != nil {
		return nil, err
	}
	query, err := automationLatestRunsSQL(owner)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return map[string]automation.Run{}, nil
	}
	idsJSON, err := json.Marshal(ids)
	if err != nil {
		return nil, fmt.Errorf("store: encode latest automation run owner ids: %w", err)
	}
	return readLatestAutomationRuns(ctx, g.db, query, owner, string(idsJSON))
}

func automationLatestRunsSQL(owner automation.RunOwnerKind) (string, error) {
	var column, index string
	switch owner {
	case automation.RunOwnerJob:
		column, index = "job_id", "idx_automation_runs_job_latest"
	case automation.RunOwnerTrigger:
		column, index = "trigger_id", "idx_automation_runs_trigger_latest"
	default:
		return "", fmt.Errorf("store: invalid automation run owner %q", owner)
	}
	// dynamic-sql: the closed owner kind selects its owner column and matching partial index.
	return `WITH requested_ids(owner_id) AS (
        SELECT DISTINCT CAST(value AS TEXT) FROM json_each(?)
    ), latest_ids(latest_id) AS (
        SELECT (
            SELECT candidate.id FROM automation_runs AS candidate INDEXED BY ` + index + `
            WHERE candidate.` + column + ` = requested.owner_id
            ORDER BY candidate.started_at DESC, candidate.id DESC LIMIT 1
        ) FROM requested_ids AS requested
    )
    SELECT id, ` + automationRunProfileIDSQL + `, job_id, trigger_id, session_id, task_id, task_run_id, fire_id,
        status, attempt, scheduled_at, started_at, ended_at, error,
        delivery_error, delivery_error_at, loop_run_id, metadata_json
    FROM automation_runs JOIN latest_ids ON latest_ids.latest_id = automation_runs.id`, nil
}

func readLatestAutomationRuns(
	ctx context.Context, executor automationCatalogExecutor, query string,
	owner automation.RunOwnerKind, idsJSON string,
) (runs map[string]automation.Run, err error) {
	rows, err := executor.QueryContext(ctx, query, idsJSON)
	if err != nil {
		return nil, fmt.Errorf("store: query latest automation runs: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close latest automation run rows: %w", closeErr))
		}
	}()
	runs = make(map[string]automation.Run)
	for rows.Next() {
		run, scanErr := scanAutomationRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		id := run.JobID
		if owner == automation.RunOwnerTrigger {
			id = run.TriggerID
		}
		runs[id] = run
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate latest automation runs: %w", err)
	}
	return runs, nil
}

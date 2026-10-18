package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// PutTaskDesignationRollup upserts one terminal rollup artifact.
func (g *TaskRepo) PutTaskDesignationRollup(ctx context.Context, rollup store.TaskDesignationRollup) error {
	if err := g.checkReady(ctx, "put task designation rollup"); err != nil {
		return err
	}
	normalized := rollup
	normalized.DesignationGroupID = strings.TrimSpace(normalized.DesignationGroupID)
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.SummaryJSON = json.RawMessage(strings.TrimSpace(string(normalized.SummaryJSON)))
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if err := normalized.Validate(); err != nil {
		return fmt.Errorf("store: validate task designation rollup: %w", err)
	}
	if err := g.queries.UpsertTaskDesignationRollup(ctx, sqlcgen.UpsertTaskDesignationRollupParams{
		DesignationGroupID: normalized.DesignationGroupID,
		TaskID:             normalized.TaskID,
		SummaryJson:        string(normalized.SummaryJSON),
		CreatedAt:          store.FormatTimestamp(normalized.CreatedAt),
	}); err != nil {
		return fmt.Errorf("store: upsert task designation rollup: %w", err)
	}
	return nil
}

// ListTaskDesignationRollups returns rollups by task or group.
func (g *TaskRepo) ListTaskDesignationRollups(
	ctx context.Context,
	query store.TaskDesignationRollupQuery,
) (rollups []store.TaskDesignationRollup, err error) {
	if err := g.checkReady(ctx, "list task designation rollups"); err != nil {
		return nil, err
	}
	normalized := store.TaskDesignationRollupQuery{
		DesignationGroupID: strings.TrimSpace(query.DesignationGroupID),
		TaskID:             strings.TrimSpace(query.TaskID),
		Limit:              query.Limit,
	}
	if err := normalized.Validate(); err != nil {
		return nil, fmt.Errorf("store: validate task designation rollup query: %w", err)
	}
	// dynamic-sql: optional task/group filters and the caller-provided limit change the statement shape.
	sqlQuery := `SELECT designation_group_id, task_id, summary_json, created_at FROM task_designation_rollups`
	where, args := store.BuildClauses(
		store.StringClause("designation_group_id", normalized.DesignationGroupID),
		store.StringClause("task_id", normalized.TaskID),
	)
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += " ORDER BY created_at DESC, designation_group_id ASC"
	sqlQuery, args = store.AppendLimit(sqlQuery, args, normalized.Limit)

	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query task designation rollups: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			closeErr = fmt.Errorf("store: close task designation rollup rows: %w", closeErr)
			if err != nil {
				err = errors.Join(err, closeErr)
				return
			}
			err = closeErr
		}
	}()

	rollups = make([]store.TaskDesignationRollup, 0)
	for rows.Next() {
		rollup, scanErr := scanTaskDesignationRollup(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		rollups = append(rollups, rollup)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate task designation rollups: %w", err)
	}
	return rollups, nil
}

func scanTaskDesignationRollup(scanner rowScanner) (store.TaskDesignationRollup, error) {
	var rollup store.TaskDesignationRollup
	var summaryRaw, createdRaw string
	if err := scanner.Scan(&rollup.DesignationGroupID, &rollup.TaskID, &summaryRaw, &createdRaw); err != nil {
		return store.TaskDesignationRollup{}, fmt.Errorf("store: scan task designation rollup: %w", err)
	}
	rollup.SummaryJSON = json.RawMessage(summaryRaw)
	createdAt, err := store.ParseTimestamp(createdRaw)
	if err != nil {
		return store.TaskDesignationRollup{}, fmt.Errorf("store: parse task designation rollup created_at: %w", err)
	}
	rollup.CreatedAt = createdAt
	return rollup, nil
}

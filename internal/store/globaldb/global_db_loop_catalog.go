package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

const loopCatalogLatestRunHeadColumnsSQL = `lr.id, lr.loop_name, lr.status, lr.created_at`

const loopCatalogLatestRunsSQL = `WITH requested_names(loop_name) AS (
		SELECT DISTINCT CAST(value AS TEXT) FROM json_each(?)
	), latest_ids(id) AS (
		SELECT (
			SELECT candidate.id
			FROM loop_runs AS candidate INDEXED BY idx_loop_runs_catalog
			WHERE candidate.workspace_id = ?
				AND candidate.loop_name = requested.loop_name
			ORDER BY candidate.created_at DESC, candidate.id DESC
			LIMIT 1
		)
		FROM requested_names AS requested
	)
	SELECT ` + loopCatalogLatestRunHeadColumnsSQL + `
	FROM loop_runs AS lr
	JOIN latest_ids ON latest_ids.id = lr.id`

type loopCatalogQueryExecutor interface {
	sqlcgen.DBTX
}

func (g *LoopRepo) ListLoopCatalogRunSummaries(
	ctx context.Context,
	query looppkg.CatalogRunQuery,
) (summaries map[string]looppkg.CatalogRunSummary, err error) {
	if err := g.checkReady(ctx, "list loop catalog run summaries"); err != nil {
		return nil, err
	}
	normalized, err := normalizeLoopCatalogRunQuery(query)
	if err != nil {
		return nil, err
	}
	summaries = make(map[string]looppkg.CatalogRunSummary, len(normalized.LoopNames))
	for _, name := range normalized.LoopNames {
		summaries[name] = looppkg.CatalogRunSummary{LoopName: name}
	}
	if len(normalized.LoopNames) == 0 {
		return summaries, nil
	}

	namesJSON, err := json.Marshal(normalized.LoopNames)
	if err != nil {
		return nil, fmt.Errorf("store: encode loop catalog names: %w", err)
	}
	tx, err := g.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("store: begin loop catalog read: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("store: rollback loop catalog read: %w", rollbackErr))
		}
	}()

	if err := readLoopCatalogRunSummaries(ctx, tx, string(namesJSON), normalized, summaries); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("store: commit loop catalog read: %w", err)
	}
	return summaries, nil
}

func readLoopCatalogRunSummaries(
	ctx context.Context,
	executor loopCatalogQueryExecutor,
	namesJSON string,
	query looppkg.CatalogRunQuery,
	summaries map[string]looppkg.CatalogRunSummary,
) error {
	if err := readLoopCatalogAggregates(ctx, executor, namesJSON, query, summaries); err != nil {
		return err
	}
	return readLoopCatalogLatestRuns(ctx, executor, namesJSON, query.WorkspaceID, summaries)
}

func readLoopCatalogAggregates(
	ctx context.Context,
	executor loopCatalogQueryExecutor,
	namesJSON string,
	query looppkg.CatalogRunQuery,
	summaries map[string]looppkg.CatalogRunSummary,
) (err error) {
	rows, err := sqlcgen.New(executor).ListLoopCatalogAggregates(ctx, sqlcgen.ListLoopCatalogAggregatesParams{
		NamesJson: namesJSON, WorkspaceID: string(query.WorkspaceID),
		AggregateAfter: store.FormatTimestamp(query.AggregateAfter),
	})
	if err != nil {
		return fmt.Errorf("store: query loop catalog aggregates: %w", err)
	}
	for _, row := range rows {
		aggregate := looppkg.CatalogRunAggregate{Runs: int(row.Runs)}
		if row.Succeeded.Valid {
			aggregate.Succeeded = int(row.Succeeded.Float64)
		}
		if row.Failed.Valid {
			aggregate.Failed = int(row.Failed.Float64)
		}
		summary := summaries[row.LoopName]
		summary.LoopName = row.LoopName
		summary.Aggregate30d = aggregate
		summaries[row.LoopName] = summary
	}
	return nil
}

func readLoopCatalogLatestRuns(
	ctx context.Context,
	executor loopCatalogQueryExecutor,
	namesJSON string,
	workspaceID looppkg.WorkspaceID,
	summaries map[string]looppkg.CatalogRunSummary,
) (err error) {
	rows, err := sqlcgen.New(executor).ListLoopCatalogLatestRuns(ctx, sqlcgen.ListLoopCatalogLatestRunsParams{
		NamesJson: namesJSON, WorkspaceID: string(workspaceID),
	})
	if err != nil {
		return fmt.Errorf("store: query loop catalog latest runs: %w", err)
	}
	for _, row := range rows {
		runStatus := looppkg.Status(row.Status)
		if !runStatus.Valid() {
			return fmt.Errorf(
				"%w: loop catalog latest run %q status is invalid: %q",
				looppkg.ErrValidation,
				row.ID,
				row.Status,
			)
		}
		createdAt, parseErr := parseLoopRunTimestamp(row.CreatedAt)
		if parseErr != nil {
			return fmt.Errorf("store: parse loop catalog latest run created_at: %w", parseErr)
		}
		run := looppkg.CatalogRunHead{
			ID:        looppkg.RunID(row.ID),
			LoopName:  row.LoopName,
			Status:    runStatus,
			CreatedAt: createdAt,
		}
		summary := summaries[row.LoopName]
		summary.LoopName = row.LoopName
		summary.LastRun = &run
		summaries[row.LoopName] = summary
	}
	return nil
}

func normalizeLoopCatalogRunQuery(query looppkg.CatalogRunQuery) (looppkg.CatalogRunQuery, error) {
	normalized := query
	normalized.WorkspaceID = looppkg.WorkspaceID(strings.TrimSpace(string(query.WorkspaceID)))
	normalized.AggregateAfter = query.AggregateAfter.UTC()
	if normalized.WorkspaceID == "" {
		return looppkg.CatalogRunQuery{}, fmt.Errorf("%w: workspace_id is required", looppkg.ErrValidation)
	}
	if normalized.AggregateAfter.IsZero() {
		return looppkg.CatalogRunQuery{}, fmt.Errorf("%w: aggregate_after is required", looppkg.ErrValidation)
	}
	names := make([]string, 0, len(query.LoopNames))
	seen := make(map[string]struct{}, len(query.LoopNames))
	for _, raw := range query.LoopNames {
		name, err := looppkg.ValidateName(raw)
		if err != nil {
			return looppkg.CatalogRunQuery{}, fmt.Errorf("%w: %v", looppkg.ErrValidation, err)
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	slices.Sort(names)
	normalized.LoopNames = names
	return normalized, nil
}

var _ looppkg.CatalogRunReader = (*LoopRepo)(nil)

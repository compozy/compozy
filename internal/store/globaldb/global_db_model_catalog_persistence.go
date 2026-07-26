package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func upsertModelCatalogSourceStatus(
	ctx context.Context,
	exec modelCatalogSQLExecutor,
	status modelcatalog.SourceStatus,
) error {
	if err := sqlcgen.New(exec).UpsertModelCatalogSourceStatus(ctx, sqlcgen.UpsertModelCatalogSourceStatusParams{
		SourceID: status.SourceID, ProviderID: status.ProviderID, SourceKind: string(status.SourceKind),
		Priority: int64(status.Priority), RefreshState: string(status.RefreshState),
		LastRefreshAt: store.FormatNullableTimestamp(status.LastRefresh),
		NextRefreshAt: store.FormatNullableTimestamp(status.NextRefresh),
		LastSuccessAt: store.FormatNullableTimestamp(status.LastSuccess), LastError: status.LastError,
		RowCount: int64(status.RowCount), Stale: int64(boolToSQLiteInt(status.Stale)),
	}); err != nil {
		return fmt.Errorf("store: upsert model catalog source status: %w", err)
	}
	return nil
}

func insertModelCatalogRow(ctx context.Context, exec modelCatalogSQLExecutor, row modelcatalog.ModelRow) error {
	if err := sqlcgen.New(exec).InsertModelCatalogRow(ctx, modelCatalogRowParams(row)); err != nil {
		return fmt.Errorf(
			"store: insert model catalog row %q/%q/%q: %w",
			row.SourceID,
			row.ProviderID,
			row.ModelID,
			err,
		)
	}
	return nil
}

func insertModelCatalogReasoningEfforts(
	ctx context.Context,
	exec modelCatalogSQLExecutor,
	row modelcatalog.ModelRow,
) error {
	for rank, effort := range row.ReasoningEfforts {
		if err := sqlcgen.New(exec).InsertModelCatalogReasoningEffort(
			ctx,
			sqlcgen.InsertModelCatalogReasoningEffortParams{
				SourceID: row.SourceID, ProviderID: row.ProviderID, ModelID: row.ModelID,
				Effort: string(effort), Rank: int64(rank),
			},
		); err != nil {
			return fmt.Errorf("store: insert model catalog reasoning effort %q: %w", effort, err)
		}
	}
	return nil
}

func listModelCatalogReasoningEfforts(
	ctx context.Context,
	exec modelCatalogSQLExecutor,
	opts modelcatalog.ListOptions,
) (efforts map[modelCatalogRowKey][]modelcatalog.ReasoningEffort, err error) {
	// dynamic-sql: this join reuses the optional model-row filter set selected at runtime.
	sqlQuery := `SELECT
			e.source_id,
			e.provider_id,
			e.model_id,
			e.effort
		FROM model_catalog_reasoning_efforts e
		JOIN model_catalog_rows r
			ON r.source_id = e.source_id
			AND r.provider_id = e.provider_id
			AND r.model_id = e.model_id`
	where, args := modelCatalogRowFilterClauses(opts, "r")
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += ` ORDER BY e.source_id ASC, e.provider_id ASC, e.model_id ASC, e.rank ASC, e.effort ASC`

	rows, err := exec.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query model catalog reasoning efforts: %w", err)
	}
	defer func() {
		err = joinRowsCloseError(
			rows,
			err,
			"model catalog reasoning effort query",
		)
	}()

	efforts = make(map[modelCatalogRowKey][]modelcatalog.ReasoningEffort)
	for rows.Next() {
		var (
			sourceID   string
			providerID string
			modelID    string
			effort     string
		)
		if err := rows.Scan(&sourceID, &providerID, &modelID, &effort); err != nil {
			return nil, fmt.Errorf("store: scan model catalog reasoning effort: %w", err)
		}
		key := modelCatalogKey(sourceID, providerID, modelID)
		efforts[key] = append(efforts[key], modelcatalog.ReasoningEffort(effort))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate model catalog reasoning efforts: %w", err)
	}
	return efforts, nil
}

func scanModelCatalogRow(scanner interface{ Scan(dest ...any) error }) (modelcatalog.ModelRow, error) {
	scan := modelCatalogRowScan{}
	if err := scanner.Scan(scan.destinations()...); err != nil {
		return modelcatalog.ModelRow{}, fmt.Errorf("store: scan model catalog row: %w", err)
	}
	return scan.modelRow()
}

func scanModelCatalogSourceStatus(scanner interface{ Scan(dest ...any) error }) (modelcatalog.SourceStatus, error) {
	var (
		status         modelcatalog.SourceStatus
		sourceKind     string
		lastRefreshRaw string
		nextRefreshRaw string
		lastSuccessRaw string
		staleRaw       int
	)
	if err := scanner.Scan(
		&status.SourceID,
		&status.ProviderID,
		&sourceKind,
		&status.Priority,
		&status.RefreshState,
		&lastRefreshRaw,
		&nextRefreshRaw,
		&lastSuccessRaw,
		&status.LastError,
		&status.RowCount,
		&staleRaw,
	); err != nil {
		return modelcatalog.SourceStatus{}, fmt.Errorf("store: scan model catalog source status: %w", err)
	}
	var err error
	status.SourceKind = modelcatalog.SourceKind(sourceKind)
	status.RefreshState = modelcatalog.RefreshState(strings.TrimSpace(string(status.RefreshState)))
	if status.LastRefresh, err = parseOptionalModelCatalogTimestamp(lastRefreshRaw, "last_refresh_at"); err != nil {
		return modelcatalog.SourceStatus{}, err
	}
	if status.NextRefresh, err = parseOptionalModelCatalogTimestamp(nextRefreshRaw, "next_refresh_at"); err != nil {
		return modelcatalog.SourceStatus{}, err
	}
	if status.LastSuccess, err = parseOptionalModelCatalogTimestamp(lastSuccessRaw, "last_success_at"); err != nil {
		return modelcatalog.SourceStatus{}, err
	}
	status.Stale = staleRaw != 0
	return status, nil
}

func modelCatalogRowFilterClauses(opts modelcatalog.ListOptions, alias string) ([]string, []any) {
	column := func(name string) string {
		if strings.TrimSpace(alias) == "" {
			return name
		}
		return strings.TrimSpace(alias) + "." + name
	}

	where := make([]string, 0, 3)
	args := make([]any, 0, 2)
	if providerID := strings.TrimSpace(opts.ProviderID); providerID != "" {
		where = append(where, column("provider_id")+" = ?")
		args = append(args, providerID)
	}
	if sourceID := strings.TrimSpace(opts.SourceID); sourceID != "" {
		where = append(where, column("source_id")+" = ?")
		args = append(args, sourceID)
	}
	if !opts.IncludeStale && !opts.IncludeAll {
		where = append(where, column("stale")+" = 0")
	}
	return where, args
}

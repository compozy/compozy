package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"

	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

var _ modelcatalog.Store = (*ModelCatalogRepo)(nil)

type modelCatalogSQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type modelCatalogRowKey struct {
	sourceID   string
	providerID string
	modelID    string
}

// ReplaceSourceRows atomically replaces all model rows and status for one provider-scoped source.
func (g *ModelCatalogRepo) ReplaceSourceRows(
	ctx context.Context,
	sourceID string,
	providerID string,
	rows []modelcatalog.ModelRow,
	status modelcatalog.SourceStatus,
) error {
	if err := g.checkReady(ctx, "replace model catalog source rows"); err != nil {
		return err
	}
	normalizedRows, normalizedStatus, err := normalizeModelCatalogReplacement(sourceID, providerID, rows, status)
	if err != nil {
		return err
	}

	return g.withModelCatalogImmediateTransaction(
		ctx,
		"model catalog source replacement",
		func(exec modelCatalogSQLExecutor) error {
			queries := sqlcgen.New(exec)
			if err := upsertModelCatalogSourceStatus(ctx, exec, normalizedStatus); err != nil {
				return err
			}
			deleteParams := sqlcgen.DeleteModelCatalogReasoningEffortsParams{
				SourceID: normalizedStatus.SourceID, ProviderID: normalizedStatus.ProviderID,
			}
			if err := queries.DeleteModelCatalogReasoningEfforts(ctx, deleteParams); err != nil {
				return fmt.Errorf("store: delete model catalog reasoning efforts: %w", err)
			}
			if err := queries.DeleteModelCatalogRows(
				ctx,
				sqlcgen.DeleteModelCatalogRowsParams(deleteParams),
			); err != nil {
				return fmt.Errorf("store: delete model catalog source rows: %w", err)
			}
			for _, row := range normalizedRows {
				if err := insertModelCatalogRow(ctx, exec, row); err != nil {
					return err
				}
				if err := insertModelCatalogReasoningEfforts(ctx, exec, row); err != nil {
					return err
				}
			}
			return nil
		},
	)
}

// ListRows returns deterministic catalog source rows matching the query.
func (g *ModelCatalogRepo) ListRows(
	ctx context.Context,
	opts modelcatalog.ListOptions,
) (catalogRows []modelcatalog.ModelRow, err error) {
	if err := g.checkReady(ctx, "list model catalog rows"); err != nil {
		return nil, err
	}
	if err := g.withModelCatalogReadTransaction(
		ctx,
		"list model catalog rows",
		func(exec modelCatalogSQLExecutor) error {
			var listErr error
			catalogRows, listErr = listModelCatalogRows(ctx, exec, opts)
			return listErr
		},
	); err != nil {
		return nil, err
	}
	return catalogRows, nil
}

func listModelCatalogRows(
	ctx context.Context,
	exec modelCatalogSQLExecutor,
	opts modelcatalog.ListOptions,
) (catalogRows []modelcatalog.ModelRow, err error) {
	// dynamic-sql: provider/source/availability filters are optional and shared with the reasoning-effort join.
	sqlQuery := `SELECT
			source_id,
			provider_id,
			model_id,
			source_kind,
			priority,
			available,
			stale,
			refreshed_at,
			expires_at,
			display_name,
			context_window,
			max_input_tokens,
			max_output_tokens,
			supports_tools,
			supports_reasoning,
			default_reasoning_effort,
			cost_input_per_million,
			cost_output_per_million,
			cost_cache_read_per_million,
			cost_cache_write_per_million,
			cost_reasoning_per_million,
			explicitly_curated,
			deprecated,
			hidden,
			featured,
			deprecated_set,
			hidden_set,
			featured_set,
			release_date,
			last_error
		FROM model_catalog_rows`
	where, args := modelCatalogRowFilterClauses(opts, "")
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += ` ORDER BY provider_id ASC, model_id ASC, priority DESC, refreshed_at DESC, source_id ASC`

	rows, err := exec.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query model catalog rows: %w", err)
	}
	defer func() {
		if closeErr := joinRowsCloseError(rows, nil, "model catalog row query"); closeErr != nil {
			joinCleanupError(&err, closeErr)
		}
	}()

	catalogRows = make([]modelcatalog.ModelRow, 0)
	for rows.Next() {
		row, scanErr := scanModelCatalogRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		catalogRows = append(catalogRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate model catalog rows: %w", err)
	}

	efforts, err := listModelCatalogReasoningEfforts(ctx, exec, opts)
	if err != nil {
		return nil, err
	}
	for index := range catalogRows {
		key := modelCatalogKey(catalogRows[index].SourceID, catalogRows[index].ProviderID, catalogRows[index].ModelID)
		catalogRows[index].ReasoningEfforts = efforts[key]
	}
	return catalogRows, nil
}

// ListSourceStatus returns provider-scoped source status rows.
func (g *ModelCatalogRepo) ListSourceStatus(
	ctx context.Context,
	providerID string,
) (statuses []modelcatalog.SourceStatus, err error) {
	if err := g.checkReady(ctx, "list model catalog source status"); err != nil {
		return nil, err
	}
	sqlQuery := `SELECT
			source_id,
			provider_id,
			source_kind,
			priority,
			refresh_state,
			last_refresh_at,
			next_refresh_at,
			last_success_at,
			last_error,
			row_count,
			stale
		FROM model_catalog_sources`
	where, args := store.BuildClauses(store.StringClause("provider_id", providerID))
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += ` ORDER BY provider_id ASC, source_id ASC`

	// dynamic-sql: the provider filter is optional while stable ordering is preserved.
	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query model catalog source status: %w", err)
	}
	defer func() {
		if closeErr := joinRowsCloseError(
			rows,
			nil,
			"model catalog source status query",
		); closeErr != nil {
			joinCleanupError(&err, closeErr)
		}
	}()

	statuses = make([]modelcatalog.SourceStatus, 0)
	for rows.Next() {
		status, scanErr := scanModelCatalogSourceStatus(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		statuses = append(statuses, status)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate model catalog source status: %w", err)
	}
	return statuses, nil
}

func normalizeModelCatalogReplacement(
	sourceID string,
	providerID string,
	rows []modelcatalog.ModelRow,
	status modelcatalog.SourceStatus,
) ([]modelcatalog.ModelRow, modelcatalog.SourceStatus, error) {
	trimmedSourceID, err := requireModelCatalogValue(sourceID, "source id")
	if err != nil {
		return nil, modelcatalog.SourceStatus{}, err
	}
	trimmedProviderID, err := requireModelCatalogValue(providerID, "provider id")
	if err != nil {
		return nil, modelcatalog.SourceStatus{}, err
	}
	normalizedStatus, err := normalizeModelCatalogStatus(trimmedSourceID, trimmedProviderID, status)
	if err != nil {
		return nil, modelcatalog.SourceStatus{}, err
	}

	normalizedRows := make([]modelcatalog.ModelRow, 0, len(rows))
	for index, row := range rows {
		normalizedRow, err := normalizeModelCatalogRow(trimmedSourceID, trimmedProviderID, normalizedStatus, row)
		if err != nil {
			return nil, modelcatalog.SourceStatus{}, fmt.Errorf("store: normalize model catalog row %d: %w", index, err)
		}
		if normalizedStatus.Priority == 0 {
			normalizedStatus.Priority = normalizedRow.Priority
		}
		if normalizedRow.Priority != normalizedStatus.Priority {
			return nil, modelcatalog.SourceStatus{}, fmt.Errorf(
				"store: model catalog row %q priority %d does not match source priority %d",
				normalizedRow.ModelID,
				normalizedRow.Priority,
				normalizedStatus.Priority,
			)
		}
		normalizedRows = append(normalizedRows, normalizedRow)
	}
	normalizedStatus.RowCount = len(normalizedRows)
	return normalizedRows, normalizedStatus, nil
}

func normalizeModelCatalogStatus(
	sourceID string,
	providerID string,
	status modelcatalog.SourceStatus,
) (modelcatalog.SourceStatus, error) {
	normalized := status
	if normalized.SourceID = strings.TrimSpace(normalized.SourceID); normalized.SourceID == "" {
		normalized.SourceID = sourceID
	}
	if normalized.ProviderID = strings.TrimSpace(normalized.ProviderID); normalized.ProviderID == "" {
		normalized.ProviderID = providerID
	}
	if normalized.SourceID != sourceID {
		return modelcatalog.SourceStatus{}, fmt.Errorf(
			"store: model catalog status source id %q does not match %q",
			normalized.SourceID,
			sourceID,
		)
	}
	if normalized.ProviderID != providerID {
		return modelcatalog.SourceStatus{}, fmt.Errorf(
			"store: model catalog status provider id %q does not match %q",
			normalized.ProviderID,
			providerID,
		)
	}
	normalized.SourceKind = modelcatalog.SourceKind(strings.TrimSpace(string(normalized.SourceKind)))
	if normalized.SourceKind == "" {
		return modelcatalog.SourceStatus{}, fmt.Errorf("store: model catalog status source kind is required")
	}
	if err := modelcatalog.ValidateSourceIdentity(normalized.SourceID, normalized.SourceKind); err != nil {
		return modelcatalog.SourceStatus{}, fmt.Errorf("store: validate model catalog source identity: %w", err)
	}
	normalized.RefreshState = modelcatalog.RefreshState(strings.TrimSpace(string(normalized.RefreshState)))
	if normalized.RefreshState == "" {
		normalized.RefreshState = modelcatalog.RefreshStateIdle
	}
	normalized.LastError = strings.TrimSpace(normalized.LastError)
	if normalized.RowCount < 0 {
		return modelcatalog.SourceStatus{}, fmt.Errorf(
			"store: model catalog status row count %d is invalid",
			normalized.RowCount,
		)
	}
	return normalized, nil
}

func normalizeModelCatalogRow(
	sourceID string,
	providerID string,
	status modelcatalog.SourceStatus,
	row modelcatalog.ModelRow,
) (modelcatalog.ModelRow, error) {
	normalized := row
	if normalized.SourceID = strings.TrimSpace(normalized.SourceID); normalized.SourceID == "" {
		normalized.SourceID = sourceID
	}
	if normalized.ProviderID = strings.TrimSpace(normalized.ProviderID); normalized.ProviderID == "" {
		normalized.ProviderID = providerID
	}
	if normalized.SourceID != sourceID {
		return modelcatalog.ModelRow{}, fmt.Errorf(
			"source id %q does not match %q",
			normalized.SourceID,
			sourceID,
		)
	}
	if normalized.ProviderID != providerID {
		return modelcatalog.ModelRow{}, fmt.Errorf(
			"provider id %q does not match %q",
			normalized.ProviderID,
			providerID,
		)
	}
	modelID, err := requireModelCatalogValue(normalized.ModelID, "model id")
	if err != nil {
		return modelcatalog.ModelRow{}, err
	}
	normalized.ModelID = modelID
	normalized.SourceKind = modelcatalog.SourceKind(strings.TrimSpace(string(normalized.SourceKind)))
	if normalized.SourceKind == "" {
		normalized.SourceKind = status.SourceKind
	}
	if normalized.SourceKind != status.SourceKind {
		return modelcatalog.ModelRow{}, fmt.Errorf(
			"source kind %q does not match status source kind %q",
			normalized.SourceKind,
			status.SourceKind,
		)
	}
	normalized.DisplayName = strings.TrimSpace(normalized.DisplayName)
	normalized.LastError = strings.TrimSpace(normalized.LastError)
	if err := validateModelCatalogRates(normalized); err != nil {
		return modelcatalog.ModelRow{}, err
	}
	if normalized.ReleaseDate != nil {
		releaseDate, err := modelcatalog.NormalizeReleaseDate(*normalized.ReleaseDate)
		if err != nil {
			return modelcatalog.ModelRow{}, err
		}
		normalized.ReleaseDate = releaseDate
	}
	if normalized.DefaultReasoningEffort != nil {
		effort := modelcatalog.ReasoningEffort(strings.TrimSpace(string(*normalized.DefaultReasoningEffort)))
		switch {
		case effort == "":
			normalized.DefaultReasoningEffort = nil
		case !modelcatalog.IsValidEffort(string(effort)):
			return modelcatalog.ModelRow{}, fmt.Errorf("default reasoning effort %q is unsupported", effort)
		default:
			normalized.DefaultReasoningEffort = &effort
		}
	}
	for index, effort := range normalized.ReasoningEfforts {
		trimmed := modelcatalog.ReasoningEffort(strings.TrimSpace(string(effort)))
		if trimmed == "" {
			return modelcatalog.ModelRow{}, fmt.Errorf("reasoning effort %d is required", index)
		}
		if !modelcatalog.IsValidEffort(string(trimmed)) {
			return modelcatalog.ModelRow{}, fmt.Errorf("reasoning effort %d value %q is unsupported", index, trimmed)
		}
		normalized.ReasoningEfforts[index] = trimmed
	}
	return normalized, nil
}

func validateModelCatalogRates(row modelcatalog.ModelRow) error {
	for _, rate := range []struct {
		name  string
		value *float64
	}{
		{name: "cost input per million", value: row.CostInputPerMillion},
		{name: "cost output per million", value: row.CostOutputPerMillion},
		{name: "cost cache read per million", value: row.CostCacheReadPerMillion},
		{name: "cost cache write per million", value: row.CostCacheWritePerMillion},
		{name: "cost reasoning per million", value: row.CostReasoningPerMillion},
	} {
		if rate.value != nil && (math.IsNaN(*rate.value) || math.IsInf(*rate.value, 0) || *rate.value < 0) {
			return fmt.Errorf("%s must be finite and non-negative", rate.name)
		}
	}
	return nil
}

func requireModelCatalogValue(value string, field string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("store: model catalog %s is required", field)
	}
	return trimmed, nil
}

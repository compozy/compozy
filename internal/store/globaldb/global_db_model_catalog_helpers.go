package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

type modelCatalogRowScan struct {
	row                      modelcatalog.ModelRow
	sourceKind               string
	available                sql.NullInt64
	stale                    int
	refreshedAt              string
	expiresAt                string
	contextWindow            sql.NullInt64
	maxInputTokens           sql.NullInt64
	maxOutputTokens          sql.NullInt64
	supportsTools            sql.NullInt64
	supportsReasoning        sql.NullInt64
	defaultReasoningEffort   sql.NullString
	costInputPerMillion      sql.NullFloat64
	costOutputPerMillion     sql.NullFloat64
	costCacheReadPerMillion  sql.NullFloat64
	costCacheWritePerMillion sql.NullFloat64
	costReasoningPerMillion  sql.NullFloat64
	explicitlyCurated        int
	deprecated               int
	hidden                   int
	featured                 int
	deprecatedSet            int
	hiddenSet                int
	featuredSet              int
	releaseDate              sql.NullString
}

func (s *modelCatalogRowScan) destinations() []any {
	return []any{
		&s.row.SourceID,
		&s.row.ProviderID,
		&s.row.ModelID,
		&s.sourceKind,
		&s.row.Priority,
		&s.available,
		&s.stale,
		&s.refreshedAt,
		&s.expiresAt,
		&s.row.DisplayName,
		&s.contextWindow,
		&s.maxInputTokens,
		&s.maxOutputTokens,
		&s.supportsTools,
		&s.supportsReasoning,
		&s.defaultReasoningEffort,
		&s.costInputPerMillion,
		&s.costOutputPerMillion,
		&s.costCacheReadPerMillion,
		&s.costCacheWritePerMillion,
		&s.costReasoningPerMillion,
		&s.explicitlyCurated,
		&s.deprecated,
		&s.hidden,
		&s.featured,
		&s.deprecatedSet,
		&s.hiddenSet,
		&s.featuredSet,
		&s.releaseDate,
		&s.row.LastError,
	}
}

func (s *modelCatalogRowScan) modelRow() (modelcatalog.ModelRow, error) {
	row := s.row
	row.SourceKind = modelcatalog.SourceKind(s.sourceKind)
	var err error
	if row.Available, err = nullableSQLiteIntToBool(s.available, "available"); err != nil {
		return modelcatalog.ModelRow{}, err
	}
	row.Stale = s.stale != 0
	if row.RefreshedAt, err = parseOptionalModelCatalogTimestamp(s.refreshedAt, "refreshed_at"); err != nil {
		return modelcatalog.ModelRow{}, err
	}
	if row.ExpiresAt, err = parseOptionalModelCatalogTimestamp(s.expiresAt, "expires_at"); err != nil {
		return modelcatalog.ModelRow{}, err
	}
	row.ContextWindow = store.NullInt64(s.contextWindow)
	row.MaxInputTokens = store.NullInt64(s.maxInputTokens)
	row.MaxOutputTokens = store.NullInt64(s.maxOutputTokens)
	if row.SupportsTools, err = nullableSQLiteIntToBool(s.supportsTools, "supports_tools"); err != nil {
		return modelcatalog.ModelRow{}, err
	}
	if row.SupportsReasoning, err = nullableSQLiteIntToBool(s.supportsReasoning, "supports_reasoning"); err != nil {
		return modelcatalog.ModelRow{}, err
	}
	row.DefaultReasoningEffort = nullReasoningEffort(s.defaultReasoningEffort)
	row.CostInputPerMillion = store.NullFloat64(s.costInputPerMillion)
	row.CostOutputPerMillion = store.NullFloat64(s.costOutputPerMillion)
	row.CostCacheReadPerMillion = store.NullFloat64(s.costCacheReadPerMillion)
	row.CostCacheWritePerMillion = store.NullFloat64(s.costCacheWritePerMillion)
	row.CostReasoningPerMillion = store.NullFloat64(s.costReasoningPerMillion)
	row.ExplicitlyCurated = s.explicitlyCurated != 0
	if row.Deprecated, err = sqliteBoolWithPresence(s.deprecated, s.deprecatedSet, "deprecated"); err != nil {
		return modelcatalog.ModelRow{}, err
	}
	if row.Hidden, err = sqliteBoolWithPresence(s.hidden, s.hiddenSet, "hidden"); err != nil {
		return modelcatalog.ModelRow{}, err
	}
	if row.Featured, err = sqliteBoolWithPresence(s.featured, s.featuredSet, "featured"); err != nil {
		return modelcatalog.ModelRow{}, err
	}
	if s.releaseDate.Valid {
		row.ReleaseDate = &s.releaseDate.String
	}
	return row, nil
}

func (g *ModelCatalogRepo) withModelCatalogImmediateTransaction(
	ctx context.Context,
	action string,
	run func(exec modelCatalogSQLExecutor) error,
) error {
	if err := store.ExecuteWrite(ctx, g.db, func(_ context.Context, tx *store.WriteTx) error {
		return run(tx)
	}); err != nil {
		return fmt.Errorf("store: %s transaction: %w", action, err)
	}
	return nil
}

func (g *ModelCatalogRepo) withModelCatalogReadTransaction(
	ctx context.Context,
	action string,
	run func(exec modelCatalogSQLExecutor) error,
) (err error) {
	conn, err := g.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("store: open connection for %s: %w", action, err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close %s transaction connection: %w", action, closeErr))
		}
	}()

	tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("store: begin %s transaction: %w", action, err)
	}

	finished := false
	defer func() {
		if !finished {
			if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
				joinCleanupError(&err, fmt.Errorf("store: rollback %s transaction: %w", action, rollbackErr))
			}
		}
	}()

	if err := run(tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("store: commit %s transaction: %w", action, err)
	}
	finished = true
	return nil
}

func modelCatalogKey(sourceID string, providerID string, modelID string) modelCatalogRowKey {
	return modelCatalogRowKey{sourceID: sourceID, providerID: providerID, modelID: modelID}
}

func boolToSQLiteInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nullableBoolToSQLiteInt(value *bool) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(boolToSQLiteInt(*value)), Valid: true}
}

func boolPointerToSQLiteInt(value *bool) int {
	return boolToSQLiteInt(value != nil && *value)
}

func sqliteBoolWithPresence(value int, present int, field string) (*bool, error) {
	switch present {
	case 0:
		return nil, nil
	case 1:
	default:
		return nil, fmt.Errorf("store: model catalog %s presence value %d is invalid", field, present)
	}
	switch value {
	case 0:
		converted := false
		return &converted, nil
	case 1:
		converted := true
		return &converted, nil
	default:
		return nil, fmt.Errorf("store: model catalog %s boolean value %d is invalid", field, value)
	}
}

func nullableSQLiteIntToBool(value sql.NullInt64, field string) (*bool, error) {
	if !value.Valid {
		return nil, nil
	}
	switch value.Int64 {
	case 0:
		converted := false
		return &converted, nil
	case 1:
		converted := true
		return &converted, nil
	default:
		return nil, fmt.Errorf("store: model catalog %s boolean value %d is invalid", field, value.Int64)
	}
}

func nullableReasoningEffort(value *modelcatalog.ReasoningEffort) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	trimmed := strings.TrimSpace(string(*value))
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}

func nullableStringPtr(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}

func nullableModelCatalogInt64(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

func nullableModelCatalogFloat64(value *float64) sql.NullFloat64 {
	if value == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *value, Valid: true}
}

func modelCatalogRowParams(row modelcatalog.ModelRow) sqlcgen.InsertModelCatalogRowParams {
	return sqlcgen.InsertModelCatalogRowParams{
		SourceID:   row.SourceID,
		ProviderID: row.ProviderID,
		ModelID:    row.ModelID,
		SourceKind: string(
			row.SourceKind,
		),
		Priority:    int64(row.Priority),
		Available:   nullableBoolToSQLiteInt(row.Available),
		Stale:       int64(boolToSQLiteInt(row.Stale)),
		RefreshedAt: store.FormatNullableTimestamp(row.RefreshedAt),
		ExpiresAt:   store.FormatNullableTimestamp(row.ExpiresAt),
		DisplayName: row.DisplayName,
		ContextWindow: nullableModelCatalogInt64(
			row.ContextWindow,
		),
		MaxInputTokens: nullableModelCatalogInt64(row.MaxInputTokens),
		MaxOutputTokens: nullableModelCatalogInt64(
			row.MaxOutputTokens,
		),
		SupportsTools:            nullableBoolToSQLiteInt(row.SupportsTools),
		SupportsReasoning:        nullableBoolToSQLiteInt(row.SupportsReasoning),
		DefaultReasoningEffort:   nullableReasoningEffort(row.DefaultReasoningEffort),
		CostInputPerMillion:      nullableModelCatalogFloat64(row.CostInputPerMillion),
		CostOutputPerMillion:     nullableModelCatalogFloat64(row.CostOutputPerMillion),
		CostCacheReadPerMillion:  nullableModelCatalogFloat64(row.CostCacheReadPerMillion),
		CostCacheWritePerMillion: nullableModelCatalogFloat64(row.CostCacheWritePerMillion),
		CostReasoningPerMillion:  nullableModelCatalogFloat64(row.CostReasoningPerMillion),
		ExplicitlyCurated:        int64(boolToSQLiteInt(row.ExplicitlyCurated)),
		Deprecated: int64(
			boolPointerToSQLiteInt(row.Deprecated),
		),
		Hidden: int64(boolPointerToSQLiteInt(row.Hidden)),
		Featured: int64(
			boolPointerToSQLiteInt(row.Featured),
		),
		DeprecatedSet: int64(boolToSQLiteInt(row.Deprecated != nil)),
		HiddenSet:     int64(boolToSQLiteInt(row.Hidden != nil)),
		FeaturedSet:   int64(boolToSQLiteInt(row.Featured != nil)),
		ReleaseDate:   nullableStringPtr(row.ReleaseDate),
		LastError:     row.LastError,
	}
}

func nullReasoningEffort(value sql.NullString) *modelcatalog.ReasoningEffort {
	if !value.Valid {
		return nil
	}
	trimmed := strings.TrimSpace(value.String)
	if trimmed == "" {
		return nil
	}
	effort := modelcatalog.ReasoningEffort(trimmed)
	return &effort
}

func parseOptionalModelCatalogTimestamp(value string, field string) (time.Time, error) {
	parsed, err := store.ParseNullableTimestamp(value)
	if err != nil {
		return time.Time{}, fmt.Errorf("store: parse model catalog %s: %w", field, err)
	}
	if parsed == nil {
		return time.Time{}, nil
	}
	return *parsed, nil
}

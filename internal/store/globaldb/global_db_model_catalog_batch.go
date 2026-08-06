package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

type normalizedModelCatalogReplacement struct {
	rows   []modelcatalog.ModelRow
	status modelcatalog.SourceStatus
}

// ReplaceSourceRowsBatch publishes a complete model catalog generation in one transaction.
func (g *ModelCatalogRepo) ReplaceSourceRowsBatch(
	ctx context.Context,
	replacements []modelcatalog.SourceRowsReplacement,
) error {
	if err := g.checkReady(ctx, "replace model catalog source rows batch"); err != nil {
		return err
	}
	if len(replacements) == 0 {
		return nil
	}

	normalized := make([]normalizedModelCatalogReplacement, 0, len(replacements))
	seen := make(map[string]struct{}, len(replacements))
	for index, replacement := range replacements {
		rows, status, err := normalizeModelCatalogReplacement(
			replacement.SourceID,
			replacement.ProviderID,
			replacement.Rows,
			replacement.Status,
		)
		if err != nil {
			return fmt.Errorf("store: normalize model catalog replacement %d: %w", index, err)
		}
		key := modelCatalogReplacementKey(status.SourceID, status.ProviderID)
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf(
				"store: model catalog replacement %q/%q is duplicated",
				status.SourceID,
				status.ProviderID,
			)
		}
		seen[key] = struct{}{}
		normalized = append(normalized, normalizedModelCatalogReplacement{rows: rows, status: status})
	}

	return g.withModelCatalogImmediateTransaction(
		ctx,
		"model catalog source replacement batch",
		func(exec modelCatalogSQLExecutor) error {
			for _, replacement := range normalized {
				if err := replaceModelCatalogSourceRows(
					ctx,
					exec,
					replacement.rows,
					replacement.status,
				); err != nil {
					return err
				}
			}
			return nil
		},
	)
}

func replaceModelCatalogSourceRows(
	ctx context.Context,
	exec modelCatalogSQLExecutor,
	rows []modelcatalog.ModelRow,
	status modelcatalog.SourceStatus,
) error {
	queries := sqlcgen.New(exec)
	if err := upsertModelCatalogSourceStatus(ctx, exec, status); err != nil {
		return err
	}
	deleteParams := sqlcgen.DeleteModelCatalogReasoningEffortsParams{
		SourceID:   status.SourceID,
		ProviderID: status.ProviderID,
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
	for _, row := range rows {
		if err := insertModelCatalogRow(ctx, exec, row); err != nil {
			return err
		}
		if err := insertModelCatalogReasoningEfforts(ctx, exec, row); err != nil {
			return err
		}
	}
	return nil
}

func modelCatalogReplacementKey(sourceID string, providerID string) string {
	return strings.TrimSpace(sourceID) + "\x00" + strings.TrimSpace(providerID)
}

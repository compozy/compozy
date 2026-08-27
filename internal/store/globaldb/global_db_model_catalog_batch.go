package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

type normalizedModelCatalogReplacement struct {
	executionContext modelcatalog.CatalogExecutionContext
	rows             []modelcatalog.ModelRow
	status           modelcatalog.SourceStatus
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
		executionContext, rows, status, err := normalizeModelCatalogReplacement(
			replacement.ExecutionContext,
			replacement.SourceID,
			replacement.ProviderID,
			replacement.Rows,
			replacement.Status,
		)
		if err != nil {
			return fmt.Errorf("store: normalize model catalog replacement %d: %w", index, err)
		}
		key, err := modelCatalogReplacementKey(executionContext, status.SourceID, status.ProviderID)
		if err != nil {
			return err
		}
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf(
				"store: model catalog replacement %q/%q is duplicated",
				status.SourceID,
				status.ProviderID,
			)
		}
		seen[key] = struct{}{}
		normalized = append(normalized, normalizedModelCatalogReplacement{
			executionContext: executionContext,
			rows:             rows,
			status:           status,
		})
	}

	return g.withModelCatalogImmediateTransaction(
		ctx,
		"model catalog source replacement batch",
		func(exec modelCatalogSQLExecutor) error {
			for _, replacement := range normalized {
				if err := replaceModelCatalogSourceRows(
					ctx,
					exec,
					replacement.executionContext,
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
	executionContext modelcatalog.CatalogExecutionContext,
	rows []modelcatalog.ModelRow,
	status modelcatalog.SourceStatus,
) error {
	queries := sqlcgen.New(exec)
	contextID, err := upsertModelCatalogExecutionContext(ctx, exec, executionContext)
	if err != nil {
		return err
	}
	if err := pruneSupersededModelCatalogSourceContexts(
		ctx,
		exec,
		executionContext,
		contextID,
		status.SourceID,
		status.ProviderID,
	); err != nil {
		return err
	}
	if err := upsertModelCatalogSourceStatus(ctx, exec, contextID, status); err != nil {
		return err
	}
	deleteParams := sqlcgen.DeleteModelCatalogReasoningEffortsParams{
		ContextID:  contextID,
		SourceID:   status.SourceID,
		ProviderID: status.ProviderID,
	}
	if err := queries.DeleteModelCatalogReasoningEfforts(ctx, deleteParams); err != nil {
		return fmt.Errorf("store: delete model catalog reasoning efforts: %w", err)
	}
	if err := queries.DeleteModelCatalogTransportBindingSelections(
		ctx,
		sqlcgen.DeleteModelCatalogTransportBindingSelectionsParams(deleteParams),
	); err != nil {
		return fmt.Errorf("store: delete model catalog binding options: %w", err)
	}
	if err := queries.DeleteModelCatalogTransportBindings(
		ctx,
		sqlcgen.DeleteModelCatalogTransportBindingsParams(deleteParams),
	); err != nil {
		return fmt.Errorf("store: delete model catalog transport bindings: %w", err)
	}
	if err := queries.DeleteModelCatalogOptionValues(
		ctx,
		sqlcgen.DeleteModelCatalogOptionValuesParams(deleteParams),
	); err != nil {
		return fmt.Errorf("store: delete model catalog option values: %w", err)
	}
	if err := queries.DeleteModelCatalogOptions(
		ctx,
		sqlcgen.DeleteModelCatalogOptionsParams(deleteParams),
	); err != nil {
		return fmt.Errorf("store: delete model catalog options: %w", err)
	}
	if err := queries.DeleteModelCatalogRows(
		ctx,
		sqlcgen.DeleteModelCatalogRowsParams(deleteParams),
	); err != nil {
		return fmt.Errorf("store: delete model catalog source rows: %w", err)
	}
	for _, row := range rows {
		if err := insertModelCatalogRow(ctx, exec, contextID, row); err != nil {
			return err
		}
		if err := insertModelCatalogOptions(ctx, exec, contextID, row); err != nil {
			return err
		}
		if err := insertModelCatalogReasoningEfforts(ctx, exec, contextID, row); err != nil {
			return err
		}
		if err := insertModelCatalogTransportBindings(ctx, exec, contextID, row); err != nil {
			return err
		}
		if err := insertModelCatalogBindingSelections(ctx, exec, contextID, row); err != nil {
			return err
		}
	}
	return deleteUnusedModelCatalogExecutionContexts(ctx, exec)
}

func modelCatalogReplacementKey(
	executionContext modelcatalog.CatalogExecutionContext,
	sourceID string,
	providerID string,
) (string, error) {
	contextID, err := executionContext.ID()
	if err != nil {
		return "", err
	}
	return contextID + "\x00" + strings.TrimSpace(sourceID) + "\x00" + strings.TrimSpace(providerID), nil
}

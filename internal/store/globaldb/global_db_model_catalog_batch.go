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
	removeSource     bool
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
		entry, err := normalizeModelCatalogBatchReplacement(replacement)
		if err != nil {
			return fmt.Errorf("store: normalize model catalog replacement %d: %w", index, err)
		}
		key, err := modelCatalogReplacementKey(entry.executionContext, entry.status.SourceID, entry.status.ProviderID)
		if err != nil {
			return err
		}
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf(
				"store: model catalog replacement %q/%q is duplicated",
				entry.status.SourceID,
				entry.status.ProviderID,
			)
		}
		seen[key] = struct{}{}
		normalized = append(normalized, entry)
	}

	return g.withModelCatalogImmediateTransaction(
		ctx,
		"model catalog source replacement batch",
		func(exec modelCatalogSQLExecutor) error {
			for _, replacement := range normalized {
				if replacement.removeSource {
					if err := deleteModelCatalogSource(ctx, exec, replacement.status); err != nil {
						return err
					}
					continue
				}
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
	// OpenCode partitions must survive installation switches for same-invocation stale fallback.
	if len(rows) > 0 && status.SourceID != modelcatalog.SourceKindProviderLiveID("opencode") {
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
	}
	if err := upsertModelCatalogSourceStatus(ctx, exec, contextID, status); err != nil {
		return err
	}
	unchanged, err := touchUnchangedModelCatalogSourceRows(ctx, exec, executionContext, contextID, rows, status)
	if err != nil {
		return err
	}
	if unchanged {
		return deleteUnusedModelCatalogExecutionContexts(ctx, exec)
	}
	if err := deleteModelCatalogSourceRowData(ctx, queries, contextID, status); err != nil {
		return err
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

// deleteModelCatalogSourceRowData removes every row and child record for one source/provider in a context.
func deleteModelCatalogSourceRowData(
	ctx context.Context,
	queries *sqlcgen.Queries,
	contextID string,
	status modelcatalog.SourceStatus,
) error {
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
	return nil
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

func normalizeModelCatalogBatchReplacement(
	replacement modelcatalog.SourceRowsReplacement,
) (normalizedModelCatalogReplacement, error) {
	if replacement.RemoveSource {
		if len(replacement.Rows) != 0 || replacement.ExecutionContext != (modelcatalog.CatalogExecutionContext{}) {
			return normalizedModelCatalogReplacement{}, fmt.Errorf(
				"store: live source removal cannot include rows or an execution context",
			)
		}
		sourceID, providerID := strings.TrimSpace(replacement.SourceID), strings.TrimSpace(replacement.ProviderID)
		if providerID == "" || sourceID != modelcatalog.SourceKindProviderLiveID(providerID) {
			return normalizedModelCatalogReplacement{}, fmt.Errorf(
				"store: live source removal requires matching source and provider IDs",
			)
		}
		return normalizedModelCatalogReplacement{
			executionContext: modelcatalog.GlobalCatalogExecutionContext(),
			status:           modelcatalog.SourceStatus{SourceID: sourceID, ProviderID: providerID}, removeSource: true,
		}, nil
	}
	executionContext, rows, status, err := normalizeModelCatalogReplacement(
		replacement.ExecutionContext,
		replacement.SourceID,
		replacement.ProviderID,
		replacement.Rows,
		replacement.Status,
	)
	return normalizedModelCatalogReplacement{executionContext: executionContext, rows: rows, status: status}, err
}

func deleteModelCatalogSource(
	ctx context.Context,
	exec modelCatalogSQLExecutor,
	status modelcatalog.SourceStatus,
) error {
	if err := sqlcgen.New(exec).DeleteModelCatalogSource(ctx, sqlcgen.DeleteModelCatalogSourceParams{
		SourceID: status.SourceID, ProviderID: status.ProviderID,
	}); err != nil {
		return fmt.Errorf("store: delete model catalog source: %w", err)
	}
	return deleteUnusedModelCatalogExecutionContexts(ctx, exec)
}

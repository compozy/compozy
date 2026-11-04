package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

const modelCatalogRevisionKey = "store.model_catalog_revision"

type modelCatalogReplacementPlan struct {
	revision     string
	replacements []normalizedModelCatalogReplacement
}

// Hydration and JSON comparison belong to the read side. The durable revision
// fences the result against writes through another handle or process as well.
func (g *ModelCatalogRepo) prepareModelCatalogReplacements(
	ctx context.Context,
	replacements []normalizedModelCatalogReplacement,
) (modelCatalogReplacementPlan, error) {
	plan := modelCatalogReplacementPlan{replacements: replacements}
	stored := make([][]modelcatalog.ModelRow, len(replacements))
	err := g.withModelCatalogReadTransaction(
		ctx,
		"prepare model catalog replacement",
		func(exec modelCatalogSQLExecutor) error {
			var err error
			plan.revision, err = readModelCatalogRevision(ctx, exec)
			if err != nil {
				return err
			}
			for index, replacement := range replacements {
				if replacement.removeSource {
					continue
				}
				stored[index], err = listModelCatalogRows(ctx, exec, modelcatalog.ListOptions{
					ExecutionContext: replacement.executionContext, SourceID: replacement.status.SourceID,
					ProviderID: replacement.status.ProviderID, IncludeAll: true, IncludeStale: true,
				})
				if err != nil {
					return err
				}
			}
			return nil
		},
	)
	if err != nil {
		return modelCatalogReplacementPlan{}, err
	}
	// Close the snapshot before the potentially expensive content comparison.
	for index, replacement := range replacements {
		_, uniform := uniformModelCatalogRefreshedAt(replacement.rows)
		plan.replacements[index].unchanged = !replacement.removeSource && uniform &&
			modelCatalogRowContentEqual(stored[index], replacement.rows, replacement.status)
	}
	return plan, nil
}

func (g *ModelCatalogRepo) commitModelCatalogReplacements(
	ctx context.Context,
	action string,
	plan modelCatalogReplacementPlan,
) error {
	nextRevision, err := store.NewID("catalog")
	if err != nil {
		return err
	}
	updatedAt := store.FormatTimestamp(time.Now())
	if err := g.writeBackground(ctx, action, func(ctx context.Context, tx *store.WriteTx) error {
		revision, err := readModelCatalogRevision(ctx, tx)
		if err != nil {
			return err
		}
		changedSources := make(map[modelCatalogRowKey]struct{}, len(plan.replacements))
		for _, replacement := range plan.replacements {
			// A preceding replacement can prune sibling contexts or remove the
			// whole source. Their prepared comparisons must not survive that write.
			key := modelCatalogKey(replacement.status.SourceID, replacement.status.ProviderID, "")
			_, changed := changedSources[key]
			changedSources[key] = struct{}{}
			if replacement.removeSource {
				if err := deleteModelCatalogSource(ctx, tx, replacement.status); err != nil {
					return err
				}
				continue
			}
			unchanged := replacement.unchanged && revision == plan.revision && !changed
			if err := replaceModelCatalogSourceRows(ctx, tx, replacement.executionContext,
				replacement.rows, replacement.status, unchanged); err != nil {
				return err
			}
		}
		return sqlcgen.New(tx).UpsertAppMetadata(ctx, sqlcgen.UpsertAppMetadataParams{
			Key: modelCatalogRevisionKey, Value: nextRevision, UpdatedAt: updatedAt,
		})
	}); err != nil {
		return fmt.Errorf("store: %s transaction: %w", action, err)
	}
	return nil
}

func readModelCatalogRevision(ctx context.Context, exec modelCatalogSQLExecutor) (string, error) {
	revision, err := sqlcgen.New(exec).GetAppMetadata(ctx, modelCatalogRevisionKey)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("store: read model catalog revision: %w", err)
	}
	return revision, nil
}

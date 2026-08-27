package globaldb

import (
	"context"

	"github.com/compozy/compozy/internal/modelcatalog"
)

func hydrateModelCatalogRows(
	ctx context.Context,
	exec modelCatalogSQLExecutor,
	opts modelcatalog.ListOptions,
	rows []modelcatalog.ModelRow,
) ([]modelcatalog.ModelRow, error) {
	efforts, err := listModelCatalogReasoningEfforts(ctx, exec, opts)
	if err != nil {
		return nil, err
	}
	bindings, err := listModelCatalogTransportBindings(ctx, exec, opts)
	if err != nil {
		return nil, err
	}
	options, err := listModelCatalogOptions(ctx, exec, opts)
	if err != nil {
		return nil, err
	}
	for index := range rows {
		key := modelCatalogKey(rows[index].SourceID, rows[index].ProviderID, rows[index].ModelID)
		rows[index].ReasoningEfforts = efforts[key]
		rows[index].ConfigOptions = options[key]
		rows[index].TransportBindings = bindings[key]
	}
	return rows, nil
}

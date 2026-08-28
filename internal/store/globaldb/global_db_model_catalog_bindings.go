package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func normalizeModelCatalogTransportBindings(
	bindings []modelcatalog.ModelTransportBinding,
) ([]modelcatalog.ModelTransportBinding, error) {
	if len(bindings) == 0 {
		return nil, nil
	}

	normalized := make([]modelcatalog.ModelTransportBinding, 0, len(bindings))
	seen := make(map[string]struct{}, len(bindings))
	for index, binding := range bindings {
		transportModelID, err := requireModelCatalogValue(binding.TransportModelID, "transport model id")
		if err != nil {
			return nil, fmt.Errorf("store: normalize transport binding %d: %w", index, err)
		}
		if _, exists := seen[transportModelID]; exists {
			return nil, fmt.Errorf(
				"store: duplicate transport model id %q at binding %d",
				transportModelID,
				index,
			)
		}
		seen[transportModelID] = struct{}{}

		binding.TransportModelID = transportModelID
		binding.Label = strings.TrimSpace(binding.Label)
		if binding.ReasoningEffort != nil {
			effort := modelcatalog.ReasoningEffort(strings.TrimSpace(string(*binding.ReasoningEffort)))
			switch {
			case effort == "":
				binding.ReasoningEffort = nil
			case !modelcatalog.IsValidEffort(string(effort)):
				return nil, fmt.Errorf(
					"store: transport binding %q reasoning effort %q is unsupported",
					transportModelID,
					effort,
				)
			default:
				binding.ReasoningEffort = new(effort)
			}
		}
		normalized = append(normalized, binding)
	}
	return normalized, nil
}

func normalizeModelCatalogRowBindings(row *modelcatalog.ModelRow) error {
	bindings, err := normalizeModelCatalogTransportBindings(row.TransportBindings)
	if err != nil {
		return err
	}
	row.TransportBindings = bindings
	for index := range row.TransportBindings {
		selections, err := normalizeModelCatalogBindingSelections(
			row.TransportBindings[index].OptionSelections,
			row.ConfigOptions,
		)
		if err != nil {
			return fmt.Errorf(
				"store: normalize transport binding %q options: %w",
				row.TransportBindings[index].TransportModelID,
				err,
			)
		}
		row.TransportBindings[index].OptionSelections = selections
	}
	return nil
}

func insertModelCatalogTransportBindings(
	ctx context.Context,
	exec modelCatalogSQLExecutor,
	contextID string,
	row modelcatalog.ModelRow,
) error {
	queries := sqlcgen.New(exec)
	for rank, binding := range row.TransportBindings {
		if err := queries.InsertModelCatalogTransportBinding(
			ctx,
			sqlcgen.InsertModelCatalogTransportBindingParams{
				ContextID:        contextID,
				SourceID:         row.SourceID,
				ProviderID:       row.ProviderID,
				ModelID:          row.ModelID,
				TransportModelID: binding.TransportModelID,
				Label:            binding.Label,
				ReasoningEffort:  nullableReasoningEffort(binding.ReasoningEffort),
				Fast:             nullableBoolToSQLiteInt(binding.Fast),
				Thinking:         nullableBoolToSQLiteInt(binding.Thinking),
				Rank:             int64(rank),
			},
		); err != nil {
			return fmt.Errorf(
				"store: insert model catalog transport binding %q/%q/%q/%q: %w",
				row.SourceID,
				row.ProviderID,
				row.ModelID,
				binding.TransportModelID,
				err,
			)
		}
	}
	return nil
}

func listModelCatalogTransportBindings(
	ctx context.Context,
	exec modelCatalogSQLExecutor,
	opts modelcatalog.ListOptions,
) (map[modelCatalogRowKey][]modelcatalog.ModelTransportBinding, error) {
	contextQueries, err := modelCatalogContextQueries(opts)
	if err != nil {
		return nil, err
	}
	rows := make([]sqlcgen.ModelCatalogTransportBinding, 0)
	queries := sqlcgen.New(exec)
	for _, contextQuery := range contextQueries {
		queried, queryErr := queries.ListModelCatalogTransportBindings(
			ctx,
			sqlcgen.ListModelCatalogTransportBindingsParams{
				ContextID:    contextQuery.contextID,
				ProviderID:   strings.TrimSpace(opts.ProviderID),
				SourceID:     contextQuery.sourceID,
				IncludeStale: int64(boolToSQLiteInt(opts.IncludeStale)),
				IncludeAll:   int64(boolToSQLiteInt(opts.IncludeAll)),
			},
		)
		if queryErr != nil {
			return nil, fmt.Errorf("store: query model catalog transport bindings: %w", queryErr)
		}
		rows = append(rows, queried...)
	}
	selectionByBinding, err := listModelCatalogBindingSelections(ctx, exec, opts)
	if err != nil {
		return nil, err
	}

	bindings := make(map[modelCatalogRowKey][]modelcatalog.ModelTransportBinding)
	for _, row := range rows {
		binding, err := modelCatalogTransportBindingFromGenerated(row)
		if err != nil {
			return nil, err
		}
		key := modelCatalogKey(row.SourceID, row.ProviderID, row.ModelID)
		binding.OptionSelections = selectionByBinding[modelCatalogBindingKey{
			row:              key,
			transportModelID: row.TransportModelID,
		}]
		bindings[key] = append(bindings[key], binding)
	}
	return bindings, nil
}

func modelCatalogTransportBindingFromGenerated(
	row sqlcgen.ModelCatalogTransportBinding,
) (modelcatalog.ModelTransportBinding, error) {
	transportModelID, err := requireModelCatalogValue(row.TransportModelID, "transport model id")
	if err != nil {
		return modelcatalog.ModelTransportBinding{}, err
	}
	fast, err := nullableSQLiteIntToBool(row.Fast, "transport binding fast")
	if err != nil {
		return modelcatalog.ModelTransportBinding{}, err
	}
	thinking, err := nullableSQLiteIntToBool(row.Thinking, "transport binding thinking")
	if err != nil {
		return modelcatalog.ModelTransportBinding{}, err
	}
	reasoningEffort, err := nullableModelCatalogBindingReasoningEffort(row.ReasoningEffort)
	if err != nil {
		return modelcatalog.ModelTransportBinding{}, err
	}
	return modelcatalog.ModelTransportBinding{
		TransportModelID: transportModelID,
		Label:            strings.TrimSpace(row.Label),
		ReasoningEffort:  reasoningEffort,
		Fast:             fast,
		Thinking:         thinking,
	}, nil
}

func nullableModelCatalogBindingReasoningEffort(
	value sql.NullString,
) (*modelcatalog.ReasoningEffort, error) {
	if !value.Valid {
		return nil, nil
	}
	effort := modelcatalog.ReasoningEffort(strings.TrimSpace(value.String))
	if effort == "" {
		return nil, nil
	}
	if !modelcatalog.IsValidEffort(string(effort)) {
		return nil, fmt.Errorf("store: transport binding reasoning effort %q is unsupported", effort)
	}
	return new(effort), nil
}

func insertModelCatalogBindingSelections(
	ctx context.Context,
	exec modelCatalogSQLExecutor,
	contextID string,
	row modelcatalog.ModelRow,
) error {
	queries := sqlcgen.New(exec)
	for _, binding := range row.TransportBindings {
		for _, selection := range binding.OptionSelections {
			if err := queries.InsertModelCatalogTransportBindingSelection(
				ctx,
				sqlcgen.InsertModelCatalogTransportBindingSelectionParams{
					ContextID:        contextID,
					SourceID:         row.SourceID,
					ProviderID:       row.ProviderID,
					ModelID:          row.ModelID,
					TransportModelID: binding.TransportModelID,
					OptionID:         selection.ID,
					ValueID:          nullableModelCatalogString(selection.ValueID),
					BoolValue:        nullableBoolToSQLiteInt(selection.BoolValue),
				},
			); err != nil {
				return fmt.Errorf("store: insert binding option %q/%q: %w", binding.TransportModelID, selection.ID, err)
			}
		}
	}
	return nil
}

func listModelCatalogBindingSelections(
	ctx context.Context,
	exec modelCatalogSQLExecutor,
	opts modelcatalog.ListOptions,
) (map[modelCatalogBindingKey][]modelcatalog.ModelOptionSelection, error) {
	contextQueries, err := modelCatalogContextQueries(opts)
	if err != nil {
		return nil, err
	}
	rows := make([]sqlcgen.ModelCatalogTransportBindingSelection, 0)
	queries := sqlcgen.New(exec)
	for _, contextQuery := range contextQueries {
		queried, queryErr := queries.ListModelCatalogTransportBindingSelections(
			ctx,
			sqlcgen.ListModelCatalogTransportBindingSelectionsParams{
				ContextID: contextQuery.contextID, ProviderID: strings.TrimSpace(opts.ProviderID),
				SourceID:     contextQuery.sourceID,
				IncludeStale: int64(boolToSQLiteInt(opts.IncludeStale)),
				IncludeAll:   int64(boolToSQLiteInt(opts.IncludeAll)),
			},
		)
		if queryErr != nil {
			return nil, fmt.Errorf("store: query model catalog binding options: %w", queryErr)
		}
		rows = append(rows, queried...)
	}
	selections := make(map[modelCatalogBindingKey][]modelcatalog.ModelOptionSelection, len(rows))
	for _, row := range rows {
		selection, err := modelCatalogBindingSelectionFromGenerated(row)
		if err != nil {
			return nil, err
		}
		key := modelCatalogBindingKey{
			row:              modelCatalogKey(row.SourceID, row.ProviderID, row.ModelID),
			transportModelID: row.TransportModelID,
		}
		selections[key] = append(selections[key], selection)
	}
	return selections, nil
}

type modelCatalogBindingKey struct {
	row              modelCatalogRowKey
	transportModelID string
}

func modelCatalogBindingSelectionFromGenerated(
	row sqlcgen.ModelCatalogTransportBindingSelection,
) (modelcatalog.ModelOptionSelection, error) {
	boolValue, err := nullableSQLiteIntToBool(row.BoolValue, "binding option bool value")
	if err != nil {
		return modelcatalog.ModelOptionSelection{}, err
	}
	valueID := ""
	if row.ValueID.Valid {
		valueID = strings.TrimSpace(row.ValueID.String)
	}
	selection := modelcatalog.ModelOptionSelection{ID: row.OptionID, ValueID: valueID, BoolValue: boolValue}
	if err := modelcatalog.ValidateModelOptionSelection(selection); err != nil {
		return modelcatalog.ModelOptionSelection{}, fmt.Errorf("store: scan binding option %q: %w", row.OptionID, err)
	}
	return selection, nil
}

func nullableModelCatalogString(value string) sql.NullString {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}

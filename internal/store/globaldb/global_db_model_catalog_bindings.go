package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// normalizeModelCatalogTransportBindings validates transport identities and their complete capability snapshots.
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
		binding.ConfigOptions, err = normalizeModelCatalogOptions(binding.ConfigOptions)
		if err != nil {
			return nil, fmt.Errorf("store: transport binding %q options: %w", transportModelID, err)
		}
		normalized = append(normalized, binding)
	}
	return normalized, nil
}

// normalizeModelCatalogRowBindings validates configuration-matrix coordinates against their logical model.
// OptionSelections chooses a transport; ConfigOptions describes controls available after that transport is selected.
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

// insertModelCatalogTransportBindings persists each normalized snapshot atomically with its source row.
func insertModelCatalogTransportBindings(
	ctx context.Context,
	exec modelCatalogSQLExecutor,
	contextID string,
	row modelcatalog.ModelRow,
) error {
	queries := sqlcgen.New(exec)
	for rank, binding := range row.TransportBindings {
		options, err := json.Marshal(binding.ConfigOptions)
		if err != nil {
			return fmt.Errorf("store: encode transport binding options: %w", err)
		}
		if err := queries.InsertModelCatalogTransportBinding(
			ctx,
			sqlcgen.InsertModelCatalogTransportBindingParams{
				ContextID:         contextID,
				SourceID:          row.SourceID,
				ProviderID:        row.ProviderID,
				ModelID:           row.ModelID,
				TransportModelID:  binding.TransportModelID,
				Label:             binding.Label,
				ReasoningEffort:   nullableReasoningEffort(binding.ReasoningEffort),
				Fast:              nullableBoolToSQLiteInt(binding.Fast),
				Thinking:          nullableBoolToSQLiteInt(binding.Thinking),
				Rank:              int64(rank),
				ConfigOptionsJson: string(options),
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

// listModelCatalogTransportBindings restores validated snapshots within the requested catalog execution contexts.
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

// modelCatalogTransportBindingFromGenerated decodes and validates stored capabilities at the repository boundary.
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
	var options []modelcatalog.ModelOptionDescriptor
	if err := json.Unmarshal([]byte(row.ConfigOptionsJson), &options); err != nil {
		return modelcatalog.ModelTransportBinding{}, fmt.Errorf("store: decode transport binding options: %w", err)
	}
	options, err = normalizeModelCatalogOptions(options)
	if err != nil {
		return modelcatalog.ModelTransportBinding{}, err
	}
	return modelcatalog.ModelTransportBinding{
		TransportModelID: transportModelID,
		ConfigOptions:    options,
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

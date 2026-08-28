package globaldb

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func normalizeModelCatalogOptions(
	options []modelcatalog.ModelOptionDescriptor,
) ([]modelcatalog.ModelOptionDescriptor, error) {
	if len(options) == 0 {
		return nil, nil
	}
	normalized := make([]modelcatalog.ModelOptionDescriptor, 0, len(options))
	seen := make(map[string]struct{}, len(options))
	for index, option := range options {
		candidate, err := normalizeModelCatalogOption(option)
		if err != nil {
			return nil, fmt.Errorf("store: normalize model catalog option %d: %w", index, err)
		}
		if _, exists := seen[candidate.ID]; exists {
			return nil, fmt.Errorf("store: duplicate model catalog option id %q", candidate.ID)
		}
		seen[candidate.ID] = struct{}{}
		normalized = append(normalized, candidate)
	}
	slices.SortFunc(
		normalized,
		func(left modelcatalog.ModelOptionDescriptor, right modelcatalog.ModelOptionDescriptor) int {
			return cmp.Compare(left.ID, right.ID)
		},
	)
	return normalized, nil
}

func normalizeModelCatalogOption(
	option modelcatalog.ModelOptionDescriptor,
) (modelcatalog.ModelOptionDescriptor, error) {
	normalized := option
	var err error
	if normalized.ID, err = requireModelCatalogValue(normalized.ID, "option id"); err != nil {
		return modelcatalog.ModelOptionDescriptor{}, err
	}
	normalized.Label = strings.TrimSpace(normalized.Label)
	normalized.Description = strings.TrimSpace(normalized.Description)
	normalized.Category = strings.TrimSpace(normalized.Category)
	normalized.Kind = modelcatalog.ModelOptionKind(strings.TrimSpace(string(normalized.Kind)))
	if normalized.Kind == "" {
		return modelcatalog.ModelOptionDescriptor{}, fmt.Errorf(
			"model catalog option %q kind is required",
			normalized.ID,
		)
	}
	normalized.CurrentValueID = strings.TrimSpace(normalized.CurrentValueID)
	normalized.CurrentBool = cloneModelCatalogBool(normalized.CurrentBool)
	if normalized.CurrentValueID != "" && normalized.CurrentBool != nil {
		return modelcatalog.ModelOptionDescriptor{}, fmt.Errorf(
			"model catalog option %q current value must set at most one of value_id or bool_value",
			normalized.ID,
		)
	}
	if normalized.Kind == modelcatalog.ModelOptionKindSelect && normalized.CurrentBool != nil {
		return modelcatalog.ModelOptionDescriptor{}, fmt.Errorf(
			"model catalog select option %q cannot set current bool value",
			normalized.ID,
		)
	}
	if normalized.Kind == modelcatalog.ModelOptionKindBoolean && normalized.CurrentValueID != "" {
		return modelcatalog.ModelOptionDescriptor{}, fmt.Errorf(
			"model catalog boolean option %q cannot set current value id",
			normalized.ID,
		)
	}
	values, err := normalizeModelCatalogOptionValues(normalized.ID, normalized.Values)
	if err != nil {
		return modelcatalog.ModelOptionDescriptor{}, err
	}
	if normalized.Kind == modelcatalog.ModelOptionKindBoolean && len(values) > 0 {
		return modelcatalog.ModelOptionDescriptor{}, fmt.Errorf(
			"model catalog boolean option %q cannot advertise select values",
			normalized.ID,
		)
	}
	normalized.Values = values
	if normalized.Kind == modelcatalog.ModelOptionKindSelect && normalized.CurrentValueID != "" {
		if !slices.ContainsFunc(values, func(value modelcatalog.ModelOptionValue) bool {
			return value.ValueID == normalized.CurrentValueID
		}) {
			return modelcatalog.ModelOptionDescriptor{}, fmt.Errorf(
				"model catalog select option %q current value %q is not advertised",
				normalized.ID,
				normalized.CurrentValueID,
			)
		}
	}
	return normalized, nil
}

func normalizeModelCatalogOptionValues(
	optionID string,
	values []modelcatalog.ModelOptionValue,
) ([]modelcatalog.ModelOptionValue, error) {
	if len(values) == 0 {
		return nil, nil
	}
	normalized := make([]modelcatalog.ModelOptionValue, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for index, value := range values {
		valueID, err := requireModelCatalogValue(value.ValueID, "option value id")
		if err != nil {
			return nil, fmt.Errorf("model catalog option %q value %d: %w", optionID, index, err)
		}
		if _, exists := seen[valueID]; exists {
			return nil, fmt.Errorf("model catalog option %q has duplicate value id %q", optionID, valueID)
		}
		if value.Order < 0 {
			return nil, fmt.Errorf(
				"model catalog option %q value %q order %d is invalid",
				optionID,
				valueID,
				value.Order,
			)
		}
		seen[valueID] = struct{}{}
		value.ValueID = valueID
		value.Label = strings.TrimSpace(value.Label)
		value.Description = strings.TrimSpace(value.Description)
		value.GroupID = strings.TrimSpace(value.GroupID)
		value.GroupLabel = strings.TrimSpace(value.GroupLabel)
		normalized = append(normalized, value)
	}
	slices.SortFunc(normalized, func(left modelcatalog.ModelOptionValue, right modelcatalog.ModelOptionValue) int {
		if order := cmp.Compare(left.Order, right.Order); order != 0 {
			return order
		}
		return cmp.Compare(left.ValueID, right.ValueID)
	})
	return normalized, nil
}

func normalizeModelCatalogBindingSelections(
	selections []modelcatalog.ModelOptionSelection,
	options []modelcatalog.ModelOptionDescriptor,
) ([]modelcatalog.ModelOptionSelection, error) {
	if len(selections) == 0 {
		return nil, nil
	}
	optionByID := make(map[string]modelcatalog.ModelOptionDescriptor, len(options))
	for _, option := range options {
		optionByID[option.ID] = option
	}
	normalized := make([]modelcatalog.ModelOptionSelection, 0, len(selections))
	seen := make(map[string]struct{}, len(selections))
	for index, selection := range selections {
		candidate, err := normalizeModelCatalogOptionSelection(selection)
		if err != nil {
			return nil, fmt.Errorf("store: normalize binding option %d: %w", index, err)
		}
		if _, exists := seen[candidate.ID]; exists {
			return nil, fmt.Errorf("store: duplicate binding option id %q", candidate.ID)
		}
		option, exists := optionByID[candidate.ID]
		if !exists {
			return nil, fmt.Errorf("store: binding option %q is not advertised by the model", candidate.ID)
		}
		switch option.Kind {
		case modelcatalog.ModelOptionKindSelect:
			if candidate.BoolValue != nil {
				return nil, fmt.Errorf("store: select binding option %q cannot set bool_value", candidate.ID)
			}
			if !slices.ContainsFunc(option.Values, func(value modelcatalog.ModelOptionValue) bool {
				return value.ValueID == candidate.ValueID
			}) {
				return nil, fmt.Errorf(
					"store: binding option %q value %q is not advertised by the model",
					candidate.ID,
					candidate.ValueID,
				)
			}
		case modelcatalog.ModelOptionKindBoolean:
			if candidate.ValueID != "" {
				return nil, fmt.Errorf("store: boolean binding option %q cannot set value_id", candidate.ID)
			}
		}
		seen[candidate.ID] = struct{}{}
		normalized = append(normalized, candidate)
	}
	slices.SortFunc(
		normalized,
		func(left modelcatalog.ModelOptionSelection, right modelcatalog.ModelOptionSelection) int {
			return cmp.Compare(left.ID, right.ID)
		},
	)
	return normalized, nil
}

func normalizeModelCatalogOptionSelection(
	selection modelcatalog.ModelOptionSelection,
) (modelcatalog.ModelOptionSelection, error) {
	normalized := selection
	normalized.ID = strings.TrimSpace(normalized.ID)
	normalized.ValueID = strings.TrimSpace(normalized.ValueID)
	normalized.BoolValue = cloneModelCatalogBool(normalized.BoolValue)
	if err := modelcatalog.ValidateModelOptionSelection(normalized); err != nil {
		return modelcatalog.ModelOptionSelection{}, err
	}
	return normalized, nil
}

func cloneModelCatalogBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	return new(*value)
}

func insertModelCatalogOptions(
	ctx context.Context,
	exec modelCatalogSQLExecutor,
	contextID string,
	row modelcatalog.ModelRow,
) error {
	queries := sqlcgen.New(exec)
	for _, option := range row.ConfigOptions {
		if err := queries.InsertModelCatalogOption(ctx, sqlcgen.InsertModelCatalogOptionParams{
			ContextID:      contextID,
			SourceID:       row.SourceID,
			ProviderID:     row.ProviderID,
			ModelID:        row.ModelID,
			OptionID:       option.ID,
			Label:          option.Label,
			Description:    option.Description,
			Category:       option.Category,
			Kind:           string(option.Kind),
			CurrentValueID: nullableModelCatalogString(option.CurrentValueID),
			CurrentBool:    nullableBoolToSQLiteInt(option.CurrentBool),
		}); err != nil {
			return fmt.Errorf("store: insert model catalog option %q: %w", option.ID, err)
		}
		for _, value := range option.Values {
			if err := queries.InsertModelCatalogOptionValue(ctx, sqlcgen.InsertModelCatalogOptionValueParams{
				ContextID: contextID, SourceID: row.SourceID,
				ProviderID: row.ProviderID, ModelID: row.ModelID,
				OptionID: option.ID, ValueID: value.ValueID, Label: value.Label,
				Description: value.Description, GroupID: value.GroupID,
				GroupLabel: value.GroupLabel, Rank: int64(value.Order),
			}); err != nil {
				return fmt.Errorf("store: insert model catalog option %q value %q: %w", option.ID, value.ValueID, err)
			}
		}
	}
	return nil
}

func listModelCatalogOptions(
	ctx context.Context,
	exec modelCatalogSQLExecutor,
	opts modelcatalog.ListOptions,
) (map[modelCatalogRowKey][]modelcatalog.ModelOptionDescriptor, error) {
	queries := sqlcgen.New(exec)
	contextQueries, err := modelCatalogContextQueries(opts)
	if err != nil {
		return nil, err
	}
	optionRows := make([]sqlcgen.ModelCatalogOption, 0)
	valueRows := make([]sqlcgen.ModelCatalogOptionValue, 0)
	for _, contextQuery := range contextQueries {
		params := sqlcgen.ListModelCatalogOptionsParams{
			ContextID: contextQuery.contextID, ProviderID: strings.TrimSpace(opts.ProviderID),
			SourceID: contextQuery.sourceID, IncludeStale: int64(boolToSQLiteInt(opts.IncludeStale)),
			IncludeAll: int64(boolToSQLiteInt(opts.IncludeAll)),
		}
		queriedOptions, queryErr := queries.ListModelCatalogOptions(ctx, params)
		if queryErr != nil {
			return nil, fmt.Errorf("store: query model catalog options: %w", queryErr)
		}
		queriedValues, queryErr := queries.ListModelCatalogOptionValues(
			ctx,
			sqlcgen.ListModelCatalogOptionValuesParams(params),
		)
		if queryErr != nil {
			return nil, fmt.Errorf("store: query model catalog option values: %w", queryErr)
		}
		optionRows = append(optionRows, queriedOptions...)
		valueRows = append(valueRows, queriedValues...)
	}
	valuesByOption := make(map[modelCatalogOptionKey][]modelcatalog.ModelOptionValue, len(valueRows))
	for _, valueRow := range valueRows {
		value, err := modelCatalogOptionValueFromGenerated(valueRow)
		if err != nil {
			return nil, err
		}
		key := modelCatalogOptionKey{
			row:      modelCatalogKey(valueRow.SourceID, valueRow.ProviderID, valueRow.ModelID),
			optionID: valueRow.OptionID,
		}
		valuesByOption[key] = append(valuesByOption[key], value)
	}
	optionsByRow := make(map[modelCatalogRowKey][]modelcatalog.ModelOptionDescriptor, len(optionRows))
	for _, optionRow := range optionRows {
		option, err := modelCatalogOptionFromGenerated(optionRow)
		if err != nil {
			return nil, err
		}
		key := modelCatalogKey(optionRow.SourceID, optionRow.ProviderID, optionRow.ModelID)
		option.Values = valuesByOption[modelCatalogOptionKey{row: key, optionID: option.ID}]
		optionsByRow[key] = append(optionsByRow[key], option)
	}
	return optionsByRow, nil
}

type modelCatalogOptionKey struct {
	row      modelCatalogRowKey
	optionID string
}

func modelCatalogOptionFromGenerated(
	row sqlcgen.ModelCatalogOption,
) (modelcatalog.ModelOptionDescriptor, error) {
	currentBool, err := nullableSQLiteIntToBool(row.CurrentBool, "option current bool")
	if err != nil {
		return modelcatalog.ModelOptionDescriptor{}, err
	}
	currentValueID := ""
	if row.CurrentValueID.Valid {
		currentValueID = strings.TrimSpace(row.CurrentValueID.String)
	}
	if currentValueID != "" && currentBool != nil {
		return modelcatalog.ModelOptionDescriptor{}, fmt.Errorf(
			"store: model catalog option %q has both typed current values",
			row.OptionID,
		)
	}
	return modelcatalog.ModelOptionDescriptor{
		ID: row.OptionID, Label: strings.TrimSpace(row.Label),
		Description: strings.TrimSpace(row.Description), Category: strings.TrimSpace(row.Category),
		Kind:           modelcatalog.ModelOptionKind(strings.TrimSpace(row.Kind)),
		CurrentValueID: currentValueID, CurrentBool: currentBool,
	}, nil
}

func modelCatalogOptionValueFromGenerated(
	row sqlcgen.ModelCatalogOptionValue,
) (modelcatalog.ModelOptionValue, error) {
	if row.Rank < 0 {
		return modelcatalog.ModelOptionValue{}, fmt.Errorf(
			"store: model catalog option value %q has invalid order %d",
			row.ValueID,
			row.Rank,
		)
	}
	return modelcatalog.ModelOptionValue{
		ValueID: row.ValueID, Label: strings.TrimSpace(row.Label), Description: strings.TrimSpace(row.Description),
		GroupID: strings.TrimSpace(row.GroupID), GroupLabel: strings.TrimSpace(row.GroupLabel), Order: int(row.Rank),
	}, nil
}

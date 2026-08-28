package loop

import (
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

const (
	runtimeACPOptionIDKey        = "id"
	runtimeACPOptionValueIDKey   = "value_id"
	runtimeACPOptionBoolValueKey = "bool_value"
)

func runtimeACPOptionsFromValue(value any) ([]dsl.ACPOptionSelection, error) {
	var rawSelections []any
	switch typed := value.(type) {
	case []any:
		rawSelections = typed
	case []dsl.ACPOptionSelection:
		return dsl.NormalizeACPOptionSelections(typed)
	case []map[string]any:
		rawSelections = make([]any, len(typed))
		for index, selection := range typed {
			rawSelections[index] = selection
		}
	default:
		return nil, fmt.Errorf("acp_options must be an array")
	}
	selections := make([]dsl.ACPOptionSelection, 0, len(rawSelections))
	for index, raw := range rawSelections {
		selection, err := runtimeACPOptionFromValue(raw)
		if err != nil {
			return nil, fmt.Errorf("acp_options[%d]: %w", index, err)
		}
		selections = append(selections, selection)
	}
	return dsl.NormalizeACPOptionSelections(selections)
}

func runtimeACPOptionFromValue(value any) (dsl.ACPOptionSelection, error) {
	fields, err := runtimeACPOptionFields(value)
	if err != nil {
		return dsl.ACPOptionSelection{}, err
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	selection := dsl.ACPOptionSelection{}
	for _, key := range keys {
		raw := fields[key]
		switch key {
		case runtimeACPOptionIDKey, runtimeACPOptionValueIDKey:
			text, ok := raw.(string)
			if !ok {
				return dsl.ACPOptionSelection{}, fmt.Errorf("%s must be a string", key)
			}
			if key == runtimeACPOptionIDKey {
				selection.ID = text
			} else {
				selection.ValueID = text
			}
		case runtimeACPOptionBoolValueKey:
			boolValue, ok := raw.(bool)
			if !ok {
				return dsl.ACPOptionSelection{}, fmt.Errorf("%s must be a boolean", key)
			}
			selection.BoolValue = new(boolValue)
		default:
			return dsl.ACPOptionSelection{}, fmt.Errorf("%s is unknown", key)
		}
	}
	normalized, err := selection.Normalize()
	if err != nil {
		return dsl.ACPOptionSelection{}, err
	}
	return normalized, nil
}

func runtimeACPOptionFields(value any) (map[string]any, error) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, nil
	case map[any]any:
		fields := make(map[string]any, len(typed))
		for key, raw := range typed {
			name, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("field name must be a string")
			}
			fields[name] = raw
		}
		return fields, nil
	case dsl.ACPOptionSelection:
		fields := map[string]any{runtimeACPOptionIDKey: typed.ID}
		if typed.ValueID != "" {
			fields[runtimeACPOptionValueIDKey] = typed.ValueID
		}
		if typed.BoolValue != nil {
			fields[runtimeACPOptionBoolValueKey] = *typed.BoolValue
		}
		return fields, nil
	default:
		return nil, fmt.Errorf("must be an object")
	}
}

func runtimeACPOptionsValue(options []dsl.ACPOptionSelection) []map[string]any {
	if len(options) == 0 {
		return nil
	}
	cloned := dsl.CloneACPOptionSelections(options)
	slices.SortStableFunc(cloned, func(left, right dsl.ACPOptionSelection) int {
		return strings.Compare(strings.TrimSpace(left.ID), strings.TrimSpace(right.ID))
	})
	value := make([]map[string]any, len(cloned))
	for index, option := range cloned {
		entry := map[string]any{
			runtimeACPOptionIDKey: option.ID,
		}
		if option.ValueID != "" {
			entry[runtimeACPOptionValueIDKey] = option.ValueID
		}
		if option.BoolValue != nil {
			entry[runtimeACPOptionBoolValueKey] = *option.BoolValue
		}
		value[index] = entry
	}
	return value
}

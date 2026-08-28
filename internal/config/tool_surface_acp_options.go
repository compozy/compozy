package config

import "fmt"

func normalizeToolACPOptions(value any) (any, error) {
	values, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("config: expected ACP options array, got %T", value)
	}
	selections := make([]ACPOptionSelection, 0, len(values))
	for index, value := range values {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("config: ACP option %d must be an object", index)
		}
		for key := range object {
			switch key {
			case "id", "value_id", "bool_value":
			default:
				return nil, fmt.Errorf("config: ACP option %d has unknown field %q", index, key)
			}
		}
		id, err := toolACPOptionString(object, "id", index)
		if err != nil {
			return nil, err
		}
		valueID, err := toolACPOptionString(object, "value_id", index)
		if err != nil {
			return nil, err
		}
		selection := ACPOptionSelection{ID: id, ValueID: valueID}
		if rawBool, exists := object["bool_value"]; exists {
			boolValue, boolOK := rawBool.(bool)
			if !boolOK {
				return nil, fmt.Errorf("config: ACP option %d bool_value must be a boolean", index)
			}
			selection.BoolValue = new(boolValue)
		}
		selections = append(selections, selection)
	}
	normalized, err := normalizeACPOptionSelectionsAt("acp_options", selections)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(normalized))
	for _, selection := range normalized {
		item := map[string]any{"id": selection.ID}
		if selection.BoolValue != nil {
			item["bool_value"] = *selection.BoolValue
		} else {
			item["value_id"] = selection.ValueID
		}
		result = append(result, item)
	}
	return result, nil
}

func toolACPOptionString(object map[string]any, key string, index int) (string, error) {
	value, exists := object[key]
	if !exists {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("config: ACP option %d %s must be a string", index, key)
	}
	return text, nil
}

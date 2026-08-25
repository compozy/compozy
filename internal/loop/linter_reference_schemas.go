package loop

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
)

func inputSchema(input dsl.Input) refs.Schema {
	switch input.Type {
	case dsl.InputTypeAgent, dsl.InputTypeRef, dsl.InputTypeFile:
		return refs.Schema{jsonSchemaTypeKey: jsonSchemaStringType}
	case dsl.InputTypeRuntime:
		return refs.Schema{
			jsonSchemaTypeKey: jsonSchemaObjectType,
			jsonSchemaPropertiesKey: map[string]any{
				runtimeFieldProvider:  map[string]any{jsonSchemaTypeKey: jsonSchemaStringType},
				runtimeFieldModel:     map[string]any{jsonSchemaTypeKey: jsonSchemaStringType},
				runtimeFieldReasoning: map[string]any{jsonSchemaTypeKey: jsonSchemaStringType},
				runtimeFieldACPOptions: map[string]any{
					jsonSchemaTypeKey: jsonSchemaArrayType,
					"items": map[string]any{
						jsonSchemaTypeKey: jsonSchemaObjectType,
						jsonSchemaPropertiesKey: map[string]any{
							runtimeACPOptionIDKey:        map[string]any{jsonSchemaTypeKey: jsonSchemaStringType},
							runtimeACPOptionValueIDKey:   map[string]any{jsonSchemaTypeKey: jsonSchemaStringType},
							runtimeACPOptionBoolValueKey: map[string]any{jsonSchemaTypeKey: jsonSchemaBooleanType},
						},
						jsonSchemaAdditionalPropertiesKey: false,
						jsonSchemaRequiredKey:             []string{runtimeACPOptionIDKey},
						jsonSchemaOneOfKey: []any{
							map[string]any{jsonSchemaRequiredKey: []string{runtimeACPOptionValueIDKey}},
							map[string]any{jsonSchemaRequiredKey: []string{runtimeACPOptionBoolValueKey}},
						},
					},
				},
			},
			jsonSchemaAdditionalPropertiesKey: false,
		}
	default:
		return refs.Schema{jsonSchemaTypeKey: string(input.Type)}
	}
}

func convertSchema(schema dsl.Schema) refs.Schema {
	out := refs.Schema{}
	for key, value := range schema {
		out[key] = normalizeSchemaValue(value)
	}
	return out
}

func normalizeSchemaValue(value any) any {
	switch typed := value.(type) {
	case dsl.Schema:
		normalized := map[string]any{}
		for key, child := range typed {
			normalized[key] = normalizeSchemaValue(child)
		}
		return normalized
	case map[string]any:
		normalized := map[string]any{}
		for key, child := range typed {
			normalized[key] = normalizeSchemaValue(child)
		}
		return normalized
	case map[any]any:
		normalized := map[string]any{}
		for key, child := range typed {
			normalized[fmt.Sprint(key)] = normalizeSchemaValue(child)
		}
		return normalized
	case []any:
		normalized := make([]any, 0, len(typed))
		for _, child := range typed {
			normalized = append(normalized, normalizeSchemaValue(child))
		}
		return normalized
	default:
		return typed
	}
}

func schemaFromJSON(raw json.RawMessage) (refs.Schema, error) {
	decoded := map[string]any{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("decode JSON schema: %w", err)
	}
	return refs.Schema(decoded), nil
}

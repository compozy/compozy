package loop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const jsonSchemaEnumKey = "enum"

func validateRunAgentStructured(schema dsl.Schema, result ActionPromptResult) (json.RawMessage, error) {
	return ValidateActionStructured(schema, result)
}

// ValidateActionStructured applies the Loop-owned generation-output schema validator.
// Child action executors use this seam instead of owning a second schema implementation.
func ValidateActionStructured(schema dsl.Schema, result ActionPromptResult) (json.RawMessage, error) {
	if len(schema) == 0 {
		if len(bytes.TrimSpace(result.Structured)) > 0 {
			return cloneRawMessage(result.Structured), nil
		}
		return nil, nil
	}
	raw, err := structuredCandidate(result)
	if err != nil {
		return nil, reasonError(ReasonCodeActionSchemaInvalid, errors.Join(ErrActionSchemaInvalid, err), nil)
	}
	if err := validateJSONSchema(schema, raw); err != nil {
		return nil, reasonError(ReasonCodeActionSchemaInvalid, errors.Join(ErrActionSchemaInvalid, err), nil)
	}
	return raw, nil
}

func structuredCandidate(result ActionPromptResult) (json.RawMessage, error) {
	if len(bytes.TrimSpace(result.Structured)) > 0 {
		if !json.Valid(result.Structured) {
			return nil, errors.New("structured result is not valid JSON")
		}
		return cloneRawMessage(result.Structured), nil
	}
	return extractJSONObject(result.Text)
}

// ActionStructuredCandidate returns the structured object supplied by one action prompt result.
func ActionStructuredCandidate(result ActionPromptResult) (json.RawMessage, error) {
	return structuredCandidate(result)
}

func validateJSONSchema(schema dsl.Schema, raw json.RawMessage) error {
	schemaDoc, err := normalizeLoopSchema(schema)
	if err != nil {
		return err
	}
	schemaData, err := json.Marshal(schemaDoc)
	if err != nil {
		return fmt.Errorf("marshal output schema: %w", err)
	}
	schemaValue, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaData))
	if err != nil {
		return fmt.Errorf("parse output schema: %w", err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("parse structured output: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("output_schema.json", schemaValue); err != nil {
		return fmt.Errorf("add output schema resource: %w", err)
	}
	compiled, err := compiler.Compile("output_schema.json")
	if err != nil {
		return fmt.Errorf("compile output schema: %w", err)
	}
	if err := compiled.Validate(instance); err != nil {
		return fmt.Errorf("validate structured output: %w", err)
	}
	return nil
}

func normalizeLoopSchema(schema dsl.Schema) (map[string]any, error) {
	normalized, err := normalizeJSONValue(map[string]any(schema))
	if err != nil {
		return nil, err
	}
	schemaMap, ok := normalized.(map[string]any)
	if !ok {
		return nil, errors.New("output schema must normalize to object")
	}
	if isFullJSONSchema(schemaMap) {
		return schemaMap, nil
	}
	properties := make(map[string]any, len(schemaMap))
	required := make([]string, 0, len(schemaMap))
	for key, value := range schemaMap {
		properties[key] = shorthandPropertySchema(value)
		required = append(required, key)
	}
	sort.Strings(required)
	return map[string]any{
		jsonSchemaTypeKey:       jsonSchemaObjectType,
		jsonSchemaPropertiesKey: properties,
		jsonSchemaRequiredKey:   required,
	}, nil
}

func isFullJSONSchema(schema map[string]any) bool {
	fullKeys := map[string]struct{}{
		"$defs":                 {},
		"$id":                   {},
		"$schema":               {},
		"additionalProperties":  {},
		"allOf":                 {},
		"anyOf":                 {},
		"contains":              {},
		"contentEncoding":       {},
		"contentMediaType":      {},
		"const":                 {},
		"default":               {},
		"dependentRequired":     {},
		"dependentSchemas":      {},
		"description":           {},
		jsonSchemaEnumKey:       {},
		"examples":              {},
		"exclusiveMaximum":      {},
		"exclusiveMinimum":      {},
		"format":                {},
		"if":                    {},
		jsonSchemaItemsKey:      {},
		"maxItems":              {},
		"maxLength":             {},
		"maxProperties":         {},
		"maximum":               {},
		"minItems":              {},
		"minLength":             {},
		"minProperties":         {},
		"minimum":               {},
		"multipleOf":            {},
		"not":                   {},
		"oneOf":                 {},
		"pattern":               {},
		"prefixItems":           {},
		jsonSchemaPropertiesKey: {},
		"propertyNames":         {},
		jsonSchemaRequiredKey:   {},
		"then":                  {},
		jsonSchemaTitleKey:      {},
		jsonSchemaTypeKey:       {},
		"unevaluatedProperties": {},
	}
	hasKeyword := false
	for key := range schema {
		if _, ok := fullKeys[key]; !ok {
			return false
		}
		hasKeyword = true
	}
	return hasKeyword
}

func shorthandPropertySchema(value any) any {
	switch typed := value.(type) {
	case string:
		return map[string]any{jsonSchemaTypeKey: typed}
	case map[string]any:
		if isFullJSONSchema(typed) {
			return typed
		}
		nested := make(map[string]any, len(typed))
		for key, child := range typed {
			nested[key] = shorthandPropertySchema(child)
		}
		return map[string]any{
			jsonSchemaTypeKey:       jsonSchemaObjectType,
			jsonSchemaPropertiesKey: nested,
		}
	default:
		return value
	}
}

func schemaRetryPrompt(prompt string, schema dsl.Schema, validationErr error) (string, error) {
	schemaDoc, err := normalizeLoopSchema(schema)
	if err != nil {
		return "", err
	}
	schemaData, err := json.Marshal(schemaDoc)
	if err != nil {
		return "", fmt.Errorf("marshal retry output schema: %w", err)
	}
	return fmt.Sprintf(
		"%s\n\n"+
			"Your previous response did not satisfy output_schema: %v\n"+
			"Return exactly one JSON object that satisfies this output_schema: %s",
		prompt,
		validationErr,
		string(schemaData),
	), nil
}

func extractJSONObject(text string) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(text)
	if raw, ok := validJSONObject(trimmed); ok {
		return raw, nil
	}
	if raw, ok := extractFencedJSONObject(trimmed); ok {
		return raw, nil
	}
	if raw, ok := extractBalancedJSONObject(trimmed); ok {
		return raw, nil
	}
	return nil, errors.New("no JSON object found")
}

func extractFencedJSONObject(text string) (json.RawMessage, bool) {
	start := 0
	for {
		open := strings.Index(text[start:], "```")
		if open < 0 {
			return nil, false
		}
		bodyStart := start + open + len("```")
		closeRel := strings.Index(text[bodyStart:], "```")
		if closeRel < 0 {
			return nil, false
		}
		body := text[bodyStart : bodyStart+closeRel]
		if newline := strings.IndexByte(body, '\n'); newline >= 0 {
			body = body[newline+1:]
		}
		if raw, ok := validJSONObject(strings.TrimSpace(body)); ok {
			return raw, true
		}
		start = bodyStart + closeRel + len("```")
	}
}

func extractBalancedJSONObject(text string) (json.RawMessage, bool) {
	start := strings.IndexByte(text, '{')
	if start < 0 {
		return nil, false
	}
	depth := 0
	inString := false
	escaped := false
	for idx := start; idx < len(text); idx++ {
		char := text[idx]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch char {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return validJSONObject(strings.TrimSpace(text[start : idx+1]))
			}
		}
	}
	return nil, false
}

func validJSONObject(candidate string) (json.RawMessage, bool) {
	if candidate == "" || candidate[0] != '{' || !json.Valid([]byte(candidate)) {
		return nil, false
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(candidate), &decoded); err != nil {
		return nil, false
	}
	if decoded == nil {
		return nil, false
	}
	return json.RawMessage(candidate), true
}

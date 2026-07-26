package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"unicode/utf8"
)

const (
	schemaTypeObject  = "object"
	schemaTypeArray   = "array"
	schemaTypeString  = "string"
	schemaTypeNumber  = "number"
	schemaTypeInteger = "integer"
	schemaTypeBoolean = "boolean"
	schemaTypeNull    = "null"
)

func normalizeCallInput(input json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(input)) == 0 {
		return json.RawMessage(`{}`)
	}
	return cloneRawMessage(input)
}

func validateCallInput(d Descriptor, input json.RawMessage) error {
	normalized := normalizeCallInput(input)
	if !json.Valid(normalized) {
		return NewToolError(
			ErrorCodeInvalidInput,
			d.ID,
			fmt.Sprintf("tool %q input is not valid JSON", d.ID),
			ErrToolInvalidInput,
			ReasonSchemaInvalid,
		)
	}
	if err := validateJSONSchemaValue(d.ID, d.InputSchema, normalized); err != nil {
		return err
	}
	return nil
}

func validateJSONSchemaValue(id ToolID, schema json.RawMessage, raw json.RawMessage) error {
	var schemaValue map[string]json.RawMessage
	if err := json.Unmarshal(schema, &schemaValue); err != nil {
		return NewToolError(
			ErrorCodeInvalidInput,
			id,
			fmt.Sprintf("tool %q input schema is invalid", id),
			fmt.Errorf("%w: %w", ErrToolInvalidInput, err),
			ReasonSchemaInvalid,
		)
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return NewToolError(
			ErrorCodeInvalidInput,
			id,
			fmt.Sprintf("tool %q input is invalid", id),
			fmt.Errorf("%w: %w", ErrToolInvalidInput, err),
			ReasonSchemaInvalid,
		)
	}
	if err := validateSchemaNode("$", schemaValue, value); err != nil {
		return NewToolError(
			ErrorCodeInvalidInput,
			id,
			fmt.Sprintf("tool %q input failed schema validation", id),
			fmt.Errorf("%w: %w", ErrToolInvalidInput, err),
			ReasonSchemaInvalid,
		)
	}
	return nil
}

func validateSchemaNode(path string, schema map[string]json.RawMessage, value any) error {
	types, err := schemaTypes(schema["type"])
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if len(types) > 0 && !jsonValueMatchesAnyType(value, types) {
		return fmt.Errorf("%s: expected %v, got %s", path, types, jsonValueType(value))
	}
	if err := validateSchemaEnum(path, schema["enum"], value); err != nil {
		return err
	}
	if err := validateSchemaAllOf(path, schema["allOf"], value); err != nil {
		return err
	}
	if err := validateSchemaAnyOf(path, schema["anyOf"], value); err != nil {
		return err
	}
	if err := validateSchemaOneOf(path, schema["oneOf"], value); err != nil {
		return err
	}
	if err := validateSchemaNot(path, schema["not"], value); err != nil {
		return err
	}
	if err := validateSchemaStringNode(path, schema, value); err != nil {
		return err
	}
	if err := validateSchemaArrayNode(path, schema, value); err != nil {
		return err
	}
	if slices.Contains(types, schemaTypeObject) || jsonValueType(value) == schemaTypeObject {
		return validateSchemaObjectNode(path, schema, value)
	}
	return nil
}

func validateSchemaStringNode(path string, schema map[string]json.RawMessage, value any) error {
	stringValue, ok := value.(string)
	if !ok {
		return nil
	}
	minLength, ok, err := schemaNonNegativeInteger(schema["minLength"])
	if err != nil {
		return fmt.Errorf("%s.minLength: %w", path, err)
	}
	stringLength := utf8.RuneCountInString(stringValue)
	if ok && stringLength < minLength {
		return fmt.Errorf("%s: string length %d is less than minLength %d", path, stringLength, minLength)
	}
	return nil
}

func validateSchemaArrayNode(path string, schema map[string]json.RawMessage, value any) error {
	arrayValue, ok := value.([]any)
	if !ok {
		return nil
	}
	minItems, ok, err := schemaNonNegativeInteger(schema["minItems"])
	if err != nil {
		return fmt.Errorf("%s.minItems: %w", path, err)
	}
	if ok && len(arrayValue) < minItems {
		return fmt.Errorf("%s: array length %d is less than minItems %d", path, len(arrayValue), minItems)
	}
	items, ok, err := schemaNode(schema["items"])
	if err != nil {
		return fmt.Errorf("%s.items: %w", path, err)
	}
	if !ok {
		return nil
	}
	for idx, item := range arrayValue {
		if err := validateSchemaNode(fmt.Sprintf("%s[%d]", path, idx), items, item); err != nil {
			return err
		}
	}
	return nil
}

func validateSchemaObjectNode(path string, schema map[string]json.RawMessage, value any) error {
	objectValue, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	required, err := schemaStringArray(schema["required"])
	if err != nil {
		return fmt.Errorf("%s.required: %w", path, err)
	}
	for _, field := range required {
		if _, ok := objectValue[field]; !ok {
			return fmt.Errorf("%s.%s: required field missing", path, field)
		}
	}
	properties, err := schemaProperties(schema["properties"])
	if err != nil {
		return fmt.Errorf("%s.properties: %w", path, err)
	}
	if len(properties) > 0 {
		for key, childSchema := range properties {
			childValue, ok := objectValue[key]
			if !ok {
				continue
			}
			if err := validateSchemaNode(path+"."+key, childSchema, childValue); err != nil {
				return err
			}
		}
	}
	if additionalPropertiesFalse(schema["additionalProperties"]) {
		for key := range objectValue {
			if _, ok := properties[key]; !ok {
				return fmt.Errorf("%s.%s: additional property is not allowed", path, key)
			}
		}
	}
	return nil
}

func validateSchemaEnum(path string, raw json.RawMessage, value any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var allowed []any
	if err := json.Unmarshal(raw, &allowed); err != nil {
		return fmt.Errorf("%s.enum: %w", path, err)
	}
	for _, candidate := range allowed {
		if reflect.DeepEqual(candidate, value) {
			return nil
		}
	}
	return fmt.Errorf("%s: value is not allowed", path)
}

func validateSchemaAllOf(path string, raw json.RawMessage, value any) error {
	nodes, err := schemaNodeArray(raw)
	if err != nil {
		return fmt.Errorf("%s.allOf: %w", path, err)
	}
	for idx, node := range nodes {
		if err := validateSchemaNode(fmt.Sprintf("%s.allOf[%d]", path, idx), node, value); err != nil {
			return err
		}
	}
	return nil
}

func validateSchemaAnyOf(path string, raw json.RawMessage, value any) error {
	nodes, err := schemaNodeArray(raw)
	if err != nil {
		return fmt.Errorf("%s.anyOf: %w", path, err)
	}
	if len(nodes) == 0 {
		return nil
	}
	for idx, node := range nodes {
		if err := validateSchemaNode(fmt.Sprintf("%s.anyOf[%d]", path, idx), node, value); err == nil {
			return nil
		}
	}
	return fmt.Errorf("%s: value must match at least one anyOf schema", path)
}

func validateSchemaOneOf(path string, raw json.RawMessage, value any) error {
	nodes, err := schemaNodeArray(raw)
	if err != nil {
		return fmt.Errorf("%s.oneOf: %w", path, err)
	}
	if len(nodes) == 0 {
		return nil
	}
	matches := 0
	for idx, node := range nodes {
		if err := validateSchemaNode(fmt.Sprintf("%s.oneOf[%d]", path, idx), node, value); err == nil {
			matches++
		}
	}
	if matches != 1 {
		return fmt.Errorf("%s: value matched %d oneOf schemas, want exactly one", path, matches)
	}
	return nil
}

func validateSchemaNot(path string, raw json.RawMessage, value any) error {
	node, ok, err := schemaNode(raw)
	if err != nil {
		return fmt.Errorf("%s.not: %w", path, err)
	}
	if !ok {
		return nil
	}
	if err := validateSchemaNode(path+".not", node, value); err == nil {
		return fmt.Errorf("%s: value matched forbidden schema", path)
	}
	return nil
}

func validateJSONSchemaDocument(path string, schema map[string]json.RawMessage) error {
	if _, err := schemaTypes(schema["type"]); err != nil {
		return fmt.Errorf("%s.type: %w", path, err)
	}

	properties, err := schemaProperties(schema["properties"])
	if err != nil {
		return fmt.Errorf("%s.properties: %w", path, err)
	}
	for key, childSchema := range properties {
		if err := validateJSONSchemaDocument(path+".properties."+key, childSchema); err != nil {
			return err
		}
	}
	if _, _, err := schemaNonNegativeInteger(schema["minLength"]); err != nil {
		return fmt.Errorf("%s.minLength: %w", path, err)
	}
	if _, _, err := schemaNonNegativeInteger(schema["minItems"]); err != nil {
		return fmt.Errorf("%s.minItems: %w", path, err)
	}
	items, ok, err := schemaNode(schema["items"])
	if err != nil {
		return fmt.Errorf("%s.items: %w", path, err)
	}
	if ok {
		if err := validateJSONSchemaDocument(path+".items", items); err != nil {
			return err
		}
	}

	if err := validateJSONSchemaDocumentArray(path+".allOf", schema["allOf"]); err != nil {
		return err
	}
	if err := validateJSONSchemaDocumentArray(path+".anyOf", schema["anyOf"]); err != nil {
		return err
	}
	if err := validateJSONSchemaDocumentArray(path+".oneOf", schema["oneOf"]); err != nil {
		return err
	}

	node, ok, err := schemaNode(schema["not"])
	if err != nil {
		return fmt.Errorf("%s.not: %w", path, err)
	}
	if ok {
		if err := validateJSONSchemaDocument(path+".not", node); err != nil {
			return err
		}
	}
	return nil
}

func validateJSONSchemaDocumentArray(path string, raw json.RawMessage) error {
	nodes, err := schemaNodeArray(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for idx, node := range nodes {
		if err := validateJSONSchemaDocument(fmt.Sprintf("%s[%d]", path, idx), node); err != nil {
			return err
		}
	}
	return nil
}

func schemaTypes(raw json.RawMessage) ([]string, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		if err := validateSchemaTypeName(single); err != nil {
			return nil, err
		}
		return []string{single}, nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err != nil {
		return nil, fmt.Errorf("type must be a string or string array: %w", err)
	}
	for _, item := range many {
		if err := validateSchemaTypeName(item); err != nil {
			return nil, err
		}
	}
	return many, nil
}

func validateSchemaTypeName(schemaType string) error {
	switch schemaType {
	case schemaTypeObject,
		schemaTypeArray,
		schemaTypeString,
		schemaTypeNumber,
		schemaTypeInteger,
		schemaTypeBoolean,
		schemaTypeNull:
		return nil
	default:
		return fmt.Errorf("unsupported JSON Schema type %q", schemaType)
	}
}

func schemaStringArray(raw json.RawMessage) ([]string, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func schemaProperties(raw json.RawMessage) (map[string]map[string]json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var values map[string]map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func schemaNode(raw json.RawMessage) (map[string]json.RawMessage, bool, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, false, nil
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false, err
	}
	return value, true, nil
}

func schemaNodeArray(raw json.RawMessage) ([]map[string]json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var values []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func schemaNonNegativeInteger(raw json.RawMessage) (int, bool, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return 0, false, nil
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, false, err
	}
	if value < 0 {
		return 0, false, fmt.Errorf("value must be non-negative")
	}
	return value, true, nil
}

func additionalPropertiesFalse(raw json.RawMessage) bool {
	if len(bytes.TrimSpace(raw)) == 0 {
		return false
	}
	var allowed bool
	if err := json.Unmarshal(raw, &allowed); err != nil {
		return false
	}
	return !allowed
}

func jsonValueMatchesAnyType(value any, types []string) bool {
	for _, item := range types {
		if jsonValueMatchesType(value, item) {
			return true
		}
	}
	return false
}

func jsonValueMatchesType(value any, schemaType string) bool {
	switch schemaType {
	case schemaTypeObject:
		_, ok := value.(map[string]any)
		return ok
	case schemaTypeArray:
		_, ok := value.([]any)
		return ok
	case schemaTypeString:
		_, ok := value.(string)
		return ok
	case schemaTypeNumber:
		_, ok := value.(float64)
		return ok
	case schemaTypeInteger:
		number, ok := value.(float64)
		return ok && number == float64(int64(number))
	case schemaTypeBoolean:
		_, ok := value.(bool)
		return ok
	case schemaTypeNull:
		return value == nil
	default:
		return false
	}
}

func jsonValueType(value any) string {
	switch value.(type) {
	case map[string]any:
		return schemaTypeObject
	case []any:
		return "array"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

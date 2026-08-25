package contracts

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

var schemaKeywords = map[string]struct{}{
	"$defs": {}, "$id": {}, "$ref": {}, "$schema": {},
	schemaAdditionalProperties: {}, schemaAllOf: {}, schemaAnyOf: {}, "const": {},
	schemaContains: {}, "contentEncoding": {}, "contentMediaType": {}, "default": {},
	"dependentRequired": {}, schemaDependentSchemas: {}, "description": {}, schemaElse: {},
	"enum": {}, "examples": {}, "exclusiveMaximum": {}, "exclusiveMinimum": {},
	"format": {}, schemaIf: {}, schemaItems: {}, "maxItems": {}, "maxLength": {},
	"maxProperties": {}, "maximum": {}, "minItems": {}, "minLength": {},
	"minProperties": {}, "minimum": {}, "multipleOf": {}, "not": {}, schemaOneOf: {},
	"pattern": {}, schemaPatternProperties: {}, schemaPrefixItems: {}, schemaProperties: {},
	schemaPropertyNames: {}, schemaRequired: {}, schemaThen: {}, "title": {}, schemaType: {},
	schemaUnevaluatedProperties: {}, "x-compozy-kind": {},
}

func normalizeSchema(raw json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, errors.New("schema must be a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("schema contains trailing JSON values")
		}
		return nil, fmt.Errorf("parse trailing schema content: %w", err)
	}
	object, ok := decoded.(map[string]any)
	if !ok || object == nil {
		return nil, errors.New("schema root must be an object")
	}
	if !isFullSchema(object) {
		object = shorthandObjectSchema(object)
	}
	if rootType, exists := object[schemaType]; exists && rootType != schemaObjectType {
		return nil, fmt.Errorf("schema root type must be object, got %q", rootType)
	}
	if _, exists := object[schemaType]; !exists {
		object[schemaType] = schemaObjectType
	}
	sortSchemaSets(object)
	canonical, err := json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("canonicalize schema: %w", err)
	}
	return canonical, nil
}

func sortSchemaSets(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == schemaRequired {
				if list, ok := child.([]any); ok {
					sort.Slice(list, func(i, j int) bool {
						return fmt.Sprint(list[i]) < fmt.Sprint(list[j])
					})
				}
			}
			sortSchemaSets(child)
		}
	case []any:
		for _, child := range typed {
			sortSchemaSets(child)
		}
	}
}

func isFullSchema(object map[string]any) bool {
	if len(object) == 0 {
		return false
	}
	for key := range object {
		if _, ok := schemaKeywords[key]; !ok {
			return false
		}
	}
	return true
}

func shorthandObjectSchema(object map[string]any) map[string]any {
	properties := make(map[string]any, len(object))
	required := make([]string, 0, len(object))
	for key, value := range object {
		properties[key] = shorthandValueSchema(value)
		required = append(required, key)
	}
	sort.Strings(required)
	return map[string]any{
		schemaType:       schemaObjectType,
		schemaProperties: properties,
		schemaRequired:   required,
	}
}

func shorthandValueSchema(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		if isFullSchema(typed) {
			return typed
		}
		return shorthandObjectSchema(typed)
	case []any:
		itemSchema := map[string]any{}
		if len(typed) > 0 {
			inferred := shorthandValueSchema(typed[0])
			if object, ok := inferred.(map[string]any); ok {
				itemSchema = object
			}
		}
		return map[string]any{schemaType: "array", schemaItems: itemSchema}
	case string:
		return map[string]any{schemaType: "string"}
	case json.Number:
		if strings.ContainsAny(string(typed), ".eE") {
			return map[string]any{schemaType: "number"}
		}
		return map[string]any{schemaType: "number"}
	case bool:
		return map[string]any{schemaType: "boolean"}
	case nil:
		return map[string]any{schemaType: "null"}
	default:
		return map[string]any{}
	}
}

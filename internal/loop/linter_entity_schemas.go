package loop

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
)

func (c *lintContext) lintEntityKindAnnotations(nodeID dsl.NodeID, schema map[string]any) {
	c.walkEntityKindSchema(nodeID, "", schema)
}

func (c *lintContext) walkEntityKindSchema(nodeID dsl.NodeID, path string, schema map[string]any) {
	if rawKind, exists := schema[jsonSchemaEntityKindKey]; exists {
		kind, ok := rawKind.(string)
		fieldPath := strings.TrimPrefix(path+"."+jsonSchemaEntityKindKey, ".")
		if !ok || !dsl.EntityKind(strings.TrimSpace(kind)).Valid() {
			c.addNodePath(
				nodeID,
				fieldPath,
				CodeRequestEntityKindInvalid,
				"%s must name a supported entity kind",
				jsonSchemaEntityKindKey,
			)
		} else if schema[jsonSchemaTypeKey] != jsonSchemaStringType {
			c.addNodePath(
				nodeID,
				fieldPath,
				CodeRequestEntityKindInvalid,
				"%s requires a string schema",
				jsonSchemaEntityKindKey,
			)
		}
	}
	c.walkEntitySchemaMap(nodeID, path, schema[jsonSchemaPropertiesKey])
	for _, keyword := range []string{"patternProperties", jsonSchemaDependentSchemasKey} {
		c.walkEntitySchemaMap(nodeID, appendSchemaPath(path, keyword), schema[keyword])
	}
	c.walkEntitySchemaValue(nodeID, appendSchemaPath(path, jsonSchemaItemsKey), schema[jsonSchemaItemsKey])
	for _, keyword := range []string{
		jsonSchemaAdditionalPropertiesKey, "unevaluatedProperties", "propertyNames",
	} {
		c.walkEntitySchemaValue(nodeID, appendSchemaPath(path, keyword), schema[keyword])
	}
	for _, keyword := range []string{jsonSchemaAllOfKey, jsonSchemaAnyOfKey, jsonSchemaOneOfKey, "prefixItems"} {
		values, ok := entitySchemaList(schema[keyword])
		if !ok {
			continue
		}
		for index, value := range values {
			c.walkEntitySchemaValue(
				nodeID,
				fmt.Sprintf("%s.%d", appendSchemaPath(path, keyword), index),
				value,
			)
		}
	}
	for _, keyword := range []string{"not", "if", jsonSchemaThenKey, "else", "contains"} {
		c.walkEntitySchemaValue(nodeID, appendSchemaPath(path, keyword), schema[keyword])
	}
}

func (c *lintContext) walkEntitySchemaMap(nodeID dsl.NodeID, path string, value any) {
	properties, ok := entitySchemaObject(value)
	if !ok {
		return
	}
	for name, child := range properties {
		c.walkEntitySchemaValue(nodeID, appendSchemaPath(path, name), child)
	}
}

func (c *lintContext) walkEntitySchemaValue(nodeID dsl.NodeID, path string, value any) {
	schema, ok := entitySchemaObject(value)
	if !ok {
		return
	}
	c.walkEntityKindSchema(nodeID, path, schema)
}

func entitySchemaObject(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case dsl.Schema:
		return map[string]any(typed), true
	case refs.Schema:
		return map[string]any(typed), true
	default:
		return nil, false
	}
}

func entitySchemaList(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case []dsl.Schema:
		values := make([]any, len(typed))
		for index := range typed {
			values[index] = typed[index]
		}
		return values, true
	case []refs.Schema:
		values := make([]any, len(typed))
		for index := range typed {
			values[index] = typed[index]
		}
		return values, true
	default:
		return nil, false
	}
}

func (c *lintContext) addNodePath(
	nodeID dsl.NodeID,
	path string,
	code string,
	format string,
	args ...any,
) {
	c.errors = append(c.errors, LintError{
		NodeID: nodeID, Path: strings.TrimSpace(path), Code: code,
		Message: fmt.Sprintf(format, args...), Severity: SeverityError,
	})
}

func appendSchemaPath(path string, segment string) string {
	if value := strings.TrimSpace(path); value != "" {
		return value + "." + segment
	}
	return segment
}

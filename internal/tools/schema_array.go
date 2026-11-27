package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// JSON Schema keywords that describe array elements.
const (
	schemaKeyItems           = "items"
	schemaKeyPrefixItems     = "prefixItems"
	schemaKeyAdditionalItems = "additionalItems"
)

// schemaArrayItems holds the positional and trailing item schemas of an array schema.
// JSON Schema 2020-12 spells a tuple as prefixItems plus items for the rest; draft-07
// spells it as an items array plus additionalItems.
type schemaArrayItems struct {
	prefixKey string
	prefix    []map[string]json.RawMessage
	restKey   string
	rest      map[string]json.RawMessage
	hasRest   bool
}

func decodeSchemaArrayItems(schema map[string]json.RawMessage) (schemaArrayItems, error) {
	items := schemaArrayItems{prefixKey: schemaKeyPrefixItems, restKey: schemaKeyItems}
	if isTupleItems(schema[schemaKeyItems]) {
		items.prefixKey, items.restKey = schemaKeyItems, schemaKeyAdditionalItems
	}
	prefix, err := schemaNodeArray(schema[items.prefixKey])
	if err != nil {
		return items, fmt.Errorf("%s: %w", items.prefixKey, err)
	}
	rest, hasRest, err := schemaNode(schema[items.restKey])
	if err != nil {
		return items, fmt.Errorf("%s: %w", items.restKey, err)
	}
	items.prefix, items.rest, items.hasRest = prefix, rest, hasRest
	return items, nil
}

// isTupleItems reports the draft-07 tuple form of items, the only form additionalItems constrains.
func isTupleItems(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '['
}

// at returns the schema that governs the array element at idx.
func (s schemaArrayItems) at(idx int) (map[string]json.RawMessage, bool) {
	if idx < len(s.prefix) {
		return s.prefix[idx], true
	}
	return s.rest, s.hasRest
}

func (v schemaValidator) validateSchemaArrayNode(path string, schema map[string]json.RawMessage, value any) error {
	arrayValue, ok := value.([]any)
	if !ok {
		return nil
	}
	minItems, ok, err := schemaNonNegativeInteger(schema["minItems"])
	if err != nil {
		return fmt.Errorf("%s.minItems: %w", path, err)
	}
	if ok && len(arrayValue) < minItems {
		return schemaViolation(path, "array length %d is less than minItems %d", len(arrayValue), minItems)
	}
	items, err := decodeSchemaArrayItems(schema)
	if err != nil {
		return fmt.Errorf("%s.%w", path, err)
	}
	for idx, item := range arrayValue {
		node, ok := items.at(idx)
		if !ok {
			continue
		}
		if err := v.validateSchemaNode(fmt.Sprintf("%s[%d]", path, idx), node, item); err != nil {
			return err
		}
	}
	return nil
}

func (v schemaValidator) validateJSONSchemaDocumentItems(path string, schema map[string]json.RawMessage) error {
	items, err := decodeSchemaArrayItems(schema)
	if err != nil {
		return fmt.Errorf("%s.%w", path, err)
	}
	for idx, node := range items.prefix {
		if err := v.validateJSONSchemaDocument(fmt.Sprintf("%s.%s[%d]", path, items.prefixKey, idx), node); err != nil {
			return err
		}
	}
	if items.hasRest {
		return v.validateJSONSchemaDocument(path+"."+items.restKey, items.rest)
	}
	return nil
}

package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

func validateSchemaEnum(path string, raw json.RawMessage, value any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var allowed []any
	if err := json.Unmarshal(raw, &allowed); err != nil {
		return fmt.Errorf("%s.enum: %w", path, err)
	}
	for _, candidate := range allowed {
		// JSON enum members are runtime-shaped maps, slices, numbers, and scalars;
		// comparable-only equality cannot express this contract. BenchmarkValidateToolSpec owns the cost.
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

package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
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
	return schemaViolation(path, "value is not allowed")
}

func (v schemaValidator) validateSchemaAllOf(path string, raw json.RawMessage, value any) error {
	nodes, err := schemaNodeArray(raw)
	if err != nil {
		return fmt.Errorf("%s.allOf: %w", path, err)
	}
	for idx, node := range nodes {
		if err := v.validateSchemaNode(fmt.Sprintf("%s.allOf[%d]", path, idx), node, value); err != nil {
			return err
		}
	}
	return nil
}

func (v schemaValidator) validateSchemaAnyOf(path string, raw json.RawMessage, value any) error {
	nodes, err := schemaNodeArray(raw)
	if err != nil {
		return fmt.Errorf("%s.anyOf: %w", path, err)
	}
	if len(nodes) == 0 {
		return nil
	}
	for idx, node := range nodes {
		if err := v.validateSchemaNode(fmt.Sprintf("%s.anyOf[%d]", path, idx), node, value); err == nil {
			return nil
		}
	}
	return schemaViolation(path, "value must match at least one anyOf schema")
}

func (v schemaValidator) validateSchemaOneOf(path string, raw json.RawMessage, value any) error {
	nodes, err := schemaNodeArray(raw)
	if err != nil {
		return fmt.Errorf("%s.oneOf: %w", path, err)
	}
	if len(nodes) == 0 {
		return nil
	}
	matches := 0
	for idx, node := range nodes {
		if err := v.validateSchemaNode(fmt.Sprintf("%s.oneOf[%d]", path, idx), node, value); err == nil {
			matches++
		}
	}
	if matches != 1 {
		return schemaViolation(path, "value matched %d oneOf schemas, want exactly one", matches)
	}
	return nil
}

func (v schemaValidator) validateSchemaNot(path string, raw json.RawMessage, value any) error {
	node, ok, err := schemaNode(raw)
	if err != nil {
		return fmt.Errorf("%s.not: %w", path, err)
	}
	if !ok {
		return nil
	}
	if err := v.validateSchemaNode(path+".not", node, value); err == nil {
		return schemaViolation(path, "value matched forbidden schema")
	}
	return nil
}

// schemaValidationError contains only declared schema paths and authored constraints.
// Submitted values and unknown property names must never enter operator diagnostics.
type schemaValidationError struct{ message string }

func (e *schemaValidationError) Error() string { return e.message }

func schemaViolation(path, format string, args ...any) error {
	path = strings.ToValidUTF8(path, "?")
	if runes := []rune(path); len(runes) > 128 {
		path = string(runes[:128]) + "..."
	}
	return &schemaValidationError{message: fmt.Sprintf("%q: %s", path, fmt.Sprintf(format, args...))}
}

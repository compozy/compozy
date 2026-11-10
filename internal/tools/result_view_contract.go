package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

func validateResultViews(d Descriptor) error {
	views := d.ResultViews()
	var input struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(d.InputSchema, &input); err != nil {
		return err
	}
	selector, exists := input.Properties[ResultViewInputKey]
	if views == nil {
		if d.Backend.Kind == BackendNativeGo && exists {
			return NewValidationError(
				"input_schema.properties.result_view",
				ReasonSchemaInvalid,
				"result_view requires output_views",
			)
		}
		return nil
	}
	if d.Backend.Kind != BackendNativeGo {
		return NewValidationError("output_views", ReasonSchemaInvalid, "result views require native_go")
	}
	if !exists || !equalSchema(selector, json.RawMessage(ResultViewSelectorSchema)) ||
		slices.Contains(input.Required, ResultViewInputKey) {
		return NewValidationError(
			"input_schema.properties.result_view",
			ReasonSchemaInvalid,
			"result_view must be the optional compact/full selector",
		)
	}
	for name, schema := range map[string]json.RawMessage{"compact": views.Compact.Schema, "full": views.Full.Schema} {
		if err := ValidateJSONObject("output_views."+name+".schema", schema, true); err != nil {
			return err
		}
	}
	if !equalSchema(d.OutputSchema, views.Compact.Schema) {
		return NewValidationError("output_schema", ReasonSchemaInvalid, "output_schema must equal the compact schema")
	}
	var compact map[string]json.RawMessage
	if err := json.Unmarshal(views.Compact.Schema, &compact); err != nil {
		return err
	}
	return validateClosedResultSchema("output_views.compact.schema", compact)
}

func equalSchema(left, right json.RawMessage) bool {
	a, err := CanonicalJSON(left)
	if err != nil {
		return false
	}
	b, err := CanonicalJSON(right)
	return err == nil && bytes.Equal(a, b)
}

// x-compozy-free-form explicitly marks maps whose keys belong to the tool's requested content.
func validateClosedResultSchema(path string, node map[string]json.RawMessage) error {
	types, err := schemaTypes(node["type"])
	if err != nil {
		return NewValidationError(path, ReasonSchemaInvalid, err.Error())
	}
	if slices.Contains(types, "object") ||
		(len(types) == 0 && len(node["$ref"]) == 0 &&
			len(node["anyOf"]) == 0 && len(node["oneOf"]) == 0 && len(node["allOf"]) == 0 &&
			len(node["enum"]) == 0 && len(node["const"]) == 0) {
		if !additionalPropertiesFalse(node["additionalProperties"]) &&
			!bytes.Equal(node["x-compozy-free-form"], []byte("true")) {
			return NewValidationError(
				path,
				ReasonSchemaInvalid,
				"compact objects must be closed or explicitly marked x-compozy-free-form",
			)
		}
	}
	return validateClosedResultChildren(path, node)
}

func validateClosedResultChildren(path string, node map[string]json.RawMessage) error {
	for _, key := range []string{"properties", "$defs", "definitions", "patternProperties"} {
		children, err := schemaProperties(node[key])
		if err != nil {
			return NewValidationError(path+"."+key, ReasonSchemaInvalid, err.Error())
		}
		for name, child := range children {
			if err := validateClosedResultSchema(path+"."+key+"."+name, child); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"items", "additionalProperties", "not", "if", "then", "else"} {
		raw := bytes.TrimSpace(node[key])
		if len(raw) == 0 || raw[0] != '{' {
			continue
		}
		child, _, err := schemaNode(raw)
		if err != nil {
			return err
		}
		if err := validateClosedResultSchema(path+"."+key, child); err != nil {
			return err
		}
	}
	for _, key := range []string{"anyOf", "oneOf", "allOf", "prefixItems"} {
		children, err := schemaNodeArray(node[key])
		if err != nil {
			return err
		}
		for i, child := range children {
			if err := validateClosedResultSchema(fmt.Sprintf("%s.%s[%d]", path, key, i), child); err != nil {
				return err
			}
		}
	}
	return nil
}

func populateResultViewDigests(d *Descriptor) error {
	views := d.ResultViews()
	if views == nil {
		return nil
	}
	for name, contract := range map[string]*ResultViewContract{"compact": &views.Compact, "full": &views.Full} {
		digest, err := SchemaDigest(contract.Schema)
		if err != nil {
			return wrapField(err, "output_views."+name+".schema")
		}
		if existing := strings.TrimSpace(contract.Digest); existing != "" && existing != digest {
			return NewValidationError(
				"output_views."+name+".digest",
				ReasonRuntimeDescriptorMismatch,
				"view digest does not match schema",
			)
		}
		contract.Digest = digest
	}
	d.SetResultViews(views)
	return nil
}

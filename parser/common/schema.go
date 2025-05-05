package common

import (
	"bytes"
	"encoding/json"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Schema represents a JSON schema
type Schema struct {
	Type        string                 `json:"type" yaml:"type"`
	Properties  map[string]interface{} `json:"properties,omitempty" yaml:"properties,omitempty"`
	Required    []string               `json:"required,omitempty" yaml:"required,omitempty"`
	Items       *Schema                `json:"items,omitempty" yaml:"items,omitempty"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Enum        []string               `json:"enum,omitempty" yaml:"enum,omitempty"`
}

// Validate validates a JSON schema
func (s *Schema) Validate() error {
	if s == nil {
		return nil
	}

	// Convert schema to map for validation
	schemaMap := map[string]interface{}{
		"type": s.Type,
	}
	if s.Properties != nil {
		schemaMap["properties"] = s.Properties
	}
	if s.Required != nil {
		schemaMap["required"] = s.Required
	}
	if s.Items != nil {
		schemaMap["items"] = s.Items
	}
	if s.Description != "" {
		schemaMap["description"] = s.Description
	}
	if s.Enum != nil {
		schemaMap["enum"] = s.Enum
	}

	// Convert schema to JSON
	schemaJSON, err := json.Marshal(schemaMap)
	if err != nil {
		return err
	}

	// Compile and validate the schema
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", bytes.NewReader(schemaJSON)); err != nil {
		return err
	}
	if _, err := compiler.Compile("schema.json"); err != nil {
		return err
	}

	return nil
}

// InputSchema represents the input schema for a component
type InputSchema struct {
	Schema `yaml:",inline"`
}

// Validate validates the input schema
func (s *InputSchema) Validate() error {
	return s.Schema.Validate()
}

// OutputSchema represents the output schema for a component
type OutputSchema struct {
	Schema `yaml:",inline"`
}

// Validate validates the output schema
func (s *OutputSchema) Validate() error {
	return s.Schema.Validate()
}

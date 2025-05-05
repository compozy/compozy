package common

import (
	"github.com/xeipuuv/gojsonschema"
)

// Schema represents a JSON schema
type Schema struct {
	Schema map[string]interface{} `json:"schema" yaml:"schema"`
}

// Validate validates a JSON schema
func (s *Schema) Validate() error {
	if s.Schema == nil {
		return nil
	}

	// Convert schema to JSON string
	schemaLoader := gojsonschema.NewGoLoader(s.Schema)

	// Validate the schema itself
	_, err := gojsonschema.NewSchema(schemaLoader)
	if err != nil {
		return err
	}

	return nil
}

// InputSchema represents the input schema for a component
type InputSchema Schema

// Validate validates the input schema
func (s *InputSchema) Validate() error {
	return (*Schema)(s).Validate()
}

// OutputSchema represents the output schema for a component
type OutputSchema Schema

// Validate validates the output schema
func (s *OutputSchema) Validate() error {
	return (*Schema)(s).Validate()
}

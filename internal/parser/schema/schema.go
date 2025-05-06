package schema

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// SchemaValidatorError represents errors that can occur during schema validation
type SchemaValidatorError struct {
	Message string
	Code    string
}

func (e *SchemaValidatorError) Error() string {
	return e.Message
}

// Error codes
const (
	ErrCodeInvalidPackageRef      = "INVALID_PACKAGE_REF"
	ErrCodeInputSchemaNotAllowed  = "INPUT_SCHEMA_NOT_ALLOWED"
	ErrCodeOutputSchemaNotAllowed = "OUTPUT_SCHEMA_NOT_ALLOWED"
	ErrCodeInvalidSchemaType      = "INVALID_SCHEMA_TYPE"
	ErrCodeMissingSchemaProps     = "MISSING_SCHEMA_PROPERTIES"
)

// Error messages
const (
	ErrMsgInvalidPackageRef      = "Invalid package reference: %s"
	ErrMsgInputSchemaNotAllowed  = "Input schema not allowed for reference type %s"
	ErrMsgOutputSchemaNotAllowed = "Output schema not allowed for reference type %s"
	ErrMsgInvalidSchemaType      = "Schema type must be object"
	ErrMsgMissingSchemaProps     = "Schema must have properties"
)

// SchemaValidator validates input and output schemas
type SchemaValidator struct {
	pkgRef       *pkgref.PackageRefConfig
	inputSchema  *InputSchema
	outputSchema *OutputSchema
}

// NewSchemaValidator creates a new SchemaValidator
func NewSchemaValidator(pkgRef *pkgref.PackageRefConfig, inputSchema *InputSchema, outputSchema *OutputSchema) *SchemaValidator {
	return &SchemaValidator{
		pkgRef:       pkgRef,
		inputSchema:  inputSchema,
		outputSchema: outputSchema,
	}
}

func (v *SchemaValidator) validateSchema(schema *Schema, isTopLevel bool) error {
	if schema == nil {
		return nil
	}

	// Only validate object type and properties for top-level schemas
	if isTopLevel {
		if schema.GetType() != "object" {
			return &SchemaValidatorError{
				Code:    ErrCodeInvalidSchemaType,
				Message: ErrMsgInvalidSchemaType,
			}
		}

		if schema.GetProperties() == nil {
			return &SchemaValidatorError{
				Code:    ErrCodeMissingSchemaProps,
				Message: ErrMsgMissingSchemaProps,
			}
		}
	}

	// For object types, validate nested properties
	if schema.GetType() == "object" && schema.GetProperties() != nil {
		for propName, propSchema := range schema.GetProperties() {
			if propSchema == nil {
				return &SchemaValidatorError{
					Code:    ErrCodeInvalidSchemaType,
					Message: fmt.Sprintf("Property %s has nil schema", propName),
				}
			}
			// Recursively validate nested schemas, but they are not top-level
			if err := v.validateSchema(propSchema, false); err != nil {
				return err
			}
		}
	}

	return nil
}

// Validate implements the Validator interface
func (v *SchemaValidator) Validate() error {
	// First validate package reference if it exists
	if v.pkgRef != nil {
		ref, err := pkgref.Parse(string(*v.pkgRef))
		if err != nil {
			return &SchemaValidatorError{
				Code:    ErrCodeInvalidPackageRef,
				Message: fmt.Sprintf(ErrMsgInvalidPackageRef, err.Error()),
			}
		}

		switch ref.Type.Type {
		case "id", "dep", "file":
			if v.inputSchema != nil {
				return &SchemaValidatorError{
					Code:    ErrCodeInputSchemaNotAllowed,
					Message: fmt.Sprintf(ErrMsgInputSchemaNotAllowed, ref.Type.Type),
				}
			}
			if v.outputSchema != nil {
				return &SchemaValidatorError{
					Code:    ErrCodeOutputSchemaNotAllowed,
					Message: fmt.Sprintf(ErrMsgOutputSchemaNotAllowed, ref.Type.Type),
				}
			}
		}
	}

	// Then validate schema structure if schemas exist
	if v.inputSchema != nil {
		if err := v.validateSchema(&v.inputSchema.Schema, true); err != nil {
			return err
		}
	}
	if v.outputSchema != nil {
		if err := v.validateSchema(&v.outputSchema.Schema, true); err != nil {
			return err
		}
	}

	return nil
}

// Validate validates a value against the schema using jsonschema/v5
func (s *Schema) Validate(value any) error {
	if s == nil {
		return nil
	}

	// Convert schema to JSON for jsonschema
	schemaJSON, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	// Compile the schema
	schema, err := jsonschema.CompileString("schema.json", string(schemaJSON))
	if err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}

	// Perform validation
	if err := schema.Validate(value); err != nil {
		return err
	}

	return nil
}

func (s *Schema) GetType() string {
	if typ, ok := (*s)["type"].(string); ok {
		return typ
	}
	return ""
}

func (s *Schema) GetProperties() map[string]*Schema {
	props, ok := (*s)["properties"].(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]*Schema)
	for k, v := range props {
		if schema, ok := v.(map[string]any); ok {
			s := Schema(schema)
			result[k] = &s
		}
	}
	return result
}

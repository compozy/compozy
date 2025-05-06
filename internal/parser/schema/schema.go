package schema

import (
	"fmt"

	"github.com/compozy/compozy/internal/parser/pkgref"
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

// validateSchema validates the basic structure of a schema
func (v *SchemaValidator) validateSchema(schema *Schema) error {
	if schema == nil {
		return nil
	}

	// Validate that the schema is an object
	if schema.Type != "object" {
		return &SchemaValidatorError{
			Code:    ErrCodeInvalidSchemaType,
			Message: ErrMsgInvalidSchemaType,
		}
	}

	// Validate that the schema has properties
	if schema.Properties == nil {
		return &SchemaValidatorError{
			Code:    ErrCodeMissingSchemaProps,
			Message: ErrMsgMissingSchemaProps,
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
		if err := v.validateSchema(&v.inputSchema.Schema); err != nil {
			return err
		}
	}
	if v.outputSchema != nil {
		if err := v.validateSchema(&v.outputSchema.Schema); err != nil {
			return err
		}
	}

	return nil
}

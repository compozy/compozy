package schema

import (
	"fmt"

	"github.com/compozy/compozy/internal/parser/pkgref"
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

// Validate implements the Validator interface
func (v *SchemaValidator) Validate() error {
	// First validate package reference if it exists
	if v.pkgRef != nil {
		ref, err := v.pkgRef.IntoRef()
		if err != nil {
			return fmt.Errorf("invalid package reference: %w", err)
		}

		switch ref.Type.Type {
		case "id", "dep", "file":
			if v.inputSchema != nil {
				return fmt.Errorf("input schema not allowed for reference type %s", ref.Type.Type)
			}
			if v.outputSchema != nil {
				return fmt.Errorf("output schema not allowed for reference type %s", ref.Type.Type)
			}
		}
	}

	return nil
}

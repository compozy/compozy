package agent

import (
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/compozy/compozy/internal/parser/schema"
)

// -----------------------------------------------------------------------------
// ActionsValidator
// -----------------------------------------------------------------------------

type ActionsValidator struct {
	actions []*ActionConfig
}

func NewActionsValidator(actions []*ActionConfig) *ActionsValidator {
	return &ActionsValidator{actions: actions}
}

func (v *ActionsValidator) Validate() error {
	if v.actions == nil {
		return nil
	}
	for _, action := range v.actions {
		if err := action.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// PackageRefValidator
// -----------------------------------------------------------------------------

type PackageRefValidator struct {
	cwd    string
	pkgRef *pkgref.PackageRefConfig
}

func NewPackageRefValidator(pkgRef *pkgref.PackageRefConfig, cwd string) *PackageRefValidator {
	return &PackageRefValidator{
		cwd:    cwd,
		pkgRef: pkgRef,
	}
}

func (v *PackageRefValidator) Validate() error {
	if v.cwd == "" {
		return fmt.Errorf("cwd is required")
	}
	if v.pkgRef == nil {
		return nil
	}
	ref, err := v.pkgRef.IntoRef()
	if err != nil {
		return fmt.Errorf("invalid package reference: %w", err)
	}
	if !ref.Component.IsAgent() {
		return fmt.Errorf("invalid package reference: %w", errors.New("invalid component type"))
	}
	if err := ref.Type.Validate(v.cwd); err != nil {
		return fmt.Errorf("invalid package reference: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// SchemaValidator
// -----------------------------------------------------------------------------

type SchemaValidator struct {
	pkgRef       *pkgref.PackageRefConfig
	inputSchema  *schema.InputSchema
	outputSchema *schema.OutputSchema
}

func NewSchemaValidator(pkgRef *pkgref.PackageRefConfig, inputSchema *schema.InputSchema, outputSchema *schema.OutputSchema) *SchemaValidator {
	return &SchemaValidator{
		pkgRef:       pkgRef,
		inputSchema:  inputSchema,
		outputSchema: outputSchema,
	}
}

func (v *SchemaValidator) Validate() error {
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

package tool

import (
	"errors"
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/schema"
)

// -----------------------------------------------------------------------------
// ExecuteValidator
// -----------------------------------------------------------------------------

type ExecuteValidator struct {
	execute string
	cwd     *common.CWD
	id      string
}

func NewExecuteValidator(execute string, cwd *common.CWD) *ExecuteValidator {
	return &ExecuteValidator{execute: execute, cwd: cwd}
}

func (v *ExecuteValidator) WithID(id string) *ExecuteValidator {
	v.id = id
	return v
}

func (v *ExecuteValidator) Validate() error {
	if v.execute == "" {
		return nil
	}

	if !IsTypeScript(v.execute) {
		if v.id == "" {
			return fmt.Errorf("tool ID is required for TypeScript execution")
		}
		return fmt.Errorf("invalid typescript file: %s", v.execute)
	}

	_, err := v.cwd.JoinAndCheck(v.execute)
	if err != nil {
		return err
	}
	return nil
}

// -----------------------------------------------------------------------------
// PackageRefValidator
// -----------------------------------------------------------------------------

type PackageRefValidator struct {
	cwd    string
	pkgRef *common.PackageRefConfig
}

func NewPackageRefValidator(pkgRef *common.PackageRefConfig, cwd string) *PackageRefValidator {
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
	if !ref.Component.IsTool() {
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
	pkgRef       *common.PackageRefConfig
	inputSchema  *schema.InputSchema
	outputSchema *schema.OutputSchema
}

func NewSchemaValidator(
	pkgRef *common.PackageRefConfig,
	inputSchema *schema.InputSchema,
	outputSchema *schema.OutputSchema,
) *SchemaValidator {
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
			if ref.Component.IsTool() {
				if v.inputSchema != nil {
					return fmt.Errorf("input schema not allowed for reference type %s", ref.Type.Type)
				}
				if v.outputSchema != nil {
					return fmt.Errorf("output schema not allowed for reference type %s", ref.Type.Type)
				}
			}
		}
	}

	return nil
}

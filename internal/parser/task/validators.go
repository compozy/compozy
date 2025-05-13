package task

import (
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/compozy/compozy/internal/parser/schema"
)

// -----------------------------------------------------------------------------
// TaskTypeValidator
// -----------------------------------------------------------------------------

type TypeValidator struct {
	pkgRef    *pkgref.PackageRefConfig
	taskType  Type
	action    string
	condition string
	routes    map[string]string
}

func NewTaskTypeValidator(
	pkgRef *pkgref.PackageRefConfig,
	taskType Type,
	action string,
	condition string,
	routes map[string]string,
) *TypeValidator {
	return &TypeValidator{
		pkgRef:    pkgRef,
		taskType:  taskType,
		action:    action,
		condition: condition,
		routes:    routes,
	}
}

func (v *TypeValidator) Validate() error {
	if v.taskType == "" {
		return nil
	}

	ref, err := v.getPackageRef()
	if err != nil {
		return err
	}

	if err := v.validateTaskType(); err != nil {
		return err
	}

	if ref != nil {
		if err := v.validateBasicTaskWithRef(ref); err != nil {
			return err
		}
	}

	if v.taskType == TaskTypeDecision {
		if err := v.validateDecisionTask(); err != nil {
			return err
		}
	}

	return nil
}

func (v *TypeValidator) getPackageRef() (*pkgref.PackageRef, error) {
	if v.pkgRef != nil {
		return v.pkgRef.IntoRef()
	}
	return nil, nil
}

func (v *TypeValidator) validateTaskType() error {
	if v.taskType != TaskTypeBasic && v.taskType != TaskTypeDecision {
		return fmt.Errorf("invalid task type: %s", v.taskType)
	}
	return nil
}

func (v *TypeValidator) validateBasicTaskWithRef(ref *pkgref.PackageRef) error {
	if v.taskType != TaskTypeBasic {
		return nil
	}
	isTask := ref.Component.IsTask()
	isAgent := ref.Component.IsAgent()
	isTool := ref.Component.IsTool()
	if (isTask || isTool) && v.action != "" {
		return fmt.Errorf("action is not allowed when referencing a task or tool")
	}
	if isAgent && v.action == "" {
		return fmt.Errorf("action is required when referencing an agent")
	}
	if ref.Component.IsTool() && v.action != "" {
		return fmt.Errorf("action is not allowed when referencing a tool")
	}
	return nil
}

func (v *TypeValidator) validateDecisionTask() error {
	if v.condition == "" && len(v.routes) == 0 {
		return fmt.Errorf("condition or routes are required for decision task type")
	}
	if len(v.routes) == 0 {
		return fmt.Errorf("decision task must have at least one route")
	}
	for _, route := range v.routes {
		if route == "" {
			return fmt.Errorf("route cannot be empty")
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
	if !ref.Component.IsTask() && !ref.Component.IsTool() && !ref.Component.IsAgent() {
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

func NewSchemaValidator(
	pkgRef *pkgref.PackageRefConfig,
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
			if ref.Component.IsTask() {
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

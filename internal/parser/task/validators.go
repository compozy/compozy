package task

import (
	"github.com/compozy/compozy/internal/parser/agent"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/package_ref"
)

// CWDValidator validates the current working directory
type CWDValidator struct {
	cwd *common.CWD
}

func NewCWDValidator(cwd *common.CWD) *CWDValidator {
	return &CWDValidator{cwd: cwd}
}

func (v *CWDValidator) Validate() error {
	if v.cwd == nil || v.cwd.Get() == "" {
		return NewMissingPathError()
	}
	return nil
}

// PackageRefValidator validates the package reference
type PackageRefValidator struct {
	pkgRef *package_ref.PackageRefConfig
	cwd    *common.CWD
}

func NewPackageRefValidator(pkgRef *package_ref.PackageRefConfig, cwd *common.CWD) *PackageRefValidator {
	return &PackageRefValidator{pkgRef: pkgRef, cwd: cwd}
}

func (v *PackageRefValidator) Validate() error {
	if v.pkgRef == nil {
		return nil
	}
	ref, err := package_ref.Parse(string(*v.pkgRef))
	if err != nil {
		return NewInvalidPackageRefError(err)
	}
	if !ref.Component.IsTask() && !ref.Component.IsAgent() && !ref.Component.IsTool() {
		return NewInvalidTypeError()
	}
	if err := ref.Type.Validate(v.cwd.Get()); err != nil {
		return NewInvalidPackageRefError(err)
	}
	return nil
}

// SchemaValidator validates input/output schemas
type SchemaValidator struct {
	schema interface{ Validate() error }
}

func NewSchemaValidator(schema interface{ Validate() error }) *SchemaValidator {
	return &SchemaValidator{schema: schema}
}

func (v *SchemaValidator) Validate() error {
	if v.schema == nil {
		return nil
	}
	if s, ok := v.schema.(*common.InputSchema); ok && s == nil {
		return nil
	}
	if s, ok := v.schema.(*common.OutputSchema); ok && s == nil {
		return nil
	}
	return v.schema.Validate()
}

// TaskTypeValidator validates the task type and its configuration
type TaskTypeValidator struct {
	taskType  TaskType
	action    *agent.ActionID
	condition TaskCondition
	routes    map[TaskRoute]TaskRoute
}

func NewTaskTypeValidator(taskType TaskType, action *agent.ActionID, condition TaskCondition, routes map[TaskRoute]TaskRoute) *TaskTypeValidator {
	return &TaskTypeValidator{
		taskType:  taskType,
		action:    action,
		condition: condition,
		routes:    routes,
	}
}

func (v *TaskTypeValidator) Validate() error {
	if v.taskType == "" {
		return nil
	}
	switch v.taskType {
	case TaskTypeBasic:
		if v.action == nil {
			return NewInvalidTaskTypeError("Basic task configuration is required for basic task type")
		}
	case TaskTypeDecision:
		if v.condition == "" && len(v.routes) == 0 {
			return NewInvalidTaskTypeError("Decision task configuration is required for decision task type")
		}
		if len(v.routes) == 0 {
			return NewInvalidDecisionTaskError()
		}
	default:
		return NewInvalidTaskTypeError(string(v.taskType))
	}
	return nil
}

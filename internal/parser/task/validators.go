package task

import (
	"github.com/compozy/compozy/internal/parser/agent"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/package_ref"
)

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

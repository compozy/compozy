package task

import (
	"fmt"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/pkgref"
)

// PackageRefValidator validates the package reference
type PackageRefValidator struct {
	pkgRef *pkgref.PackageRefConfig
	cwd    *common.CWD
}

func NewPackageRefValidator(pkgRef *pkgref.PackageRefConfig, cwd *common.CWD) *PackageRefValidator {
	return &PackageRefValidator{pkgRef: pkgRef, cwd: cwd}
}

func (v *PackageRefValidator) Validate() error {
	if v.pkgRef == nil {
		return nil
	}
	ref, err := pkgref.Parse(string(*v.pkgRef))
	if err != nil {
		return fmt.Errorf("invalid package reference: %w", err)
	}
	if !ref.Component.IsTask() && !ref.Component.IsAgent() && !ref.Component.IsTool() {
		return fmt.Errorf("package reference must be a task, agent, or tool")
	}
	if err := ref.Type.Validate(v.cwd.Get()); err != nil {
		return fmt.Errorf("invalid package reference: %w", err)
	}
	return nil
}

// TaskTypeValidator validates the task type and its configuration
type TaskTypeValidator struct {
	taskType  TaskType
	action    string
	condition string
	routes    map[string]string
}

func NewTaskTypeValidator(taskType TaskType, action string, condition string, routes map[string]string) *TaskTypeValidator {
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
		if v.action == "" {
			return fmt.Errorf("invalid task type: Basic task configuration is required for basic task type")
		}
	case TaskTypeDecision:
		if v.condition == "" && len(v.routes) == 0 {
			return fmt.Errorf("invalid task type: Decision task configuration is required for decision task type")
		}
		if len(v.routes) == 0 {
			return fmt.Errorf("decision task must have at least one route")
		}
	default:
		return fmt.Errorf("invalid task type: %s", string(v.taskType))
	}
	return nil
}

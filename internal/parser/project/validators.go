package project

import (
	"github.com/compozy/compozy/internal/parser/common"
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

// WorkflowsValidator validates the workflows configuration
type WorkflowsValidator struct {
	workflows []*WorkflowSourceConfig
}

func NewWorkflowsValidator(workflows []*WorkflowSourceConfig) *WorkflowsValidator {
	return &WorkflowsValidator{workflows: workflows}
}

func (v *WorkflowsValidator) Validate() error {
	if len(v.workflows) == 0 {
		return NewNoWorkflowsError()
	}
	return nil
}

// EnvironmentsValidator validates the environments configuration
type EnvironmentsValidator struct {
	environments map[string]*EnvironmentConfig
}

func NewEnvironmentsValidator(environments map[string]*EnvironmentConfig) *EnvironmentsValidator {
	return &EnvironmentsValidator{environments: environments}
}

func (v *EnvironmentsValidator) Validate() error {
	if v.environments == nil {
		return nil
	}
	for name, env := range v.environments {
		if env == nil {
			return NewInvalidEnvironmentError(name)
		}
		if err := env.Validate(); err != nil {
			return NewInvalidEnvironmentError(name)
		}
	}
	return nil
}

// EnvironmentValidator validates a single environment configuration
func (e *EnvironmentConfig) Validate() error {
	if e.LogLevel == "" {
		return NewInvalidLogLevelError(string(e.LogLevel))
	}
	if !IsValidLogLevel(e.LogLevel) {
		return NewInvalidLogLevelError(string(e.LogLevel))
	}
	return nil
}

// DependenciesValidator validates the dependencies configuration
type DependenciesValidator struct {
	deps *Dependencies
}

func NewDependenciesValidator(deps *Dependencies) *DependenciesValidator {
	return &DependenciesValidator{deps: deps}
}

func (v *DependenciesValidator) Validate() error {
	if v.deps == nil {
		return nil
	}
	return v.deps.Validate()
}

// Validate validates the dependencies
func (d *Dependencies) Validate() error {
	// Add any specific dependency validation logic here
	return nil
}

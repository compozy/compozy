package task

import (
	"fmt"
)

// -----------------------------------------------------------------------------
// TaskTypeValidator
// -----------------------------------------------------------------------------

type TypeValidator struct {
	config *Config
}

func NewTaskTypeValidator(config *Config) *TypeValidator {
	return &TypeValidator{
		config: config,
	}
}

func (v *TypeValidator) Validate() error {
	if v.config.Type == "" {
		return nil
	}
	if v.config.Type != TaskTypeBasic && v.config.Type != TaskTypeDecision {
		return fmt.Errorf("invalid task type: %s", v.config.Type)
	}
	if err := v.validateBasicTaskWithRef(); err != nil {
		return err
	}
	if v.config.Type == TaskTypeDecision {
		if err := v.validateDecisionTask(); err != nil {
			return err
		}
	}
	return nil
}

func (v *TypeValidator) validateBasicTaskWithRef() error {
	if v.config.Type != TaskTypeBasic {
		return nil
	}
	if v.config.GetTool() != nil && v.config.Action != "" {
		return fmt.Errorf("action is not allowed when executor type is tool")
	}
	if v.config.GetAgent() != nil && v.config.Action == "" {
		return fmt.Errorf("action is required when executor type is agent")
	}
	return nil
}

func (v *TypeValidator) validateDecisionTask() error {
	if v.config.Condition == "" {
		return fmt.Errorf("condition is required for decision tasks")
	}
	if len(v.config.Routes) == 0 {
		return fmt.Errorf("routes are required for decision tasks")
	}
	return nil
}

package task

import (
	"fmt"
)

// isRefEmpty checks if a reference field is empty
func isRefEmpty(ref any) bool {
	if ref == nil {
		return true
	}
	switch v := ref.(type) {
	case string:
		return v == ""
	case map[string]any:
		return len(v) == 0
	default:
		return false
	}
}

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
	if !isRefEmpty(v.config.Executor.Ref) {
		if err := v.validateBasicTaskWithRef(); err != nil {
			return err
		}
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
	executorType := v.config.Executor.Type
	if executorType == ExecutorTool && v.config.Executor.Action != "" {
		return fmt.Errorf("action is not allowed when executor type is tool")
	}
	if executorType == ExecutorAgent && v.config.Executor.Action == "" {
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

// -----------------------------------------------------------------------------
// ExecutorValidator
// -----------------------------------------------------------------------------

type ExecutorValidator struct {
	config *Config
}

func NewExecutorValidator(config *Config) *ExecutorValidator {
	return &ExecutorValidator{
		config: config,
	}
}

func (v *ExecutorValidator) Validate() error {
	if v.config.Type != TaskTypeBasic {
		return nil
	}
	if v.config.Executor.Type == "" {
		return fmt.Errorf("executor type is required")
	}
	if v.config.Executor.Type != ExecutorAgent && v.config.Executor.Type != ExecutorTool {
		return fmt.Errorf("invalid executor type: %s", v.config.Executor.Type)
	}
	if isRefEmpty(v.config.Executor.Ref) {
		return fmt.Errorf("executor reference is required")
	}
	if v.config.Executor.Type == ExecutorAgent {
		if agent, err := v.config.Executor.GetAgent(); err == nil && agent != nil {
			if err := agent.Validate(); err != nil {
				return fmt.Errorf("resolved agent config is invalid: %w", err)
			}
		}
	}
	if v.config.Executor.Type == ExecutorTool {
		if tool, err := v.config.Executor.GetTool(); err == nil && tool != nil {
			if err := tool.Validate(); err != nil {
				return fmt.Errorf("resolved tool config is invalid: %w", err)
			}
		}
	}
	return nil
}

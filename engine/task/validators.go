package task

import (
	"errors"
	"fmt"
	"strings"
)

// -----------------------------------------------------------------------------
// CycleValidator - Detects circular dependencies in task references
// -----------------------------------------------------------------------------

type CycleValidator struct {
	visited  map[string]bool
	visiting map[string]bool
}

func NewCycleValidator() *CycleValidator {
	return &CycleValidator{
		visited:  make(map[string]bool),
		visiting: make(map[string]bool),
	}
}

func (v *CycleValidator) Validate() error {
	// This is a placeholder - actual validation happens in ValidateConfig
	return nil
}

func (v *CycleValidator) ValidateConfig(config *Config) error {
	if config.ID == "" {
		return fmt.Errorf("task ID is required for cycle detection")
	}
	return v.detectCycle(config, make(map[string]bool), make(map[string]bool))
}

func (v *CycleValidator) detectCycle(config *Config, visited map[string]bool, visiting map[string]bool) error {
	taskID := config.ID
	if visiting[taskID] {
		return fmt.Errorf("circular dependency detected involving task: %s", taskID)
	}
	if visited[taskID] {
		return nil // Already processed this task
	}

	visiting[taskID] = true

	// Check parallel task dependencies
	switch config.Type {
	case TaskTypeParallel:
		for i := range config.Tasks {
			if err := v.detectCycle(&config.Tasks[i], visited, visiting); err != nil {
				return err
			}
		}

		// Check task reference if present
		if config.Task != nil {
			if err := v.detectCycle(config.Task, visited, visiting); err != nil {
				return err
			}
		}
	case TaskTypeComposite:
		for i := range config.Tasks {
			if err := v.detectCycle(&config.Tasks[i], visited, visiting); err != nil {
				return err
			}
		}
	}

	// Mark as visited and remove from visiting
	visiting[taskID] = false
	visited[taskID] = true

	return nil
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
	switch v.config.Type {
	case TaskTypeBasic:
		return v.validateExecutorFields()
	case TaskTypeRouter:
		if err := v.validateExecutorFields(); err != nil {
			return err
		}
		return v.validateRouterTask()
	case TaskTypeParallel:
		if err := v.validateExecutorFields(); err != nil {
			return err
		}
		return v.validateParallelTask()
	case TaskTypeCollection:
		if err := v.validateExecutorFields(); err != nil {
			return err
		}
		return v.validateCollectionTask()
	case TaskTypeAggregate:
		return v.validateAggregateTask()
	case TaskTypeComposite:
		if err := v.validateExecutorFields(); err != nil {
			return err
		}
		return v.validateCompositeTask()
	case TaskTypeSignal:
		return v.validateSignalTask()
	default:
		return fmt.Errorf("invalid task type: %s", v.config.Type)
	}
}

func (v *TypeValidator) validateExecutorFields() error {
	if v.config.GetTool() != nil && v.config.Action != "" {
		return fmt.Errorf("action is not allowed when executor type is tool")
	}
	if v.config.GetAgent() != nil && v.config.Action == "" {
		return fmt.Errorf("action is required when executor type is agent")
	}
	return nil
}

func (v *TypeValidator) validateRouterTask() error {
	if v.config.Condition == "" {
		return fmt.Errorf("condition is required for router tasks")
	}
	if len(v.config.Routes) == 0 {
		return fmt.Errorf("routes are required for router tasks")
	}
	return nil
}

func (v *TypeValidator) validateParallelTask() error {
	if len(v.config.Tasks) == 0 {
		return fmt.Errorf("parallel tasks must have at least one sub-task")
	}

	// Check for duplicate IDs first before validating individual items
	seen := make(map[string]bool)
	for i := range v.config.Tasks {
		task := &v.config.Tasks[i]
		if seen[task.ID] {
			return fmt.Errorf("duplicate task ID in parallel execution: %s", task.ID)
		}
		seen[task.ID] = true
	}

	// Then validate each individual task
	for i := range v.config.Tasks {
		task := &v.config.Tasks[i]
		if err := v.validateParallelTaskItem(task); err != nil {
			return fmt.Errorf("invalid parallel task item %s: %w", task.ID, err)
		}
	}

	strategy := v.config.GetStrategy()
	if strategy != StrategyWaitAll && strategy != StrategyFailFast && strategy != StrategyBestEffort &&
		strategy != StrategyRace {
		return fmt.Errorf("invalid parallel strategy: %s", strategy)
	}
	return nil
}

func (v *TypeValidator) validateParallelTaskItem(item *Config) error {
	if item.ID == "" {
		return fmt.Errorf("task item ID is required")
	}
	// Each task in parallel execution should be a valid task configuration
	if err := item.Validate(); err != nil {
		return fmt.Errorf("invalid task configuration: %w", err)
	}
	return nil
}

func (v *TypeValidator) validateCollectionTask() error {
	validator := NewCollectionValidator(v.config)
	return validator.Validate()
}

// -----------------------------------------------------------------------------
// CollectionValidator - Validates collection task configuration
// -----------------------------------------------------------------------------

type CollectionValidator struct {
	config *Config
}

func NewCollectionValidator(config *Config) *CollectionValidator {
	return &CollectionValidator{config: config}
}

func (v *CollectionValidator) Validate() error {
	if err := v.validateStructure(); err != nil {
		return err
	}
	if err := v.validateConfig(); err != nil {
		return err
	}
	if err := v.validateTaskTemplate(); err != nil {
		return err
	}
	return nil
}

// validateStructure ensures that collection tasks have exactly one of 'task' or 'tasks' configured
func (v *CollectionValidator) validateStructure() error {
	hasTask := v.config.Task != nil
	hasTasks := len(v.config.Tasks) > 0

	// Ensure exactly one is provided
	if !hasTask && !hasTasks {
		return errors.New("collection tasks must have either a 'task' template or 'tasks' array configured")
	}

	if hasTask && hasTasks {
		return errors.New(
			"collection tasks cannot have both 'task' template and 'tasks' array configured - use only one",
		)
	}

	return nil
}

// validateConfig validates collection configuration details
func (v *CollectionValidator) validateConfig() error {
	cc := &v.config.CollectionConfig

	if strings.TrimSpace(cc.Items) == "" {
		return errors.New("collection config: items field is required")
	}

	if cc.Mode != "" && !ValidateCollectionMode(string(cc.Mode)) {
		return fmt.Errorf("collection config: invalid mode '%s', must be 'parallel' or 'sequential'", cc.Mode)
	}

	if cc.Batch < 0 {
		return errors.New("collection config: batch size cannot be negative")
	}

	// Validate batch and mode compatibility
	if cc.Batch > 0 && cc.Mode == CollectionModeParallel {
		return errors.New(
			"collection config: batch size cannot be combined with parallel mode – " +
				"switch to sequential or remove batch",
		)
	}

	return nil
}

// validateTaskTemplate validates the task template if provided
func (v *CollectionValidator) validateTaskTemplate() error {
	if v.config.Task != nil {
		if err := v.config.Task.Validate(); err != nil {
			return fmt.Errorf("invalid collection task template: %w", err)
		}
	}

	return nil
}

func (v *TypeValidator) validateAggregateTask() error {
	if v.config.Outputs == nil || len(*v.config.Outputs) == 0 {
		return fmt.Errorf("aggregate tasks must have outputs defined")
	}
	// Aggregate tasks should not have action, agent, or tool
	if v.config.Action != "" {
		return fmt.Errorf("aggregate tasks cannot have an action field")
	}
	if v.config.Agent != nil {
		return fmt.Errorf("aggregate tasks cannot have an agent")
	}
	if v.config.Tool != nil {
		return fmt.Errorf("aggregate tasks cannot have a tool")
	}
	// Aggregate tasks should not have other execution-related fields
	if v.config.Sleep != "" {
		return fmt.Errorf("aggregate tasks cannot have a sleep field")
	}
	if v.config.With != nil && len(*v.config.With) > 0 {
		return fmt.Errorf("aggregate tasks cannot have a with field")
	}
	return nil
}

func (v *TypeValidator) validateCompositeTask() error {
	if len(v.config.Tasks) == 0 {
		return fmt.Errorf("composite tasks must have at least one sub-task")
	}
	// Check for duplicate IDs
	seen := make(map[string]bool)
	for i := range v.config.Tasks {
		task := &v.config.Tasks[i]
		if seen[task.ID] {
			return fmt.Errorf("duplicate task ID in composite execution: %s", task.ID)
		}
		seen[task.ID] = true
	}
	// Validate each individual task
	for i := range v.config.Tasks {
		task := &v.config.Tasks[i]
		if err := v.validateCompositeTaskItem(task); err != nil {
			return fmt.Errorf("invalid composite task item %s: %w", task.ID, err)
		}
	}
	// Validate strategy
	strategy := v.config.GetStrategy()
	if strategy != "" && strategy != StrategyFailFast && strategy != StrategyBestEffort {
		return fmt.Errorf("invalid composite strategy: %s", strategy)
	}
	return nil
}

func (v *TypeValidator) validateCompositeTaskItem(item *Config) error {
	if item.ID == "" {
		return fmt.Errorf("task item ID is required")
	}
	// All task types are now supported as subtasks with nested tasks implementation
	// Each task in composite execution should be a valid task configuration
	if err := item.Validate(); err != nil {
		return fmt.Errorf("invalid task configuration: %w", err)
	}
	return nil
}

func (v *TypeValidator) validateSignalTask() error {
	if v.config.Signal == nil || strings.TrimSpace(v.config.Signal.ID) == "" {
		return fmt.Errorf("signal.id is required for signal tasks")
	}
	// Signal tasks should not have action, agent, or tool
	if v.config.Action != "" {
		return fmt.Errorf("signal tasks cannot have an action field")
	}
	if v.config.Agent != nil {
		return fmt.Errorf("signal tasks cannot have an agent")
	}
	if v.config.Tool != nil {
		return fmt.Errorf("signal tasks cannot have a tool")
	}
	return nil
}

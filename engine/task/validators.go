package task

import (
	"fmt"
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
	if config.Type == TaskTypeParallel {
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
	}

	// Check collection task dependencies
	if config.Type == TaskTypeCollection {
		// Check task template
		if config.Template != nil {
			if err := v.detectCycle(config.Template, visited, visiting); err != nil {
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
	if v.config.Type != TaskTypeBasic && v.config.Type != TaskTypeRouter &&
		v.config.Type != TaskTypeParallel && v.config.Type != TaskTypeCollection {
		return fmt.Errorf("invalid task type: %s", v.config.Type)
	}
	if err := v.validateBasicTaskWithRef(); err != nil {
		return err
	}
	if v.config.Type == TaskTypeRouter {
		if err := v.validateRouterTask(); err != nil {
			return err
		}
	}
	if v.config.Type == TaskTypeParallel {
		if err := v.validateParallelTask(); err != nil {
			return err
		}
	}
	if v.config.Type == TaskTypeCollection {
		if err := v.validateCollectionTask(); err != nil {
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
	config := v.config

	// Validate required fields
	if config.Items == "" {
		return fmt.Errorf("items is required for collection tasks")
	}

	if config.Template == nil {
		return fmt.Errorf("task template is required for collection tasks")
	}

	// Validate task template
	if err := config.Template.Validate(); err != nil {
		return fmt.Errorf("invalid task template: %w", err)
	}

	// Validate mode
	mode := config.GetMode()
	if mode != CollectionModeParallel && mode != CollectionModeSequential {
		return fmt.Errorf("invalid collection mode: %s", mode)
	}

	// Validate batch size for sequential mode
	if mode == CollectionModeSequential && config.Batch <= 0 {
		return fmt.Errorf("batch size must be greater than 0 for sequential mode")
	}

	// Validate parallel strategy for parallel mode
	if mode == CollectionModeParallel {
		strategy := config.GetStrategy()
		if strategy != StrategyWaitAll && strategy != StrategyFailFast &&
			strategy != StrategyBestEffort && strategy != StrategyRace {
			return fmt.Errorf("invalid parallel strategy: %s", strategy)
		}
	}

	return nil
}

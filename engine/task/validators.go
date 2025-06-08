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
		for i := range config.ParallelTask.Tasks {
			if err := v.detectCycle(&config.ParallelTask.Tasks[i], visited, visiting); err != nil {
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
		// Check task template reference
		if config.Task != nil {
			if err := v.detectCycle(config.Task, visited, visiting); err != nil {
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
	if v.config.Type != TaskTypeBasic && v.config.Type != TaskTypeRouter && v.config.Type != TaskTypeParallel && v.config.Type != TaskTypeCollection {
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
	if len(v.config.ParallelTask.Tasks) == 0 {
		return fmt.Errorf("parallel tasks must have at least one sub-task")
	}

	// Check for duplicate IDs first before validating individual items
	seen := make(map[string]bool)
	for i := range v.config.ParallelTask.Tasks {
		task := &v.config.ParallelTask.Tasks[i]
		if seen[task.ID] {
			return fmt.Errorf("duplicate task ID in parallel execution: %s", task.ID)
		}
		seen[task.ID] = true
	}

	// Then validate each individual task
	for i := range v.config.ParallelTask.Tasks {
		task := &v.config.ParallelTask.Tasks[i]
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
	// Validate required fields
	if v.config.CollectionTask.Items == "" {
		return fmt.Errorf("collection tasks must specify an items expression")
	}
	
	if v.config.Task == nil {
		return fmt.Errorf("collection tasks must specify a task template")
	}
	
	// Validate task template
	if v.config.Task.ID == "" {
		return fmt.Errorf("collection task template must have an ID")
	}
	
	// Validate task template configuration
	if err := v.config.Task.Validate(); err != nil {
		return fmt.Errorf("invalid collection task template: %w", err)
	}
	
	// Validate mode
	mode := v.config.CollectionTask.GetMode()
	if mode != CollectionModeParallel && mode != CollectionModeSequential {
		return fmt.Errorf("invalid collection mode: %s", mode)
	}
	
	// Validate batch size for sequential mode
	if mode == CollectionModeSequential {
		batch := v.config.CollectionTask.GetBatch()
		if batch <= 0 {
			return fmt.Errorf("batch size must be positive for sequential mode, got: %d", batch)
		}
	}
	
	// Validate strategy for parallel mode
	if mode == CollectionModeParallel {
		strategy := v.config.GetStrategy()
		if strategy != StrategyWaitAll && strategy != StrategyFailFast && 
		   strategy != StrategyBestEffort && strategy != StrategyRace {
			return fmt.Errorf("invalid collection strategy: %s", strategy)
		}
		
		// Validate max workers
		maxWorkers := v.config.GetMaxWorkers()
		if maxWorkers <= 0 {
			return fmt.Errorf("max_workers must be positive for parallel mode, got: %d", maxWorkers)
		}
	}
	
	// Validate variable names
	itemVar := v.config.CollectionTask.GetItemVar()
	indexVar := v.config.CollectionTask.GetIndexVar()
	if itemVar == "" {
		return fmt.Errorf("item_var cannot be empty")
	}
	if indexVar == "" {
		return fmt.Errorf("index_var cannot be empty")
	}
	if itemVar == indexVar {
		return fmt.Errorf("item_var and index_var cannot be the same: %s", itemVar)
	}
	
	// Validate timeout format if provided
	if v.config.Timeout != "" {
		_, err := v.config.GetTimeout()
		if err != nil {
			return fmt.Errorf("invalid timeout format: %w", err)
		}
	}
	
	return nil
}

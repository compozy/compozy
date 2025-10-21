package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// -----------------------------------------------------------------------------
// CycleValidator - Detects circular dependencies in task references
// -----------------------------------------------------------------------------

type CycleValidator struct{}

func NewCycleValidator() *CycleValidator {
	return &CycleValidator{}
}

func (v *CycleValidator) Validate() error {
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
	if err := v.processTaskByType(config, visited, visiting); err != nil {
		return err
	}
	delete(visiting, taskID)
	visited[taskID] = true
	return nil
}

func (v *CycleValidator) processTaskByType(config *Config, visited map[string]bool, visiting map[string]bool) error {
	switch config.Type {
	case TaskTypeParallel:
		return v.processNestedTasks(config, visited, visiting)
	case TaskTypeComposite:
		return v.processTasksArray(config.Tasks, visited, visiting)
	case TaskTypeCollection:
		return v.processNestedTasks(config, visited, visiting)
	}
	return nil
}

func (v *CycleValidator) processNestedTasks(config *Config, visited map[string]bool, visiting map[string]bool) error {
	if err := v.processTasksArray(config.Tasks, visited, visiting); err != nil {
		return err
	}
	if config.Task != nil {
		if err := v.detectCycle(config.Task, visited, visiting); err != nil {
			return err
		}
	}
	return nil
}

func (v *CycleValidator) processTasksArray(tasks []Config, visited map[string]bool, visiting map[string]bool) error {
	for i := range tasks {
		if err := v.detectCycle(&tasks[i], visited, visiting); err != nil {
			return err
		}
	}
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

func (v *TypeValidator) Validate(ctx context.Context) error {
	if v.config.Type == "" {
		return nil
	}
	switch v.config.Type {
	case TaskTypeBasic:
		return v.validateBasicTask(ctx)
	case TaskTypeRouter:
		if err := v.validateExecutorFields(ctx); err != nil {
			return err
		}
		return v.validateRouterTask(ctx)
	case TaskTypeParallel:
		if err := v.validateExecutorFields(ctx); err != nil {
			return err
		}
		return v.validateParallelTask(ctx)
	case TaskTypeCollection:
		if err := v.validateExecutorFields(ctx); err != nil {
			return err
		}
		return v.validateCollectionTask(ctx)
	case TaskTypeAggregate:
		return v.validateAggregateTask(ctx)
	case TaskTypeComposite:
		if err := v.validateExecutorFields(ctx); err != nil {
			return err
		}
		return v.validateCompositeTask(ctx)
	case TaskTypeSignal:
		return v.validateSignalTask(ctx)
	case TaskTypeWait:
		return v.validateWaitTask(ctx)
	case TaskTypeMemory:
		return v.validateMemoryTask(ctx)
	default:
		return fmt.Errorf("invalid task type: %s", v.config.Type)
	}
}

func (v *TypeValidator) validateExecutorFields(_ context.Context) error {
	hasAgent, hasTool, hasDirectLLM, prompt := v.detectExecutorUsage()
	if err := v.enforceExecutorExclusivity(hasAgent, hasTool, hasDirectLLM); err != nil {
		return err
	}
	if err := v.validateToolExecutor(hasTool, prompt); err != nil {
		return err
	}
	if err := v.validateAgentExecutor(hasAgent, prompt); err != nil {
		return err
	}
	return v.validateDirectLLMExecutor(hasDirectLLM)
}

// detectExecutorUsage determines which executor types are configured for the task.
func (v *TypeValidator) detectExecutorUsage() (bool, bool, bool, string) {
	hasAgent := v.config.GetAgent() != nil
	hasTool := v.config.GetTool() != nil
	prompt := strings.TrimSpace(v.config.Prompt)
	hasDirectLLM := v.config.ModelConfig.Provider != "" && prompt != ""
	return hasAgent, hasTool, hasDirectLLM, prompt
}

// enforceExecutorExclusivity ensures only one executor type is active.
func (v *TypeValidator) enforceExecutorExclusivity(hasAgent, hasTool, hasDirectLLM bool) error {
	executorCount := 0
	if hasAgent {
		executorCount++
	}
	if hasTool {
		executorCount++
	}
	if hasDirectLLM {
		executorCount++
	}
	if executorCount > 1 {
		return fmt.Errorf(
			"cannot specify multiple executor types; use only one: " +
				"agent, tool, or direct LLM (model_config + prompt)",
		)
	}
	return nil
}

// validateToolExecutor checks tool-specific field constraints.
func (v *TypeValidator) validateToolExecutor(hasTool bool, prompt string) error {
	if !hasTool {
		return nil
	}
	if v.config.Action != "" {
		return fmt.Errorf("action is not allowed when executor type is tool")
	}
	if prompt != "" {
		return fmt.Errorf("prompt is not allowed when executor type is tool")
	}
	return nil
}

// validateAgentExecutor verifies agent requirements are satisfied.
func (v *TypeValidator) validateAgentExecutor(hasAgent bool, prompt string) error {
	if !hasAgent {
		return nil
	}
	if v.config.Action == "" && prompt == "" {
		return fmt.Errorf("tasks using agents must specify at least one of 'action' or 'prompt'")
	}
	return nil
}

// validateDirectLLMExecutor enforces direct LLM configuration rules.
func (v *TypeValidator) validateDirectLLMExecutor(hasDirectLLM bool) error {
	if !hasDirectLLM {
		return nil
	}
	if v.config.ModelConfig.Model == "" {
		return fmt.Errorf("model is required in model_config for direct LLM tasks")
	}
	return nil
}

func (v *TypeValidator) validateBasicTask(ctx context.Context) error {
	return v.validateExecutorFields(ctx)
}

func (v *TypeValidator) validateRouterTask(_ context.Context) error {
	if strings.TrimSpace(v.config.Action) != "" {
		return fmt.Errorf("router tasks cannot have an action field")
	}
	expression := strings.TrimSpace(v.config.Condition)
	if expression == "" {
		return fmt.Errorf("condition is required for router tasks")
	}
	evaluator, err := NewCELEvaluator()
	if err != nil {
		return fmt.Errorf("failed to create CEL evaluator: %w", err)
	}
	if err := evaluator.ValidateExpression(expression); err != nil {
		return fmt.Errorf("invalid router condition: %w", err)
	}
	if len(v.config.Routes) == 0 {
		return fmt.Errorf("routes are required for router tasks")
	}
	return nil
}

func (v *TypeValidator) validateParallelTask(ctx context.Context) error {
	if len(v.config.Tasks) == 0 {
		return fmt.Errorf("parallel tasks must have at least one sub-task")
	}
	seen := make(map[string]bool)
	for i := range v.config.Tasks {
		task := &v.config.Tasks[i]
		if seen[task.ID] {
			return fmt.Errorf("duplicate task ID in parallel execution: %s", task.ID)
		}
		seen[task.ID] = true
	}
	for i := range v.config.Tasks {
		task := &v.config.Tasks[i]
		if err := v.validateParallelTaskItem(ctx, task); err != nil {
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

func (v *TypeValidator) validateParallelTaskItem(ctx context.Context, item *Config) error {
	if item.ID == "" {
		return fmt.Errorf("task item ID is required")
	}
	if err := item.Validate(ctx); err != nil {
		return fmt.Errorf("invalid task configuration: %w", err)
	}
	return nil
}

func (v *TypeValidator) validateCollectionTask(ctx context.Context) error {
	validator := NewCollectionValidator(v.config)
	return validator.Validate(ctx)
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

func (v *CollectionValidator) Validate(ctx context.Context) error {
	if err := v.validateStructure(ctx); err != nil {
		return err
	}
	if err := v.validateConfig(ctx); err != nil {
		return err
	}
	if err := v.validateTaskTemplate(ctx); err != nil {
		return err
	}
	return nil
}

// validateStructure ensures that collection tasks have exactly one of 'task' or 'tasks' configured
func (v *CollectionValidator) validateStructure(_ context.Context) error {
	hasTask := v.config.Task != nil
	hasTasks := len(v.config.Tasks) > 0
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
func (v *CollectionValidator) validateConfig(_ context.Context) error {
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
	return nil
}

// validateTaskTemplate validates the task template if provided
func (v *CollectionValidator) validateTaskTemplate(ctx context.Context) error {
	if v.config.Task != nil {
		if err := v.config.Task.Validate(ctx); err != nil {
			return fmt.Errorf("invalid collection task template: %w", err)
		}
	}
	return nil
}

func (v *TypeValidator) validateAggregateTask(_ context.Context) error {
	if v.config.Outputs == nil || len(*v.config.Outputs) == 0 {
		return fmt.Errorf("aggregate tasks must have outputs defined")
	}
	if v.config.Action != "" {
		return fmt.Errorf("aggregate tasks cannot have an action field")
	}
	if v.config.Agent != nil {
		return fmt.Errorf("aggregate tasks cannot have an agent")
	}
	if v.config.Tool != nil {
		return fmt.Errorf("aggregate tasks cannot have a tool")
	}
	if v.config.Sleep != "" {
		return fmt.Errorf("aggregate tasks cannot have a sleep field")
	}
	if v.config.With != nil && len(*v.config.With) > 0 {
		return fmt.Errorf("aggregate tasks cannot have a with field")
	}
	return nil
}

func (v *TypeValidator) validateCompositeTask(ctx context.Context) error {
	if len(v.config.Tasks) == 0 {
		return fmt.Errorf("composite tasks must have at least one sub-task")
	}
	seen := make(map[string]bool)
	for i := range v.config.Tasks {
		task := &v.config.Tasks[i]
		if seen[task.ID] {
			return fmt.Errorf("duplicate task ID in composite execution: %s", task.ID)
		}
		seen[task.ID] = true
	}
	for i := range v.config.Tasks {
		task := &v.config.Tasks[i]
		if err := v.validateCompositeTaskItem(ctx, task); err != nil {
			return fmt.Errorf("invalid composite task item %s: %w", task.ID, err)
		}
	}
	if v.config.Strategy != "" {
		return fmt.Errorf("composite tasks cannot have a strategy: %s", v.config.Strategy)
	}
	return nil
}

func (v *TypeValidator) validateCompositeTaskItem(ctx context.Context, item *Config) error {
	if item.ID == "" {
		return fmt.Errorf("task item ID is required")
	}
	if err := item.Validate(ctx); err != nil {
		return fmt.Errorf("invalid task configuration: %w", err)
	}
	return nil
}

func (v *TypeValidator) validateSignalTask(_ context.Context) error {
	if v.config.Signal == nil || strings.TrimSpace(v.config.Signal.ID) == "" {
		return fmt.Errorf("signal.id is required for signal tasks")
	}
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

func (v *TypeValidator) validateWaitTask(_ context.Context) error {
	if v.config.Action != "" {
		return fmt.Errorf("wait tasks cannot have an action field")
	}
	if v.config.Agent != nil {
		return fmt.Errorf("wait tasks cannot have an agent")
	}
	if v.config.Tool != nil {
		return fmt.Errorf("wait tasks cannot have a tool")
	}
	return nil
}

func (v *TypeValidator) validateMemoryTask(_ context.Context) error {
	if v.config.Action != "" {
		return fmt.Errorf("memory tasks cannot have an action field")
	}
	if v.config.Agent != nil {
		return fmt.Errorf("memory tasks cannot have an agent")
	}
	if v.config.Tool != nil {
		return fmt.Errorf("memory tasks cannot have a tool")
	}
	return nil
}

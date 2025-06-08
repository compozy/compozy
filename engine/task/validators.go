package task

import (
	"fmt"
	"regexp"
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

	// Validate required fields and basic structure
	if err := v.validateCollectionRequiredFields(config); err != nil {
		return err
	}

	// Validate security aspects
	if err := v.validateCollectionSecurity(config); err != nil {
		return err
	}

	// Validate task template
	if err := config.Template.Validate(); err != nil {
		return fmt.Errorf("invalid task template: %w", err)
	}

	// Validate execution mode and parameters
	return v.validateCollectionExecution(config)
}

func (v *TypeValidator) validateCollectionRequiredFields(config *Config) error {
	if config.Items == "" {
		return fmt.Errorf("items is required for collection tasks")
	}

	if config.Template == nil {
		return fmt.Errorf("task template is required for collection tasks")
	}

	return nil
}

func (v *TypeValidator) validateCollectionSecurity(config *Config) error {
	// Security validation for items expression
	if err := v.validateTemplateExpression(config.Items, "items"); err != nil {
		return err
	}

	// Security validation for filter expression if present
	if config.Filter != "" {
		if err := v.validateTemplateExpression(config.Filter, "filter"); err != nil {
			return err
		}
	}

	// Validate variable names for security
	if config.ItemVar != "" {
		if err := v.validateVariableName(config.ItemVar, "item_var"); err != nil {
			return err
		}
	}
	if config.IndexVar != "" {
		if err := v.validateVariableName(config.IndexVar, "index_var"); err != nil {
			return err
		}
	}

	return nil
}

func (v *TypeValidator) validateCollectionExecution(config *Config) error {
	// Validate mode
	mode := config.GetMode()
	if mode != CollectionModeParallel && mode != CollectionModeSequential {
		return fmt.Errorf("invalid collection mode: %s", mode)
	}

	// Validate mode-specific parameters
	if mode == CollectionModeSequential {
		return v.validateSequentialMode(config)
	}

	return v.validateParallelMode(config)
}

func (v *TypeValidator) validateSequentialMode(config *Config) error {
	batch := config.GetBatch()
	if batch <= 0 {
		return fmt.Errorf("batch size must be greater than 0 for sequential mode")
	}
	if batch > DefaultMaxBatchSize {
		return fmt.Errorf("batch size %d exceeds maximum allowed size %d", batch, DefaultMaxBatchSize)
	}
	return nil
}

func (v *TypeValidator) validateParallelMode(config *Config) error {
	maxWorkers := config.GetMaxWorkers()
	if maxWorkers > DefaultMaxParallelWorkers {
		return fmt.Errorf("max_workers %d exceeds maximum allowed %d", maxWorkers, DefaultMaxParallelWorkers)
	}

	strategy := config.GetStrategy()
	if strategy != StrategyWaitAll && strategy != StrategyFailFast &&
		strategy != StrategyBestEffort && strategy != StrategyRace {
		return fmt.Errorf("invalid parallel strategy: %s", strategy)
	}

	return nil
}

// -----------------------------------------------------------------------------
// Security Validation Methods
// -----------------------------------------------------------------------------

var (
	// Regular expression for valid variable names (alphanumeric + underscore, starting with letter/underscore)
	validVariableNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

	// Dangerous patterns to detect in template expressions
	dangerousPatterns = []string{
		"exec",
		"system",
		"eval",
		"import",
		"require",
		"os.",
		"fs.",
		"path.",
		"process.",
		"child_process",
		"execSync",
		"spawn",
		"shell",
		"cmd",
		"powershell",
		"bash",
		"sh",
		"/bin/",
		"/usr/bin/",
		"rm -",
		"del ",
		"format ",
		"mkfs",
	}
)

// validateTemplateExpression checks template expressions for security issues
func (v *TypeValidator) validateTemplateExpression(expr, fieldName string) error {
	if expr == "" {
		return nil
	}

	// Check for dangerous patterns
	lowerExpr := strings.ToLower(expr)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerExpr, pattern) {
			return fmt.Errorf("potentially dangerous pattern '%s' detected in %s expression", pattern, fieldName)
		}
	}

	// Check for command injection patterns
	if strings.Contains(expr, "$(") || strings.Contains(expr, "`") {
		return fmt.Errorf("command substitution patterns detected in %s expression", fieldName)
	}

	// Check for path traversal
	if strings.Contains(expr, "../") || strings.Contains(expr, "..\\") {
		return fmt.Errorf("path traversal patterns detected in %s expression", fieldName)
	}

	// Check for excessively long expressions (possible DoS)
	if len(expr) > 10000 {
		return fmt.Errorf("%s expression is too long (max 10000 characters)", fieldName)
	}

	// Check for excessive nesting (possible DoS)
	openBraces := strings.Count(expr, "{{")
	closeBraces := strings.Count(expr, "}}")
	if openBraces > 50 || closeBraces > 50 {
		return fmt.Errorf("%s expression has excessive template nesting", fieldName)
	}
	if openBraces != closeBraces {
		return fmt.Errorf("%s expression has mismatched template braces", fieldName)
	}

	return nil
}

// validateVariableName ensures variable names are safe and follow conventions
func (v *TypeValidator) validateVariableName(varName, fieldName string) error {
	if varName == "" {
		return nil
	}

	// Check length
	if len(varName) > 100 {
		return fmt.Errorf("%s variable name is too long (max 100 characters)", fieldName)
	}

	// Check format
	if !validVariableNameRegex.MatchString(varName) {
		return fmt.Errorf(
			"%s variable name '%s' is invalid (must start with letter/underscore, contain only alphanumeric/underscore)",
			fieldName,
			varName,
		)
	}

	// Check for reserved words
	reservedWords := []string{
		"workflow", "task", "tasks", "output", "outputs", "input", "inputs",
		"config", "state", "status", "error", "errors", "system", "admin",
		"root", "sudo", "exec", "eval", "import", "require", "module",
	}
	lowerVarName := strings.ToLower(varName)
	for _, reserved := range reservedWords {
		if lowerVarName == reserved {
			return fmt.Errorf("%s variable name '%s' conflicts with reserved word", fieldName, varName)
		}
	}

	return nil
}

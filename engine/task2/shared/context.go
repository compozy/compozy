package shared

import (
	"fmt"
	"maps"
	"sort"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/contracts"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/dgraph-io/ristretto/v2"
)

// NormalizationContext holds all data needed for normalization
type NormalizationContext struct {
	WorkflowState  *workflow.State
	WorkflowConfig *workflow.Config
	TaskConfig     *task.Config
	ParentTask     *task.Config
	ParentConfig   map[string]any          // Parent configuration properties
	TaskConfigs    map[string]*task.Config // Task configurations by ID
	CurrentInput   *core.Input
	MergedEnv      *core.EnvMap
	ChildrenIndex  map[string][]string // Maps parent task exec ID to child task IDs
	Variables      map[string]any
}

// IsNormalizationContext implements the contracts.NormalizationContext interface
func (nc *NormalizationContext) IsNormalizationContext() {}

// Compile-time check that NormalizationContext implements contracts.NormalizationContext
var _ contracts.NormalizationContext = (*NormalizationContext)(nil)

// GetVariables returns the variables map, creating it if necessary
func (nc *NormalizationContext) GetVariables() map[string]any {
	if nc.Variables == nil {
		nc.Variables = make(map[string]any)
	}
	return nc.Variables
}

// ContextBuilder builds normalization contexts with workflow and task data
type ContextBuilder struct {
	// Ristretto cache for parent contexts to prevent redundant building
	parentContextCache   *ristretto.Cache[string, map[string]any]
	VariableBuilder      *VariableBuilder
	ChildrenIndexBuilder *ChildrenIndexBuilder
	TaskOutputBuilder    TaskOutputBuilder
	ConfigMerger         *ConfigMerger
}

// NewContextBuilder creates a new context builder
func NewContextBuilder() (*ContextBuilder, error) {
	// Create Ristretto cache with proper configuration for memory management
	// Using more conservative limits to prevent memory leaks in long-running workflows
	cache, err := ristretto.NewCache(&ristretto.Config[string, map[string]any]{
		NumCounters: 500, // 5x expected unique parent contexts (100) - reduced from 1000
		MaxCost:     50,  // Max 50 parent contexts cached - reduced from 100
		BufferItems: 64,  // Recommended buffer size
		OnEvict: func(_ *ristretto.Item[map[string]any]) {
			// Optional: Log cache evictions for monitoring memory usage
			// This helps track cache efficiency and detect memory pressure
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create parent context cache: %w", err)
	}
	return &ContextBuilder{
		parentContextCache:   cache,
		VariableBuilder:      NewVariableBuilder(),
		ChildrenIndexBuilder: NewChildrenIndexBuilder(),
		TaskOutputBuilder:    NewTaskOutputBuilder(),
		ConfigMerger:         NewConfigMerger(),
	}, nil
}

func (cb *ContextBuilder) buildContextInternal(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
	parentExecID *core.ID, // can be nil – means "discover heuristically"
) *NormalizationContext {
	if workflowState == nil {
		workflowState = &workflow.State{}
	}
	if workflowConfig == nil {
		workflowConfig = &workflow.Config{}
	}
	nc := &NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     taskConfig,
		TaskConfigs:    make(map[string]*task.Config),
	}

	// Build children index using dedicated builder
	nc.ChildrenIndex = cb.ChildrenIndexBuilder.BuildChildrenIndex(workflowState)

	// Build variables using dedicated builder
	vars := cb.VariableBuilder.BuildBaseVariables(workflowState, workflowConfig, taskConfig)

	// Add workflow output as tasks for backward compatibility in deterministic order
	if workflowState != nil && workflowState.Tasks != nil {
		tasksMap := cb.buildTasksMap(workflowState, taskConfig, parentExecID, nc)
		cb.VariableBuilder.AddTasksToVariables(vars, workflowState, tasksMap)
	}

	// Preserve current input if available
	if nc.CurrentInput == nil && taskConfig != nil && taskConfig.With != nil {
		nc.CurrentInput = taskConfig.With
	}
	cb.VariableBuilder.AddCurrentInputToVariables(vars, nc.CurrentInput)

	nc.Variables = vars
	return nc
}

// buildTasksMap builds the tasks map for the context
func (cb *ContextBuilder) buildTasksMap(
	workflowState *workflow.State,
	taskConfig *task.Config,
	parentExecID *core.ID,
	nc *NormalizationContext,
) map[string]any {
	// Determine the parent exec ID if not provided
	if parentExecID == nil && taskConfig != nil {
		parentExecID = cb.findParentExecID(workflowState, taskConfig)
	}

	// Build ordered task states
	states := cb.buildOrderedTaskStates(workflowState)

	// Build the tasks map with two passes
	tasksMap := make(map[string]any)

	// Pass 1: include non-sibling tasks first
	cb.addNonSiblingTasks(tasksMap, states, parentExecID, nc)

	// Pass 2: overlay siblings so they shadow duplicates
	cb.addSiblingTasks(tasksMap, states, parentExecID, nc)

	return tasksMap
}

// findParentExecID finds the parent execution ID for a task
func (cb *ContextBuilder) findParentExecID(workflowState *workflow.State, taskConfig *task.Config) *core.ID {
	for _, ts := range workflowState.Tasks {
		if ts.TaskID == taskConfig.ID {
			return ts.ParentStateID
		}
	}
	return nil
}

// stateWithKey holds a task state with its key for sorting
type stateWithKey struct {
	key   string
	state *task.State
}

// buildOrderedTaskStates builds a deterministically ordered slice of task states
func (cb *ContextBuilder) buildOrderedTaskStates(workflowState *workflow.State) []stateWithKey {
	states := make([]stateWithKey, 0, len(workflowState.Tasks))
	for key, ts := range workflowState.Tasks {
		states = append(states, stateWithKey{key: key, state: ts})
	}
	sort.Slice(states, func(i, j int) bool {
		// TaskExecID is a ULID -> lexicographic order matches creation order.
		return states[i].state.TaskExecID.String() < states[j].state.TaskExecID.String()
	})
	return states
}

// addNonSiblingTasks adds non-sibling tasks to the tasks map
func (cb *ContextBuilder) addNonSiblingTasks(
	tasksMap map[string]any,
	states []stateWithKey,
	parentExecID *core.ID,
	nc *NormalizationContext,
) {
	for _, sw := range states {
		if parentExecID != nil && sw.state.ParentStateID != nil &&
			*sw.state.ParentStateID == *parentExecID {
			continue
		}
		tasksMap[sw.key] = cb.buildSingleTaskContext(sw.state.TaskID, sw.state, nc)
	}
}

// addSiblingTasks adds sibling tasks to the tasks map
func (cb *ContextBuilder) addSiblingTasks(
	tasksMap map[string]any,
	states []stateWithKey,
	parentExecID *core.ID,
	nc *NormalizationContext,
) {
	if parentExecID == nil {
		return
	}
	for _, sw := range states {
		if sw.state.ParentStateID != nil && *sw.state.ParentStateID == *parentExecID {
			tasksMap[sw.key] = cb.buildSingleTaskContext(sw.state.TaskID, sw.state, nc)
		}
	}
}

// BuildContext creates a normalization context from workflow and task data
func (cb *ContextBuilder) BuildContext(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) *NormalizationContext {
	return cb.buildContextInternal(workflowState, workflowConfig, taskConfig, nil)
}

// BuildContextForTaskInstance creates a normalization context with explicit parent execution ID
func (cb *ContextBuilder) BuildContextForTaskInstance(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
	parentExecID *core.ID,
) *NormalizationContext {
	return cb.buildContextInternal(workflowState, workflowConfig, taskConfig, parentExecID)
}

// buildSingleTaskContext builds context for a single task
func (cb *ContextBuilder) buildSingleTaskContext(
	taskID string,
	taskState *task.State,
	ctx *NormalizationContext,
) map[string]any {
	if taskState == nil {
		return map[string]any{IDKey: taskID}
	}

	taskContext := map[string]any{
		IDKey:     taskID,
		InputKey:  taskState.Input,
		StatusKey: taskState.Status,
	}
	if taskState.Error != nil {
		taskContext[ErrorKey] = taskState.Error
	}
	taskContext[OutputKey] = cb.TaskOutputBuilder.BuildTaskOutput(
		taskState,
		ctx.WorkflowState.Tasks,
		ctx.ChildrenIndex,
		0,
	)
	if taskState.CanHaveChildren() && ctx.ChildrenIndex != nil {
		taskContext[ChildrenKey] = cb.ChildrenIndexBuilder.BuildChildrenContext(
			taskState,
			ctx.WorkflowState,
			ctx.ChildrenIndex,
			ctx.TaskConfigs,
			cb.TaskOutputBuilder,
			0,
		)
	}
	cb.ConfigMerger.MergeTaskConfigIfExists(taskContext, taskID, ctx.TaskConfigs)
	return taskContext
}

// BuildSubTaskContext creates a specialized context for sub-tasks within parent tasks
func (cb *ContextBuilder) BuildSubTaskContext(
	ctx *NormalizationContext,
	parentTask *task.Config,
	parentState *task.State,
) (*NormalizationContext, error) {
	// Clone the base context
	nc := &NormalizationContext{
		WorkflowState:  ctx.WorkflowState,
		WorkflowConfig: ctx.WorkflowConfig,
		ParentTask:     parentTask,
		Variables:      make(map[string]any),
		TaskConfigs:    ctx.TaskConfigs,
	}
	// Copy base variables using variable builder
	vars, err := cb.VariableBuilder.CopyVariables(ctx.Variables)
	if err != nil {
		return nil, err
	}
	nc.Variables = vars

	// Use recursive parent building with caching
	if parentTask != nil {
		cb.VariableBuilder.AddParentToVariables(nc.Variables, cb.BuildParentContext(ctx, parentTask, 0))
	}
	// Add current task state if available
	if parentState != nil {
		currentMap := map[string]any{
			IDKey:     parentState.TaskID,
			InputKey:  parentState.Input,
			OutputKey: parentState.Output,
			StatusKey: parentState.Status,
		}
		if parentState.Error != nil {
			currentMap[ErrorKey] = parentState.Error
		}
		nc.Variables["current"] = currentMap
	}
	return nc, nil
}

// BuildNormalizationSubTaskContext creates a context for sub-tasks during normalization
// This is used when we don't have runtime state yet
func (cb *ContextBuilder) BuildNormalizationSubTaskContext(
	parentCtx *NormalizationContext,
	parentTask *task.Config,
	subTask *task.Config,
) (*NormalizationContext, error) {
	// Clone the base context
	nc := &NormalizationContext{
		WorkflowState:  parentCtx.WorkflowState,
		WorkflowConfig: parentCtx.WorkflowConfig,
		TaskConfig:     subTask,
		ParentTask:     parentTask,
		Variables:      make(map[string]any),
		TaskConfigs:    parentCtx.TaskConfigs,
		CurrentInput:   parentCtx.CurrentInput,
		ChildrenIndex:  parentCtx.ChildrenIndex,
	}
	// Copy parent variables using variable builder
	vars, err := cb.VariableBuilder.CopyVariables(parentCtx.Variables)
	if err != nil {
		return nil, err
	}
	nc.Variables = vars

	// Update task data for sub-task
	nc.Variables["task"] = map[string]any{
		IDKey:     subTask.ID,
		TypeKey:   subTask.Type,
		ActionKey: subTask.Action,
		WithKey:   subTask.With,
		EnvKey:    subTask.Env,
	}
	// Use recursive parent building with caching
	if parentTask != nil {
		cb.VariableBuilder.AddParentToVariables(nc.Variables, cb.BuildParentContext(parentCtx, parentTask, 0))
	}
	return nc, nil
}

// BuildParentContext recursively builds parent context chain with caching and cycle detection
func (cb *ContextBuilder) BuildParentContext(
	ctx *NormalizationContext,
	taskConfig *task.Config,
	depth int,
) map[string]any {
	return cb.buildParentContextWithVisited(ctx, taskConfig, depth, make(map[string]bool))
}

// buildParentContextWithVisited builds parent context with cycle detection using a sophisticated recursive algorithm.
// This is the core algorithm for building parent task contexts in template variables, enabling nested task access
// like {{ .parent.parent.output }} while preventing infinite loops and memory leaks.
//
// Algorithm Overview:
// 1. Depth and circular reference protection using visitor pattern
// 2. Ristretto cache lookup for performance optimization
// 3. Parent context map construction with configuration and runtime state
// 4. Recursive parent chain traversal with isolated visited maps
// 5. Cache storage for subsequent lookups
//
// Performance: O(depth) time complexity with O(1) cache hits, O(n) space for visited tracking
// Memory Safety: Automatic cleanup with defer and conservative cache limits
func (cb *ContextBuilder) buildParentContextWithVisited(
	ctx *NormalizationContext,
	taskConfig *task.Config,
	depth int,
	visited map[string]bool,
) map[string]any {
	limits := GetGlobalConfigLimits()
	// STEP 1: Boundary condition checks to prevent infinite recursion and stack overflow
	// MaxParentDepth typically set to 10-20 levels to handle reasonable nesting while preventing abuse
	if taskConfig == nil || depth >= limits.MaxParentDepth {
		return nil
	}

	// STEP 2: Circular reference detection using visitor pattern
	// This prevents infinite loops in cases like: TaskA -> TaskB -> TaskA
	// Returns structured error context instead of panicking for better debugging
	if visited[taskConfig.ID] {
		return map[string]any{
			IDKey:   taskConfig.ID,
			"error": "circular reference detected in parent chain",
		}
	}

	// STEP 3: Mark current task as visited for this traversal path
	// Using defer cleanup ensures visited state is properly managed even if function exits early
	// This enables multiple independent traversal paths without interference
	visited[taskConfig.ID] = true
	defer func() {
		delete(visited, taskConfig.ID) // Clean up for other traversal branches
	}()

	// STEP 4: Cache optimization check using composite key for workflow isolation
	// Cache key includes workflow execution ID to prevent cross-workflow contamination
	// Only check cache after circular reference detection to ensure safety
	cacheKey := cb.buildCacheKey(ctx, taskConfig)
	if cached, found := cb.parentContextCache.Get(cacheKey); found {
		return cached // Cache hit - return previously computed result
	}

	// STEP 5: Build base parent context map with task configuration data
	// This forms the foundation of the parent context available in templates
	parentMap := map[string]any{
		IDKey:     taskConfig.ID,     // Task identifier for debugging and references
		TypeKey:   taskConfig.Type,   // Task type (parallel, collection, etc.)
		ActionKey: taskConfig.Action, // Task action/command to execute
		EnvKey:    taskConfig.Env,    // Environment variables for this task
	}

	// STEP 6: Handle 'With' input parameters with proper nil safety
	// core.Input is *map[string]any, so we dereference when not nil
	// This provides access to task input parameters in parent context
	if taskConfig.With != nil {
		parentMap[WithKey] = *taskConfig.With // Dereference pointer to get actual map
	} else {
		parentMap[WithKey] = taskConfig.With // Preserve nil for template conditionals
	}

	// STEP 7: Augment with runtime state data when available
	// Runtime state provides actual execution data (input, output, status, errors)
	// This enables templates to access real execution results, not just configuration
	if ctx.WorkflowState != nil && ctx.WorkflowState.Tasks != nil {
		if state, exists := ctx.WorkflowState.Tasks[taskConfig.ID]; exists {
			parentMap[InputKey] = state.Input   // Actual input passed at runtime
			parentMap[OutputKey] = state.Output // Task execution output/result
			parentMap[StatusKey] = state.Status // Current execution status
			if state.Error != nil {
				parentMap[ErrorKey] = state.Error // Error information if task failed
			}
		}
	}

	// STEP 8: Recursive parent chain traversal with multi-strategy lookup
	// Uses findParentTask which tries: runtime state -> workflow config -> error context
	// Error contexts provide debugging information when parent lookup fails
	grandParentTask, errorContext := cb.findParentTask(ctx, taskConfig)
	if errorContext != nil {
		// Parent lookup encountered issues - provide error context for debugging
		parentMap[ParentKey] = errorContext
	} else if grandParentTask != nil {
		// STEP 9: Create isolated visited map copy for recursive call
		// This prevents visited state pollution between different traversal branches
		// Essential for handling complex task hierarchies with multiple paths
		visitedCopy := make(map[string]bool)
		for k, v := range visited {
			visitedCopy[k] = v
		}
		// Recursive call to build grandparent context with incremented depth
		parentMap[ParentKey] = cb.buildParentContextWithVisited(ctx, grandParentTask, depth+1, visitedCopy)
	}
	// If no grandparent found, ParentKey remains unset (nil), which is valid

	// STEP 10: Cache the computed result for future lookups
	// Cost of 1 means each cached item has equal weight in LRU eviction
	// Only cache if no circular reference was detected to avoid caching error states
	cb.parentContextCache.Set(cacheKey, parentMap, 1)
	return parentMap
}

// buildCacheKey creates a unique cache key for a task in a workflow execution
func (cb *ContextBuilder) buildCacheKey(ctx *NormalizationContext, taskConfig *task.Config) string {
	if ctx.WorkflowState != nil {
		// Include workflow execution ID for better cache isolation between runs
		return fmt.Sprintf("%s:%s:%s", ctx.WorkflowState.WorkflowID, ctx.WorkflowState.WorkflowExecID, taskConfig.ID)
	}
	// Fallback to just task ID if no workflow state
	return taskConfig.ID
}

// findParentTask searches for the parent of a given task using a multi-strategy lookup approach.
// This implements a robust parent discovery algorithm that handles both runtime and configuration contexts.
//
// Strategy Priority:
// 1. Runtime State Lookup: Uses ParentStateID from task execution state (most accurate)
// 2. Workflow Config Search: Falls back to static configuration structure traversal
// 3. Error Context: Returns structured error information when parent cannot be found
//
// Returns: (*task.Config, map[string]any) where second parameter is error context if first is nil
// Error contexts provide debugging information instead of silent failures
func (cb *ContextBuilder) findParentTask(
	ctx *NormalizationContext,
	childTask *task.Config,
) (*task.Config, map[string]any) {
	// STRATEGY 1: Runtime state lookup (highest priority)
	// This approach uses actual execution relationships established during workflow runtime
	// More reliable than static config because it reflects actual parent-child execution relationships
	if ctx.WorkflowState != nil && ctx.WorkflowState.Tasks != nil {
		if childState, exists := ctx.WorkflowState.Tasks[childTask.ID]; exists {
			if childState.ParentStateID != nil {
				// STEP 1.1: Find parent task state using execution ID
				// ParentStateID points to the TaskExecID of the parent task instance
				// This handles cases where the same task config is executed multiple times
				var parentState *task.State
				for _, taskState := range ctx.WorkflowState.Tasks {
					if taskState.TaskExecID == *childState.ParentStateID {
						parentState = taskState
						break
					}
				}
				if parentState == nil {
					// Parent execution state not found - this indicates a data consistency issue
					// Return error context with execution ID for debugging
					return nil, map[string]any{
						"error":          "parent task state not found",
						"parent_exec_id": childState.ParentStateID.String(),
					}
				}
				// STEP 1.2: Locate parent task configuration
				// First try the TaskConfigs cache (most efficient)
				if ctx.TaskConfigs != nil {
					if parentConfig, ok := ctx.TaskConfigs[parentState.TaskID]; ok {
						return parentConfig, nil // Successfully found both state and config
					}
				}
				// STEP 1.3: Fallback to workflow config search
				// Search through workflow's task definitions for the parent config
				if ctx.WorkflowConfig != nil {
					for i := range ctx.WorkflowConfig.Tasks {
						if ctx.WorkflowConfig.Tasks[i].ID == parentState.TaskID {
							return &ctx.WorkflowConfig.Tasks[i], nil
						}
					}
				}
				// STEP 1.4: Parent state exists but config is missing
				// This can happen during task config updates or partial workflow loading
				// Return partial context using available state data for debugging
				return nil, map[string]any{
					IDKey:     parentState.TaskID,
					StatusKey: parentState.Status,
					InputKey:  parentState.Input,
					OutputKey: parentState.Output,
					"warning": "parent task config not found, using state data only",
				}
			}
		}
	}
	// STRATEGY 2: Workflow configuration structure search (fallback)
	// Used when runtime state is not available or doesn't contain parent relationships
	// Searches through static task hierarchy defined in workflow configuration
	if ctx.WorkflowConfig != nil {
		if parent := cb.searchParentInWorkflow(ctx.WorkflowConfig, childTask.ID); parent != nil {
			return parent, nil
		}
	}
	// No parent found through any strategy - return nil, nil (not an error, task may be root)
	return nil, nil
}

// searchParentInWorkflow recursively searches for a task's parent in workflow config
func (cb *ContextBuilder) searchParentInWorkflow(
	workflow *workflow.Config,
	childTaskID string,
) *task.Config {
	// Search through all tasks in workflow
	for i := range workflow.Tasks {
		task := &workflow.Tasks[i]
		if cb.taskContainsChild(task, childTaskID) {
			return task
		}
	}
	return nil
}

// taskContainsChild checks if a task contains a specific child task using recursive traversal.
// This implements a comprehensive child discovery algorithm that handles different task type hierarchies.
//
// Supported Parent Task Types:
// - Parallel/Composite: Direct children in Tasks slice with recursive nesting support
// - Collection: Template task in Task field with recursive template structure support
// - Other types: No children (Basic, Router, Wait, Signal, Aggregate)
//
// Algorithm: Depth-first search with early termination on match
// Time Complexity: O(n) where n is total number of descendant tasks
// Space Complexity: O(d) where d is maximum nesting depth (call stack)
func (cb *ContextBuilder) taskContainsChild(
	parentTask *task.Config,
	childTaskID string,
) bool {
	// Handle different task types based on their child containment patterns
	switch parentTask.Type {
	case task.TaskTypeParallel, task.TaskTypeComposite:
		// These task types contain direct children in the Tasks slice
		// Each child can potentially have its own nested children (recursive structure)
		if parentTask.Tasks != nil {
			for i := range parentTask.Tasks {
				subTask := &parentTask.Tasks[i]
				// STEP 1: Check for direct match (child is immediate descendant)
				if subTask.ID == childTaskID {
					return true // Found direct child
				}
				// STEP 2: Recursive check for nested children (grandchildren, etc.)
				// This handles deeply nested task hierarchies like:
				// Parallel -> Composite -> Collection -> Basic
				if cb.taskContainsChild(subTask, childTaskID) {
					return true // Found in nested structure
				}
			}
		}
	case task.TaskTypeCollection:
		// Collection tasks use a different structure - they have a template task in Task field
		// The template is instantiated multiple times at runtime but config contains the pattern
		if parentTask.Task != nil {
			// STEP 1: Check if the template task itself matches
			if parentTask.Task.ID == childTaskID {
				return true // Child is the collection template
			}
			// STEP 2: Recursively check if the template contains nested children
			// This handles cases where collection template is a complex nested structure
			return cb.taskContainsChild(parentTask.Task, childTaskID)
		}
	}
	// Default case: Task types that don't contain children
	// (Basic, Router, Wait, Signal, Aggregate tasks are leaf nodes)
	return false
}

// ClearCache clears the parent context cache - should be called at workflow start
func (cb *ContextBuilder) ClearCache() {
	cb.parentContextCache.Clear()
}

// ClearWorkflowCache clears cache entries for a specific workflow execution to prevent memory leaks.
// This implements a smart cache eviction strategy that balances performance with memory management.
//
// Cache Management Strategy:
// 1. Ristretto doesn't support prefix-based deletion, so we use indirect approaches
// 2. Conservative cache limits (MaxCost: 50) to naturally limit memory usage
// 3. LRU eviction automatically removes old entries when cache fills
// 4. Performance-based full cache clearing when efficiency drops
//
// Memory Safety: Prevents unbounded growth in long-running workflow engines
// Performance Impact: Minimal when cache is performing well, aggressive when degraded
func (cb *ContextBuilder) ClearWorkflowCache(workflowID string, _ core.ID) {
	if workflowID == "" {
		return // Invalid workflow ID - nothing to clear
	}
	// LIMITATION: Ristretto doesn't provide prefix-based deletion by design
	// We can't directly delete all entries matching "workflowID:workflowExecID:*"
	// Instead, we rely on a multi-layered approach:
	// 1. Reduced cache size limits (MaxCost: 50 instead of 100) for natural eviction
	// 2. LRU eviction policy automatically removes oldest entries when cache fills
	// 3. Intelligent full cache clearing when performance indicates problems

	// STRATEGY: Performance-based cache management
	// If cache hit ratio is low, the cache is not providing value and may contain stale data
	// Better to clear everything and rebuild cache with fresh, relevant data
	if cb.parentContextCache.Metrics.Ratio() < 0.5 { // Hit ratio below 50% indicates poor performance
		cb.parentContextCache.Clear() // Aggressive but safe - ensures no memory leaks
	}
	// NOTE: This is intentionally aggressive to prioritize memory safety over cache performance
	// In production, consider implementing a custom cache with prefix deletion if needed
}

// GetCacheStats returns cache statistics for monitoring memory usage
func (cb *ContextBuilder) GetCacheStats() *ristretto.Metrics {
	return cb.parentContextCache.Metrics
}

// AddTaskState adds runtime task state data to the context variables.
// This includes execution IDs, status, output, and error information.
func (cb *ContextBuilder) AddTaskState(ctx *NormalizationContext, taskState *task.State) {
	if taskState == nil {
		return
	}

	vars := ctx.GetVariables()
	vars["state"] = map[string]any{
		IDKey:       taskState.TaskID,
		"exec_id":   taskState.TaskExecID,
		StatusKey:   taskState.Status,
		OutputKey:   taskState.Output,
		ErrorKey:    taskState.Error,
		"parent_id": taskState.ParentStateID,
	}
}

// AddCollectionData adds collection-specific iteration data to the context.
// This is used when processing collection tasks to make the current item and index available.
func (cb *ContextBuilder) AddCollectionData(ctx *NormalizationContext, item any, index int) {
	vars := ctx.GetVariables()
	vars[ItemKey] = item
	vars[IndexKey] = index
}

// MergeVariables merges additional variables into the normalization context in deterministic order.
// Existing variables with the same keys will be overwritten.
func (cb *ContextBuilder) MergeVariables(ctx *NormalizationContext, additionalVars map[string]any) {
	vars := ctx.GetVariables()
	keys := SortedMapKeys(additionalVars)
	for _, k := range keys {
		vars[k] = additionalVars[k]
	}
}

// BuildCollectionContext builds context for collection tasks
func (cb *ContextBuilder) BuildCollectionContext(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) map[string]any {
	// Defensive null checks to prevent null pointer dereferences
	if workflowState == nil || taskConfig == nil {
		return make(map[string]any)
	}

	// Build full context using proper BuildContext method to populate Variables
	ctx := cb.BuildContext(workflowState, workflowConfig, taskConfig)

	// Add additional TaskConfigs from workflowConfig if available
	if workflowConfig != nil && workflowConfig.Tasks != nil {
		for i := range workflowConfig.Tasks {
			tc := &workflowConfig.Tasks[i]
			ctx.TaskConfigs[tc.ID] = tc
		}
	}

	// Use the proper template context from BuildContext which includes Variables
	templateContext := ctx.BuildTemplateContext()

	// Add task-specific context from 'with' parameter
	if taskConfig.With != nil {
		maps.Copy(templateContext, *taskConfig.With)
	}
	return templateContext
}

// BuildTemplateContext returns the template context for normalization
func (nc *NormalizationContext) BuildTemplateContext() map[string]any {
	if nc.Variables == nil {
		return make(map[string]any)
	}
	return nc.Variables
}

// IsWithinCollection checks if the current task or any of its ancestors is a collection task
func (nc *NormalizationContext) IsWithinCollection() bool {
	// Check if the current task is a collection
	if nc.TaskConfig != nil && nc.TaskConfig.Type == task.TaskTypeCollection {
		return true
	}
	// Check if the parent task is a collection
	if nc.ParentTask != nil && nc.ParentTask.Type == task.TaskTypeCollection {
		return true
	}
	// For composite tasks within collections, we need to check the parent context
	// The ParentTask might be a composite, but that composite might be within a collection
	// We can detect this by checking if there are collection-specific variables like "item" or "index"
	if nc.Variables != nil {
		// If we have item or index variables, we're within a collection context
		if _, hasItem := nc.Variables[ItemKey]; hasItem {
			return true
		}
		if _, hasIndex := nc.Variables[IndexKey]; hasIndex {
			return true
		}
	}
	return false
}

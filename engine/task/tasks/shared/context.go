package shared

import (
	"context"
	"fmt"
	"sort"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/contracts"
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

// NewContextBuilder creates and returns a ContextBuilder configured with its
// auxiliary builders and a bounded in-memory cache used to memoize parent
// normalization contexts.
//
// The returned ContextBuilder contains:
// - a ristretto-based parentContextCache configured for modest memory usage,
// - a VariableBuilder, ChildrenIndexBuilder, TaskOutputBuilder and ConfigMerger.
//
// Returns an error if the internal cache cannot be created.
func NewContextBuilder(ctx context.Context) (*ContextBuilder, error) {
	// NOTE: Use conservative cache bounds to avoid unbounded growth in long-running workflows.
	cache, err := ristretto.NewCache(&ristretto.Config[string, map[string]any]{
		NumCounters: 500, // 5x expected unique parent contexts (100) - reduced from 1000
		MaxCost:     50,  // Max 50 parent contexts cached - reduced from 100
		BufferItems: 64,  // Recommended buffer size
		OnEvict: func(_ *ristretto.Item[map[string]any]) {
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create parent context cache: %w", err)
	}
	return &ContextBuilder{
		parentContextCache:   cache,
		VariableBuilder:      NewVariableBuilder(),
		ChildrenIndexBuilder: NewChildrenIndexBuilder(),
		TaskOutputBuilder:    NewTaskOutputBuilder(ctx),
		ConfigMerger:         NewConfigMerger(),
	}, nil
}

// NewContextBuilderWithContext returns a ContextBuilder initialized with a
// TaskOutputBuilder that respects limits from the provided context.
func NewContextBuilderWithContext(ctx context.Context) (*ContextBuilder, error) {
	b, err := NewContextBuilder(ctx)
	if err != nil {
		return nil, err
	}
	b.TaskOutputBuilder = NewTaskOutputBuilderWithContext(ctx)
	return b, nil
}

func (cb *ContextBuilder) buildContextInternal(
	ctx context.Context,
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
	nc.ChildrenIndex = cb.ChildrenIndexBuilder.BuildChildrenIndex(workflowState)
	vars := cb.VariableBuilder.BuildBaseVariables(workflowState, workflowConfig, taskConfig)
	if workflowState != nil && workflowState.Tasks != nil {
		tasksMap := cb.buildTasksMap(ctx, workflowState, taskConfig, parentExecID, nc)
		cb.VariableBuilder.AddTasksToVariables(vars, workflowState, tasksMap)
	}
	if nc.CurrentInput == nil && taskConfig != nil && taskConfig.With != nil {
		nc.CurrentInput = taskConfig.With
	}
	cb.VariableBuilder.AddCurrentInputToVariables(vars, nc.CurrentInput)
	nc.Variables = vars
	return nc
}

// buildTasksMap builds the tasks map for the context
func (cb *ContextBuilder) buildTasksMap(
	ctx context.Context,
	workflowState *workflow.State,
	taskConfig *task.Config,
	parentExecID *core.ID,
	nc *NormalizationContext,
) map[string]any {
	if parentExecID == nil && taskConfig != nil {
		parentExecID = cb.findParentExecID(workflowState, taskConfig)
	}
	states := cb.buildOrderedTaskStates(workflowState)
	tasksMap := make(map[string]any)
	cb.addNonSiblingTasks(ctx, tasksMap, states, parentExecID, nc)
	cb.addSiblingTasks(ctx, tasksMap, states, parentExecID, nc)
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
		return states[i].state.TaskExecID.String() < states[j].state.TaskExecID.String()
	})
	return states
}

// addNonSiblingTasks adds non-sibling tasks to the tasks map
func (cb *ContextBuilder) addNonSiblingTasks(
	ctx context.Context,
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
		tasksMap[sw.key] = cb.buildSingleTaskContext(ctx, sw.state.TaskID, sw.state, nc)
	}
}

// addSiblingTasks adds sibling tasks to the tasks map
func (cb *ContextBuilder) addSiblingTasks(
	ctx context.Context,
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
			tasksMap[sw.key] = cb.buildSingleTaskContext(ctx, sw.state.TaskID, sw.state, nc)
		}
	}
}

// BuildContext creates a normalization context from workflow and task data
func (cb *ContextBuilder) BuildContext(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) *NormalizationContext {
	return cb.buildContextInternal(ctx, workflowState, workflowConfig, taskConfig, nil)
}

// BuildContextForTaskInstance creates a normalization context with explicit parent execution ID
func (cb *ContextBuilder) BuildContextForTaskInstance(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
	parentExecID *core.ID,
) *NormalizationContext {
	return cb.buildContextInternal(ctx, workflowState, workflowConfig, taskConfig, parentExecID)
}

// buildSingleTaskContext builds context for a single task
func (cb *ContextBuilder) buildSingleTaskContext(
	ctx context.Context,
	taskID string,
	taskState *task.State,
	normCtx *NormalizationContext,
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
		normCtx.WorkflowState.Tasks,
		normCtx.ChildrenIndex,
		0,
	)
	if taskState.CanHaveChildren() && normCtx.ChildrenIndex != nil {
		taskContext[ChildrenKey] = cb.ChildrenIndexBuilder.BuildChildrenContext(
			ctx,
			taskState,
			normCtx.WorkflowState,
			normCtx.ChildrenIndex,
			normCtx.TaskConfigs,
			cb.TaskOutputBuilder,
			0,
		)
	}
	cb.ConfigMerger.MergeTaskConfigIfExists(taskContext, taskID, normCtx.TaskConfigs)
	return taskContext
}

// BuildSubTaskContext creates a specialized context for sub-tasks within parent tasks
func (cb *ContextBuilder) BuildSubTaskContext(
	ctx context.Context,
	normCtx *NormalizationContext,
	parentTask *task.Config,
	parentState *task.State,
) (*NormalizationContext, error) {
	nc := &NormalizationContext{
		WorkflowState:  normCtx.WorkflowState,
		WorkflowConfig: normCtx.WorkflowConfig,
		ParentTask:     parentTask,
		Variables:      make(map[string]any),
		TaskConfigs:    normCtx.TaskConfigs,
	}
	vars, err := cb.VariableBuilder.CopyVariables(normCtx.Variables)
	if err != nil {
		return nil, err
	}
	nc.Variables = vars
	if parentTask != nil {
		cb.VariableBuilder.AddParentToVariables(nc.Variables, cb.BuildParentContext(ctx, normCtx, parentTask, 0))
	}
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
	ctx context.Context,
	parentCtx *NormalizationContext,
	parentTask *task.Config,
	subTask *task.Config,
) (*NormalizationContext, error) {
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
	vars, err := cb.VariableBuilder.CopyVariables(parentCtx.Variables)
	if err != nil {
		return nil, err
	}
	nc.Variables = vars
	nc.Variables["task"] = map[string]any{
		IDKey:     subTask.ID,
		TypeKey:   subTask.Type,
		ActionKey: subTask.Action,
		WithKey:   subTask.With,
		EnvKey:    subTask.Env,
	}
	if parentTask != nil {
		cb.VariableBuilder.AddParentToVariables(nc.Variables, cb.BuildParentContext(ctx, parentCtx, parentTask, 0))
	}
	return nc, nil
}

// BuildParentContext recursively builds parent context chain with caching and cycle detection
func (cb *ContextBuilder) BuildParentContext(
	ctx context.Context,
	normCtx *NormalizationContext,
	taskConfig *task.Config,
	depth int,
) map[string]any {
	return cb.buildParentContextWithVisited(ctx, normCtx, taskConfig, depth, make(map[string]bool))
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
	ctx context.Context,
	normCtx *NormalizationContext,
	taskConfig *task.Config,
	depth int,
	visited map[string]bool,
) map[string]any {
	if cb.shouldStopParentTraversal(ctx, taskConfig, depth) {
		return nil
	}
	if circular := cb.detectCircularParent(visited, taskConfig); circular != nil {
		return circular
	}
	visited[taskConfig.ID] = true
	defer delete(visited, taskConfig.ID)
	if cached := cb.getCachedParentContext(normCtx, taskConfig); cached != nil {
		return cached
	}
	parentMap := cb.createParentMap(taskConfig)
	cb.populateParentRuntimeData(normCtx, taskConfig, parentMap)
	cb.attachParentChain(ctx, normCtx, taskConfig, depth, visited, parentMap)
	cb.parentContextCache.Set(cb.buildCacheKey(normCtx, taskConfig), parentMap, 1)
	return parentMap
}

// shouldStopParentTraversal determines if traversal should halt due to limits or nil config.
func (cb *ContextBuilder) shouldStopParentTraversal(ctx context.Context, taskConfig *task.Config, depth int) bool {
	if taskConfig == nil {
		return true
	}
	limits := GetGlobalConfigLimits(ctx)
	return depth >= limits.MaxParentDepth
}

// detectCircularParent returns an error context when a circular dependency is found.
func (cb *ContextBuilder) detectCircularParent(visited map[string]bool, taskConfig *task.Config) map[string]any {
	if visited[taskConfig.ID] {
		return map[string]any{
			IDKey:   taskConfig.ID,
			"error": "circular reference detected in parent chain",
		}
	}
	return nil
}

// getCachedParentContext retrieves a cached parent context when available.
func (cb *ContextBuilder) getCachedParentContext(
	normCtx *NormalizationContext,
	taskConfig *task.Config,
) map[string]any {
	cacheKey := cb.buildCacheKey(normCtx, taskConfig)
	if cached, found := cb.parentContextCache.Get(cacheKey); found {
		return cached
	}
	return nil
}

// createParentMap initializes the parent context map using configuration data.
func (cb *ContextBuilder) createParentMap(taskConfig *task.Config) map[string]any {
	parentMap := map[string]any{
		IDKey:     taskConfig.ID,
		TypeKey:   taskConfig.Type,
		ActionKey: taskConfig.Action,
		EnvKey:    taskConfig.Env,
	}
	if taskConfig.With != nil {
		parentMap[WithKey] = *taskConfig.With
	} else {
		parentMap[WithKey] = taskConfig.With
	}
	return parentMap
}

// populateParentRuntimeData augments parent context with runtime execution information.
func (cb *ContextBuilder) populateParentRuntimeData(
	normCtx *NormalizationContext,
	taskConfig *task.Config,
	parentMap map[string]any,
) {
	if normCtx.WorkflowState == nil || normCtx.WorkflowState.Tasks == nil {
		return
	}
	state, exists := normCtx.WorkflowState.Tasks[taskConfig.ID]
	if !exists {
		return
	}
	parentMap[InputKey] = state.Input
	parentMap[OutputKey] = state.Output
	parentMap[StatusKey] = state.Status
	if state.Error != nil {
		parentMap[ErrorKey] = state.Error
	}
}

// attachParentChain resolves and attaches the next parent level when available.
func (cb *ContextBuilder) attachParentChain(
	ctx context.Context,
	normCtx *NormalizationContext,
	taskConfig *task.Config,
	depth int,
	visited map[string]bool,
	parentMap map[string]any,
) {
	grandParentTask, errorContext := cb.findParentTask(normCtx, taskConfig)
	if errorContext != nil {
		parentMap[ParentKey] = errorContext
		return
	}
	if grandParentTask == nil {
		return
	}
	visitedCopy := copyVisitedFlags(visited)
	parentMap[ParentKey] = cb.buildParentContextWithVisited(ctx, normCtx, grandParentTask, depth+1, visitedCopy)
}

// copyVisitedFlags clones the visited set for recursive traversal.
func copyVisitedFlags(source map[string]bool) map[string]bool {
	cloned := make(map[string]bool, len(source))
	for k, v := range source {
		cloned[k] = v
	}
	return cloned
}

// buildCacheKey creates a unique cache key for a task in a workflow execution
func (cb *ContextBuilder) buildCacheKey(normCtx *NormalizationContext, taskConfig *task.Config) string {
	if normCtx.WorkflowState != nil {
		return fmt.Sprintf(
			"%s:%s:%s",
			normCtx.WorkflowState.WorkflowID,
			normCtx.WorkflowState.WorkflowExecID,
			taskConfig.ID,
		)
	}
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
	normCtx *NormalizationContext,
	childTask *task.Config,
) (*task.Config, map[string]any) {
	if parentConfig, errCtx, handled := cb.findParentFromRuntime(normCtx, childTask); handled {
		return parentConfig, errCtx
	}
	if normCtx.WorkflowConfig != nil {
		if parent := cb.searchParentInWorkflow(normCtx.WorkflowConfig, childTask.ID); parent != nil {
			return parent, nil
		}
	}
	return nil, nil
}

// findParentFromRuntime uses runtime state to locate the parent task when possible.
func (cb *ContextBuilder) findParentFromRuntime(
	normCtx *NormalizationContext,
	childTask *task.Config,
) (*task.Config, map[string]any, bool) {
	if normCtx.WorkflowState == nil || normCtx.WorkflowState.Tasks == nil {
		return nil, nil, false
	}
	childState, exists := normCtx.WorkflowState.Tasks[childTask.ID]
	if !exists || childState.ParentStateID == nil {
		return nil, nil, false
	}
	parentState := cb.lookupParentState(normCtx.WorkflowState.Tasks, childState.ParentStateID)
	if parentState == nil {
		return nil, map[string]any{
			"error":          "parent task state not found",
			"parent_exec_id": childState.ParentStateID.String(),
		}, true
	}
	if parentConfig := cb.parentConfigFromCache(normCtx.TaskConfigs, parentState.TaskID); parentConfig != nil {
		return parentConfig, nil, true
	}
	if parentConfig := cb.parentConfigFromWorkflow(normCtx.WorkflowConfig, parentState.TaskID); parentConfig != nil {
		return parentConfig, nil, true
	}
	return nil, map[string]any{
		IDKey:     parentState.TaskID,
		StatusKey: parentState.Status,
		InputKey:  parentState.Input,
		OutputKey: parentState.Output,
		"warning": "parent task config not found, using state data only",
	}, true
}

// lookupParentState finds a parent state entry by execution identifier.
func (cb *ContextBuilder) lookupParentState(
	taskStates map[string]*task.State,
	parentStateID *core.ID,
) *task.State {
	for _, taskState := range taskStates {
		if taskState.TaskExecID == *parentStateID {
			return taskState
		}
	}
	return nil
}

// parentConfigFromCache retrieves parent configuration from cached task configs.
func (cb *ContextBuilder) parentConfigFromCache(
	taskConfigs map[string]*task.Config,
	parentTaskID string,
) *task.Config {
	if taskConfigs == nil {
		return nil
	}
	if parentConfig, ok := taskConfigs[parentTaskID]; ok {
		return parentConfig
	}
	return nil
}

// parentConfigFromWorkflow searches workflow configuration for a parent task.
func (cb *ContextBuilder) parentConfigFromWorkflow(
	workflowCfg *workflow.Config,
	parentTaskID string,
) *task.Config {
	if workflowCfg == nil {
		return nil
	}
	for i := range workflowCfg.Tasks {
		if workflowCfg.Tasks[i].ID == parentTaskID {
			return &workflowCfg.Tasks[i]
		}
	}
	return nil
}

// searchParentInWorkflow recursively searches for a task's parent in workflow config
func (cb *ContextBuilder) searchParentInWorkflow(
	workflow *workflow.Config,
	childTaskID string,
) *task.Config {
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
	switch parentTask.Type {
	case task.TaskTypeParallel, task.TaskTypeComposite:
		if parentTask.Tasks != nil {
			for i := range parentTask.Tasks {
				subTask := &parentTask.Tasks[i]
				if subTask.ID == childTaskID {
					return true // Found direct child
				}
				if cb.taskContainsChild(subTask, childTaskID) {
					return true // Found in nested structure
				}
			}
		}
	case task.TaskTypeCollection:
		if parentTask.Task != nil {
			if parentTask.Task.ID == childTaskID {
				return true // Child is the collection template
			}
			return cb.taskContainsChild(parentTask.Task, childTaskID)
		}
	}
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
	if cb.parentContextCache.Metrics.Ratio() < 0.5 { // Hit ratio below 50% indicates poor performance
		cb.parentContextCache.Clear() // Aggressive but safe - ensures no memory leaks
	}
	// NOTE: This is intentionally aggressive to prioritize memory safety over cache performance
}

// GetCacheStats returns cache statistics for monitoring memory usage
func (cb *ContextBuilder) GetCacheStats() *ristretto.Metrics {
	return cb.parentContextCache.Metrics
}

// AddTaskState adds runtime task state data to the context variables.
// This includes execution IDs, status, output, and error information.
func (cb *ContextBuilder) AddTaskState(normCtx *NormalizationContext, taskState *task.State) {
	if taskState == nil {
		return
	}
	vars := normCtx.GetVariables()
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
func (cb *ContextBuilder) AddCollectionData(normCtx *NormalizationContext, item any, index int) {
	vars := normCtx.GetVariables()
	vars[ItemKey] = item
	vars[IndexKey] = index
}

// MergeVariables merges additional variables into the normalization context in deterministic order.
// Existing variables with the same keys will be overwritten.
func (cb *ContextBuilder) MergeVariables(normCtx *NormalizationContext, additionalVars map[string]any) {
	vars := normCtx.GetVariables()
	keys := SortedMapKeys(additionalVars)
	for _, k := range keys {
		vars[k] = additionalVars[k]
	}
}

// BuildCollectionContext builds context for collection tasks
func (cb *ContextBuilder) BuildCollectionContext(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) map[string]any {
	if workflowState == nil || taskConfig == nil {
		return make(map[string]any)
	}
	normCtx := cb.BuildContext(ctx, workflowState, workflowConfig, taskConfig)
	if workflowConfig != nil && workflowConfig.Tasks != nil {
		for i := range workflowConfig.Tasks {
			tc := &workflowConfig.Tasks[i]
			normCtx.TaskConfigs[tc.ID] = tc
		}
	}
	templateContext := normCtx.BuildTemplateContext()
	if taskConfig.With != nil {
		templateContext = core.CopyMaps(templateContext, *taskConfig.With)
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
	if nc.TaskConfig != nil && nc.TaskConfig.Type == task.TaskTypeCollection {
		return true
	}
	if nc.ParentTask != nil && nc.ParentTask.Type == task.TaskTypeCollection {
		return true
	}
	if nc.Variables != nil {
		if _, hasItem := nc.Variables[ItemKey]; hasItem {
			return true
		}
		if _, hasIndex := nc.Variables[IndexKey]; hasIndex {
			return true
		}
	}
	return false
}

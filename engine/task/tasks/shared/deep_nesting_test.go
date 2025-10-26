package shared_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/workflow"
)

func TestDeepNesting_CollectionCompositeParallelBasic(t *testing.T) {
	t.Run("Should build correct context for Collection -> Composite -> Parallel -> Basic nesting", func(t *testing.T) {
		// Arrange - Create the 4-level deep nested structure with PROPER parent-child relationships
		builder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		// Create task hierarchy: Collection contains Composite, which contains Parallel, which contains Basic
		basicTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "basic-task",
				Type: task.TaskTypeBasic,
				With: &core.Input{"param": "basic-value"},
			},
			BasicTask: task.BasicTask{Action: "test-action"},
		}

		parallelTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parallel-task",
				Type: task.TaskTypeParallel,
			},
			Tasks: []task.Config{*basicTask}, // Parallel contains Basic
		}

		compositeTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "composite-task",
				Type: task.TaskTypeComposite,
			},
			Tasks: []task.Config{*parallelTask}, // Composite contains Parallel
		}

		collectionTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "collection-task",
				Type: task.TaskTypeCollection,
			},
			CollectionConfig: task.CollectionConfig{
				Items: `["item1", "item2"]`,
			},
			Task: compositeTask, // Collection template contains Composite
		}

		// Create runtime state with proper parent-child execution relationships
		collectionExecID := core.MustNewID()
		compositeExecID := core.MustNewID()
		parallelExecID := core.MustNewID()
		basicExecID := core.MustNewID()

		// Additional execution IDs for runtime instances
		compositeTask0ExecID := core.MustNewID()
		parallelTask0ExecID := core.MustNewID()
		basicTask0ExecID := core.MustNewID()

		workflowState := &workflow.State{
			WorkflowID:     "deep-nesting-test",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				// Collection task (root) - Uses config ID directly
				"collection-task": {
					TaskID:        "collection-task",
					TaskExecID:    collectionExecID,
					Status:        core.StatusRunning,
					ExecutionType: task.ExecutionCollection, // Required for CanHaveChildren()
					Input:         &core.Input{"items": []any{"item1", "item2"}},
				},
				// Composite task instance - Uses config ID directly for findParentTask lookup
				"composite-task": {
					TaskID:        "composite-task",
					TaskExecID:    compositeExecID,
					ParentStateID: &collectionExecID, // Points to collection execution
					Status:        core.StatusRunning,
					ExecutionType: task.ExecutionComposite, // Required for CanHaveChildren()
					Input:         &core.Input{"item": "item1", "index": 0},
				},
				// Parallel task instance - Uses config ID directly
				"parallel-task": {
					TaskID:        "parallel-task",
					TaskExecID:    parallelExecID,
					ParentStateID: &compositeExecID, // Points to composite execution
					Status:        core.StatusRunning,
					ExecutionType: task.ExecutionParallel, // Required for CanHaveChildren()
				},
				// Basic task instance - Uses config ID directly
				"basic-task": {
					TaskID:        "basic-task",
					TaskExecID:    basicExecID,
					ParentStateID: &parallelExecID, // Points to parallel execution
					Status:        core.StatusSuccess,
					ExecutionType: task.ExecutionBasic, // Basic task - cannot have children
					Input:         &core.Input{"param": "basic-value"},
					Output: &core.Output{
						"result": "success",
						"item":   "item1",
						"level":  "basic",
					},
				},
				// Additional runtime instances for collection items (with suffixes)
				"composite-task-0": {
					TaskID:        "composite-task",
					TaskExecID:    compositeTask0ExecID,
					ParentStateID: &collectionExecID,
					Status:        core.StatusRunning,
					ExecutionType: task.ExecutionComposite, // Required for CanHaveChildren()
					Input:         &core.Input{"item": "item1", "index": 0},
				},
				"parallel-task-0": {
					TaskID:        "parallel-task",
					TaskExecID:    parallelTask0ExecID,
					ParentStateID: &compositeTask0ExecID, // Should point to composite-task-0's execution ID
					Status:        core.StatusRunning,
					ExecutionType: task.ExecutionParallel, // Required for CanHaveChildren()
				},
				"basic-task-0": {
					TaskID:        "basic-task",
					TaskExecID:    basicTask0ExecID,
					ParentStateID: &parallelTask0ExecID, // Should point to parallel-task-0's execution ID
					Status:        core.StatusSuccess,
					ExecutionType: task.ExecutionBasic, // Basic task - cannot have children
					Input:         &core.Input{"param": "basic-value"},
					Output: &core.Output{
						"result": "success",
						"item":   "item1",
						"level":  "basic",
					},
				},
			},
		}

		ctx := &shared.NormalizationContext{
			WorkflowState: workflowState,
			TaskConfigs: map[string]*task.Config{
				"collection-task": collectionTask,
				"composite-task":  compositeTask,
				"parallel-task":   parallelTask,
				"basic-task":      basicTask,
			},
			Variables: make(map[string]any),
		}

		// Test 1: BuildParentContext from basic task should traverse up the runtime parent chain
		t.Run("Should traverse 4-level parent chain from basic task", func(t *testing.T) {
			result := builder.BuildParentContext(t.Context(), ctx, basicTask, 0)

			require.NotNil(t, result)
			assert.Equal(t, "basic-task", result[shared.IDKey])
			assert.Equal(t, task.TaskTypeBasic, result[shared.TypeKey])
			assert.Equal(t, "test-action", result[shared.ActionKey])

			// Should have runtime state data merged
			assert.Equal(t, core.StatusSuccess, result[shared.StatusKey])
			if output, ok := result[shared.OutputKey].(*core.Output); ok {
				assert.Equal(t, "success", (*output)["result"])
			}

			// Should have parent context (parallel task)
			parentContext, hasParent := result[shared.ParentKey].(map[string]any)
			require.True(t, hasParent, "Basic task should have parallel task as parent")
			assert.Equal(t, "parallel-task", parentContext[shared.IDKey])
			assert.Equal(t, task.TaskTypeParallel, parentContext[shared.TypeKey])

			// Should have grandparent context (composite task)
			grandParentContext, hasGrandParent := parentContext[shared.ParentKey].(map[string]any)
			require.True(t, hasGrandParent, "Parallel task should have composite task as parent")
			assert.Equal(t, "composite-task", grandParentContext[shared.IDKey])
			assert.Equal(t, task.TaskTypeComposite, grandParentContext[shared.TypeKey])

			// Should have great-grandparent context (collection task)
			greatGrandParentContext, hasGreatGrandParent := grandParentContext[shared.ParentKey].(map[string]any)
			require.True(t, hasGreatGrandParent, "Composite task should have collection task as parent")
			assert.Equal(t, "collection-task", greatGrandParentContext[shared.IDKey])
			assert.Equal(t, task.TaskTypeCollection, greatGrandParentContext[shared.TypeKey])

			// Collection should be the root (no parent)
			_, hasRootParent := greatGrandParentContext[shared.ParentKey]
			assert.False(t, hasRootParent, "Collection task should be the root with no parent")

			t.Logf("Successfully traversed 4-level parent chain: basic -> parallel -> composite -> collection")
		})

		// Test 2: BuildChildrenContext from collection task should traverse down the execution hierarchy
		t.Run("Should traverse 4-level children hierarchy from collection task", func(t *testing.T) {
			childrenBuilder := shared.NewChildrenIndexBuilder()
			outputBuilder := shared.NewTaskOutputBuilder(t.Context())

			// Build children index based on parent relationships in workflow state
			childrenIndex := childrenBuilder.BuildChildrenIndex(workflowState)

			// Debug: Log children index structure
			t.Logf("Children index: %+v", childrenIndex)
			t.Logf("Collection exec ID: %s", collectionExecID.String())
			t.Logf("Composite exec ID: %s", compositeExecID.String())

			// Update context with children index
			ctx.ChildrenIndex = childrenIndex

			collectionState := workflowState.Tasks["collection-task"]
			result := childrenBuilder.BuildChildrenContext(
				t.Context(),
				collectionState,
				workflowState,
				childrenIndex,
				ctx.TaskConfigs,
				outputBuilder,
				0,
			)

			require.NotNil(t, result)

			// Debug: Log BuildChildrenContext result
			t.Logf("BuildChildrenContext result keys: %v", getMapKeys(result))

			// Should have composite child (runtime instance)
			compositeChild, hasComposite := result["composite-task-0"]
			if !hasComposite {
				// Debug: Log available children
				availableChildren := make([]string, 0, len(result))
				for k := range result {
					availableChildren = append(availableChildren, k)
				}
				t.Logf("Available children: %v", availableChildren)
			}
			require.True(t, hasComposite, "Collection should have composite-task-0 as child")

			compositeMap, ok := compositeChild.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "composite-task-0", compositeMap[shared.IDKey])
			assert.Equal(t, core.StatusRunning, compositeMap[shared.StatusKey])

			// Composite should have parallel children
			compositeChildren, hasCompositeChildren := compositeMap[shared.ChildrenKey].(map[string]any)
			if !hasCompositeChildren {
				t.Logf("Composite task context: %+v", compositeMap)
				compositeKeys := make([]string, 0, len(compositeMap))
				for k := range compositeMap {
					compositeKeys = append(compositeKeys, k)
				}
				t.Logf("Composite task keys: %v", compositeKeys)
			}
			require.True(t, hasCompositeChildren, "Composite task should have children")

			parallelChild, hasParallel := compositeChildren["parallel-task-0"]
			require.True(t, hasParallel, "Composite should have parallel-task-0 as child")

			parallelMap, ok := parallelChild.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "parallel-task-0", parallelMap[shared.IDKey])
			assert.Equal(t, core.StatusRunning, parallelMap[shared.StatusKey])

			// Parallel should have basic children
			parallelChildren, hasParallelChildren := parallelMap[shared.ChildrenKey].(map[string]any)
			require.True(t, hasParallelChildren, "Parallel task should have children")

			basicChild, hasBasic := parallelChildren["basic-task-0"]
			require.True(t, hasBasic, "Parallel should have basic-task-0 as child")

			basicMap, ok := basicChild.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "basic-task-0", basicMap[shared.IDKey])
			assert.Equal(t, core.StatusSuccess, basicMap[shared.StatusKey])
			assert.Contains(t, basicMap, shared.OutputKey)

			// Basic task should have no children (leaf node)
			_, hasBasicChildren := basicMap[shared.ChildrenKey]
			assert.False(t, hasBasicChildren, "Basic task should have no children")

			t.Logf("Successfully traversed 4-level children hierarchy: collection -> composite -> parallel -> basic")
		})

		// Test 3: Full context building should provide access to entire hierarchy
		t.Run("Should build complete template context with nested task access", func(t *testing.T) {
			// Build full context including task hierarchy and children index
			ctx.ChildrenIndex = shared.NewChildrenIndexBuilder().BuildChildrenIndex(workflowState)
			fullContext := builder.BuildContext(
				t.Context(),
				workflowState,
				&workflow.Config{ID: "deep-nesting-test", Tasks: []task.Config{*collectionTask}},
				collectionTask,
			)

			require.NotNil(t, fullContext)
			require.NotNil(t, fullContext.Variables)

			// Should have workflow context
			workflowContext, hasWorkflow := fullContext.Variables[shared.WorkflowKey].(map[string]any)
			require.True(t, hasWorkflow, "Should have workflow context")
			assert.Equal(t, "deep-nesting-test", workflowContext[shared.IDKey])

			// Should have tasks context with all runtime task instances
			tasksContext, hasTasks := fullContext.Variables[shared.TasksKey].(map[string]any)
			require.True(t, hasTasks, "Should have tasks context")

			// All runtime task instances should be accessible
			assert.Contains(t, tasksContext, "collection-task")
			assert.Contains(t, tasksContext, "composite-task")
			assert.Contains(t, tasksContext, "parallel-task")
			assert.Contains(t, tasksContext, "basic-task")
			// Should also have runtime instances
			assert.Contains(t, tasksContext, "composite-task-0")
			assert.Contains(t, tasksContext, "parallel-task-0")
			assert.Contains(t, tasksContext, "basic-task-0")

			// Verify basic task output is accessible for template usage (use config ID)
			basicTaskContext, hasBasic := tasksContext["basic-task"].(map[string]any)
			require.True(t, hasBasic, "Basic task should be in tasks context")
			assert.Equal(t, core.StatusSuccess, basicTaskContext[shared.StatusKey])

			// Verify output data structure
			if output, hasOutput := basicTaskContext[shared.OutputKey].(*core.Output); hasOutput {
				assert.Equal(t, "success", (*output)["result"])
				assert.Equal(t, "item1", (*output)["item"])
				assert.Equal(t, "basic", (*output)["level"])
			}

			// Verify parent context accessibility - use original ctx with proper TaskConfigs
			parentContext := builder.BuildParentContext(t.Context(), ctx, basicTask, 0)
			require.NotNil(t, parentContext)
			assert.Equal(t, "basic-task", parentContext[shared.IDKey])

			// Should be able to traverse full parent chain
			levelsTraversed := 0
			current := parentContext
			for current != nil {
				levelsTraversed++
				if parent, ok := current[shared.ParentKey].(map[string]any); ok {
					current = parent
				} else {
					break
				}
				if levelsTraversed > 5 { // Safety check
					break
				}
			}
			assert.Equal(
				t,
				4,
				levelsTraversed,
				"Should traverse exactly 4 levels: basic -> parallel -> composite -> collection",
			)

			t.Logf(
				"Successfully built complete context with %d task instances and %d-level parent traversal",
				len(tasksContext),
				levelsTraversed,
			)
		})

		// Test 4: Collection-specific context building
		t.Run("Should build collection context with nested structure awareness", func(t *testing.T) {
			collectionContext := builder.BuildCollectionContext(
				t.Context(),
				workflowState,
				&workflow.Config{ID: "deep-nesting-test"},
				collectionTask,
			)

			require.NotNil(t, collectionContext)

			// Should have base template context structure
			assert.Contains(t, collectionContext, shared.WorkflowKey)
			assert.Contains(t, collectionContext, shared.TasksKey)

			// Workflow context should be properly populated
			workflowContext, ok := collectionContext[shared.WorkflowKey].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "deep-nesting-test", workflowContext[shared.IDKey])

			// Tasks context should include all runtime instances
			tasksContext, ok := collectionContext[shared.TasksKey].(map[string]any)
			require.True(t, ok)

			// Should contain runtime task instances
			taskKeys := make([]string, 0, len(tasksContext))
			for k := range tasksContext {
				taskKeys = append(taskKeys, k)
			}

			// Verify we have both config IDs and runtime instances
			hasConfigIDs := false
			hasRuntimeInstances := false
			for _, key := range taskKeys {
				// Config IDs
				if key == "collection-task" || key == "composite-task" || key == "parallel-task" ||
					key == "basic-task" {
					hasConfigIDs = true
				}
				// Runtime instances have suffixes like -0, -1, etc.
				if key == "composite-task-0" || key == "parallel-task-0" || key == "basic-task-0" {
					hasRuntimeInstances = true
				}
			}
			assert.True(t, hasConfigIDs, "Collection context should include config task IDs")
			assert.True(t, hasRuntimeInstances, "Collection context should include runtime task instances")

			t.Logf("Collection context contains %d tasks: %v", len(tasksContext), taskKeys)
		})
	})
}

func TestDeepNesting_DepthLimitEnforcement(t *testing.T) {
	t.Run("Should enforce depth limits and handle deep parent chains properly", func(t *testing.T) {
		// Arrange - Create a deep hierarchy that tests depth limit enforcement
		builder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		limits := shared.GetGlobalConfigLimits(t.Context())
		// Create exactly at the limit to test boundary behavior
		testDepth := limits.MaxParentDepth

		// Build a realistic task hierarchy with proper parent-child relationships
		taskConfigs := make(map[string]*task.Config)
		taskStates := make(map[string]*task.State)
		var parentExecID *core.ID

		for i := range testDepth {
			taskID := fmt.Sprintf("level-%d-task", i)
			execID := core.MustNewID()

			// Create realistic task config based on level
			taskConfig := &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   taskID,
					Type: task.TaskTypeBasic,
				},
			}

			// Make some levels more realistic
			switch i {
			case 0:
				taskConfig.Type = task.TaskTypeCollection
				taskConfig.CollectionConfig = task.CollectionConfig{Items: `["item1"]`}
			case 1:
				taskConfig.Type = task.TaskTypeComposite
			case 2:
				taskConfig.Type = task.TaskTypeParallel
			default:
				taskConfig.Type = task.TaskTypeBasic
				taskConfig.BasicTask = task.BasicTask{Action: fmt.Sprintf("action-level-%d", i)}
			}

			taskConfigs[taskID] = taskConfig

			// Create runtime state
			state := &task.State{
				TaskID:        taskID,
				TaskExecID:    execID,
				Status:        core.StatusRunning,
				ParentStateID: parentExecID, // Link to parent execution
				Input:         &core.Input{"level": i, "task_id": taskID},
			}

			taskStates[taskID] = state
			parentExecID = &execID // This becomes parent for next level
		}

		workflowState := &workflow.State{
			WorkflowID:     "depth-limit-test",
			WorkflowExecID: core.MustNewID(),
			Tasks:          taskStates,
		}

		ctx := &shared.NormalizationContext{
			WorkflowState: workflowState,
			TaskConfigs:   taskConfigs,
			Variables:     make(map[string]any),
		}

		// Test with deepest task (should hit depth limit)
		deepestTaskID := fmt.Sprintf("level-%d-task", testDepth-1)
		deepestTask := taskConfigs[deepestTaskID]

		result := builder.BuildParentContext(t.Context(), ctx, deepestTask, 0)

		// Should return something but respect depth limits
		require.NotNil(t, result)
		assert.Equal(t, deepestTaskID, result[shared.IDKey])

		// Count actual traversed levels
		current := result
		traversedLevels := 0
		taskIDs := []string{}

		for current != nil {
			if taskID, ok := current[shared.IDKey].(string); ok {
				taskIDs = append(taskIDs, taskID)
			}
			traversedLevels++

			if parent, ok := current[shared.ParentKey].(map[string]any); ok {
				current = parent
				// Safety check to prevent infinite loops
				if traversedLevels > limits.MaxParentDepth+5 {
					t.Errorf("Traversed too many levels: %d, safety break", traversedLevels)
					break
				}
			} else {
				break
			}
		}

		// Assert depth limit enforcement
		assert.LessOrEqual(t, traversedLevels, limits.MaxParentDepth+1,
			"Should not exceed MaxParentDepth+1 (including self): got %d levels, limit %d",
			traversedLevels, limits.MaxParentDepth)

		assert.Greater(t, traversedLevels, 1, "Should traverse at least self + some parents")

		t.Logf("Traversed %d levels with depth limit %d. Task chain: %v",
			traversedLevels, limits.MaxParentDepth, taskIDs)

		// Test with a mid-level task (should traverse more)
		midTaskID := fmt.Sprintf("level-%d-task", testDepth/2)
		midTask := taskConfigs[midTaskID]

		midResult := builder.BuildParentContext(t.Context(), ctx, midTask, 0)
		require.NotNil(t, midResult)

		// Count mid-level traversal
		midCurrent := midResult
		midTraversed := 0
		for midCurrent != nil {
			midTraversed++
			if parent, ok := midCurrent[shared.ParentKey].(map[string]any); ok {
				midCurrent = parent
				if midTraversed > limits.MaxParentDepth+5 {
					break
				}
			} else {
				break
			}
		}

		// Mid-level should be able to traverse to root (fewer levels)
		expectedMidLevels := (testDepth / 2) + 1 // +1 for self
		assert.Equal(t, expectedMidLevels, midTraversed,
			"Mid-level task should traverse exactly to root: expected %d, got %d",
			expectedMidLevels, midTraversed)

		t.Logf("Mid-level task traversed %d levels (expected %d)", midTraversed, expectedMidLevels)
	})
}

func TestDeepNesting_CircularReferenceDetection(t *testing.T) {
	t.Run("Should detect and handle circular references in deep nesting", func(t *testing.T) {
		// Arrange - Create a hierarchy with a circular reference
		builder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		// Create tasks that will form a cycle: A -> B -> C -> A
		taskA := &task.Config{
			BaseConfig: task.BaseConfig{ID: "task-a", Type: task.TaskTypeComposite},
		}
		taskB := &task.Config{
			BaseConfig: task.BaseConfig{ID: "task-b", Type: task.TaskTypeParallel},
		}
		taskC := &task.Config{
			BaseConfig: task.BaseConfig{ID: "task-c", Type: task.TaskTypeBasic},
			BasicTask:  task.BasicTask{Action: "test-action"},
		}

		// Create execution IDs
		execA, execB, execC := core.MustNewID(), core.MustNewID(), core.MustNewID()

		// Create workflow state with circular parent references
		workflowState := &workflow.State{
			WorkflowID:     "circular-test",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"task-a": {
					TaskID:        "task-a",
					TaskExecID:    execA,
					ParentStateID: &execC, // A points to C (creating cycle)
					Status:        core.StatusRunning,
				},
				"task-b": {
					TaskID:        "task-b",
					TaskExecID:    execB,
					ParentStateID: &execA, // B points to A
					Status:        core.StatusRunning,
				},
				"task-c": {
					TaskID:        "task-c",
					TaskExecID:    execC,
					ParentStateID: &execB, // C points to B (completing cycle)
					Status:        core.StatusRunning,
				},
			},
		}

		ctx := &shared.NormalizationContext{
			WorkflowState: workflowState,
			TaskConfigs: map[string]*task.Config{
				"task-a": taskA,
				"task-b": taskB,
				"task-c": taskC,
			},
			Variables: make(map[string]any),
		}

		// Act - Try to build parent context for task in cycle
		result := builder.BuildParentContext(t.Context(), ctx, taskC, 0)

		// Assert - Should detect circular reference and handle gracefully
		require.NotNil(t, result)
		assert.Equal(t, "task-c", result[shared.IDKey])

		// Should detect cycle somewhere in the parent chain
		foundCycleError := false
		current := result
		for i := 0; i < 10 && current != nil; i++ { // Limit iterations to prevent infinite loop
			if errorVal, hasError := current["error"]; hasError {
				errorStr, ok := errorVal.(string)
				if ok && errorStr == "circular reference detected in parent chain" {
					foundCycleError = true
					break
				}
			}

			if parent, ok := current[shared.ParentKey].(map[string]any); ok {
				current = parent
			} else {
				break
			}
		}

		assert.True(t, foundCycleError, "Should detect circular reference in parent chain")
		t.Logf("Successfully detected circular reference in deep nesting scenario")
	})
}

// Helper function to get map keys for debugging
func getMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

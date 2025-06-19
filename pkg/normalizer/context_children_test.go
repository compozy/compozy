package normalizer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

func TestContextBuilder_ChildrenContext(t *testing.T) {
	cb := NewContextBuilder()
	t.Run("Should add children property for collection task", func(t *testing.T) {
		parentID := core.MustNewID()
		parentState := &task.State{
			TaskID:        "process-items",
			TaskExecID:    parentID,
			ExecutionType: task.ExecutionCollection,
			Status:        core.StatusRunning,
			Input:         &core.Input{"items": []string{"item1", "item2"}},
			Output: &core.Output{
				"outputs": map[string]any{
					"process-item-0": map[string]any{"result": "processed1"},
					"process-item-1": map[string]any{"result": "processed2"},
				},
				"collection_metadata": map[string]any{"item_count": 2},
			},
		}
		child1ID := core.MustNewID()
		child1State := &task.State{
			TaskID:        "process-item-0",
			TaskExecID:    child1ID,
			ParentStateID: &parentID,
			ExecutionType: task.ExecutionBasic,
			Status:        core.StatusSuccess,
			Input:         &core.Input{"item": "item1"},
			Output:        &core.Output{"result": "processed1"},
		}
		child2ID := core.MustNewID()
		child2State := &task.State{
			TaskID:        "process-item-1",
			TaskExecID:    child2ID,
			ParentStateID: &parentID,
			ExecutionType: task.ExecutionBasic,
			Status:        core.StatusSuccess,
			Input:         &core.Input{"item": "item2"},
			Output:        &core.Output{"result": "processed2"},
		}
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"process-items":  parentState,
				"process-item-0": child1State,
				"process-item-1": child2State,
			},
		}
		ctx := &NormalizationContext{
			WorkflowState: workflowState,
			TaskConfigs: map[string]*task.Config{
				"process-items": {
					BaseConfig: task.BaseConfig{
						ID:   "process-items",
						Type: task.TaskTypeCollection,
					},
				},
			},
		}
		context := cb.BuildContext(ctx)
		require.NotNil(t, context)
		tasksCtx := context["tasks"].(map[string]any)
		parentCtx := tasksCtx["process-items"].(map[string]any)
		children, ok := parentCtx["children"].(map[string]any)
		require.True(t, ok, "Parent task should have children property")
		assert.Len(t, children, 2, "Should have 2 children")
		child1Ctx, ok := children["process-item-0"].(map[string]any)
		require.True(t, ok, "Should have child context for process-item-0")
		assert.Equal(t, "process-item-0", child1Ctx["id"])
		assert.Equal(t, core.StatusSuccess, child1Ctx["status"])
		assert.Equal(t, &core.Input{"item": "item1"}, child1Ctx["input"])
		assert.Equal(t, core.Output{"result": "processed1"}, child1Ctx["output"])
		child2Ctx, ok := children["process-item-1"].(map[string]any)
		require.True(t, ok, "Should have child context for process-item-1")
		assert.Equal(t, "process-item-1", child2Ctx["id"])
		assert.Equal(t, core.StatusSuccess, child2Ctx["status"])
		assert.Equal(t, &core.Input{"item": "item2"}, child2Ctx["input"])
		assert.Equal(t, core.Output{"result": "processed2"}, child2Ctx["output"])
	})
	t.Run("Should handle nested children (grandchildren)", func(t *testing.T) {
		grandparentID := core.MustNewID()
		grandparentState := &task.State{
			TaskID:        "batch-processor",
			TaskExecID:    grandparentID,
			ExecutionType: task.ExecutionCollection,
			Status:        core.StatusRunning,
		}

		parentID := core.MustNewID()
		parentState := &task.State{
			TaskID:        "process-batch-0",
			TaskExecID:    parentID,
			ParentStateID: &grandparentID,
			ExecutionType: task.ExecutionCollection,
			Status:        core.StatusRunning,
		}

		childID := core.MustNewID()
		childState := &task.State{
			TaskID:        "process-item-0",
			TaskExecID:    childID,
			ParentStateID: &parentID,
			ExecutionType: task.ExecutionBasic,
			Status:        core.StatusSuccess,
			Output:        &core.Output{"result": "processed"},
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"batch-processor": grandparentState,
				"process-batch-0": parentState,
				"process-item-0":  childState,
			},
		}

		ctx := &NormalizationContext{
			WorkflowState: workflowState,
			TaskConfigs:   map[string]*task.Config{},
		}

		context := cb.BuildContext(ctx)
		require.NotNil(t, context)

		tasksCtx := context["tasks"].(map[string]any)
		grandparentCtx := tasksCtx["batch-processor"].(map[string]any)
		grandparentChildren := grandparentCtx["children"].(map[string]any)

		parentCtx := grandparentChildren["process-batch-0"].(map[string]any)
		parentChildren := parentCtx["children"].(map[string]any)

		childCtx := parentChildren["process-item-0"].(map[string]any)
		assert.Equal(t, "process-item-0", childCtx["id"])
		assert.Equal(t, core.Output{"result": "processed"}, childCtx["output"])
	})

	t.Run("Should not add children property for basic tasks", func(t *testing.T) {
		basicState := &task.State{
			TaskID:        "basic-task",
			TaskExecID:    core.MustNewID(),
			ExecutionType: task.ExecutionBasic,
			Status:        core.StatusSuccess,
			Output:        &core.Output{"result": "done"},
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"basic-task": basicState,
			},
		}

		ctx := &NormalizationContext{
			WorkflowState: workflowState,
			TaskConfigs:   map[string]*task.Config{},
		}

		context := cb.BuildContext(ctx)
		require.NotNil(t, context)

		tasksCtx := context["tasks"].(map[string]any)
		basicCtx := tasksCtx["basic-task"].(map[string]any)

		_, hasChildren := basicCtx["children"]
		assert.False(t, hasChildren, "Basic task should not have children property")
	})

	t.Run("Should handle failed child tasks", func(t *testing.T) {
		parentID := core.MustNewID()
		parentState := &task.State{
			TaskID:        "parallel-tasks",
			TaskExecID:    parentID,
			ExecutionType: task.ExecutionParallel,
			Status:        core.StatusFailed,
		}

		successChildID := core.MustNewID()
		successChild := &task.State{
			TaskID:        "task-1",
			TaskExecID:    successChildID,
			ParentStateID: &parentID,
			ExecutionType: task.ExecutionBasic,
			Status:        core.StatusSuccess,
			Output:        &core.Output{"result": "ok"},
		}

		failedChildID := core.MustNewID()
		failedChild := &task.State{
			TaskID:        "task-2",
			TaskExecID:    failedChildID,
			ParentStateID: &parentID,
			ExecutionType: task.ExecutionBasic,
			Status:        core.StatusFailed,
			Error:         &core.Error{Message: "Task failed", Code: "TASK_ERROR"},
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"parallel-tasks": parentState,
				"task-1":         successChild,
				"task-2":         failedChild,
			},
		}

		ctx := &NormalizationContext{
			WorkflowState: workflowState,
			TaskConfigs:   map[string]*task.Config{},
		}

		context := cb.BuildContext(ctx)
		require.NotNil(t, context)

		tasksCtx := context["tasks"].(map[string]any)
		parentCtx := tasksCtx["parallel-tasks"].(map[string]any)
		children := parentCtx["children"].(map[string]any)

		task1Ctx := children["task-1"].(map[string]any)
		assert.Equal(t, core.StatusSuccess, task1Ctx["status"])
		assert.Equal(t, core.Output{"result": "ok"}, task1Ctx["output"])

		task2Ctx := children["task-2"].(map[string]any)
		assert.Equal(t, core.StatusFailed, task2Ctx["status"])
		errorObj := task2Ctx["error"].(*core.Error)
		assert.Equal(t, "Task failed", errorObj.Message)
		assert.Equal(t, "TASK_ERROR", errorObj.Code)
	})

	t.Run("Should respect max depth limit", func(t *testing.T) {
		states := make(map[string]*task.State)
		var prevID *core.ID

		for i := 0; i < 15; i++ {
			taskID := core.MustNewID()
			state := &task.State{
				TaskID:        fmt.Sprintf("task-%d", i),
				TaskExecID:    taskID,
				ExecutionType: task.ExecutionCollection,
				Status:        core.StatusSuccess,
				ParentStateID: prevID,
			}
			states[state.TaskID] = state
			prevID = &taskID
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks:          states,
		}

		ctx := &NormalizationContext{
			WorkflowState: workflowState,
			TaskConfigs:   map[string]*task.Config{},
		}

		context := cb.BuildContext(ctx)
		require.NotNil(t, context)

		tasksCtx := context["tasks"].(map[string]any)
		currentCtx := tasksCtx["task-0"].(map[string]any)

		for i := 1; i < maxContextDepth; i++ {
			children, ok := currentCtx["children"].(map[string]any)
			require.True(t, ok, "Should have children at level %d", i)

			nextTaskID := fmt.Sprintf("task-%d", i)
			nextCtx, ok := children[nextTaskID].(map[string]any)
			require.True(t, ok, "Should have child %s at level %d", nextTaskID, i)
			currentCtx = nextCtx
		}

		children, ok := currentCtx["children"].(map[string]any)
		require.True(t, ok, "Should have children property at depth 9")

		// task-10 should exist but have empty children (depth limit reached)
		task10Ctx, ok := children["task-10"].(map[string]any)
		require.True(t, ok, "Should have task-10 as child")

		// task-10's children should be empty due to depth limit
		task10Children, ok := task10Ctx["children"].(map[string]any)
		assert.True(t, ok, "task-10 should have children property")
		assert.Empty(t, task10Children, "task-10's children should be empty at max depth")
	})
}

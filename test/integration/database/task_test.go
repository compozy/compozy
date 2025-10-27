package database_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/repo"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/test/helpers"
)

func TestMultiDriver_TaskOperations(t *testing.T) {
	forEachDriver(t, "TaskOperations", func(t *testing.T, _ string, provider *repo.Provider) {
		taskRepo := provider.NewTaskRepo()
		workflowRepo := provider.NewWorkflowRepo()

		t.Run("Should create and retrieve tasks", func(t *testing.T) {
			ctx := helpers.NewTestContext(t)
			execID := createWorkflowState(ctx, t, workflowRepo, "task-create-retrieve", core.StatusRunning)
			state := insertTaskState(ctx, t, taskRepo, execID, "task-create-retrieve", "root-task", nil)

			fetched, err := taskRepo.GetState(ctx, state.TaskExecID)
			require.NoError(t, err)
			assert.Equal(t, state.TaskExecID, fetched.TaskExecID)
			assert.Equal(t, core.StatusRunning, fetched.Status)
		})

		t.Run("Should list tasks by workflow", func(t *testing.T) {
			ctx := helpers.NewTestContext(t)
			execID := createWorkflowState(ctx, t, workflowRepo, "task-list-workflow", core.StatusRunning)
			insertTaskState(ctx, t, taskRepo, execID, "task-list-workflow", "task-1", nil)
			insertTaskState(ctx, t, taskRepo, execID, "task-list-workflow", "task-2", nil)
			insertTaskState(ctx, t, taskRepo, execID, "task-list-workflow", "task-3", nil)

			filter := &task.StateFilter{WorkflowExecID: &execID}
			states, err := taskRepo.ListStates(ctx, filter)
			require.NoError(t, err)
			require.Len(t, states, 3)

			taskIDs := []string{states[0].TaskID, states[1].TaskID, states[2].TaskID}
			assert.ElementsMatch(t, taskIDs, []string{"task-1", "task-2", "task-3"})
		})

		t.Run("Should list children of parent", func(t *testing.T) {
			ctx := helpers.NewTestContext(t)
			execID := createWorkflowState(ctx, t, workflowRepo, "task-list-children", core.StatusRunning)
			parent := insertTaskState(ctx, t, taskRepo, execID, "task-list-children", "parent", nil)
			childA := insertTaskState(ctx, t, taskRepo, execID, "task-list-children", "child-1", &parent.TaskExecID)
			childB := insertTaskState(ctx, t, taskRepo, execID, "task-list-children", "child-2", &parent.TaskExecID)

			children, err := taskRepo.ListChildren(ctx, parent.TaskExecID)
			require.NoError(t, err)
			require.Len(t, children, 2)

			var ids []core.ID
			for _, child := range children {
				ids = append(ids, child.TaskExecID)
			}
			assert.Contains(t, ids, childA.TaskExecID)
			assert.Contains(t, ids, childB.TaskExecID)
		})

		t.Run("Should handle deep hierarchy", func(t *testing.T) {
			ctx := helpers.NewTestContext(t)
			execID := createWorkflowState(ctx, t, workflowRepo, "task-deep-hierarchy", core.StatusRunning)
			parent := insertTaskState(ctx, t, taskRepo, execID, "task-deep-hierarchy", "parent", nil)
			child := insertTaskState(ctx, t, taskRepo, execID, "task-deep-hierarchy", "child", &parent.TaskExecID)
			grandchild := insertTaskState(
				ctx,
				t,
				taskRepo,
				execID,
				"task-deep-hierarchy",
				"grandchild",
				&child.TaskExecID,
			)

			children, err := taskRepo.ListChildren(ctx, parent.TaskExecID)
			require.NoError(t, err)
			require.Len(t, children, 1)
			assert.Equal(t, child.TaskExecID, children[0].TaskExecID)

			grandChildren, err := taskRepo.ListChildren(ctx, child.TaskExecID)
			require.NoError(t, err)
			require.Len(t, grandChildren, 1)
			assert.Equal(t, grandchild.TaskExecID, grandChildren[0].TaskExecID)
		})
	})
}

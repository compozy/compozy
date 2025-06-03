package store

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----
// Test Fixtures
// -----

func createTestWorkflowState() *workflow.State {
	stateID := workflow.StateID{
		WorkflowID:   "test-workflow",
		WorkflowExec: "exec-123",
	}
	input := core.Input{"key": "value"}
	return &workflow.State{
		Status:  core.StatusRunning,
		StateID: stateID,
		Input:   &input,
		Output:  nil,
		Error:   nil,
		Tasks:   make(map[string]*task.State),
	}
}

func createTestTaskState() *task.State {
	stateID := task.StateID{
		TaskID:     "test-task",
		TaskExecID: "task-exec-123",
	}
	agentID := "test-agent"
	input := core.Input{"task": "data"}
	return &task.State{
		Status:    core.StatusRunning,
		Component: core.ComponentAgent,
		StateID:   stateID,
		AgentID:   &agentID,
		ToolID:    nil,
		Input:     &input,
		Output:    nil,
		Error:     nil,
	}
}

func setupTestRepository(t *testing.T) *MemDBRepository {
	repo, err := NewMemDBRepository()
	require.NoError(t, err)
	return repo
}

// -----
// Workflow State Operations Tests
// -----

func TestMemDBRepository_UpsertState(t *testing.T) {
	t.Run("Should create workflow state successfully", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		state := createTestWorkflowState()
		err := repo.UpsertState(ctx, state)
		assert.NoError(t, err)
	})

	t.Run("Should update existing workflow state", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		state := createTestWorkflowState()

		err := repo.UpsertState(ctx, state)
		require.NoError(t, err)

		state.Status = core.StatusSuccess
		err = repo.UpsertState(ctx, state)
		assert.NoError(t, err)

		retrieved, err := repo.GetState(ctx, state.StateID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, retrieved.Status)
	})
}

func TestMemDBRepository_GetState(t *testing.T) {
	t.Run("Should get workflow state successfully", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		state := createTestWorkflowState()

		err := repo.UpsertState(ctx, state)
		require.NoError(t, err)

		retrieved, err := repo.GetState(ctx, state.StateID)

		assert.NoError(t, err)
		assert.Equal(t, state.StateID, retrieved.StateID)
		assert.Equal(t, state.Status, retrieved.Status)
	})

	t.Run("Should return error when state not found", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		stateID := workflow.StateID{
			WorkflowID:   "non-existent",
			WorkflowExec: "exec-456",
		}

		_, err := repo.GetState(ctx, stateID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workflow state not found")
	})
}

func TestMemDBRepository_DeleteState(t *testing.T) {
	t.Run("Should delete workflow state successfully", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		state := createTestWorkflowState()

		err := repo.UpsertState(ctx, state)
		require.NoError(t, err)

		err = repo.DeleteState(ctx, state.StateID)

		assert.NoError(t, err)

		_, err = repo.GetState(ctx, state.StateID)
		assert.Error(t, err)
	})

	t.Run("Should return error when deleting non-existent state", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		stateID := workflow.StateID{
			WorkflowID:   "non-existent",
			WorkflowExec: "exec-456",
		}

		err := repo.DeleteState(ctx, stateID)

		assert.Error(t, err)
	})
}

func TestMemDBRepository_ListStates(t *testing.T) {
	t.Run("Should list all workflow states without filter", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		state1 := createTestWorkflowState()
		state1.StateID.WorkflowExec = "exec-1"
		state2 := createTestWorkflowState()
		state2.StateID.WorkflowID = "test-workflow-2"
		state2.StateID.WorkflowExec = "exec-2"

		err := repo.UpsertState(ctx, state1)
		require.NoError(t, err)
		err = repo.UpsertState(ctx, state2)
		require.NoError(t, err)

		states, err := repo.ListStates(ctx, &workflow.StateFilter{})

		assert.NoError(t, err)
		assert.Len(t, states, 2)
	})

	t.Run("Should filter workflow states by status", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		state1 := createTestWorkflowState()
		state2 := createTestWorkflowState()
		state2.StateID.WorkflowID = "test-workflow-2"
		state2.Status = core.StatusSuccess

		err := repo.UpsertState(ctx, state1)
		require.NoError(t, err)
		err = repo.UpsertState(ctx, state2)
		require.NoError(t, err)

		status := core.StatusSuccess
		filter := &workflow.StateFilter{
			Status: &status,
		}
		states, err := repo.ListStates(ctx, filter)

		assert.NoError(t, err)
		assert.Len(t, states, 1)
		assert.Equal(t, core.StatusSuccess, states[0].Status)
	})
}

// -----
// Task Management Operations Tests
// -----

func TestMemDBRepository_AddTaskToWorkflow(t *testing.T) {
	t.Run("Should add task to workflow successfully", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		workflowState := createTestWorkflowState()
		taskState := createTestTaskState()

		err := repo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		err = repo.AddTaskToWorkflow(ctx, workflowState.StateID, taskState)

		assert.NoError(t, err)

		retrieved, err := repo.GetState(ctx, workflowState.StateID)
		require.NoError(t, err)
		assert.Len(t, retrieved.Tasks, 1)
		assert.Contains(t, retrieved.Tasks, taskState.StateID.String())
	})

	t.Run("Should return error when workflow not found", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		stateID := workflow.StateID{
			WorkflowID:   "non-existent",
			WorkflowExec: "exec-456",
		}
		taskState := createTestTaskState()

		err := repo.AddTaskToWorkflow(ctx, stateID, taskState)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workflow state not found")
	})
}

func TestMemDBRepository_RemoveTaskFromWorkflow(t *testing.T) {
	t.Run("Should remove task from workflow successfully", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		workflowState := createTestWorkflowState()
		taskState := createTestTaskState()

		err := repo.UpsertState(ctx, workflowState)
		require.NoError(t, err)
		err = repo.AddTaskToWorkflow(ctx, workflowState.StateID, taskState)
		require.NoError(t, err)

		err = repo.RemoveTaskFromWorkflow(ctx, workflowState.StateID, taskState.StateID)

		assert.NoError(t, err)

		retrieved, err := repo.GetState(ctx, workflowState.StateID)
		require.NoError(t, err)
		assert.Len(t, retrieved.Tasks, 0)
	})

	t.Run("Should return error when workflow not found", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		stateID := workflow.StateID{
			WorkflowID:   "non-existent",
			WorkflowExec: "exec-456",
		}
		taskState := createTestTaskState()

		err := repo.RemoveTaskFromWorkflow(ctx, stateID, taskState.StateID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workflow state not found")
	})
}

func TestMemDBRepository_UpdateTaskState(t *testing.T) {
	t.Run("Should update task state successfully", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		workflowState := createTestWorkflowState()
		taskState := createTestTaskState()

		err := repo.UpsertState(ctx, workflowState)
		require.NoError(t, err)
		err = repo.AddTaskToWorkflow(ctx, workflowState.StateID, taskState)
		require.NoError(t, err)

		taskState.Status = core.StatusSuccess
		err = repo.UpdateTaskState(ctx, workflowState.StateID, taskState.StateID, taskState)

		assert.NoError(t, err)

		retrieved, err := repo.GetTaskState(ctx, workflowState.StateID, taskState.StateID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, retrieved.Status)
	})

	t.Run("Should return error when workflow not found", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		stateID := workflow.StateID{
			WorkflowID:   "non-existent",
			WorkflowExec: "exec-456",
		}
		taskState := createTestTaskState()

		err := repo.UpdateTaskState(ctx, stateID, taskState.StateID, taskState)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workflow state not found")
	})
}

// -----
// Task Query Operations Tests
// -----

func TestMemDBRepository_GetTaskState(t *testing.T) {
	t.Run("Should get task state successfully", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		workflowState := createTestWorkflowState()
		taskState := createTestTaskState()

		err := repo.UpsertState(ctx, workflowState)
		require.NoError(t, err)
		err = repo.AddTaskToWorkflow(ctx, workflowState.StateID, taskState)
		require.NoError(t, err)

		retrieved, err := repo.GetTaskState(ctx, workflowState.StateID, taskState.StateID)

		assert.NoError(t, err)
		assert.Equal(t, taskState.StateID, retrieved.StateID)
		assert.Equal(t, taskState.Status, retrieved.Status)
	})

	t.Run("Should return error when task not found", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		workflowState := createTestWorkflowState()
		taskStateID := task.StateID{
			TaskID:     "non-existent",
			TaskExecID: "exec-456",
		}

		err := repo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		_, err = repo.GetTaskState(ctx, workflowState.StateID, taskStateID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task state not found")
	})
}

func TestMemDBRepository_GetTaskByID(t *testing.T) {
	t.Run("Should get task by ID successfully", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		workflowState := createTestWorkflowState()
		taskState := createTestTaskState()

		err := repo.UpsertState(ctx, workflowState)
		require.NoError(t, err)
		err = repo.AddTaskToWorkflow(ctx, workflowState.StateID, taskState)
		require.NoError(t, err)

		retrieved, err := repo.GetTaskByID(ctx, workflowState.StateID, taskState.StateID.TaskID)

		assert.NoError(t, err)
		assert.Equal(t, taskState.StateID.TaskID, retrieved.StateID.TaskID)
	})

	t.Run("Should return error when task not found", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		workflowState := createTestWorkflowState()

		err := repo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		_, err = repo.GetTaskByID(ctx, workflowState.StateID, "non-existent")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task not found")
	})
}

func TestMemDBRepository_GetTaskByAgentID(t *testing.T) {
	t.Run("Should get task by agent ID successfully", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		workflowState := createTestWorkflowState()
		taskState := createTestTaskState()

		err := repo.UpsertState(ctx, workflowState)
		require.NoError(t, err)
		err = repo.AddTaskToWorkflow(ctx, workflowState.StateID, taskState)
		require.NoError(t, err)

		retrieved, err := repo.GetTaskByAgentID(ctx, workflowState.StateID, *taskState.AgentID)

		assert.NoError(t, err)
		assert.Equal(t, *taskState.AgentID, *retrieved.AgentID)
	})

	t.Run("Should return error when task not found", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		workflowState := createTestWorkflowState()

		err := repo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		_, err = repo.GetTaskByAgentID(ctx, workflowState.StateID, "non-existent-agent")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task not found")
	})
}

// -----
// Task List Operations Tests
// -----

func TestMemDBRepository_ListTasksInWorkflow(t *testing.T) {
	t.Run("Should list all tasks in workflow", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		workflowState := createTestWorkflowState()
		taskState1 := createTestTaskState()
		taskState1.StateID.TaskExecID = "task-exec-1"
		taskState2 := createTestTaskState()
		taskState2.StateID.TaskID = "test-task-2"
		taskState2.StateID.TaskExecID = "task-exec-2"

		err := repo.UpsertState(ctx, workflowState)
		require.NoError(t, err)
		err = repo.AddTaskToWorkflow(ctx, workflowState.StateID, taskState1)
		require.NoError(t, err)
		err = repo.AddTaskToWorkflow(ctx, workflowState.StateID, taskState2)
		require.NoError(t, err)

		tasks, err := repo.ListTasksInWorkflow(ctx, workflowState.StateID)

		assert.NoError(t, err)
		assert.Len(t, tasks, 2)
		assert.Contains(t, tasks, taskState1.StateID.String())
		assert.Contains(t, tasks, taskState2.StateID.String())
	})

	t.Run("Should return empty map when no tasks", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		workflowState := createTestWorkflowState()

		err := repo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		tasks, err := repo.ListTasksInWorkflow(ctx, workflowState.StateID)

		assert.NoError(t, err)
		assert.Len(t, tasks, 0)
	})
}

func TestMemDBRepository_ListTasksByStatus(t *testing.T) {
	t.Run("Should list tasks by status", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		workflowState := createTestWorkflowState()
		taskState1 := createTestTaskState()
		taskState1.StateID.TaskExecID = "task-exec-status-1"
		taskState2 := createTestTaskState()
		taskState2.StateID.TaskID = "test-task-2"
		taskState2.StateID.TaskExecID = "task-exec-status-2"
		taskState2.Status = core.StatusSuccess

		err := repo.UpsertState(ctx, workflowState)
		require.NoError(t, err)
		err = repo.AddTaskToWorkflow(ctx, workflowState.StateID, taskState1)
		require.NoError(t, err)
		err = repo.AddTaskToWorkflow(ctx, workflowState.StateID, taskState2)
		require.NoError(t, err)

		tasks, err := repo.ListTasksByStatus(ctx, workflowState.StateID, core.StatusSuccess)

		assert.NoError(t, err)
		assert.Len(t, tasks, 1)
		assert.Equal(t, core.StatusSuccess, tasks[0].Status)
	})
}

func TestMemDBRepository_ListTasksByAgent(t *testing.T) {
	t.Run("Should list tasks by agent ID", func(t *testing.T) {
		repo := setupTestRepository(t)
		ctx := context.Background()
		workflowState := createTestWorkflowState()
		taskState1 := createTestTaskState()
		taskState1.StateID.TaskExecID = "task-exec-agent-1"
		taskState2 := createTestTaskState()
		taskState2.StateID.TaskID = "test-task-2"
		taskState2.StateID.TaskExecID = "task-exec-agent-2"
		agent2ID := "test-agent-2"
		taskState2.AgentID = &agent2ID

		err := repo.UpsertState(ctx, workflowState)
		require.NoError(t, err)
		err = repo.AddTaskToWorkflow(ctx, workflowState.StateID, taskState1)
		require.NoError(t, err)
		err = repo.AddTaskToWorkflow(ctx, workflowState.StateID, taskState2)
		require.NoError(t, err)

		tasks, err := repo.ListTasksByAgent(ctx, workflowState.StateID, "test-agent-2")

		assert.NoError(t, err)
		assert.Len(t, tasks, 1)
		assert.Equal(t, "test-agent-2", *tasks[0].AgentID)
	})
}

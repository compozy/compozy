package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	store "github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

func TestWorkflowRepoIntegration(t *testing.T) {
	t.Run("Should upsert and retrieve workflow state", func(t *testing.T) {
		env := newRepoTestEnv(t)
		truncateRepoTables(env.ctx, t, env.pool)

		workflowExecID := core.MustNewID()
		workflowID := "wf-upsert"
		input := core.Input{"config": "value"}
		state := &workflow.State{
			WorkflowExecID: workflowExecID,
			WorkflowID:     workflowID,
			Status:         core.StatusRunning,
			Input:          &input,
			Tasks:          make(map[string]*task.State),
		}

		require.NoError(t, env.workflowRepo.UpsertState(env.ctx, state))

		fetched, err := env.workflowRepo.GetState(env.ctx, workflowExecID)
		require.NoError(t, err)
		assert.Equal(t, workflowExecID, fetched.WorkflowExecID)
		assert.Equal(t, workflowID, fetched.WorkflowID)
		require.NotNil(t, fetched.Input)
		assert.Equal(t, "value", (*fetched.Input)["config"])
		assert.Empty(t, fetched.Tasks)

		output := core.Output{"result": "done"}
		state.Status = core.StatusSuccess
		state.Output = &output
		require.NoError(t, env.workflowRepo.UpsertState(env.ctx, state))

		updated, err := env.workflowRepo.GetState(env.ctx, workflowExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, updated.Status)
		require.NotNil(t, updated.Output)
		assert.Equal(t, "done", (*updated.Output)["result"])
	})

	t.Run("Should get workflow state with and without tasks", func(t *testing.T) {
		t.Run("Should get workflow state without associated tasks", func(t *testing.T) {
			env := newRepoTestEnv(t)
			truncateRepoTables(env.ctx, t, env.pool)

			workflowExecID := core.MustNewID()
			workflowID := "wf-no-tasks"
			upsertState := &workflow.State{
				WorkflowExecID: workflowExecID,
				WorkflowID:     workflowID,
				Status:         core.StatusPending,
				Tasks:          make(map[string]*task.State),
			}
			require.NoError(t, env.workflowRepo.UpsertState(env.ctx, upsertState))

			fetched, err := env.workflowRepo.GetState(env.ctx, workflowExecID)
			require.NoError(t, err)
			assert.Equal(t, workflowID, fetched.WorkflowID)
			assert.Empty(t, fetched.Tasks)
		})

		t.Run("Should populate workflow state with task hierarchy", func(t *testing.T) {
			env := newRepoTestEnv(t)
			truncateRepoTables(env.ctx, t, env.pool)

			workflowExecID := core.MustNewID()
			workflowID := "wf-with-tasks"
			upsertState := &workflow.State{
				WorkflowExecID: workflowExecID,
				WorkflowID:     workflowID,
				Status:         core.StatusRunning,
				Tasks:          make(map[string]*task.State),
			}
			require.NoError(t, env.workflowRepo.UpsertState(env.ctx, upsertState))

			agentID := "agent-workflow"
			actionID := "action-workflow"
			taskState := &task.State{
				Component:      core.ComponentAgent,
				Status:         core.StatusPending,
				TaskID:         "root-task",
				TaskExecID:     core.MustNewID(),
				WorkflowID:     workflowID,
				WorkflowExecID: workflowExecID,
				ExecutionType:  task.ExecutionBasic,
				AgentID:        &agentID,
				ActionID:       &actionID,
			}
			require.NoError(t, env.taskRepo.UpsertState(env.ctx, taskState))

			fetched, err := env.workflowRepo.GetState(env.ctx, workflowExecID)
			require.NoError(t, err)
			require.NotNil(t, fetched.Tasks)
			assert.Len(t, fetched.Tasks, 1)
			assert.Equal(t, agentID, *fetched.Tasks["root-task"].AgentID)
		})

		t.Run("Should return error when workflow state does not exist", func(t *testing.T) {
			env := newRepoTestEnv(t)
			truncateRepoTables(env.ctx, t, env.pool)

			_, err := env.workflowRepo.GetState(env.ctx, core.MustNewID())
			require.ErrorIs(t, err, store.ErrWorkflowNotFound)
		})
	})

	t.Run("Should retrieve workflow state by workflow ID", func(t *testing.T) {
		env := newRepoTestEnv(t)
		truncateRepoTables(env.ctx, t, env.pool)

		workflowExecID := core.MustNewID()
		workflowID := "wf-get-by-id"
		upsertState := &workflow.State{
			WorkflowExecID: workflowExecID,
			WorkflowID:     workflowID,
			Status:         core.StatusRunning,
			Tasks:          make(map[string]*task.State),
		}
		require.NoError(t, env.workflowRepo.UpsertState(env.ctx, upsertState))

		agentID := "agent-get-by-id"
		actionID := "action-get-by-id"
		taskState := &task.State{
			Component:      core.ComponentAgent,
			Status:         core.StatusPending,
			TaskID:         "task-get-by-id",
			TaskExecID:     core.MustNewID(),
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
			AgentID:        &agentID,
			ActionID:       &actionID,
		}
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, taskState))

		fetched, err := env.workflowRepo.GetStateByID(env.ctx, workflowID)
		require.NoError(t, err)
		assert.Equal(t, workflowExecID, fetched.WorkflowExecID)
		require.NotNil(t, fetched.Tasks)
		assert.Contains(t, fetched.Tasks, "task-get-by-id")
	})

	t.Run("Should list workflow states with filters and include tasks", func(t *testing.T) {
		env := newRepoTestEnv(t)
		truncateRepoTables(env.ctx, t, env.pool)

		runningExecID := core.MustNewID()
		successExecID := core.MustNewID()

		runningState := &workflow.State{
			WorkflowExecID: runningExecID,
			WorkflowID:     "wf-running",
			Status:         core.StatusRunning,
			Tasks:          make(map[string]*task.State),
		}
		successState := &workflow.State{
			WorkflowExecID: successExecID,
			WorkflowID:     "wf-success",
			Status:         core.StatusSuccess,
			Tasks:          make(map[string]*task.State),
		}
		require.NoError(t, env.workflowRepo.UpsertState(env.ctx, runningState))
		require.NoError(t, env.workflowRepo.UpsertState(env.ctx, successState))

		agentID := "agent-list"
		actionID := "action-list"
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, &task.State{
			Component:      core.ComponentAgent,
			Status:         core.StatusRunning,
			TaskID:         "task-list",
			TaskExecID:     core.MustNewID(),
			WorkflowID:     "wf-running",
			WorkflowExecID: runningExecID,
			ExecutionType:  task.ExecutionBasic,
			AgentID:        &agentID,
			ActionID:       &actionID,
		}))

		statusFilter := core.StatusRunning
		filter := &workflow.StateFilter{Status: &statusFilter}
		states, err := env.workflowRepo.ListStates(env.ctx, filter)
		require.NoError(t, err)
		require.Len(t, states, 1)
		assert.Equal(t, "wf-running", states[0].WorkflowID)
		require.NotNil(t, states[0].Tasks)
		assert.Len(t, states[0].Tasks, 1)
	})

	t.Run("Should update workflow status", func(t *testing.T) {
		env := newRepoTestEnv(t)
		truncateRepoTables(env.ctx, t, env.pool)

		workflowExecID := core.MustNewID()
		workflowID := "wf-update"
		initial := &workflow.State{
			WorkflowExecID: workflowExecID,
			WorkflowID:     workflowID,
			Status:         core.StatusRunning,
			Tasks:          make(map[string]*task.State),
		}
		require.NoError(t, env.workflowRepo.UpsertState(env.ctx, initial))

		require.NoError(t, env.workflowRepo.UpdateStatus(env.ctx, workflowExecID, core.StatusSuccess))

		fetched, err := env.workflowRepo.GetState(env.ctx, workflowExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, fetched.Status)
	})
}

package store

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	store "github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
)

func TestTaskRepoIntegration(t *testing.T) {
	t.Run("Should upsert and retrieve task state", func(t *testing.T) {
		env := newRepoTestEnv(t)

		workflowExecID := core.MustNewID()
		workflowID := "wf-upsert"
		input := core.Input{"key": "value"}
		upsertWorkflowState(t, env, workflowID, workflowExecID, &input)

		agentID := "agent-upsert"
		actionID := "default_action"
		taskExecID := core.MustNewID()
		state := &task.State{
			Component:      core.ComponentAgent,
			Status:         core.StatusPending,
			TaskID:         "task-upsert",
			TaskExecID:     taskExecID,
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
			AgentID:        &agentID,
			ActionID:       &actionID,
			Input:          &input,
		}

		require.NoError(t, env.taskRepo.UpsertState(env.ctx, state))

		stored, err := env.taskRepo.GetState(env.ctx, taskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusPending, stored.Status)
		assert.Equal(t, taskExecID, stored.TaskExecID)
		assert.Equal(t, actionID, *stored.ActionID)
		require.NotNil(t, stored.Input)
		assert.Equal(t, "value", (*stored.Input)["key"])

		state.Status = core.StatusSuccess
		output := core.Output{"result": "ok"}
		state.Output = &output
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, state))

		updated, err := env.taskRepo.GetState(env.ctx, taskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, updated.Status)
		require.NotNil(t, updated.Output)
		assert.Equal(t, "ok", (*updated.Output)["result"])
	})

	t.Run("Should manage transactions", func(t *testing.T) {
		t.Run("Should commit child states when closure succeeds", func(t *testing.T) {
			env := newRepoTestEnv(t)

			workflowExecID := core.MustNewID()
			workflowID := "wf-tx-success"
			upsertWorkflowState(t, env, workflowID, workflowExecID, nil)

			parentExecID := core.MustNewID()
			parentState := &task.State{
				Component:      core.ComponentTask,
				Status:         core.StatusRunning,
				TaskID:         "parent-task",
				TaskExecID:     parentExecID,
				WorkflowID:     workflowID,
				WorkflowExecID: workflowExecID,
				ExecutionType:  task.ExecutionParallel,
			}
			require.NoError(t, env.taskRepo.UpsertState(env.ctx, parentState))

			childOneExec := core.MustNewID()
			childTwoExec := core.MustNewID()
			parentPtr := parentExecID
			toolID := "tool-child"
			agentID := "agent-child"
			actionID := "child-action"
			childInput := core.Input{"task": "child"}

			err := env.taskRepo.WithTransaction(env.ctx, func(r task.Repository) error {
				stateOne := &task.State{
					Component:      core.ComponentAgent,
					Status:         core.StatusPending,
					TaskID:         "child-one",
					TaskExecID:     childOneExec,
					WorkflowID:     workflowID,
					WorkflowExecID: workflowExecID,
					ExecutionType:  task.ExecutionBasic,
					ParentStateID:  &parentPtr,
					AgentID:        &agentID,
					ActionID:       &actionID,
					Input:          &childInput,
				}
				if err := r.UpsertState(env.ctx, stateOne); err != nil {
					return err
				}
				stateTwo := &task.State{
					Component:      core.ComponentTool,
					Status:         core.StatusPending,
					TaskID:         "child-two",
					TaskExecID:     childTwoExec,
					WorkflowID:     workflowID,
					WorkflowExecID: workflowExecID,
					ExecutionType:  task.ExecutionBasic,
					ParentStateID:  &parentPtr,
					ToolID:         &toolID,
				}
				return r.UpsertState(env.ctx, stateTwo)
			})

			require.NoError(t, err)

			children, err := env.taskRepo.ListChildren(env.ctx, parentExecID)
			require.NoError(t, err)
			assert.Len(t, children, 2)

			outputs, err := env.taskRepo.ListChildrenOutputs(env.ctx, parentExecID)
			require.NoError(t, err)
			assert.Len(t, outputs, 0)
		})

		t.Run("Should rollback child states when closure fails", func(t *testing.T) {
			env := newRepoTestEnv(t)

			workflowExecID := core.MustNewID()
			workflowID := "wf-tx-rollback"
			upsertWorkflowState(t, env, workflowID, workflowExecID, nil)

			parentExecID := core.MustNewID()
			parentState := &task.State{
				Component:      core.ComponentTask,
				Status:         core.StatusRunning,
				TaskID:         "parent-task",
				TaskExecID:     parentExecID,
				WorkflowID:     workflowID,
				WorkflowExecID: workflowExecID,
				ExecutionType:  task.ExecutionParallel,
			}
			require.NoError(t, env.taskRepo.UpsertState(env.ctx, parentState))

			parentPtr := parentExecID
			childExec := core.MustNewID()
			agentID := "agent-rollback"
			actionID := "rollback-action"

			failingErr := errors.New("intentional failure")
			err := env.taskRepo.WithTransaction(env.ctx, func(r task.Repository) error {
				child := &task.State{
					Component:      core.ComponentAgent,
					Status:         core.StatusPending,
					TaskID:         "child-rollback",
					TaskExecID:     childExec,
					WorkflowID:     workflowID,
					WorkflowExecID: workflowExecID,
					ExecutionType:  task.ExecutionBasic,
					ParentStateID:  &parentPtr,
					AgentID:        &agentID,
					ActionID:       &actionID,
				}
				if err := r.UpsertState(env.ctx, child); err != nil {
					return err
				}
				return failingErr
			})

			require.ErrorIs(t, err, failingErr)

			children, err := env.taskRepo.ListChildren(env.ctx, parentExecID)
			require.NoError(t, err)
			assert.Empty(t, children)
		})
	})

	t.Run("Should get task state and handle not found", func(t *testing.T) {
		env := newRepoTestEnv(t)

		workflowExecID := core.MustNewID()
		workflowID := "wf-get"
		upsertWorkflowState(t, env, workflowID, workflowExecID, nil)

		agentID := "agent-get"
		actionID := "get-action"
		taskExecID := core.MustNewID()
		state := &task.State{
			Component:      core.ComponentAgent,
			Status:         core.StatusRunning,
			TaskID:         "task-get",
			TaskExecID:     taskExecID,
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
			AgentID:        &agentID,
			ActionID:       &actionID,
		}
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, state))

		fetched, err := env.taskRepo.GetState(env.ctx, taskExecID)
		require.NoError(t, err)
		assert.Equal(t, "task-get", fetched.TaskID)

		_, err = env.taskRepo.GetState(env.ctx, core.MustNewID())
		require.ErrorIs(t, err, store.ErrTaskNotFound)
	})

	t.Run("Should list task states using available filters", func(t *testing.T) {
		env := newRepoTestEnv(t)

		workflowExecID := core.MustNewID()
		workflowID := "wf-list"
		upsertWorkflowState(t, env, workflowID, workflowExecID, nil)

		agentID := "agent-list"
		actionID := "list-action"
		toolID := "tool-list"

		stateA := &task.State{
			Component:      core.ComponentAgent,
			Status:         core.StatusPending,
			TaskID:         "task-a",
			TaskExecID:     core.MustNewID(),
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
			AgentID:        &agentID,
			ActionID:       &actionID,
		}
		stateB := &task.State{
			Component:      core.ComponentTool,
			Status:         core.StatusRunning,
			TaskID:         "task-b",
			TaskExecID:     core.MustNewID(),
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
			ToolID:         &toolID,
		}
		otherWorkflowExec := core.MustNewID()
		upsertWorkflowState(t, env, "wf-other", otherWorkflowExec, nil)
		stateC := &task.State{
			Component:      core.ComponentAgent,
			Status:         core.StatusFailed,
			TaskID:         "task-c",
			TaskExecID:     core.MustNewID(),
			WorkflowID:     "wf-other",
			WorkflowExecID: otherWorkflowExec,
			ExecutionType:  task.ExecutionBasic,
			AgentID:        &agentID,
			ActionID:       &actionID,
		}

		require.NoError(t, env.taskRepo.UpsertState(env.ctx, stateA))
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, stateB))
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, stateC))

		tasksByWorkflow, err := env.taskRepo.ListTasksInWorkflow(env.ctx, workflowExecID)
		require.NoError(t, err)
		assert.Len(t, tasksByWorkflow, 2)
		assert.Equal(t, "task-a", tasksByWorkflow["task-a"].TaskID)
		assert.Equal(t, "task-b", tasksByWorkflow["task-b"].TaskID)

		byStatus, err := env.taskRepo.ListTasksByStatus(env.ctx, workflowExecID, core.StatusPending)
		require.NoError(t, err)
		require.Len(t, byStatus, 1)
		assert.Equal(t, "task-a", byStatus[0].TaskID)

		byAgent, err := env.taskRepo.ListTasksByAgent(env.ctx, workflowExecID, agentID)
		require.NoError(t, err)
		require.Len(t, byAgent, 1)
		assert.Equal(t, "task-a", byAgent[0].TaskID)

		byTool, err := env.taskRepo.ListTasksByTool(env.ctx, workflowExecID, toolID)
		require.NoError(t, err)
		require.Len(t, byTool, 1)
		assert.Equal(t, "task-b", byTool[0].TaskID)

		filter := &task.StateFilter{WorkflowExecID: &workflowExecID}
		filtered, err := env.taskRepo.ListStates(env.ctx, filter)
		require.NoError(t, err)
		assert.Len(t, filtered, 2)
	})

	t.Run("Should list children, child outputs, and fetch child by task id", func(t *testing.T) {
		env := newRepoTestEnv(t)

		workflowExecID := core.MustNewID()
		workflowID := "wf-children"
		upsertWorkflowState(t, env, workflowID, workflowExecID, nil)

		parentExecID := core.MustNewID()
		parentState := &task.State{
			Component:      core.ComponentTask,
			Status:         core.StatusRunning,
			TaskID:         "parent",
			TaskExecID:     parentExecID,
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionParallel,
		}
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, parentState))

		parentPtr := parentExecID
		agentID := "agent-child"
		actionID := "child-action"
		output := core.Output{"result": "ok"}
		childOne := &task.State{
			Component:      core.ComponentAgent,
			Status:         core.StatusSuccess,
			TaskID:         "child-1",
			TaskExecID:     core.MustNewID(),
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
			ParentStateID:  &parentPtr,
			AgentID:        &agentID,
			ActionID:       &actionID,
			Output:         &output,
		}
		toolID := "tool-child"
		childTwo := &task.State{
			Component:      core.ComponentTool,
			Status:         core.StatusRunning,
			TaskID:         "child-2",
			TaskExecID:     core.MustNewID(),
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
			ParentStateID:  &parentPtr,
			ToolID:         &toolID,
		}

		require.NoError(t, env.taskRepo.UpsertState(env.ctx, childOne))
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, childTwo))

		children, err := env.taskRepo.ListChildren(env.ctx, parentExecID)
		require.NoError(t, err)
		require.Len(t, children, 2)
		assert.Equal(t, parentExecID, *children[0].ParentStateID)

		outputs, err := env.taskRepo.ListChildrenOutputs(env.ctx, parentExecID)
		require.NoError(t, err)
		require.Len(t, outputs, 1)
		assert.Equal(t, "ok", (*outputs["child-1"])["result"])

		childByTask, err := env.taskRepo.GetChildByTaskID(env.ctx, parentExecID, "child-1")
		require.NoError(t, err)
		assert.Equal(t, "child-1", childByTask.TaskID)

		emptyParentID := core.MustNewID()
		children, err = env.taskRepo.ListChildren(env.ctx, emptyParentID)
		require.NoError(t, err)
		assert.Empty(t, children)
	})

	t.Run("Should build task tree for hierarchical executions", func(t *testing.T) {
		env := newRepoTestEnv(t)

		workflowExecID := core.MustNewID()
		workflowID := "wf-tree"
		upsertWorkflowState(t, env, workflowID, workflowExecID, nil)

		rootExecID := core.MustNewID()
		rootState := &task.State{
			Component:      core.ComponentTask,
			Status:         core.StatusRunning,
			TaskID:         "root",
			TaskExecID:     rootExecID,
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionParallel,
		}
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, rootState))

		rootPtr := rootExecID
		agentID := "agent-tree"
		actionID := "tree-action"
		childOneExec := core.MustNewID()
		childOne := &task.State{
			Component:      core.ComponentAgent,
			Status:         core.StatusRunning,
			TaskID:         "child-1",
			TaskExecID:     childOneExec,
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
			ParentStateID:  &rootPtr,
			AgentID:        &agentID,
			ActionID:       &actionID,
		}
		childTwo := &task.State{
			Component:      core.ComponentTask,
			Status:         core.StatusPending,
			TaskID:         "child-2",
			TaskExecID:     core.MustNewID(),
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
			ParentStateID:  &rootPtr,
		}
		grandParentPtr := childOneExec
		grandChild := &task.State{
			Component:      core.ComponentTool,
			Status:         core.StatusPending,
			TaskID:         "grandchild",
			TaskExecID:     core.MustNewID(),
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
			ParentStateID:  &grandParentPtr,
		}

		require.NoError(t, env.taskRepo.UpsertState(env.ctx, childOne))
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, childTwo))
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, grandChild))

		tree, err := env.taskRepo.GetTaskTree(env.ctx, rootExecID)
		require.NoError(t, err)
		require.Len(t, tree, 4)
		assert.Equal(t, "root", tree[0].TaskID)
		assert.Equal(t, rootExecID, tree[0].TaskExecID)
		assert.True(t, tree[0].IsParallelRoot())
		assert.Equal(t, "child-1", tree[1].TaskID)
		assert.Equal(t, "child-2", tree[2].TaskID)
		assert.Equal(t, "grandchild", tree[3].TaskID)

		emptyTree, err := env.taskRepo.GetTaskTree(env.ctx, core.MustNewID())
		require.NoError(t, err)
		assert.Empty(t, emptyTree)
	})

	t.Run("Should aggregate task progress information", func(t *testing.T) {
		env := newRepoTestEnv(t)

		workflowExecID := core.MustNewID()
		workflowID := "wf-progress"
		upsertWorkflowState(t, env, workflowID, workflowExecID, nil)

		parentExecID := core.MustNewID()
		parentState := &task.State{
			Component:      core.ComponentTask,
			Status:         core.StatusRunning,
			TaskID:         "parent",
			TaskExecID:     parentExecID,
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionParallel,
		}
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, parentState))

		parentPtr := parentExecID
		agentID := "agent-progress"
		actionID := "progress-action"
		successChild := &task.State{
			Component:      core.ComponentAgent,
			Status:         core.StatusSuccess,
			TaskID:         "child-success",
			TaskExecID:     core.MustNewID(),
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
			ParentStateID:  &parentPtr,
			AgentID:        &agentID,
			ActionID:       &actionID,
		}
		failedChild := &task.State{
			Component:      core.ComponentAgent,
			Status:         core.StatusFailed,
			TaskID:         "child-failed",
			TaskExecID:     core.MustNewID(),
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
			ParentStateID:  &parentPtr,
			AgentID:        &agentID,
			ActionID:       &actionID,
		}
		runningChild := &task.State{
			Component:      core.ComponentTool,
			Status:         core.StatusRunning,
			TaskID:         "child-running",
			TaskExecID:     core.MustNewID(),
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
			ParentStateID:  &parentPtr,
		}

		require.NoError(t, env.taskRepo.UpsertState(env.ctx, successChild))
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, failedChild))
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, runningChild))

		progressInfo, err := env.taskRepo.GetProgressInfo(env.ctx, parentExecID)
		require.NoError(t, err)
		require.NotNil(t, progressInfo)
		assert.Equal(t, 3, progressInfo.TotalChildren)
		assert.Equal(t, 1, progressInfo.SuccessCount)
		assert.Equal(t, 1, progressInfo.FailedCount)
		assert.Equal(t, 1, progressInfo.RunningCount)
		assert.True(t, progressInfo.HasFailures())
		assert.False(t, progressInfo.IsComplete(task.StrategyWaitAll))

		status := progressInfo.CalculateOverallStatus(task.StrategyWaitAll)
		assert.Equal(t, core.StatusRunning, status)

		progressInfo, err = env.taskRepo.GetProgressInfo(env.ctx, core.MustNewID())
		require.NoError(t, err)
		assert.Equal(t, 0, progressInfo.TotalChildren)
		assert.Equal(t, 0.0, progressInfo.CompletionRate)
		assert.Equal(t, 0.0, progressInfo.FailureRate)
		assert.Empty(t, progressInfo.StatusCounts)

		upsertWorkflowState(t, env, workflowID, workflowExecID, nil)
		parentExecID = core.MustNewID()
		parentState.TaskExecID = parentExecID
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, parentState))
		parentPtr = parentExecID
		successChild.TaskExecID = core.MustNewID()
		successChild.ParentStateID = &parentPtr
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, successChild))
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, &task.State{
			Component:      core.ComponentAgent,
			Status:         core.StatusSuccess,
			TaskID:         "child-success-2",
			TaskExecID:     core.MustNewID(),
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
			ParentStateID:  &parentPtr,
			AgentID:        &agentID,
			ActionID:       &actionID,
		}))

		progressInfo, err = env.taskRepo.GetProgressInfo(env.ctx, parentExecID)
		require.NoError(t, err)
		status = progressInfo.CalculateOverallStatus(task.StrategyWaitAll)
		assert.Equal(t, core.StatusSuccess, status)
		assert.True(t, progressInfo.IsComplete(task.StrategyWaitAll))
		assert.True(t, progressInfo.IsAllComplete())
	})
}

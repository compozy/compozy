package database_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/repo"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/helpers"
)

var driverMatrix = []string{"sqlite", "postgres"}

func forEachDriver(
	t *testing.T,
	name string,
	fn func(t *testing.T, driver string, provider *repo.Provider),
) {
	t.Helper()
	for _, driver := range driverMatrix {
		driver := driver
		t.Run(fmt.Sprintf("%s/%s", name, driver), func(t *testing.T) {
			provider, cleanup := helpers.SetupTestDatabase(t, driver)
			t.Cleanup(cleanup)
			fn(t, driver, provider)
		})
	}
}

func TestMultiDriver_WorkflowExecution(t *testing.T) {
	forEachDriver(t, "WorkflowExecution", func(t *testing.T, driver string, provider *repo.Provider) {
		t.Run("Should execute workflow end to end", func(t *testing.T) {
			ctx := helpers.NewTestContext(t)
			execID := createWorkflowState(ctx, t, provider.NewWorkflowRepo(), "workflow-end-to-end", core.StatusRunning)

			repo := provider.NewWorkflowRepo()
			state, err := repo.GetState(ctx, execID)
			require.NoError(t, err)
			assert.Equal(t, "workflow-end-to-end", state.WorkflowID)
			require.NoError(t, repo.UpdateStatus(ctx, execID, core.StatusSuccess))

			updated, err := repo.GetState(ctx, execID)
			require.NoError(t, err)
			assert.Equal(t, core.StatusSuccess, updated.Status)
		})

		t.Run("Should persist task hierarchy", func(t *testing.T) {
			ctx := helpers.NewTestContext(t)
			repo := provider.NewWorkflowRepo()
			taskRepo := provider.NewTaskRepo()
			execID := createWorkflowState(ctx, t, repo, "workflow-hierarchy", core.StatusRunning)

			parent := insertTaskState(ctx, t, taskRepo, execID, "workflow-hierarchy", "parent", nil)
			childA := insertTaskState(ctx, t, taskRepo, execID, "workflow-hierarchy", "child-a", &parent.TaskExecID)
			childB := insertTaskState(ctx, t, taskRepo, execID, "workflow-hierarchy", "child-b", &parent.TaskExecID)

			children, err := taskRepo.ListChildren(ctx, parent.TaskExecID)
			require.NoError(t, err)
			assert.Len(t, children, 2)
			childIDs := []core.ID{children[0].TaskExecID, children[1].TaskExecID}
			assert.Contains(t, childIDs, childA.TaskExecID)
			assert.Contains(t, childIDs, childB.TaskExecID)
		})

		t.Run("Should handle concurrent workflows", func(t *testing.T) {
			t.Parallel()
			ctx := helpers.NewTestContext(t)
			repo := provider.NewWorkflowRepo()
			concurrency := driverConcurrency(driver)
			var wg sync.WaitGroup
			errCh := make(chan error, concurrency)
			for i := 0; i < concurrency; i++ {
				index := i
				wg.Go(func() {
					execID := core.MustNewID()
					state := &workflow.State{
						WorkflowID:     fmt.Sprintf("concurrent-%d", index),
						WorkflowExecID: execID,
						Status:         core.StatusRunning,
					}
					if err := repo.UpsertState(ctx, state); err != nil {
						errCh <- err
						return
					}
					if err := repo.UpdateStatus(ctx, execID, core.StatusSuccess); err != nil {
						errCh <- err
					}
				})
			}
			wg.Wait()
			close(errCh)
			for err := range errCh {
				require.NoError(t, err)
			}
		})
	})
}

func driverConcurrency(driver string) int {
	if driver == "sqlite" {
		return 5
	}
	return 25
}

func insertTaskState(
	ctx context.Context,
	t *testing.T,
	repository task.Repository,
	workflowExecID core.ID,
	workflowID string,
	taskID string,
	parent *core.ID,
) *task.State {
	t.Helper()
	taskExecID := core.MustNewID()
	state := &task.State{
		Component:      core.ComponentTask,
		Status:         core.StatusRunning,
		TaskID:         taskID,
		TaskExecID:     taskExecID,
		WorkflowID:     workflowID,
		WorkflowExecID: workflowExecID,
		ExecutionType:  task.ExecutionBasic,
	}
	if parent != nil {
		state.ParentStateID = parent
	}
	require.NoError(t, repository.UpsertState(ctx, state))
	return state
}

func createWorkflowState(
	ctx context.Context,
	t *testing.T,
	repository workflow.Repository,
	workflowID string,
	status core.StatusType,
) core.ID {
	t.Helper()
	execID := core.MustNewID()
	input := core.NewInput(map[string]any{"origin": workflowID})
	state := &workflow.State{
		WorkflowID:     workflowID,
		WorkflowExecID: execID,
		Status:         status,
		Input:          &input,
		Tasks:          make(map[string]*task.State),
	}
	require.NoError(t, repository.UpsertState(ctx, state))
	return execID
}

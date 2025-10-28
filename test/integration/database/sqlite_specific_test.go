package database_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/test/helpers"
)

func TestSQLite_Specific(t *testing.T) {
	provider, cleanup := helpers.SetupTestDatabase(t, "sqlite")
	t.Cleanup(cleanup)

	t.Run("Should support in memory mode", func(t *testing.T) {
		ctx := helpers.NewTestContext(t)
		repo := provider.NewWorkflowRepo()
		execID := createWorkflowState(ctx, t, repo, "sqlite-memory", core.StatusRunning)

		state, err := repo.GetState(ctx, execID)
		require.NoError(t, err)
		assert.Equal(t, "sqlite-memory", state.WorkflowID)
	})

	t.Run("Should enforce foreign keys", func(t *testing.T) {
		ctx := helpers.NewTestContext(t)
		taskRepo := provider.NewTaskRepo()
		state := &task.State{
			Component:      core.ComponentTask,
			Status:         core.StatusRunning,
			TaskID:         "orphan-task",
			TaskExecID:     core.MustNewID(),
			WorkflowID:     "missing-workflow",
			WorkflowExecID: core.MustNewID(),
			ExecutionType:  task.ExecutionBasic,
		}
		err := taskRepo.UpsertState(ctx, state)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "FOREIGN KEY")
	})

	t.Run("Should handle concurrent reads", func(t *testing.T) {
		ctx := helpers.NewTestContext(t)
		repo := provider.NewWorkflowRepo()
		execID := createWorkflowState(ctx, t, repo, "sqlite-concurrent-read", core.StatusRunning)

		var wg sync.WaitGroup
		errCh := make(chan error, 20)
		for i := 0; i < 20; i++ {
			wg.Go(func() {
				_, err := repo.GetState(ctx, execID)
				if err != nil {
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

	t.Run("Should serialize concurrent writes", func(t *testing.T) {
		ctx := helpers.NewTestContext(t)
		repo := provider.NewWorkflowRepo()
		taskRepo := provider.NewTaskRepo()
		execID := createWorkflowState(ctx, t, repo, "sqlite-concurrent-write", core.StatusRunning)

		var wg sync.WaitGroup
		errCh := make(chan error, 5)
		for i := 0; i < 5; i++ {
			index := i
			wg.Go(func() {
				state := &task.State{
					Component:      core.ComponentTask,
					Status:         core.StatusRunning,
					TaskID:         "write-" + core.MustNewID().String(),
					TaskExecID:     core.MustNewID(),
					WorkflowID:     "sqlite-concurrent-write",
					WorkflowExecID: execID,
					ExecutionType:  task.ExecutionBasic,
				}
				if err := taskRepo.UpsertState(ctx, state); err != nil {
					errCh <- err
					return
				}
				if err := repo.UpdateStatus(ctx, execID, core.StatusSuccess); err != nil && index == 0 {
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
}

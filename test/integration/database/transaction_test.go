package database_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/repo"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/test/helpers"
)

func TestMultiDriver_Transactions(t *testing.T) {
	forEachDriver(t, "Transactions", func(t *testing.T, _ string, provider *repo.Provider) {
		taskRepo := provider.NewTaskRepo()
		workflowRepo := provider.NewWorkflowRepo()

		t.Run("Should rollback on error", func(t *testing.T) {
			ctx := helpers.NewTestContext(t)
			execID := createWorkflowState(ctx, t, workflowRepo, "tx-rollback", core.StatusRunning)
			target := &task.State{
				Component:      core.ComponentTask,
				Status:         core.StatusRunning,
				TaskID:         "rollback",
				TaskExecID:     core.MustNewID(),
				WorkflowID:     "tx-rollback",
				WorkflowExecID: execID,
				ExecutionType:  task.ExecutionBasic,
			}
			expectedErr := errors.New("rollback")
			err := taskRepo.WithTransaction(ctx, func(repo task.Repository) error {
				require.NoError(t, repo.UpsertState(ctx, target))
				return expectedErr
			})
			require.ErrorIs(t, err, expectedErr)

			_, err = taskRepo.GetState(ctx, target.TaskExecID)
			require.ErrorIs(t, err, store.ErrTaskNotFound)
		})

		t.Run("Should commit on success", func(t *testing.T) {
			ctx := helpers.NewTestContext(t)
			execID := createWorkflowState(ctx, t, workflowRepo, "tx-commit", core.StatusRunning)
			state := &task.State{
				Component:      core.ComponentTask,
				Status:         core.StatusRunning,
				TaskID:         "commit",
				TaskExecID:     core.MustNewID(),
				WorkflowID:     "tx-commit",
				WorkflowExecID: execID,
				ExecutionType:  task.ExecutionBasic,
			}
			err := taskRepo.WithTransaction(ctx, func(repo task.Repository) error {
				return repo.UpsertState(ctx, state)
			})
			require.NoError(t, err)

			fetched, err := taskRepo.GetState(ctx, state.TaskExecID)
			require.NoError(t, err)
			require.Equal(t, state.TaskID, fetched.TaskID)
		})

		t.Run("Should handle nested transactions", func(t *testing.T) {
			ctx := helpers.NewTestContext(t)
			execID := createWorkflowState(ctx, t, workflowRepo, "tx-nested", core.StatusRunning)
			parentID := core.MustNewID()
			childID := core.MustNewID()
			err := taskRepo.WithTransaction(ctx, func(repo task.Repository) error {
				parent := &task.State{
					Component:      core.ComponentTask,
					Status:         core.StatusRunning,
					TaskID:         "parent",
					TaskExecID:     parentID,
					WorkflowID:     "tx-nested",
					WorkflowExecID: execID,
					ExecutionType:  task.ExecutionBasic,
				}
				if err := repo.UpsertState(ctx, parent); err != nil {
					return err
				}
				return repo.WithTransaction(ctx, func(inner task.Repository) error {
					child := &task.State{
						Component:      core.ComponentTask,
						Status:         core.StatusRunning,
						TaskID:         "child",
						TaskExecID:     childID,
						WorkflowID:     "tx-nested",
						WorkflowExecID: execID,
						ExecutionType:  task.ExecutionBasic,
						ParentStateID:  &parentID,
					}
					return inner.UpsertState(ctx, child)
				})
			})
			require.NoError(t, err)

			children, err := taskRepo.ListChildren(ctx, parentID)
			require.NoError(t, err)
			require.Len(t, children, 1)
			require.Equal(t, childID, children[0].TaskExecID)
		})
	})
}

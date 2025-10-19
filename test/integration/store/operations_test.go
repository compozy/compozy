package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/postgres"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/helpers"
)

// setupTestWithSharedContainer creates a test database using the shared container pattern
// This is 70-90% faster than creating individual containers
func setupTestWithSharedContainer(t *testing.T) (context.Context, *pgxpool.Pool, func()) {
	ctx := t.Context()
	pool, cleanup := helpers.GetSharedPostgresDB(t)
	// ensure tables exist
	require.NoError(t, helpers.EnsureTablesExistForTest(pool))
	return ctx, pool, func() { cleanup() }
}

// TestStoreOperations_Integration tests comprehensive store repository operations
func TestStoreOperations_Integration(t *testing.T) {
	t.Run("Should perform complete auth repository operations", func(t *testing.T) {
		ctx, pool, cleanup := setupTestWithSharedContainer(t)
		defer cleanup()
		authRepo := postgres.NewAuthRepo(pool)

		// Test user creation
		userID := core.MustNewID()
		user := &model.User{
			ID:    userID,
			Email: "test@example.com",
			Role:  model.RoleAdmin,
		}

		err := authRepo.CreateUser(ctx, user)
		require.NoError(t, err)
		// Note: CreatedAt is set by the database trigger, not by the Go struct
		// So we need to retrieve the user to check the timestamp
		createdUser, err := authRepo.GetUserByID(ctx, userID)
		require.NoError(t, err)
		assert.True(t, createdUser.CreatedAt.After(time.Time{}), "CreatedAt should be set after creation")

		// Test user retrieval
		retrievedUser, err := authRepo.GetUserByID(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, retrievedUser.ID)
		assert.Equal(t, user.Email, retrievedUser.Email)
		assert.Equal(t, user.Role, retrievedUser.Role)

		// Test user retrieval by email
		retrievedByEmail, err := authRepo.GetUserByEmail(ctx, "test@example.com")
		require.NoError(t, err)
		assert.Equal(t, user.ID, retrievedByEmail.ID)

		// Test API key creation
		apiKeyID := core.MustNewID()
		apiKey := &model.APIKey{
			ID:          apiKeyID,
			UserID:      userID,
			Prefix:      "cpzy_",
			Hash:        []byte("test-hash"),
			Fingerprint: []byte("test-fingerprint"),
		}

		err = authRepo.CreateAPIKey(ctx, apiKey)
		require.NoError(t, err)
		// Note: CreatedAt is set by the database trigger, not by the Go struct
		// So we need to retrieve the key to check the timestamp
		createdKey, err := authRepo.GetAPIKeyByID(ctx, apiKeyID)
		require.NoError(t, err)
		assert.True(t, createdKey.CreatedAt.After(time.Time{}), "CreatedAt should be set after creation")

		// Test API key retrieval by ID
		retrievedKey, err := authRepo.GetAPIKeyByID(ctx, apiKeyID)
		require.NoError(t, err)
		assert.Equal(t, apiKey.ID, retrievedKey.ID)
		assert.Equal(t, apiKey.UserID, retrievedKey.UserID)
		assert.Equal(t, apiKey.Prefix, retrievedKey.Prefix)

		// Test API key listing
		keys, err := authRepo.ListAPIKeysByUserID(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, keys, 1)
		assert.Equal(t, apiKey.ID, keys[0].ID)

		// Test API key deletion
		err = authRepo.DeleteAPIKey(ctx, apiKey.ID)
		require.NoError(t, err)

		// Verify key is deleted
		_, err = authRepo.GetAPIKeyByID(ctx, apiKeyID)
		assert.Error(t, err, "should return error for deleted API key")
	})

	t.Run("Should perform complete task repository operations", func(t *testing.T) {
		ctx, pool, cleanup := setupTestWithSharedContainer(t)
		defer cleanup()
		taskRepo := postgres.NewTaskRepo(pool)
		workflowRepo := postgres.NewWorkflowRepo(pool)

		// First create a workflow state (required for foreign key constraint)
		workflowExecID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowExecID: workflowExecID,
			WorkflowID:     "test-workflow",
			Status:         core.StatusRunning,
			Input:          &core.Input{"config": "test"},
			Tasks:          make(map[string]*task.State),
		}
		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Test task state creation
		taskExecID := core.MustNewID()
		taskState := &task.State{
			Component:      core.ComponentTask,
			Status:         core.StatusRunning,
			TaskID:         "test-task",
			TaskExecID:     taskExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
			Input:          &core.Input{"param": "value"},
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		err = taskRepo.UpsertState(ctx, taskState)
		require.NoError(t, err)

		// Test task state retrieval
		retrievedState, err := taskRepo.GetState(ctx, taskExecID)
		require.NoError(t, err)
		assert.Equal(t, taskState.TaskID, retrievedState.TaskID)
		assert.Equal(t, taskState.Status, retrievedState.Status)
		assert.Equal(t, taskState.Input, retrievedState.Input)

		// Test task state listing with filters
		filter := &task.StateFilter{
			WorkflowExecID: &workflowExecID,
			Status:         &taskState.Status,
		}
		states, err := taskRepo.ListStates(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, states, 1)
		assert.Equal(t, taskState.TaskID, states[0].TaskID)

		// Test task state update
		taskState.Status = core.StatusSuccess
		taskState.Output = &core.Output{"result": "success"}
		taskState.UpdatedAt = time.Now()

		err = taskRepo.UpsertState(ctx, taskState)
		require.NoError(t, err)

		// Verify update
		updatedState, err := taskRepo.GetState(ctx, taskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, updatedState.Status)
		assert.Equal(t, taskState.Output, updatedState.Output)
	})

	t.Run("Should perform complete workflow repository operations", func(t *testing.T) {
		ctx, pool, cleanup := setupTestWithSharedContainer(t)
		defer cleanup()
		workflowRepo := postgres.NewWorkflowRepo(pool)

		// Test workflow state creation
		workflowExecID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowExecID: workflowExecID,
			WorkflowID:     "test-workflow",
			Status:         core.StatusRunning,
			Input:          &core.Input{"config": "test"},
			Tasks:          make(map[string]*task.State),
		}

		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Test workflow state retrieval
		retrievedState, err := workflowRepo.GetState(ctx, workflowState.WorkflowExecID)
		require.NoError(t, err)
		assert.Equal(t, workflowState.WorkflowExecID, retrievedState.WorkflowExecID)
		assert.Equal(t, workflowState.Status, retrievedState.Status)
		assert.Equal(t, workflowState.Input, retrievedState.Input)

		// Test workflow state listing with filters
		filter := &workflow.StateFilter{
			WorkflowExecID: &workflowState.WorkflowExecID, // Use WorkflowExecID for unique filtering
			Status:         &workflowState.Status,
		}
		states, err := workflowRepo.ListStates(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, states, 1)
		assert.Equal(t, workflowState.WorkflowExecID, states[0].WorkflowExecID)

		// Test workflow state update
		workflowState.Status = core.StatusSuccess
		workflowState.Output = &core.Output{"result": "completed"}

		err = workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Verify update
		updatedState, err := workflowRepo.GetState(ctx, workflowState.WorkflowExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, updatedState.Status)
		assert.Equal(t, workflowState.Output, updatedState.Output)

		// Test workflow completion (without complex output transformer)
		completedState, err := workflowRepo.CompleteWorkflow(ctx, workflowState.WorkflowExecID, nil)
		require.NoError(t, err)
		assert.NotNil(t, completedState)
		assert.Equal(t, core.StatusSuccess, completedState.Status)
	})

	t.Run("Should handle concurrent repository operations", func(t *testing.T) {
		ctx, pool, cleanup := setupTestWithSharedContainer(t)
		defer cleanup()

		authRepo := postgres.NewAuthRepo(pool)

		// Test concurrent user creation
		numUsers := 10
		userIDs := make([]core.ID, numUsers)

		// Create users concurrently
		errChan := make(chan error, numUsers)
		for i := range numUsers {
			go func(index int) {
				userID := core.MustNewID()
				userIDs[index] = userID
				user := &model.User{
					ID:    userID,
					Email: fmt.Sprintf("user%d@example.com", index),
					Role:  model.RoleUser,
				}
				errChan <- authRepo.CreateUser(ctx, user)
			}(i)
		}

		// Wait for all operations to complete
		for range numUsers {
			err := <-errChan
			require.NoError(t, err, "concurrent user creation should succeed")
		}

		// Verify all users were created
		for i, userID := range userIDs {
			require.NotEmpty(t, userID, "userID at index %d should not be empty", i)
			user, err := authRepo.GetUserByID(ctx, userID)
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("user%d@example.com", i), user.Email)
		}
	})

	t.Run("Should handle error scenarios gracefully", func(t *testing.T) {
		ctx, pool, cleanup := setupTestWithSharedContainer(t)
		defer cleanup()
		authRepo := postgres.NewAuthRepo(pool)
		taskRepo := postgres.NewTaskRepo(pool)

		// Test duplicate user creation
		userID := core.MustNewID()
		user := &model.User{
			ID:    userID,
			Email: "duplicate@example.com",
			Role:  model.RoleAdmin,
		}

		err := authRepo.CreateUser(ctx, user)
		require.NoError(t, err)

		// Attempt to create the same user again
		err = authRepo.CreateUser(ctx, user)
		assert.Error(t, err, "should return error for duplicate user creation")

		// Test retrieval of non-existent records
		_, err = authRepo.GetUserByID(ctx, core.MustNewID())
		assert.Error(t, err, "should return error for non-existent user")

		_, err = taskRepo.GetState(ctx, core.MustNewID())
		assert.Error(t, err, "should return error for non-existent task state")

		// Test duplicate user creation (same ID)
		duplicateUser := &model.User{
			ID:    userID, // Reuse the same ID from earlier test
			Email: "another@example.com",
			Role:  model.RoleAdmin,
		}
		err = authRepo.CreateUser(ctx, duplicateUser)
		assert.Error(t, err, "should return error for duplicate user ID")

		// Test duplicate email creation (same email)
		duplicateEmailUser := &model.User{
			ID:    core.MustNewID(),
			Email: "duplicate@example.com", // Reuse email from earlier test
			Role:  model.RoleAdmin,
		}
		err = authRepo.CreateUser(ctx, duplicateEmailUser)
		assert.Error(t, err, "should return error for duplicate email")
	})
}

// TestMain handles shared container lifecycle for all tests in this package
func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()

	// Cleanup shared container
	helpers.CleanupSharedContainer(context.Background())

	os.Exit(code)
}

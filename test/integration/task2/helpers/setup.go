package helpers

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
	utils "github.com/compozy/compozy/test/helpers"
	"github.com/compozy/compozy/test/integration/worker/helpers"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestSetup contains all the common test infrastructure
type TestSetup struct {
	Context             context.Context
	Pool                *pgxpool.Pool
	TaskRepo            task.Repository
	WorkflowRepo        workflow.Repository
	TemplateEngine      *tplengine.TemplateEngine
	ContextBuilder      *shared.ContextBuilder
	ParentStatusManager *MockParentStatusManager
	OutputTransformer   *MockOutputTransformer
	BaseHandler         *shared.BaseResponseHandler
	DBHelper            *helpers.DatabaseHelper
}

// NewTestSetup creates a new test setup with all common infrastructure
// This version uses shared database resources for optimal performance
func NewTestSetup(t *testing.T) *TestSetup {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}
	ctx := t.Context()
	taskRepo, workflowRepo, cleanup := getSharedTestRepos(ctx, t)
	t.Cleanup(cleanup)
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder := &shared.ContextBuilder{}
	parentStatusManager := &MockParentStatusManager{}
	outputTransformer := &MockOutputTransformer{}
	baseHandler := shared.NewBaseResponseHandler(
		templateEngine,
		contextBuilder,
		parentStatusManager,
		workflowRepo,
		taskRepo,
		outputTransformer,
	)
	return &TestSetup{
		Context:             ctx,
		Pool:                nil, // Pool managed by shared helper
		TaskRepo:            taskRepo,
		WorkflowRepo:        workflowRepo,
		TemplateEngine:      templateEngine,
		ContextBuilder:      contextBuilder,
		ParentStatusManager: parentStatusManager,
		OutputTransformer:   outputTransformer,
		BaseHandler:         baseHandler,
		DBHelper:            nil, // Managed by shared helper
	}
}

// getSharedTestRepos provides shared database resources across all tests
// This eliminates the need for individual container creation
func getSharedTestRepos(ctx context.Context, t *testing.T) (task.Repository, workflow.Repository, func()) {
	return utils.SetupTestRepos(ctx, t)
}

// CreateWorkflowState creates and saves a workflow state for testing
func (ts *TestSetup) CreateWorkflowState(t *testing.T, workflowID string) (*workflow.State, core.ID) {
	workflowExecID := core.MustNewID()
	workflowState := &workflow.State{
		WorkflowID:     workflowID,
		WorkflowExecID: workflowExecID,
		Status:         core.StatusPending,
	}
	err := ts.WorkflowRepo.UpsertState(ts.Context, workflowState)
	require.NoError(t, err)
	return workflowState, workflowExecID
}

// CreateTaskState creates and saves a task state for testing
func (ts *TestSetup) CreateTaskState(t *testing.T, config *TaskStateConfig) *task.State {
	taskState := &task.State{
		TaskExecID:     core.MustNewID(),
		WorkflowID:     config.WorkflowID,
		WorkflowExecID: config.WorkflowExecID,
		TaskID:         config.TaskID,
		Status:         config.Status,
		Output:         config.Output,
		ParentStateID:  config.ParentStateID,
	}
	err := ts.TaskRepo.UpsertState(ts.Context, taskState)
	require.NoError(t, err)
	saved, err := ts.TaskRepo.GetState(ts.Context, taskState.TaskExecID)
	require.NoError(t, err, "Failed to verify saved state")
	require.Equal(t, taskState.TaskExecID, saved.TaskExecID, "TaskExecID mismatch after save")
	require.Equal(t, taskState.WorkflowID, saved.WorkflowID, "WorkflowID mismatch after save")
	require.Equal(t, taskState.WorkflowExecID, saved.WorkflowExecID, "WorkflowExecID mismatch after save")
	require.Equal(t, taskState.TaskID, saved.TaskID, "TaskID mismatch after save")
	require.Equal(t, taskState.Status, saved.Status, "Status mismatch after save")
	require.Equal(t, taskState.ParentStateID, saved.ParentStateID, "ParentStateID mismatch after save")
	if taskState.Output == nil {
		require.Nil(t, saved.Output, "Output should be nil")
	} else {
		require.NotNil(t, saved.Output, "Output should not be nil")
	}
	return taskState
}

// TaskStateConfig holds configuration for creating a test task state
type TaskStateConfig struct {
	WorkflowID     string
	WorkflowExecID core.ID
	TaskID         string
	Status         core.StatusType
	Output         *core.Output
	ParentStateID  *core.ID
}

// GetSavedTaskState retrieves a task state from the database
func (ts *TestSetup) GetSavedTaskState(t *testing.T, taskExecID core.ID) *task.State {
	savedState, err := ts.TaskRepo.GetState(ts.Context, taskExecID)
	if err != nil {
		t.Logf("Failed to get task state with ID %s: %v", taskExecID, err)
	}
	require.NoError(t, err)
	return savedState
}

package helpers

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
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
func NewTestSetup(t *testing.T) *TestSetup {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}

	// Setup test infrastructure
	dbHelper := helpers.NewDatabaseHelper(t)
	t.Cleanup(func() {
		dbHelper.Cleanup(t)
	})

	ctx := context.Background()
	pool := dbHelper.GetPool()

	// Create real repository instances
	taskRepo := store.NewTaskRepo(pool)
	workflowRepo := store.NewWorkflowRepo(pool)

	// Create handler dependencies
	templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
	contextBuilder := &shared.ContextBuilder{}
	parentStatusManager := &MockParentStatusManager{}
	outputTransformer := &MockOutputTransformer{}

	// Create base handler with real repositories
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
		Pool:                pool,
		TaskRepo:            taskRepo,
		WorkflowRepo:        workflowRepo,
		TemplateEngine:      templateEngine,
		ContextBuilder:      contextBuilder,
		ParentStatusManager: parentStatusManager,
		OutputTransformer:   outputTransformer,
		BaseHandler:         baseHandler,
		DBHelper:            dbHelper,
	}
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
	require.NoError(t, err)
	return savedState
}

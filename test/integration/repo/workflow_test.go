package store

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	utils "github.com/compozy/compozy/test/helpers"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
)

func TestWorkflowRepo_UpsertState(t *testing.T) {
	mockSetup := utils.NewMockSetup(t)
	defer mockSetup.Close()

	repo := store.NewWorkflowRepo(mockSetup.Mock)
	ctx := context.Background()
	state := &workflow.State{
		WorkflowID:     "wf1",
		WorkflowExecID: core.ID("exec1"),
		Status:         core.StatusPending,
		Input:          &core.Input{"key": "value"},
		Tasks:          make(map[string]*task.State),
	}

	dataBuilder := utils.NewDataBuilder()
	inputJSON := dataBuilder.MustCreateInputData(map[string]any{"key": "value"})
	expectedOutputJSON := dataBuilder.MustCreateNilJSONB()
	expectedErrorJSON := dataBuilder.MustCreateNilJSONB()

	queries := mockSetup.NewQueryExpectations()
	queries.ExpectWorkflowStateQueryForUpsert([]any{
		state.WorkflowExecID, state.WorkflowID, state.Status,
		inputJSON,
		expectedOutputJSON,
		expectedErrorJSON,
	})

	err := repo.UpsertState(ctx, state)
	assert.NoError(t, err)
	mockSetup.ExpectationsWereMet()
}

func TestWorkflowRepo_GetState(t *testing.T) {
	mockSetup := utils.NewMockSetup(t)
	defer mockSetup.Close()

	repo := store.NewWorkflowRepo(mockSetup.Mock)
	ctx := context.Background()
	workflowExecID := core.ID("exec1")

	dataBuilder := utils.NewDataBuilder()
	inputData := dataBuilder.MustCreateInputData(map[string]any{"key": "value"})

	// Use utility functions to set up the test
	tx := mockSetup.NewTransactionExpectations()
	queries := mockSetup.NewQueryExpectations()
	workflowRowBuilder := mockSetup.NewWorkflowStateRowBuilder()
	taskRowBuilder := mockSetup.NewTaskStateRowBuilder()

	tx.ExpectBegin()

	// Create workflow state rows
	workflowRows := workflowRowBuilder.CreateWorkflowStateRows(
		"exec1", "wf1", core.StatusPending, inputData,
	)

	// Expect workflow query
	queries.ExpectWorkflowStateQuery(
		"SELECT workflow_exec_id, workflow_id, status, input, output, error FROM workflow_states",
		[]any{workflowExecID},
		workflowRows,
	)

	// Create empty task rows and expect task query
	taskRows := taskRowBuilder.CreateEmptyTaskStateRows()
	queries.ExpectTaskStateQuery(workflowExecID, taskRows)

	tx.ExpectCommit()

	state, err := repo.GetState(ctx, workflowExecID)
	assert.NoError(t, err)
	assert.Equal(t, "wf1", state.WorkflowID)
	assert.Equal(t, core.StatusPending, state.Status)
	assert.NotNil(t, state.Input)
	assert.NotNil(t, state.Tasks)
	assert.Len(t, state.Tasks, 0) // No tasks in this test
	mockSetup.ExpectationsWereMet()
}

func TestWorkflowRepo_GetState_WithTasks(t *testing.T) {
	mockSetup := utils.NewMockSetup(t)
	defer mockSetup.Close()

	repo := store.NewWorkflowRepo(mockSetup.Mock)
	ctx := context.Background()
	workflowExecID := core.ID("exec1")

	dataBuilder := utils.NewDataBuilder()
	inputData := dataBuilder.MustCreateInputData(map[string]any{"key": "value"})
	taskInputData := dataBuilder.MustCreateInputData(map[string]any{"task_key": "task_value"})

	tx := mockSetup.NewTransactionExpectations()
	queries := mockSetup.NewQueryExpectations()
	workflowRowBuilder := mockSetup.NewWorkflowStateRowBuilder()
	taskRowBuilder := mockSetup.NewTaskStateRowBuilder()

	tx.ExpectBegin()

	// Create workflow state rows
	workflowRows := workflowRowBuilder.CreateWorkflowStateRows(
		"exec1", "wf1", core.StatusPending, inputData,
	)

	// Expect workflow query
	queries.ExpectWorkflowStateQuery(
		"SELECT workflow_exec_id, workflow_id, status, input, output, error FROM workflow_states",
		[]any{workflowExecID},
		workflowRows,
	)

	// Create task rows with data - use string directly instead of pointer
	taskRows := taskRowBuilder.CreateTaskStateRowsWithExecution(
		"task_exec1", "task1", "exec1", "wf1",
		core.StatusPending, task.ExecutionBasic, "agent1", nil, taskInputData,
	)
	queries.ExpectTaskStateQuery(workflowExecID, taskRows)

	tx.ExpectCommit()

	state, err := repo.GetState(ctx, workflowExecID)
	assert.NoError(t, err)
	assert.Equal(t, "wf1", state.WorkflowID)
	assert.Equal(t, core.StatusPending, state.Status)
	assert.NotNil(t, state.Input)
	assert.NotNil(t, state.Tasks)
	assert.Len(t, state.Tasks, 1)
	assert.Contains(t, state.Tasks, "task1")
	assert.Equal(t, "agent1", *state.Tasks["task1"].AgentID)
	mockSetup.ExpectationsWereMet()
}

func TestWorkflowRepo_GetState_NotFound(t *testing.T) {
	mockSetup := utils.NewMockSetup(t)
	defer mockSetup.Close()

	repo := store.NewWorkflowRepo(mockSetup.Mock)
	ctx := context.Background()
	workflowExecID := core.ID("exec1")

	tx := mockSetup.NewTransactionExpectations()
	tx.ExpectBegin()

	mockSetup.Mock.ExpectQuery("SELECT workflow_exec_id, workflow_id, status, input, output, error FROM workflow_states").
		WithArgs(workflowExecID).
		WillReturnError(pgx.ErrNoRows)

	tx.ExpectRollback()

	_, err := repo.GetState(ctx, workflowExecID)
	assert.ErrorIs(t, err, store.ErrWorkflowNotFound)
	mockSetup.ExpectationsWereMet()
}

// Helper function for simpler Get tests
func testSimpleWorkflowGet(
	t *testing.T,
	testName string,
	setupAndRun func(*utils.MockSetup, *store.WorkflowRepo, context.Context),
) {
	mockSetup := utils.NewMockSetup(t)
	defer mockSetup.Close()

	repo := store.NewWorkflowRepo(mockSetup.Mock)
	ctx := context.Background()

	t.Run(testName, func(_ *testing.T) {
		setupAndRun(mockSetup, repo, ctx)
		mockSetup.ExpectationsWereMet()
	})
}

func TestWorkflowRepo_GetStateByID(t *testing.T) {
	testSimpleWorkflowGet(
		t,
		"should get state by ID",
		func(mockSetup *utils.MockSetup, repo *store.WorkflowRepo, ctx context.Context) {
			workflowID := "wf1"

			tx := mockSetup.NewTransactionExpectations()
			queries := mockSetup.NewQueryExpectations()
			workflowRowBuilder := mockSetup.NewWorkflowStateRowBuilder()
			taskRowBuilder := mockSetup.NewTaskStateRowBuilder()

			tx.ExpectBegin()

			workflowRows := workflowRowBuilder.CreateWorkflowStateRows("exec1", "wf1", core.StatusPending, nil)
			queries.ExpectWorkflowStateQuery(
				"SELECT workflow_exec_id, workflow_id, status, input, output, error FROM workflow_states",
				[]any{workflowID},
				workflowRows,
			)

			taskRows := taskRowBuilder.CreateEmptyTaskStateRows()
			queries.ExpectTaskStateQuery(core.ID("exec1"), taskRows)

			tx.ExpectCommit()

			state, err := repo.GetStateByID(ctx, workflowID)
			assert.NoError(t, err)
			assert.Equal(t, workflowID, state.WorkflowID)
			assert.NotNil(t, state.Tasks)
		},
	)
}

// Continue with similar simplified patterns for other tests...
func TestWorkflowRepo_GetStateByTaskID(t *testing.T) {
	testSimpleWorkflowGet(
		t,
		"should get state by task ID",
		func(mockSetup *utils.MockSetup, repo *store.WorkflowRepo, ctx context.Context) {
			workflowID := "wf1"
			taskID := "task1"

			tx := mockSetup.NewTransactionExpectations()
			queries := mockSetup.NewQueryExpectations()
			workflowRowBuilder := mockSetup.NewWorkflowStateRowBuilder()
			taskRowBuilder := mockSetup.NewTaskStateRowBuilder()

			tx.ExpectBegin()

			workflowRows := workflowRowBuilder.CreateWorkflowStateRows("exec1", "wf1", core.StatusPending, nil)
			queries.ExpectWorkflowStateQuery(
				"SELECT w.workflow_exec_id, w.workflow_id, w.status, w.input, w.output, w.error FROM workflow_states w",
				[]any{workflowID, taskID},
				workflowRows,
			)

			taskRows := taskRowBuilder.CreateEmptyTaskStateRows()
			queries.ExpectTaskStateQuery(core.ID("exec1"), taskRows)

			tx.ExpectCommit()

			state, err := repo.GetStateByTaskID(ctx, workflowID, taskID)
			assert.NoError(t, err)
			assert.Equal(t, workflowID, state.WorkflowID)
			assert.NotNil(t, state.Tasks)
		},
	)
}

func TestWorkflowRepo_GetStateByAgentID(t *testing.T) {
	testSimpleWorkflowGet(
		t,
		"should get state by agent ID",
		func(mockSetup *utils.MockSetup, repo *store.WorkflowRepo, ctx context.Context) {
			workflowID := "wf1"
			agentID := "agent1"

			tx := mockSetup.NewTransactionExpectations()
			queries := mockSetup.NewQueryExpectations()
			workflowRowBuilder := mockSetup.NewWorkflowStateRowBuilder()
			taskRowBuilder := mockSetup.NewTaskStateRowBuilder()

			tx.ExpectBegin()

			workflowRows := workflowRowBuilder.CreateWorkflowStateRows("exec1", "wf1", core.StatusPending, nil)
			queries.ExpectWorkflowStateQuery(
				"SELECT w.workflow_exec_id, w.workflow_id, w.status, w.input, w.output, w.error FROM workflow_states w",
				[]any{workflowID, agentID},
				workflowRows,
			)

			taskRows := taskRowBuilder.CreateEmptyTaskStateRows()
			queries.ExpectTaskStateQuery(core.ID("exec1"), taskRows)

			tx.ExpectCommit()

			state, err := repo.GetStateByAgentID(ctx, workflowID, agentID)
			assert.NoError(t, err)
			assert.Equal(t, workflowID, state.WorkflowID)
			assert.NotNil(t, state.Tasks)
		},
	)
}

func TestWorkflowRepo_GetStateByToolID(t *testing.T) {
	testSimpleWorkflowGet(
		t,
		"should get state by tool ID",
		func(mockSetup *utils.MockSetup, repo *store.WorkflowRepo, ctx context.Context) {
			workflowID := "wf1"
			toolID := "tool1"

			tx := mockSetup.NewTransactionExpectations()
			queries := mockSetup.NewQueryExpectations()
			workflowRowBuilder := mockSetup.NewWorkflowStateRowBuilder()
			taskRowBuilder := mockSetup.NewTaskStateRowBuilder()

			tx.ExpectBegin()

			workflowRows := workflowRowBuilder.CreateWorkflowStateRows("exec1", "wf1", core.StatusPending, nil)
			queries.ExpectWorkflowStateQuery(
				"SELECT w.workflow_exec_id, w.workflow_id, w.status, w.input, w.output, w.error FROM workflow_states w",
				[]any{workflowID, toolID},
				workflowRows,
			)

			taskRows := taskRowBuilder.CreateEmptyTaskStateRows()
			queries.ExpectTaskStateQuery(core.ID("exec1"), taskRows)

			tx.ExpectCommit()

			state, err := repo.GetStateByToolID(ctx, workflowID, toolID)
			assert.NoError(t, err)
			assert.Equal(t, workflowID, state.WorkflowID)
			assert.NotNil(t, state.Tasks)
		},
	)
}

func TestWorkflowRepo_UpdateStatus(t *testing.T) {
	mockSetup := utils.NewMockSetup(t)
	defer mockSetup.Close()

	repo := store.NewWorkflowRepo(mockSetup.Mock)
	ctx := context.Background()
	workflowExecID := "exec1"
	newStatus := core.StatusRunning

	// Expect the update query with 1 row affected (success)
	mockSetup.Mock.ExpectExec("UPDATE workflow_states").
		WithArgs(newStatus, workflowExecID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := repo.UpdateStatus(ctx, workflowExecID, newStatus)
	assert.NoError(t, err)
	mockSetup.ExpectationsWereMet()
}

func TestWorkflowRepo_UpdateStatus_NotFound(t *testing.T) {
	mockSetup := utils.NewMockSetup(t)
	defer mockSetup.Close()

	repo := store.NewWorkflowRepo(mockSetup.Mock)
	ctx := context.Background()
	workflowExecID := "nonexistent"
	newStatus := core.StatusRunning

	// Expect the update query with 0 rows affected (not found)
	mockSetup.Mock.ExpectExec("UPDATE workflow_states").
		WithArgs(newStatus, workflowExecID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err := repo.UpdateStatus(ctx, workflowExecID, newStatus)
	assert.ErrorIs(t, err, store.ErrWorkflowNotFound)
	mockSetup.ExpectationsWereMet()
}

package store

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	testutils "github.com/compozy/compozy/test"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

func TestTaskRepo_UpsertState(t *testing.T) {
	mockSetup := testutils.NewMockSetup(t)
	defer mockSetup.Close()

	repo := store.NewTaskRepo(mockSetup.Mock)
	ctx := context.Background()
	workflowID := "wf1"
	workflowExecID := core.ID("exec1")
	agentID := "agent1"
	state := &task.State{
		StateID: task.StateID{TaskExecID: core.ID("task_exec1"), TaskID: "task1"},
		Status:  core.StatusPending,
		AgentID: &agentID,
		Input:   &core.Input{"key": "value"},
	}

	dataBuilder := testutils.NewDataBuilder()
	inputJSON := dataBuilder.MustCreateInputData(map[string]any{"key": "value"})
	expectedOutputJSON := dataBuilder.MustCreateNilJSONB()
	expectedErrorJSON := dataBuilder.MustCreateNilJSONB()

	queries := mockSetup.NewQueryExpectations()
	queries.ExpectTaskStateQueryForUpsert([]any{
		state.TaskExecID, state.TaskID, workflowExecID, workflowID, state.Status,
		state.AgentID, state.ToolID, inputJSON, // Use actual input data
		expectedOutputJSON,
		expectedErrorJSON,
	})

	err := repo.UpsertState(ctx, workflowID, workflowExecID, state)
	assert.NoError(t, err)
	mockSetup.ExpectationsWereMet()
}

// Helper function for task Get tests
func testTaskGet(t *testing.T, testName string, setupAndRun func(*testutils.MockSetup, *store.TaskRepo, context.Context)) {
	mockSetup := testutils.NewMockSetup(t)
	defer mockSetup.Close()

	repo := store.NewTaskRepo(mockSetup.Mock)
	ctx := context.Background()

	t.Run(testName, func(_ *testing.T) {
		setupAndRun(mockSetup, repo, ctx)
		mockSetup.ExpectationsWereMet()
	})
}

func TestTaskRepo_GetState(t *testing.T) {
	testTaskGet(t, "should get task state", func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
		workflowID := "wf1"
		workflowExecID := core.ID("exec1")
		taskStateID := task.StateID{TaskExecID: core.ID("task_exec1"), TaskID: "task1"}

		dataBuilder := testutils.NewDataBuilder()
		inputData := dataBuilder.MustCreateInputData(map[string]any{"key": "value"})

		taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
		taskRows := taskRowBuilder.CreateTaskStateRows(
			"task_exec1", "task1", "exec1", "wf1",
			core.StatusPending, "agent1", nil, inputData,
		)

		mockSetup.Mock.ExpectQuery("SELECT task_exec_id, task_id, workflow_exec_id, workflow_id").
			WithArgs(taskStateID.TaskExecID, workflowID, workflowExecID).
			WillReturnRows(taskRows)

		state, err := repo.GetState(ctx, workflowID, workflowExecID, taskStateID)
		assert.NoError(t, err)
		assert.Equal(t, taskStateID.TaskExecID, state.TaskExecID)
		assert.Equal(t, core.StatusPending, state.Status)
		assert.NotNil(t, state.Input)
		assert.Equal(t, "agent1", *state.AgentID)
	})
}

func TestTaskRepo_GetState_NotFound(t *testing.T) {
	testTaskGet(t, "should return not found error", func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
		workflowID := "wf1"
		workflowExecID := core.ID("exec1")
		taskStateID := task.StateID{TaskExecID: core.ID("task_exec1"), TaskID: "task1"}

		mockSetup.Mock.ExpectQuery("SELECT task_exec_id, task_id, workflow_exec_id, workflow_id").
			WithArgs(taskStateID.TaskExecID, workflowID, workflowExecID).
			WillReturnError(pgx.ErrNoRows)

		_, err := repo.GetState(ctx, workflowID, workflowExecID, taskStateID)
		assert.ErrorIs(t, err, store.TaskErrNotFound)
	})
}

func TestTaskRepo_GetTaskByID(t *testing.T) {
	testTaskGet(t, "should get task by ID", func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
		workflowID := "wf1"
		workflowExecID := core.ID("exec1")
		taskID := "task1"

		taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
		taskRows := taskRowBuilder.CreateTaskStateRows(
			"task_exec1", "task1", "exec1", "wf1",
			core.StatusPending, nil, nil, nil,
		)

		mockSetup.Mock.ExpectQuery("SELECT task_exec_id, task_id, workflow_exec_id, workflow_id").
			WithArgs(taskID, workflowID, workflowExecID).
			WillReturnRows(taskRows)

		state, err := repo.GetTaskByID(ctx, workflowID, workflowExecID, taskID)
		assert.NoError(t, err)
		assert.Equal(t, taskID, state.TaskID)
	})
}

func TestTaskRepo_GetTaskByExecID(t *testing.T) {
	testTaskGet(t, "should get task by exec ID", func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
		workflowID := "wf1"
		workflowExecID := core.ID("exec1")
		taskExecID := core.ID("task_exec1")

		taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
		taskRows := taskRowBuilder.CreateTaskStateRows(
			"task_exec1", "task1", "exec1", "wf1",
			core.StatusPending, nil, nil, nil,
		)

		mockSetup.Mock.ExpectQuery("SELECT task_exec_id, task_id, workflow_exec_id, workflow_id").
			WithArgs(taskExecID, workflowID, workflowExecID).
			WillReturnRows(taskRows)

		state, err := repo.GetTaskByExecID(ctx, workflowID, workflowExecID, taskExecID)
		assert.NoError(t, err)
		assert.Equal(t, taskExecID, state.TaskExecID)
	})
}

func TestTaskRepo_GetTaskByAgentID(t *testing.T) {
	testTaskGet(t, "should get task by agent ID", func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
		workflowID := "wf1"
		workflowExecID := core.ID("exec1")
		agentID := "agent1"

		taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
		taskRows := taskRowBuilder.CreateTaskStateRows(
			"task_exec1", "task1", "exec1", "wf1",
			core.StatusPending, agentID, nil, nil,
		)

		mockSetup.Mock.ExpectQuery("SELECT task_exec_id, task_id, workflow_exec_id, workflow_id").
			WithArgs(agentID, workflowID, workflowExecID).
			WillReturnRows(taskRows)

		state, err := repo.GetTaskByAgentID(ctx, workflowID, workflowExecID, agentID)
		assert.NoError(t, err)
		assert.Equal(t, "agent1", *state.AgentID)
	})
}

func TestTaskRepo_GetTaskByToolID(t *testing.T) {
	testTaskGet(t, "should get task by tool ID", func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
		workflowID := "wf1"
		workflowExecID := core.ID("exec1")
		toolID := "tool1"

		taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
		taskRows := taskRowBuilder.CreateTaskStateRows(
			"task_exec1", "task1", "exec1", "wf1",
			core.StatusPending, nil, toolID, nil,
		)

		mockSetup.Mock.ExpectQuery("SELECT task_exec_id, task_id, workflow_exec_id, workflow_id").
			WithArgs(toolID, workflowID, workflowExecID).
			WillReturnRows(taskRows)

		state, err := repo.GetTaskByToolID(ctx, workflowID, workflowExecID, toolID)
		assert.NoError(t, err)
		assert.Equal(t, "tool1", *state.ToolID)
	})
}

// Helper for list tests that return multiple states
func testTaskList(t *testing.T, testName string, setupAndRun func(*testutils.MockSetup, *store.TaskRepo, context.Context)) {
	mockSetup := testutils.NewMockSetup(t)
	defer mockSetup.Close()

	repo := store.NewTaskRepo(mockSetup.Mock)
	ctx := context.Background()

	t.Run(testName, func(_ *testing.T) {
		setupAndRun(mockSetup, repo, ctx)
		mockSetup.ExpectationsWereMet()
	})
}

func TestTaskRepo_ListTasksInWorkflow(t *testing.T) {
	testTaskList(t, "should list tasks in workflow", func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
		workflowExecID := core.ID("exec1")
		agentID := "agent1"
		toolID := "tool1"

		taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
		taskRows := taskRowBuilder.CreateEmptyTaskStateRows().
			AddRow("task_exec1", "task1", "exec1", "wf1", core.StatusPending, agentID, nil, nil, nil, nil).
			AddRow("task_exec2", "task2", "exec1", "wf1", core.StatusRunning, nil, toolID, nil, nil, nil)

		mockSetup.Mock.ExpectQuery("SELECT task_exec_id, task_id, workflow_exec_id, workflow_id").
			WithArgs(workflowExecID).
			WillReturnRows(taskRows)

		states, err := repo.ListTasksInWorkflow(ctx, workflowExecID)
		assert.NoError(t, err)
		assert.Len(t, states, 2)
		assert.Contains(t, states, "task1")
		assert.Contains(t, states, "task2")
		assert.Equal(t, "agent1", *states["task1"].AgentID)
		assert.Equal(t, "tool1", *states["task2"].ToolID)
	})
}

func TestTaskRepo_ListTasksByStatus(t *testing.T) {
	testTaskList(t, "should list tasks by status", func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
		workflowExecID := core.ID("exec1")
		status := core.StatusPending

		taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
		taskRows := taskRowBuilder.CreateTaskStateRows(
			"task_exec1", "task1", "exec1", "wf1",
			core.StatusPending, nil, nil, nil,
		)

		mockSetup.Mock.ExpectQuery("SELECT task_exec_id, task_id, workflow_exec_id, workflow_id").
			WithArgs(workflowExecID, status).
			WillReturnRows(taskRows)

		states, err := repo.ListTasksByStatus(ctx, workflowExecID, status)
		assert.NoError(t, err)
		assert.Len(t, states, 1)
		assert.Equal(t, core.StatusPending, states[0].Status)
	})
}

func TestTaskRepo_ListTasksByAgent(t *testing.T) {
	testTaskList(t, "should list tasks by agent", func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
		workflowExecID := core.ID("exec1")
		agentID := "agent1"

		taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
		taskRows := taskRowBuilder.CreateTaskStateRows(
			"task_exec1", "task1", "exec1", "wf1",
			core.StatusPending, agentID, nil, nil,
		)

		mockSetup.Mock.ExpectQuery("SELECT task_exec_id, task_id, workflow_exec_id, workflow_id").
			WithArgs(workflowExecID, agentID).
			WillReturnRows(taskRows)

		states, err := repo.ListTasksByAgent(ctx, workflowExecID, agentID)
		assert.NoError(t, err)
		assert.Len(t, states, 1)
		assert.Equal(t, "agent1", *states[0].AgentID)
	})
}

func TestTaskRepo_ListTasksByTool(t *testing.T) {
	testTaskList(t, "should list tasks by tool", func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
		workflowExecID := core.ID("exec1")
		toolID := "tool1"

		taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
		taskRows := taskRowBuilder.CreateTaskStateRows(
			"task_exec1", "task1", "exec1", "wf1",
			core.StatusPending, nil, toolID, nil,
		)

		mockSetup.Mock.ExpectQuery("SELECT task_exec_id, task_id, workflow_exec_id, workflow_id").
			WithArgs(workflowExecID, toolID).
			WillReturnRows(taskRows)

		states, err := repo.ListTasksByTool(ctx, workflowExecID, toolID)
		assert.NoError(t, err)
		assert.Len(t, states, 1)
		assert.Equal(t, "tool1", *states[0].ToolID)
	})
}

func TestTaskRepo_ListStates(t *testing.T) {
	testTaskList(t, "should list states with filter", func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
		filter := &task.StateFilter{
			Status:         &[]core.StatusType{core.StatusPending}[0],
			WorkflowExecID: &[]core.ID{core.ID("exec1")}[0],
		}

		taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
		taskRows := taskRowBuilder.CreateTaskStateRows(
			"task_exec1", "task1", "exec1", "wf1",
			core.StatusPending, nil, nil, nil,
		)

		mockSetup.Mock.ExpectQuery("SELECT task_exec_id, task_id, workflow_exec_id, workflow_id").
			WithArgs(core.StatusPending, core.ID("exec1")).
			WillReturnRows(taskRows)

		states, err := repo.ListStates(ctx, filter)
		assert.NoError(t, err)
		assert.Len(t, states, 1)
		assert.Equal(t, core.StatusPending, states[0].Status)
	})
}

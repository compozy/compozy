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
	actionID := "default_action"
	state := &task.State{
		TaskExecID:     core.ID("task_exec1"),
		TaskID:         "task1",
		Component:      core.ComponentAgent,
		Status:         core.StatusPending,
		WorkflowID:     workflowID,
		WorkflowExecID: workflowExecID,
		ExecutionType:  task.ExecutionBasic,
		AgentID:        &agentID,
		ActionID:       &actionID,
		Input:          &core.Input{"key": "value"},
	}

	dataBuilder := testutils.NewDataBuilder()
	inputJSON := dataBuilder.MustCreateInputData(map[string]any{"key": "value"})
	expectedOutputJSON := dataBuilder.MustCreateNilJSONB()
	expectedErrorJSON := dataBuilder.MustCreateNilJSONB()
	expectedParallelStateJSON := dataBuilder.MustCreateNilJSONB()

	queries := mockSetup.NewQueryExpectations()
	queries.ExpectTaskStateQueryForUpsert([]any{
		state.TaskExecID, state.TaskID, state.WorkflowExecID, state.WorkflowID, state.Component, state.Status,
		state.ExecutionType, state.AgentID, state.ActionID, state.ToolID, inputJSON,
		expectedOutputJSON,
		expectedErrorJSON,
		expectedParallelStateJSON,
	})

	err := repo.UpsertState(ctx, state)
	assert.NoError(t, err)
	mockSetup.ExpectationsWereMet()
}

func TestTaskRepo_UpsertParallelState(t *testing.T) {
	mockSetup := testutils.NewMockSetup(t)
	defer mockSetup.Close()

	repo := store.NewTaskRepo(mockSetup.Mock)
	ctx := context.Background()
	workflowID := "wf1"
	workflowExecID := core.ID("exec1")

	// Create a parallel state
	parallelState := &task.ParallelState{
		Strategy:   task.StrategyWaitAll,
		MaxWorkers: 3,
		Timeout:    "5m",
		SubTasks: map[string]*task.State{
			"subtask1": {
				TaskID:         "subtask1",
				TaskExecID:     core.MustNewID(),
				WorkflowID:     workflowID,
				WorkflowExecID: workflowExecID,
				Component:      core.ComponentAgent,
				Status:         core.StatusPending,
				ExecutionType:  task.ExecutionBasic,
				AgentID:        testutils.StringPtr("agent1"),
				ActionID:       testutils.StringPtr("action1"),
				Input:          &core.Input{"param": "value1"},
			},
		},
		CompletedTasks: make([]string, 0),
		FailedTasks:    make([]string, 0),
	}

	state := &task.State{
		TaskExecID:     core.ID("task_exec1"),
		TaskID:         "parallel_task1",
		Component:      core.ComponentTask,
		Status:         core.StatusRunning,
		WorkflowID:     workflowID,
		WorkflowExecID: workflowExecID,
		ExecutionType:  task.ExecutionParallel,
		ParallelState:  parallelState,
	}

	dataBuilder := testutils.NewDataBuilder()
	expectedInputJSON := dataBuilder.MustCreateNilJSONB()
	expectedOutputJSON := dataBuilder.MustCreateNilJSONB()
	expectedErrorJSON := dataBuilder.MustCreateNilJSONB()
	parallelStateJSON := dataBuilder.MustCreateParallelStateData(parallelState)

	queries := mockSetup.NewQueryExpectations()
	queries.ExpectTaskStateQueryForUpsert([]any{
		state.TaskExecID, state.TaskID, state.WorkflowExecID, state.WorkflowID, state.Component, state.Status,
		state.ExecutionType, state.AgentID, state.ActionID, state.ToolID, expectedInputJSON,
		expectedOutputJSON,
		expectedErrorJSON,
		parallelStateJSON,
	})

	err := repo.UpsertState(ctx, state)
	assert.NoError(t, err)
	mockSetup.ExpectationsWereMet()
}

// Helper function for task Get tests
func testTaskGet(
	t *testing.T,
	testName string,
	setupAndRun func(*testutils.MockSetup, *store.TaskRepo, context.Context),
) {
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
	testTaskGet(
		t,
		"should get basic task state",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			taskExecID := core.ID("task_exec1")

			dataBuilder := testutils.NewDataBuilder()
			inputData := dataBuilder.MustCreateInputData(map[string]any{"key": "value"})

			taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
			taskRows := taskRowBuilder.CreateTaskStateRowsWithExecution(
				"task_exec1", "task1", "exec1", "wf1",
				core.StatusPending, task.ExecutionBasic, "agent1", nil, inputData,
			)

			mockSetup.Mock.ExpectQuery("SELECT \\*").
				WithArgs(taskExecID).
				WillReturnRows(taskRows)

			state, err := repo.GetState(ctx, taskExecID)
			assert.NoError(t, err)
			assert.Equal(t, taskExecID, state.TaskExecID)
			assert.Equal(t, core.StatusPending, state.Status)
			assert.Equal(t, task.ExecutionBasic, state.ExecutionType)
			assert.NotNil(t, state.Input)
			assert.Equal(t, "agent1", *state.AgentID)
		},
	)
}

func TestTaskRepo_GetParallelState(t *testing.T) {
	testTaskGet(
		t,
		"should get parallel task state",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			taskExecID := core.ID("task_exec1")

			// Create parallel state data
			parallelState := &task.ParallelState{
				Strategy:   task.StrategyWaitAll,
				MaxWorkers: 2,
				SubTasks: map[string]*task.State{
					"subtask1": {
						TaskID:         "subtask1",
						TaskExecID:     core.MustNewID(),
						WorkflowID:     "wf1",
						WorkflowExecID: core.ID("exec1"),
						Component:      core.ComponentAgent,
						Status:         core.StatusSuccess,
						ExecutionType:  task.ExecutionBasic,
						Output:         &core.Output{"result": "success"},
					},
				},
				CompletedTasks: []string{"subtask1"},
				FailedTasks:    make([]string, 0),
			}

			dataBuilder := testutils.NewDataBuilder()
			parallelStateData := dataBuilder.MustCreateParallelStateData(parallelState)

			taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
			taskRows := taskRowBuilder.CreateParallelTaskStateRows(
				"task_exec1", "parallel_task1", "exec1", "wf1",
				core.StatusSuccess, task.ExecutionParallel, parallelStateData,
			)

			mockSetup.Mock.ExpectQuery("SELECT \\*").
				WithArgs(taskExecID).
				WillReturnRows(taskRows)

			state, err := repo.GetState(ctx, taskExecID)
			assert.NoError(t, err)
			assert.Equal(t, taskExecID, state.TaskExecID)
			assert.Equal(t, core.StatusSuccess, state.Status)
			assert.Equal(t, task.ExecutionParallel, state.ExecutionType)
			assert.True(t, state.IsParallel())
			assert.NotNil(t, state.ParallelState)
			assert.Equal(t, task.StrategyWaitAll, state.ParallelState.Strategy)
			assert.Equal(t, 2, state.ParallelState.MaxWorkers)
			assert.Len(t, state.SubTasks, 1)
			assert.Contains(t, state.SubTasks, "subtask1")
		},
	)
}

func TestTaskRepo_GetState_NotFound(t *testing.T) {
	testTaskGet(
		t,
		"should return not found error",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			taskExecID := core.ID("task_exec1")

			mockSetup.Mock.ExpectQuery("SELECT \\*").
				WithArgs(taskExecID).
				WillReturnError(pgx.ErrNoRows)

			_, err := repo.GetState(ctx, taskExecID)
			assert.ErrorIs(t, err, store.ErrTaskNotFound)
		},
	)
}

// Helper for list tests that return multiple states
func testTaskList(
	t *testing.T,
	testName string,
	setupAndRun func(*testutils.MockSetup, *store.TaskRepo, context.Context),
) {
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
	testTaskList(
		t,
		"should list tasks in workflow",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			workflowExecID := core.ID("exec1")
			agentID := "agent1"
			toolID := "tool1"

			taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
			taskRows := taskRowBuilder.CreateEmptyTaskStateRows().
				AddRow("task_exec1", "task1", "exec1", "wf1", core.ComponentAgent, core.StatusPending,
					task.ExecutionBasic, agentID, "default_action", nil, nil, nil, nil, nil, nil, nil).
				AddRow("task_exec2", "task2", "exec1", "wf1", core.ComponentTool, core.StatusRunning,
					task.ExecutionBasic, nil, nil, toolID, nil, nil, nil, nil, nil, nil)

			mockSetup.Mock.ExpectQuery("SELECT \\*").
				WithArgs(workflowExecID).
				WillReturnRows(taskRows)

			states, err := repo.ListTasksInWorkflow(ctx, workflowExecID)
			assert.NoError(t, err)
			assert.Len(t, states, 2)
			assert.Contains(t, states, "task1")
			assert.Contains(t, states, "task2")
			assert.Equal(t, "agent1", *states["task1"].AgentID)
			assert.Equal(t, "tool1", *states["task2"].ToolID)
			assert.Equal(t, task.ExecutionBasic, states["task1"].ExecutionType)
			assert.Equal(t, task.ExecutionBasic, states["task2"].ExecutionType)
		},
	)
}

func TestTaskRepo_ListTasksByStatus(t *testing.T) {
	testTaskList(
		t,
		"should list tasks by status",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			workflowExecID := core.ID("exec1")
			status := core.StatusPending

			taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
			taskRows := taskRowBuilder.CreateTaskStateRowsWithExecution(
				"task_exec1", "task1", "exec1", "wf1",
				core.StatusPending, task.ExecutionBasic, nil, nil, nil,
			)

			mockSetup.Mock.ExpectQuery("SELECT \\*").
				WithArgs(workflowExecID, status).
				WillReturnRows(taskRows)

			states, err := repo.ListTasksByStatus(ctx, workflowExecID, status)
			assert.NoError(t, err)
			assert.Len(t, states, 1)
			assert.Equal(t, core.StatusPending, states[0].Status)
			assert.Equal(t, task.ExecutionBasic, states[0].ExecutionType)
		},
	)
}

func TestTaskRepo_ListTasksByAgent(t *testing.T) {
	testTaskList(
		t,
		"should list tasks by agent",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			workflowExecID := core.ID("exec1")
			agentID := "agent1"

			taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
			taskRows := taskRowBuilder.CreateTaskStateRowsWithExecution(
				"task_exec1", "task1", "exec1", "wf1",
				core.StatusPending, task.ExecutionBasic, agentID, nil, nil,
			)

			mockSetup.Mock.ExpectQuery("SELECT \\*").
				WithArgs(workflowExecID, agentID).
				WillReturnRows(taskRows)

			states, err := repo.ListTasksByAgent(ctx, workflowExecID, agentID)
			assert.NoError(t, err)
			assert.Len(t, states, 1)
			assert.Equal(t, "agent1", *states[0].AgentID)
			assert.Equal(t, task.ExecutionBasic, states[0].ExecutionType)
		},
	)
}

func TestTaskRepo_ListTasksByTool(t *testing.T) {
	testTaskList(
		t,
		"should list tasks by tool",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			workflowExecID := core.ID("exec1")
			toolID := "tool1"

			taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
			taskRows := taskRowBuilder.CreateTaskStateRowsWithExecution(
				"task_exec1", "task1", "exec1", "wf1",
				core.StatusPending, task.ExecutionBasic, nil, toolID, nil,
			)

			mockSetup.Mock.ExpectQuery("SELECT \\*").
				WithArgs(workflowExecID, toolID).
				WillReturnRows(taskRows)

			states, err := repo.ListTasksByTool(ctx, workflowExecID, toolID)
			assert.NoError(t, err)
			assert.Len(t, states, 1)
			assert.Equal(t, "tool1", *states[0].ToolID)
			assert.Equal(t, task.ExecutionBasic, states[0].ExecutionType)
		},
	)
}

func TestTaskRepo_ListStates(t *testing.T) {
	testTaskList(
		t,
		"should list states with filter",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			filter := &task.StateFilter{
				Status:         &[]core.StatusType{core.StatusPending}[0],
				WorkflowExecID: &[]core.ID{core.ID("exec1")}[0],
			}

			taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
			taskRows := taskRowBuilder.CreateTaskStateRowsWithExecution(
				"task_exec1", "task1", "exec1", "wf1",
				core.StatusPending, task.ExecutionBasic, nil, nil, nil,
			)

			mockSetup.Mock.ExpectQuery("SELECT \\*").
				WithArgs(core.StatusPending, core.ID("exec1")).
				WillReturnRows(taskRows)

			states, err := repo.ListStates(ctx, filter)
			assert.NoError(t, err)
			assert.Len(t, states, 1)
			assert.Equal(t, core.StatusPending, states[0].Status)
			assert.Equal(t, task.ExecutionBasic, states[0].ExecutionType)
		},
	)
}

func TestTaskRepo_ListStatesWithExecutionTypeFilter(t *testing.T) {
	testTaskList(
		t,
		"should filter states by execution type",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			// Create parallel state data
			parallelState := &task.ParallelState{
				Strategy:       task.StrategyWaitAll,
				MaxWorkers:     2,
				SubTasks:       make(map[string]*task.State),
				CompletedTasks: make([]string, 0),
				FailedTasks:    make([]string, 0),
			}

			dataBuilder := testutils.NewDataBuilder()
			parallelStateData := dataBuilder.MustCreateParallelStateData(parallelState)

			taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
			taskRows := taskRowBuilder.CreateParallelTaskStateRows(
				"task_exec1", "parallel_task1", "exec1", "wf1",
				core.StatusRunning, task.ExecutionParallel, parallelStateData,
			)

			mockSetup.Mock.ExpectQuery("SELECT \\*").
				WithArgs(task.ExecutionParallel).
				WillReturnRows(taskRows)

			executionType := task.ExecutionParallel
			filter := &task.StateFilter{ExecutionType: &executionType}
			states, err := repo.ListStates(ctx, filter)
			assert.NoError(t, err)
			assert.Len(t, states, 1)
			assert.Equal(t, task.ExecutionParallel, states[0].ExecutionType)
		},
	)
}

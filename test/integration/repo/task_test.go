package store

import (
	"context"
	"fmt"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/logger"
	testutils "github.com/compozy/compozy/test"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
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

	queries := mockSetup.NewQueryExpectations()
	queries.ExpectTaskStateQueryForUpsert([]any{
		state.TaskExecID, state.TaskID, state.WorkflowExecID, state.WorkflowID, state.Component, state.Status,
		state.ExecutionType, (*string)(nil), state.AgentID, state.ActionID, state.ToolID, inputJSON,
		expectedOutputJSON,
		expectedErrorJSON,
	})

	err := repo.UpsertState(ctx, state)
	assert.NoError(t, err)
	mockSetup.ExpectationsWereMet()
}

func TestTaskRepo_CreateChildStatesInTransaction(t *testing.T) {
	// Initialize logger to prevent nil pointer dereference
	logger.Init(logger.DefaultConfig())

	testTaskList(
		t,
		"should create multiple child states atomically",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			parentStateID := core.ID("parent-exec-123")
			workflowExecID := core.ID("exec1")

			// Create child states to insert
			childStates := []*task.State{
				task.CreateAgentSubTaskState(
					"child1", core.ID("child1-exec"), "wf1", workflowExecID,
					&parentStateID, "agent1", "action1", &core.Input{"task": "subtask1"}),
				task.CreateToolSubTaskState(
					"child2", core.ID("child2-exec"), "wf1", workflowExecID,
					&parentStateID, "tool1", &core.Input{"task": "subtask2"}),
			}

			dataBuilder := testutils.NewDataBuilder()
			input1JSON := dataBuilder.MustCreateInputData(map[string]any{"task": "subtask1"})
			input2JSON := dataBuilder.MustCreateInputData(map[string]any{"task": "subtask2"})
			nilJSON := dataBuilder.MustCreateNilJSONB()

			// Expect transaction begin
			mockSetup.Mock.ExpectBegin()

			// Convert parentStateID to *string as the implementation does
			parentStateIDStr := string(parentStateID)

			// Expect first child insert
			mockSetup.Mock.ExpectExec("INSERT INTO task_states").
				WithArgs(
					childStates[0].TaskExecID, childStates[0].TaskID, childStates[0].WorkflowExecID,
					childStates[0].WorkflowID, childStates[0].Component, childStates[0].Status,
					childStates[0].ExecutionType, &parentStateIDStr, childStates[0].AgentID,
					childStates[0].ActionID, (*string)(nil), input1JSON, nilJSON, nilJSON,
				).WillReturnResult(pgxmock.NewResult("INSERT", 1))

			// Expect second child insert
			mockSetup.Mock.ExpectExec("INSERT INTO task_states").
				WithArgs(
					childStates[1].TaskExecID, childStates[1].TaskID, childStates[1].WorkflowExecID,
					childStates[1].WorkflowID, childStates[1].Component, childStates[1].Status,
					childStates[1].ExecutionType, &parentStateIDStr, (*string)(nil), (*string)(nil),
					childStates[1].ToolID, input2JSON, nilJSON, nilJSON,
				).WillReturnResult(pgxmock.NewResult("INSERT", 1))

			// Expect commit
			mockSetup.Mock.ExpectCommit()

			err := repo.CreateChildStatesInTransaction(ctx, parentStateID, childStates)
			assert.NoError(t, err)
		},
	)

	testTaskList(
		t,
		"should rollback transaction on error",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			parentStateID := core.ID("parent-exec-456")
			workflowExecID := core.ID("exec2")

			childStates := []*task.State{
				task.CreateAgentSubTaskState(
					"child1", core.ID("child1-exec"), "wf2", workflowExecID,
					&parentStateID, "agent1", "action1", &core.Input{"task": "subtask1"}),
			}

			dataBuilder := testutils.NewDataBuilder()
			inputJSON := dataBuilder.MustCreateInputData(map[string]any{"task": "subtask1"})
			nilJSON := dataBuilder.MustCreateNilJSONB()

			// Expect transaction begin
			mockSetup.Mock.ExpectBegin()

			// Convert parentStateID to *string as the implementation does
			parentStateIDStr := string(parentStateID)

			// Expect insert to fail
			mockSetup.Mock.ExpectExec("INSERT INTO task_states").
				WithArgs(
					childStates[0].TaskExecID, childStates[0].TaskID, childStates[0].WorkflowExecID,
					childStates[0].WorkflowID, childStates[0].Component, childStates[0].Status,
					childStates[0].ExecutionType, &parentStateIDStr, childStates[0].AgentID,
					childStates[0].ActionID, (*string)(nil), inputJSON, nilJSON, nilJSON,
				).WillReturnError(fmt.Errorf("database error"))

			// Expect rollback
			mockSetup.Mock.ExpectRollback()

			err := repo.CreateChildStatesInTransaction(ctx, parentStateID, childStates)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "database error")
		},
	)
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

func TestTaskRepo_GetParallelStateEquivalent(t *testing.T) {
	testTaskGet(
		t,
		"should get parent task with parallel execution type",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			parentExecID := core.ID("parent-exec-123")

			dataBuilder := testutils.NewDataBuilder()
			inputData := dataBuilder.MustCreateInputData(map[string]any{"strategy": "wait_all"})

			taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
			parentRows := taskRowBuilder.CreateTaskStateRowsWithExecution(
				"parent-exec-123", "parent-task", "exec1", "wf1",
				core.StatusRunning, task.ExecutionParallel, "", nil, inputData,
			)

			mockSetup.Mock.ExpectQuery("SELECT \\*").
				WithArgs(parentExecID).
				WillReturnRows(parentRows)

			state, err := repo.GetState(ctx, parentExecID)
			assert.NoError(t, err)
			assert.Equal(t, parentExecID, state.TaskExecID)
			assert.Equal(t, "parent-task", state.TaskID)
			assert.Equal(t, core.StatusRunning, state.Status)
			assert.Equal(t, task.ExecutionParallel, state.ExecutionType)
			assert.Nil(t, state.ParentStateID) // Parent task has no parent
			assert.True(t, state.IsParallelRoot())
		},
	)

	testTaskGet(
		t,
		"should get child task with parent reference",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			childExecID := core.ID("child-exec-456")
			parentExecID := core.ID("parent-exec-123")

			dataBuilder := testutils.NewDataBuilder()
			inputData := dataBuilder.MustCreateInputData(map[string]any{"task": "subtask"})

			taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
			childRows := taskRowBuilder.CreateEmptyTaskStateRows().
				AddRow("child-exec-456", "child-task", "exec1", "wf1", core.ComponentAgent, core.StatusPending,
					task.ExecutionBasic, parentExecID, "agent1", "action1", nil, inputData, nil, nil, nil, nil)

			mockSetup.Mock.ExpectQuery("SELECT \\*").
				WithArgs(childExecID).
				WillReturnRows(childRows)

			state, err := repo.GetState(ctx, childExecID)
			assert.NoError(t, err)
			assert.Equal(t, childExecID, state.TaskExecID)
			assert.Equal(t, "child-task", state.TaskID)
			assert.Equal(t, core.StatusPending, state.Status)
			assert.Equal(t, task.ExecutionBasic, state.ExecutionType)
			assert.NotNil(t, state.ParentStateID)
			assert.Equal(t, parentExecID, *state.ParentStateID)
			assert.True(t, state.IsChildTask())
			assert.False(t, state.IsParallelRoot())
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
					task.ExecutionBasic, nil, agentID, "default_action", nil, nil, nil, nil, nil, nil).
				AddRow("task_exec2", "task2", "exec1", "wf1", core.ComponentTool, core.StatusRunning,
					task.ExecutionBasic, nil, nil, nil, toolID, nil, nil, nil, nil, nil)

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

func TestTaskRepo_ListChildren(t *testing.T) {
	testTaskList(
		t,
		"should list child tasks for a given parent",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			parentStateID := core.ID("parent-exec-123")

			// Create child task rows with parent_state_id set manually
			childRows := mockSetup.Mock.NewRows([]string{
				"task_exec_id", "task_id", "workflow_exec_id", "workflow_id",
				"component", "status", "execution_type", "parent_state_id",
				"agent_id", "action_id", "tool_id", "input", "output", "error",
				"created_at", "updated_at",
			}).AddRow(
				"child_exec1", "child1", "exec1", "wf1",
				core.ComponentTask, core.StatusPending, task.ExecutionBasic, parentStateID,
				nil, nil, nil, nil, nil, nil, nil, nil,
			)

			mockSetup.Mock.ExpectQuery("SELECT \\*").
				WithArgs(parentStateID).
				WillReturnRows(childRows)

			children, err := repo.ListChildren(ctx, parentStateID)
			assert.NoError(t, err)
			assert.Len(t, children, 1)
			assert.Equal(t, "child1", children[0].TaskID)
			assert.Equal(t, core.StatusPending, children[0].Status)
			assert.Equal(t, parentStateID, *children[0].ParentStateID)
		},
	)

	testTaskList(
		t,
		"should return empty list when parent has no children",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			parentStateID := core.ID("parent-with-no-children")

			// Mock empty result set
			emptyRows := mockSetup.Mock.NewRows([]string{
				"task_exec_id", "task_id", "workflow_exec_id", "workflow_id",
				"component", "status", "execution_type", "parent_state_id",
				"agent_id", "action_id", "tool_id", "input", "output", "error",
				"created_at", "updated_at",
			})

			mockSetup.Mock.ExpectQuery("SELECT \\*").
				WithArgs(parentStateID).
				WillReturnRows(emptyRows)

			children, err := repo.ListChildren(ctx, parentStateID)
			assert.NoError(t, err)
			assert.Len(t, children, 0)
		},
	)

	testTaskList(
		t,
		"should be equivalent to ListStates with ParentStateID filter",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			parentStateID := core.ID("parent-exec-456")

			// Create child task rows
			childRows := mockSetup.Mock.NewRows([]string{
				"task_exec_id", "task_id", "workflow_exec_id", "workflow_id",
				"component", "status", "execution_type", "parent_state_id",
				"agent_id", "action_id", "tool_id", "input", "output", "error",
				"created_at", "updated_at",
			}).AddRow(
				"child_exec2", "child2", "exec2", "wf2",
				core.ComponentTask, core.StatusRunning, task.ExecutionBasic, parentStateID,
				nil, nil, nil, nil, nil, nil, nil, nil,
			)

			// Test ListChildren
			mockSetup.Mock.ExpectQuery("SELECT \\*").
				WithArgs(parentStateID).
				WillReturnRows(childRows)

			children, err := repo.ListChildren(ctx, parentStateID)
			assert.NoError(t, err)
			assert.Len(t, children, 1)
			assert.Equal(t, "child2", children[0].TaskID)

			// Test equivalent ListStates call with ParentStateID filter
			childRowsCopy := mockSetup.Mock.NewRows([]string{
				"task_exec_id", "task_id", "workflow_exec_id", "workflow_id",
				"component", "status", "execution_type", "parent_state_id",
				"agent_id", "action_id", "tool_id", "input", "output", "error",
				"created_at", "updated_at",
			}).AddRow(
				"child_exec2", "child2", "exec2", "wf2",
				core.ComponentTask, core.StatusRunning, task.ExecutionBasic, parentStateID,
				nil, nil, nil, nil, nil, nil, nil, nil,
			)

			mockSetup.Mock.ExpectQuery("SELECT \\*").
				WithArgs(parentStateID).
				WillReturnRows(childRowsCopy)

			filter := &task.StateFilter{ParentStateID: &parentStateID}
			filteredStates, err := repo.ListStates(ctx, filter)
			assert.NoError(t, err)
			assert.Len(t, filteredStates, 1)
			assert.Equal(t, children[0].TaskID, filteredStates[0].TaskID)
			assert.Equal(t, children[0].Status, filteredStates[0].Status)
		},
	)
}

func TestTaskRepo_ListStatesWithExecutionTypeFilter(t *testing.T) {
	testTaskList(
		t,
		"should filter states by execution type - parallel only",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			execType := task.ExecutionParallel
			filter := &task.StateFilter{
				ExecutionType: &execType,
			}

			taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
			taskRows := taskRowBuilder.CreateTaskStateRowsWithExecution(
				"parent-exec-1", "parent-task", "exec1", "wf1",
				core.StatusRunning, task.ExecutionParallel, "", nil, nil,
			)

			mockSetup.Mock.ExpectQuery("SELECT \\*").
				WithArgs(task.ExecutionParallel).
				WillReturnRows(taskRows)

			states, err := repo.ListStates(ctx, filter)
			assert.NoError(t, err)
			assert.Len(t, states, 1)
			assert.Equal(t, task.ExecutionParallel, states[0].ExecutionType)
			assert.True(t, states[0].IsParallelExecution())
		},
	)

	testTaskList(
		t,
		"should filter states by execution type - basic only",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			execType := task.ExecutionBasic
			filter := &task.StateFilter{
				ExecutionType: &execType,
			}

			taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
			taskRows := taskRowBuilder.CreateTaskStateRowsWithExecution(
				"child-exec-1", "child-task", "exec1", "wf1",
				core.StatusPending, task.ExecutionBasic, "agent1", nil, nil,
			)

			mockSetup.Mock.ExpectQuery("SELECT \\*").
				WithArgs(task.ExecutionBasic).
				WillReturnRows(taskRows)

			states, err := repo.ListStates(ctx, filter)
			assert.NoError(t, err)
			assert.Len(t, states, 1)
			assert.Equal(t, task.ExecutionBasic, states[0].ExecutionType)
			assert.True(t, states[0].IsBasic())
		},
	)

	testTaskList(
		t,
		"should filter by parent state ID and execution type combined",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			parentStateID := core.ID("parent-exec-123")
			execType := task.ExecutionBasic
			filter := &task.StateFilter{
				ParentStateID: &parentStateID,
				ExecutionType: &execType,
			}

			taskRowBuilder := mockSetup.NewTaskStateRowBuilder()
			taskRows := taskRowBuilder.CreateEmptyTaskStateRows().
				AddRow("child-exec-1", "child1", "exec1", "wf1", core.ComponentAgent, core.StatusPending,
					task.ExecutionBasic, parentStateID, "agent1", "action1", nil, nil, nil, nil, nil, nil)

			mockSetup.Mock.ExpectQuery("SELECT \\*").
				WithArgs(parentStateID, task.ExecutionBasic).
				WillReturnRows(taskRows)

			states, err := repo.ListStates(ctx, filter)
			assert.NoError(t, err)
			assert.Len(t, states, 1)
			assert.Equal(t, task.ExecutionBasic, states[0].ExecutionType)
			assert.Equal(t, parentStateID, *states[0].ParentStateID)
			assert.True(t, states[0].IsChildTask())
		},
	)
}

func TestTaskRepo_GetTaskTree(t *testing.T) {
	testTaskList(
		t,
		"should retrieve complete task hierarchy using CTE",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			rootStateID := core.ID("root-exec-123")

			// Create hierarchical task tree with root and children
			treeRows := mockSetup.Mock.NewRows([]string{
				"task_exec_id", "task_id", "workflow_exec_id", "workflow_id",
				"component", "status", "execution_type", "parent_state_id",
				"agent_id", "action_id", "tool_id", "input", "output", "error",
				"created_at", "updated_at",
			}).AddRow(
				// Root task (depth 0)
				"root-exec-123", "root-task", "exec1", "wf1",
				core.ComponentTask, core.StatusRunning, task.ExecutionParallel, nil,
				nil, nil, nil, nil, nil, nil, nil, nil,
			).AddRow(
				// Child task 1 (depth 1)
				"child1-exec-456", "child1", "exec1", "wf1",
				core.ComponentAgent, core.StatusPending, task.ExecutionBasic, rootStateID,
				nil, nil, nil, nil, nil, nil, nil, nil,
			).AddRow(
				// Child task 2 (depth 1)
				"child2-exec-789", "child2", "exec1", "wf1",
				core.ComponentAgent, core.StatusSuccess, task.ExecutionBasic, rootStateID,
				nil, nil, nil, nil, nil, nil, nil, nil,
			).AddRow(
				// Grandchild task (depth 2)
				"grandchild-exec-999", "grandchild", "exec1", "wf1",
				core.ComponentTool, core.StatusRunning, task.ExecutionBasic, core.ID("child1-exec-456"),
				nil, nil, nil, nil, nil, nil, nil, nil,
			)

			mockSetup.Mock.ExpectQuery("WITH RECURSIVE task_tree").
				WithArgs(rootStateID).
				WillReturnRows(treeRows)

			tree, err := repo.GetTaskTree(ctx, rootStateID)
			assert.NoError(t, err)
			assert.Len(t, tree, 4)

			// Verify root task
			assert.Equal(t, "root-task", tree[0].TaskID)
			assert.Equal(t, rootStateID, tree[0].TaskExecID)
			assert.Nil(t, tree[0].ParentStateID)
			assert.Equal(t, task.ExecutionParallel, tree[0].ExecutionType)

			// Verify children are ordered by depth, then created_at
			assert.Equal(t, "child1", tree[1].TaskID)
			assert.Equal(t, rootStateID, *tree[1].ParentStateID)
			assert.Equal(t, "child2", tree[2].TaskID)
			assert.Equal(t, rootStateID, *tree[2].ParentStateID)

			// Verify grandchild
			assert.Equal(t, "grandchild", tree[3].TaskID)
			assert.Equal(t, core.ID("child1-exec-456"), *tree[3].ParentStateID)
		},
	)

	testTaskList(
		t,
		"should return only root task when no children exist",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			rootStateID := core.ID("lonely-root-123")

			// Create single row for root task only
			singleRow := mockSetup.Mock.NewRows([]string{
				"task_exec_id", "task_id", "workflow_exec_id", "workflow_id",
				"component", "status", "execution_type", "parent_state_id",
				"agent_id", "action_id", "tool_id", "input", "output", "error",
				"created_at", "updated_at",
			}).AddRow(
				"lonely-root-123", "lonely-task", "exec1", "wf1",
				core.ComponentTask, core.StatusSuccess, task.ExecutionBasic, nil,
				nil, nil, nil, nil, nil, nil, nil, nil,
			)

			mockSetup.Mock.ExpectQuery("WITH RECURSIVE task_tree").
				WithArgs(rootStateID).
				WillReturnRows(singleRow)

			tree, err := repo.GetTaskTree(ctx, rootStateID)
			assert.NoError(t, err)
			assert.Len(t, tree, 1)
			assert.Equal(t, "lonely-task", tree[0].TaskID)
			assert.Nil(t, tree[0].ParentStateID)
		},
	)

	testTaskList(
		t,
		"should return empty slice when root task does not exist",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			nonExistentRootID := core.ID("non-existent-root")

			// Mock empty result set
			emptyRows := mockSetup.Mock.NewRows([]string{
				"task_exec_id", "task_id", "workflow_exec_id", "workflow_id",
				"component", "status", "execution_type", "parent_state_id",
				"agent_id", "action_id", "tool_id", "input", "output", "error",
				"created_at", "updated_at",
			})

			mockSetup.Mock.ExpectQuery("WITH RECURSIVE task_tree").
				WithArgs(nonExistentRootID).
				WillReturnRows(emptyRows)

			tree, err := repo.GetTaskTree(ctx, nonExistentRootID)
			assert.NoError(t, err)
			assert.Len(t, tree, 0)
		},
	)
}

func TestTaskRepo_GetProgressInfo(t *testing.T) {
	testTaskList(
		t,
		"should aggregate progress information for parent task",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			parentStateID := core.ID("parent-exec-123")

			// Mock first query for aggregate counts
			aggregateRows := mockSetup.Mock.NewRows([]string{
				"total_children", "completed", "failed", "running", "pending",
			}).AddRow(3, 1, 1, 1, 0)

			mockSetup.Mock.ExpectQuery("SELECT").
				WithArgs(parentStateID).
				WillReturnRows(aggregateRows)

			// Mock second query for detailed status counts
			statusRows := mockSetup.Mock.NewRows([]string{
				"status", "status_count",
			}).AddRow(string(core.StatusSuccess), 1).
				AddRow(string(core.StatusFailed), 1).
				AddRow(string(core.StatusRunning), 1)

			mockSetup.Mock.ExpectQuery("SELECT status").
				WithArgs(parentStateID).
				WillReturnRows(statusRows)

			progressInfo, err := repo.GetProgressInfo(ctx, parentStateID)
			assert.NoError(t, err)
			assert.NotNil(t, progressInfo)

			// Verify aggregated counts
			assert.Equal(t, 3, progressInfo.TotalChildren)
			assert.Equal(t, 1, progressInfo.CompletedCount)
			assert.Equal(t, 1, progressInfo.FailedCount)
			assert.Equal(t, 1, progressInfo.RunningCount)
			assert.Equal(t, 0, progressInfo.PendingCount)

			// Verify calculated rates
			assert.InDelta(t, 0.333, progressInfo.CompletionRate, 0.01)
			assert.InDelta(t, 0.333, progressInfo.FailureRate, 0.01)

			// Verify status counts map
			assert.Equal(t, 1, progressInfo.StatusCounts[core.StatusSuccess])
			assert.Equal(t, 1, progressInfo.StatusCounts[core.StatusFailed])
			assert.Equal(t, 1, progressInfo.StatusCounts[core.StatusRunning])
		},
	)

	testTaskList(
		t,
		"should return empty progress info when parent has no children",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			parentStateID := core.ID("parent-with-no-children")

			// Mock aggregate query with zero results
			emptyAggregateRows := mockSetup.Mock.NewRows([]string{
				"total_children", "completed", "failed", "running", "pending",
			}).AddRow(0, 0, 0, 0, 0)

			mockSetup.Mock.ExpectQuery("SELECT").
				WithArgs(parentStateID).
				WillReturnRows(emptyAggregateRows)

			// Mock empty status query
			emptyStatusRows := mockSetup.Mock.NewRows([]string{
				"status", "status_count",
			})

			mockSetup.Mock.ExpectQuery("SELECT status").
				WithArgs(parentStateID).
				WillReturnRows(emptyStatusRows)

			progressInfo, err := repo.GetProgressInfo(ctx, parentStateID)
			assert.NoError(t, err)
			assert.NotNil(t, progressInfo)
			assert.Equal(t, 0, progressInfo.TotalChildren)
			assert.Equal(t, 0.0, progressInfo.CompletionRate)
			assert.Equal(t, 0.0, progressInfo.FailureRate)
			assert.NotNil(t, progressInfo.StatusCounts)
			assert.Len(t, progressInfo.StatusCounts, 0)
		},
	)

	testTaskList(
		t,
		"should calculate progress for wait_all strategy correctly",
		func(mockSetup *testutils.MockSetup, repo *store.TaskRepo, ctx context.Context) {
			parentStateID := core.ID("wait-all-parent")

			// Mock aggregate query for all completed tasks
			allCompletedAggregateRows := mockSetup.Mock.NewRows([]string{
				"total_children", "completed", "failed", "running", "pending",
			}).AddRow(2, 2, 0, 0, 0)

			mockSetup.Mock.ExpectQuery("SELECT").
				WithArgs(parentStateID).
				WillReturnRows(allCompletedAggregateRows)

			// Mock status query for all completed tasks
			allCompletedStatusRows := mockSetup.Mock.NewRows([]string{
				"status", "status_count",
			}).AddRow(string(core.StatusSuccess), 2)

			mockSetup.Mock.ExpectQuery("SELECT status").
				WithArgs(parentStateID).
				WillReturnRows(allCompletedStatusRows)

			progressInfo, err := repo.GetProgressInfo(ctx, parentStateID)
			assert.NoError(t, err)

			// Test different strategies
			waitAllStatus := progressInfo.CalculateOverallStatus(task.StrategyWaitAll)
			assert.Equal(t, core.StatusSuccess, waitAllStatus)
			assert.True(t, progressInfo.IsComplete(task.StrategyWaitAll))
			assert.False(t, progressInfo.HasFailures())
			assert.True(t, progressInfo.IsAllComplete())
		},
	)
}

package helpers

import (
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	pgdriver "github.com/compozy/compozy/engine/infra/postgres"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
)

// -----
// Mock Database Utilities
// -----

// MockSetup holds common mock database setup
type MockSetup struct {
	Mock pgxmock.PgxPoolIface
	T    *testing.T
}

// NewMockSetup creates a new mock database setup
func NewMockSetup(t *testing.T) *MockSetup {
	mock, err := pgxmock.NewPool()
	assert.NoError(t, err)
	return &MockSetup{
		Mock: mock,
		T:    t,
	}
}

// Close closes the mock database
func (m *MockSetup) Close() {
	m.Mock.Close()
}

// ExpectationsWereMet checks if all expectations were met
func (m *MockSetup) ExpectationsWereMet() {
	assert.NoError(m.T, m.Mock.ExpectationsWereMet())
}

// -----
// Transaction Expectations
// -----

// TransactionExpectations handles common transaction expectations
type TransactionExpectations struct {
	mock pgxmock.PgxPoolIface
}

// NewTransactionExpectations creates transaction expectation helper
func (m *MockSetup) NewTransactionExpectations() *TransactionExpectations {
	return &TransactionExpectations{mock: m.Mock}
}

// ExpectBegin expects a transaction to begin
func (te *TransactionExpectations) ExpectBegin() *TransactionExpectations {
	te.mock.ExpectBegin()
	return te
}

// ExpectCommit expects a transaction to commit
func (te *TransactionExpectations) ExpectCommit() *TransactionExpectations {
	te.mock.ExpectCommit()
	return te
}

// ExpectRollback expects a transaction to rollback
func (te *TransactionExpectations) ExpectRollback() *TransactionExpectations {
	te.mock.ExpectRollback()
	return te
}

// -----
// Row Builders
// -----

// WorkflowStateRowBuilder helps build workflow state rows
type WorkflowStateRowBuilder struct {
	mock pgxmock.PgxPoolIface
}

// NewWorkflowStateRowBuilder creates a workflow state row builder
func (m *MockSetup) NewWorkflowStateRowBuilder() *WorkflowStateRowBuilder {
	return &WorkflowStateRowBuilder{mock: m.Mock}
}

// CreateWorkflowStateRows creates mock rows for workflow states
func (w *WorkflowStateRowBuilder) CreateWorkflowStateRows(
	workflowExecID, workflowID string,
	status core.StatusType,
	inputData []byte,
) *pgxmock.Rows {
	return w.mock.NewRows([]string{
		"workflow_exec_id", "workflow_id", "status", "input", "output", "error",
	}).AddRow(
		workflowExecID, workflowID, status, inputData, nil, nil,
	)
}

// TaskStateRowBuilder helps build task state rows
type TaskStateRowBuilder struct {
	mock pgxmock.PgxPoolIface
}

// NewTaskStateRowBuilder creates a task state row builder
func (m *MockSetup) NewTaskStateRowBuilder() *TaskStateRowBuilder {
	return &TaskStateRowBuilder{mock: m.Mock}
}

// CreateEmptyTaskStateRows creates empty mock rows for task states
func (t *TaskStateRowBuilder) CreateEmptyTaskStateRows() *pgxmock.Rows {
	return t.mock.NewRows([]string{
		"task_exec_id", "task_id", "workflow_exec_id", "workflow_id",
		"component", "status", "execution_type", "parent_state_id", "agent_id", "action_id", "tool_id",
		"input", "output", "error", "created_at", "updated_at",
	})
}

// CreateTaskStateRows creates mock rows for task states with data
func (t *TaskStateRowBuilder) CreateTaskStateRows(
	taskExecID, taskID, workflowExecID, workflowID string,
	status core.StatusType,
	agentID, toolID any,
	inputData []byte,
) *pgxmock.Rows {
	var component core.ComponentType
	var actionID any
	var executionType = "basic"
	switch {
	case agentID != nil:
		component = core.ComponentAgent
		actionID = DefaultActionID // Required for agent components
	case toolID != nil:
		component = core.ComponentTool
		actionID = nil // Not required for tool components
	default:
		component = core.ComponentTask
		actionID = DefaultActionID // Task components may have actions
	}
	now := time.Now()
	return t.mock.NewRows([]string{
		"task_exec_id", "task_id", "workflow_exec_id", "workflow_id",
		"component", "status", "execution_type", "parent_state_id", "agent_id", "action_id", "tool_id",
		"input", "output", "error", "created_at", "updated_at",
	}).AddRow(
		taskExecID, taskID, workflowExecID, workflowID,
		component, status, executionType, nil, agentID, actionID, toolID, inputData,
		nil, nil, now, now,
	)
}

// CreateTaskStateRowsWithExecution creates mock rows for task states with explicit execution type
func (t *TaskStateRowBuilder) CreateTaskStateRowsWithExecution(
	taskExecID, taskID, workflowExecID, workflowID string,
	status core.StatusType,
	executionType any,
	agentID, toolID any,
	inputData []byte,
) *pgxmock.Rows {
	var component core.ComponentType
	var actionID any
	switch {
	case agentID != nil:
		component = core.ComponentAgent
		actionID = DefaultActionID // Required for agent components
	case toolID != nil:
		component = core.ComponentTool
		actionID = nil // Not required for tool components
	default:
		component = core.ComponentTask
		actionID = DefaultActionID // Task components may have actions
	}
	columns := []string{
		"task_exec_id", "task_id", "workflow_exec_id", "workflow_id",
		"component", "status", "execution_type", "parent_state_id",
		"agent_id", "action_id", "tool_id", "input", "output",
		"error", "created_at", "updated_at",
	}
	now := time.Now()
	values := []any{
		taskExecID, taskID, workflowExecID, workflowID,
		component, status, executionType, nil,
		agentID, actionID, toolID, inputData, nil,
		nil, now, now,
	}
	return t.mock.NewRows(columns).AddRow(values...)
}

// -----
// Query Expectations
// -----

// QueryExpectations handles common query expectations
type QueryExpectations struct {
	mock pgxmock.PgxPoolIface
}

// NewQueryExpectations creates query expectation helper
func (m *MockSetup) NewQueryExpectations() *QueryExpectations {
	return &QueryExpectations{mock: m.Mock}
}

// ExpectWorkflowStateQuery expects a workflow state query
func (q *QueryExpectations) ExpectWorkflowStateQuery(
	queryPattern string,
	args []any,
	rows *pgxmock.Rows,
) *QueryExpectations {
	q.mock.ExpectQuery(queryPattern).WithArgs(args...).WillReturnRows(rows)
	return q
}

// ExpectTaskStateQuery expects a task state query
func (q *QueryExpectations) ExpectTaskStateQuery(
	workflowExecID core.ID,
	rows *pgxmock.Rows,
) *QueryExpectations {
	q.mock.ExpectQuery("SELECT \\*").
		WithArgs(workflowExecID).
		WillReturnRows(rows)
	return q
}

// ExpectTaskStateQueryForUpsert expects a task state upsert query
func (q *QueryExpectations) ExpectTaskStateQueryForUpsert(
	args []any,
) *QueryExpectations {
	q.mock.ExpectExec("INSERT INTO task_states").
		WithArgs(args...).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	return q
}

// ExpectWorkflowStateQueryForUpsert expects a workflow state upsert query
func (q *QueryExpectations) ExpectWorkflowStateQueryForUpsert(
	args []any,
) *QueryExpectations {
	q.mock.ExpectExec("INSERT INTO workflow_states").
		WithArgs(args...).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	return q
}

// -----
// Data Builders
// -----

// DataBuilder helps create test data
type DataBuilder struct{}

// NewDataBuilder creates a test data builder
func NewDataBuilder() *DataBuilder {
	return &DataBuilder{}
}

// MustCreateInputData creates test input data as JSONB
func (td *DataBuilder) MustCreateInputData(data map[string]any) []byte {
	input := core.Input(data)
	inputData, err := pgdriver.ToJSONB(&input)
	if err != nil {
		panic(err)
	}
	return inputData
}

// MustCreateNilJSONB creates nil JSONB data
func (td *DataBuilder) MustCreateNilJSONB() []byte {
	data, err := pgdriver.ToJSONB(nil)
	if err != nil {
		panic(err)
	}
	return data
}

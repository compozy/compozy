package test

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
)

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
		"status", "agent_id", "tool_id", "input", "output", "error",
	})
}

// CreateTaskStateRows creates mock rows for task states with data
func (t *TaskStateRowBuilder) CreateTaskStateRows(
	taskExecID, taskID, workflowExecID, workflowID string,
	status core.StatusType,
	agentID, toolID interface{},
	inputData []byte,
) *pgxmock.Rows {
	return t.mock.NewRows([]string{
		"task_exec_id", "task_id", "workflow_exec_id", "workflow_id",
		"status", "agent_id", "tool_id", "input", "output", "error",
	}).AddRow(
		taskExecID, taskID, workflowExecID, workflowID,
		status, agentID, toolID, inputData, nil, nil,
	)
}

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
	q.mock.ExpectQuery("SELECT task_exec_id, task_id, workflow_exec_id, workflow_id").
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

// DataBuilder helps create test data
type DataBuilder struct{}

// NewDataBuilder creates a test data builder
func NewDataBuilder() *DataBuilder {
	return &DataBuilder{}
}

// MustCreateInputData creates test input data as JSONB
func (td *DataBuilder) MustCreateInputData(data map[string]any) []byte {
	input := core.Input(data)
	inputData, err := store.ToJSONB(&input)
	if err != nil {
		panic(err)
	}
	return inputData
}

// MustCreateNilJSONB creates nil JSONB data
func (td *DataBuilder) MustCreateNilJSONB() []byte {
	data, err := store.ToJSONB(nil)
	if err != nil {
		panic(err)
	}
	return data
}

// SimpleWorkflowTest runs a simple workflow state test with task population
func (m *MockSetup) SimpleWorkflowTest(
	testName string,
	setupQuery func(*QueryExpectations, *WorkflowStateRowBuilder, *TaskStateRowBuilder),
	runTest func() error,
) {
	m.T.Run(testName, func(t *testing.T) {
		tx := m.NewTransactionExpectations()
		queries := m.NewQueryExpectations()
		workflowRows := m.NewWorkflowStateRowBuilder()
		taskRows := m.NewTaskStateRowBuilder()

		tx.ExpectBegin()
		setupQuery(queries, workflowRows, taskRows)
		tx.ExpectCommit()

		err := runTest()
		assert.NoError(t, err)
		m.ExpectationsWereMet()
	})
}

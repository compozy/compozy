package sqlite

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

type rowScanner interface {
	Scan(dest ...any) error
}

type taskStateRow struct {
	component      string
	status         string
	taskExecID     string
	taskID         string
	workflowID     string
	workflowExecID string
	executionType  string
	usageJSON      sql.NullString
	agentID        sql.NullString
	toolID         sql.NullString
	actionID       sql.NullString
	parentStateID  sql.NullString
	inputJSON      sql.NullString
	outputJSON     sql.NullString
	errorJSON      sql.NullString
	createdAt      string
	updatedAt      string
}

func scanTaskStateRow(scanner rowScanner) (*taskStateRow, error) {
	var row taskStateRow
	if err := scanner.Scan(
		&row.component,
		&row.status,
		&row.taskExecID,
		&row.taskID,
		&row.workflowID,
		&row.workflowExecID,
		&row.executionType,
		&row.usageJSON,
		&row.agentID,
		&row.toolID,
		&row.actionID,
		&row.parentStateID,
		&row.inputJSON,
		&row.outputJSON,
		&row.errorJSON,
		&row.createdAt,
		&row.updatedAt,
	); err != nil {
		return nil, fmt.Errorf("sqlite task: scan state row: %w", err)
	}
	return &row, nil
}

func (r *taskStateRow) toState() (*task.State, error) {
	state := &task.State{
		Component:      core.ComponentType(r.component),
		Status:         core.StatusType(r.status),
		TaskExecID:     core.ID(r.taskExecID),
		TaskID:         r.taskID,
		WorkflowID:     r.workflowID,
		WorkflowExecID: core.ID(r.workflowExecID),
		ExecutionType:  task.ExecutionType(r.executionType),
	}
	if r.agentID.Valid {
		id := r.agentID.String
		state.AgentID = &id
	}
	if r.toolID.Valid {
		id := r.toolID.String
		state.ToolID = &id
	}
	if r.actionID.Valid {
		id := r.actionID.String
		state.ActionID = &id
	}
	if r.parentStateID.Valid && strings.TrimSpace(r.parentStateID.String) != "" {
		parent := core.ID(r.parentStateID.String)
		state.ParentStateID = &parent
	}
	if err := decodeUsage(r.usageJSON, &state.Usage); err != nil {
		return nil, err
	}
	if err := decodeInput(r.inputJSON, &state.Input); err != nil {
		return nil, err
	}
	if err := decodeOutput(r.outputJSON, &state.Output); err != nil {
		return nil, err
	}
	if err := decodeError(r.errorJSON, &state.Error); err != nil {
		return nil, err
	}
	createdAt, err := parseSQLiteTime(r.createdAt)
	if err != nil {
		return nil, fmt.Errorf("sqlite task: parse created_at: %w", err)
	}
	updatedAt, err := parseSQLiteTime(r.updatedAt)
	if err != nil {
		return nil, fmt.Errorf("sqlite task: parse updated_at: %w", err)
	}
	state.CreatedAt = createdAt
	state.UpdatedAt = updatedAt
	return state, nil
}

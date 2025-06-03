package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
)

// TaskErrNotFound is returned when a task state is not found.
var TaskErrNotFound = fmt.Errorf("task state not found")

// TaskRepo implements the task.Repository interface.
type TaskRepo struct {
	db DBInterface
}

func NewTaskRepo(db DBInterface) *TaskRepo {
	return &TaskRepo{db: db}
}

// ListStates retrieves task states based on the provided filter.
func (r *TaskRepo) ListStates(ctx context.Context, filter *task.StateFilter) ([]*task.State, error) {
	sb := squirrel.Select(
		"task_exec_id", "task_id", "workflow_exec_id", "workflow_id",
		"status", "agent_id", "tool_id", "input", "output", "error",
	).
		From("task_states").
		PlaceholderFormat(squirrel.Dollar)

	if filter != nil {
		if filter.Status != nil {
			sb = sb.Where("status = ?", *filter.Status)
		}
		if filter.WorkflowID != nil {
			sb = sb.Where("workflow_id = ?", *filter.WorkflowID)
		}
		if filter.WorkflowExecID != nil {
			sb = sb.Where("workflow_exec_id = ?", *filter.WorkflowExecID)
		}
		if filter.TaskID != nil {
			sb = sb.Where("task_id = ?", *filter.TaskID)
		}
		if filter.TaskExecID != nil {
			sb = sb.Where("task_exec_id = ?", *filter.TaskExecID)
		}
		if filter.AgentID != nil {
			sb = sb.Where("agent_id = ?", *filter.AgentID)
		}
		if filter.ToolID != nil {
			sb = sb.Where("tool_id = ?", *filter.ToolID)
		}
	}

	sql, args, err := sb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building query: %w", err)
	}

	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, sql, args...); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}

	var states []*task.State
	for _, stateDB := range statesDB {
		state, err := stateDB.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting state: %w", err)
		}
		states = append(states, state)
	}

	return states, nil
}

// UpsertState inserts or updates a task state.
func (r *TaskRepo) UpsertState(
	ctx context.Context,
	workflowID string,
	workflowExecID core.ID,
	state *task.State,
) error {
	input, err := ToJSONB(state.Input)
	if err != nil {
		return fmt.Errorf("marshaling input: %w", err)
	}
	output, err := ToJSONB(state.Output)
	if err != nil {
		return fmt.Errorf("marshaling output: %w", err)
	}
	errJSON, err := ToJSONB(state.Error)
	if err != nil {
		return fmt.Errorf("marshaling error: %w", err)
	}

	query := `
		INSERT INTO task_states (
			task_exec_id, task_id, workflow_exec_id, workflow_id, status,
			agent_id, tool_id, input, output, error
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (task_exec_id) DO UPDATE SET
			task_id = $2,
			workflow_exec_id = $3,
			workflow_id = $4,
			status = $5,
			agent_id = $6,
			tool_id = $7,
			input = $8,
			output = $9,
			error = $10,
			updated_at = now()
	`

	_, err = r.db.Exec(ctx, query,
		state.TaskExecID, state.TaskID, workflowExecID, workflowID, state.Status,
		state.AgentID, state.ToolID, input, output, errJSON,
	)
	if err != nil {
		return fmt.Errorf("executing upsert: %w", err)
	}

	return nil
}

// GetState retrieves a task state by its StateID.
func (r *TaskRepo) GetState(
	ctx context.Context,
	workflowID string,
	workflowExecID core.ID,
	taskStateID task.StateID,
) (*task.State, error) {
	query := `
		SELECT task_exec_id, task_id, workflow_exec_id, workflow_id,
		       status, agent_id, tool_id, input, output, error
		FROM task_states
		WHERE task_exec_id = $1 AND workflow_id = $2 AND workflow_exec_id = $3
	`

	var stateDB task.StateDB
	err := pgxscan.Get(ctx, r.db, &stateDB, query, taskStateID.TaskExecID, workflowID, workflowExecID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, TaskErrNotFound
		}
		return nil, fmt.Errorf("scanning state: %w", err)
	}

	return stateDB.ToState()
}

// GetTaskByID retrieves a task state by task ID.
func (r *TaskRepo) GetTaskByID(
	ctx context.Context,
	workflowID string,
	workflowExecID core.ID,
	taskID string,
) (*task.State, error) {
	query := `
		SELECT task_exec_id, task_id, workflow_exec_id, workflow_id,
		       status, agent_id, tool_id, input, output, error
		FROM task_states
		WHERE task_id = $1 AND workflow_id = $2 AND workflow_exec_id = $3
	`

	var stateDB task.StateDB
	err := pgxscan.Get(ctx, r.db, &stateDB, query, taskID, workflowID, workflowExecID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, TaskErrNotFound
		}
		return nil, fmt.Errorf("scanning state: %w", err)
	}

	return stateDB.ToState()
}

// GetTaskByExecID retrieves a task state by task execution ID.
func (r *TaskRepo) GetTaskByExecID(
	ctx context.Context,
	workflowID string,
	workflowExecID core.ID,
	taskExecID core.ID,
) (*task.State, error) {
	query := `
		SELECT task_exec_id, task_id, workflow_exec_id, workflow_id,
		       status, agent_id, tool_id, input, output, error
		FROM task_states
		WHERE task_exec_id = $1 AND workflow_id = $2 AND workflow_exec_id = $3
	`

	var stateDB task.StateDB
	err := pgxscan.Get(ctx, r.db, &stateDB, query, taskExecID, workflowID, workflowExecID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, TaskErrNotFound
		}
		return nil, fmt.Errorf("scanning state: %w", err)
	}

	return stateDB.ToState()
}

// GetTaskByAgentID retrieves a task state by agent ID.
func (r *TaskRepo) GetTaskByAgentID(
	ctx context.Context,
	workflowID string,
	workflowExecID core.ID,
	agentID string,
) (*task.State, error) {
	query := `
		SELECT task_exec_id, task_id, workflow_exec_id, workflow_id,
		       status, agent_id, tool_id, input, output, error
		FROM task_states
		WHERE agent_id = $1 AND workflow_id = $2 AND workflow_exec_id = $3
	`

	var stateDB task.StateDB
	err := pgxscan.Get(ctx, r.db, &stateDB, query, agentID, workflowID, workflowExecID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, TaskErrNotFound
		}
		return nil, fmt.Errorf("scanning state: %w", err)
	}

	return stateDB.ToState()
}

// GetTaskByToolID retrieves a task state by tool ID.
func (r *TaskRepo) GetTaskByToolID(
	ctx context.Context,
	workflowID string,
	workflowExecID core.ID,
	toolID string,
) (*task.State, error) {
	query := `
		SELECT task_exec_id, task_id, workflow_exec_id, workflow_id,
		       status, agent_id, tool_id, input, output, error
		FROM task_states
		WHERE tool_id = $1 AND workflow_id = $2 AND workflow_exec_id = $3
	`

	var stateDB task.StateDB
	err := pgxscan.Get(ctx, r.db, &stateDB, query, toolID, workflowID, workflowExecID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, TaskErrNotFound
		}
		return nil, fmt.Errorf("scanning state: %w", err)
	}

	return stateDB.ToState()
}

// ListTasksInWorkflow retrieves all task states for a workflow execution.
func (r *TaskRepo) ListTasksInWorkflow(
	ctx context.Context,
	workflowExecID core.ID,
) (map[string]*task.State, error) {
	query := `
		SELECT task_exec_id, task_id, workflow_exec_id, workflow_id,
		       status, agent_id, tool_id, input, output, error
		FROM task_states
		WHERE workflow_exec_id = $1
	`

	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, query, workflowExecID); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}

	result := make(map[string]*task.State)
	for _, stateDB := range statesDB {
		state, err := stateDB.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting state: %w", err)
		}
		result[state.TaskID] = state
	}

	return result, nil
}

// ListTasksByStatus retrieves task states by status.
func (r *TaskRepo) ListTasksByStatus(
	ctx context.Context,
	workflowExecID core.ID,
	status core.StatusType,
) ([]*task.State, error) {
	query := `
		SELECT task_exec_id, task_id, workflow_exec_id, workflow_id,
		       status, agent_id, tool_id, input, output, error
		FROM task_states
		WHERE workflow_exec_id = $1 AND status = $2
	`

	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, query, workflowExecID, status); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}

	var states []*task.State
	for _, stateDB := range statesDB {
		state, err := stateDB.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting state: %w", err)
		}
		states = append(states, state)
	}

	return states, nil
}

// ListTasksByAgent retrieves task states by agent ID.
func (r *TaskRepo) ListTasksByAgent(
	ctx context.Context,
	workflowExecID core.ID,
	agentID string,
) ([]*task.State, error) {
	query := `
		SELECT task_exec_id, task_id, workflow_exec_id, workflow_id,
		       status, agent_id, tool_id, input, output, error
		FROM task_states
		WHERE workflow_exec_id = $1 AND agent_id = $2
	`

	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, query, workflowExecID, agentID); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}

	var states []*task.State
	for _, stateDB := range statesDB {
		state, err := stateDB.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting state: %w", err)
		}
		states = append(states, state)
	}

	return states, nil
}

// ListTasksByTool retrieves task states by tool ID.
func (r *TaskRepo) ListTasksByTool(
	ctx context.Context,
	workflowExecID core.ID,
	toolID string,
) ([]*task.State, error) {
	query := `
		SELECT task_exec_id, task_id, workflow_exec_id, workflow_id,
		       status, agent_id, tool_id, input, output, error
		FROM task_states
		WHERE workflow_exec_id = $1 AND tool_id = $2
	`

	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, query, workflowExecID, toolID); err != nil {
		return nil, fmt.Errorf("scanning states: %w", err)
	}

	var states []*task.State
	for _, stateDB := range statesDB {
		state, err := stateDB.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting state: %w", err)
		}
		states = append(states, state)
	}

	return states, nil
}

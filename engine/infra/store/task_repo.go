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

// ErrTaskNotFound is returned when a task state is not found.
var ErrTaskNotFound = fmt.Errorf("task state not found")

// TaskRepo implements the task.Repository interface.
type TaskRepo struct {
	db DBInterface
}

func NewTaskRepo(db DBInterface) *TaskRepo {
	return &TaskRepo{db: db}
}

// ListStates retrieves task states based on the provided filter.
func (r *TaskRepo) ListStates(ctx context.Context, filter *task.StateFilter) ([]*task.State, error) {
	sb := squirrel.Select("*").
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
		if filter.ActionID != nil {
			sb = sb.Where("action_id = ?", *filter.ActionID)
		}
		if filter.ToolID != nil {
			sb = sb.Where("tool_id = ?", *filter.ToolID)
		}
		if filter.ExecutionType != nil {
			sb = sb.Where("execution_type = ?", *filter.ExecutionType)
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

// UpsertState inserts or updates a task state (supports both basic and parallel execution).
func (r *TaskRepo) UpsertState(ctx context.Context, state *task.State) error {
	// Marshal common fields
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

	// Handle execution type specific fields
	var parallelStateJSON []byte
	if state.IsParallel() {
		parallelStateJSON, err = ToJSONB(state.ParallelState)
		if err != nil {
			return fmt.Errorf("marshaling parallel state: %w", err)
		}
	}

	query := `
		INSERT INTO task_states (
			task_exec_id, task_id, workflow_exec_id, workflow_id, component, status,
			execution_type, agent_id, action_id, tool_id, input, output, error, parallel_state
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (task_exec_id) DO UPDATE SET
			task_id = $2,
			workflow_exec_id = $3,
			workflow_id = $4,
			component = $5,
			status = $6,
			execution_type = $7,
			agent_id = $8,
			action_id = $9,
			tool_id = $10,
			input = $11,
			output = $12,
			error = $13,
			parallel_state = $14,
			updated_at = now()
	`

	_, err = r.db.Exec(ctx, query,
		state.TaskExecID, state.TaskID, state.WorkflowExecID, state.WorkflowID,
		state.Component, state.Status, state.ExecutionType,
		state.AgentID, state.ActionID, state.ToolID,
		input, output, errJSON, parallelStateJSON,
	)
	if err != nil {
		return fmt.Errorf("executing upsert: %w", err)
	}

	return nil
}

// GetState retrieves a task state by its task execution ID.
func (r *TaskRepo) GetState(ctx context.Context, taskExecID core.ID) (*task.State, error) {
	query := `
		SELECT *
		FROM task_states
		WHERE task_exec_id = $1
	`

	var stateDB task.StateDB
	err := pgxscan.Get(ctx, r.db, &stateDB, query, taskExecID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("scanning state: %w", err)
	}

	return stateDB.ToState()
}

// ListTasksInWorkflow retrieves all task states for a workflow execution.
func (r *TaskRepo) ListTasksInWorkflow(ctx context.Context, workflowExecID core.ID) (map[string]*task.State, error) {
	query := `
		SELECT *
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
		SELECT *
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
		SELECT *
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
func (r *TaskRepo) ListTasksByTool(ctx context.Context, workflowExecID core.ID, toolID string) ([]*task.State, error) {
	query := `
		SELECT *
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

// -----------------------------------------------------------------------------
// Parallel Execution Specific Methods
// -----------------------------------------------------------------------------

// ListParallelTasks retrieves all parallel execution tasks for a workflow.
func (r *TaskRepo) ListParallelTasks(ctx context.Context, workflowExecID core.ID) ([]*task.State, error) {
	query := `
		SELECT *
		FROM task_states
		WHERE workflow_exec_id = $1 AND execution_type = 'parallel'
	`

	var statesDB []*task.StateDB
	if err := pgxscan.Select(ctx, r.db, &statesDB, query, workflowExecID); err != nil {
		return nil, fmt.Errorf("scanning parallel tasks: %w", err)
	}

	var states []*task.State
	for _, stateDB := range statesDB {
		state, err := stateDB.ToState()
		if err != nil {
			return nil, fmt.Errorf("converting parallel task state: %w", err)
		}
		states = append(states, state)
	}

	return states, nil
}

// UpdateSubTaskStatus updates the status of a sub-task within a parallel execution.
func (r *TaskRepo) UpdateSubTaskStatus(
	ctx context.Context,
	parentTaskExecID core.ID,
	subTaskID string,
	status core.StatusType,
	output *core.Output,
	err *core.Error,
) error {
	// First, get the current parallel task state
	parentState, getErr := r.GetState(ctx, parentTaskExecID)
	if getErr != nil {
		return fmt.Errorf("failed to get parent task state: %w", getErr)
	}

	if !parentState.IsParallel() {
		return fmt.Errorf("task %s is not a parallel execution", parentTaskExecID)
	}

	// Update the sub-task status in memory
	updateErr := parentState.UpdateSubTaskStatus(subTaskID, status, output, err)
	if updateErr != nil {
		return fmt.Errorf("failed to update sub-task status: %w", updateErr)
	}

	// Persist the updated parallel state
	return r.UpsertState(ctx, parentState)
}

// GetSubTaskProgress returns progress information for a parallel task.
func (r *TaskRepo) GetSubTaskProgress(
	ctx context.Context,
	taskExecID core.ID,
) (completed, failed, total int, err error) {
	state, err := r.GetState(ctx, taskExecID)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get task state: %w", err)
	}

	if !state.IsParallel() {
		return 0, 0, 0, fmt.Errorf("task %s is not a parallel execution", taskExecID)
	}

	completed, failed, total = state.GetParallelProgress()
	return completed, failed, total, nil
}

// ListRunningSubTasks returns all currently running sub-tasks within a parallel execution.
func (r *TaskRepo) ListRunningSubTasks(ctx context.Context, parentTaskExecID core.ID) ([]string, error) {
	state, err := r.GetState(ctx, parentTaskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent task state: %w", err)
	}

	if !state.IsParallel() || state.ParallelState == nil {
		return nil, fmt.Errorf("task %s is not a parallel execution", parentTaskExecID)
	}

	var runningTasks []string
	for taskID, subTask := range state.ParallelState.SubTasks {
		if subTask.Status == core.StatusRunning || subTask.Status == core.StatusPending {
			runningTasks = append(runningTasks, taskID)
		}
	}

	return runningTasks, nil
}

// GetParallelTaskBySubTaskID finds a parallel task that contains a specific sub-task.
func (r *TaskRepo) GetParallelTaskBySubTaskID(
	ctx context.Context,
	workflowExecID core.ID,
	subTaskID string,
) (*task.State, error) {
	query := `
		SELECT *
		FROM task_states
		WHERE workflow_exec_id = $1
		  AND execution_type = 'parallel'
		  AND parallel_state->'sub_tasks' ? $2
	`

	var stateDB task.StateDB
	err := pgxscan.Get(ctx, r.db, &stateDB, query, workflowExecID, subTaskID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("parallel task containing sub-task %s not found", subTaskID)
		}
		return nil, fmt.Errorf("scanning parallel task state: %w", err)
	}

	return stateDB.ToState()
}

// BulkUpdateSubTaskStatuses updates multiple sub-task statuses in a single transaction.
func (r *TaskRepo) BulkUpdateSubTaskStatuses(
	ctx context.Context,
	parentTaskExecID core.ID,
	updates map[string]struct {
		Status core.StatusType
		Output *core.Output
		Error  *core.Error
	},
) error {
	// Get current state
	parentState, err := r.GetState(ctx, parentTaskExecID)
	if err != nil {
		return fmt.Errorf("failed to get parent task state: %w", err)
	}

	if !parentState.IsParallel() {
		return fmt.Errorf("task %s is not a parallel execution", parentTaskExecID)
	}

	// Apply all updates
	for subTaskID, update := range updates {
		err := parentState.UpdateSubTaskStatus(subTaskID, update.Status, update.Output, update.Error)
		if err != nil {
			return fmt.Errorf("failed to update sub-task %s: %w", subTaskID, err)
		}
	}

	// Persist the updated state
	return r.UpsertState(ctx, parentState)
}

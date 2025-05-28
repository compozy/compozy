package store

import (
	"context"
	"database/sql"
	"fmt"
	"slices"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	db "github.com/compozy/compozy/engine/infra/store/sqlc"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/pb"
)

type TaskRepository struct {
	store        *Store
	workflowRepo *WorkflowRepository
}

func (s *Store) NewTaskRepository(workflowRepo *WorkflowRepository) *TaskRepository {
	return &TaskRepository{store: s, workflowRepo: workflowRepo}
}

func (r *TaskRepository) CreateExecution(
	ctx context.Context,
	metadata *pb.TaskMetadata,
	config *task.Config,
) (*task.Execution, error) {
	workflowExecID := core.ID(metadata.WorkflowExecId)
	workflowExecution, err := r.workflowRepo.GetExecution(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow execution: %w", err)
	}

	// Get the main execution map for template context
	mainExecMap, err := r.workflowRepo.ExecutionToMap(ctx, workflowExecution)
	if err != nil {
		return nil, fmt.Errorf("failed to get main execution map: %w", err)
	}

	workflowEnv := workflowExecution.GetEnv()
	taskEnv := config.GetEnv()
	taskInput := config.GetInput()
	parentInput := workflowExecution.GetInput()
	requestData, err := task.NewRequestData(
		metadata,
		parentInput,
		taskInput,
		workflowEnv,
		taskEnv,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create task request data: %w", err)
	}
	execution, err := task.NewExecutionWithContext(requestData, mainExecMap)
	if err != nil {
		return nil, fmt.Errorf("failed to create task execution: %w", err)
	}
	key := execution.StoreKey()
	if err := r.store.UpsertJSON(ctx, key, execution); err != nil {
		return nil, fmt.Errorf("failed to save task execution: %w", err)
	}
	return execution, nil
}

func (r *TaskRepository) GetExecution(
	ctx context.Context,
	taskExecID core.ID,
) (*task.Execution, error) {
	data, err := r.store.queries.GetTaskExecutionByExecID(ctx, taskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task execution: %w", err)
	}
	return core.UnmarshalExecution[*task.Execution](data.Data)
}

func (r *TaskRepository) ListExecutions(ctx context.Context) ([]task.Execution, error) {
	execs, err := r.store.queries.ListTaskExecutions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list task executions by status: %w", err)
	}
	return UnmarshalExecutions[task.Execution](execs)
}

func (r *TaskRepository) ListExecutionsByStatus(ctx context.Context, status core.StatusType) ([]task.Execution, error) {
	execs, err := r.store.queries.ListTaskExecutionsByStatus(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list task executions by status: %w", err)
	}
	return UnmarshalExecutions[task.Execution](execs)
}

func (r *TaskRepository) ListExecutionsByWorkflowID(ctx context.Context, workflowID string) ([]task.Execution, error) {
	execs, err := r.store.queries.ListTaskExecutionsByWorkflowID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to list task executions by workflow ID: %w", err)
	}
	return UnmarshalExecutions[task.Execution](execs)
}

func (r *TaskRepository) ListExecutionsByWorkflowExecID(
	ctx context.Context,
	workflowExecID core.ID,
) ([]task.Execution, error) {
	execs, err := r.store.queries.ListTaskExecutionsByWorkflowExecID(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to list task executions by workflow exec ID: %w", err)
	}
	return UnmarshalExecutions[task.Execution](execs)
}

func (r *TaskRepository) ListExecutionsByTaskID(ctx context.Context, taskID string) ([]task.Execution, error) {
	execID := sql.NullString{String: taskID, Valid: true}
	execs, err := r.store.queries.ListTaskExecutionsByTaskID(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("failed to list task executions by task ID: %w", err)
	}
	return UnmarshalExecutions[task.Execution](execs)
}

func (r *TaskRepository) ListExecutionsByWorkflowAndTaskID(
	ctx context.Context,
	workflowID, taskID string,
) ([]task.Execution, error) {
	arg := db.ListTaskExecutionsByWorkflowAndTaskIDParams{
		WorkflowID: workflowID,
		TaskID:     sql.NullString{String: taskID, Valid: true},
	}
	execs, err := r.store.queries.ListTaskExecutionsByWorkflowAndTaskID(ctx, arg)
	if err != nil {
		return nil, fmt.Errorf("failed to list task executions by workflow and task ID: %w", err)
	}
	return UnmarshalExecutions[task.Execution](execs)
}

func (r *TaskRepository) ListChildrenExecutions(ctx context.Context, taskExecID core.ID) ([]core.Execution, error) {
	execs, err := r.store.queries.ListTaskChildrenExecutionsByTaskExecID(ctx, taskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to list task children executions: %w", err)
	}
	agents, tools, err := r.BuildExecutions(ctx, execs)
	if err != nil {
		return nil, fmt.Errorf("failed to build executions map: %w", err)
	}
	return slices.Concat(agents, tools), nil
}

func (r *TaskRepository) ListChildrenExecutionsByTaskID(ctx context.Context, taskID string) ([]core.Execution, error) {
	tID := sql.NullString{String: taskID, Valid: true}
	execs, err := r.store.queries.ListTaskChildrenExecutionsByTaskID(ctx, tID)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow children executions: %w", err)
	}
	agents, tools, err := r.BuildExecutions(ctx, execs)
	if err != nil {
		return nil, fmt.Errorf("failed to build executions map: %w", err)
	}
	return slices.Concat(agents, tools), nil
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func (r *TaskRepository) ExecutionsToMap(_ context.Context, execs []core.Execution) ([]*core.ExecutionMap, error) {
	execMaps := make([]*core.ExecutionMap, len(execs))
	for i, exec := range execs {
		execMaps[i] = exec.AsExecMap()
	}
	return execMaps, nil
}

func (r *TaskRepository) BuildExecutions(_ context.Context, execs []db.Execution) (
	[]core.Execution,
	[]core.Execution,
	error,
) {
	agents := make([]core.Execution, 0)
	tools := make([]core.Execution, 0)
	for i := range execs {
		switch execs[i].ComponentType {
		case core.ComponentAgent:
			item, err := core.UnmarshalExecution[*agent.Execution](execs[i].Data)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to unmarshal agent execution: %w", err)
			}
			agents = append(agents, item)
		case core.ComponentTool:
			item, err := core.UnmarshalExecution[*tool.Execution](execs[i].Data)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to unmarshal tool execution: %w", err)
			}
			tools = append(tools, item)
		}
	}
	return agents, tools, nil
}

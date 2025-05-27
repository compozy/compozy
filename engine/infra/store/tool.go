package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/pb"
)

type ToolRepository struct {
	store        *Store
	workflowRepo *WorkflowRepository
	taskRepo     *TaskRepository
}

func (s *Store) NewToolRepository(
	workflowRepo *WorkflowRepository,
	taskRepo *TaskRepository,
) *ToolRepository {
	return &ToolRepository{
		store:        s,
		workflowRepo: workflowRepo,
		taskRepo:     taskRepo,
	}
}

func (r *ToolRepository) CreateExecution(
	ctx context.Context,
	metadata *pb.ToolMetadata,
	config *tool.Config,
) (*tool.Execution, error) {
	workflowExecID := core.ID(metadata.WorkflowExecId)
	workflowExecution, err := r.workflowRepo.GetExecution(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow execution: %w", err)
	}
	taskExecID := core.ID(metadata.TaskExecId)
	taskExecution, err := r.taskRepo.GetExecution(ctx, taskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to load task execution: %w", err)
	}
	parentInput := workflowExecution.GetInput()
	toolEnv := config.GetEnv()
	taskEnv := taskExecution.GetEnv()
	taskInput := taskExecution.GetInput()
	toolInput := config.GetInput()
	requestData, err := tool.NewRequestData(
		metadata,
		parentInput,
		taskInput,
		toolInput,
		taskEnv,
		toolEnv,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool request data: %w", err)
	}
	execution, err := tool.NewExecution(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool execution: %w", err)
	}
	key := execution.StoreKey()
	if err := r.store.UpsertJSON(ctx, key, execution); err != nil {
		return nil, fmt.Errorf("failed to save tool execution: %w", err)
	}
	return execution, nil
}

func (r *ToolRepository) GetExecution(
	ctx context.Context,
	toolExecID core.ID,
) (*tool.Execution, error) {
	data, err := r.store.queries.GetToolExecutionByExecID(ctx, toolExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool execution: %w", err)
	}
	return core.UnmarshalExecution[*tool.Execution](data.Data)
}

func (r *ToolRepository) ListExecutions(ctx context.Context) ([]tool.Execution, error) {
	execs, err := r.store.queries.ListToolExecutions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tool executions by status: %w", err)
	}
	return UnmarshalExecutions[tool.Execution](execs)
}

func (r *ToolRepository) ListExecutionsByStatus(ctx context.Context, status core.StatusType) ([]tool.Execution, error) {
	execs, err := r.store.queries.ListToolExecutionsByStatus(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list tool executions by status: %w", err)
	}
	return UnmarshalExecutions[tool.Execution](execs)
}

func (r *ToolRepository) ListExecutionsByWorkflowID(ctx context.Context, workflowID string) ([]tool.Execution, error) {
	execs, err := r.store.queries.ListToolExecutionsByWorkflowID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tool executions by workflow ID: %w", err)
	}
	return UnmarshalExecutions[tool.Execution](execs)
}

func (r *ToolRepository) ListExecutionsByWorkflowExecID(
	ctx context.Context,
	workflowExecID core.ID,
) ([]tool.Execution, error) {
	execs, err := r.store.queries.ListToolExecutionsByWorkflowExecID(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tool executions by workflow exec ID: %w", err)
	}
	return UnmarshalExecutions[tool.Execution](execs)
}

func (r *ToolRepository) ListExecutionsByTaskID(ctx context.Context, taskID string) ([]tool.Execution, error) {
	execID := sql.NullString{String: taskID, Valid: true}
	execs, err := r.store.queries.ListToolExecutionsByTaskID(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tool executions by task ID: %w", err)
	}
	return UnmarshalExecutions[tool.Execution](execs)
}

func (r *ToolRepository) ListExecutionsByTaskExecID(
	ctx context.Context,
	taskExecID core.ID,
) ([]tool.Execution, error) {
	execs, err := r.store.queries.ListToolExecutionsByTaskExecID(ctx, taskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tool executions by task exec ID: %w", err)
	}
	return UnmarshalExecutions[tool.Execution](execs)
}

func (r *ToolRepository) ListExecutionsByToolID(ctx context.Context, toolID string) ([]tool.Execution, error) {
	execID := sql.NullString{String: toolID, Valid: true}
	execs, err := r.store.queries.ListToolExecutionsByToolID(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tool executions by tool ID: %w", err)
	}
	return UnmarshalExecutions[tool.Execution](execs)
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func (r *ToolRepository) ExecutionsToMap(_ context.Context, execs []core.Execution) ([]*core.ExecutionMap, error) {
	execMaps := make([]*core.ExecutionMap, len(execs))
	for i, exec := range execs {
		execMaps[i] = exec.AsExecMap()
	}
	return execMaps, nil
}

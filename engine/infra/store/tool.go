package store

import (
	"context"
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

func NewToolRepository(
	store *Store,
	workflowRepo *WorkflowRepository,
	taskRepo *TaskRepository,
) *ToolRepository {
	return &ToolRepository{
		store:        store,
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
	workflowExecution, err := r.workflowRepo.LoadExecution(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow execution: %w", err)
	}
	taskExecID := core.ID(metadata.TaskExecId)
	taskExecution, err := r.taskRepo.LoadExecution(ctx, workflowExecID, taskExecID)
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

func (r *ToolRepository) LoadExecution(
	ctx context.Context,
	wExecID core.ID,
	toolExecID core.ID,
) (*tool.Execution, error) {
	key := tool.NewStoreKey(wExecID, toolExecID)
	execution, err := GetAndUnmarshalJSON[tool.Execution](ctx, r.store, key.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to get tool execution: %w", err)
	}
	return execution, nil
}

func (r *ToolRepository) LoadExecutionsJSON(
	ctx context.Context,
	wExecID core.ID,
) (map[core.ID]core.JSONMap, error) {
	executions, err := GetExecutionsByComponent[*tool.Execution](ctx, r.store, wExecID, core.ComponentTool)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool executions: %w", err)
	}
	jsonMap := make(map[core.ID]core.JSONMap)
	for _, execution := range executions {
		jsonMap[execution.GetID()] = core.JSONMapFromExecution(execution)
	}
	return jsonMap, nil
}

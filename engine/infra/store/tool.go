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
	workflowExecution, err := r.workflowRepo.LoadExecution(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow execution: %w", err)
	}
	taskExecID := core.ID(metadata.TaskExecId)
	taskExecution, err := r.taskRepo.LoadExecution(ctx, taskExecID)
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
	toolExecID core.ID,
) (*tool.Execution, error) {
	data, err := r.store.GetToolExecutionByExecID(ctx, toolExecID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get tool execution: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("tool execution not found")
	}

	execution, err := unmarshalExecution[*tool.Execution](*data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal tool execution: %w", err)
	}

	return execution, nil
}

func (r *ToolRepository) LoadExecutionsMapByWorkflowExecID(
	ctx context.Context,
	wExecID core.ID,
) (map[core.ID]any, error) {
	data, err := r.store.ListToolExecutionsByWorkflowExecID(ctx, string(wExecID))
	if err != nil {
		return nil, fmt.Errorf("failed to get tool executions: %w", err)
	}
	executions, err := unmarshalExecutions[*tool.Execution](data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal tool executions: %w", err)
	}
	jsonMap := make(map[core.ID]any)
	for _, execution := range executions {
		jsonMap[execution.GetID()] = execution.AsMap()
	}
	return jsonMap, nil
}

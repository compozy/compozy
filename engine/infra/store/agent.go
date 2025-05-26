package store

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/pb"
)

type AgentRepository struct {
	store        *Store
	workflowRepo *WorkflowRepository
	taskRepo     *TaskRepository
}

func NewAgentRepository(
	store *Store,
	workflowRepo *WorkflowRepository,
	taskRepo *TaskRepository,
) *AgentRepository {
	return &AgentRepository{
		store:        store,
		workflowRepo: workflowRepo,
		taskRepo:     taskRepo,
	}
}

func (r *AgentRepository) CreateExecution(
	ctx context.Context,
	metadata *pb.AgentMetadata,
	config *agent.Config,
) (*agent.Execution, error) {
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
	agentEnv := config.GetEnv()
	taskEnv := taskExecution.GetEnv()
	taskInput := taskExecution.GetInput()
	agentInput := config.GetInput()
	requestData, err := agent.NewRequestData(
		metadata,
		parentInput,
		taskInput,
		agentInput,
		taskEnv,
		agentEnv,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent request data: %w", err)
	}
	execution, err := agent.NewExecution(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent execution: %w", err)
	}
	key := execution.StoreKey()
	if err := r.store.UpsertJSON(ctx, key, execution); err != nil {
		return nil, fmt.Errorf("failed to save agent execution: %w", err)
	}
	return execution, nil
}

func (r *AgentRepository) LoadExecution(
	ctx context.Context,
	wExecID core.ID,
	agentExecID core.ID,
) (*agent.Execution, error) {
	key := agent.NewStoreKey(wExecID, agentExecID)
	execution, err := GetAndUnmarshalJSON[agent.Execution](ctx, r.store, key.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to get agent execution: %w", err)
	}
	return execution, nil
}

func (r *AgentRepository) LoadExecutionsJSON(
	ctx context.Context,
	wExecID core.ID,
) (map[core.ID]core.JSONMap, error) {
	executions, err := GetExecutionsByComponent[*agent.Execution](ctx, r.store, wExecID, core.ComponentAgent)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent executions: %w", err)
	}
	jsonMap := make(map[core.ID]core.JSONMap)
	for _, execution := range executions {
		jsonMap[execution.GetID()] = core.JSONMapFromExecution(execution)
	}
	return jsonMap, nil
}

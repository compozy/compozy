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

func (s *Store) NewAgentRepository(
	workflowRepo *WorkflowRepository,
	taskRepo *TaskRepository,
) *AgentRepository {
	return &AgentRepository{
		store:        s,
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
	taskExecution, err := r.taskRepo.LoadExecution(ctx, taskExecID)
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
	agentExecID core.ID,
) (*agent.Execution, error) {
	data, err := r.store.GetAgentExecutionByExecID(ctx, agentExecID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get agent execution: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("agent execution not found")
	}
	return unmarshalExecution[*agent.Execution](*data)
}

func (r *AgentRepository) LoadExecutionsMapByWorkflowExecID(
	ctx context.Context,
	wExecID core.ID,
) (map[core.ID]any, error) {
	workflowExecID := wExecID.String()
	data, err := r.store.ListAgentExecutionsByWorkflowExecID(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent executions: %w", err)
	}
	executions, err := unmarshalExecutions[*agent.Execution](data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal agent executions: %w", err)
	}
	jsonMap := make(map[core.ID]any)
	for _, execution := range executions {
		jsonMap[execution.GetID()] = execution.AsMap()
	}
	return jsonMap, nil
}

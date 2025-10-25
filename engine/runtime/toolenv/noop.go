package toolenv

import (
	"context"
	"fmt"
)

type noopAgentExecutor struct{}

// NoopAgentExecutor returns an AgentExecutor that fails when invoked.
func NoopAgentExecutor() AgentExecutor { //nolint:ireturn // helper returns interface for tests
	return noopAgentExecutor{}
}

func (noopAgentExecutor) ExecuteAgent(context.Context, AgentRequest) (*AgentResult, error) {
	return nil, fmt.Errorf("tool environment: agent executor not configured")
}

type noopTaskExecutor struct{}

// NoopTaskExecutor returns a TaskExecutor that fails when invoked.
func NoopTaskExecutor() TaskExecutor { //nolint:ireturn // helper returns interface for tests
	return noopTaskExecutor{}
}

func (noopTaskExecutor) ExecuteTask(context.Context, TaskRequest) (*TaskResult, error) {
	return nil, fmt.Errorf("tool environment: task executor not configured")
}

type noopWorkflowExecutor struct{}

// NoopWorkflowExecutor returns a WorkflowExecutor that fails when invoked.
func NoopWorkflowExecutor() WorkflowExecutor { //nolint:ireturn // helper returns interface for tests
	return noopWorkflowExecutor{}
}

func (noopWorkflowExecutor) ExecuteWorkflow(context.Context, WorkflowRequest) (*WorkflowResult, error) {
	return nil, fmt.Errorf("tool environment: workflow executor not configured")
}

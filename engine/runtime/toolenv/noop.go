package toolenv

import (
	"context"
	"errors"
)

var (
	// ErrAgentExecutorNotConfigured indicates that a test attempted to run without wiring an agent executor.
	ErrAgentExecutorNotConfigured = errors.New("tool environment: agent executor not configured")
	// ErrTaskExecutorNotConfigured indicates that a test attempted to run without wiring a task executor.
	ErrTaskExecutorNotConfigured = errors.New("tool environment: task executor not configured")
	// ErrWorkflowExecutorNotConfigured indicates that a test attempted to run without wiring a workflow executor.
	ErrWorkflowExecutorNotConfigured = errors.New("tool environment: workflow executor not configured")
)

type noopAgentExecutor struct{}

// NoopAgentExecutor returns an AgentExecutor that fails when invoked.
func NoopAgentExecutor() AgentExecutor { //nolint:ireturn // helper returns interface for tests
	return noopAgentExecutor{}
}

func (noopAgentExecutor) ExecuteAgent(context.Context, AgentRequest) (*AgentResult, error) {
	return nil, ErrAgentExecutorNotConfigured
}

type noopTaskExecutor struct{}

// NoopTaskExecutor returns a TaskExecutor that fails when invoked.
func NoopTaskExecutor() TaskExecutor { //nolint:ireturn // helper returns interface for tests
	return noopTaskExecutor{}
}

func (noopTaskExecutor) ExecuteTask(context.Context, TaskRequest) (*TaskResult, error) {
	return nil, ErrTaskExecutorNotConfigured
}

type noopWorkflowExecutor struct{}

// NoopWorkflowExecutor returns a WorkflowExecutor that fails when invoked.
func NoopWorkflowExecutor() WorkflowExecutor { //nolint:ireturn // helper returns interface for tests
	return noopWorkflowExecutor{}
}

func (noopWorkflowExecutor) ExecuteWorkflow(context.Context, WorkflowRequest) (*WorkflowResult, error) {
	return nil, ErrWorkflowExecutorNotConfigured
}

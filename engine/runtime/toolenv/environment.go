package toolenv

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task"
)

// AgentRequest captures the parameters required to execute an agent.
type AgentRequest struct {
	AgentID string
	Action  string
	Prompt  string
	With    core.Input
	Timeout time.Duration
}

// AgentResult represents the outcome of an agent execution.
type AgentResult struct {
	ExecID core.ID
	Output *core.Output
}

// AgentExecutor exposes synchronous agent execution capabilities to tools.
type AgentExecutor interface {
	ExecuteAgent(context.Context, AgentRequest) (*AgentResult, error)
}

// Environment provides tool handlers access to execution services without
// coupling them to concrete implementations that might introduce package cycles.
type Environment interface {
	AgentExecutor() AgentExecutor
	TaskExecutor() TaskExecutor
	WorkflowExecutor() WorkflowExecutor
	TaskRepository() task.Repository
	ResourceStore() resources.ResourceStore
}

type environment struct {
	agent    AgentExecutor
	task     TaskExecutor
	workflow WorkflowExecutor
	repo     task.Repository
	store    resources.ResourceStore
}

// New constructs an Environment using the supplied dependencies.
// All dependencies must be non-nil, otherwise an error is returned describing the missing values.
func New(
	agent AgentExecutor,
	taskExec TaskExecutor,
	workflowExec WorkflowExecutor,
	repo task.Repository,
	store resources.ResourceStore,
) (Environment, error) {
	missing := make([]string, 0, 5)
	if agent == nil {
		missing = append(missing, "agent executor")
	}
	if taskExec == nil {
		missing = append(missing, "task executor")
	}
	if workflowExec == nil {
		missing = append(missing, "workflow executor")
	}
	if repo == nil {
		missing = append(missing, "task repository")
	}
	if store == nil {
		missing = append(missing, "resource store")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("tool environment: missing dependencies: %s", strings.Join(missing, ", "))
	}
	return &environment{
		agent:    agent,
		task:     taskExec,
		workflow: workflowExec,
		repo:     repo,
		store:    store,
	}, nil
}

func (e *environment) AgentExecutor() AgentExecutor {
	return e.agent
}

func (e *environment) TaskExecutor() TaskExecutor {
	return e.task
}

func (e *environment) WorkflowExecutor() WorkflowExecutor {
	return e.workflow
}

func (e *environment) TaskRepository() task.Repository {
	return e.repo
}

func (e *environment) ResourceStore() resources.ResourceStore {
	return e.store
}

package toolenv

import (
	"context"
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
	TaskRepository() task.Repository
	ResourceStore() resources.ResourceStore
}

type environment struct {
	agent AgentExecutor
	repo  task.Repository
	store resources.ResourceStore
}

// New constructs an Environment using the supplied agent executor, repository,
// and resource store. Callers are responsible for ensuring dependencies are non-nil.
func New(agent AgentExecutor, repo task.Repository, store resources.ResourceStore) Environment {
	return &environment{
		agent: agent,
		repo:  repo,
		store: store,
	}
}

func (e *environment) AgentExecutor() AgentExecutor {
	return e.agent
}

func (e *environment) TaskRepository() task.Repository {
	return e.repo
}

func (e *environment) ResourceStore() resources.ResourceStore {
	return e.store
}

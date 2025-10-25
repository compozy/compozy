package toolenvtest

import (
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/task"
)

// NoopEnvironment implements toolenv.Environment with noop executors for tests.
type NoopEnvironment struct {
	agent    toolenv.AgentExecutor
	task     toolenv.TaskExecutor
	workflow toolenv.WorkflowExecutor
	repo     task.Repository
	store    resources.ResourceStore
}

// Option mutates the noop environment during construction.
type Option func(*NoopEnvironment)

// WithTaskRepository sets the repository returned by the noop environment.
func WithTaskRepository(repo task.Repository) Option {
	return func(env *NoopEnvironment) {
		env.repo = repo
	}
}

// WithResourceStore sets the resource store returned by the noop environment.
func WithResourceStore(store resources.ResourceStore) Option {
	return func(env *NoopEnvironment) {
		env.store = store
	}
}

// NewNoopEnvironment builds a noop environment using the provided options.
func NewNoopEnvironment(options ...Option) toolenv.Environment {
	env := &NoopEnvironment{
		agent:    toolenv.NoopAgentExecutor(),
		task:     toolenv.NoopTaskExecutor(),
		workflow: toolenv.NoopWorkflowExecutor(),
	}
	for _, opt := range options {
		if opt != nil {
			opt(env)
		}
	}
	return env
}

// AgentExecutor returns the noop agent executor.
func (n *NoopEnvironment) AgentExecutor() toolenv.AgentExecutor {
	return n.agent
}

// TaskExecutor returns the noop task executor.
func (n *NoopEnvironment) TaskExecutor() toolenv.TaskExecutor {
	return n.task
}

// WorkflowExecutor returns the noop workflow executor.
func (n *NoopEnvironment) WorkflowExecutor() toolenv.WorkflowExecutor {
	return n.workflow
}

// TaskRepository returns the configured repository, if any.
func (n *NoopEnvironment) TaskRepository() task.Repository {
	return n.repo
}

// ResourceStore returns the configured resource store, if any.
func (n *NoopEnvironment) ResourceStore() resources.ResourceStore {
	return n.store
}

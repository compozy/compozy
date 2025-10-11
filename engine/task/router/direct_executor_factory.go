package tkrouter

import (
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/task"
	directexec "github.com/compozy/compozy/engine/task/directexec"
	"github.com/compozy/compozy/engine/workflow"
)

const directExecutorFactoryKey = appstate.ExtensionKey("router.direct_executor_factory")

type DirectExecutor = directexec.DirectExecutor
type ExecMetadata = directexec.ExecMetadata

// DirectExecutorFactory builds a DirectExecutor instance for the provided state and repository.
type DirectExecutorFactory func(*appstate.State, task.Repository) (DirectExecutor, error)

// SetDirectExecutorFactory registers a factory on the application state so tests can inject overrides.
func SetDirectExecutorFactory(state *appstate.State, factory DirectExecutorFactory) {
	if state == nil {
		return
	}
	if factory == nil {
		state.SetExtension(directExecutorFactoryKey, nil)
		return
	}
	state.SetExtension(directExecutorFactoryKey, factory)
}

// ResolveDirectExecutor returns the configured DirectExecutor implementation for the given state.
func ResolveDirectExecutor(state *appstate.State, repo task.Repository) (DirectExecutor, error) {
	if state != nil {
		if value, ok := state.Extension(directExecutorFactoryKey); ok {
			factory, ok := value.(DirectExecutorFactory)
			if ok && factory != nil {
				return factory(state, repo)
			}
		}
	}
	return NewDirectExecutor(state, repo, nil)
}

func NewDirectExecutor(
	state *appstate.State,
	taskRepo task.Repository,
	workflowRepo workflow.Repository,
) (DirectExecutor, error) {
	return directexec.NewDirectExecutor(state, taskRepo, workflowRepo)
}

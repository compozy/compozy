package agentcatalog

import (
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/task"
)

type testEnvironment struct {
	store resources.ResourceStore
}

func newTestEnvironment(store resources.ResourceStore) toolenv.Environment {
	return &testEnvironment{store: store}
}

func (t *testEnvironment) AgentExecutor() toolenv.AgentExecutor {
	return toolenv.NoopAgentExecutor()
}

func (t *testEnvironment) TaskExecutor() toolenv.TaskExecutor {
	return toolenv.NoopTaskExecutor()
}

func (t *testEnvironment) WorkflowExecutor() toolenv.WorkflowExecutor {
	return toolenv.NoopWorkflowExecutor()
}

func (t *testEnvironment) TaskRepository() task.Repository {
	return nil
}

func (t *testEnvironment) ResourceStore() resources.ResourceStore {
	return t.store
}

package callagents

import (
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/task"
)

type stubEnvironment struct {
	executor toolenv.AgentExecutor
}

func (s *stubEnvironment) AgentExecutor() toolenv.AgentExecutor {
	return s.executor
}

func (s *stubEnvironment) TaskRepository() task.Repository {
	return nil
}

func (s *stubEnvironment) ResourceStore() resources.ResourceStore {
	return nil
}

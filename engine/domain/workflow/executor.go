// Package workflow provides workflow configuration and execution functionality
package workflow

import (
	"github.com/compozy/compozy/engine/domain/workflow/executor"
	"github.com/compozy/compozy/pkg/nats"
)

type (
	Executor        = executor.Executor
	ExecutorOptions = executor.Options
)

// NewExecutor creates a new workflow executor instance
func NewExecutor(natsServer *nats.Server, workflows []*Config, opts *ExecutorOptions) (*Executor, error) {
	wfConfigs := make([]executor.WorkflowConfig, len(workflows))
	for i, wf := range workflows {
		wfConfigs[i] = wf
	}
	return executor.New(natsServer, wfConfigs, opts)
}

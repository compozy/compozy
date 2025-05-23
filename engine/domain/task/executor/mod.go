package executor

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/domain/project"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/stmanager"
	"github.com/compozy/compozy/pkg/nats"
)

type Executor struct {
	nc        *nats.Client
	stManager *stmanager.Manager
	pConfig   *project.Config
	workflows []*workflow.Config
}

func NewExecutor(
	nc *nats.Client,
	stManager *stmanager.Manager,
	pConfig *project.Config,
	workflows []*workflow.Config,
) *Executor {
	return &Executor{nc: nc, stManager: stManager, pConfig: pConfig, workflows: workflows}
}

func (e *Executor) Start(ctx context.Context) error {
	if err := e.subscribeExecute(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to WorkflowExecuteCmd: %w", err)
	}
	return nil
}

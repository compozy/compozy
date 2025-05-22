package wfexecutor

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/stmanager"
	"github.com/compozy/compozy/pkg/nats"
)

type Executor struct {
	stm *stmanager.Manager
	nc  *nats.Client
	wfs []*workflow.Config
}

func NewWorkflowExecutor(nc *nats.Client, stm *stmanager.Manager, wfs []*workflow.Config) *Executor {
	return &Executor{nc: nc, stm: stm, wfs: wfs}
}

func (e *Executor) Start(ctx context.Context) error {
	if err := e.subscribeExecute(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to WorkflowExecuteCmd: %w", err)
	}
	return nil
}

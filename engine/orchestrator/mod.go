package orchestrator

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/domain/project"
	"github.com/compozy/compozy/engine/domain/workflow"
	wfexecutor "github.com/compozy/compozy/engine/domain/workflow/executor"
	"github.com/compozy/compozy/engine/stmanager"
	"github.com/compozy/compozy/engine/store"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
)

type Orchestrator struct {
	nc        *nats.Client
	stManager *stmanager.Manager
	pjc       *project.Config
	workflows []*workflow.Config
	wfexec    *wfexecutor.Executor
}

func NewOrchestrator(
	ctx context.Context,
	ns *nats.Server,
	store *store.Store,
	pjc *project.Config,
	wfs []*workflow.Config,
) (*Orchestrator, error) {
	nc, err := nats.NewClient(ns.Conn)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize NATS client: %w", err)
	}
	if err := nc.Setup(ctx); err != nil {
		return nil, fmt.Errorf("failed to setup NATS client: %w", err)
	}

	stManager, err := stmanager.NewManager(
		stmanager.WithNatsClient(nc),
		stmanager.WithStore(store),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize state manager: %w", err)
	}

	wfExec := wfexecutor.NewWorkflowExecutor(nc, stManager, wfs)
	orch := &Orchestrator{
		nc:        nc,
		stManager: stManager,
		pjc:       pjc,
		workflows: wfs,
		wfexec:    wfExec,
	}
	return orch, nil
}

func (o *Orchestrator) Start(ctx context.Context) error {
	if err := o.subscribeWorkflow(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to workflow commands: %w", err)
	}
	if err := o.stManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start state manager: %w", err)
	}
	if err := o.wfexec.Start(ctx); err != nil {
		return fmt.Errorf("failed to start workflow executor: %w", err)
	}
	return nil
}

func (o *Orchestrator) Stop(ctx context.Context) error {
	logger.Debug("Shutting down Orchestrator")
	if err := o.nc.CloseWithContext(ctx); err != nil {
		return fmt.Errorf("failed to close NATS client: %w", err)
	}
	if err := o.stManager.CloseWithContext(ctx); err != nil {
		return fmt.Errorf("failed to stop state manager: %w", err)
	}
	logger.Debug("Orchestrator stopped successfully")
	return nil
}

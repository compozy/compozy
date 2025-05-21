package orchestrator

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/domain/project"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/server"
)

type SystemComponent string

const (
	CompOrchestrator     SystemComponent = "engine.Orchestrator"
	CompWorkflowExecutor SystemComponent = "workflow.Executor"
	CompTaskExecutor     SystemComponent = "task.Executor"
)

type Orchestrator struct {
	natsClient    *nats.Client
	stManager     *state.Manager
	ProjectConfig *project.Config
	Workflows     []*workflow.Config
}

func NewOrchestartor(appState server.AppState) (*Orchestrator, error) {
	natsClient, err := nats.NewClient(appState.NatsServer.Conn)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize NATS client: %w", err)
	}

	stManager, err := state.NewManager(
		state.WithNatsClient(natsClient),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize state manager: %w", err)
	}

	return &Orchestrator{
		natsClient:    natsClient,
		stManager:     stManager,
		ProjectConfig: appState.ProjectConfig,
		Workflows:     appState.Workflows,
	}, nil
}

func (o *Orchestrator) Start(ctx context.Context) error {
	if err := o.stManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start state manager: %w", err)
	}

	if err := o.subWorkflowCmds(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to workflow commands: %w", err)
	}

	return nil
}

func (o *Orchestrator) Stop(ctx context.Context) error {
	logger.Debug("Shutting down Orchestrator")
	if err := o.natsClient.CloseWithContext(ctx); err != nil {
		return fmt.Errorf("failed to close NATS client: %w", err)
	}

	if err := o.stManager.CloseWithContext(ctx); err != nil {
		return fmt.Errorf("failed to stop state manager: %w", err)
	}

	logger.Debug("Orchestrator stopped successfully")
	return nil
}

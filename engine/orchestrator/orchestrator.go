package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/compozy/compozy/engine/domain/project"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/stmanager"
	"github.com/compozy/compozy/engine/store"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
)

type SystemComponent string

const (
	ComponentOrchestrator     SystemComponent = "engine.Orchestrator"
	ComponentWorkflowExecutor SystemComponent = "workflow.Executor"
	ComponentTaskExecutor     SystemComponent = "task.Executor"
)

type Orchestrator struct {
	natsClient    *nats.Client
	stManager     *stmanager.Manager
	ProjectConfig *project.Config
	Workflows     []*workflow.Config
}

func NewOrchestartor(
	ctx context.Context,
	natsServer *nats.Server,
	pjConfig *project.Config,
	wfConfigs []*workflow.Config,
) (*Orchestrator, error) {
	natsClient, err := nats.NewClient(natsServer.Conn)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize NATS client: %w", err)
	}
	if err := natsClient.Setup(ctx); err != nil {
		return nil, fmt.Errorf("failed to setup NATS client: %w", err)
	}

	dataDir := filepath.Join(pjConfig.GetCWD().PathStr(), "/.compozy/data")
	store, err := store.NewStore(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create state store: %w", err)
	}

	stManager, err := stmanager.NewManager(
		stmanager.WithNatsClient(natsClient),
		stmanager.WithStore(store),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize state manager: %w", err)
	}

	return &Orchestrator{
		natsClient:    natsClient,
		stManager:     stManager,
		ProjectConfig: pjConfig,
		Workflows:     wfConfigs,
	}, nil
}

func (o *Orchestrator) Start(ctx context.Context) error {
	if err := o.stManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start state manager: %w", err)
	}

	if err := o.subscribeWorkflow(ctx); err != nil {
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

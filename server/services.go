package server

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/project"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/orchestrator"
	"github.com/compozy/compozy/engine/store"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
)

func setupNatsServer() (*nats.Server, error) {
	opts := nats.DefaultServerOptions()
	opts.EnableJetStream = true
	natsServer, err := nats.NewNatsServer(opts)
	if err != nil {
		logger.Error("Failed to setup NATS server", "error", err)
		return nil, err
	}
	return natsServer, nil
}

func loadProject(cwd string, file string) (*project.Config, []*workflow.Config, error) {
	pCWD, err := common.CWDFromPath(cwd)
	if err != nil {
		return nil, nil, err
	}
	logger.Info("Starting compozy server")
	logger.Debug("Loading config file", "config_file", file)

	projectConfig, err := project.Load(pCWD, file)
	if err != nil {
		logger.Error("Failed to load project config", "error", err)
		return nil, nil, err
	}

	if err := projectConfig.Validate(); err != nil {
		logger.Error("Invalid project config", "error", err)
		return nil, nil, err
	}

	// Load wfs from sources
	wfs, err := projectConfig.WorkflowsFromSources()
	if err != nil {
		logger.Error("Failed to load workflows", "error", err)
		return nil, nil, err
	}

	return projectConfig, wfs, nil
}

func getServices(
	ctx context.Context,
	ns *nats.Server,
	pjc *project.Config,
	wfs []*workflow.Config,
) (*orchestrator.Orchestrator, *store.Store, error) {
	// Load store
	dataDir := filepath.Join(pjc.GetCWD().PathStr(), "/.compozy/data")
	store, err := store.NewStore(dataDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create state store: %w", err)
	}

	// Load orchestrator
	orch, err := orchestrator.NewOrchestrator(ctx, ns, store, pjc, wfs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create orchestrator: %w", err)
	}
	return orch, store, nil
}

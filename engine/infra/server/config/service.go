package config

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
)

// Service defines the contract for configuration loading and processing
type Service interface {
	LoadProject(
		ctx context.Context,
		cwd string,
		file string,
	) (*project.Config, []*workflow.Config, *autoload.ConfigRegistry, error)
}

// service is the concrete implementation of the Service interface
type service struct {
	envFilePath string
	store       resources.ResourceStore
}

// NewService creates and initializes a new config service
func NewService(envFilePath string, store resources.ResourceStore) Service {
	return &service{envFilePath: envFilePath, store: store}
}

// LoadProject loads a project configuration and handles AutoLoad integration
func (s *service) LoadProject(
	ctx context.Context,
	cwd string,
	file string,
) (*project.Config, []*workflow.Config, *autoload.ConfigRegistry, error) {
	log := logger.FromContext(ctx)
	pCWD, err := core.CWDFromPath(cwd)
	if err != nil {
		return nil, nil, nil, err
	}
	log.Info("Starting compozy server")
	log.Debug("Loading config file", "config_file", file)

	projectConfig, err := project.Load(ctx, pCWD, file, s.envFilePath)
	if err != nil {
		log.Error("Failed to load project config", "error", err)
		return nil, nil, nil, err
	}

	if err := projectConfig.Validate(); err != nil {
		log.Error("Invalid project config", "error", err)
		return nil, nil, nil, err
	}

	// Create shared configuration registry
	configRegistry := autoload.NewConfigRegistry()

	// Run AutoLoad if enabled
	if projectConfig.AutoLoad != nil && projectConfig.AutoLoad.Enabled {
		log.Info("AutoLoad enabled, discovering and loading configurations")
		autoLoader := autoload.New(pCWD.PathStr(), projectConfig.AutoLoad, configRegistry)
		if err := autoLoader.Load(ctx); err != nil {
			log.Error("AutoLoad failed", "error", err)
			return nil, nil, nil, fmt.Errorf("autoload failed: %w", err)
		}
	}

	// Load workflows without directive evaluation (deprecated). Any $-prefixed
	// keys are rejected at parse time; ID-based linking occurs in compile phase.
	workflows, err := workflow.WorkflowsFromProject(projectConfig)
	if err != nil {
		log.Error("Failed to load workflows", "error", err)
		return nil, nil, nil, err
	}

	// Validate webhook slugs
	slugs := workflow.SlugsFromList(workflows)
	if err := project.NewWebhookSlugsValidator(slugs).Validate(); err != nil {
		log.Error("Invalid webhook configuration", "error", err)
		return nil, nil, nil, fmt.Errorf("webhook configuration invalid: %w", err)
	}

	compiled, err := s.indexAndCompile(ctx, projectConfig, workflows, configRegistry)
	if err != nil {
		return nil, nil, nil, err
	}
	return projectConfig, compiled, configRegistry, nil
}

// indexAndCompile builds an in-memory resource store, indexes discovered
// resources, and compiles workflows against the store.
func (s *service) indexAndCompile(
	ctx context.Context,
	projectConfig *project.Config,
	workflows []*workflow.Config,
	configRegistry *autoload.ConfigRegistry,
) ([]*workflow.Config, error) {
	log := logger.FromContext(ctx)
	if s.store == nil {
		return nil, fmt.Errorf("resource store not provided")
	}
	store := s.store
	if err := projectConfig.IndexToResourceStore(ctx, store); err != nil {
		log.Error("Failed to index project resources", "error", err)
		return nil, err
	}
	for _, wf := range workflows {
		if wf == nil {
			continue
		}
		if err := wf.IndexToResourceStore(ctx, projectConfig.Name, store); err != nil {
			log.Error("Failed to index workflow resources", "workflow_id", wf.ID, "error", err)
			return nil, err
		}
	}
	if configRegistry != nil {
		if err := configRegistry.SyncToResourceStore(ctx, projectConfig.Name, store); err != nil {
			log.Error("Failed to publish autoload resources to store", "error", err)
			return nil, err
		}
	}
	compiled := make([]*workflow.Config, 0, len(workflows))
	for _, wf := range workflows {
		if wf == nil {
			continue
		}
		c, err := wf.Compile(ctx, projectConfig, store)
		if err != nil {
			log.Error("Workflow compile failed", "workflow_id", wf.ID, "error", err)
			return nil, err
		}
		compiled = append(compiled, c)
	}
	return compiled, nil
}

package config

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/ref"
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
	log         logger.Logger
	envFilePath string
}

// NewService creates and initializes a new config service
func NewService(log logger.Logger, envFilePath string) Service {
	return &service{log: log, envFilePath: envFilePath}
}

// LoadProject loads a project configuration and handles AutoLoad integration
func (s *service) LoadProject(
	ctx context.Context,
	cwd string,
	file string,
) (*project.Config, []*workflow.Config, *autoload.ConfigRegistry, error) {
	pCWD, err := core.CWDFromPath(cwd)
	if err != nil {
		return nil, nil, nil, err
	}
	s.log.Info("Starting compozy server")
	s.log.Debug("Loading config file", "config_file", file)

	projectConfig, err := project.Load(ctx, pCWD, file, s.envFilePath)
	if err != nil {
		s.log.Error("Failed to load project config", "error", err)
		return nil, nil, nil, err
	}

	if err := projectConfig.Validate(); err != nil {
		s.log.Error("Invalid project config", "error", err)
		return nil, nil, nil, err
	}

	// Create shared configuration registry
	configRegistry := autoload.NewConfigRegistry()

	// Run AutoLoad if enabled
	if projectConfig.AutoLoad != nil && projectConfig.AutoLoad.Enabled {
		s.log.Info("AutoLoad enabled, discovering and loading configurations")
		autoLoader := autoload.New(pCWD.PathStr(), projectConfig.AutoLoad, configRegistry)
		if err := autoLoader.Load(ctx); err != nil {
			s.log.Error("AutoLoad failed", "error", err)
			return nil, nil, nil, fmt.Errorf("autoload failed: %w", err)
		}
	}

	globalScope, err := projectConfig.AsMap()
	if err != nil {
		s.log.Error("Failed to convert project config to map", "error", err)
		return nil, nil, nil, err
	}

	// Load workflows from sources with registry-aware evaluator
	var evaluatorOptions []ref.EvalConfigOption
	evaluatorOptions = append(evaluatorOptions,
		ref.WithGlobalScope(globalScope),
		ref.WithCacheEnabled(),
	)

	// Add resource resolver for auto-loaded configurations
	if projectConfig.AutoLoad != nil && projectConfig.AutoLoad.Enabled {
		resolver := &autoloadResourceResolver{registry: configRegistry}
		evaluatorOptions = append(evaluatorOptions, ref.WithResourceResolver(resolver))
	}

	ev := ref.NewEvaluator(evaluatorOptions...)

	workflows, err := workflow.WorkflowsFromProject(projectConfig, ev)
	if err != nil {
		s.log.Error("Failed to load workflows", "error", err)
		return nil, nil, nil, err
	}

	return projectConfig, workflows, configRegistry, nil
}

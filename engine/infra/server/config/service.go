package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	modeRepo    = "repo"
	modeBuilder = "builder"
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

// NewServiceWithDefaults creates a Service using the default in-memory
// ResourceStore. Prefer passing an explicit store to NewService when possible.
// This helper eases migration for code that previously did not provide a store.
func NewServiceWithDefaults(envFilePath string) Service {
	return NewService(envFilePath, resources.NewMemoryResourceStore())
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
	projectConfig, configRegistry, err := s.loadProjectArtifacts(ctx, pCWD, file)
	if err != nil {
		return nil, nil, nil, err
	}
	mode, err := s.resolveMode(ctx, projectConfig)
	if err != nil {
		return nil, nil, nil, err
	}
	workflows, err := s.compileByMode(ctx, projectConfig, configRegistry, mode)
	if err != nil {
		return nil, nil, nil, err
	}
	return projectConfig, workflows, configRegistry, nil
}

// loadProjectArtifacts loads, validates, and optionally autoloads project assets.
func (s *service) loadProjectArtifacts(
	ctx context.Context,
	cwd *core.PathCWD,
	file string,
) (*project.Config, *autoload.ConfigRegistry, error) {
	log := logger.FromContext(ctx)
	log.Info("Loading project configuration")
	log.Debug("Loading config file", "config_file", file)
	projectConfig, err := project.Load(ctx, cwd, file, s.envFilePath)
	if err != nil {
		log.Error("Failed to load project config", "error", err)
		return nil, nil, err
	}
	if err := projectConfig.Validate(ctx); err != nil {
		log.Error("Invalid project config", "error", err)
		return nil, nil, err
	}
	configRegistry := autoload.NewConfigRegistry()
	if err := s.runAutoLoad(ctx, cwd, projectConfig, configRegistry); err != nil {
		return nil, nil, err
	}
	return projectConfig, configRegistry, nil
}

// runAutoLoad executes the autoload pipeline when enabled.
func (s *service) runAutoLoad(
	ctx context.Context,
	cwd *core.PathCWD,
	projectConfig *project.Config,
	configRegistry *autoload.ConfigRegistry,
) error {
	if projectConfig.AutoLoad == nil || !projectConfig.AutoLoad.Enabled {
		return nil
	}
	log := logger.FromContext(ctx)
	log.Info("AutoLoad enabled, discovering and loading configurations")
	autoLoader := autoload.New(cwd.PathStr(), projectConfig.AutoLoad, configRegistry)
	if err := autoLoader.Load(ctx); err != nil {
		log.Error("AutoLoad failed", "error", err)
		return fmt.Errorf("autoload failed: %w", err)
	}
	return nil
}

// compileByMode compiles workflows according to the configured mode.
func (s *service) compileByMode(
	ctx context.Context,
	projectConfig *project.Config,
	configRegistry *autoload.ConfigRegistry,
	mode string,
) ([]*workflow.Config, error) {
	switch mode {
	case modeBuilder:
		return s.compileFromStore(ctx, projectConfig, configRegistry)
	default:
		return s.loadFromRepo(ctx, projectConfig, configRegistry)
	}
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
		return nil, fmt.Errorf("indexAndCompile: resource store not provided")
	}
	if err := s.indexProjectAndWorkflows(ctx, projectConfig, workflows, configRegistry); err != nil {
		log.Error("Failed to index resources", "error", err)
		return nil, fmt.Errorf("indexAndCompile: project=%s: %w", projectConfig.Name, err)
	}
	compiled := make([]*workflow.Config, 0, len(workflows))
	for _, wf := range workflows {
		if wf == nil {
			continue
		}
		c, err := wf.Compile(ctx, projectConfig, s.store)
		if err != nil {
			log.Error("Workflow compile failed", "workflow_id", wf.ID, "error", err)
			return nil, fmt.Errorf("indexAndCompile: project=%s workflow=%s: %w", projectConfig.Name, wf.ID, err)
		}
		compiled = append(compiled, c)
	}
	return compiled, nil
}

// indexProjectAndWorkflows indexes the project, workflows, and autoload registry into the store.
func (s *service) indexProjectAndWorkflows(
	ctx context.Context,
	projectConfig *project.Config,
	wfs []*workflow.Config,
	reg *autoload.ConfigRegistry,
) error {
	if err := projectConfig.IndexToResourceStore(ctx, s.store); err != nil {
		return err
	}
	for _, wf := range wfs {
		if wf == nil {
			continue
		}
		if err := wf.IndexToResourceStore(ctx, projectConfig.Name, s.store); err != nil {
			return err
		}
	}
	if reg != nil {
		if err := reg.SyncToResourceStore(ctx, projectConfig.Name, s.store); err != nil {
			return err
		}
	}
	return nil
}

// compileFromStore compiles workflows by enumerating them from the ResourceStore.
// If the store is empty and seeding is enabled, it seeds once from repo YAML.
func (s *service) compileFromStore(
	ctx context.Context,
	projectConfig *project.Config,
	configRegistry *autoload.ConfigRegistry,
) ([]*workflow.Config, error) {
	if s.store == nil {
		return nil, fmt.Errorf("resource store not provided")
	}
	if err := s.publishAutoloadResources(ctx, projectConfig, configRegistry); err != nil {
		return nil, err
	}
	keys, skip, err := s.workflowKeysFromStore(ctx, projectConfig, configRegistry)
	if err != nil {
		return nil, err
	}
	if skip {
		return []*workflow.Config{}, nil
	}
	decoded, err := s.decodeAllWorkflows(ctx, keys)
	if err != nil {
		return nil, err
	}
	slugs := workflow.SlugsFromList(decoded)
	if err := project.NewWebhookSlugsValidator(slugs).Validate(ctx); err != nil {
		return nil, fmt.Errorf("webhook configuration invalid: %w", err)
	}
	return s.compileDecodedWorkflows(ctx, projectConfig, decoded)
}

// publishAutoloadResources syncs autoload data into the resource store.
func (s *service) publishAutoloadResources(
	ctx context.Context,
	projectConfig *project.Config,
	configRegistry *autoload.ConfigRegistry,
) error {
	if configRegistry == nil {
		return nil
	}
	if err := configRegistry.SyncToResourceStore(ctx, projectConfig.Name, s.store); err != nil {
		return fmt.Errorf("publish autoload resources failed: %w", err)
	}
	return nil
}

// workflowKeysFromStore lists workflow keys and seeds the store when required.
func (s *service) workflowKeysFromStore(
	ctx context.Context,
	projectConfig *project.Config,
	configRegistry *autoload.ConfigRegistry,
) ([]resources.ResourceKey, bool, error) {
	log := logger.FromContext(ctx)
	keys, err := s.store.List(ctx, projectConfig.Name, resources.ResourceWorkflow)
	if err != nil {
		return nil, false, fmt.Errorf("list workflows from store failed (project=%s): %w", projectConfig.Name, err)
	}
	if len(keys) > 0 {
		return keys, false, nil
	}
	if !s.shouldSeedFromRepo(ctx, projectConfig) {
		log.Info("Store empty; skipping YAML seed (disabled)")
		return nil, true, nil
	}
	log.Info("Store empty; seeding from repo YAML")
	if err := s.seedFromRepo(ctx, projectConfig, configRegistry); err != nil {
		return nil, false, err
	}
	relisted, err := s.store.List(ctx, projectConfig.Name, resources.ResourceWorkflow)
	if err != nil {
		return nil, false, fmt.Errorf("list after seed failed (project=%s): %w", projectConfig.Name, err)
	}
	return relisted, false, nil
}

// compileDecodedWorkflows compiles decoded workflow configurations.
func (s *service) compileDecodedWorkflows(
	ctx context.Context,
	projectConfig *project.Config,
	decoded []*workflow.Config,
) ([]*workflow.Config, error) {
	compiled := make([]*workflow.Config, 0, len(decoded))
	for _, wf := range decoded {
		cwf, err := wf.Compile(ctx, projectConfig, s.store)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, cwf)
	}
	return compiled, nil
}

// shouldSeedFromRepo controls one-time seeding behavior for builder mode.
// Default is disabled to avoid surprising mutations.
// Docs: see /docs/core/configuration/server#seed-from-repo-on-empty
func (s *service) shouldSeedFromRepo(ctx context.Context, _ *project.Config) bool {
	if c := config.FromContext(ctx); c != nil {
		return c.Server.SourceOfTruth == "builder" && c.Server.SeedFromRepoOnEmpty
	}
	return false
}

// seedFromRepo indexes repo YAML content into the ResourceStore once
func (s *service) seedFromRepo(
	ctx context.Context,
	projectConfig *project.Config,
	configRegistry *autoload.ConfigRegistry,
) error {
	workflows, err := workflow.WorkflowsFromProject(ctx, projectConfig)
	if err != nil {
		return fmt.Errorf("seed load workflows failed: %w", err)
	}
	if err := projectConfig.IndexToResourceStore(ctx, s.store); err != nil {
		return fmt.Errorf("seed index project failed: %w", err)
	}
	for _, wf := range workflows {
		if wf == nil {
			continue
		}
		if err := wf.IndexToResourceStore(ctx, projectConfig.Name, s.store); err != nil {
			return fmt.Errorf("seed index workflow '%s' failed: %w", wf.ID, err)
		}
	}
	if configRegistry != nil {
		if err := configRegistry.SyncToResourceStore(ctx, projectConfig.Name, s.store); err != nil {
			return fmt.Errorf("seed publish autoload failed: %w", err)
		}
	}
	return nil
}

// normalizeMode lower-cases and validates the source_of_truth value.
func (s *service) normalizeMode(raw string) (string, error) {
	m := strings.ToLower(strings.TrimSpace(raw))
	if m == "" {
		return modeRepo, nil
	}
	if m != modeRepo && m != modeBuilder {
		return "", fmt.Errorf("invalid source_of_truth: %s (allowed: repo|builder)", raw)
	}
	return m, nil
}

// resolveMode applies server default, project override, logs and records a metric
func (s *service) resolveMode(ctx context.Context, projectConfig *project.Config) (string, error) {
	log := logger.FromContext(ctx)
	mode := modeRepo
	if c := config.FromContext(ctx); c != nil && c.Server.SourceOfTruth != "" {
		var err error
		mode, err = s.normalizeMode(c.Server.SourceOfTruth)
		if err != nil {
			return "", err
		}
	}
	if projectConfig.Opts.SourceOfTruth != "" {
		var err error
		mode, err = s.normalizeMode(projectConfig.Opts.SourceOfTruth)
		if err != nil {
			return "", err
		}
	}
	log.Info("Resolved source of truth mode", "mode", mode)
	if meter := otel.GetMeterProvider().Meter("compozy"); meter != nil {
		sel, err := meter.Int64Counter(
			monitoringmetrics.MetricNameWithSubsystem("mode", "selected_total"),
			metric.WithDescription("Count of server mode selections at startup"),
		)
		if err == nil {
			sel.Add(ctx, 1, metric.WithAttributes(attribute.String("mode", mode)))
		}
	}
	return mode, nil
}

// decodeAllWorkflows loads and decodes workflow configs from store keys
func (s *service) decodeAllWorkflows(ctx context.Context, keys []resources.ResourceKey) ([]*workflow.Config, error) {
	out := make([]*workflow.Config, 0, len(keys))
	for _, k := range keys {
		v, _, err := s.store.Get(ctx, k)
		if err != nil {
			return nil, fmt.Errorf("get workflow '%s' failed: %w", k.ID, err)
		}
		switch tv := v.(type) {
		case *workflow.Config:
			out = append(out, tv)
		case workflow.Config:
			wf := tv
			out = append(out, &wf)
		case map[string]any:
			var tmp workflow.Config
			if err := tmp.FromMap(tv); err != nil {
				return nil, fmt.Errorf("decode workflow '%s' failed: %w", k.ID, err)
			}
			out = append(out, &tmp)
		default:
			return nil, fmt.Errorf("unsupported workflow value type %T for key %s", tv, k.ID)
		}
	}
	return out, nil
}

// loadFromRepo loads workflows from YAML and compiles after indexing
func (s *service) loadFromRepo(
	ctx context.Context,
	projectConfig *project.Config,
	configRegistry *autoload.ConfigRegistry,
) ([]*workflow.Config, error) {
	log := logger.FromContext(ctx)
	workflows, err := workflow.WorkflowsFromProject(ctx, projectConfig)
	if err != nil {
		log.Error("Failed to load workflows", "error", err)
		return nil, err
	}
	slugs := workflow.SlugsFromList(workflows)
	if err := project.NewWebhookSlugsValidator(slugs).Validate(ctx); err != nil {
		log.Error("Invalid webhook configuration", "error", err)
		return nil, fmt.Errorf("webhook configuration invalid: %w", err)
	}
	compiled, err := s.indexAndCompile(ctx, projectConfig, workflows, configRegistry)
	if err != nil {
		return nil, err
	}
	return compiled, nil
}

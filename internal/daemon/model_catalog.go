package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/modelcatalog"
)

const (
	defaultModelCatalogRefreshTimeout = 10 * time.Second
	modelCatalogRefreshCleanupGrace   = 10 * time.Second
)

type modelCatalogRuntime struct {
	service            modelcatalog.Service
	logger             *slog.Logger
	now                func() time.Time
	timeout            time.Duration
	configSource       *modelcatalog.ProviderConfigSource
	liveSources        map[string]*modelcatalog.LiveProviderSource
	dynamicSources     map[string]modelcatalog.Source
	executionContextMu sync.RWMutex
	executionContexts  map[string]modelcatalog.CatalogExecutionContext
	// generationGate keeps source snapshots and persisted rows in one public generation.
	generationGate lifecycleRWGate

	ctx            context.Context
	workers        *ownedWorkerGroup
	refreshCancels modelCatalogRefreshCancels

	refreshLoopOnce  sync.Once
	newRefreshTicker func(time.Duration) modelCatalogRefreshTicker

	backgroundRefreshMu      sync.Mutex
	backgroundRefreshRunning bool
}

var _ modelcatalog.Service = (*modelCatalogRuntime)(nil)

type modelCatalogRefreshResult struct {
	statuses []modelcatalog.SourceStatus
	err      error
}

type modelCatalogDefaultExecutionContextSetter interface {
	SetDefaultExecutionContext(modelcatalog.CatalogExecutionContext) error
}

func newModelCatalogRuntime(
	ctx context.Context,
	service modelcatalog.Service,
	logger *slog.Logger,
	now func() time.Time,
	timeout time.Duration,
) (*modelCatalogRuntime, error) {
	if ctx == nil {
		return nil, errors.New("daemon: model catalog lifecycle context is required")
	}
	if service == nil {
		return nil, errors.New("daemon: model catalog service is required")
	}
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	if timeout <= 0 {
		timeout = defaultModelCatalogRefreshTimeout
	}
	// #nosec G118 -- cancel is owned by modelCatalogRuntime and invoked from Shutdown.
	runtimeCtx, cancel := context.WithCancel(ctx)
	runtime := &modelCatalogRuntime{
		service:           service,
		logger:            logger,
		now:               now,
		timeout:           timeout,
		ctx:               runtimeCtx,
		workers:           newOwnedWorkerGroup(cancel),
		newRefreshTicker:  newRealModelCatalogRefreshTicker,
		executionContexts: make(map[string]modelcatalog.CatalogExecutionContext),
	}
	defaultExecutionContext, err := runtime.resolveCatalogExecutionContext(modelcatalog.CatalogExecutionContext{})
	if err != nil {
		cancel()
		return nil, err
	}
	if setter, ok := service.(modelCatalogDefaultExecutionContextSetter); ok {
		if err := setter.SetDefaultExecutionContext(defaultExecutionContext); err != nil {
			cancel()
			return nil, fmt.Errorf("daemon: configure model catalog default execution context: %w", err)
		}
	}
	return runtime, nil
}

func (r *modelCatalogRuntime) ListModels(
	ctx context.Context,
	opts modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	if r == nil || r.service == nil {
		return nil, errors.New("daemon: model catalog service is unavailable")
	}
	if ctx == nil {
		return nil, errors.New("daemon: model catalog list context is required")
	}
	executionContext, err := r.resolveCatalogExecutionContext(opts.ExecutionContext)
	if err != nil {
		return nil, err
	}
	opts.ExecutionContext = executionContext
	now := r.now().UTC()
	if !opts.Now.IsZero() {
		now = opts.Now
	}
	if opts.Refresh {
		refreshOpts := modelcatalog.RefreshOptions{
			ProviderID:       opts.ProviderID,
			SourceID:         opts.SourceID,
			ExecutionContext: opts.ExecutionContext,
			Force:            true,
			Now:              now,
		}
		_, refreshErr := r.Refresh(ctx, refreshOpts)
		listOpts := opts
		listOpts.Refresh = false
		listOpts.SkipRefreshIfEmpty = true
		listOpts.Now = now
		models, listErr := r.listModelsInGeneration(ctx, listOpts)
		if listErr != nil {
			return nil, listErr
		}
		if len(models) == 0 && refreshErr != nil {
			return nil, refreshErr
		}
		r.refreshDynamicSourcesInBackground()
		return models, nil
	}
	opts.Now = now
	opts.SkipRefreshIfEmpty = true
	models, err := r.listModelsInGeneration(ctx, opts)
	if err != nil {
		return nil, err
	}
	r.refreshDynamicSourcesInBackground()
	return models, nil
}

func (r *modelCatalogRuntime) listModelsInGeneration(
	ctx context.Context,
	opts modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	complete, admitted := r.workers.Begin()
	if !admitted {
		return nil, fmt.Errorf("daemon: model catalog list stopped: %w", r.ctx.Err())
	}
	defer complete()

	listCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopRootCancel := context.AfterFunc(r.ctx, cancel)
	defer stopRootCancel()

	release, err := r.generationGate.lock(listCtx, true)
	if err != nil {
		return nil, fmt.Errorf("daemon: wait to list model catalog generation: %w", err)
	}
	defer release()
	return r.service.ListModels(listCtx, opts)
}

func (r *modelCatalogRuntime) Refresh(
	ctx context.Context,
	opts modelcatalog.RefreshOptions,
) ([]modelcatalog.SourceStatus, error) {
	if r == nil || r.service == nil {
		return nil, errors.New("daemon: model catalog service is unavailable")
	}
	if ctx == nil {
		return nil, errors.New("daemon: model catalog refresh context is required")
	}
	executionContext, err := r.resolveCatalogExecutionContext(opts.ExecutionContext)
	if err != nil {
		return nil, err
	}
	opts.ExecutionContext = executionContext
	release, err := r.generationGate.lock(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("daemon: wait to refresh model catalog generation: %w", err)
	}
	return r.refresh(ctx, opts, release, true)
}

func (r *modelCatalogRuntime) refresh(
	ctx context.Context,
	opts modelcatalog.RefreshOptions,
	releaseGeneration func(),
	detachOnRequestCancel bool,
) ([]modelcatalog.SourceStatus, error) {
	if err := r.ctx.Err(); err != nil {
		if releaseGeneration != nil {
			releaseGeneration()
		}
		return nil, fmt.Errorf("daemon: model catalog refresh unavailable: %w", err)
	}
	runtimeNow := r.now().UTC()
	now := runtimeNow
	if !opts.Now.IsZero() {
		now = opts.Now
	}
	refreshOpts := opts
	refreshOpts.Now = now
	if strings.TrimSpace(refreshOpts.RequestID) == "" {
		refreshOpts.RequestID = fmt.Sprintf("model-catalog-refresh-%d", runtimeNow.UnixNano())
	}

	refreshCtx := context.WithoutCancel(ctx)
	refreshCtx, cancel := context.WithTimeout(refreshCtx, r.timeout)
	resultCh := make(chan modelCatalogRefreshResult, 1)
	complete, admitted := r.workers.Begin()
	if !admitted {
		cancel()
		if releaseGeneration != nil {
			releaseGeneration()
		}
		return nil, fmt.Errorf("daemon: model catalog refresh stopped: %w", r.ctx.Err())
	}
	unregisterRefreshCancel := r.refreshCancels.register(cancel)

	go func(release func()) {
		defer complete()
		defer unregisterRefreshCancel()
		if release != nil {
			defer release()
		}
		stopRootCancel := context.AfterFunc(r.ctx, cancel)
		defer func() {
			stopRootCancel()
			cancel()
		}()

		statuses, err := r.service.Refresh(refreshCtx, refreshOpts)
		if err != nil {
			r.logRefreshFailure(refreshOpts, err)
		}
		resultCh <- modelCatalogRefreshResult{statuses: statuses, err: err}
	}(releaseGeneration)

	if !detachOnRequestCancel {
		result := <-resultCh
		return result.statuses, result.err
	}
	select {
	case result := <-resultCh:
		return result.statuses, result.err
	case <-ctx.Done():
		return nil, fmt.Errorf("daemon: model catalog refresh request canceled: %w", ctx.Err())
	case <-r.ctx.Done():
		return nil, fmt.Errorf("daemon: model catalog refresh stopped: %w", r.ctx.Err())
	}
}

func (r *modelCatalogRuntime) ListSourceStatus(
	ctx context.Context,
	opts modelcatalog.StatusOptions,
) ([]modelcatalog.SourceStatus, error) {
	if r == nil || r.service == nil {
		return nil, errors.New("daemon: model catalog service is unavailable")
	}
	if ctx == nil {
		return nil, errors.New("daemon: model catalog status context is required")
	}
	executionContext, err := r.resolveCatalogExecutionContext(opts.ExecutionContext)
	if err != nil {
		return nil, err
	}
	opts.ExecutionContext = executionContext
	release, err := r.generationGate.lock(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("daemon: wait to list model catalog status generation: %w", err)
	}
	defer release()
	return r.service.ListSourceStatus(ctx, opts)
}

func (r *modelCatalogRuntime) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: model catalog shutdown context is required")
	}
	done := r.workers.Stop()
	r.refreshCancels.stop()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("daemon: wait for model catalog refresh workers: %w", ctx.Err())
	}
}

func (r *modelCatalogRuntime) logRefreshFailure(opts modelcatalog.RefreshOptions, err error) {
	if r == nil || r.logger == nil || err == nil {
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	r.logger.Warn(
		"daemon.model_catalog.refresh_failed",
		"refresh_request_id",
		opts.RequestID,
		"provider_id",
		strings.TrimSpace(opts.ProviderID),
		"source_id",
		strings.TrimSpace(opts.SourceID),
		"error",
		modelcatalog.RedactString(err.Error()),
	)
}

func (d *Daemon) bootModelCatalog(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if state == nil {
		return errors.New("daemon: model catalog state is required")
	}
	store, ok := state.registry.(modelcatalog.Store)
	if !ok {
		if state.logger != nil {
			state.logger.Warn(
				"daemon.model_catalog.disabled",
				"reason",
				"registry_missing_model_catalog_store",
				"registry_type",
				fmt.Sprintf("%T", state.registry),
			)
		}
		return nil
	}

	sourceTimeout, err := time.ParseDuration(state.cfg.ModelCatalog.Sources.ModelsDev.EffectiveTimeout())
	if err != nil {
		return fmt.Errorf("daemon: parse model catalog source timeout: %w", err)
	}
	httpClient := &http.Client{Timeout: sourceTimeout}
	sources, err := d.modelCatalogSources(state, httpClient, sourceTimeout)
	if err != nil {
		return err
	}
	reasoningApply, err := effectiveCatalogReasoningApply(&state.cfg)
	if err != nil {
		return err
	}
	service, err := modelcatalog.NewService(store, sources, modelcatalog.MergeOptions{
		ReasoningApply: reasoningApply,
	})
	if err != nil {
		return fmt.Errorf("daemon: create model catalog service: %w", err)
	}
	if err := rehydrateStaticModelCatalogSources(ctx, service, d.now); err != nil {
		return err
	}
	runtime, err := newModelCatalogRuntime(
		ctx,
		service,
		state.logger,
		d.now,
		sourceTimeout+modelCatalogRefreshCleanupGrace,
	)
	if err != nil {
		return err
	}
	for _, source := range sources {
		if ttlSource, ok := source.(interface{ TTL() time.Duration }); ok && ttlSource.TTL() > 0 {
			if runtime.dynamicSources == nil {
				runtime.dynamicSources = make(map[string]modelcatalog.Source)
			}
			runtime.dynamicSources[source.ID()] = source
		}
		switch typed := source.(type) {
		case *modelcatalog.ProviderConfigSource:
			if typed.ID() == modelcatalog.SourceIDConfig {
				runtime.configSource = typed
			}
		case *modelcatalog.LiveProviderSource:
			if runtime.liveSources == nil {
				runtime.liveSources = make(map[string]*modelcatalog.LiveProviderSource)
			}
			runtime.liveSources[typed.ProviderIDs()[0]] = typed
		}
	}
	state.modelCatalog = runtime
	runtime.startDynamicRefreshLoop()
	if cleanup != nil {
		cleanup.add(runtime.Shutdown)
	}
	return nil
}

func (d *Daemon) modelCatalogSources(
	state *bootState,
	httpClient *http.Client,
	defaultTimeout time.Duration,
) ([]modelcatalog.Source, error) {
	modelsDev, err := modelcatalog.NewModelsDevSource(
		state.cfg.Providers,
		state.cfg.ModelCatalog.Sources.ModelsDev,
		modelcatalog.WithModelsDevHTTPClient(httpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: create models.dev model catalog source: %w", err)
	}

	sources := []modelcatalog.Source{
		modelcatalog.NewBuiltinSource(),
		modelcatalog.NewConfigSource(state.cfg.Providers),
		modelsDev,
	}
	liveSources, err := modelcatalog.NewLiveProviderSources(&modelcatalog.LiveProviderSourcesConfig{
		Providers:      state.cfg.Providers,
		HomePaths:      d.homePaths,
		BaseEnv:        os.Environ(),
		SecretResolver: d.modelCatalogSecretResolver(state),
		HTTPClient:     httpClient,
		DefaultTimeout: defaultTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("daemon: create live provider model catalog sources: %w", err)
	}
	sources = append(sources, liveSources...)
	extensionSources, err := d.modelCatalogExtensionSources(state)
	if err != nil {
		return nil, err
	}
	sources = append(sources, extensionSources...)
	return sources, nil
}

func (d *Daemon) modelCatalogExtensionSources(state *bootState) ([]modelcatalog.Source, error) {
	dbSource, ok := state.registry.(extensionDBSource)
	if !ok || dbSource.DB() == nil {
		return nil, nil
	}
	registry := extensionpkg.NewRegistry(dbSource.DB())
	sources, err := extensionpkg.NewExtensionModelSources(registry, func() extensionpkg.ModelSourceRuntime {
		runtime, ok := state.currentExtensionRuntime().(extensionpkg.ModelSourceRuntime)
		if !ok {
			return nil
		}
		return runtime
	})
	if err != nil {
		return nil, fmt.Errorf("daemon: create extension model catalog sources: %w", err)
	}
	return sources, nil
}

func (d *Daemon) modelCatalogSecretResolver(state *bootState) modelcatalog.ProviderSecretResolver {
	if state != nil && state.providerVault != nil {
		return state.providerVault
	}
	return modelcatalog.EnvSecretResolver{
		LookupEnv: func(key string) (string, bool) {
			value := ""
			if d.getenv != nil {
				value = d.getenv(key)
			} else {
				value = os.Getenv(key)
			}
			return value, strings.TrimSpace(value) != ""
		},
	}
}

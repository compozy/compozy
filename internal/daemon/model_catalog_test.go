package daemon

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
)

func TestDaemonModelCatalogWiring(t *testing.T) {
	t.Parallel()

	t.Run("Should compose catalog service when global DB and config are available", func(t *testing.T) {
		t.Parallel()

		daemonInstance, httpDeps, udsDeps := bootModelCatalogTestDaemon(t, nil)
		if daemonInstance.modelCatalog == nil {
			t.Fatal("boot() modelCatalog = nil, want daemon-owned service")
		}
		if httpDeps.ModelCatalog == nil {
			t.Fatal("HTTP RuntimeDeps ModelCatalog = nil, want injected service")
		}
		if udsDeps.ModelCatalog == nil {
			t.Fatal("UDS RuntimeDeps ModelCatalog = nil, want injected service")
		}

		ctx := testutil.Context(t)
		models, err := httpDeps.ModelCatalog.ListModels(ctx, modelcatalog.ListOptions{ProviderID: "codex"})
		if err != nil {
			t.Fatalf("ModelCatalog.ListModels(codex) error = %v", err)
		}
		if !containsCatalogModel(models, "codex", "gpt-5.6-sol") {
			t.Fatalf("ModelCatalog.ListModels(codex) missing builtin gpt-5.6-sol row: %#v", models)
		}
	})

	t.Run("Should rehydrate explicit curation before the first client list", func(t *testing.T) {
		t.Parallel()

		_, httpDeps, _ := bootModelCatalogTestDaemonWithSetup(
			t,
			func(cfg *aghconfig.Config) {
				provider := cfg.Providers["codex"]
				provider.Models = aghconfig.ProviderModelsConfig{
					Default: "operator-default-only",
					Curated: []aghconfig.ProviderModelConfig{{ID: "gpt-5.6-sol"}},
				}
				if cfg.Providers == nil {
					cfg.Providers = make(map[string]aghconfig.ProviderConfig)
				}
				cfg.Providers["codex"] = provider
			},
			seedPreExplicitCurationRows,
		)

		ctx := testutil.Context(t)
		curated, err := httpDeps.ModelCatalog.ListModels(ctx, modelcatalog.ListOptions{ProviderID: "codex"})
		if err != nil {
			t.Fatalf("ModelCatalog.ListModels(curated) error = %v", err)
		}
		sol, ok := findCatalogModel(curated, "gpt-5.6-sol")
		if !ok || !sol.ExplicitlyCurated || !sol.Curated {
			t.Fatalf("curated gpt-5.6-sol = %#v, want explicit curated row", sol)
		}
		if containsCatalogModel(curated, "codex", "operator-default-only") {
			t.Fatalf("curated models = %#v, want default-only config row excluded", curated)
		}
		if containsCatalogModel(curated, "codex", "provider-live-only") {
			t.Fatalf("curated models = %#v, want live-only row excluded", curated)
		}

		all, err := httpDeps.ModelCatalog.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID: "codex",
			View:       modelcatalog.CatalogViewAll,
		})
		if err != nil {
			t.Fatalf("ModelCatalog.ListModels(all) error = %v", err)
		}
		defaultOnly, ok := findCatalogModel(all, "operator-default-only")
		if !ok || defaultOnly.ExplicitlyCurated || defaultOnly.Curated {
			t.Fatalf("all default-only model = %#v, want non-explicit non-curated row", defaultOnly)
		}
		if _, ok := findCatalogModel(all, "provider-live-only"); !ok {
			t.Fatalf("all models = %#v, want live-only row preserved", all)
		}
	})

	t.Run("Should record live source status when optional dependency is missing", func(t *testing.T) {
		t.Parallel()

		daemonInstance, _, _ := bootModelCatalogTestDaemon(t, nil)
		ctx := testutil.Context(t)
		_, err := daemonInstance.modelCatalog.Refresh(ctx, modelcatalog.RefreshOptions{
			ProviderID: "hermes",
			SourceID:   modelcatalog.SourceKindProviderLiveID("hermes"),
			Force:      true,
		})
		if !errors.Is(err, modelcatalog.ErrAllSourcesFailed) {
			t.Fatalf("ModelCatalog.Refresh(hermes live) error = %v, want ErrAllSourcesFailed", err)
		}

		statuses, err := daemonInstance.modelCatalog.ListSourceStatus(ctx, "hermes")
		if err != nil {
			t.Fatalf("ModelCatalog.ListSourceStatus(hermes) error = %v", err)
		}
		status, ok := findSourceStatus(statuses, modelcatalog.SourceKindProviderLiveID("hermes"))
		if !ok {
			t.Fatalf("ListSourceStatus(hermes) missing provider_live status: %#v", statuses)
		}
		if got, want := status.RefreshState, modelcatalog.RefreshStateFailed; got != want {
			t.Fatalf("provider_live refresh state = %q, want %q", got, want)
		}
		if status.LastError == "" {
			t.Fatal("provider_live LastError = empty, want redacted failure detail")
		}
	})

	t.Run("Should cancel and join refresh work on shutdown", func(t *testing.T) {
		t.Parallel()

		service := newBlockingModelCatalogService()
		runtime, err := newModelCatalogRuntime(
			testutil.Context(t),
			service,
			discardLogger(),
			func() time.Time {
				return time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
			},
			5*time.Second,
		)
		if err != nil {
			t.Fatalf("newModelCatalogRuntime() error = %v", err)
		}

		requestCtx, cancelRequest := context.WithCancel(testutil.Context(t))
		resultCh := make(chan error, 1)
		go func() {
			_, refreshErr := runtime.Refresh(requestCtx, modelcatalog.RefreshOptions{
				ProviderID: "codex",
				SourceID:   modelcatalog.SourceIDBuiltin,
				Force:      true,
			})
			resultCh <- refreshErr
		}()

		waitForCatalogTestSignal(t, service.started, "refresh start")
		cancelRequest()
		refreshErr := waitForCatalogTestError(t, resultCh, "refresh request cancellation")
		if !errors.Is(refreshErr, context.Canceled) {
			t.Fatalf("Refresh() error = %v, want context.Canceled", refreshErr)
		}
		select {
		case <-service.released:
			t.Fatal("refresh worker stopped on request cancellation; want daemon shutdown to own worker cancellation")
		default:
		}

		shutdownCtx, cancelShutdown := context.WithTimeout(testutil.Context(t), time.Second)
		defer cancelShutdown()
		if err := runtime.Shutdown(shutdownCtx); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		waitForCatalogTestSignal(t, service.released, "refresh release")
	})

	t.Run("Should return shutdown deadline when refresh worker does not stop in time", func(t *testing.T) {
		t.Parallel()

		service := newManuallyReleasedModelCatalogService()
		runtime, err := newModelCatalogRuntime(
			testutil.Context(t),
			service,
			discardLogger(),
			func() time.Time {
				return time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
			},
			5*time.Second,
		)
		if err != nil {
			t.Fatalf("newModelCatalogRuntime() error = %v", err)
		}

		refreshErrCh := make(chan error, 1)
		go func() {
			_, refreshErr := runtime.Refresh(testutil.Context(t), modelcatalog.RefreshOptions{Force: true})
			refreshErrCh <- refreshErr
		}()
		waitForCatalogTestSignal(t, service.started, "manual refresh start")

		shutdownCtx, cancelShutdown := context.WithTimeout(testutil.Context(t), time.Nanosecond)
		defer cancelShutdown()
		err = runtime.Shutdown(shutdownCtx)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Shutdown(deadline) error = %v, want context.DeadlineExceeded", err)
		}
		close(service.release)
		waitForCatalogTestSignal(t, service.released, "manual refresh release")
		refreshErr := waitForCatalogTestError(t, refreshErrCh, "manual refresh shutdown cancellation")
		if !errors.Is(refreshErr, context.Canceled) {
			t.Fatalf("Refresh(shutdown) error = %v, want context.Canceled", refreshErr)
		}
	})

	t.Run("Should apply runtime timeout to detached refresh work", func(t *testing.T) {
		t.Parallel()

		service := newBlockingModelCatalogService()
		runtime, err := newModelCatalogRuntime(
			testutil.Context(t),
			service,
			discardLogger(),
			func() time.Time {
				return time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
			},
			20*time.Millisecond,
		)
		if err != nil {
			t.Fatalf("newModelCatalogRuntime() error = %v", err)
		}

		_, err = runtime.Refresh(testutil.Context(t), modelcatalog.RefreshOptions{
			ProviderID: "codex",
			SourceID:   modelcatalog.SourceIDBuiltin,
			Force:      true,
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Refresh(timeout) error = %v, want context.DeadlineExceeded", err)
		}
		waitForCatalogTestSignal(t, service.released, "timed refresh release")
	})

	t.Run("Should redact source errors in refresh logs", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer
		runtime := &modelCatalogRuntime{
			logger: slog.New(slog.NewTextHandler(&logs, nil)),
		}
		runtime.logRefreshFailure(
			modelcatalog.RefreshOptions{
				ProviderID: "codex",
				SourceID:   modelcatalog.SourceIDModelsDev,
				RequestID:  "req-redaction",
			},
			errors.New("source failed with api_key=sk-super-secret-token-123"),
		)
		output := logs.String()
		if strings.Contains(output, "sk-super-secret-token-123") {
			t.Fatalf("log output = %q, want secret redacted", output)
		}
		if !strings.Contains(output, "[REDACTED]") {
			t.Fatalf("log output = %q, want redaction marker", output)
		}
	})

	t.Run("Should refresh before listing when list requests refresh", func(t *testing.T) {
		t.Parallel()

		service := &recordingModelCatalogService{
			models: []modelcatalog.Model{{ProviderID: "codex", ModelID: "gpt-5.4"}},
		}
		runtime, err := newModelCatalogRuntime(
			testutil.Context(t),
			service,
			discardLogger(),
			func() time.Time {
				return time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
			},
			5*time.Second,
		)
		if err != nil {
			t.Fatalf("newModelCatalogRuntime() error = %v", err)
		}

		models, err := runtime.ListModels(testutil.Context(t), modelcatalog.ListOptions{
			ProviderID: "codex",
			Refresh:    true,
		})
		if err != nil {
			t.Fatalf("ListModels(refresh) error = %v", err)
		}
		if !containsCatalogModel(models, "codex", "gpt-5.4") {
			t.Fatalf("ListModels(refresh) = %#v, want gpt-5.4", models)
		}
		if service.refreshCalls != 1 {
			t.Fatalf("Refresh calls = %d, want 1", service.refreshCalls)
		}
		if !service.lastRefresh.Force || service.lastRefresh.ProviderID != "codex" {
			t.Fatalf("Refresh opts = %#v, want forced codex refresh", service.lastRefresh)
		}
		if service.lastList.Refresh {
			t.Fatalf("List opts Refresh = true, want false after daemon refresh handoff")
		}
		if _, err := runtime.ListSourceStatus(testutil.Context(t), "codex"); err != nil {
			t.Fatalf("ListSourceStatus() error = %v", err)
		}
	})

	t.Run("Should validate runtime dependencies", func(t *testing.T) {
		t.Parallel()

		if _, err := newModelCatalogRuntime(testutil.Context(t), nil, nil, nil, 0); err == nil {
			t.Fatal("newModelCatalogRuntime(nil service) error = nil, want validation error")
		}
		runtime, err := newModelCatalogRuntime(
			testutil.Context(t),
			&recordingModelCatalogService{},
			nil,
			nil,
			0,
		)
		if err != nil {
			t.Fatalf("newModelCatalogRuntime(defaults) error = %v", err)
		}
		if runtime.timeout != defaultModelCatalogRefreshTimeout {
			t.Fatalf("runtime timeout = %s, want %s", runtime.timeout, defaultModelCatalogRefreshTimeout)
		}
		if err := runtime.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown(context.Background()) error = %v", err)
		}
		var nilRuntime *modelCatalogRuntime
		if err := nilRuntime.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown(nil runtime) error = %v", err)
		}
		unavailable := &modelCatalogRuntime{}
		if _, err := unavailable.ListModels(testutil.Context(t), modelcatalog.ListOptions{}); err == nil {
			t.Fatal("ListModels(unavailable) error = nil, want validation error")
		}
		if _, err := unavailable.Refresh(testutil.Context(t), modelcatalog.RefreshOptions{}); err == nil {
			t.Fatal("Refresh(unavailable) error = nil, want validation error")
		}
		if _, err := unavailable.ListSourceStatus(testutil.Context(t), "codex"); err == nil {
			t.Fatal("ListSourceStatus(unavailable) error = nil, want validation error")
		}
	})

	t.Run("Should disable catalog when registry does not expose store", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		daemonInstance := newTestDaemon(t, homePaths, &cfg)
		state := &bootState{
			cfg:      cfg,
			logger:   discardLogger(),
			registry: &recordingRegistry{path: homePaths.DatabaseFile},
		}
		if err := daemonInstance.bootModelCatalog(testutil.Context(t), state, &bootCleanup{}); err != nil {
			t.Fatalf("bootModelCatalog(non-store registry) error = %v", err)
		}
		if state.modelCatalog != nil {
			t.Fatalf("bootModelCatalog(non-store registry) modelCatalog = %#v, want nil", state.modelCatalog)
		}
	})

	t.Run("Should reject invalid timeouts during catalog boot", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		cfg.ModelCatalog.Sources.ModelsDev.Timeout = "not-a-duration"
		daemonInstance := newTestDaemon(t, homePaths, &cfg)
		state := &bootState{
			cfg: cfg,
			registry: &modelCatalogStoreRegistry{
				recordingRegistry: &recordingRegistry{path: homePaths.DatabaseFile},
			},
		}
		if err := daemonInstance.bootModelCatalog(testutil.Context(t), state, &bootCleanup{}); err == nil {
			t.Fatal("bootModelCatalog(invalid timeout) error = nil, want validation error")
		}

		cfg = testConfig(t, homePaths)
		cfg.ModelCatalog.Sources.ModelsDev.TTL = "not-a-duration"
		state = &bootState{cfg: cfg}
		if _, err := daemonInstance.modelCatalogSources(state, nil, defaultModelCatalogRefreshTimeout); err == nil {
			t.Fatal("modelCatalogSources(invalid ttl) error = nil, want validation error")
		}
	})

	t.Run("Should use env secret resolver when vault is unavailable", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		daemonInstance := newTestDaemon(t, homePaths, &cfg)
		daemonInstance.getenv = func(key string) string {
			if key == "MODEL_CATALOG_TEST_KEY" {
				return "secret-value"
			}
			return ""
		}
		value, err := daemonInstance.modelCatalogSecretResolver(&bootState{}).
			ResolveRef(testutil.Context(t), "env:MODEL_CATALOG_TEST_KEY")
		if err != nil {
			t.Fatalf("ResolveRef(env) error = %v", err)
		}
		if value != "secret-value" {
			t.Fatalf("ResolveRef(env) = %q, want secret-value", value)
		}
	})
}

func bootModelCatalogTestDaemon(
	t *testing.T,
	mutate func(*aghconfig.Config),
) (*Daemon, RuntimeDeps, RuntimeDeps) {
	t.Helper()

	return bootModelCatalogTestDaemonWithSetup(t, mutate, nil)
}

func bootModelCatalogTestDaemonWithSetup(
	t *testing.T,
	mutate func(*aghconfig.Config),
	setup func(*testing.T, aghconfig.HomePaths),
) (*Daemon, RuntimeDeps, RuntimeDeps) {
	t.Helper()

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Memory.Enabled = false
	cfg.Network.Enabled = false
	cfg.Skills.Enabled = false
	modelsDevEnabled := false
	cfg.ModelCatalog.Sources.ModelsDev.Enabled = &modelsDevEnabled
	if mutate != nil {
		mutate(&cfg)
	}
	if setup != nil {
		setup(t, homePaths)
	}

	daemonInstance := newTestDaemon(t, homePaths, &cfg)
	var sessionManagerCatalog modelcatalog.Service
	daemonInstance.newSessionManager = func(_ context.Context, deps SessionManagerDeps) (SessionManager, error) {
		sessionManagerCatalog = deps.ModelCatalog
		return &fakeSessionManager{}, nil
	}
	daemonInstance.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}

	var httpDeps RuntimeDeps
	var udsDeps RuntimeDeps
	daemonInstance.httpFactory = func(_ context.Context, deps RuntimeDeps) (Server, error) {
		httpDeps = deps
		return &fakeServer{name: "http"}, nil
	}
	daemonInstance.udsFactory = func(_ context.Context, deps RuntimeDeps) (Server, error) {
		udsDeps = deps
		return &fakeServer{name: "uds"}, nil
	}

	if err := daemonInstance.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	if sessionManagerCatalog != daemonInstance.modelCatalog {
		t.Fatalf(
			"session manager ModelCatalog = %p, want daemon runtime %p",
			sessionManagerCatalog,
			daemonInstance.modelCatalog,
		)
	}
	t.Cleanup(func() {
		if err := daemonInstance.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})
	return daemonInstance, httpDeps, udsDeps
}

func seedPreExplicitCurationRows(t *testing.T, homePaths aghconfig.HomePaths) {
	t.Helper()

	ctx := testutil.Context(t)
	db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	refreshedAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	persist := func(
		sourceID string,
		kind modelcatalog.SourceKind,
		priority int,
		rows []modelcatalog.ModelRow,
	) {
		t.Helper()
		if err := db.ReplaceSourceRows(ctx, sourceID, "codex", rows, modelcatalog.SourceStatus{
			SourceID:     sourceID,
			SourceKind:   kind,
			ProviderID:   "codex",
			Priority:     priority,
			LastRefresh:  refreshedAt,
			LastSuccess:  refreshedAt,
			RefreshState: modelcatalog.RefreshStateSucceeded,
			RowCount:     len(rows),
		}); err != nil {
			t.Fatalf("ReplaceSourceRows(%s) error = %v", sourceID, err)
		}
	}
	persist(
		modelcatalog.SourceIDBuiltin,
		modelcatalog.SourceKindBuiltin,
		modelcatalog.PriorityBuiltin,
		[]modelcatalog.ModelRow{
			{
				ProviderID:  "codex",
				ModelID:     "gpt-5.6-sol",
				DisplayName: "stale builtin row",
				SourceID:    modelcatalog.SourceIDBuiltin,
				SourceKind:  modelcatalog.SourceKindBuiltin,
				Priority:    modelcatalog.PriorityBuiltin,
				RefreshedAt: refreshedAt,
			},
		},
	)
	persist(
		modelcatalog.SourceIDConfig,
		modelcatalog.SourceKindConfig,
		modelcatalog.PriorityConfig,
		[]modelcatalog.ModelRow{
			{
				ProviderID:  "codex",
				ModelID:     "operator-default-only",
				SourceID:    modelcatalog.SourceIDConfig,
				SourceKind:  modelcatalog.SourceKindConfig,
				Priority:    modelcatalog.PriorityConfig,
				RefreshedAt: refreshedAt,
			},
			{
				ProviderID:  "codex",
				ModelID:     "gpt-5.6-sol",
				SourceID:    modelcatalog.SourceIDConfig,
				SourceKind:  modelcatalog.SourceKindConfig,
				Priority:    modelcatalog.PriorityConfig,
				RefreshedAt: refreshedAt,
			},
		})
	available := true
	liveSourceID := modelcatalog.SourceKindProviderLiveID("codex")
	persist(
		liveSourceID,
		modelcatalog.SourceKindProviderLive,
		modelcatalog.PriorityProviderLive,
		[]modelcatalog.ModelRow{
			{
				ProviderID:  "codex",
				ModelID:     "provider-live-only",
				SourceID:    liveSourceID,
				SourceKind:  modelcatalog.SourceKindProviderLive,
				Priority:    modelcatalog.PriorityProviderLive,
				Available:   &available,
				RefreshedAt: refreshedAt,
			},
		},
	)
	if err := db.Close(ctx); err != nil {
		t.Fatalf("CloseGlobalDB() error = %v", err)
	}
}

func containsCatalogModel(models []modelcatalog.Model, providerID string, modelID string) bool {
	for _, model := range models {
		if model.ProviderID == providerID && model.ModelID == modelID {
			return true
		}
	}
	return false
}

func findCatalogModel(models []modelcatalog.Model, modelID string) (modelcatalog.Model, bool) {
	for _, model := range models {
		if model.ModelID == modelID {
			return model, true
		}
	}
	return modelcatalog.Model{}, false
}

func findSourceStatus(
	statuses []modelcatalog.SourceStatus,
	sourceID string,
) (modelcatalog.SourceStatus, bool) {
	for _, status := range statuses {
		if status.SourceID == sourceID {
			return status, true
		}
	}
	return modelcatalog.SourceStatus{}, false
}

type blockingModelCatalogService struct {
	started      chan struct{}
	released     chan struct{}
	startOnce    sync.Once
	releasedOnce sync.Once
}

type recordingModelCatalogService struct {
	models       []modelcatalog.Model
	refreshCalls int
	lastRefresh  modelcatalog.RefreshOptions
	lastList     modelcatalog.ListOptions
}

func (s *recordingModelCatalogService) ListModels(
	_ context.Context,
	opts modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	s.lastList = opts
	return append([]modelcatalog.Model(nil), s.models...), nil
}

func (s *recordingModelCatalogService) Refresh(
	_ context.Context,
	opts modelcatalog.RefreshOptions,
) ([]modelcatalog.SourceStatus, error) {
	s.refreshCalls++
	s.lastRefresh = opts
	return nil, nil
}

func (s *recordingModelCatalogService) ListSourceStatus(
	context.Context,
	string,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}

type manuallyReleasedModelCatalogService struct {
	started  chan struct{}
	release  chan struct{}
	released chan struct{}
	once     sync.Once
}

func newManuallyReleasedModelCatalogService() *manuallyReleasedModelCatalogService {
	return &manuallyReleasedModelCatalogService{
		started:  make(chan struct{}),
		release:  make(chan struct{}),
		released: make(chan struct{}),
	}
}

func (s *manuallyReleasedModelCatalogService) ListModels(
	context.Context,
	modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	return nil, nil
}

func (s *manuallyReleasedModelCatalogService) Refresh(
	ctx context.Context,
	_ modelcatalog.RefreshOptions,
) ([]modelcatalog.SourceStatus, error) {
	s.once.Do(func() {
		close(s.started)
	})
	<-s.release
	close(s.released)
	return nil, ctx.Err()
}

func (s *manuallyReleasedModelCatalogService) ListSourceStatus(
	context.Context,
	string,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}

type modelCatalogStoreRegistry struct {
	*recordingRegistry
}

func (r *modelCatalogStoreRegistry) ReplaceSourceRows(
	context.Context,
	string,
	string,
	[]modelcatalog.ModelRow,
	modelcatalog.SourceStatus,
) error {
	return nil
}

func (r *modelCatalogStoreRegistry) ListRows(
	context.Context,
	modelcatalog.ListOptions,
) ([]modelcatalog.ModelRow, error) {
	return nil, nil
}

func (r *modelCatalogStoreRegistry) ListSourceStatus(
	context.Context,
	string,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}

func newBlockingModelCatalogService() *blockingModelCatalogService {
	return &blockingModelCatalogService{
		started:  make(chan struct{}),
		released: make(chan struct{}),
	}
}

func (s *blockingModelCatalogService) ListModels(
	context.Context,
	modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	return nil, nil
}

func (s *blockingModelCatalogService) Refresh(
	ctx context.Context,
	_ modelcatalog.RefreshOptions,
) ([]modelcatalog.SourceStatus, error) {
	s.startOnce.Do(func() {
		close(s.started)
	})
	<-ctx.Done()
	s.releasedOnce.Do(func() {
		close(s.released)
	})
	return nil, ctx.Err()
}

func (s *blockingModelCatalogService) ListSourceStatus(
	context.Context,
	string,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}

func waitForCatalogTestSignal(t *testing.T, ch <-chan struct{}, label string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for %s", label)
	}
}

func waitForCatalogTestError(t *testing.T, ch <-chan error, label string) error {
	t.Helper()

	select {
	case err := <-ch:
		return err
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for %s", label)
	}
	return nil
}

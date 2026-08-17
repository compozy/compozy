package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
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

	t.Run("Should apply live discovery config without coupling metadata edits to provider access", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		provider, err := cfg.ResolveProvider("cursor")
		if err != nil {
			t.Fatalf("ResolveProvider(cursor) error = %v", err)
		}
		enabled := true
		provider.Models.Discovery = compozyconfig.ProviderModelsDiscoveryConfig{
			Enabled: &enabled,
			Command: "cursor-old",
			Timeout: "2s",
		}
		cfg.Providers = map[string]compozyconfig.ProviderConfig{"cursor": provider}

		probe := &configReloadCursorACPProbe{}
		liveSource, err := modelcatalog.NewLiveProviderSource(
			"cursor",
			provider,
			&modelcatalog.LiveProviderSourcesConfig{
				HomePaths:      homePaths,
				CursorACPProbe: probe,
				DefaultTimeout: 5 * time.Second,
			},
		)
		if err != nil {
			t.Fatalf("NewLiveProviderSource() error = %v", err)
		}
		configSource := modelcatalog.NewConfigSource(cfg.Providers)
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		service, err := modelcatalog.NewService(
			db,
			[]modelcatalog.Source{configSource, liveSource},
			modelcatalog.MergeOptions{},
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		runtime, err := newModelCatalogRuntime(ctx, service, discardLogger(), nil, 5*time.Second)
		if err != nil {
			t.Fatalf("newModelCatalogRuntime() error = %v", err)
		}
		runtime.configSource = configSource
		runtime.liveSources = map[string]*modelcatalog.LiveProviderSource{"cursor": liveSource}
		t.Cleanup(func() {
			if shutdownErr := runtime.Shutdown(testutil.Context(t)); shutdownErr != nil {
				t.Fatalf("Shutdown() error = %v", shutdownErr)
			}
			if closeErr := db.Close(testutil.Context(t)); closeErr != nil {
				t.Fatalf("CloseGlobalDB() error = %v", closeErr)
			}
		})

		if _, err := runtime.Refresh(ctx, modelcatalog.RefreshOptions{
			ProviderID: "cursor",
			SourceID:   modelcatalog.SourceKindProviderLiveID("cursor"),
			Force:      true,
		}); err != nil {
			t.Fatalf("Refresh(initial cursor) error = %v", err)
		}

		next := cfg
		next.Providers = compozyconfig.CloneProviderConfigs(cfg.Providers)
		provider = next.Providers["cursor"]
		provider.Models.Discovery.Command = "cursor-new"
		provider.Models.Discovery.Timeout = "3s"
		next.Providers["cursor"] = provider
		if err := runtime.ReconcileConfig(ctx, &next); err != nil {
			t.Fatalf("ReconcileConfig(new discovery command) error = %v", err)
		}
		request := probe.LastRequest(t)
		if request.Command != "cursor-new" {
			t.Fatalf("discovery request = %#v, want cursor-new", request)
		}
		if request.Timeout != 3*time.Second {
			t.Fatalf("discovery timeout = %s, want 3s", request.Timeout)
		}
		models, err := service.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID:         "cursor",
			View:               modelcatalog.CatalogViewAll,
			SkipRefreshIfEmpty: true,
		})
		if err != nil {
			t.Fatalf("ListModels(after live config) error = %v", err)
		}
		if !containsCatalogModel(models, "cursor", "new-model") ||
			containsCatalogModel(models, "cursor", "old-model") {
			t.Fatalf("ListModels(after live config) = %#v, want only new-model", models)
		}

		callsBeforeMetadata := probe.CallCount()
		provider = next.Providers["cursor"]
		provider.Models.Curated = append(provider.Models.Curated, compozyconfig.ProviderModelConfig{
			ID: "metadata-only",
		})
		next.Providers["cursor"] = provider
		if err := runtime.ReconcileConfig(ctx, &next); err != nil {
			t.Fatalf("ReconcileConfig(metadata only) error = %v", err)
		}
		if calls := probe.CallCount(); calls != callsBeforeMetadata {
			t.Fatalf("discovery calls after metadata edit = %d, want %d", calls, callsBeforeMetadata)
		}

		provider = next.Providers["cursor"]
		provider.Models.Discovery.Command = "cursor-offline"
		next.Providers["cursor"] = provider
		if err := runtime.ReconcileConfig(ctx, &next); err != nil {
			t.Fatalf("ReconcileConfig(offline discovery) error = %v", err)
		}
		models, err = service.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID:         "cursor",
			View:               modelcatalog.CatalogViewAll,
			IncludeStale:       true,
			SkipRefreshIfEmpty: true,
		})
		if err != nil {
			t.Fatalf("ListModels(offline discovery) error = %v", err)
		}
		model, ok := findCatalogModel(models, "new-model")
		if !ok || !model.Stale {
			t.Fatalf("offline discovery model = %#v, want stale new-model", model)
		}
		statuses, err := service.ListSourceStatus(ctx, "cursor")
		if err != nil {
			t.Fatalf("ListSourceStatus(offline discovery) error = %v", err)
		}
		status, ok := findSourceStatus(statuses, modelcatalog.SourceKindProviderLiveID("cursor"))
		if !ok || status.RefreshState != modelcatalog.RefreshStateFailed {
			t.Fatalf("offline live source status = %#v, want failed", status)
		}

		disabled := false
		provider = next.Providers["cursor"]
		provider.Models.Discovery.Enabled = &disabled
		next.Providers["cursor"] = provider
		if err := runtime.ReconcileConfig(ctx, &next); err != nil {
			t.Fatalf("ReconcileConfig(disabled discovery) error = %v", err)
		}
		models, err = service.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID:         "cursor",
			View:               modelcatalog.CatalogViewAll,
			SkipRefreshIfEmpty: true,
		})
		if err != nil {
			t.Fatalf("ListModels(disabled discovery) error = %v", err)
		}
		if containsCatalogModel(models, "cursor", "new-model") {
			t.Fatalf("ListModels(disabled discovery) = %#v, want live row cleared", models)
		}
		statuses, err = service.ListSourceStatus(ctx, "cursor")
		if err != nil {
			t.Fatalf("ListSourceStatus(disabled discovery) error = %v", err)
		}
		status, ok = findSourceStatus(statuses, modelcatalog.SourceKindProviderLiveID("cursor"))
		if !ok || status.RefreshState != modelcatalog.RefreshStateDisabled {
			t.Fatalf("disabled live source status = %#v, want disabled", status)
		}
	})

	t.Run("Should publish config and live rows as one serialized generation", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		provider, err := cfg.ResolveProvider("cursor")
		if err != nil {
			t.Fatalf("ResolveProvider(cursor) error = %v", err)
		}
		enabled := true
		provider.Models.Default = "config-a"
		provider.Models.Discovery = compozyconfig.ProviderModelsDiscoveryConfig{
			Enabled: &enabled,
			Command: "cursor-a",
			Timeout: "2s",
		}
		cfg.Providers = map[string]compozyconfig.ProviderConfig{"cursor": provider}

		probe := newGenerationCursorACPProbe()
		t.Cleanup(probe.releaseAll)
		liveSource, err := modelcatalog.NewLiveProviderSource(
			"cursor",
			provider,
			&modelcatalog.LiveProviderSourcesConfig{
				HomePaths:      homePaths,
				CursorACPProbe: probe,
				DefaultTimeout: 5 * time.Second,
			},
		)
		if err != nil {
			t.Fatalf("NewLiveProviderSource() error = %v", err)
		}
		configSource := modelcatalog.NewConfigSource(cfg.Providers)
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		service, err := modelcatalog.NewService(
			db,
			[]modelcatalog.Source{configSource, liveSource},
			modelcatalog.MergeOptions{},
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		runtime, err := newModelCatalogRuntime(ctx, service, discardLogger(), nil, 5*time.Second)
		if err != nil {
			t.Fatalf("newModelCatalogRuntime() error = %v", err)
		}
		runtime.configSource = configSource
		runtime.liveSources = map[string]*modelcatalog.LiveProviderSource{"cursor": liveSource}
		t.Cleanup(func() {
			if shutdownErr := runtime.Shutdown(testutil.Context(t)); shutdownErr != nil {
				t.Fatalf("Shutdown() error = %v", shutdownErr)
			}
			if closeErr := db.Close(testutil.Context(t)); closeErr != nil {
				t.Fatalf("CloseGlobalDB() error = %v", closeErr)
			}
		})

		refreshA := make(chan error, 1)
		go func() {
			_, refreshErr := runtime.Refresh(ctx, modelcatalog.RefreshOptions{
				ProviderID: "cursor",
				SourceID:   modelcatalog.SourceKindProviderLiveID("cursor"),
				Force:      true,
			})
			refreshA <- refreshErr
		}()
		probe.waitForCommand(t, "cursor-a")

		next := cfg
		next.Providers = compozyconfig.CloneProviderConfigs(cfg.Providers)
		provider = next.Providers["cursor"]
		provider.Models.Default = "config-b"
		provider.Models.Discovery.Command = "cursor-b"
		next.Providers["cursor"] = provider
		reconcileB := make(chan error, 1)
		go func() {
			reconcileB <- runtime.ReconcileConfig(ctx, &next)
		}()

		probe.release("cursor-a")
		if refreshErr := waitForCatalogTestError(t, refreshA, "config A refresh"); refreshErr != nil {
			t.Fatalf("Refresh(config A) error = %v", refreshErr)
		}
		probe.waitForCommand(t, "cursor-b")

		type listResult struct {
			models []modelcatalog.Model
			err    error
		}
		readerStarted := make(chan struct{})
		readerResult := make(chan listResult, 1)
		go func() {
			close(readerStarted)
			models, listErr := runtime.ListModels(ctx, modelcatalog.ListOptions{
				ProviderID:         "cursor",
				View:               modelcatalog.CatalogViewAll,
				IncludeAll:         true,
				SkipRefreshIfEmpty: true,
			})
			readerResult <- listResult{models: models, err: listErr}
		}()
		<-readerStarted
		select {
		case result := <-readerResult:
			t.Fatalf("ListModels() returned during config B publication: %#v", result)
		default:
		}

		probe.release("cursor-b")
		if reconcileErr := waitForCatalogTestError(t, reconcileB, "config B reconcile"); reconcileErr != nil {
			t.Fatalf("ReconcileConfig(config B) error = %v", reconcileErr)
		}
		result := <-readerResult
		if result.err != nil {
			t.Fatalf("ListModels(after config B) error = %v", result.err)
		}
		if !containsCatalogModel(result.models, "cursor", "config-b") ||
			!containsCatalogModel(result.models, "cursor", "live-b") ||
			containsCatalogModel(result.models, "cursor", "config-a") ||
			containsCatalogModel(result.models, "cursor", "live-a") {
			t.Fatalf("ListModels(after config B) = %#v, want only generation B rows", result.models)
		}
		if got, want := probe.commands(), []string{"cursor-a", "cursor-b"}; !slices.Equal(got, want) {
			t.Fatalf("discovery commands = %#v, want %#v", got, want)
		}
	})

	t.Run("Should honor reconciliation cancellation while waiting for an active generation", func(t *testing.T) {
		t.Parallel()

		service := newBlockingModelCatalogService()
		runtime, err := newModelCatalogRuntime(
			testutil.Context(t),
			service,
			discardLogger(),
			nil,
			5*time.Second,
		)
		if err != nil {
			t.Fatalf("newModelCatalogRuntime() error = %v", err)
		}
		runtime.configSource = modelcatalog.NewConfigSource(map[string]compozyconfig.ProviderConfig{
			"cursor": {Models: compozyconfig.ProviderModelsConfig{Default: "old-model"}},
		})
		runtime.liveSources = map[string]*modelcatalog.LiveProviderSource{}

		refreshResult := make(chan error, 1)
		go func() {
			_, refreshErr := runtime.Refresh(testutil.Context(t), modelcatalog.RefreshOptions{
				ProviderID: "cursor",
				Force:      true,
			})
			refreshResult <- refreshErr
		}()
		waitForCatalogTestSignal(t, service.started, "active model catalog refresh")

		alreadyCanceled, cancel := context.WithCancel(testutil.Context(t))
		cancel()
		err = runtime.ReconcileConfig(alreadyCanceled, &compozyconfig.Config{})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ReconcileConfig(already canceled) error = %v, want context.Canceled", err)
		}

		canceled := newCancelWhenWaitedContext(testutil.Context(t))
		err = runtime.ReconcileConfig(canceled, &compozyconfig.Config{})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ReconcileConfig(wait cancellation) error = %v, want context.Canceled", err)
		}
		if got, want := runtime.configSource.ProviderIDs(), []string{"cursor"}; !slices.Equal(got, want) {
			t.Fatalf("config source providers after canceled reconcile = %#v, want %#v", got, want)
		}

		if err := runtime.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		if refreshErr := waitForCatalogTestError(
			t,
			refreshResult,
			"active refresh shutdown",
		); !errors.Is(refreshErr, context.Canceled) {
			t.Fatalf("Refresh(shutdown) error = %v, want context.Canceled", refreshErr)
		}
	})

	t.Run("Should validate status context before and while waiting for a generation", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		provider, err := cfg.ResolveProvider("cursor")
		if err != nil {
			t.Fatalf("ResolveProvider(cursor) error = %v", err)
		}
		enabled := true
		provider.Models.Discovery = compozyconfig.ProviderModelsDiscoveryConfig{
			Enabled: &enabled,
			Command: "cursor-a",
			Timeout: "2s",
		}
		cfg.Providers = map[string]compozyconfig.ProviderConfig{"cursor": provider}
		probe := newGenerationCursorACPProbe()
		t.Cleanup(probe.releaseAll)
		liveSource, err := modelcatalog.NewLiveProviderSource(
			"cursor",
			provider,
			&modelcatalog.LiveProviderSourcesConfig{
				HomePaths:      homePaths,
				CursorACPProbe: probe,
				DefaultTimeout: 5 * time.Second,
			},
		)
		if err != nil {
			t.Fatalf("NewLiveProviderSource() error = %v", err)
		}
		configSource := modelcatalog.NewConfigSource(cfg.Providers)
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		service, err := modelcatalog.NewService(
			db,
			[]modelcatalog.Source{configSource, liveSource},
			modelcatalog.MergeOptions{},
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		runtime, err := newModelCatalogRuntime(ctx, service, discardLogger(), nil, 5*time.Second)
		if err != nil {
			t.Fatalf("newModelCatalogRuntime() error = %v", err)
		}
		runtime.configSource = configSource
		runtime.liveSources = map[string]*modelcatalog.LiveProviderSource{"cursor": liveSource}
		t.Cleanup(func() {
			if shutdownErr := runtime.Shutdown(testutil.Context(t)); shutdownErr != nil {
				t.Fatalf("Shutdown() error = %v", shutdownErr)
			}
			if closeErr := db.Close(testutil.Context(t)); closeErr != nil {
				t.Fatalf("CloseGlobalDB() error = %v", closeErr)
			}
		})

		next := cfg
		next.Providers = compozyconfig.CloneProviderConfigs(cfg.Providers)
		provider = next.Providers["cursor"]
		provider.Models.Discovery.Command = "cursor-b"
		next.Providers["cursor"] = provider
		reconcileResult := make(chan error, 1)
		go func() {
			reconcileResult <- runtime.ReconcileConfig(ctx, &next)
		}()
		probe.waitForCommand(t, "cursor-b")

		type statusResult struct {
			statuses []modelcatalog.SourceStatus
			err      error
		}
		nilResult := make(chan statusResult, 1)
		go func() {
			var missing context.Context
			statuses, statusErr := runtime.ListSourceStatus(missing, "cursor")
			nilResult <- statusResult{statuses: statuses, err: statusErr}
		}()
		select {
		case result := <-nilResult:
			if result.err == nil {
				t.Fatal("ListSourceStatus(nil context) error = nil, want validation error")
			}
		case <-time.After(time.Second):
			t.Fatal("ListSourceStatus(nil context) waited for generation lock")
		}

		canceledResult := make(chan statusResult, 1)
		go func() {
			statuses, statusErr := runtime.ListSourceStatus(
				newCancelWhenWaitedContext(ctx),
				"cursor",
			)
			canceledResult <- statusResult{statuses: statuses, err: statusErr}
		}()
		select {
		case result := <-canceledResult:
			if !errors.Is(result.err, context.Canceled) {
				t.Fatalf("ListSourceStatus(wait cancellation) error = %v, want context.Canceled", result.err)
			}
		case <-time.After(time.Second):
			t.Fatal("ListSourceStatus(canceled context) waited for generation publication")
		}

		probe.release("cursor-b")
		if reconcileErr := waitForCatalogTestError(
			t,
			reconcileResult,
			"status generation reconcile",
		); reconcileErr != nil {
			t.Fatalf("ReconcileConfig() error = %v", reconcileErr)
		}
	})

	t.Run("Should keep the previous generation when a later provider publication fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		providers := make(map[string]compozyconfig.ProviderConfig, 2)
		for _, providerID := range []string{"claude", "codex"} {
			provider, err := cfg.ResolveProvider(providerID)
			if err != nil {
				t.Fatalf("ResolveProvider(%s) error = %v", providerID, err)
			}
			provider.Models = compozyconfig.ProviderModelsConfig{Default: "old-" + providerID}
			providers[providerID] = provider
		}
		cfg.Providers = providers
		configSource := modelcatalog.NewConfigSource(cfg.Providers)
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		service, err := modelcatalog.NewService(
			db,
			[]modelcatalog.Source{configSource},
			modelcatalog.MergeOptions{},
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		for _, providerID := range []string{"claude", "codex"} {
			if _, refreshErr := service.Refresh(ctx, modelcatalog.RefreshOptions{
				ProviderID: providerID,
				SourceID:   modelcatalog.SourceIDConfig,
				Force:      true,
			}); refreshErr != nil {
				t.Fatalf("Refresh(seed %s) error = %v", providerID, refreshErr)
			}
		}
		runtime, err := newModelCatalogRuntime(ctx, service, discardLogger(), nil, 5*time.Second)
		if err != nil {
			t.Fatalf("newModelCatalogRuntime() error = %v", err)
		}
		runtime.configSource = configSource
		runtime.liveSources = map[string]*modelcatalog.LiveProviderSource{}
		t.Cleanup(func() {
			if shutdownErr := runtime.Shutdown(testutil.Context(t)); shutdownErr != nil {
				t.Fatalf("Shutdown() error = %v", shutdownErr)
			}
			if closeErr := db.Close(testutil.Context(t)); closeErr != nil {
				t.Fatalf("CloseGlobalDB() error = %v", closeErr)
			}
		})

		next := cfg
		next.Providers = compozyconfig.CloneProviderConfigs(cfg.Providers)
		claude := next.Providers["claude"]
		claude.Models = compozyconfig.ProviderModelsConfig{Default: "new-claude"}
		next.Providers["claude"] = claude
		codex := next.Providers["codex"]
		codex.Models = compozyconfig.ProviderModelsConfig{
			Default: "new-codex",
			Curated: []compozyconfig.ProviderModelConfig{{
				ID:               "invalid-codex",
				ReasoningEfforts: []string{"high", "high"},
			}},
		}
		next.Providers["codex"] = codex
		err = runtime.ReconcileConfig(ctx, &next)
		if err == nil {
			t.Fatal("ReconcileConfig() error = nil, want later provider persistence failure")
		}

		models, listErr := runtime.ListModels(ctx, modelcatalog.ListOptions{
			View:               modelcatalog.CatalogViewAll,
			IncludeAll:         true,
			SkipRefreshIfEmpty: true,
		})
		if listErr != nil {
			t.Fatalf("ListModels(after failed generation) error = %v", listErr)
		}
		got := make([]string, 0, len(models))
		for _, model := range models {
			got = append(got, model.ProviderID+"/"+model.ModelID)
		}
		want := []string{"claude/old-claude", "codex/old-codex"}
		if !slices.Equal(got, want) {
			t.Fatalf("models after failed generation = %#v, want %#v", got, want)
		}
		for _, providerID := range []string{"claude", "codex"} {
			rows, sourceErr := configSource.ListModels(ctx, modelcatalog.ListOptions{ProviderID: providerID})
			if sourceErr != nil {
				t.Fatalf("config source ListModels(%s) error = %v", providerID, sourceErr)
			}
			if len(rows) != 1 || rows[0].ModelID != "old-"+providerID {
				t.Fatalf("config source rows for %s = %#v, want previous generation", providerID, rows)
			}
		}
	})

	t.Run("Should return live status persistence failures from config reconciliation", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		provider, err := cfg.ResolveProvider("cursor")
		if err != nil {
			t.Fatalf("ResolveProvider(cursor) error = %v", err)
		}
		enabled := true
		provider.Models.Discovery = compozyconfig.ProviderModelsDiscoveryConfig{
			Enabled: &enabled,
			Command: "cursor-old",
			Timeout: "2s",
		}
		cfg.Providers = map[string]compozyconfig.ProviderConfig{"cursor": provider}

		probe := &configReloadCursorACPProbe{}
		liveSource, err := modelcatalog.NewLiveProviderSource(
			"cursor",
			provider,
			&modelcatalog.LiveProviderSourcesConfig{
				HomePaths:      homePaths,
				CursorACPProbe: probe,
				DefaultTimeout: 5 * time.Second,
			},
		)
		if err != nil {
			t.Fatalf("NewLiveProviderSource() error = %v", err)
		}
		configSource := modelcatalog.NewConfigSource(cfg.Providers)
		db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		persistErr := errors.New("catalog persistence unavailable")
		store := &failingModelCatalogStore{
			Store:        db,
			failedSource: modelcatalog.SourceKindProviderLiveID("cursor"),
			err:          persistErr,
		}
		service, err := modelcatalog.NewService(
			store,
			[]modelcatalog.Source{configSource, liveSource},
			modelcatalog.MergeOptions{},
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		runtime, err := newModelCatalogRuntime(ctx, service, discardLogger(), nil, 5*time.Second)
		if err != nil {
			t.Fatalf("newModelCatalogRuntime() error = %v", err)
		}
		runtime.configSource = configSource
		runtime.liveSources = map[string]*modelcatalog.LiveProviderSource{"cursor": liveSource}
		t.Cleanup(func() {
			if shutdownErr := runtime.Shutdown(testutil.Context(t)); shutdownErr != nil {
				t.Fatalf("Shutdown() error = %v", shutdownErr)
			}
			if closeErr := db.Close(testutil.Context(t)); closeErr != nil {
				t.Fatalf("CloseGlobalDB() error = %v", closeErr)
			}
		})

		next := cfg
		next.Providers = compozyconfig.CloneProviderConfigs(cfg.Providers)
		provider = next.Providers["cursor"]
		provider.Models.Discovery.Command = "cursor-offline"
		next.Providers["cursor"] = provider
		err = runtime.ReconcileConfig(ctx, &next)
		if !errors.Is(err, persistErr) {
			t.Fatalf("ReconcileConfig() error = %v, want persistence identity", err)
		}
		statuses, statusErr := db.ListSourceStatus(ctx, "cursor")
		if statusErr != nil {
			t.Fatalf("ListSourceStatus(cursor) error = %v", statusErr)
		}
		if status, ok := findSourceStatus(statuses, modelcatalog.SourceKindProviderLiveID("cursor")); ok &&
			status.RefreshState == modelcatalog.RefreshStateFailed {
			t.Fatalf("live source status = %#v, want no unpersisted failure", status)
		}
		rows, sourceErr := liveSource.ListModels(ctx, modelcatalog.ListOptions{ProviderID: "cursor"})
		if sourceErr != nil {
			t.Fatalf("live source ListModels(after failed reconcile) error = %v", sourceErr)
		}
		if len(rows) != 1 || rows[0].ModelID != "old-model" {
			t.Fatalf("live source rows after failed reconcile = %#v, want old provider snapshot", rows)
		}
		if request := probe.LastRequest(t); request.Command != "cursor-old" {
			t.Fatalf("live source command after failed reconcile = %q, want cursor-old", request.Command)
		}
	})

	t.Run("Should rehydrate explicit curation before the first client list", func(t *testing.T) {
		t.Parallel()

		_, httpDeps, _ := bootModelCatalogTestDaemonWithSetup(
			t,
			func(cfg *compozyconfig.Config) {
				provider := cfg.Providers["codex"]
				provider.Models = compozyconfig.ProviderModelsConfig{
					Default: "operator-default-only",
					Curated: []compozyconfig.ProviderModelConfig{{ID: "gpt-5.6-sol"}},
				}
				if cfg.Providers == nil {
					cfg.Providers = make(map[string]compozyconfig.ProviderConfig)
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
		if err := runtime.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown(retry) error = %v", err)
		}
		if _, err := runtime.Refresh(testutil.Context(t), modelcatalog.RefreshOptions{Force: true}); err == nil {
			t.Fatal("Refresh(after shutdown) error = nil, want admission failure")
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
		var missingContext context.Context
		if err := runtime.Shutdown(missingContext); err == nil {
			t.Fatal("Shutdown(nil context) error = nil, want validation error")
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
	mutate func(*compozyconfig.Config),
) (*Daemon, RuntimeDeps, RuntimeDeps) {
	t.Helper()

	return bootModelCatalogTestDaemonWithSetup(t, mutate, nil)
}

func bootModelCatalogTestDaemonWithSetup(
	t *testing.T,
	mutate func(*compozyconfig.Config),
	setup func(*testing.T, compozyconfig.HomePaths),
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
	daemonInstance.newSessionManager = func(_ context.Context, _ SessionManagerDeps) (SessionManager, error) {
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
	t.Cleanup(func() {
		if err := daemonInstance.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})
	return daemonInstance, httpDeps, udsDeps
}

func seedPreExplicitCurationRows(t *testing.T, homePaths compozyconfig.HomePaths) {
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

type configReloadCursorACPProbe struct {
	mu       sync.Mutex
	requests []modelcatalog.CursorACPModelProbeRequest
}

type generationCursorACPProbe struct {
	mu        sync.Mutex
	requests  []modelcatalog.CursorACPModelProbeRequest
	started   chan modelcatalog.CursorACPModelProbeRequest
	releaseA  chan struct{}
	releaseB  chan struct{}
	releaseAO sync.Once
	releaseBO sync.Once
}

func newGenerationCursorACPProbe() *generationCursorACPProbe {
	return &generationCursorACPProbe{
		started:  make(chan modelcatalog.CursorACPModelProbeRequest, 2),
		releaseA: make(chan struct{}),
		releaseB: make(chan struct{}),
	}
}

func (e *generationCursorACPProbe) InspectCursorModels(
	ctx context.Context,
	req modelcatalog.CursorACPModelProbeRequest,
) ([]acp.SessionConfigOption, error) {
	e.mu.Lock()
	e.requests = append(e.requests, req)
	e.mu.Unlock()
	e.started <- req

	var release <-chan struct{}
	var modelID string
	switch req.Command {
	case "cursor-a":
		release = e.releaseA
		modelID = "live-a"
	case "cursor-b":
		release = e.releaseB
		modelID = "live-b"
	default:
		return nil, fmt.Errorf("unexpected discovery command %q", req.Command)
	}
	select {
	case <-release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return cursorModelOptions(modelID), nil
}

func (e *generationCursorACPProbe) waitForCommand(t *testing.T, want string) {
	t.Helper()
	ctx := testutil.Context(t)
	select {
	case req := <-e.started:
		if req.Command != want {
			t.Fatalf("discovery command = %q, want %q", req.Command, want)
		}
	case <-ctx.Done():
		t.Fatalf("timeout waiting for discovery command %q: %v", want, ctx.Err())
	}
}

func (e *generationCursorACPProbe) release(command string) {
	switch command {
	case "cursor-a":
		e.releaseAO.Do(func() { close(e.releaseA) })
	case "cursor-b":
		e.releaseBO.Do(func() { close(e.releaseB) })
	}
}

func (e *generationCursorACPProbe) releaseAll() {
	e.release("cursor-a")
	e.release("cursor-b")
}

func (e *generationCursorACPProbe) commands() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	commands := make([]string, 0, len(e.requests))
	for _, req := range e.requests {
		commands = append(commands, req.Command)
	}
	return commands
}

type failingModelCatalogStore struct {
	modelcatalog.Store
	failedSource string
	err          error
}

var _ modelcatalog.Store = (*failingModelCatalogStore)(nil)

func (s *failingModelCatalogStore) ReplaceSourceRows(
	ctx context.Context,
	sourceID string,
	providerID string,
	rows []modelcatalog.ModelRow,
	status modelcatalog.SourceStatus,
) error {
	if sourceID == s.failedSource {
		return s.err
	}
	return s.Store.ReplaceSourceRows(ctx, sourceID, providerID, rows, status)
}

func (s *failingModelCatalogStore) ReplaceSourceRowsBatch(
	ctx context.Context,
	replacements []modelcatalog.SourceRowsReplacement,
) error {
	for _, replacement := range replacements {
		if replacement.SourceID == s.failedSource {
			return s.err
		}
	}
	return s.Store.ReplaceSourceRowsBatch(ctx, replacements)
}

func (e *configReloadCursorACPProbe) InspectCursorModels(
	_ context.Context,
	req modelcatalog.CursorACPModelProbeRequest,
) ([]acp.SessionConfigOption, error) {
	e.mu.Lock()
	e.requests = append(e.requests, req)
	e.mu.Unlock()
	if req.Command == "cursor-offline" {
		return nil, errors.New("cursor discovery offline")
	}
	modelID := "old-model"
	if req.Command == "cursor-new" {
		modelID = "new-model"
	}
	return cursorModelOptions(modelID), nil
}

func (e *configReloadCursorACPProbe) LastRequest(
	t *testing.T,
) modelcatalog.CursorACPModelProbeRequest {
	t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.requests) == 0 {
		t.Fatal("discovery requests = empty")
	}
	return e.requests[len(e.requests)-1]
}

func (e *configReloadCursorACPProbe) CallCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.requests)
}

func cursorModelOptions(modelID string) []acp.SessionConfigOption {
	return []acp.SessionConfigOption{{
		ID:       "model",
		Category: "model",
		Kind:     acp.SessionConfigOptionKindSelect,
		Values: []acp.SessionConfigOptionValue{{
			Value: modelID,
			Label: modelID,
		}},
	}}
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

type cancelWhenWaitedContext struct {
	context.Context
	done     chan struct{}
	cancel   sync.Once
	mu       sync.Mutex
	canceled bool
}

func newCancelWhenWaitedContext(parent context.Context) *cancelWhenWaitedContext {
	return &cancelWhenWaitedContext{Context: parent, done: make(chan struct{})}
}

func (c *cancelWhenWaitedContext) Done() <-chan struct{} {
	c.cancel.Do(func() {
		c.mu.Lock()
		c.canceled = true
		close(c.done)
		c.mu.Unlock()
	})
	return c.done
}

func (c *cancelWhenWaitedContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.canceled {
		return context.Canceled
	}
	return nil
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

func (r *modelCatalogStoreRegistry) ReplaceSourceRowsBatch(
	context.Context,
	[]modelcatalog.SourceRowsReplacement,
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

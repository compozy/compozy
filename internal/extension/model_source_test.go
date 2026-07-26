package extensionpkg

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apicontract "github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	extensioncontract "github.com/compozy/agh/internal/extension/contract"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
)

func TestModelSourceShouldPersistValidatedRowsThroughCatalogService(t *testing.T) {
	t.Parallel()

	t.Run("Should deep copy curation flags at the extension boundary", func(t *testing.T) {
		t.Parallel()

		deprecated := false
		hidden := true
		featured := false
		rows := []extensioncontract.ModelSourceRow{{
			SourceID:   "extension:ext-models",
			ProviderID: "codex",
			ModelID:    "gpt-5.6-sol",
			Deprecated: &deprecated,
			Hidden:     &hidden,
			Featured:   &featured,
		}}
		cloned := cloneModelSourceRows(rows)
		deprecated = true
		hidden = false
		featured = true

		row := cloned[0]
		if row.Deprecated == nil || *row.Deprecated || row.Hidden == nil || !*row.Hidden ||
			row.Featured == nil || *row.Featured {
			t.Fatalf("cloned curation flags = %#v, want false/true/false snapshot", row)
		}
	})

	t.Run("Should persist validated rows through catalog service", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
		store := openModelSourceTestStore(t)
		runtime := &fakeModelSourceRuntime{}
		source := newTestModelSource(t, "ext-models", runtime)
		available := true
		inputCost := 1.25
		outputCost := 10.5
		cacheReadCost := 0.125
		cacheWriteCost := 2.5
		reasoningCost := 12.0
		releaseDate := "2026-06-26"
		runtime.rows = []extensioncontract.ModelSourceRow{
			{
				SourceID:          source.ID(),
				ProviderID:        "codex",
				ModelID:           "gpt-5.4-extension",
				DisplayName:       "GPT 5.4 Extension",
				Available:         &available,
				ReasoningEfforts:  []apicontract.ReasoningEffort{"high"},
				ContextWindow:     new(int64(200000)),
				SupportsTools:     new(true),
				SupportsReasoning: new(true),
				Deprecated:        new(true),
				Hidden:            new(true),
				Featured:          new(true),
				ReleaseDate:       &releaseDate,
				Cost: &apicontract.ModelCatalogCostPayload{
					InputPerMillion:      &inputCost,
					OutputPerMillion:     &outputCost,
					CacheReadPerMillion:  &cacheReadCost,
					CacheWritePerMillion: &cacheWriteCost,
					ReasoningPerMillion:  &reasoningCost,
				},
			},
		}
		service := newTestModelCatalogService(t, store, []modelcatalog.Source{source})

		statuses, err := service.Refresh(ctx, modelcatalog.RefreshOptions{
			ProviderID: "codex",
			SourceID:   source.ID(),
			Force:      true,
			Now:        now,
		})
		if err != nil {
			t.Fatalf("Refresh() error = %v, want nil", err)
		}
		if len(statuses) != 1 || statuses[0].RefreshState != modelcatalog.RefreshStateSucceeded {
			t.Fatalf("Refresh() statuses = %#v, want succeeded extension status", statuses)
		}

		models, err := service.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID:   "codex",
			IncludeStale: true,
			View:         modelcatalog.CatalogViewAll,
			Now:          now,
		})
		if err != nil {
			t.Fatalf("ListModels() error = %v, want nil", err)
		}
		if len(models) != 1 || models[0].ModelID != "gpt-5.4-extension" {
			t.Fatalf("ListModels() = %#v, want persisted extension model", models)
		}
		if len(models[0].Sources) != 1 || models[0].Sources[0].SourceID != source.ID() {
			t.Fatalf("ListModels()[0].Sources = %#v, want extension source ref", models[0].Sources)
		}
		if !models[0].Deprecated || !models[0].Hidden || !models[0].Featured ||
			models[0].ReleaseDate == nil || *models[0].ReleaseDate != releaseDate {
			t.Fatalf("ListModels()[0] curation = %#v, want all flags and release %q", models[0], releaseDate)
		}
		if models[0].CostInputPerMillion == nil || *models[0].CostInputPerMillion != inputCost ||
			models[0].CostOutputPerMillion == nil || *models[0].CostOutputPerMillion != outputCost ||
			models[0].CostCacheReadPerMillion == nil || *models[0].CostCacheReadPerMillion != cacheReadCost ||
			models[0].CostCacheWritePerMillion == nil || *models[0].CostCacheWritePerMillion != cacheWriteCost ||
			models[0].CostReasoningPerMillion == nil || *models[0].CostReasoningPerMillion != reasoningCost {
			t.Fatalf("ListModels()[0] five-rate pricing = %#v, want extension rates", models[0])
		}
	})
}

func TestNewExtensionModelSourcesShouldFilterRegistryModelSourceCapabilities(t *testing.T) {
	t.Run("Should filter registry model source capabilities", func(t *testing.T) {
		withDaemonVersion(t, "0.5.0")

		store := openModelSourceTestStore(t)
		registry := NewRegistry(store.DB())
		modelFixture := createManagerTestExtension(t, managerTestManifest("ext-registry-models", managerManifestOptions{
			capabilities: []string{extensionprotocol.CapabilityProvideModelSource},
		}), nil)
		installManagerFixture(t, registry, modelFixture, SourceUser, true)
		memoryFixture := createManagerTestExtension(
			t,
			managerTestManifest("ext-registry-memory", managerManifestOptions{
				capabilities: []string{"memory.backend"},
			}),
			nil,
		)
		installManagerFixture(t, registry, memoryFixture, SourceUser, true)

		sources, err := NewExtensionModelSources(registry, func() ModelSourceRuntime { return nil })
		if err != nil {
			t.Fatalf("NewExtensionModelSources() error = %v, want nil", err)
		}
		if len(sources) != 1 || sources[0].ID() != "extension:ext-registry-models" {
			t.Fatalf("NewExtensionModelSources() = %#v, want only model.source extension", sources)
		}
		if got, err := NewExtensionModelSources(nil, nil); err != nil || got != nil {
			t.Fatalf("NewExtensionModelSources(nil) = (%#v, %v), want nil, nil", got, err)
		}
	})
}

func TestModelSourceIdentityHelpersShouldValidateInputs(t *testing.T) {
	t.Parallel()

	t.Run("Should validate inputs", func(t *testing.T) {
		t.Parallel()

		var nilSource *ModelSource
		if got := nilSource.ID(); got != "" {
			t.Fatalf("(*ModelSource)(nil).ID() = %q, want empty", got)
		}
		if _, err := NewExtensionModelSource(ExtensionInfo{Name: "bad/source"}, nil); err == nil {
			t.Fatal("NewExtensionModelSource(invalid name) error = nil, want slug validation error")
		}
	})
}

func TestModelSourceListModelsShouldRejectInvalidRuntimeState(t *testing.T) {
	t.Parallel()

	runtime := &fakeModelSourceRuntime{}
	source := newTestModelSource(t, "ext-invalid-state", runtime)
	tests := []struct {
		name   string
		source *ModelSource
		ctx    context.Context
	}{
		{
			name:   "Should reject nil context",
			source: source,
		},
		{
			name: "Should reject disabled extension source",
			source: mustTestModelSource(t, ExtensionInfo{
				Name:    "ext-disabled-source",
				Enabled: false,
				Capabilities: CapabilitiesConfig{
					Provides: []string{extensionprotocol.CapabilityProvideModelSource},
				},
			}, func() ModelSourceRuntime {
				return runtime
			}),
			ctx: testutil.Context(t),
		},
		{
			name: "Should reject missing model source capability",
			source: mustTestModelSource(t, ExtensionInfo{
				Name:    "ext-missing-capability",
				Enabled: true,
			}, func() ModelSourceRuntime {
				return runtime
			}),
			ctx: testutil.Context(t),
		},
		{
			name: "Should reject nil runtime resolver",
			source: mustTestModelSource(t, ExtensionInfo{
				Name:    "ext-nil-resolver",
				Enabled: true,
				Capabilities: CapabilitiesConfig{
					Provides: []string{extensionprotocol.CapabilityProvideModelSource},
				},
			}, nil),
			ctx: testutil.Context(t),
		},
		{
			name: "Should reject unavailable runtime",
			source: mustTestModelSource(t, ExtensionInfo{
				Name:    "ext-nil-runtime",
				Enabled: true,
				Capabilities: CapabilitiesConfig{
					Provides: []string{extensionprotocol.CapabilityProvideModelSource},
				},
			}, func() ModelSourceRuntime {
				return nil
			}),
			ctx: testutil.Context(t),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tt.source.ListModels(tt.ctx, modelcatalog.ListOptions{})
			if err == nil {
				t.Fatal("ListModels() error = nil, want invalid runtime state failure")
			}
		})
	}
}

func TestModelSourceShouldRejectMalformedRowsAndRecordSourceStatus(t *testing.T) {
	t.Parallel()

	t.Run("Should reject malformed rows and record source status", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 5, 7, 10, 30, 0, 0, time.UTC)
		store := openModelSourceTestStore(t)
		runtime := &fakeModelSourceRuntime{}
		source := newTestModelSource(t, "ext-malformed", runtime)
		runtime.rows = []extensioncontract.ModelSourceRow{
			{
				SourceID:   source.ID(),
				ProviderID: "codex",
			},
		}
		service := newTestModelCatalogService(t, store, []modelcatalog.Source{source})

		statuses, err := service.Refresh(ctx, modelcatalog.RefreshOptions{
			ProviderID: "codex",
			SourceID:   source.ID(),
			Force:      true,
			Now:        now,
		})
		if err == nil {
			t.Fatal("Refresh() error = nil, want malformed row failure")
		}
		if len(statuses) != 1 || statuses[0].RefreshState != modelcatalog.RefreshStateFailed {
			t.Fatalf("Refresh() statuses = %#v, want failed status", statuses)
		}
		if statuses[0].LastError == "" {
			t.Fatalf("Refresh() status LastError = empty, want malformed row error")
		}
	})
}

func TestModelSourceShouldRejectInvalidRowMetadata(t *testing.T) {
	t.Parallel()

	baseRuntime := &fakeModelSourceRuntime{}
	baseSource := newTestModelSource(t, "ext-row-validation", baseRuntime)
	sourceID := baseSource.ID()
	negativeInt := int64(-1)
	negativeCost := float64(-1)
	defaultEffort := apicontract.ReasoningEffort("medium")
	tests := []struct {
		name    string
		row     extensioncontract.ModelSourceRow
		opts    modelcatalog.ListOptions
		wantErr string
	}{
		{
			name: "Should reject provider outside requested filter",
			row: extensioncontract.ModelSourceRow{
				SourceID:   sourceID,
				ProviderID: "other",
				ModelID:    "model",
			},
			opts:    modelcatalog.ListOptions{ProviderID: "codex"},
			wantErr: `provider_id "other" is outside requested provider "codex"`,
		},
		{
			name: "Should reject non-extension priority",
			row: extensioncontract.ModelSourceRow{
				SourceID:   sourceID,
				ProviderID: "codex",
				ModelID:    "model",
				Priority:   modelcatalog.PriorityConfig,
			},
			wantErr: "priority 120 must equal 100",
		},
		{
			name: "Should reject negative token metadata",
			row: extensioncontract.ModelSourceRow{
				SourceID:      sourceID,
				ProviderID:    "codex",
				ModelID:       "model",
				ContextWindow: &negativeInt,
			},
			wantErr: "context_window must be non-negative",
		},
		{
			name: "Should reject negative cost metadata",
			row: extensioncontract.ModelSourceRow{
				SourceID:   sourceID,
				ProviderID: "codex",
				ModelID:    "model",
				Cost: &apicontract.ModelCatalogCostPayload{
					InputPerMillion: &negativeCost,
				},
			},
			wantErr: "cost.input_per_million must be finite and non-negative",
		},
		{
			name: "Should reject negative output cost metadata",
			row: extensioncontract.ModelSourceRow{
				SourceID:   sourceID,
				ProviderID: "codex",
				ModelID:    "model",
				Cost: &apicontract.ModelCatalogCostPayload{
					OutputPerMillion: &negativeCost,
				},
			},
			wantErr: "cost.output_per_million must be finite and non-negative",
		},
		{
			name: "Should reject non-finite reasoning cost metadata",
			row: extensioncontract.ModelSourceRow{
				SourceID:   sourceID,
				ProviderID: "codex",
				ModelID:    "model",
				Cost: &apicontract.ModelCatalogCostPayload{
					ReasoningPerMillion: new(math.NaN()),
				},
			},
			wantErr: "cost.reasoning_per_million must be finite and non-negative",
		},
		{
			name: "Should reject invalid release date metadata",
			row: extensioncontract.ModelSourceRow{
				SourceID:    sourceID,
				ProviderID:  "codex",
				ModelID:     "model",
				ReleaseDate: new("June 2026"),
			},
			wantErr: "release_date",
		},
		{
			name: "Should reject unsupported reasoning effort",
			row: extensioncontract.ModelSourceRow{
				SourceID:         sourceID,
				ProviderID:       "codex",
				ModelID:          "model",
				ReasoningEfforts: []apicontract.ReasoningEffort{"turbo"},
			},
			wantErr: `reasoning effort "turbo" is not supported`,
		},
		{
			name: "Should reject duplicate reasoning efforts",
			row: extensioncontract.ModelSourceRow{
				SourceID:         sourceID,
				ProviderID:       "codex",
				ModelID:          "model",
				ReasoningEfforts: []apicontract.ReasoningEffort{"high", "high"},
			},
			wantErr: `reasoning_efforts contains duplicate "high"`,
		},
		{
			name: "Should reject default effort outside advertised list",
			row: extensioncontract.ModelSourceRow{
				SourceID:               sourceID,
				ProviderID:             "codex",
				ModelID:                "model",
				ReasoningEfforts:       []apicontract.ReasoningEffort{"high"},
				DefaultReasoningEffort: &defaultEffort,
			},
			wantErr: `default_reasoning_effort "medium" is not in reasoning_efforts`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runtime := &fakeModelSourceRuntime{rows: []extensioncontract.ModelSourceRow{tt.row}}
			source := newTestModelSource(t, "ext-row-validation", runtime)
			_, err := source.ListModels(testutil.Context(t), tt.opts)
			if err == nil {
				t.Fatal("ListModels() error = nil, want row validation failure")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ListModels() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestModelSourceShouldRejectRowsWithInvalidSourceID(t *testing.T) {
	t.Parallel()

	t.Run("Should reject rows with invalid source id", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 5, 7, 10, 45, 0, 0, time.UTC)
		store := openModelSourceTestStore(t)
		runtime := &fakeModelSourceRuntime{}
		source := newTestModelSource(t, "ext-invalid-source", runtime)
		runtime.rows = []extensioncontract.ModelSourceRow{
			{
				SourceID:   "extension:Bad",
				ProviderID: "codex",
				ModelID:    "bad-source-model",
			},
		}
		service := newTestModelCatalogService(t, store, []modelcatalog.Source{source})

		statuses, err := service.Refresh(ctx, modelcatalog.RefreshOptions{
			ProviderID: "codex",
			SourceID:   source.ID(),
			Force:      true,
			Now:        now,
		})
		if err == nil {
			t.Fatal("Refresh() error = nil, want source_id validation failure")
		}
		if len(statuses) != 1 || statuses[0].LastError == "" {
			t.Fatalf("Refresh() statuses = %#v, want recorded source_id validation error", statuses)
		}
	})
}

func TestModelSourceShouldRecordMalformedSubprocessRows(t *testing.T) {
	t.Run("Should record malformed subprocess rows", func(t *testing.T) {
		withDaemonVersion(t, "0.5.0")

		ctx := testutil.Context(t)
		now := time.Date(2026, 5, 7, 10, 50, 0, 0, time.UTC)
		store, _, source := startSubprocessModelSource(t, "ext-subprocess-malformed", "model_source_malformed")
		service := newTestModelCatalogService(t, store, []modelcatalog.Source{source})

		statuses, err := service.Refresh(ctx, modelcatalog.RefreshOptions{
			ProviderID: "codex",
			SourceID:   source.ID(),
			Force:      true,
			Now:        now,
		})
		if err == nil {
			t.Fatal("Refresh() error = nil, want malformed subprocess row failure")
		}
		if len(statuses) != 1 || statuses[0].RefreshState != modelcatalog.RefreshStateFailed {
			t.Fatalf("Refresh() statuses = %#v, want failed subprocess source status", statuses)
		}
	})
}

func TestModelSourceShouldPreserveStaleRowsWhenRuntimeIsUnavailable(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve stale rows when runtime is unavailable", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 5, 7, 11, 0, 0, 0, time.UTC)
		store := openModelSourceTestStore(t)
		runtime := &fakeModelSourceRuntime{}
		source := newTestModelSource(t, "ext-stale", runtime)
		runtime.rows = []extensioncontract.ModelSourceRow{
			{
				SourceID:   source.ID(),
				ProviderID: "codex",
				ModelID:    "stale-model",
			},
		}
		service := newTestModelCatalogService(t, store, []modelcatalog.Source{source})
		if _, err := service.Refresh(ctx, modelcatalog.RefreshOptions{
			ProviderID: "codex",
			SourceID:   source.ID(),
			Force:      true,
			Now:        now,
		}); err != nil {
			t.Fatalf("initial Refresh() error = %v, want nil", err)
		}

		runtime.rows = nil
		runtime.err = errors.New("extension offline")
		statuses, err := service.Refresh(ctx, modelcatalog.RefreshOptions{
			ProviderID: "codex",
			SourceID:   source.ID(),
			Force:      true,
			Now:        now.Add(time.Minute),
		})
		if err == nil {
			t.Fatal("stale Refresh() error = nil, want degraded stale fallback error")
		}
		if len(statuses) != 1 || !statuses[0].Stale || statuses[0].RowCount != 1 {
			t.Fatalf("stale Refresh() statuses = %#v, want one stale preserved row", statuses)
		}
		models, err := service.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID:   "codex",
			IncludeStale: true,
			Now:          now.Add(time.Minute),
		})
		if err != nil {
			t.Fatalf("ListModels(include stale) error = %v, want nil", err)
		}
		if len(models) != 1 || !models[0].Stale || models[0].LastError == "" {
			t.Fatalf("ListModels(include stale) = %#v, want stale model with last error", models)
		}
	})
}

func TestModelSourceShouldPreserveStaleRowsWhenSubprocessExtensionStops(t *testing.T) {
	t.Run("Should preserve stale rows when subprocess extension stops", func(t *testing.T) {
		withDaemonVersion(t, "0.5.0")

		ctx := testutil.Context(t)
		now := time.Date(2026, 5, 7, 11, 15, 0, 0, time.UTC)
		store, manager, source := startSubprocessModelSource(t, "ext-subprocess-stale", "model_source_success")
		service := newTestModelCatalogService(t, store, []modelcatalog.Source{source})
		if _, err := service.Refresh(ctx, modelcatalog.RefreshOptions{
			ProviderID: "codex",
			SourceID:   source.ID(),
			Force:      true,
			Now:        now,
		}); err != nil {
			t.Fatalf("initial Refresh() error = %v, want nil", err)
		}
		if err := manager.Stop(ctx); err != nil {
			t.Fatalf("Stop() error = %v, want nil", err)
		}

		statuses, err := service.Refresh(ctx, modelcatalog.RefreshOptions{
			ProviderID: "codex",
			SourceID:   source.ID(),
			Force:      true,
			Now:        now.Add(time.Minute),
		})
		if err == nil {
			t.Fatal("stale Refresh() error = nil, want degraded stale fallback error")
		}
		if len(statuses) != 1 || !statuses[0].Stale || statuses[0].RowCount != 1 {
			t.Fatalf("stale Refresh() statuses = %#v, want stale preserved subprocess row", statuses)
		}
	})
}

func TestModelSourceShouldFailClosedWithoutBlockingCatalogList(t *testing.T) {
	t.Parallel()

	t.Run("Should fail closed without blocking catalog list", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 5, 7, 11, 30, 0, 0, time.UTC)
		store := openModelSourceTestStore(t)
		runtime := &fakeModelSourceRuntime{}
		deniedSource, err := NewExtensionModelSource(ExtensionInfo{
			Name:    "ext-denied",
			Enabled: true,
		}, func() ModelSourceRuntime {
			return runtime
		})
		if err != nil {
			t.Fatalf("NewExtensionModelSource() error = %v", err)
		}
		configSource := modelcatalog.NewConfigSource(map[string]aghconfig.ProviderConfig{
			"codex": {
				Models: aghconfig.ProviderModelsConfig{
					Curated: []aghconfig.ProviderModelConfig{{ID: "configured-model"}},
				},
			},
		})
		service := newTestModelCatalogService(t, store, []modelcatalog.Source{configSource, deniedSource})

		models, err := service.ListModels(ctx, modelcatalog.ListOptions{
			ProviderID:   "codex",
			IncludeStale: true,
			Now:          now,
		})
		if err != nil {
			t.Fatalf("ListModels() error = %v, want config source to remain available", err)
		}
		if len(models) != 1 || models[0].ModelID != "configured-model" {
			t.Fatalf("ListModels() = %#v, want config model despite denied extension source", models)
		}
		statuses, err := service.ListSourceStatus(ctx, "codex")
		if err != nil {
			t.Fatalf("ListSourceStatus() error = %v, want nil", err)
		}
		foundDenied := false
		for _, status := range statuses {
			if status.SourceID == deniedSource.ID() {
				foundDenied = status.RefreshState == modelcatalog.RefreshStateFailed && status.LastError != ""
			}
		}
		if !foundDenied {
			t.Fatalf("ListSourceStatus() = %#v, want failed denied extension source", statuses)
		}
	})
}

type fakeModelSourceRuntime struct {
	rows  []extensioncontract.ModelSourceRow
	err   error
	calls []extensioncontract.ModelSourceListParams
}

func (r *fakeModelSourceRuntime) ListModelSourceRows(
	_ context.Context,
	_ string,
	params extensioncontract.ModelSourceListParams,
) ([]extensioncontract.ModelSourceRow, error) {
	r.calls = append(r.calls, params)
	return cloneModelSourceRows(r.rows), r.err
}

func newTestModelSource(t *testing.T, name string, runtime *fakeModelSourceRuntime) *ModelSource {
	t.Helper()

	return mustTestModelSource(t, ExtensionInfo{
		Name:    name,
		Enabled: true,
		Capabilities: CapabilitiesConfig{
			Provides: []string{extensionprotocol.CapabilityProvideModelSource},
		},
	}, func() ModelSourceRuntime {
		return runtime
	})
}

func mustTestModelSource(
	t *testing.T,
	info ExtensionInfo,
	resolver ModelSourceRuntimeResolver,
) *ModelSource {
	t.Helper()

	source, err := NewExtensionModelSource(info, resolver)
	if err != nil {
		t.Fatalf("NewExtensionModelSource() error = %v", err)
	}
	return source
}

func openModelSourceTestStore(t *testing.T) *globaldb.GlobalDB {
	t.Helper()

	store, err := globaldb.OpenGlobalDB(testutil.Context(t), filepath.Join(t.TempDir(), "agh.db"))
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(testutil.Context(t)); err != nil {
			t.Fatalf("GlobalDB.Close() error = %v", err)
		}
	})
	return store
}

func newTestModelCatalogService(
	t *testing.T,
	store modelcatalog.Store,
	sources []modelcatalog.Source,
) modelcatalog.Service {
	t.Helper()

	service, err := modelcatalog.NewService(store, sources, modelcatalog.MergeOptions{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func startSubprocessModelSource(
	t *testing.T,
	name string,
	scenario string,
) (*globaldb.GlobalDB, *Manager, *ModelSource) {
	t.Helper()

	store := openModelSourceTestStore(t)
	registry := NewRegistry(store.DB())
	fixture := createManagerTestExtension(t, managerTestManifest(name, managerManifestOptions{
		command:      helperCommand(t),
		args:         helperArgs(),
		withEnv:      helperEnv(scenario, ""),
		capabilities: []string{extensionprotocol.CapabilityProvideModelSource},
	}), nil)
	installManagerFixture(t, registry, fixture, SourceUser, true)

	manager := NewManager(registry)
	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	info, err := registry.Get(name)
	if err != nil {
		t.Fatalf("Registry.Get(%q) error = %v", name, err)
	}
	source, err := NewExtensionModelSource(*info, func() ModelSourceRuntime {
		return manager
	})
	if err != nil {
		t.Fatalf("NewExtensionModelSource() error = %v", err)
	}
	return store, manager, source
}

package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/config/lifecycle"
	"github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/modelcatalog"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/session"
	settingspkg "github.com/compozy/compozy/internal/settings"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
)

func TestBaseHandlersModelCatalogDependency(t *testing.T) {
	t.Parallel()

	t.Run("Should carry model catalog service from config", func(t *testing.T) {
		t.Parallel()

		service := coreModelCatalogServiceStub{}
		handlers := NewBaseHandlers(&BaseHandlerConfig{ModelCatalog: service})
		if handlers.ModelCatalog == nil {
			t.Fatal("NewBaseHandlers() ModelCatalog = nil, want injected service")
		}
		if handlers.ModelCatalog != service {
			t.Fatalf("NewBaseHandlers() ModelCatalog = %#v, want %#v", handlers.ModelCatalog, service)
		}
	})
}

func TestProviderModelPayloadConversion(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve nullable availability and source stale fields", func(t *testing.T) {
		t.Parallel()

		effort := modelcatalog.ReasoningEffortHigh
		fast := true
		releaseDate := "2026-06-26"
		model := modelcatalog.Model{
			ProviderID:             "codex",
			ModelID:                "gpt-5.4",
			DisplayName:            "GPT-5.4",
			Available:              nil,
			AvailabilityState:      modelcatalog.AvailabilityStateUnknown,
			Stale:                  true,
			RefreshedAt:            time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC),
			SupportsReasoning:      new(true),
			ReasoningEfforts:       []modelcatalog.ReasoningEffort{modelcatalog.ReasoningEffortHigh},
			DefaultReasoningEffort: &effort,
			ConfigOptions: []modelcatalog.ModelOptionDescriptor{{
				ID: "thinking", Label: "Thinking", Kind: modelcatalog.ModelOptionKindBoolean,
				CurrentBool: new(false),
			}},
			TransportBindings: []modelcatalog.ModelTransportBinding{
				{
					TransportModelID: "private-provider-alias",
					ReasoningEffort:  &effort,
					Fast:             &fast,
				},
			},
			Curated:         true,
			Featured:        true,
			ReleaseDate:     &releaseDate,
			ReasoningSource: modelcatalog.ReasoningSourceACP,
			Sources: []modelcatalog.SourceRef{
				{
					SourceID:    modelcatalog.SourceIDConfig,
					SourceKind:  modelcatalog.SourceKindConfig,
					Priority:    modelcatalog.PriorityConfig,
					RefreshedAt: time.Date(2026, 5, 7, 11, 0, 0, 0, time.UTC),
					Stale:       true,
					LastError:   "cached provider config",
				},
			},
		}

		payload := ProviderModelPayloadFromModel(model)
		if payload.Available != nil {
			t.Fatalf("Available = %#v, want nil", payload.Available)
		}
		if !payload.Stale || len(payload.Sources) != 1 || !payload.Sources[0].Stale {
			t.Fatalf("Payload = %#v, want stale model and source", payload)
		}
		if payload.DefaultReasoningEffort == nil || *payload.DefaultReasoningEffort != "high" {
			t.Fatalf("DefaultReasoningEffort = %#v, want high", payload.DefaultReasoningEffort)
		}
		if len(payload.Configurations) != 1 || payload.Configurations[0].ReasoningEffort == nil ||
			*payload.Configurations[0].ReasoningEffort != contract.ReasoningEffort("high") ||
			payload.Configurations[0].Fast == nil || !*payload.Configurations[0].Fast {
			t.Fatalf("Configurations = %#v, want high fast", payload.Configurations)
		}
		if len(payload.ConfigOptions) != 1 || payload.ConfigOptions[0].ID != "thinking" ||
			payload.ConfigOptions[0].Kind != "boolean" || payload.ConfigOptions[0].CurrentBool == nil ||
			*payload.ConfigOptions[0].CurrentBool {
			t.Fatalf("ConfigOptions = %#v, want public thinking=false descriptor", payload.ConfigOptions)
		}
		if !payload.Curated || !payload.Featured || payload.ReleaseDate != releaseDate ||
			payload.ReasoningSource != modelcatalog.ReasoningSourceACP {
			t.Fatalf("curation/reasoning metadata = %#v, want curated featured ACP model", payload)
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal(payload) error = %v", err)
		}
		if !strings.Contains(string(encoded), `"available":null`) {
			t.Fatalf("payload JSON = %s, want nullable available field", encoded)
		}
		if strings.Contains(string(encoded), "private-provider-alias") ||
			strings.Contains(string(encoded), "transport_model_id") {
			t.Fatalf("payload JSON leaked private transport binding: %s", encoded)
		}
	})

	t.Run("Should redact source errors in native and OpenAI projections", func(t *testing.T) {
		t.Parallel()

		model := seedModelCatalogModel("codex", "gpt-5.4")
		model.LastError = "provider failed with api_key=sk-native-secret-token"
		model.Sources[0].LastError = "source failed with OAUTH_TOKEN=oauth-secret-token"

		nativePayload := ProviderModelPayloadFromModel(model)
		assertRedactedModelCatalogPayload(t, nativePayload.LastError, "sk-native-secret-token")
		assertRedactedModelCatalogPayload(t, nativePayload.Sources[0].LastError, "oauth-secret-token")

		openAIPayload := OpenAIModelPayloadFromModel(model)
		assertRedactedModelCatalogPayload(t, openAIPayload.Compozy.LastError, "sk-native-secret-token")

		statusPayloads := SourceStatusPayloadsFromStatuses([]modelcatalog.SourceStatus{
			{
				SourceID:     modelcatalog.SourceIDModelsDev,
				SourceKind:   modelcatalog.SourceKindModelsDev,
				ProviderID:   "codex",
				RefreshState: modelcatalog.RefreshStateFailed,
				LastError:    "models.dev failed with Bearer ya29.api-secret-token",
			},
		})
		if got, want := len(statusPayloads), 1; got != want {
			t.Fatalf("len(statusPayloads) = %d, want %d", got, want)
		}
		assertRedactedModelCatalogPayload(t, statusPayloads[0].LastError, "ya29.api-secret-token")
	})

	t.Run("Should preserve five-rate cost payloads in native and OpenAI projections", func(t *testing.T) {
		t.Parallel()

		input := 1.0
		output := 2.0
		cacheRead := 0.5
		cacheWrite := 3.0
		reasoning := 4.0
		model := seedModelCatalogModel("codex", "gpt-5.4")
		model.CostInputPerMillion = &input
		model.CostOutputPerMillion = &output
		model.CostCacheReadPerMillion = &cacheRead
		model.CostCacheWritePerMillion = &cacheWrite
		model.CostReasoningPerMillion = &reasoning

		nativeCost := ProviderModelPayloadFromModel(model).Cost
		openAICost := OpenAIModelPayloadFromModel(model).Compozy.Cost
		for name, cost := range map[string]*contract.ModelCatalogCostPayload{
			"native": nativeCost,
			"openai": openAICost,
		} {
			if cost == nil || cost.InputPerMillion != &input || cost.OutputPerMillion != &output ||
				cost.CacheReadPerMillion != &cacheRead || cost.CacheWritePerMillion != &cacheWrite ||
				cost.ReasoningPerMillion != &reasoning {
				t.Fatalf("%s five-rate cost = %#v, want complete payload", name, cost)
			}
		}
	})
}

func TestProviderModelCatalogHandlers(t *testing.T) {
	t.Parallel()

	t.Run("Should pass list filters and return native model payload", func(t *testing.T) {
		t.Parallel()

		service := &modelCatalogServiceSpy{
			listModelsFn: func(_ context.Context, opts modelcatalog.ListOptions) ([]modelcatalog.Model, error) {
				if got, want := opts.ProviderID, "codex"; got != want {
					t.Fatalf("ProviderID = %q, want %q", got, want)
				}
				if got, want := opts.SourceID, modelcatalog.SourceIDConfig; got != want {
					t.Fatalf("SourceID = %q, want %q", got, want)
				}
				if !opts.Refresh || !opts.IncludeStale {
					t.Fatalf("ListOptions = %#v, want refresh and include_stale", opts)
				}
				if got, want := opts.View, modelcatalog.CatalogViewCurated; got != want {
					t.Fatalf("View = %q, want %q", got, want)
				}
				if opts.ExecutionContext.Scope != modelcatalog.ExecutionScopeProfile ||
					opts.ExecutionContext.ProfileID != store.DefaultProfileID {
					t.Fatalf("ExecutionContext = %#v, want default profile scope", opts.ExecutionContext)
				}
				return []modelcatalog.Model{seedModelCatalogModel("codex", "gpt-5.4")}, nil
			},
		}
		engine := newModelCatalogCoreEngine(t, service)

		recorder := performModelCatalogRequest(
			t,
			engine,
			http.MethodGet,
			"/model-catalog/providers/codex/models?source_id=config&refresh=true&include_stale=true",
			nil,
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
		var payload contract.ProviderModelListResponse
		decodeModelCatalogResponse(t, recorder, &payload)
		if len(payload.Models) != 1 || payload.Models[0].ProviderID != "codex" {
			t.Fatalf("payload = %#v, want codex model", payload)
		}
	})

	t.Run("Should resolve a workspace alias to its canonical catalog context", func(t *testing.T) {
		t.Parallel()

		service := &modelCatalogServiceSpy{
			listModelsFn: func(_ context.Context, opts modelcatalog.ListOptions) ([]modelcatalog.Model, error) {
				if opts.ExecutionContext.Scope != modelcatalog.ExecutionScopeWorkspace ||
					opts.ExecutionContext.ProfileID != store.DefaultProfileID ||
					opts.ExecutionContext.WorkspaceID != "ws-canonical" {
					t.Fatalf("ExecutionContext = %#v, want default profile in ws-canonical", opts.ExecutionContext)
				}
				return nil, nil
			},
		}
		engine := newModelCatalogCoreEngineWithConfig(t, &BaseHandlerConfig{
			ModelCatalog: service,
			Workspaces: workspaceResolveServiceStub{resolve: func(
				_ context.Context,
				ref string,
			) (workspacepkg.ResolvedWorkspace, error) {
				if ref != "alpha" {
					t.Fatalf("Resolve() ref = %q, want alpha", ref)
				}
				return workspacepkg.ResolvedWorkspace{Workspace: workspacepkg.Workspace{ID: "ws-canonical"}}, nil
			}},
		})
		recorder := performModelCatalogRequest(
			t,
			engine,
			http.MethodGet,
			"/model-catalog/providers/codex/models?workspace_id=alpha",
			nil,
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("Should resolve an explicit profile catalog context", func(t *testing.T) {
		t.Parallel()

		service := &modelCatalogServiceSpy{
			listModelsFn: func(_ context.Context, opts modelcatalog.ListOptions) ([]modelcatalog.Model, error) {
				if opts.ExecutionContext.Scope != modelcatalog.ExecutionScopeProfile ||
					opts.ExecutionContext.ProfileID != "profile-marketing" {
					t.Fatalf("ExecutionContext = %#v, want marketing profile scope", opts.ExecutionContext)
				}
				return nil, nil
			},
		}
		engine := newModelCatalogCoreEngineWithConfig(t, &BaseHandlerConfig{
			ModelCatalog: service,
			Profiles:     modelCatalogProfileServiceStub{},
		})
		recorder := performModelCatalogRequest(
			t,
			engine,
			http.MethodGet,
			"/model-catalog/providers/codex/models?profile=marketing",
			nil,
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("Should reject an aggregate profile catalog context", func(t *testing.T) {
		t.Parallel()

		service := &modelCatalogServiceSpy{
			listModelsFn: func(context.Context, modelcatalog.ListOptions) ([]modelcatalog.Model, error) {
				t.Fatal("ListModels() called for all_profiles catalog request")
				return nil, nil
			},
		}
		engine := newModelCatalogCoreEngine(t, service)
		recorder := performModelCatalogRequest(
			t,
			engine,
			http.MethodGet,
			"/model-catalog/providers/codex/models?all_profiles=true",
			nil,
		)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("Should deny an agent catalog read across workspaces", func(t *testing.T) {
		t.Parallel()

		engine := newModelCatalogCoreEngineWithConfig(t, &BaseHandlerConfig{
			ModelCatalog: &modelCatalogServiceSpy{},
			Sessions: modelCatalogSessionManagerStub{info: &session.Info{
				ID: "sess-agent", ProfileID: "profile-agent", AgentName: "coder",
				WorkspaceID: "ws-origin", State: session.StateActive,
			}},
			Workspaces: workspaceResolveServiceStub{resolve: func(
				_ context.Context,
				_ string,
			) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{Workspace: workspacepkg.Workspace{ID: "ws-foreign"}}, nil
			}},
			WorkspaceAccess: modelCatalogWorkspaceAccessStub{authorize: func(
				_ context.Context,
				req workspaceaccess.Request,
			) (workspaceaccess.Decision, error) {
				if req.Seam == workspaceaccess.SeamCatalog {
					return workspaceaccess.Decision{Source: workspaceaccess.SourceDenied}, nil
				}
				return workspaceaccess.Decision{Allowed: true, Source: workspaceaccess.SourceSameWorkspace}, nil
			}},
		})
		recorder := performModelCatalogRequestWithHeaders(
			t,
			engine,
			http.MethodGet,
			"/model-catalog/providers/codex/models?workspace_id=foreign",
			map[string]string{
				agentidentity.HeaderSessionID: "sess-agent",
				agentidentity.HeaderAgent:     "coder",
			},
		)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body=%s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("Should default to curated and expose the all view on demand", func(t *testing.T) {
		t.Parallel()

		service := &modelCatalogServiceSpy{
			listModelsFn: func(_ context.Context, opts modelcatalog.ListOptions) ([]modelcatalog.Model, error) {
				models := []modelcatalog.Model{seedModelCatalogModel("codex", "gpt-5.6-sol")}
				if opts.View == modelcatalog.CatalogViewAll {
					models = append(models, seedModelCatalogModel("codex", "gpt-5.6-luna"))
				}
				return models, nil
			},
		}
		engine := newModelCatalogCoreEngine(t, service)
		curatedRecorder := performModelCatalogRequest(
			t,
			engine,
			http.MethodGet,
			"/model-catalog/providers/codex/models",
			nil,
		)
		allRecorder := performModelCatalogRequest(
			t,
			engine,
			http.MethodGet,
			"/model-catalog/providers/codex/models?view=all",
			nil,
		)
		if curatedRecorder.Code != http.StatusOK || allRecorder.Code != http.StatusOK {
			t.Fatalf("catalog statuses = curated:%d all:%d", curatedRecorder.Code, allRecorder.Code)
		}
		var curated contract.ProviderModelListResponse
		var all contract.ProviderModelListResponse
		decodeModelCatalogResponse(t, curatedRecorder, &curated)
		decodeModelCatalogResponse(t, allRecorder, &all)
		if got, want := len(curated.Models), 1; got != want {
			t.Fatalf("curated model count = %d, want %d", got, want)
		}
		if got, want := len(all.Models), 2; got != want {
			t.Fatalf("all model count = %d, want %d", got, want)
		}
	})

	t.Run("Should reject an invalid catalog view", func(t *testing.T) {
		t.Parallel()

		engine := newModelCatalogCoreEngine(t, &modelCatalogServiceSpy{})
		recorder := performModelCatalogRequest(
			t,
			engine,
			http.MethodGet,
			"/model-catalog/providers/codex/models?view=hidden-only",
			nil,
		)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
		}
		var payload contract.ErrorPayload
		decodeModelCatalogResponse(t, recorder, &payload)
		if !strings.Contains(payload.Error, "expected curated or all") {
			t.Fatalf("Error = %q, want curated/all validation", payload.Error)
		}
	})

	t.Run("Should return deterministic validation error for invalid provider id", func(t *testing.T) {
		t.Parallel()

		engine := newModelCatalogCoreEngine(t, &modelCatalogServiceSpy{})

		recorder := performModelCatalogRequest(
			t,
			engine,
			http.MethodGet,
			"/model-catalog/providers/bad%20id/models",
			nil,
		)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
		}
		var payload contract.ErrorPayload
		decodeModelCatalogResponse(t, recorder, &payload)
		if !strings.Contains(payload.Error, "provider_id") {
			t.Fatalf("Error = %q, want provider_id validation message", payload.Error)
		}
	})

	t.Run("Should return source statuses when refresh fails", func(t *testing.T) {
		t.Parallel()

		secret := "sk-refresh-secret-token"
		service := &modelCatalogServiceSpy{
			refreshFn: func(_ context.Context, _ modelcatalog.RefreshOptions) ([]modelcatalog.SourceStatus, error) {
				return []modelcatalog.SourceStatus{
					{
						SourceID:     modelcatalog.SourceIDConfig,
						SourceKind:   modelcatalog.SourceKindConfig,
						ProviderID:   "codex",
						RefreshState: modelcatalog.RefreshStateFailed,
						LastError:    "config source failed with api_key=" + secret,
						Stale:        true,
					},
				}, fmt.Errorf("%w: api_key=%s", modelcatalog.ErrAllSourcesFailed, secret)
			},
		}
		engine := newModelCatalogCoreEngine(t, service)

		recorder := performModelCatalogRequest(
			t,
			engine,
			http.MethodPost,
			"/model-catalog/providers/codex/models/refresh",
			nil,
		)
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503; body=%s", recorder.Code, recorder.Body.String())
		}
		var payload contract.ProviderModelRefreshResponse
		decodeModelCatalogResponse(t, recorder, &payload)
		if len(payload.Sources) != 1 || payload.Sources[0].RefreshState != string(modelcatalog.RefreshStateFailed) {
			t.Fatalf("payload = %#v, want failed source status", payload)
		}
		if payload.Error == "" {
			t.Fatalf("payload.Error = empty, want refresh error")
		}
		assertRedactedModelCatalogPayload(t, payload.Error, secret)
		assertRedactedModelCatalogPayload(t, payload.Sources[0].LastError, secret)
	})

	t.Run("Should apply curation and return the effective model with live apply metadata", func(t *testing.T) {
		t.Parallel()

		hidden := true
		defaultEffort := contract.ReasoningEffort("max")
		defaultSpeed := contract.Speed("fast")
		settingsService := &modelCatalogSettingsServiceStub{
			applyCurationFn: func(
				_ context.Context,
				req settingspkg.ProviderModelCurationRequest,
			) (settingspkg.ProviderModelCurationResult, error) {
				if req.ProviderID != "codex" || req.ModelID != "gpt-5.6-sol" {
					t.Fatalf("curation request identity = %q/%q", req.ProviderID, req.ModelID)
				}
				if req.Hidden == nil || !*req.Hidden || req.DefaultReasoningEffort == nil ||
					*req.DefaultReasoningEffort != "max" || req.DefaultSpeed == nil ||
					*req.DefaultSpeed != speedpkg.SpeedFast {
					t.Fatalf("curation request = %#v, want hidden, max, and fast", req)
				}
				model := seedModelCatalogModel("codex", "gpt-5.6-sol")
				model.Hidden = true
				model.DefaultReasoningEffort = new(modelcatalog.ReasoningEffortMax)
				return settingspkg.ProviderModelCurationResult{
					Model:        model,
					DefaultSpeed: speedpkg.SpeedFast,
					Apply: settingspkg.ApplyResult{
						Applied: true,
						Record: settingspkg.ApplyRecord{
							ID:         "cfgapp-curate",
							Lifecycle:  lifecycle.Live,
							Generation: 4,
							ActiveHash: "sha256:curated",
						},
					},
				}, nil
			},
		}
		engine := newModelCatalogCoreEngineWithSettings(t, &modelCatalogServiceSpy{}, settingsService)
		body, err := json.Marshal(contract.ProviderModelCurationRequest{
			ModelID:                "gpt-5.6-sol",
			Hidden:                 &hidden,
			DefaultReasoningEffort: &defaultEffort,
			DefaultSpeed:           &defaultSpeed,
		})
		if err != nil {
			t.Fatalf("json.Marshal(curation request) error = %v", err)
		}
		recorder := performModelCatalogRequest(
			t,
			engine,
			http.MethodPost,
			"/model-catalog/providers/codex/models/curate",
			body,
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
		var payload contract.ProviderModelCurationResponse
		decodeModelCatalogResponse(t, recorder, &payload)
		if !payload.Model.Hidden || payload.Model.DefaultReasoningEffort == nil ||
			*payload.Model.DefaultReasoningEffort != "max" || payload.DefaultSpeed != contract.Speed("fast") {
			t.Fatalf("curated model payload = %#v, want hidden, max, and fast", payload)
		}
		if !payload.Apply.Applied || payload.Apply.Lifecycle != contract.SettingsApplyLifecycle(lifecycle.Live) ||
			payload.Apply.ActiveGeneration != 4 {
			t.Fatalf("apply payload = %#v, want live generation 4", payload.Apply)
		}
	})

	t.Run("Should preserve model_not_found in a 422 response body", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("provider model codex/missing was not found")
		item := diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            "provider.models.model_not_found",
			Code:          diagnosticcontract.CodeModelNotFound,
			Category:      diagnosticcontract.CategoryProvider,
			Title:         "Provider model not found",
			Message:       cause.Error(),
			Severity:      diagnosticcontract.SeverityError,
			DataFreshness: diagnosticcontract.FreshnessLive,
		})
		settingsService := &modelCatalogSettingsServiceStub{
			applyCurationFn: func(
				context.Context,
				settingspkg.ProviderModelCurationRequest,
			) (settingspkg.ProviderModelCurationResult, error) {
				return settingspkg.ProviderModelCurationResult{}, diagnostics.NewStructuredError(item, cause)
			},
		}
		engine := newModelCatalogCoreEngineWithSettings(t, &modelCatalogServiceSpy{}, settingsService)
		recorder := performModelCatalogRequest(
			t,
			engine,
			http.MethodPost,
			"/model-catalog/providers/codex/models/curate",
			[]byte(`{"model_id":"missing"}`),
		)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422; body=%s", recorder.Code, recorder.Body.String())
		}
		var payload contract.ErrorPayload
		decodeModelCatalogResponse(t, recorder, &payload)
		if payload.Diagnostic == nil || payload.Diagnostic.Code != diagnosticcontract.CodeModelNotFound {
			t.Fatalf("diagnostic = %#v, want model_not_found", payload.Diagnostic)
		}
	})
}

func TestOpenAIModelCatalogHandler(t *testing.T) {
	t.Parallel()

	t.Run("Should use Compozy metadata and provider filter", func(t *testing.T) {
		t.Parallel()

		service := &modelCatalogServiceSpy{
			listModelsFn: func(_ context.Context, opts modelcatalog.ListOptions) ([]modelcatalog.Model, error) {
				if got, want := opts.ProviderID, "codex"; got != want {
					t.Fatalf("ProviderID = %q, want %q", got, want)
				}
				return []modelcatalog.Model{seedModelCatalogModel("codex", "gpt-5.4")}, nil
			},
		}
		engine := newModelCatalogCoreEngine(t, service)

		recorder := performModelCatalogRequest(t, engine, http.MethodGet, "/openai/v1/models?provider_id=codex", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
		var payload contract.OpenAIModelListResponse
		decodeModelCatalogResponse(t, recorder, &payload)
		if payload.Object != "list" || len(payload.Data) != 1 {
			t.Fatalf("payload = %#v, want one OpenAI model list item", payload)
		}
		model := payload.Data[0]
		if model.Object != "model" || model.OwnedBy != "codex" || model.Compozy.ProviderID != "codex" {
			t.Fatalf("model = %#v, want OpenAI shape with compozy metadata", model)
		}
		if len(model.Compozy.Sources) != 1 || model.Compozy.Sources[0] != modelcatalog.SourceIDConfig {
			t.Fatalf("Compozy.Sources = %#v, want config source", model.Compozy.Sources)
		}
	})

	t.Run("Should return OpenAI shaped validation errors", func(t *testing.T) {
		t.Parallel()

		engine := newModelCatalogCoreEngine(t, &modelCatalogServiceSpy{})

		recorder := performModelCatalogRequest(t, engine, http.MethodGet, "/openai/v1/models?provider_id=bad%20id", nil)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
		}
		var payload contract.OpenAIErrorResponse
		decodeModelCatalogResponse(t, recorder, &payload)
		if payload.Error.Code != "invalid_request" || payload.Error.Type != "invalid_request_error" {
			t.Fatalf("error = %#v, want OpenAI invalid_request error", payload.Error)
		}
	})
}

type coreModelCatalogServiceStub struct{}

func (coreModelCatalogServiceStub) ListModels(
	context.Context,
	modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	return nil, nil
}

func (coreModelCatalogServiceStub) Refresh(
	context.Context,
	modelcatalog.RefreshOptions,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}

func (coreModelCatalogServiceStub) ListSourceStatus(
	context.Context,
	modelcatalog.StatusOptions,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}

type modelCatalogServiceSpy struct {
	listModelsFn       func(context.Context, modelcatalog.ListOptions) ([]modelcatalog.Model, error)
	refreshFn          func(context.Context, modelcatalog.RefreshOptions) ([]modelcatalog.SourceStatus, error)
	listSourceStatusFn func(context.Context, modelcatalog.StatusOptions) ([]modelcatalog.SourceStatus, error)
}

func (s *modelCatalogServiceSpy) ListModels(
	ctx context.Context,
	opts modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	if s.listModelsFn != nil {
		return s.listModelsFn(ctx, opts)
	}
	return nil, errors.New("unexpected ListModels call")
}

func (s *modelCatalogServiceSpy) Refresh(
	ctx context.Context,
	opts modelcatalog.RefreshOptions,
) ([]modelcatalog.SourceStatus, error) {
	if s.refreshFn != nil {
		return s.refreshFn(ctx, opts)
	}
	return nil, errors.New("unexpected Refresh call")
}

func (s *modelCatalogServiceSpy) ListSourceStatus(
	ctx context.Context,
	opts modelcatalog.StatusOptions,
) ([]modelcatalog.SourceStatus, error) {
	if s.listSourceStatusFn != nil {
		return s.listSourceStatusFn(ctx, opts)
	}
	return nil, errors.New("unexpected ListSourceStatus call")
}

func newModelCatalogCoreEngine(t *testing.T, service ModelCatalogService) *gin.Engine {
	return newModelCatalogCoreEngineWithSettings(t, service, nil)
}

func newModelCatalogCoreEngineWithSettings(
	t *testing.T,
	service ModelCatalogService,
	settings SettingsService,
) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	return newModelCatalogCoreEngineWithConfig(t, &BaseHandlerConfig{
		ModelCatalog:  service,
		Settings:      settings,
		TransportName: "test",
		Now: func() time.Time {
			return time.Date(2026, 5, 7, 12, 30, 0, 0, time.UTC)
		},
	})
}

func newModelCatalogCoreEngineWithConfig(t *testing.T, cfg *BaseHandlerConfig) *gin.Engine {
	t.Helper()
	if cfg == nil {
		t.Fatal("BaseHandlerConfig is required")
	}
	if cfg.TransportName == "" {
		cfg.TransportName = "test"
	}
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Date(2026, 5, 7, 12, 30, 0, 0, time.UTC) }
	}
	handlers := NewBaseHandlers(cfg)
	engine := gin.New()
	engine.GET("/model-catalog/*catalog_path", handlers.ModelCatalogRoute)
	engine.POST("/model-catalog/*catalog_path", handlers.ModelCatalogRoute)
	engine.GET("/openai/v1/models", handlers.OpenAIModels)
	return engine
}

type modelCatalogSessionManagerStub struct {
	SessionManager
	info *session.Info
}

type modelCatalogProfileServiceStub struct {
	ProfileService
}

func (modelCatalogProfileServiceStub) Resolve(
	_ context.Context,
	input profilepkg.ResolveInput,
) (profilepkg.Resolution, error) {
	if input.Flag != "marketing" {
		return profilepkg.Resolution{}, fmt.Errorf("unexpected profile %q", input.Flag)
	}
	return profilepkg.Resolution{
		Profile: profilepkg.Profile{ID: "profile-marketing", Name: "marketing", State: profilepkg.StateActive},
		Source:  profilepkg.ResolutionSourceFlag,
	}, nil
}

type modelCatalogWorkspaceAccessStub struct {
	authorize func(context.Context, workspaceaccess.Request) (workspaceaccess.Decision, error)
}

func (s modelCatalogWorkspaceAccessStub) Authorize(
	ctx context.Context,
	req workspaceaccess.Request,
) (workspaceaccess.Decision, error) {
	return s.authorize(ctx, req)
}

func (s modelCatalogSessionManagerStub) Status(context.Context, string) (*session.Info, error) {
	if s.info == nil {
		return nil, session.ErrSessionNotFound
	}
	return s.info, nil
}

type modelCatalogSettingsServiceStub struct {
	SettingsService
	applyCurationFn func(
		context.Context,
		settingspkg.ProviderModelCurationRequest,
	) (settingspkg.ProviderModelCurationResult, error)
}

func (s *modelCatalogSettingsServiceStub) ApplyProviderModelCuration(
	ctx context.Context,
	req settingspkg.ProviderModelCurationRequest,
) (settingspkg.ProviderModelCurationResult, error) {
	if s.applyCurationFn == nil {
		return settingspkg.ProviderModelCurationResult{}, errors.New("unexpected ApplyProviderModelCuration call")
	}
	return s.applyCurationFn(ctx, req)
}

func performModelCatalogRequest(
	t *testing.T,
	engine http.Handler,
	method string,
	path string,
	body []byte,
) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(string(body)))
	engine.ServeHTTP(recorder, req)
	return recorder
}

func performModelCatalogRequestWithHeaders(
	t *testing.T,
	engine http.Handler,
	method string,
	path string,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), method, path, nil)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	engine.ServeHTTP(recorder, req)
	return recorder
}

func decodeModelCatalogResponse(t *testing.T, recorder *httptest.ResponseRecorder, dest any) {
	t.Helper()

	if err := json.Unmarshal(recorder.Body.Bytes(), dest); err != nil {
		t.Fatalf("json.Unmarshal(response) error = %v; body=%s", err, recorder.Body.String())
	}
}

func seedModelCatalogModel(providerID string, modelID string) modelcatalog.Model {
	available := true
	return modelcatalog.Model{
		ProviderID:        providerID,
		ModelID:           modelID,
		DisplayName:       "GPT-5.4",
		Available:         &available,
		AvailabilityState: modelcatalog.AvailabilityStateAvailableLive,
		Sources: []modelcatalog.SourceRef{
			{
				SourceID:   modelcatalog.SourceIDConfig,
				SourceKind: modelcatalog.SourceKindConfig,
				Priority:   modelcatalog.PriorityConfig,
			},
		},
	}
}

func assertRedactedModelCatalogPayload(t *testing.T, value string, secret string) {
	t.Helper()

	if strings.Contains(value, secret) {
		t.Fatalf("payload value = %q, want secret redacted", value)
	}
	if !strings.Contains(value, "[REDACTED]") {
		t.Fatalf("payload value = %q, want redaction marker", value)
	}
}

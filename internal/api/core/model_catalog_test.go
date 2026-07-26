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

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/config/lifecycle"
	"github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/modelcatalog"
	settingspkg "github.com/compozy/agh/internal/settings"
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
			Curated:                true,
			Featured:               true,
			ReleaseDate:            &releaseDate,
			ReasoningSource:        modelcatalog.ReasoningSourceACP,
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
		assertRedactedModelCatalogPayload(t, openAIPayload.AGH.LastError, "sk-native-secret-token")

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
		openAICost := OpenAIModelPayloadFromModel(model).AGH.Cost
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
		settingsService := &modelCatalogSettingsServiceStub{
			applyCurationFn: func(
				_ context.Context,
				req settingspkg.ProviderModelCurationRequest,
			) (settingspkg.ProviderModelCurationResult, error) {
				if req.ProviderID != "codex" || req.ModelID != "gpt-5.6-sol" {
					t.Fatalf("curation request identity = %q/%q", req.ProviderID, req.ModelID)
				}
				if req.Hidden == nil || !*req.Hidden || req.DefaultReasoningEffort == nil ||
					*req.DefaultReasoningEffort != "max" {
					t.Fatalf("curation request = %#v, want hidden and max", req)
				}
				model := seedModelCatalogModel("codex", "gpt-5.6-sol")
				model.Hidden = true
				model.DefaultReasoningEffort = new(modelcatalog.ReasoningEffortMax)
				return settingspkg.ProviderModelCurationResult{
					Model: model,
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
			*payload.Model.DefaultReasoningEffort != "max" {
			t.Fatalf("curated model payload = %#v, want hidden max", payload.Model)
		}
		if !payload.Apply.Applied || payload.Apply.Lifecycle != contract.SettingsApplyLifecycle(lifecycle.Live) ||
			payload.Apply.ActiveGeneration != 4 {
			t.Fatalf("apply payload = %#v, want live generation 4", payload.Apply)
		}
	})

	t.Run("Should preserve model_not_found in a 422 response body", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("provider model codex/missing was not found")
		item := diagnostics.NewItem(
			"provider.models.model_not_found",
			diagnosticcontract.CodeModelNotFound,
			diagnosticcontract.CategoryProvider,
			"Provider model not found",
			cause.Error(),
			diagnosticcontract.SeverityError,
			diagnosticcontract.FreshnessLive,
		)
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

	t.Run("Should use AGH metadata and provider filter", func(t *testing.T) {
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
		if model.Object != "model" || model.OwnedBy != "codex" || model.AGH.ProviderID != "codex" {
			t.Fatalf("model = %#v, want OpenAI shape with agh metadata", model)
		}
		if len(model.AGH.Sources) != 1 || model.AGH.Sources[0] != modelcatalog.SourceIDConfig {
			t.Fatalf("AGH.Sources = %#v, want config source", model.AGH.Sources)
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
	string,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}

type modelCatalogServiceSpy struct {
	listModelsFn       func(context.Context, modelcatalog.ListOptions) ([]modelcatalog.Model, error)
	refreshFn          func(context.Context, modelcatalog.RefreshOptions) ([]modelcatalog.SourceStatus, error)
	listSourceStatusFn func(context.Context, string) ([]modelcatalog.SourceStatus, error)
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
	providerID string,
) ([]modelcatalog.SourceStatus, error) {
	if s.listSourceStatusFn != nil {
		return s.listSourceStatusFn(ctx, providerID)
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
	handlers := NewBaseHandlers(&BaseHandlerConfig{
		ModelCatalog:  service,
		Settings:      settings,
		TransportName: "test",
		Now: func() time.Time {
			return time.Date(2026, 5, 7, 12, 30, 0, 0, time.UTC)
		},
	})
	engine := gin.New()
	engine.GET("/model-catalog/*catalog_path", handlers.ModelCatalogRoute)
	engine.POST("/model-catalog/*catalog_path", handlers.ModelCatalogRoute)
	engine.GET("/openai/v1/models", handlers.OpenAIModels)
	return engine
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

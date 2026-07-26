package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/modelcatalog"
)

func TestHTTPHandlersModelCatalogDependency(t *testing.T) {
	t.Parallel()

	t.Run("ShouldPassModelCatalogServiceToBaseHandlers", func(t *testing.T) {
		t.Parallel()

		service := httpModelCatalogServiceStub{}
		handlers := newHandlers(&handlerConfig{modelCatalog: service})
		if handlers.BaseHandlers == nil {
			t.Fatal("newHandlers() BaseHandlers = nil")
		}
		if handlers.ModelCatalog == nil {
			t.Fatal("newHandlers() ModelCatalog = nil, want injected service")
		}
		if handlers.ModelCatalog != service {
			t.Fatalf("newHandlers() ModelCatalog = %#v, want %#v", handlers.ModelCatalog, service)
		}
		if handlers.ModelCatalog != service {
			t.Fatalf(
				"newHandlers() BaseHandlers.ModelCatalog = %#v, want %#v",
				handlers.ModelCatalog,
				service,
			)
		}
	})
}

func TestHTTPModelCatalogRoutes(t *testing.T) {
	t.Parallel()

	t.Run("Should expose native provider model list route", func(t *testing.T) {
		t.Parallel()

		service := &httpModelCatalogServiceSpy{
			listModelsFn: func(_ context.Context, opts modelcatalog.ListOptions) ([]modelcatalog.Model, error) {
				if got, want := opts.ProviderID, "codex"; got != want {
					t.Fatalf("ProviderID = %q, want %q", got, want)
				}
				return []modelcatalog.Model{httpSeedCatalogModel("codex", "gpt-5.4")}, nil
			},
		}
		engine := newHTTPModelCatalogRouter(t, service, "127.0.0.1")

		recorder := performRequest(
			t,
			engine,
			http.MethodGet,
			"/api/model-catalog/providers/codex/models",
			nil,
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
		var payload contract.ProviderModelListResponse
		decodeJSONResponse(t, recorder, &payload)
		if len(payload.Models) != 1 || payload.Models[0].ProviderID != "codex" {
			t.Fatalf("payload = %#v, want codex model", payload)
		}
	})

	t.Run("Should expose OpenAI model route with AGH metadata", func(t *testing.T) {
		t.Parallel()

		service := &httpModelCatalogServiceSpy{
			listModelsFn: func(_ context.Context, opts modelcatalog.ListOptions) ([]modelcatalog.Model, error) {
				if got, want := opts.ProviderID, "codex"; got != want {
					t.Fatalf("ProviderID = %q, want %q", got, want)
				}
				return []modelcatalog.Model{httpSeedCatalogModel("codex", "gpt-5.4")}, nil
			},
		}
		engine := newHTTPModelCatalogRouter(t, service, "127.0.0.1")

		recorder := performRequest(t, engine, http.MethodGet, "/api/openai/v1/models?provider_id=codex", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
		var payload contract.OpenAIModelListResponse
		decodeJSONResponse(t, recorder, &payload)
		if payload.Object != "list" || len(payload.Data) != 1 || payload.Data[0].AGH.ProviderID != "codex" {
			t.Fatalf("payload = %#v, want OpenAI list with agh metadata", payload)
		}
	})

	t.Run("Should return OpenAI shaped forbidden error from API middleware", func(t *testing.T) {
		t.Parallel()

		engine := newHTTPModelCatalogRouter(t, &httpModelCatalogServiceSpy{}, "0.0.0.0")

		recorder := performRequest(t, engine, http.MethodGet, "/api/openai/v1/models", nil)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body=%s", recorder.Code, recorder.Body.String())
		}
		var payload contract.OpenAIErrorResponse
		decodeJSONResponse(t, recorder, &payload)
		if payload.Error.Code != "forbidden" || !strings.Contains(payload.Error.Message, "remote HTTP API access") {
			t.Fatalf("error = %#v, want OpenAI-shaped forbidden API middleware error", payload.Error)
		}
	})
}

type httpModelCatalogServiceStub struct{}

func (httpModelCatalogServiceStub) ListModels(
	context.Context,
	modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	return nil, nil
}

func (httpModelCatalogServiceStub) Refresh(
	context.Context,
	modelcatalog.RefreshOptions,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}

func (httpModelCatalogServiceStub) ListSourceStatus(
	context.Context,
	string,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}

type httpModelCatalogServiceSpy struct {
	listModelsFn       func(context.Context, modelcatalog.ListOptions) ([]modelcatalog.Model, error)
	refreshFn          func(context.Context, modelcatalog.RefreshOptions) ([]modelcatalog.SourceStatus, error)
	listSourceStatusFn func(context.Context, string) ([]modelcatalog.SourceStatus, error)
}

func (s *httpModelCatalogServiceSpy) ListModels(
	ctx context.Context,
	opts modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	if s.listModelsFn != nil {
		return s.listModelsFn(ctx, opts)
	}
	return nil, nil
}

func (s *httpModelCatalogServiceSpy) Refresh(
	ctx context.Context,
	opts modelcatalog.RefreshOptions,
) ([]modelcatalog.SourceStatus, error) {
	if s.refreshFn != nil {
		return s.refreshFn(ctx, opts)
	}
	return nil, nil
}

func (s *httpModelCatalogServiceSpy) ListSourceStatus(
	ctx context.Context,
	providerID string,
) ([]modelcatalog.SourceStatus, error) {
	if s.listSourceStatusFn != nil {
		return s.listSourceStatusFn(ctx, providerID)
	}
	return nil, nil
}

func newHTTPModelCatalogRouter(
	t *testing.T,
	service coreModelCatalogService,
	boundHost string,
) http.Handler {
	t.Helper()

	cfg := testConfigWithDisabledNetwork(newTestHomePaths(t))
	cfg.HTTP.Host = boundHost
	cfg.HTTP.Port = 2123
	handlers := newHandlers(&handlerConfig{
		modelCatalog: service,
		staticFS:     mustStaticFS(t),
		config:       cfg,
		boundHost:    boundHost,
		logger:       discardLogger(),
		startedAt:    time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		now:          func() time.Time { return time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC) },
		pollInterval: 5 * time.Millisecond,
		agentLoader:  aghconfig.LoadAgentDef,
		httpPort:     cfg.HTTP.Port,
		workspaces:   stubWorkspaceService{},
		tasks:        &stubTaskManager{},
	})
	return newTestRouter(t, handlers)
}

type coreModelCatalogService interface {
	ListModels(context.Context, modelcatalog.ListOptions) ([]modelcatalog.Model, error)
	Refresh(context.Context, modelcatalog.RefreshOptions) ([]modelcatalog.SourceStatus, error)
	ListSourceStatus(context.Context, string) ([]modelcatalog.SourceStatus, error)
}

func httpSeedCatalogModel(providerID string, modelID string) modelcatalog.Model {
	available := true
	return modelcatalog.Model{
		ProviderID:        providerID,
		ModelID:           modelID,
		Available:         &available,
		AvailabilityState: modelcatalog.AvailabilityStateAvailableLive,
		Sources: []modelcatalog.SourceRef{
			{SourceID: modelcatalog.SourceIDConfig, SourceKind: modelcatalog.SourceKindConfig},
		},
	}
}

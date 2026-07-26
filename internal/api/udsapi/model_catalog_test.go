package udsapi

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/modelcatalog"
)

func TestUDSHandlersModelCatalogDependency(t *testing.T) {
	t.Parallel()

	t.Run("Should pass model catalog service to base handlers", func(t *testing.T) {
		t.Parallel()

		service := udsModelCatalogServiceStub{}
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
	})
}

func TestUDSModelCatalogRoutes(t *testing.T) {
	t.Parallel()

	t.Run("Should expose native provider model list route", func(t *testing.T) {
		t.Parallel()

		service := &udsModelCatalogServiceSpy{
			listModelsFn: func(_ context.Context, opts modelcatalog.ListOptions) ([]modelcatalog.Model, error) {
				if got, want := opts.ProviderID, "codex"; got != want {
					t.Fatalf("ProviderID = %q, want %q", got, want)
				}
				return []modelcatalog.Model{udsSeedCatalogModel("codex", "gpt-5.4")}, nil
			},
		}
		engine := newTestRouter(t, newHandlers(&handlerConfig{modelCatalog: service}))

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

	t.Run("Should not register OpenAI model projection", func(t *testing.T) {
		t.Parallel()

		engine := newTestRouter(t, newHandlers(&handlerConfig{modelCatalog: &udsModelCatalogServiceSpy{}}))

		recorder := performRequest(t, engine, http.MethodGet, "/api/openai/v1/models", nil)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404; body=%s", recorder.Code, recorder.Body.String())
		}
		if got, want := strings.TrimSpace(recorder.Body.String()), "404 page not found"; got != want {
			t.Fatalf("body = %q, want %q", got, want)
		}
	})
}

type udsModelCatalogServiceStub struct{}

func (udsModelCatalogServiceStub) ListModels(
	context.Context,
	modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	return nil, nil
}

func (udsModelCatalogServiceStub) Refresh(
	context.Context,
	modelcatalog.RefreshOptions,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}

func (udsModelCatalogServiceStub) ListSourceStatus(
	context.Context,
	string,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}

type udsModelCatalogServiceSpy struct {
	listModelsFn       func(context.Context, modelcatalog.ListOptions) ([]modelcatalog.Model, error)
	refreshFn          func(context.Context, modelcatalog.RefreshOptions) ([]modelcatalog.SourceStatus, error)
	listSourceStatusFn func(context.Context, string) ([]modelcatalog.SourceStatus, error)
}

func (s *udsModelCatalogServiceSpy) ListModels(
	ctx context.Context,
	opts modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	if s.listModelsFn != nil {
		return s.listModelsFn(ctx, opts)
	}
	return nil, nil
}

func (s *udsModelCatalogServiceSpy) Refresh(
	ctx context.Context,
	opts modelcatalog.RefreshOptions,
) ([]modelcatalog.SourceStatus, error) {
	if s.refreshFn != nil {
		return s.refreshFn(ctx, opts)
	}
	return nil, nil
}

func (s *udsModelCatalogServiceSpy) ListSourceStatus(
	ctx context.Context,
	providerID string,
) ([]modelcatalog.SourceStatus, error) {
	if s.listSourceStatusFn != nil {
		return s.listSourceStatusFn(ctx, providerID)
	}
	return nil, nil
}

func udsSeedCatalogModel(providerID string, modelID string) modelcatalog.Model {
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

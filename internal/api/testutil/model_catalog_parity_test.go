package testutil_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/httpapi"
	"github.com/compozy/agh/internal/api/testutil"
	"github.com/compozy/agh/internal/api/udsapi"
	"github.com/compozy/agh/internal/cli"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/config/lifecycle"
	"github.com/compozy/agh/internal/modelcatalog"
	settingspkg "github.com/compozy/agh/internal/settings"
	"github.com/gin-gonic/gin"
)

func TestModelCatalogTransportParity(t *testing.T) {
	t.Parallel()

	t.Run("Should return canonical native list JSON bytes for the same catalog state", func(t *testing.T) {
		t.Parallel()

		service := &parityModelCatalogService{
			models: []modelcatalog.Model{parityCatalogModel("codex", "gpt-5.4")},
		}
		httpEngine := newParityHTTPRouter(t, service, nil)
		udsEngine := newParityUDSRouter(t, service, nil)

		httpResp := performParityRequest(t, httpEngine, http.MethodGet, "/api/model-catalog/providers/codex/models")
		udsResp := performParityRequest(t, udsEngine, http.MethodGet, "/api/model-catalog/providers/codex/models")
		if httpResp.Code != http.StatusOK || udsResp.Code != http.StatusOK {
			t.Fatalf(
				"statuses = http:%d uds:%d, want 200; http=%s uds=%s",
				httpResp.Code,
				udsResp.Code,
				httpResp.Body.String(),
				udsResp.Body.String(),
			)
		}
		if got, want := httpResp.Body.String(), udsResp.Body.String(); got != want {
			t.Fatalf("HTTP body = %s, want UDS body %s", got, want)
		}
		var cliRecord cli.ProviderModelListRecord
		if err := json.Unmarshal(httpResp.Body.Bytes(), &cliRecord); err != nil {
			t.Fatalf("json.Unmarshal(HTTP body as CLI record) error = %v", err)
		}
		cliJSON, err := json.Marshal(cliRecord)
		if err != nil {
			t.Fatalf("json.Marshal(CLI record) error = %v", err)
		}
		if got, want := string(cliJSON), httpResp.Body.String(); got != want {
			t.Fatalf("CLI JSON = %s, want canonical native body %s", got, want)
		}

		openAIResp := performParityRequest(
			t,
			httpEngine,
			http.MethodGet,
			"/api/openai/v1/models?provider_id=codex",
		)
		if openAIResp.Code != http.StatusOK {
			t.Fatalf("OpenAI status = %d, want 200; body=%s", openAIResp.Code, openAIResp.Body.String())
		}
		var openAIPayload contract.OpenAIModelListResponse
		if err := json.Unmarshal(openAIResp.Body.Bytes(), &openAIPayload); err != nil {
			t.Fatalf("json.Unmarshal(OpenAI body) error = %v", err)
		}
		if len(openAIPayload.Data) != 1 {
			t.Fatalf("OpenAI data = %#v, want one model", openAIPayload.Data)
		}
		openAIModel := openAIPayload.Data[0]
		nativeModel := cliRecord.Models[0]
		if openAIModel.ID != nativeModel.ModelID ||
			openAIModel.OwnedBy != nativeModel.ProviderID ||
			openAIModel.AGH.ProviderID != nativeModel.ProviderID ||
			openAIModel.AGH.ModelID != nativeModel.ModelID {
			t.Fatalf("OpenAI model = %#v, want native catalog identity %#v", openAIModel, nativeModel)
		}
	})

	t.Run("Should return identical curation JSON for HTTP UDS and CLI contracts", func(t *testing.T) {
		t.Parallel()

		catalog := &parityModelCatalogService{}
		model := parityCatalogModel("codex", "gpt-5.6-sol")
		model.Hidden = true
		model.DefaultReasoningEffort = new(modelcatalog.ReasoningEffortMax)
		settingsService := &parityModelCurationService{
			result: settingspkg.ProviderModelCurationResult{
				Model: model,
				Apply: settingspkg.ApplyResult{
					Applied: true,
					Record: settingspkg.ApplyRecord{
						ID:         "cfgapp-parity",
						Lifecycle:  lifecycle.Live,
						Generation: 7,
						ActiveHash: "sha256:parity",
					},
				},
			},
		}
		httpEngine := newParityHTTPRouter(t, catalog, settingsService)
		udsEngine := newParityUDSRouter(t, catalog, settingsService)
		body := []byte(`{"model_id":"gpt-5.6-sol","hidden":true,"default_effort":"max"}`)
		path := "/api/model-catalog/providers/codex/models/curate"
		httpResp := performParityJSONRequest(t, httpEngine, http.MethodPost, path, body)
		udsResp := performParityJSONRequest(t, udsEngine, http.MethodPost, path, body)
		if httpResp.Code != http.StatusOK || udsResp.Code != http.StatusOK {
			t.Fatalf(
				"statuses = http:%d uds:%d, want 200; http=%s uds=%s",
				httpResp.Code,
				udsResp.Code,
				httpResp.Body.String(),
				udsResp.Body.String(),
			)
		}
		if got, want := httpResp.Body.String(), udsResp.Body.String(); got != want {
			t.Fatalf("HTTP body = %s, want UDS body %s", got, want)
		}
		var cliRecord cli.ProviderModelCurationRecord
		if err := json.Unmarshal(httpResp.Body.Bytes(), &cliRecord); err != nil {
			t.Fatalf("json.Unmarshal(curation as CLI record) error = %v", err)
		}
		if !cliRecord.Model.Hidden || cliRecord.Model.DefaultReasoningEffort == nil ||
			*cliRecord.Model.DefaultReasoningEffort != "max" || !cliRecord.Apply.Applied {
			t.Fatalf("CLI curation record = %#v, want hidden max applied", cliRecord)
		}
		if settingsService.calls != 2 {
			t.Fatalf("ApplyProviderModelCuration calls = %d, want 2", settingsService.calls)
		}
	})
}

func newParityHTTPRouter(
	t *testing.T,
	service *parityModelCatalogService,
	settings core.SettingsService,
) http.Handler {
	t.Helper()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	homePaths := newShortParityHomePaths(t)
	cfg := testutil.ConfigWithDisabledNetwork(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = 2123
	if _, err := httpapi.New(
		httpapi.WithEngine(engine),
		httpapi.WithHomePaths(homePaths),
		httpapi.WithConfig(&cfg),
		httpapi.WithHost(cfg.HTTP.Host),
		httpapi.WithPort(cfg.HTTP.Port),
		httpapi.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		httpapi.WithStartedAt(time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)),
		httpapi.WithNow(func() time.Time { return time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC) }),
		httpapi.WithSessionManager(testutil.StubSessionManager{}),
		httpapi.WithTaskService(&testutil.StubTaskManager{}),
		httpapi.WithObserver(testutil.StubObserver{}),
		httpapi.WithWorkspaceResolver(testutil.StubWorkspaceService{}),
		httpapi.WithModelCatalogService(service),
		httpapi.WithSettingsService(settings),
	); err != nil {
		t.Fatalf("httpapi.New() error = %v", err)
	}
	return engine
}

func newParityUDSRouter(
	t *testing.T,
	service *parityModelCatalogService,
	settings core.SettingsService,
) http.Handler {
	t.Helper()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	homePaths := newShortParityHomePaths(t)
	cfg := testutil.ConfigWithDisabledNetwork(homePaths)
	if _, err := udsapi.New(
		udsapi.WithEngine(engine),
		udsapi.WithHomePaths(homePaths),
		udsapi.WithConfig(&cfg),
		udsapi.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		udsapi.WithStartedAt(time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)),
		udsapi.WithNow(func() time.Time { return time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC) }),
		udsapi.WithSessionManager(testutil.StubSessionManager{}),
		udsapi.WithTaskService(&testutil.StubTaskManager{}),
		udsapi.WithObserver(testutil.StubObserver{}),
		udsapi.WithWorkspaceResolver(testutil.StubWorkspaceService{}),
		udsapi.WithModelCatalogService(service),
		udsapi.WithSettingsService(settings),
	); err != nil {
		t.Fatalf("udsapi.New() error = %v", err)
	}
	return engine
}

func performParityRequest(t *testing.T, handler http.Handler, method string, path string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(context.Background(), method, path, http.NoBody)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func performParityJSONRequest(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	body []byte,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(context.Background(), method, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func newShortParityHomePaths(t *testing.T) aghconfig.HomePaths {
	t.Helper()

	root, err := os.MkdirTemp("", "agh-mp-*")
	if err != nil {
		t.Fatalf("MkdirTemp() error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("RemoveAll(%q) error = %v", root, err)
		}
	})
	homePaths, err := aghconfig.ResolveHomePathsFrom(root)
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	return homePaths
}

type parityModelCatalogService struct {
	models []modelcatalog.Model
}

type parityModelCurationService struct {
	core.SettingsService
	result settingspkg.ProviderModelCurationResult
	calls  int
}

func (s *parityModelCurationService) ApplyProviderModelCuration(
	_ context.Context,
	_ settingspkg.ProviderModelCurationRequest,
) (settingspkg.ProviderModelCurationResult, error) {
	s.calls++
	return s.result, nil
}

func (s *parityModelCatalogService) ListModels(
	context.Context,
	modelcatalog.ListOptions,
) ([]modelcatalog.Model, error) {
	return append([]modelcatalog.Model(nil), s.models...), nil
}

func (*parityModelCatalogService) Refresh(
	context.Context,
	modelcatalog.RefreshOptions,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}

func (*parityModelCatalogService) ListSourceStatus(
	context.Context,
	string,
) ([]modelcatalog.SourceStatus, error) {
	return nil, nil
}

func parityCatalogModel(providerID string, modelID string) modelcatalog.Model {
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

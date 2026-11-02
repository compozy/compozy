package testutil_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/providerusage"
	"github.com/compozy/compozy/internal/store"
	"github.com/jonboulle/clockwork"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/api/udsapi"
	"github.com/compozy/compozy/internal/cli"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/config/lifecycle"
	"github.com/compozy/compozy/internal/modelcatalog"
	settingspkg "github.com/compozy/compozy/internal/settings"
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
			openAIModel.Compozy.ProviderID != nativeModel.ProviderID ||
			openAIModel.Compozy.ModelID != nativeModel.ModelID {
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
	usage ...core.ProviderUsageService,
) http.Handler {
	t.Helper()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	homePaths := newShortParityHomePaths(t)
	cfg := testutil.ConfigForTest(homePaths)
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
		httpapi.WithSessionManager(&testutil.StubSessionManager{}),
		httpapi.WithTaskService(&testutil.StubTaskManager{}),
		httpapi.WithObserver(testutil.StubObserver{}),
		httpapi.WithWorkspaceResolver(testutil.StubWorkspaceService{}),
		httpapi.WithModelCatalogService(service),
		httpapi.WithSettingsService(settings),
		httpapi.WithProviderUsageService(parityUsageService(usage)),
	); err != nil {
		t.Fatalf("httpapi.New() error = %v", err)
	}
	return engine
}

func newParityUDSRouter(
	t *testing.T,
	service *parityModelCatalogService,
	settings core.SettingsService,
	usage ...core.ProviderUsageService,
) http.Handler {
	t.Helper()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	homePaths := newShortParityHomePaths(t)
	cfg := testutil.ConfigForTest(homePaths)
	if _, err := udsapi.New(
		udsapi.WithEngine(engine),
		udsapi.WithHomePaths(homePaths),
		udsapi.WithConfig(&cfg),
		udsapi.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		udsapi.WithStartedAt(time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)),
		udsapi.WithNow(func() time.Time { return time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC) }),
		udsapi.WithSessionManager(&testutil.StubSessionManager{}),
		udsapi.WithTaskService(&testutil.StubTaskManager{}),
		udsapi.WithObserver(testutil.StubObserver{}),
		udsapi.WithWorkspaceResolver(testutil.StubWorkspaceService{}),
		udsapi.WithModelCatalogService(service),
		udsapi.WithSettingsService(settings),
		udsapi.WithProviderUsageService(parityUsageService(usage)),
	); err != nil {
		t.Fatalf("udsapi.New() error = %v", err)
	}
	return engine
}

func performParityRequest(t *testing.T, handler http.Handler, method string, path string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), method, path, http.NoBody)
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

	request := httptest.NewRequestWithContext(t.Context(), method, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func newShortParityHomePaths(t *testing.T) compozyconfig.HomePaths {
	t.Helper()

	root, err := os.MkdirTemp("", "compozy-mp-*")
	if err != nil {
		t.Fatalf("MkdirTemp() error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("RemoveAll(%q) error = %v", root, err)
		}
	})
	homePaths, err := compozyconfig.ResolveHomePathsFrom(root)
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
	modelcatalog.StatusOptions,
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

func parityUsageService(usage []core.ProviderUsageService) core.ProviderUsageService {
	if len(usage) == 0 {
		return nil
	}
	return usage[0]
}

// IT-001/002/003/006/007/010: transport parity is owned here; collection uses the
// real manager, TargetCatalog, and OpenRouter collector against an HTTP provider.
func TestProviderUsageTransportParity(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls.Add(1)
		if req.Header.Get("Authorization") != "Bearer fixture-usage-secret" {
			t.Error("missing bound credential")
		}
		w.Header().Set("Content-Type", "application/json")
		if req.URL.Path == "/api/v1/key" {
			_, _ = io.WriteString(w, `{"data":{"limit":100,"usage":37.5}}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":{"total_credits":50,"total_usage":12.5}}`)
	}))
	defer upstream.Close()
	catalog := &providerusage.TargetCatalog{}
	catalog.Replace(
		store.DefaultProfileID,
		[]providerusage.Target{
			{
				ProfileID:        store.DefaultProfileID,
				ProviderID:       "openrouter",
				ProviderName:     "OpenRouter",
				RouteFingerprint: "fixture-route",
				Route: providerusage.ResolvedRoute{
					BoundSecrets: map[string]string{"api_key": "fixture-usage-secret"},
				},
			},
		},
	)
	service := providerusage.NewService(providerusage.Options{
		Resolver: catalog,
		Clock:    clockwork.NewFakeClockAt(time.Date(2026, 10, 20, 22, 10, 0, 0, time.UTC)),
		Collectors: providerusage.Registry{
			"openrouter": providerusage.NewOpenRouterCollector(providerusage.WithCollectorBaseURL(upstream.URL)),
		},
	})
	t.Cleanup(func() {
		if err := service.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	httpRouter, udsRouter := newParityHTTPRouter(t, nil, nil, service), newParityUDSRouter(t, nil, nil, service)
	// Ready is observable before triggering the first collection; no timing sleeps.
	streamServer := httptest.NewServer(httpRouter)
	defer streamServer.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		streamServer.URL+"/api/provider-usage/stream",
		http.NoBody,
	)
	if err != nil {
		t.Fatal(err)
	}
	response, err := streamServer.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	scanner := bufio.NewScanner(response.Body)
	readUsageFrame(t, scanner, "ready", service.BootID())
	refreshed := performParityJSONRequest(
		t,
		udsRouter,
		http.MethodPost,
		"/api/provider-usage/refresh",
		[]byte(`{"provider_ids":["openrouter"],"intent":"explicit"}`),
	)
	if refreshed.Code != http.StatusOK {
		t.Fatalf("refresh: %d %s", refreshed.Code, refreshed.Body.String())
	}
	readUsageFrame(t, scanner, "provider_usage_changed", service.BootID())
	for _, path := range []string{"/api/provider-usage", "/api/provider-usage/providers/openrouter"} {
		left := performParityRequest(t, httpRouter, http.MethodGet, path)
		right := performParityRequest(t, udsRouter, http.MethodGet, path)
		if left.Code != http.StatusOK || right.Code != http.StatusOK || left.Body.String() != right.Body.String() {
			t.Fatalf("parity: HTTP=%d %s UDS=%d %s", left.Code, left.Body.String(), right.Code, right.Body.String())
		}
		var envelope contract.ProviderUsageEnvelope
		if err := json.Unmarshal(left.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if len(envelope.Targets) != 1 || envelope.Targets[0].Status != "ok" ||
			envelope.Targets[0].SpendCaps[0].Used.Value != "37.5" {
			t.Fatalf("usage: %+v", envelope)
		}
		if strings.Contains(left.Body.String(), "fixture-usage-secret") {
			t.Fatal("secret leaked")
		}
	}
	before := calls.Load()
	for range 2 {
		result := performParityJSONRequest(
			t,
			httpRouter,
			http.MethodPost,
			"/api/provider-usage/refresh",
			[]byte(`{"wait_ms":0}`),
		)
		if result.Code != http.StatusOK {
			t.Fatal(result.Body.String())
		}
	}
	if calls.Load() != before {
		t.Fatal("refresh floor did not coalesce")
	}
	for _, tc := range []struct {
		method, path, body, code string
		status                   int
	}{
		{http.MethodPost, "/api/provider-usage/refresh", `{"wait_ms":45000}`, "invalid_request", 400},
		{http.MethodGet, "/api/provider-usage/providers/nope", "", "provider_not_found", 404},
		{http.MethodGet, "/api/provider-usage?all_profiles=true", "", "invalid_request", 400},
	} {
		result := performParityJSONRequest(t, httpRouter, tc.method, tc.path, []byte(tc.body))
		if result.Code != tc.status || !strings.Contains(result.Body.String(), tc.code) {
			t.Fatalf("%s: %d %s", tc.path, result.Code, result.Body.String())
		}
	}
	if err := service.SetEnabled(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	result := performParityJSONRequest(t, udsRouter, http.MethodPost, "/api/provider-usage/refresh", []byte(`{}`))
	if result.Code != http.StatusConflict || !strings.Contains(result.Body.String(), "provider_usage_disabled") {
		t.Fatal(result.Body.String())
	}
}

func readUsageFrame(t *testing.T, scanner *bufio.Scanner, event, bootID string) {
	t.Helper()
	name := ""
	for scanner.Scan() {
		line := scanner.Text()
		if value, ok := strings.CutPrefix(line, "event:"); ok {
			name = strings.TrimSpace(value)
		}
		if strings.HasPrefix(line, "data:") && name == event {
			if !strings.Contains(line, bootID) || !strings.Contains(line, "revision") {
				t.Fatalf("invalid revision: %s", line)
			}
			return
		}
	}
	t.Fatalf("missing %s: %v", event, scanner.Err())
}

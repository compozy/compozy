package core_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	automationmodel "github.com/compozy/agh/internal/automation/model"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/config/lifecycle"
	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
	extensionpkg "github.com/compozy/agh/internal/extension"
	hookspkg "github.com/compozy/agh/internal/hooks"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/resources"
	settingspkg "github.com/compozy/agh/internal/settings"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

type stubSettingsService struct {
	GetSectionFn         func(context.Context, settingspkg.SectionRequest) (settingspkg.SectionEnvelope, error)
	UpdateSectionFn      func(context.Context, settingspkg.SectionUpdateRequest) (settingspkg.MutationResult, error)
	ApplySectionFn       func(context.Context, settingspkg.SectionUpdateRequest) (settingspkg.ApplyResult, error)
	ListCollectionFn     func(context.Context, settingspkg.CollectionRequest) (settingspkg.CollectionEnvelope, error)
	PutCollectionItemFn  func(context.Context, settingspkg.CollectionItemPutRequest) (settingspkg.MutationResult, error)
	ApplyCollectionFn    func(context.Context, settingspkg.CollectionItemPutRequest) (settingspkg.ApplyResult, error)
	ApplyModelCurationFn func(context.Context, settingspkg.ProviderModelCurationRequest) (settingspkg.ProviderModelCurationResult, error)
	InstallMCPCatalogFn  func(context.Context, settingspkg.MCPCatalogInstallRequest) (settingspkg.MCPCatalogInstallResult, error)
	BeginMCPAuthFn       func(context.Context, settingspkg.MCPAuthBeginRequest) (mcpauth.BeginResult, error)
	ExchangeMCPAuthFn    func(context.Context, settingspkg.MCPAuthExchangeRequest) (mcpauth.Status, error)
	CompleteMCPAuthFn    func(context.Context, string) (mcpauth.Status, error)
	LogoutMCPAuthFn      func(context.Context, settingspkg.MCPAuthTargetRequest) (mcpauth.Status, error)
	DeleteItemFn         func(context.Context, settingspkg.CollectionItemDeleteRequest) (settingspkg.MutationResult, error)
	ApplyDeleteFn        func(context.Context, settingspkg.CollectionItemDeleteRequest) (settingspkg.ApplyResult, error)
	ReloadFn             func(context.Context) (settingspkg.ApplyResult, error)
	ListApplyRecordsFn   func(context.Context, settingspkg.ApplyRecordFilter) ([]settingspkg.ApplyRecord, error)

	LastGetSectionRequest     settingspkg.SectionRequest
	LastUpdateSectionRequest  settingspkg.SectionUpdateRequest
	LastListCollectionRequest settingspkg.CollectionRequest
	LastPutCollectionRequest  settingspkg.CollectionItemPutRequest
	LastModelCurationRequest  settingspkg.ProviderModelCurationRequest
	LastMCPCatalogInstall     settingspkg.MCPCatalogInstallRequest
	LastMCPAuthBegin          settingspkg.MCPAuthBeginRequest
	LastMCPAuthExchange       settingspkg.MCPAuthExchangeRequest
	LastMCPAuthCallback       string
	LastMCPAuthLogout         settingspkg.MCPAuthTargetRequest
	LastDeleteRequest         settingspkg.CollectionItemDeleteRequest
	LastApplyRecordFilter     settingspkg.ApplyRecordFilter

	GetSectionCalls         int
	UpdateSectionCalls      int
	ApplySectionCalls       int
	ListCollectionCalls     int
	PutCollectionItemCalls  int
	ApplyCollectionCalls    int
	ApplyModelCurationCalls int
	InstallMCPCatalogCalls  int
	BeginMCPAuthCalls       int
	ExchangeMCPAuthCalls    int
	CompleteMCPAuthCalls    int
	LogoutMCPAuthCalls      int
	DeleteItemCalls         int
	ApplyDeleteCalls        int
	ReloadCalls             int
	ListApplyRecordsCalls   int
}

func (s *stubSettingsService) GetSection(
	ctx context.Context,
	req settingspkg.SectionRequest,
) (settingspkg.SectionEnvelope, error) {
	s.GetSectionCalls++
	s.LastGetSectionRequest = req
	if s.GetSectionFn != nil {
		return s.GetSectionFn(ctx, req)
	}
	return settingspkg.SectionEnvelope{}, nil
}

func (s *stubSettingsService) UpdateSection(
	ctx context.Context,
	req settingspkg.SectionUpdateRequest,
) (settingspkg.MutationResult, error) {
	s.UpdateSectionCalls++
	s.LastUpdateSectionRequest = req
	if s.UpdateSectionFn != nil {
		return s.UpdateSectionFn(ctx, req)
	}
	return settingspkg.MutationResult{}, nil
}

func (s *stubSettingsService) ApplySection(
	ctx context.Context,
	req settingspkg.SectionUpdateRequest,
) (settingspkg.ApplyResult, error) {
	s.ApplySectionCalls++
	s.LastUpdateSectionRequest = req
	if s.ApplySectionFn != nil {
		return s.ApplySectionFn(ctx, req)
	}
	if s.UpdateSectionFn != nil {
		result, err := s.UpdateSectionFn(ctx, req)
		if err != nil {
			return settingspkg.ApplyResult{}, err
		}
		return applyResultFromMutation(result), nil
	}
	return defaultApplyResult(req.Section), nil
}

func (s *stubSettingsService) ListCollection(
	ctx context.Context,
	req settingspkg.CollectionRequest,
) (settingspkg.CollectionEnvelope, error) {
	s.ListCollectionCalls++
	s.LastListCollectionRequest = req
	if s.ListCollectionFn != nil {
		return s.ListCollectionFn(ctx, req)
	}
	return settingspkg.CollectionEnvelope{}, nil
}

func (s *stubSettingsService) PutCollectionItem(
	ctx context.Context,
	req settingspkg.CollectionItemPutRequest,
) (settingspkg.MutationResult, error) {
	s.PutCollectionItemCalls++
	s.LastPutCollectionRequest = req
	if s.PutCollectionItemFn != nil {
		return s.PutCollectionItemFn(ctx, req)
	}
	return settingspkg.MutationResult{}, nil
}

func (s *stubSettingsService) ApplyCollectionItem(
	ctx context.Context,
	req settingspkg.CollectionItemPutRequest,
) (settingspkg.ApplyResult, error) {
	s.ApplyCollectionCalls++
	s.LastPutCollectionRequest = req
	if s.ApplyCollectionFn != nil {
		return s.ApplyCollectionFn(ctx, req)
	}
	if s.PutCollectionItemFn != nil {
		result, err := s.PutCollectionItemFn(ctx, req)
		if err != nil {
			return settingspkg.ApplyResult{}, err
		}
		return applyResultFromMutation(result), nil
	}
	return defaultApplyResult(settingspkg.SectionName(req.Collection)), nil
}

func (s *stubSettingsService) ApplyProviderModelCuration(
	ctx context.Context,
	req settingspkg.ProviderModelCurationRequest,
) (settingspkg.ProviderModelCurationResult, error) {
	s.ApplyModelCurationCalls++
	s.LastModelCurationRequest = req
	if s.ApplyModelCurationFn != nil {
		return s.ApplyModelCurationFn(ctx, req)
	}
	return settingspkg.ProviderModelCurationResult{}, nil
}

func (s *stubSettingsService) InstallMCPCatalog(
	ctx context.Context,
	req settingspkg.MCPCatalogInstallRequest,
) (settingspkg.MCPCatalogInstallResult, error) {
	s.InstallMCPCatalogCalls++
	s.LastMCPCatalogInstall = req
	if s.InstallMCPCatalogFn != nil {
		return s.InstallMCPCatalogFn(ctx, req)
	}
	return settingspkg.MCPCatalogInstallResult{}, nil
}

func (s *stubSettingsService) BeginMCPAuth(
	ctx context.Context,
	req settingspkg.MCPAuthBeginRequest,
) (mcpauth.BeginResult, error) {
	s.BeginMCPAuthCalls++
	s.LastMCPAuthBegin = req
	if s.BeginMCPAuthFn != nil {
		return s.BeginMCPAuthFn(ctx, req)
	}
	return mcpauth.BeginResult{}, nil
}

func (s *stubSettingsService) ExchangeMCPAuth(
	ctx context.Context,
	req settingspkg.MCPAuthExchangeRequest,
) (mcpauth.Status, error) {
	s.ExchangeMCPAuthCalls++
	s.LastMCPAuthExchange = req
	if s.ExchangeMCPAuthFn != nil {
		return s.ExchangeMCPAuthFn(ctx, req)
	}
	return mcpauth.Status{}, nil
}

func (s *stubSettingsService) CompleteMCPAuthCallback(
	ctx context.Context,
	callbackURL string,
) (mcpauth.Status, error) {
	s.CompleteMCPAuthCalls++
	s.LastMCPAuthCallback = callbackURL
	if s.CompleteMCPAuthFn != nil {
		return s.CompleteMCPAuthFn(ctx, callbackURL)
	}
	return mcpauth.Status{}, nil
}

func (s *stubSettingsService) LogoutMCPAuth(
	ctx context.Context,
	req settingspkg.MCPAuthTargetRequest,
) (mcpauth.Status, error) {
	s.LogoutMCPAuthCalls++
	s.LastMCPAuthLogout = req
	if s.LogoutMCPAuthFn != nil {
		return s.LogoutMCPAuthFn(ctx, req)
	}
	return mcpauth.Status{}, nil
}

func (s *stubSettingsService) DeleteCollectionItem(
	ctx context.Context,
	req settingspkg.CollectionItemDeleteRequest,
) (settingspkg.MutationResult, error) {
	s.DeleteItemCalls++
	s.LastDeleteRequest = req
	if s.DeleteItemFn != nil {
		return s.DeleteItemFn(ctx, req)
	}
	return settingspkg.MutationResult{}, nil
}

func (s *stubSettingsService) ApplyCollectionDelete(
	ctx context.Context,
	req settingspkg.CollectionItemDeleteRequest,
) (settingspkg.ApplyResult, error) {
	s.ApplyDeleteCalls++
	s.LastDeleteRequest = req
	if s.ApplyDeleteFn != nil {
		return s.ApplyDeleteFn(ctx, req)
	}
	if s.DeleteItemFn != nil {
		result, err := s.DeleteItemFn(ctx, req)
		if err != nil {
			return settingspkg.ApplyResult{}, err
		}
		return applyResultFromMutation(result), nil
	}
	return defaultApplyResult(settingspkg.SectionName(req.Collection)), nil
}

func (s *stubSettingsService) Reload(ctx context.Context) (settingspkg.ApplyResult, error) {
	s.ReloadCalls++
	if s.ReloadFn != nil {
		return s.ReloadFn(ctx)
	}
	return defaultApplyResult(""), nil
}

func (s *stubSettingsService) ListApplyRecords(
	ctx context.Context,
	filter settingspkg.ApplyRecordFilter,
) ([]settingspkg.ApplyRecord, error) {
	s.ListApplyRecordsCalls++
	s.LastApplyRecordFilter = filter
	if s.ListApplyRecordsFn != nil {
		return s.ListApplyRecordsFn(ctx, filter)
	}
	return nil, nil
}

func defaultApplyResult(section settingspkg.SectionName) settingspkg.ApplyResult {
	return settingspkg.ApplyResult{
		Section:    section,
		Scope:      settingspkg.ScopeGlobal,
		Applied:    true,
		NextAction: "none",
		Record: settingspkg.ApplyRecord{
			ID:         "cfgapp-test",
			ActiveHash: "sha256:test",
			Generation: 1,
			DiffClass:  "live",
			Status:     "applied",
			Lifecycle:  "live",
			NextAction: "none",
			CreatedAt:  time.Unix(1, 0).UTC(),
			UpdatedAt:  time.Unix(1, 0).UTC(),
		},
	}
}

func applyResultFromMutation(result settingspkg.MutationResult) settingspkg.ApplyResult {
	configLifecycle := result.Lifecycle
	if configLifecycle == "" {
		configLifecycle = lifecycle.Live
		if result.RestartRequired {
			configLifecycle = lifecycle.RestartRequired
		}
	}
	diffClass := result.DiffClass
	if diffClass == "" {
		diffClass = lifecycle.DiffClass(configLifecycle)
	}
	status := lifecycle.StatusApplied
	if !result.Applied {
		status = lifecycle.StatusBlocked
	}
	nextAction := lifecycle.NextActionForLifecycle(configLifecycle, status)
	return settingspkg.ApplyResult{
		Section:         result.Section,
		Scope:           result.Scope,
		WriteTarget:     result.WriteTarget,
		WorkspaceID:     result.WorkspaceID,
		AgentName:       result.AgentName,
		Applied:         result.Applied,
		NextAction:      nextAction,
		RestartRequired: result.RestartRequired,
		RestartScope:    result.RestartScope,
		Warnings:        append([]string(nil), result.Warnings...),
		Record: settingspkg.ApplyRecord{
			ID:         "cfgapp-test",
			ActiveHash: "sha256:test",
			Generation: 1,
			DiffClass:  diffClass,
			Status:     status,
			Lifecycle:  configLifecycle,
			NextAction: nextAction,
			CreatedAt:  time.Unix(1, 0).UTC(),
			UpdatedAt:  time.Unix(1, 0).UTC(),
		},
	}
}

type unavailableSettingsMemoryProviderService struct {
	err error
}

func (s unavailableSettingsMemoryProviderService) List(
	context.Context,
	string,
) ([]contract.MemoryProviderPayload, error) {
	return nil, s.err
}

func (s unavailableSettingsMemoryProviderService) Get(
	context.Context,
	string,
	string,
) (contract.MemoryProviderPayload, error) {
	return contract.MemoryProviderPayload{}, s.err
}

func (s unavailableSettingsMemoryProviderService) Select(
	context.Context,
	string,
	string,
) (contract.MemoryProviderPayload, error) {
	return contract.MemoryProviderPayload{}, s.err
}

func (s unavailableSettingsMemoryProviderService) Enable(
	context.Context,
	string,
	string,
	string,
) (contract.MemoryProviderLifecycleResponse, error) {
	return contract.MemoryProviderLifecycleResponse{}, s.err
}

func (s unavailableSettingsMemoryProviderService) Disable(
	context.Context,
	string,
	string,
	string,
) (contract.MemoryProviderLifecycleResponse, error) {
	return contract.MemoryProviderLifecycleResponse{}, s.err
}

type stubSettingsRestartController struct {
	RequestFn func(context.Context) (core.SettingsRestartOperation, error)
	StatusFn  func(context.Context, string) (core.SettingsRestartOperation, error)

	LastOperationID string
	RequestCalls    int
	StatusCalls     int
}

func (s *stubSettingsRestartController) RequestRestart(
	ctx context.Context,
) (core.SettingsRestartOperation, error) {
	s.RequestCalls++
	if s.RequestFn != nil {
		return s.RequestFn(ctx)
	}
	return core.SettingsRestartOperation{}, nil
}

type stubSettingsUpdateController struct {
	GetFn    func(context.Context) (core.SettingsUpdateStatus, error)
	GetCalls int
}

func (s *stubSettingsUpdateController) GetUpdate(ctx context.Context) (core.SettingsUpdateStatus, error) {
	s.GetCalls++
	if s.GetFn != nil {
		return s.GetFn(ctx)
	}
	return core.SettingsUpdateStatus{}, nil
}

func (s *stubSettingsRestartController) GetRestartOperation(
	ctx context.Context,
	operationID string,
) (core.SettingsRestartOperation, error) {
	s.StatusCalls++
	s.LastOperationID = operationID
	if s.StatusFn != nil {
		return s.StatusFn(ctx, operationID)
	}
	return core.SettingsRestartOperation{}, nil
}

type settingsHandlerFixture struct {
	Handlers   *core.BaseHandlers
	Engine     *gin.Engine
	HomePaths  aghconfig.HomePaths
	StreamDone chan struct{}
	Service    *stubSettingsService
	Restart    *stubSettingsRestartController
	Update     *stubSettingsUpdateController
}

func newSettingsHandlerFixture(
	t *testing.T,
	transport string,
	settingsService *stubSettingsService,
	restartController *stubSettingsRestartController,
) settingsHandlerFixture {
	t.Helper()

	gin.SetMode(gin.TestMode)
	homePaths := testutil.NewTestHomePaths(t)
	cfg := aghconfig.DefaultWithHome(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = 2123
	cfg.Daemon.Socket = "/tmp/settings-api-core.sock"
	streamDone := make(chan struct{})

	if settingsService == nil {
		settingsService = &stubSettingsService{}
	}
	if restartController == nil {
		restartController = &stubSettingsRestartController{}
	}
	updateController := &stubSettingsUpdateController{}

	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName:      transport,
		MaskInternalErrors: false,
		Settings:           settingsService,
		SettingsRestart:    restartController,
		SettingsUpdate:     updateController,
		HomePaths:          homePaths,
		Config:             cfg,
		Logger:             testutil.DiscardLogger(),
		Now: func() time.Time {
			return time.Date(2026, 4, 17, 18, 45, 0, 0, time.UTC)
		},
		PollInterval: 5 * time.Millisecond,
		StreamDone:   streamDone,
	})

	engine := gin.New()
	engine.Use(gin.Recovery())
	registerSettingsRoutes(engine, handlers)

	return settingsHandlerFixture{
		Handlers:   handlers,
		Engine:     engine,
		HomePaths:  homePaths,
		StreamDone: streamDone,
		Service:    settingsService,
		Restart:    restartController,
		Update:     updateController,
	}
}

func registerSettingsRoutes(engine *gin.Engine, handlers *core.BaseHandlers) {
	settings := engine.Group("/api/settings")
	settings.GET("/apply", handlers.ListSettingsApplyRecords)
	settings.POST("/reload", handlers.ReloadSettings)
	settings.GET("/general", handlers.GetSettingsGeneral)
	settings.GET("/update", handlers.GetSettingsUpdate)
	settings.PATCH("/general", handlers.UpdateSettingsGeneral)
	settings.GET("/memory", handlers.GetSettingsMemory)
	settings.PATCH("/memory", handlers.UpdateSettingsMemory)
	settings.GET("/roles", handlers.GetSettingsRoles)
	settings.PATCH("/roles", handlers.UpdateSettingsRoles)
	settings.GET("/skills", handlers.GetSettingsSkills)
	settings.PATCH("/skills", handlers.UpdateSettingsSkills)
	settings.GET("/automation", handlers.GetSettingsAutomation)
	settings.PATCH("/automation", handlers.UpdateSettingsAutomation)
	settings.GET("/network", handlers.GetSettingsNetwork)
	settings.PATCH("/network", handlers.UpdateSettingsNetwork)
	settings.GET("/window-manager", handlers.GetSettingsWindowManager)
	settings.PATCH("/window-manager", handlers.UpdateSettingsWindowManager)
	settings.GET("/observability", handlers.GetSettingsObservability)
	settings.PATCH("/observability", handlers.UpdateSettingsObservability)
	settings.GET("/hooks-extensions", handlers.GetSettingsHooksExtensions)
	settings.PATCH("/hooks-extensions", handlers.UpdateSettingsHooksExtensions)
	settings.GET("/providers", handlers.ListSettingsProviders)
	settings.GET("/providers/:name", handlers.GetSettingsProvider)
	settings.PUT("/providers/:name", handlers.PutSettingsProvider)
	settings.DELETE("/providers/:name", handlers.DeleteSettingsProvider)
	settings.GET("/mcp-servers", handlers.ListSettingsMCPServers)
	settings.POST("/mcp-servers/install", handlers.InstallSettingsMCPServer)
	settings.POST("/mcp-servers/:name/auth/begin", handlers.BeginSettingsMCPAuth)
	settings.POST("/mcp-servers/:name/auth/exchange", handlers.ExchangeSettingsMCPAuth)
	settings.POST("/mcp-servers/:name/auth/logout", handlers.LogoutSettingsMCPAuth)
	settings.PUT("/mcp-servers/:name", handlers.PutSettingsMCPServer)
	settings.DELETE("/mcp-servers/:name", handlers.DeleteSettingsMCPServer)
	settings.GET("/sandboxes", handlers.ListSettingsSandboxes)
	settings.GET("/sandboxes/:name", handlers.GetSettingsSandbox)
	settings.PUT("/sandboxes/:name", handlers.PutSettingsSandbox)
	settings.DELETE("/sandboxes/:name", handlers.DeleteSettingsSandbox)
	settings.GET("/hooks", handlers.ListSettingsHooks)
	settings.PUT("/hooks/:name", handlers.PutSettingsHook)
	settings.DELETE("/hooks/:name", handlers.DeleteSettingsHook)
	settings.POST("/actions/restart", handlers.TriggerSettingsRestart)
	settings.GET("/actions/restart/:operation_id", handlers.GetSettingsRestartStatus)
	settings.GET("/observability/log-tail", handlers.StreamSettingsObservabilityLogTail)
	engine.GET("/api/mcp/oauth/callback", handlers.CompleteSettingsMCPAuthCallback)
}

func TestSettingsMCPAuthHandlersKeepSecretsOutOfResponses(t *testing.T) {
	t.Parallel()

	t.Run("Should keep OAuth secrets out of handler responses", func(t *testing.T) {
		t.Parallel()

		expiresAt := time.Date(2026, 7, 13, 16, 5, 0, 0, time.UTC)
		service := &stubSettingsService{
			BeginMCPAuthFn: func(
				_ context.Context,
				req settingspkg.MCPAuthBeginRequest,
			) (mcpauth.BeginResult, error) {
				if req.Scope != settingspkg.ScopeWorkspace || req.WorkspaceID != "workspace-a" || req.Name != "linear" {
					t.Fatalf("BeginMCPAuth request = %#v", req)
				}
				if req.CallbackURL != "http://127.0.0.1:2123/api/mcp/oauth/callback" {
					t.Fatalf("BeginMCPAuth callback URL = %q", req.CallbackURL)
				}
				return mcpauth.BeginResult{
					AuthorizationURL: "https://auth.example/authorize?state=public-state",
					State:            "public-state",
					ExpiresAt:        expiresAt,
					CallbackURL:      req.CallbackURL,
					ManualSupported:  true,
				}, nil
			},
			ExchangeMCPAuthFn: func(
				_ context.Context,
				req settingspkg.MCPAuthExchangeRequest,
			) (mcpauth.Status, error) {
				if req.Scope != settingspkg.ScopeWorkspace || req.WorkspaceID != "workspace-a" || req.Name != "linear" {
					t.Fatalf("ExchangeMCPAuth target = %#v", req.MCPAuthTargetRequest)
				}
				if req.Code != "sensitive-one-time-code" || req.RedirectURL != "" {
					t.Fatalf("ExchangeMCPAuth input classification is incorrect")
				}
				return confirmedWorkspaceMCPAuthStatus(expiresAt), nil
			},
			LogoutMCPAuthFn: func(
				_ context.Context,
				req settingspkg.MCPAuthTargetRequest,
			) (mcpauth.Status, error) {
				if req.Scope != settingspkg.ScopeWorkspace || req.WorkspaceID != "workspace-a" || req.Name != "linear" {
					t.Fatalf("LogoutMCPAuth target = %#v", req)
				}
				status := confirmedWorkspaceMCPAuthStatus(expiresAt)
				status.Status = mcpauth.StatusNeedsLogin
				status.TokenPresent = false
				return status, nil
			},
			CompleteMCPAuthFn: func(_ context.Context, callbackURL string) (mcpauth.Status, error) {
				if !strings.Contains(callbackURL, "code=sensitive-callback-code") ||
					!strings.Contains(callbackURL, "state=public-state") {
					t.Fatalf("CompleteMCPAuthCallback URL = %q", callbackURL)
				}
				return confirmedWorkspaceMCPAuthStatus(expiresAt), nil
			},
		}
		fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)
		targetPath := "/api/settings/mcp-servers/linear/auth/"
		query := "?scope=workspace&workspace_id=workspace-a"

		begin := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			targetPath+"begin"+query,
			[]byte(`{"mode":"automatic"}`),
		)
		if begin.Code != http.StatusOK {
			t.Fatalf("begin status = %d, want %d; body=%s", begin.Code, http.StatusOK, begin.Body.String())
		}
		var beginPayload contract.SettingsMCPAuthBeginResponse
		decodeJSON(t, begin.Body.Bytes(), &beginPayload)
		if beginPayload.State != "public-state" || !beginPayload.ManualSupported ||
			!beginPayload.ExpiresAt.Equal(expiresAt) {
			t.Fatalf("begin payload = %#v", beginPayload)
		}

		exchange := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			targetPath+"exchange"+query,
			[]byte(`{"code":"sensitive-one-time-code"}`),
		)
		if exchange.Code != http.StatusOK {
			t.Fatalf("exchange status = %d, want %d; body=%s", exchange.Code, http.StatusOK, exchange.Body.String())
		}
		if strings.Contains(exchange.Body.String(), "sensitive-one-time-code") {
			t.Fatalf("exchange response leaked authorization code: %s", exchange.Body.String())
		}
		var exchangePayload contract.SettingsMCPAuthStatusPayload
		decodeJSON(t, exchange.Body.Bytes(), &exchangePayload)
		if exchangePayload.Scope != "workspace" || exchangePayload.WorkspaceID != "workspace-a" ||
			!exchangePayload.TokenPresent {
			t.Fatalf("exchange payload = %#v", exchangePayload)
		}

		logout := performRequest(t, fixture.Engine, http.MethodPost, targetPath+"logout"+query, nil)
		if logout.Code != http.StatusOK {
			t.Fatalf("logout status = %d, want %d; body=%s", logout.Code, http.StatusOK, logout.Body.String())
		}
		var logoutPayload contract.SettingsMCPAuthStatusPayload
		decodeJSON(t, logout.Body.Bytes(), &logoutPayload)
		if logoutPayload.ServerName != "linear" ||
			logoutPayload.Scope != "workspace" ||
			logoutPayload.WorkspaceID != "workspace-a" ||
			logoutPayload.Status != string(mcpauth.StatusNeedsLogin) ||
			logoutPayload.TokenPresent {
			t.Fatalf("logout payload = %#v, want redacted needs-login workspace status", logoutPayload)
		}
		if strings.Contains(logout.Body.String(), "sensitive") {
			t.Fatalf("logout response leaked secret material: %s", logout.Body.String())
		}

		callback := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/api/mcp/oauth/callback?code=sensitive-callback-code&state=public-state",
			nil,
		)
		if callback.Code != http.StatusOK {
			t.Fatalf("callback status = %d, want %d; body=%s", callback.Code, http.StatusOK, callback.Body.String())
		}
		if strings.Contains(callback.Body.String(), "sensitive-callback-code") ||
			strings.Contains(callback.Body.String(), "public-state") {
			t.Fatalf("callback response leaked inbound OAuth material: %s", callback.Body.String())
		}
		if contentType := callback.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
			t.Fatalf("callback Content-Type = %q, want HTML", contentType)
		}
		for _, expected := range []string{
			"<title>Authorization complete</title>",
			"<h1>Authorization complete</h1>",
			"You can close this tab and return to AGH.",
		} {
			if !strings.Contains(callback.Body.String(), expected) {
				t.Fatalf("callback success body missing %q: %s", expected, callback.Body.String())
			}
		}
		service.CompleteMCPAuthFn = func(context.Context, string) (mcpauth.Status, error) {
			return mcpauth.Status{}, errors.New("provider rejected sensitive-failure-code")
		}
		failedCallback := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/api/mcp/oauth/callback?code=sensitive-failure-code&state=public-state",
			nil,
		)
		if failedCallback.Code != http.StatusBadRequest {
			t.Fatalf(
				"failed callback status = %d, want %d; body=%s",
				failedCallback.Code,
				http.StatusBadRequest,
				failedCallback.Body.String(),
			)
		}
		if strings.Contains(failedCallback.Body.String(), "sensitive-failure-code") ||
			strings.Contains(failedCallback.Body.String(), "provider rejected") {
			t.Fatalf("callback failure reflected secret or internal error: %s", failedCallback.Body.String())
		}
		for _, expected := range []string{
			"<title>Authorization failed</title>",
			"<h1>Authorization failed</h1>",
			"Return to AGH and try again, or use manual authorization.",
		} {
			if !strings.Contains(failedCallback.Body.String(), expected) {
				t.Fatalf("callback failure body missing %q: %s", expected, failedCallback.Body.String())
			}
		}
	})
}

func TestSettingsMCPAuthHandlersUseEffectiveLoopbackListener(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		host         string
		mode         contract.SettingsMCPAuthBeginMode
		wantStatus   int
		wantCallback string
		wantCalls    int
	}{
		{
			name:         "Should advertise the configured IPv6 loopback listener",
			host:         "::1",
			mode:         contract.SettingsMCPAuthBeginModeAutomatic,
			wantStatus:   http.StatusOK,
			wantCallback: "http://[::1]:2123/api/mcp/oauth/callback",
			wantCalls:    1,
		},
		{
			name:       "Should reject automatic authorization on a non-loopback listener",
			host:       "0.0.0.0",
			mode:       contract.SettingsMCPAuthBeginModeAutomatic,
			wantStatus: http.StatusServiceUnavailable,
			wantCalls:  0,
		},
		{
			name:         "Should allow manual authorization without a loopback listener",
			host:         "0.0.0.0",
			mode:         contract.SettingsMCPAuthBeginModeManual,
			wantStatus:   http.StatusOK,
			wantCallback: "http://127.0.0.1:2123/api/mcp/oauth/callback",
			wantCalls:    1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service := &stubSettingsService{
				BeginMCPAuthFn: func(
					_ context.Context,
					req settingspkg.MCPAuthBeginRequest,
				) (mcpauth.BeginResult, error) {
					if req.CallbackURL != tc.wantCallback {
						t.Fatalf("BeginMCPAuth callback URL = %q, want %q", req.CallbackURL, tc.wantCallback)
					}
					return mcpauth.BeginResult{
						AuthorizationURL: "https://auth.example/authorize",
						State:            "state",
						ExpiresAt:        time.Date(2026, 7, 16, 21, 0, 0, 0, time.UTC),
						CallbackURL:      req.CallbackURL,
						ManualSupported:  true,
					}, nil
				},
			}
			fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)
			fixture.Handlers.Config.HTTP.Host = tc.host

			response := performRequest(
				t,
				fixture.Engine,
				http.MethodPost,
				"/api/settings/mcp-servers/linear/auth/begin?scope=global",
				[]byte(`{"mode":"`+string(tc.mode)+`"}`),
			)
			if response.Code != tc.wantStatus {
				t.Fatalf("begin status = %d, want %d; body=%s", response.Code, tc.wantStatus, response.Body.String())
			}
			if service.BeginMCPAuthCalls != tc.wantCalls {
				t.Fatalf("BeginMCPAuthCalls = %d, want %d", service.BeginMCPAuthCalls, tc.wantCalls)
			}
		})
	}
}

func TestSettingsMCPAuthHandlersRejectInvalidTargetsAndBodies(t *testing.T) {
	t.Parallel()

	service := &stubSettingsService{}
	fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)
	tests := []struct {
		name string
		path string
		body []byte
	}{
		{
			name: "Should require an explicit scope",
			path: "/api/settings/mcp-servers/linear/auth/begin",
			body: []byte(`{"mode":"automatic"}`),
		},
		{
			name: "Should require a supported begin mode",
			path: "/api/settings/mcp-servers/linear/auth/begin?scope=global",
			body: []byte(`{"mode":"unspecified"}`),
		},
		{
			name: "Should reject unknown exchange fields without echoing their value",
			path: "/api/settings/mcp-servers/linear/auth/exchange?scope=global",
			body: []byte(`{"code":"sensitive-code","verifier":"sensitive-verifier"}`),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			response := performRequest(t, fixture.Engine, http.MethodPost, tc.path, tc.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "sensitive-code") ||
				strings.Contains(response.Body.String(), "sensitive-verifier") {
				t.Fatalf("validation response leaked inbound material: %s", response.Body.String())
			}
		})
	}
	if service.BeginMCPAuthCalls != 0 || service.ExchangeMCPAuthCalls != 0 {
		t.Fatalf(
			"invalid requests reached service: begin=%d exchange=%d",
			service.BeginMCPAuthCalls,
			service.ExchangeMCPAuthCalls,
		)
	}
}

func TestSettingsMCPAuthHandlersMatchAcrossHTTPAndUDSTransportShims(t *testing.T) {
	t.Parallel()

	t.Run("Should match MCP auth responses across HTTP and UDS shims", func(t *testing.T) {
		t.Parallel()

		expiresAt := time.Date(2026, 7, 13, 16, 5, 0, 0, time.UTC)
		serviceFactory := func() *stubSettingsService {
			return &stubSettingsService{
				BeginMCPAuthFn: func(
					_ context.Context,
					req settingspkg.MCPAuthBeginRequest,
				) (mcpauth.BeginResult, error) {
					return mcpauth.BeginResult{
						AuthorizationURL: "https://auth.example/authorize?state=public",
						State:            "public",
						ExpiresAt:        expiresAt,
						CallbackURL:      req.CallbackURL,
						ManualSupported:  true,
					}, nil
				},
				ExchangeMCPAuthFn: func(
					context.Context,
					settingspkg.MCPAuthExchangeRequest,
				) (mcpauth.Status, error) {
					return confirmedWorkspaceMCPAuthStatus(expiresAt), nil
				},
				LogoutMCPAuthFn: func(
					context.Context,
					settingspkg.MCPAuthTargetRequest,
				) (mcpauth.Status, error) {
					status := confirmedWorkspaceMCPAuthStatus(expiresAt)
					status.Status = mcpauth.StatusNeedsLogin
					status.TokenPresent = false
					return status, nil
				},
			}
		}
		httpFixture := newSettingsHandlerFixture(t, "httpapi", serviceFactory(), nil)
		udsFixture := newSettingsHandlerFixture(t, "udsapi", serviceFactory(), nil)
		for _, request := range []struct {
			path string
			body []byte
		}{
			{
				path: "/api/settings/mcp-servers/linear/auth/begin?scope=workspace&workspace_id=workspace-a",
				body: []byte(`{"mode":"automatic"}`),
			},
			{
				path: "/api/settings/mcp-servers/linear/auth/exchange?scope=workspace&workspace_id=workspace-a",
				body: []byte(`{"code":"opaque-code"}`),
			},
			{
				path: "/api/settings/mcp-servers/linear/auth/logout?scope=workspace&workspace_id=workspace-a",
			},
		} {
			httpResponse := performRequest(t, httpFixture.Engine, http.MethodPost, request.path, request.body)
			udsResponse := performRequest(t, udsFixture.Engine, http.MethodPost, request.path, request.body)
			if httpResponse.Code != udsResponse.Code || httpResponse.Body.String() != udsResponse.Body.String() {
				t.Fatalf(
					"transport mismatch for %s: http=%d %s uds=%d %s",
					request.path,
					httpResponse.Code,
					httpResponse.Body.String(),
					udsResponse.Code,
					udsResponse.Body.String(),
				)
			}
		}
	})
}

func confirmedWorkspaceMCPAuthStatus(expiresAt time.Time) mcpauth.Status {
	return mcpauth.Status{
		ServerName:   "linear",
		Scope:        mcpauth.ScopeWorkspace,
		WorkspaceID:  "workspace-a",
		Status:       mcpauth.StatusAuthenticated,
		TokenPresent: true,
		ExpiresAt:    &expiresAt,
	}
}

func TestStatusForSettingsError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: http.StatusOK},
		{
			name: "validation sentinel",
			err:  core.NewSettingsValidationError(errors.New("bad payload")),
			want: http.StatusBadRequest,
		},
		{
			name: "not found sentinel",
			err:  core.NewSettingsNotFoundError(errors.New("missing")),
			want: http.StatusNotFound,
		},
		{
			name: "conflict sentinel",
			err:  core.NewSettingsConflictError(errors.New("conflict")),
			want: http.StatusConflict,
		},
		{
			name: "forbidden sentinel",
			err:  core.NewSettingsForbiddenError(errors.New("forbidden")),
			want: http.StatusForbidden,
		},
		{name: "workspace missing", err: workspacepkg.ErrWorkspaceNotFound, want: http.StatusNotFound},
		{
			name: "settings conflict sentinel",
			err: fmt.Errorf(
				"%w: %s",
				settingspkg.ErrConflict,
				`settings: section "general" does not support workspace scope`,
			),
			want: http.StatusConflict,
		},
		{
			name: "settings validation sentinel",
			err: fmt.Errorf(
				"%w: %s",
				settingspkg.ErrValidation,
				"settings: decode network settings request: bad json",
			),
			want: http.StatusBadRequest,
		},
		{
			name: "Should map settings unavailability to 503",
			err:  settingspkg.ErrUnavailable,
			want: http.StatusServiceUnavailable,
		},
		{
			name: "settings forbidden sentinel",
			err:  fmt.Errorf("%w: %s", settingspkg.ErrForbidden, "settings mutations are forbidden for this transport"),
			want: http.StatusForbidden,
		},
		{
			name: "Should map model catalog unavailability to 503",
			err:  fmt.Errorf("settings projection: %w", modelcatalog.ErrAllSourcesFailed),
			want: http.StatusServiceUnavailable,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := core.StatusForSettingsError(tc.err); got != tc.want {
				t.Fatalf("StatusForSettingsError(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestGetSettingsUpdateReturnsCurrentSnapshot(t *testing.T) {
	t.Parallel()

	fixture := newSettingsHandlerFixture(t, "api-core-http", &stubSettingsService{}, nil)
	fixture.Update.GetFn = func(context.Context) (core.SettingsUpdateStatus, error) {
		checkedAt := time.Date(2026, 5, 3, 19, 0, 0, 0, time.UTC)
		return core.SettingsUpdateStatus{
			Supported:      true,
			Managed:        false,
			InstallMethod:  "direct-binary",
			CurrentVersion: "v1.0.0",
			LatestVersion:  "v1.1.0",
			Available:      true,
			Status:         "available",
			Recommendation: "Run `agh update`.",
			ReleaseURL:     "https://github.com/compozy/agh/releases/tag/v1.1.0",
			CheckedAt:      &checkedAt,
		}, nil
	}

	resp := performRequest(t, fixture.Engine, http.MethodGet, "/api/settings/update", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /api/settings/update status = %d, want 200", resp.Code)
	}

	var payload contract.SettingsUpdateResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(update response) error = %v", err)
	}
	if payload.Status != contract.SettingsUpdateStatusAvailable || payload.InstallMethod != "direct-binary" {
		t.Fatalf("update payload = %#v, want available direct-binary", payload)
	}
	if fixture.Update.GetCalls != 1 {
		t.Fatalf("GetCalls = %d, want 1", fixture.Update.GetCalls)
	}
}

func TestSettingsSectionAndCollectionConversions(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 4, 17, 18, 0, 0, 0, time.UTC)
	lastConsolidatedAt := time.Date(2026, 4, 17, 17, 0, 0, 0, time.UTC)
	nextFire := time.Date(2026, 4, 17, 19, 0, 0, 0, time.UTC)
	lastSyncedAt := time.Date(2026, 4, 17, 18, 30, 0, 0, time.UTC)
	readOnly := true

	sectionEnvelopes := []settingspkg.SectionEnvelope{
		{
			Section:         settingspkg.SectionGeneral,
			Scope:           settingspkg.ScopeGlobal,
			AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal},
			General: &settingspkg.GeneralSection{
				Runtime: settingspkg.DaemonRuntimeStatus{
					Available:      true,
					Status:         "running",
					PID:            4100,
					StartedAt:      startedAt,
					UptimeSeconds:  900,
					Socket:         "/tmp/agh.sock",
					HTTPHost:       "127.0.0.1",
					HTTPPort:       2123,
					ActiveSessions: 2,
					ActiveAgents:   1,
					TotalSessions:  3,
					Version:        "test-version",
				},
				ConfigPaths: settingspkg.ConfigPaths{
					HomeDir:          "/tmp/home",
					GlobalConfig:     "/tmp/home/config.toml",
					GlobalMCPSidecar: "/tmp/home/mcp.json",
					LogFile:          "/tmp/home/agh.log",
					DaemonInfo:       "/tmp/home/daemon.json",
				},
				Settings: settingspkg.GeneralSettings{
					Defaults: aghconfig.DefaultsConfig{Agent: "coder", Provider: "openai", Sandbox: "local"},
					Limits:   aghconfig.LimitsConfig{MaxConcurrentAgents: 2},
					Permissions: aghconfig.PermissionsConfig{
						Mode: aghconfig.PermissionModeApproveReads,
					},
					SessionTimeout: 30 * time.Minute,
					HTTP:           aghconfig.HTTPConfig{Host: "127.0.0.1", Port: 2123},
					Daemon:         aghconfig.DaemonConfig{Socket: "/tmp/agh.sock"},
				},
				Actions: settingspkg.GeneralActions{
					Restart: settingspkg.ActionMetadata{
						Name:      "restart",
						Available: true,
						Behavior:  settingspkg.MutationBehaviorActionTrigger,
					},
				},
			},
		},
		{
			Section:         settingspkg.SectionMemory,
			Scope:           settingspkg.ScopeGlobal,
			AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal},
			Memory: &settingspkg.MemorySection{
				Config: aghconfig.MemoryConfig{
					Enabled:   true,
					GlobalDir: "/tmp/home/memory",
					Dream: aghconfig.DreamConfig{
						MinHours:      1.5,
						MinSessions:   2,
						CheckInterval: time.Hour,
					},
				},
				Health: settingspkg.MemoryHealthStatus{
					Available:          true,
					FileCount:          4,
					DreamEnabled:       true,
					LastConsolidatedAt: &lastConsolidatedAt,
				},
				Actions: settingspkg.MemoryActions{
					Consolidate: settingspkg.ActionMetadata{
						Name:      "consolidate",
						Available: true,
						Behavior:  settingspkg.MutationBehaviorActionTrigger,
					},
				},
			},
		},
		{
			Section:         settingspkg.SectionSkills,
			Scope:           settingspkg.ScopeGlobal,
			AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal},
			Skills: &settingspkg.SkillsSection{
				Config: aghconfig.SkillsConfig{
					Enabled:      true,
					PollInterval: time.Minute,
					Marketplace: aghconfig.MarketplaceConfig{
						Registry: "registry.example",
						BaseURL:  "https://registry.example",
					},
				},
				DiscoveredCount:  5,
				DisabledCount:    1,
				RuntimeAvailable: true,
				Links: []settingspkg.OperationalLink{{
					Label: "skills",
					Path:  "/marketplace/skills?tab=installed",
				}},
			},
		},
		{
			Section:         settingspkg.SectionAutomation,
			Scope:           settingspkg.ScopeGlobal,
			AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal},
			Automation: &settingspkg.AutomationSection{
				Config: settingspkg.AutomationSettings{
					Enabled:           true,
					Timezone:          "UTC",
					MaxConcurrentJobs: 2,
					DefaultFireLimit:  automationmodel.FireLimitConfig{Max: 5, Window: "1m"},
				},
				Runtime: settingspkg.AutomationRuntimeStatus{
					Available:        true,
					Running:          true,
					SchedulerRunning: true,
					JobTotal:         3,
					JobEnabled:       2,
					TriggerTotal:     4,
					TriggerEnabled:   3,
					NextFire:         &nextFire,
					LastSyncedAt:     &lastSyncedAt,
				},
				Links: []settingspkg.OperationalLink{{Label: "automation", Path: "/automation"}},
			},
		},
		{
			Section:         settingspkg.SectionNetwork,
			Scope:           settingspkg.ScopeGlobal,
			AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal},
			Network: &settingspkg.NetworkSection{
				Config: aghconfig.NetworkConfig{
					Enabled:      true,
					MaxReplayAge: 10,
					Live:         aghconfig.DefaultNetworkConfig().Live,
				},
				Runtime: settingspkg.NetworkRuntimeStatus{
					Available:         true,
					Enabled:           true,
					Status:            "active",
					LocalPeers:        1,
					Channels:          3,
					MessagesReceived:  4,
					MessagesDelivered: 2,
					MessagesRejected:  1,
				},
				Links: []settingspkg.OperationalLink{{Label: "network", Path: "/network"}},
			},
		},
		{
			Section:         settingspkg.SectionWindowManager,
			Scope:           settingspkg.ScopeGlobal,
			AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal},
			WindowManager: &settingspkg.WindowManagerSection{
				Config: aghconfig.WindowManagerConfig{
					NewWindowPolicy:     aghconfig.WindowNewPolicyBesideFocus,
					SmallViewportPolicy: aghconfig.WindowSmallViewportReject,
					FocusPolicy:         aghconfig.WindowFocusDirectional,
					FocusWrap:           true,
					FocusFollowsPointer: true,
					RaiseOnFocus:        false,
					DragAwayPolicy:      aghconfig.WindowDragAwayGroup,
					GroupMoveModifier:   "control",
					SwapModifier:        "meta",
					HistoryLimit:        77,
					DesktopTransition:   aghconfig.WindowDesktopTransitionCrossfade,
					Gaps: aghconfig.WindowManagerGapsConfig{
						Inner: 12, Top: 18, Right: 14, Bottom: 16, Left: 20,
					},
					Snap: aghconfig.WindowManagerSnapConfig{
						EdgeBand:     40,
						CornerReach:  180,
						ExitSlack:    20,
						RepeatRatios: []float64{0.4, 0.7, 0.3},
					},
					Bindings: aghconfig.WindowManagerBindingConfig{
						TopCenter: "none", BottomCenter: "zoom",
					},
					Shortcuts: map[string]string{"desktop.switch.next": "Meta+ArrowRight"},
				},
			},
		},
		{
			Section:         settingspkg.SectionObservability,
			Scope:           settingspkg.ScopeGlobal,
			AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal},
			Observability: &settingspkg.ObservabilitySection{
				Config: aghconfig.ObservabilityConfig{
					Enabled:        true,
					RetentionDays:  7,
					MaxGlobalBytes: 2048,
					Transcripts: aghconfig.ObservabilityTranscriptConfig{
						Enabled:            true,
						SegmentBytes:       4096,
						MaxBytesPerSession: 8192,
					},
				},
				Runtime: settingspkg.ObservabilityRuntimeStatus{
					Available:          true,
					Status:             "ok",
					GlobalDBSizeBytes:  10,
					SessionDBSizeBytes: 20,
					ActiveSessions:     2,
					ActiveAgents:       1,
					UptimeSeconds:      600,
				},
				LogTailSupport: settingspkg.CapabilityStatus{Available: true},
			},
		},
		{
			Section:         settingspkg.SectionHooksExtensions,
			Scope:           settingspkg.ScopeGlobal,
			AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal},
			HooksExtensions: &settingspkg.HooksExtensionsSection{
				Hooks: []settingspkg.HookItem{{
					Name: "capture-tool-call",
					Declaration: hookspkg.HookDecl{
						Name:         "capture-tool-call",
						Event:        hookspkg.HookToolPreCall,
						Mode:         hookspkg.HookModeAsync,
						ExecutorKind: hookspkg.HookExecutorSubprocess,
						Command:      "/bin/capture",
						Matcher: hookspkg.HookMatcher{
							ToolID: "agh__read",
						},
					},
					SourceMetadata: settingspkg.SourceMetadata{
						EffectiveSource: settingspkg.SourceRef{
							Kind:  settingspkg.SourceKindGlobalConfig,
							Scope: settingspkg.ScopeGlobal,
						},
						AvailableTargets: []settingspkg.WriteTargetKind{settingspkg.WriteTargetGlobalConfig},
					},
				}},
				Extensions: aghconfig.ExtensionsConfig{
					Marketplace: aghconfig.ExtensionsMarketplaceConfig{
						Registry: "extensions.example",
						BaseURL:  "https://extensions.example",
					},
					Resources: aghconfig.ExtensionsResourcesConfig{
						AllowedKinds: []resources.ResourceKind{resources.ResourceKind("tool")},
						MaxScope:     resources.ResourceScopeKindWorkspace,
						SnapshotRateLimit: aghconfig.ExtensionsResourceRateLimitConfig{
							Requests: 10,
							Window:   time.Minute,
							Queue:    2,
						},
						OperatorWriteRateLimit: aghconfig.ExtensionsResourceRateLimitConfig{
							Requests: 20,
							Window:   time.Minute,
							Queue:    4,
						},
					},
				},
				Installed: []settingspkg.InstalledExtension{{
					Name:          "ext-a",
					Version:       "1.0.0",
					Enabled:       true,
					State:         "active",
					Health:        "ok",
					HealthMessage: "healthy",
				}},
				TransportParity: settingspkg.TransportParityStatus{
					Known:          true,
					SettingsHTTP:   true,
					SettingsUDS:    true,
					ExtensionsHTTP: true,
					ExtensionsUDS:  true,
				},
			},
		},
	}

	for _, envelope := range sectionEnvelopes {
		t.Run("Should section/"+string(envelope.Section), func(t *testing.T) {
			t.Parallel()
			response, err := core.SettingsSectionResponseFromEnvelope(envelope)
			if err != nil {
				t.Fatalf("SettingsSectionResponseFromEnvelope(%s) error = %v", envelope.Section, err)
			}
			if envelope.Section != settingspkg.SectionWindowManager {
				return
			}
			payload, ok := response.(contract.SettingsWindowManagerResponse)
			if !ok {
				t.Fatalf("window-manager response = %T, want SettingsWindowManagerResponse", response)
			}
			want := contract.SettingsWindowManagerConfigPayload{
				NewWindowPolicy:     contract.SettingsWindowNewPolicyBesideFocus,
				SmallViewportPolicy: contract.SettingsWindowSmallViewportPolicyReject,
				FocusPolicy:         contract.SettingsWindowFocusPolicyDirectional,
				FocusWrap:           true,
				FocusFollowsPointer: true,
				RaiseOnFocus:        false,
				DragAwayPolicy:      contract.SettingsWindowDragAwayPolicyGroup,
				GroupMoveModifier:   contract.SettingsWindowDragModifierControl,
				SwapModifier:        contract.SettingsWindowDragModifierMeta,
				HistoryLimit:        77,
				DesktopTransition:   contract.SettingsWindowDesktopTransitionCrossfade,
				Gaps: contract.SettingsWindowManagerGapsPayload{
					Inner: 12, Top: 18, Right: 14, Bottom: 16, Left: 20,
				},
				Snap: contract.SettingsWindowManagerSnapPayload{
					EdgeBand: 40, CornerReach: 180, ExitSlack: 20,
					RepeatRatios: []float64{0.4, 0.7, 0.3},
				},
				Bindings: contract.SettingsWindowManagerBindingPayload{
					TopCenter:    contract.SettingsWindowBindingActionNone,
					BottomCenter: contract.SettingsWindowBindingActionZoom,
				},
				Shortcuts: map[string]string{"desktop.switch.next": "Meta+ArrowRight"},
			}
			if !reflect.DeepEqual(payload.Config, want) {
				t.Fatalf("window-manager response config = %#v, want complete conversion", payload.Config)
			}
		})
	}

	collectionEnvelopes := []settingspkg.CollectionEnvelope{
		{
			Collection:      settingspkg.CollectionProviders,
			Scope:           settingspkg.ScopeGlobal,
			AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal},
			Providers: []settingspkg.ProviderItem{
				{
					Name: "openai",
					Settings: settingspkg.ProviderSettings{
						Command: "codex",
						Models: aghconfig.ProviderModelsConfig{
							Default: "gpt-5.4",
						},
						CredentialSlots: []aghconfig.ProviderCredentialSlot{
							{
								Name:      "api_key",
								TargetEnv: "OPENAI_API_KEY",
								SecretRef: "env:OPENAI_API_KEY",
								Kind:      "api_key",
								Required:  true,
							},
						},
					},
					Default:          true,
					CommandAvailable: true,
					Credentials: []settingspkg.ProviderCredentialStatus{
						{
							Name:      "api_key",
							TargetEnv: "OPENAI_API_KEY",
							SecretRef: "env:OPENAI_API_KEY",
							Kind:      "api_key",
							Required:  true,
							Present:   true,
							Source:    "env",
						},
					},
					SourceMetadata: settingspkg.SourceMetadata{
						EffectiveSource: settingspkg.SourceRef{
							Kind:  settingspkg.SourceKindGlobalConfig,
							Scope: settingspkg.ScopeGlobal,
						},
						AvailableTargets: []settingspkg.WriteTargetKind{
							settingspkg.WriteTargetGlobalConfig,
						},
					},
					Fallback: &settingspkg.ProviderFallback{
						Source: settingspkg.SourceRef{
							Kind:  settingspkg.SourceKindBuiltinProvider,
							Scope: settingspkg.ScopeGlobal,
						},
						Settings: settingspkg.ProviderSettings{
							Command: "codex",
							Models: aghconfig.ProviderModelsConfig{
								Default: "gpt-5.4",
							},
						},
					},
				},
			},
		},
		{
			Collection:      settingspkg.CollectionMCPServers,
			Scope:           settingspkg.ScopeWorkspace,
			WorkspaceID:     "ws-1",
			AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal, settingspkg.ScopeWorkspace},
			MCPServers: []settingspkg.MCPServerItem{{
				Name:        "memory",
				Command:     "memoryd",
				Args:        []string{"serve"},
				EnvKeys:     []string{"TOKEN"},
				Scope:       settingspkg.ScopeWorkspace,
				WorkspaceID: "ws-1",
				SourceMetadata: settingspkg.SourceMetadata{
					EffectiveSource: settingspkg.SourceRef{
						Kind:        settingspkg.SourceKindWorkspaceMCPSidecar,
						Scope:       settingspkg.ScopeWorkspace,
						WorkspaceID: "ws-1",
					},
					AvailableTargets: []settingspkg.WriteTargetKind{settingspkg.WriteTargetWorkspaceMCPSidecar},
				},
			}},
		},
		{
			Collection:      settingspkg.CollectionSandboxes,
			Scope:           settingspkg.ScopeGlobal,
			AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal},
			Sandboxes: []settingspkg.SandboxItem{{
				Name: "local",
				Profile: aghconfig.SandboxProfile{
					Backend:     "local",
					SyncMode:    "session-bidirectional",
					Persistence: "reuse",
					RuntimeRoot: "/workspace",
				},
				WorkspaceUsageCount: 2,
				SourceMetadata: settingspkg.SourceMetadata{
					EffectiveSource: settingspkg.SourceRef{
						Kind:  settingspkg.SourceKindGlobalConfig,
						Scope: settingspkg.ScopeGlobal,
					},
					AvailableTargets: []settingspkg.WriteTargetKind{
						settingspkg.WriteTargetGlobalConfig,
					},
				},
			}},
		},
		{
			Collection:      settingspkg.CollectionHooks,
			Scope:           settingspkg.ScopeGlobal,
			AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal},
			Hooks: []settingspkg.HookItem{{
				Name: "capture",
				Declaration: hookspkg.HookDecl{
					Name:         "capture",
					Event:        hookspkg.HookToolPreCall,
					Mode:         hookspkg.HookModeAsync,
					ExecutorKind: hookspkg.HookExecutorSubprocess,
					Command:      "/bin/capture",
					Matcher: hookspkg.HookMatcher{
						ToolID:           "agh__read",
						ToolReadOnly:     &readOnly,
						MessageRole:      "assistant",
						MessageDeltaType: "text",
					},
				},
				SourceMetadata: settingspkg.SourceMetadata{
					EffectiveSource: settingspkg.SourceRef{
						Kind:  settingspkg.SourceKindGlobalConfig,
						Scope: settingspkg.ScopeGlobal,
					},
					AvailableTargets: []settingspkg.WriteTargetKind{
						settingspkg.WriteTargetGlobalConfig,
					},
				},
			}},
		},
	}

	for _, envelope := range collectionEnvelopes {
		t.Run("Should collection/"+string(envelope.Collection), func(t *testing.T) {
			t.Parallel()
			if _, err := core.SettingsCollectionResponseFromEnvelope(envelope); err != nil {
				t.Fatalf("SettingsCollectionResponseFromEnvelope(%s) error = %v", envelope.Collection, err)
			}
		})
	}
}

func TestUpdateSettingsGeneralRejectsInvalidPayload(t *testing.T) {
	t.Parallel()

	fixture := newSettingsHandlerFixture(t, "api-core-http", &stubSettingsService{}, nil)
	body := mustJSON(t, map[string]any{
		"config": map[string]any{
			"defaults": map[string]any{
				"agent": "coder",
			},
			"limits": map[string]any{
				"max_concurrent_agents": 2,
			},
			"permissions": map[string]any{
				"mode": "approve-reads",
			},
			"session_timeout": "not-a-duration",
			"http": map[string]any{
				"host": "127.0.0.1",
				"port": 2123,
			},
			"daemon": map[string]any{
				"socket": "/tmp/agh.sock",
			},
			"redact": map[string]any{
				"enabled": true,
			},
		},
	})

	resp := performRequest(t, fixture.Engine, http.MethodPatch, "/api/settings/general", body)
	if got, want := resp.Code, http.StatusBadRequest; got != want {
		t.Fatalf("status = %d, want %d; body=%s", got, want, resp.Body.String())
	}
	if fixture.Service.UpdateSectionCalls != 0 {
		t.Fatalf("UpdateSectionCalls = %d, want 0", fixture.Service.UpdateSectionCalls)
	}

	var payload contract.ErrorPayload
	decodeJSON(t, resp.Body.Bytes(), &payload)
	if !strings.Contains(payload.Error, "general.config.session_timeout") {
		t.Fatalf("payload.Error = %q, want section-specific context", payload.Error)
	}
}

func TestUpdateSettingsGeneralRequiresRedactionGatePresence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		redact  any
		wantErr string
	}{
		{
			name:    "Should reject an omitted redact section",
			wantErr: "general.config.redact is required",
		},
		{
			name:    "Should reject an omitted enabled field",
			redact:  map[string]any{},
			wantErr: "general.config.redact.enabled is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &stubSettingsService{}
			fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)
			config := map[string]any{
				"defaults":        map[string]any{"agent": "coder"},
				"limits":          map[string]any{"max_concurrent_agents": 2},
				"permissions":     map[string]any{"mode": "approve-reads"},
				"session_timeout": "30m",
				"http":            map[string]any{"host": "127.0.0.1", "port": 2123},
				"daemon":          map[string]any{"socket": "/tmp/agh.sock"},
			}
			if tt.redact != nil {
				config["redact"] = tt.redact
			}
			body := mustJSON(t, map[string]any{"config": config})

			resp := performRequest(t, fixture.Engine, http.MethodPatch, "/api/settings/general", body)
			if got, want := resp.Code, http.StatusBadRequest; got != want {
				t.Fatalf("status = %d, want %d; body=%s", got, want, resp.Body.String())
			}
			if service.UpdateSectionCalls != 0 {
				t.Fatalf("UpdateSectionCalls = %d, want 0", service.UpdateSectionCalls)
			}
			var payload contract.ErrorPayload
			decodeJSON(t, resp.Body.Bytes(), &payload)
			if !strings.Contains(payload.Error, tt.wantErr) {
				t.Fatalf("payload.Error = %q, want substring %q", payload.Error, tt.wantErr)
			}
		})
	}
}

func TestUpdateSettingsSectionHandlersRejectInvalidPayloads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "general", path: "/api/settings/general", want: "general.config is required"},
		{name: "memory", path: "/api/settings/memory", want: "memory.config is required"},
		{name: "Should require roles config", path: "/api/settings/roles", want: "roles.config is required"},
		{name: "skills", path: "/api/settings/skills", want: "skills.config is required"},
		{name: "automation", path: "/api/settings/automation", want: "automation.config is required"},
		{name: "network", path: "/api/settings/network", want: "network.config is required"},
		{
			name: "Should require window-manager config",
			path: "/api/settings/window-manager",
			want: "window-manager.config is required",
		},
		{name: "observability", path: "/api/settings/observability", want: "observability.config is required"},
		{name: "hooks extensions", path: "/api/settings/hooks-extensions", want: "hooks-extensions.config is required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service := &stubSettingsService{}
			fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)

			resp := performRequest(t, fixture.Engine, http.MethodPatch, tc.path, []byte(`{}`))
			if got, want := resp.Code, http.StatusBadRequest; got != want {
				t.Fatalf("status = %d, want %d; body=%s", got, want, resp.Body.String())
			}
			if service.UpdateSectionCalls != 0 {
				t.Fatalf("UpdateSectionCalls = %d, want 0", service.UpdateSectionCalls)
			}

			var payload contract.ErrorPayload
			decodeJSON(t, resp.Body.Bytes(), &payload)
			if !strings.Contains(payload.Error, tc.want) {
				t.Fatalf("payload.Error = %q, want substring %q", payload.Error, tc.want)
			}
		})
	}

	for _, field := range []string{"default_channel", "port"} {
		t.Run("network removed field "+field, func(t *testing.T) {
			t.Parallel()

			service := &stubSettingsService{}
			fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)
			body := fmt.Appendf(nil, `{"config":{%q:"removed"}}`, field)

			resp := performRequest(t, fixture.Engine, http.MethodPatch, "/api/settings/network", body)
			if got, want := resp.Code, http.StatusBadRequest; got != want {
				t.Fatalf("status = %d, want %d; body=%s", got, want, resp.Body.String())
			}
			if service.UpdateSectionCalls != 0 {
				t.Fatalf("UpdateSectionCalls = %d, want 0", service.UpdateSectionCalls)
			}
			var payload contract.ErrorPayload
			decodeJSON(t, resp.Body.Bytes(), &payload)
			if !strings.Contains(payload.Error, "unknown_field") || !strings.Contains(payload.Error, field) {
				t.Fatalf("payload.Error = %q, want unknown_field naming %q", payload.Error, field)
			}
		})
	}

	t.Run("Should reject window-manager stack without an edge-center target", func(t *testing.T) {
		t.Parallel()

		service := &stubSettingsService{}
		fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)
		config := validSettingsWindowManagerConfigPayload()
		config.Bindings.TopCenter = contract.SettingsWindowBindingAction("stack")

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPatch,
			"/api/settings/window-manager",
			mustJSON(t, contract.UpdateSettingsWindowManagerRequest{Config: config}),
		)
		if got, want := resp.Code, http.StatusBadRequest; got != want {
			t.Fatalf("status = %d, want %d; body=%s", got, want, resp.Body.String())
		}
		if service.UpdateSectionCalls != 0 {
			t.Fatalf("UpdateSectionCalls = %d, want 0", service.UpdateSectionCalls)
		}
		var payload contract.ErrorPayload
		decodeJSON(t, resp.Body.Bytes(), &payload)
		if !strings.Contains(payload.Error, "window_manager.bindings.top_center") {
			t.Fatalf("payload.Error = %q, want binding validation path", payload.Error)
		}
	})
}

func TestUpdateSettingsMemoryRejectsUnavailableProvider(t *testing.T) {
	t.Parallel()

	t.Run("Should reject unknown provider names as validation errors", func(t *testing.T) {
		t.Parallel()

		service := &stubSettingsService{}
		fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)
		fixture.Handlers.MemoryProviders = unavailableSettingsMemoryProviderService{
			err: extensionpkg.ErrMemoryProviderNotFound,
		}

		memoryPayload := validSettingsMemoryConfigPayload()
		memoryPayload.Provider.Name = "qa-missing-provider"
		body := contract.UpdateSettingsMemoryRequest{Config: memoryPayload}

		resp := performRequest(t, fixture.Engine, http.MethodPatch, "/api/settings/memory", mustJSON(t, body))
		if got, want := resp.Code, http.StatusBadRequest; got != want {
			t.Fatalf("status = %d, want %d; body=%s", got, want, resp.Body.String())
		}
		if service.UpdateSectionCalls != 0 {
			t.Fatalf("UpdateSectionCalls = %d, want 0", service.UpdateSectionCalls)
		}

		var payload contract.ErrorPayload
		decodeJSON(t, resp.Body.Bytes(), &payload)
		if !strings.Contains(payload.Error, "qa-missing-provider") {
			t.Fatalf("payload.Error = %q, want provider name", payload.Error)
		}
	})

	t.Run("Should preserve provider lookup infrastructure failures as server errors", func(t *testing.T) {
		t.Parallel()

		service := &stubSettingsService{}
		fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)
		fixture.Handlers.MemoryProviders = unavailableSettingsMemoryProviderService{
			err: errors.New("catalog backend offline"),
		}

		memoryPayload := validSettingsMemoryConfigPayload()
		memoryPayload.Provider.Name = "qa-provider"
		body := contract.UpdateSettingsMemoryRequest{Config: memoryPayload}

		resp := performRequest(t, fixture.Engine, http.MethodPatch, "/api/settings/memory", mustJSON(t, body))
		if got, want := resp.Code, http.StatusInternalServerError; got != want {
			t.Fatalf("status = %d, want %d; body=%s", got, want, resp.Body.String())
		}
		if service.UpdateSectionCalls != 0 {
			t.Fatalf("UpdateSectionCalls = %d, want 0", service.UpdateSectionCalls)
		}

		var payload contract.ErrorPayload
		decodeJSON(t, resp.Body.Bytes(), &payload)
		if !strings.Contains(payload.Error, "catalog backend offline") {
			t.Fatalf("payload.Error = %q, want backend failure", payload.Error)
		}
	})
}

func TestUpdateSettingsSectionHandlersDelegateValidPayloads(t *testing.T) {
	t.Parallel()

	memoryPayload := validSettingsMemoryConfigPayload()
	memoryPayload.GlobalDir = "/tmp/memory"
	memoryPayload.Dream.MinHours = 1.5
	memoryPayload.Dream.MinSessions = 2
	memoryPayload.Dream.CheckInterval = "1h"

	tests := []struct {
		name   string
		path   string
		body   any
		assert func(t *testing.T, req settingspkg.SectionUpdateRequest)
	}{
		{
			name: "general",
			path: "/api/settings/general",
			body: contract.UpdateSettingsGeneralRequest{
				Config: contract.SettingsGeneralConfigPayload{
					Defaults: contract.SettingsDefaultsPayload{
						Agent:    "coder",
						Provider: "openai",
						Sandbox:  "local",
					},
					Limits: contract.SettingsLimitsPayload{MaxConcurrentAgents: 2},
					Permissions: contract.SettingsPermissionsPayload{
						Mode: contract.SettingsPermissionModeApproveReads,
					},
					SessionTimeout: "30m",
					HTTP:           contract.SettingsHTTPPayload{Host: "127.0.0.1", Port: 2123},
					Daemon:         contract.SettingsDaemonPayload{Socket: "/tmp/agh.sock"},
				},
			},
			assert: func(t *testing.T, req settingspkg.SectionUpdateRequest) {
				t.Helper()
				if req.General == nil || req.General.Defaults.Agent != "coder" {
					t.Fatalf("req.General = %#v, want populated general settings", req.General)
				}
			},
		},
		{
			name: "memory",
			path: "/api/settings/memory",
			body: contract.UpdateSettingsMemoryRequest{
				Config: memoryPayload,
			},
			assert: func(t *testing.T, req settingspkg.SectionUpdateRequest) {
				t.Helper()
				if req.Memory == nil || req.Memory.Dream.MinHours != 1.5 {
					t.Fatalf("req.Memory = %#v, want populated memory config", req.Memory)
				}
			},
		},
		{
			name: "roles",
			path: "/api/settings/roles",
			body: contract.UpdateSettingsRolesRequest{
				Config: validSettingsRolesConfigPayload(),
			},
			assert: func(t *testing.T, req settingspkg.SectionUpdateRequest) {
				t.Helper()
				if req.Roles == nil || req.Roles.Dream.Model != "claude-opus" ||
					req.Roles.Coordinator.TTL != 2*time.Hour ||
					len(req.Roles.AutoTitle.FallbackChain) != 1 ||
					req.Roles.AutoTitle.FallbackChain[0].Model != "gpt-5-mini" {
					t.Fatalf("req.Roles = %#v, want complete role settings", req.Roles)
				}
			},
		},
		{
			name: "skills",
			path: "/api/settings/skills",
			body: contract.UpdateSettingsSkillsRequest{
				Config: contract.SettingsSkillsConfigPayload{
					Enabled:      true,
					PollInterval: "1m",
					Marketplace: contract.SettingsMarketplacePayload{
						Registry: "clawhub",
						BaseURL:  "https://registry.example",
					},
				},
			},
			assert: func(t *testing.T, req settingspkg.SectionUpdateRequest) {
				t.Helper()
				if req.Skills == nil || req.Skills.Marketplace.Registry != "clawhub" {
					t.Fatalf("req.Skills = %#v, want populated skills config", req.Skills)
				}
			},
		},
		{
			name: "automation",
			path: "/api/settings/automation",
			body: contract.UpdateSettingsAutomationRequest{
				Config: contract.SettingsAutomationConfigPayload{
					Enabled:           true,
					Timezone:          "UTC",
					MaxConcurrentJobs: 2,
					DefaultFireLimit:  automationmodel.FireLimitConfig{Max: 5, Window: "1m"},
				},
			},
			assert: func(t *testing.T, req settingspkg.SectionUpdateRequest) {
				t.Helper()
				if req.Automation == nil || req.Automation.DefaultFireLimit.Max != 5 {
					t.Fatalf("req.Automation = %#v, want populated automation config", req.Automation)
				}
			},
		},
		{
			name: "network",
			path: "/api/settings/network",
			body: contract.UpdateSettingsNetworkRequest{
				Config: contract.SettingsNetworkConfigPayload{
					Enabled:      true,
					MaxReplayAge: 10,
					Live: contract.SettingsNetworkLiveConfigPayload{
						Defaults: contract.SettingsNetworkLiveDefaultsPayload{
							MaxWakes:         8,
							MaxWakeWallTime:  "5m",
							MaxTotalWallTime: "30m",
							MaxInputTokens:   200_000,
							MaxOutputTokens:  50_000,
							MaxWakeDepth:     3,
							CoalesceWindow:   "500ms",
						},
						Limits: contract.SettingsNetworkLiveLimitsPayload{
							MaxWakes:          64,
							MaxWakeWallTime:   "15m",
							MaxTotalWallTime:  "2h",
							MaxInputTokens:    1_000_000,
							MaxOutputTokens:   200_000,
							MaxWakeDepth:      5,
							MinCoalesceWindow: "100ms",
							MaxCoalesceWindow: "5s",
						},
					},
				},
			},
			assert: func(t *testing.T, req settingspkg.SectionUpdateRequest) {
				t.Helper()
				if req.Network == nil || req.Network.MaxReplayAge != 10 {
					t.Fatalf("req.Network = %#v, want populated network config", req.Network)
				}
			},
		},
		{
			name: "Should delegate window-manager config",
			path: "/api/settings/window-manager",
			body: contract.UpdateSettingsWindowManagerRequest{
				Config: validSettingsWindowManagerConfigPayload(),
			},
			assert: func(t *testing.T, req settingspkg.SectionUpdateRequest) {
				t.Helper()
				if req.WindowManager == nil ||
					req.WindowManager.HistoryLimit != 77 ||
					!reflect.DeepEqual(req.WindowManager.Snap.RepeatRatios, []float64{0.4, 0.7, 0.3}) ||
					req.WindowManager.Shortcuts["desktop.switch.next"] != "Meta+ArrowRight" {
					t.Fatalf("req.WindowManager = %#v, want complete window-manager config", req.WindowManager)
				}
			},
		},
		{
			name: "observability",
			path: "/api/settings/observability",
			body: contract.UpdateSettingsObservabilityRequest{
				Config: contract.SettingsObservabilityConfigPayload{
					Enabled:        true,
					RetentionDays:  7,
					MaxGlobalBytes: 4096,
					Transcripts: contract.SettingsObservabilityTranscriptPayload{
						Enabled:            true,
						SegmentBytes:       2048,
						MaxBytesPerSession: 8192,
					},
				},
			},
			assert: func(t *testing.T, req settingspkg.SectionUpdateRequest) {
				t.Helper()
				if req.Observability == nil || req.Observability.RetentionDays != 7 {
					t.Fatalf("req.Observability = %#v, want populated observability config", req.Observability)
				}
			},
		},
		{
			name: "hooks extensions",
			path: "/api/settings/hooks-extensions",
			body: contract.UpdateSettingsHooksExtensionsRequest{
				Config: contract.SettingsExtensionsConfigPayload{
					Marketplace: contract.SettingsExtensionMarketplacePayload{
						Registry:        "github",
						BaseURL:         "https://extensions.example",
						AllowUnverified: true,
					},
					Resources: contract.SettingsExtensionResourcesPayload{
						AllowedKinds: []string{"tool"},
						MaxScope:     resources.ResourceScopeKindWorkspace,
						SnapshotRateLimit: contract.SettingsExtensionRateLimitPayload{
							Requests: 10,
							Window:   "1m",
							Queue:    2,
						},
						OperatorWriteRateLimit: contract.SettingsExtensionRateLimitPayload{
							Requests: 20,
							Window:   "1m",
							Queue:    4,
						},
					},
				},
			},
			assert: func(t *testing.T, req settingspkg.SectionUpdateRequest) {
				t.Helper()
				if req.HooksExtensions == nil ||
					req.HooksExtensions.Resources.MaxScope != resources.ResourceScopeKindWorkspace ||
					!req.HooksExtensions.Marketplace.AllowUnverified {
					t.Fatalf("req.HooksExtensions = %#v, want populated extensions config", req.HooksExtensions)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service := &stubSettingsService{
				UpdateSectionFn: func(_ context.Context, req settingspkg.SectionUpdateRequest) (settingspkg.MutationResult, error) {
					return settingspkg.MutationResult{
						Section:  req.Section,
						Scope:    req.Scope,
						Behavior: settingspkg.MutationBehaviorAppliedNow,
						Applied:  true,
					}, nil
				},
			}
			fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)

			resp := performRequest(t, fixture.Engine, http.MethodPatch, tc.path, mustJSON(t, tc.body))
			if got, want := resp.Code, http.StatusOK; got != want {
				t.Fatalf("status = %d, want %d; body=%s", got, want, resp.Body.String())
			}
			tc.assert(t, service.LastUpdateSectionRequest)
		})
	}
}

func validSettingsWindowManagerConfigPayload() contract.SettingsWindowManagerConfigPayload {
	return contract.SettingsWindowManagerConfigPayload{
		NewWindowPolicy:     contract.SettingsWindowNewPolicyBesideFocus,
		SmallViewportPolicy: contract.SettingsWindowSmallViewportPolicyReject,
		FocusPolicy:         contract.SettingsWindowFocusPolicyDirectional,
		FocusWrap:           true,
		FocusFollowsPointer: true,
		RaiseOnFocus:        false,
		DragAwayPolicy:      contract.SettingsWindowDragAwayPolicyGroup,
		GroupMoveModifier:   contract.SettingsWindowDragModifierControl,
		SwapModifier:        contract.SettingsWindowDragModifierMeta,
		HistoryLimit:        77,
		DesktopTransition:   contract.SettingsWindowDesktopTransitionCrossfade,
		Gaps: contract.SettingsWindowManagerGapsPayload{
			Inner:  12,
			Top:    18,
			Right:  14,
			Bottom: 16,
			Left:   20,
		},
		Snap: contract.SettingsWindowManagerSnapPayload{
			EdgeBand:     40,
			CornerReach:  180,
			ExitSlack:    20,
			RepeatRatios: []float64{0.4, 0.7, 0.3},
		},
		Bindings: contract.SettingsWindowManagerBindingPayload{
			TopCenter:    contract.SettingsWindowBindingActionNone,
			BottomCenter: contract.SettingsWindowBindingActionZoom,
		},
		Shortcuts: map[string]string{
			"desktop.switch.next": "Meta+ArrowRight",
			"window.focus.left":   "Alt+ArrowLeft",
		},
	}
}

func validSettingsRolesConfigPayload() contract.SettingsRolesConfigPayload {
	role := func(model string) contract.SettingsRoleConfigPayload {
		return contract.SettingsRoleConfigPayload{
			Enabled:         true,
			Provider:        "anthropic",
			Model:           model,
			ReasoningEffort: "high",
			FallbackChain: []contract.SettingsRoleFallbackPayload{{
				Provider: "openai", Model: "gpt-5-mini", ReasoningEffort: "medium",
			}},
		}
	}
	return contract.SettingsRolesConfigPayload{
		Coordinator: contract.SettingsCoordinatorRoleConfigPayload{
			SettingsRoleConfigPayload:     role("claude-sonnet"),
			TTL:                           "2h",
			MaxChildren:                   5,
			MaxActiveSessionsPerWorkspace: 3,
		},
		Dream:             role("claude-opus"),
		CheckpointSummary: role("claude-haiku"),
		MemoryExtractor:   role("claude-haiku"),
		AutoTitle:         role("claude-haiku"),
		MemoryController: contract.SettingsMemoryControllerRoleConfigPayload{
			Enabled:         true,
			Provider:        "anthropic",
			Model:           "claude-haiku",
			ReasoningEffort: "low",
			Timeout:         "250ms",
			TopK:            5,
			PromptVersion:   "v1",
			MaxTokensOut:    256,
			FallbackChain: []contract.SettingsRoleFallbackPayload{{
				Provider: "openai", Model: "gpt-5-mini", ReasoningEffort: "medium",
			}},
		},
	}
}

func validSettingsMemoryConfigPayload() contract.SettingsMemoryConfigPayload {
	return contract.SettingsMemoryConfigPayload{
		Enabled:   true,
		GlobalDir: "/tmp/agh-memory",
		Controller: contract.SettingsMemoryControllerPayload{
			Mode:            "hybrid",
			MaxLatency:      "300ms",
			DefaultOpOnFail: "noop",
			Policy: contract.SettingsMemoryControllerPolicyPayload{
				MaxContentChars: 4096,
				MaxWritesPerMin: 60,
				AllowOrigins: []string{
					"cli",
					"http",
					"uds",
					"tool",
					"extractor",
					"dreaming",
					"file",
					"provider",
				},
			},
		},
		Recall: contract.SettingsMemoryRecallPayload{
			TopK:          5,
			RawCandidates: 50,
			Fusion:        "weighted",
			Weights: contract.SettingsMemoryRecallWeightsPayload{
				BM25Unicode:  0.55,
				BM25Trigram:  0.20,
				Recency:      0.15,
				RecallSignal: 0.10,
			},
			Freshness: contract.SettingsMemoryRecallFreshnessPayload{
				BannerAfterDays: 1,
			},
			Signals: contract.SettingsMemoryRecallSignalsPayload{
				QueueCapacity:  256,
				WorkerRetryMax: 3,
				MetricsEnabled: true,
			},
		},
		Decisions: contract.SettingsMemoryDecisionsPayload{
			PruneAfterAppliedDays: 90,
			KeepAuditSummary:      true,
			MaxPostContentBytes:   65536,
		},
		Extractor: contract.SettingsMemoryExtractorPayload{
			Mode:             "post_message",
			ThrottleTurns:    1,
			Deadline:         "60s",
			SandboxInboxOnly: true,
			InboxPath:        "/tmp/agh-memory/_inbox",
			DLQPath:          "/tmp/agh-memory/_system/extractor/failures",
			Queue: contract.SettingsMemoryExtractorQueuePayload{
				Capacity:    1,
				CoalesceMax: 16,
			},
		},
		Dream: contract.SettingsMemoryDreamPayload{
			MinHours:      24,
			MinSessions:   3,
			Debounce:      "10m",
			PromptVersion: "v1",
			CheckInterval: "30m",
			Gates: contract.SettingsMemoryDreamGatesPayload{
				MinUnpromoted:  5,
				MinRecallCount: 2,
				MinScore:       0.75,
			},
			Scoring: contract.SettingsMemoryDreamScoringPayload{
				RecencyHalfLifeDays: 14,
				Weights: contract.SettingsMemoryDreamScoringWeightsPayload{
					Frequency: 0.30,
					Relevance: 0.35,
					Recency:   0.20,
					Freshness: 0.15,
				},
			},
		},
		Session: contract.SettingsMemorySessionPayload{
			LedgerFormat:     "jsonl",
			LedgerRoot:       "/tmp/agh-sessions",
			EventsPurgeGrace: "24h",
			ColdArchiveDays:  30,
			MaxArchiveBytes:  10737418240,
			UnboundPartition: "_unbound",
		},
		Daily: contract.SettingsMemoryDailyPayload{
			MaxBytes:        1048576,
			MaxLines:        5000,
			RotateFormat:    "{date}.{seq}.md",
			DreamingWindow:  7,
			ColdArchiveDays: 30,
			MaxArchiveBytes: 1073741824,
			SweepHour:       3,
			ArchivePath:     "_system/archive",
		},
		File: contract.SettingsMemoryFilePayload{
			MaxLines: 200,
			MaxBytes: 25600,
		},
		Provider: contract.SettingsMemoryProviderPayload{
			Timeout:          "2s",
			FailureThreshold: 5,
			Cooldown:         "30s",
		},
		Workspace: contract.SettingsMemoryWorkspacePayload{
			TOMLPath:   "<workspace>/.agh/workspace.toml",
			AutoCreate: true,
		},
	}
}

func TestSettingsCollectionHandlersDelegateValidPayloads(t *testing.T) {
	t.Parallel()

	readOnly := true
	inputRate := 1.0
	outputRate := 2.0
	cacheReadRate := 0.5
	cacheWriteRate := 3.0
	reasoningRate := 4.0
	tests := []struct {
		name           string
		method         string
		path           string
		body           any
		assert         func(t *testing.T, service *stubSettingsService)
		assertResponse func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:   "list providers",
			method: http.MethodGet,
			path:   "/api/settings/providers",
			assert: func(t *testing.T, service *stubSettingsService) {
				t.Helper()
				if service.LastListCollectionRequest.Collection != settingspkg.CollectionProviders {
					t.Fatalf("Collection = %q, want providers", service.LastListCollectionRequest.Collection)
				}
			},
			assertResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				t.Helper()
				var payload contract.SettingsProvidersResponse
				testutil.DecodeJSONResponse(t, resp, &payload)
				if len(payload.Providers) != 1 || payload.Providers[0].Name != "openai" {
					t.Fatalf("providers payload = %#v, want openai provider", payload)
				}
				model := payload.Providers[0].Settings.Models.Curated[0]
				if model.CostInputPerMillion == nil || *model.CostInputPerMillion != inputRate ||
					model.CostOutputPerMillion == nil || *model.CostOutputPerMillion != outputRate ||
					model.CostCacheReadPerMillion == nil || *model.CostCacheReadPerMillion != cacheReadRate ||
					model.CostCacheWritePerMillion == nil || *model.CostCacheWritePerMillion != cacheWriteRate ||
					model.CostReasoningPerMillion == nil || *model.CostReasoningPerMillion != reasoningRate {
					t.Fatalf("providers payload five-rate pricing = %#v", model)
				}
			},
		},
		{
			name:   "list mcp servers",
			method: http.MethodGet,
			path:   "/api/settings/mcp-servers?scope=workspace&workspace_id=ws-1",
			assert: func(t *testing.T, service *stubSettingsService) {
				t.Helper()
				if service.LastListCollectionRequest.Collection != settingspkg.CollectionMCPServers ||
					service.LastListCollectionRequest.Scope != settingspkg.ScopeWorkspace ||
					service.LastListCollectionRequest.WorkspaceID != "ws-1" {
					t.Fatalf("LastListCollectionRequest = %#v", service.LastListCollectionRequest)
				}
			},
			assertResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				t.Helper()
				var payload contract.SettingsMCPServersResponse
				testutil.DecodeJSONResponse(t, resp, &payload)
				if len(payload.MCPServers) != 1 || payload.MCPServers[0].Name != "memory" {
					t.Fatalf("mcp servers payload = %#v, want memory server", payload)
				}
			},
		},
		{
			name:   "get sandbox",
			method: http.MethodGet,
			path:   "/api/settings/sandboxes/local",
			assert: func(t *testing.T, service *stubSettingsService) {
				t.Helper()
				if service.LastListCollectionRequest.Collection != settingspkg.CollectionSandboxes {
					t.Fatalf("Collection = %q, want sandboxes", service.LastListCollectionRequest.Collection)
				}
			},
			assertResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				t.Helper()
				var payload contract.SettingsSandboxResponse
				testutil.DecodeJSONResponse(t, resp, &payload)
				if payload.Sandbox.Name != "local" || payload.Sandbox.Profile.Backend != "local" {
					t.Fatalf("sandbox payload = %#v, want local sandbox profile", payload)
				}
			},
		},
		{
			name:   "list hooks",
			method: http.MethodGet,
			path:   "/api/settings/hooks",
			assert: func(t *testing.T, service *stubSettingsService) {
				t.Helper()
				if service.LastListCollectionRequest.Collection != settingspkg.CollectionHooks {
					t.Fatalf("Collection = %q, want hooks", service.LastListCollectionRequest.Collection)
				}
			},
			assertResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				t.Helper()
				var payload contract.SettingsHooksResponse
				testutil.DecodeJSONResponse(t, resp, &payload)
				if len(payload.Hooks) != 1 || payload.Hooks[0].Name != "capture" {
					t.Fatalf("hooks payload = %#v, want capture hook", payload)
				}
			},
		},
		{
			name:   "put provider",
			method: http.MethodPut,
			path:   "/api/settings/providers/openai",
			body: contract.PutSettingsProviderRequest{
				Settings: contract.SettingsProviderSettingsPayload{
					Command: "codex",
					Models: &contract.SettingsProviderModelsPayload{
						Default: "gpt-5.4",
						Curated: []contract.SettingsProviderModelPayload{
							{
								ID:                       "gpt-5.4",
								DisplayName:              "GPT-5.4",
								SupportsReasoning:        new(true),
								ReasoningEfforts:         []contract.ReasoningEffort{"low", "high"},
								DefaultReasoningEffort:   "high",
								CostInputPerMillion:      new(1.0),
								CostOutputPerMillion:     new(2.0),
								CostCacheReadPerMillion:  new(0.5),
								CostCacheWritePerMillion: new(3.0),
								CostReasoningPerMillion:  new(4.0),
							},
							{ID: "gpt-5.4-mini", DisplayName: "GPT-5.4 Mini"},
						},
					},
					CredentialSlots: []contract.SettingsProviderCredentialSlotPayload{
						{
							Name:      "api_key",
							TargetEnv: "OPENAI_API_KEY",
							SecretRef: "env:OPENAI_API_KEY",
							Kind:      "api_key",
							Required:  true,
						},
					},
				},
				ModelCuration: &contract.ProviderModelCurationRequest{
					ModelID:                "gpt-5.4",
					DefaultReasoningEffort: new(modelcatalog.ReasoningEffortMax),
				},
			},
			assert: func(t *testing.T, service *stubSettingsService) {
				t.Helper()
				if service.LastPutCollectionRequest.Provider == nil ||
					service.LastPutCollectionRequest.Provider.Models.Default != "gpt-5.4" {
					t.Fatalf("LastPutCollectionRequest.Provider = %#v", service.LastPutCollectionRequest.Provider)
				}
				if got := service.LastPutCollectionRequest.Provider.Models.Curated; len(got) != 2 ||
					got[0].ID != "gpt-5.4" ||
					got[1].ID != "gpt-5.4-mini" {
					t.Fatalf("Provider.Models.Curated = %#v", got)
				}
				model := service.LastPutCollectionRequest.Provider.Models.Curated[0]
				if model.SupportsReasoning == nil || !*model.SupportsReasoning {
					t.Fatalf(
						"Provider.Models.Curated[0].SupportsReasoning = %#v, want true",
						model.SupportsReasoning,
					)
				}
				if got, want := model.DefaultReasoningEffort, "high"; got != want {
					t.Fatalf("Provider.Models.Curated[0].DefaultReasoningEffort = %q, want %q", got, want)
				}
				if model.CostInputPerMillion == nil || *model.CostInputPerMillion != 1 ||
					model.CostOutputPerMillion == nil || *model.CostOutputPerMillion != 2 ||
					model.CostCacheReadPerMillion == nil || *model.CostCacheReadPerMillion != 0.5 ||
					model.CostCacheWritePerMillion == nil || *model.CostCacheWritePerMillion != 3 ||
					model.CostReasoningPerMillion == nil || *model.CostReasoningPerMillion != 4 {
					t.Fatalf("Provider.Models.Curated[0] five-rate pricing = %#v", model)
				}
				curation := service.LastPutCollectionRequest.ProviderModelCuration
				if curation == nil || curation.ProviderID != "openai" || curation.ModelID != "gpt-5.4" {
					t.Fatalf("ProviderModelCuration = %#v", curation)
				}
				if curation.DefaultReasoningEffort == nil ||
					*curation.DefaultReasoningEffort != modelcatalog.ReasoningEffortMax {
					t.Fatalf(
						"ProviderModelCuration.DefaultReasoningEffort = %v, want max",
						curation.DefaultReasoningEffort,
					)
				}
			},
			assertResponse: assertAppliedSettingsMutation,
		},
		{
			name:   "put sandbox",
			method: http.MethodPut,
			path:   "/api/settings/sandboxes/local",
			body: contract.PutSettingsSandboxRequest{
				Profile: contract.SettingsSandboxProfilePayload{
					Backend:     "local",
					SyncMode:    "session-bidirectional",
					Persistence: "reuse",
					RuntimeRoot: "/workspace",
				},
			},
			assert: func(t *testing.T, service *stubSettingsService) {
				t.Helper()
				if service.LastPutCollectionRequest.Sandbox == nil ||
					service.LastPutCollectionRequest.Sandbox.Backend != "local" {
					t.Fatalf("LastPutCollectionRequest.Sandbox = %#v", service.LastPutCollectionRequest.Sandbox)
				}
			},
			assertResponse: assertAppliedSettingsMutation,
		},
		{
			name:   "put hook",
			method: http.MethodPut,
			path:   "/api/settings/hooks/capture",
			body: contract.PutSettingsHookRequest{
				Declaration: contract.SettingsHookDeclarationPayload{
					Name:         "capture",
					Event:        hookspkg.HookToolPreCall,
					Mode:         hookspkg.HookModeAsync,
					ExecutorKind: hookspkg.HookExecutorSubprocess,
					Command:      "/bin/capture",
					Matcher: hookspkg.HookMatcher{
						ToolID:       "agh__read",
						ToolReadOnly: &readOnly,
					},
				},
			},
			assert: func(t *testing.T, service *stubSettingsService) {
				t.Helper()
				if service.LastPutCollectionRequest.Hook == nil ||
					service.LastPutCollectionRequest.Hook.Name != "capture" {
					t.Fatalf("LastPutCollectionRequest.Hook = %#v", service.LastPutCollectionRequest.Hook)
				}
			},
			assertResponse: assertAppliedSettingsMutation,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service := &stubSettingsService{
				ListCollectionFn: func(_ context.Context, req settingspkg.CollectionRequest) (settingspkg.CollectionEnvelope, error) {
					envelope := settingspkg.CollectionEnvelope{
						Collection:      req.Collection,
						Scope:           req.Scope,
						WorkspaceID:     req.WorkspaceID,
						AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal, settingspkg.ScopeWorkspace},
					}
					switch req.Collection {
					case settingspkg.CollectionProviders:
						envelope.Providers = []settingspkg.ProviderItem{
							{
								Name: "openai",
								Settings: settingspkg.ProviderSettings{
									Command: "codex",
									Models: aghconfig.ProviderModelsConfig{
										Default: "gpt-5.4",
										Curated: []aghconfig.ProviderModelConfig{{
											ID:                       "gpt-5.4",
											CostInputPerMillion:      &inputRate,
											CostOutputPerMillion:     &outputRate,
											CostCacheReadPerMillion:  &cacheReadRate,
											CostCacheWritePerMillion: &cacheWriteRate,
											CostReasoningPerMillion:  &reasoningRate,
										}},
									},
									CredentialSlots: []aghconfig.ProviderCredentialSlot{
										{
											Name:      "api_key",
											TargetEnv: "OPENAI_API_KEY",
											SecretRef: "env:OPENAI_API_KEY",
											Kind:      "api_key",
											Required:  true,
										},
									},
								},
								CommandAvailable: true,
								SourceMetadata: settingspkg.SourceMetadata{
									EffectiveSource: settingspkg.SourceRef{
										Kind:  settingspkg.SourceKindGlobalConfig,
										Scope: settingspkg.ScopeGlobal,
									},
									AvailableTargets: []settingspkg.WriteTargetKind{
										settingspkg.WriteTargetGlobalConfig,
									},
								},
							},
						}
					case settingspkg.CollectionMCPServers:
						envelope.MCPServers = []settingspkg.MCPServerItem{{
							Name:        "memory",
							Command:     "memoryd",
							Scope:       req.Scope,
							WorkspaceID: req.WorkspaceID,
							SourceMetadata: settingspkg.SourceMetadata{
								EffectiveSource: settingspkg.SourceRef{
									Kind:        settingspkg.SourceKindWorkspaceMCPSidecar,
									Scope:       req.Scope,
									WorkspaceID: req.WorkspaceID,
								},
								AvailableTargets: []settingspkg.WriteTargetKind{
									settingspkg.WriteTargetWorkspaceMCPSidecar,
								},
							},
						}}
					case settingspkg.CollectionSandboxes:
						envelope.Sandboxes = []settingspkg.SandboxItem{{
							Name: "local",
							Profile: aghconfig.SandboxProfile{
								Backend: "local",
							},
							SourceMetadata: settingspkg.SourceMetadata{
								EffectiveSource: settingspkg.SourceRef{
									Kind:  settingspkg.SourceKindGlobalConfig,
									Scope: settingspkg.ScopeGlobal,
								},
								AvailableTargets: []settingspkg.WriteTargetKind{
									settingspkg.WriteTargetGlobalConfig,
								},
							},
						}}
					case settingspkg.CollectionHooks:
						envelope.Hooks = []settingspkg.HookItem{{
							Name: "capture",
							Declaration: hookspkg.HookDecl{
								Name:         "capture",
								Event:        hookspkg.HookToolPreCall,
								Mode:         hookspkg.HookModeAsync,
								ExecutorKind: hookspkg.HookExecutorSubprocess,
								Command:      "/bin/capture",
								Matcher:      hookspkg.HookMatcher{ToolID: "agh__read"},
							},
							SourceMetadata: settingspkg.SourceMetadata{
								EffectiveSource: settingspkg.SourceRef{
									Kind:  settingspkg.SourceKindGlobalConfig,
									Scope: settingspkg.ScopeGlobal,
								},
								AvailableTargets: []settingspkg.WriteTargetKind{
									settingspkg.WriteTargetGlobalConfig,
								},
							},
						}}
					}
					return envelope, nil
				},
				PutCollectionItemFn: func(_ context.Context, req settingspkg.CollectionItemPutRequest) (settingspkg.MutationResult, error) {
					return settingspkg.MutationResult{
						Section:  settingspkg.SectionName(req.Collection),
						Scope:    req.Scope,
						Behavior: settingspkg.MutationBehaviorAppliedNow,
						Applied:  true,
					}, nil
				},
			}
			fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)

			var body []byte
			if tc.body != nil {
				body = mustJSON(t, tc.body)
			}
			resp := performRequest(t, fixture.Engine, tc.method, tc.path, body)
			if got, want := resp.Code, http.StatusOK; got != want {
				t.Fatalf("status = %d, want %d; body=%s", got, want, resp.Body.String())
			}
			tc.assertResponse(t, resp)
			tc.assert(t, service)
		})
	}
}

func assertAppliedSettingsMutation(t *testing.T, resp *httptest.ResponseRecorder) {
	t.Helper()

	var payload contract.SettingsApplyResponse
	testutil.DecodeJSONResponse(t, resp, &payload)
	if !payload.Applied || payload.Lifecycle != contract.SettingsApplyLifecycleLive {
		t.Fatalf("settings mutation payload = %#v, want live", payload)
	}
}

func TestPutSettingsProviderPreservesModelCurationDiagnostics(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		code string
	}{
		{name: "Should preserve model_not_found", code: diagnosticcontract.CodeModelNotFound},
		{
			name: "Should preserve reasoning_effort_unsupported",
			code: diagnosticcontract.CodeReasoningEffortUnsupported,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cause := errors.New("provider model curation rejected")
			item := diagnostics.NewItem(
				"provider.models."+tc.code,
				tc.code,
				diagnosticcontract.CategoryProvider,
				"Provider model curation rejected",
				cause.Error(),
				diagnosticcontract.SeverityError,
				diagnosticcontract.FreshnessLive,
			)
			service := &stubSettingsService{
				ApplyCollectionFn: func(
					context.Context,
					settingspkg.CollectionItemPutRequest,
				) (settingspkg.ApplyResult, error) {
					return settingspkg.ApplyResult{}, diagnostics.NewStructuredError(item, cause)
				},
			}
			fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)
			body := mustJSON(t, contract.PutSettingsProviderRequest{
				Settings: contract.SettingsProviderSettingsPayload{Command: "claude-acp"},
				ModelCuration: &contract.ProviderModelCurationRequest{
					ModelID: "claude-sonnet-5",
				},
			})
			resp := performRequest(
				t,
				fixture.Engine,
				http.MethodPut,
				"/api/settings/providers/claude",
				body,
			)
			if got, want := resp.Code, http.StatusUnprocessableEntity; got != want {
				t.Fatalf("status = %d, want %d; body=%s", got, want, resp.Body.String())
			}
			var payload contract.ErrorPayload
			testutil.DecodeJSONResponse(t, resp, &payload)
			if payload.Diagnostic == nil || payload.Diagnostic.Code != tc.code {
				t.Fatalf("diagnostic = %#v, want code %q", payload.Diagnostic, tc.code)
			}
		})
	}
}

func TestSettingsCollectionMutationHandlersRejectInvalidPayloads(t *testing.T) {
	t.Parallel()

	readOnly := true
	tests := []struct {
		name              string
		method            string
		path              string
		body              []byte
		want              string
		expectPutCalls    int
		expectDeleteCalls int
	}{
		{
			name:   "provider missing settings",
			method: http.MethodPut,
			path:   "/api/settings/providers/openai",
			body:   []byte(`{}`),
			want:   "provider.settings is required",
		},
		{
			name:   "mcp server missing body",
			method: http.MethodPut,
			path:   "/api/settings/mcp-servers/server-a",
			body:   []byte(`{}`),
			want:   "mcp-servers.server is required",
		},
		{
			name:   "sandbox missing profile",
			method: http.MethodPut,
			path:   "/api/settings/sandboxes/local",
			body:   []byte(`{}`),
			want:   "sandboxes.profile is required",
		},
		{
			name:   "hook missing declaration",
			method: http.MethodPut,
			path:   "/api/settings/hooks/capture",
			body:   []byte(`{}`),
			want:   "hooks.declaration is required",
		},
		{
			name:   "delete invalid target",
			method: http.MethodDelete,
			path:   "/api/settings/mcp-servers/server-a?target=invalid",
			want:   "settings.target must be one of",
		},
		{
			name:   "hook invalid timeout",
			method: http.MethodPut,
			path:   "/api/settings/hooks/capture",
			body: mustJSON(t, contract.PutSettingsHookRequest{
				Declaration: contract.SettingsHookDeclarationPayload{
					Name:         "capture",
					Event:        hookspkg.HookToolPreCall,
					Mode:         hookspkg.HookModeAsync,
					ExecutorKind: hookspkg.HookExecutorSubprocess,
					Command:      "/bin/capture",
					Timeout:      "bad",
					Matcher: hookspkg.HookMatcher{
						ToolID:       "agh__read",
						ToolReadOnly: &readOnly,
					},
				},
			}),
			want: "hooks.declaration.timeout",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service := &stubSettingsService{}
			fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)

			resp := performRequest(t, fixture.Engine, tc.method, tc.path, tc.body)
			if got, want := resp.Code, http.StatusBadRequest; got != want {
				t.Fatalf("status = %d, want %d; body=%s", got, want, resp.Body.String())
			}
			if service.PutCollectionItemCalls != 0 {
				t.Fatalf("PutCollectionItemCalls = %d, want 0", service.PutCollectionItemCalls)
			}
			if service.DeleteItemCalls != 0 {
				t.Fatalf("DeleteItemCalls = %d, want 0", service.DeleteItemCalls)
			}

			var payload contract.ErrorPayload
			decodeJSON(t, resp.Body.Bytes(), &payload)
			if !strings.Contains(payload.Error, tc.want) {
				t.Fatalf("payload.Error = %q, want substring %q", payload.Error, tc.want)
			}
		})
	}
}

func TestSettingsRemainingReadAndDeleteHandlers(t *testing.T) {
	t.Parallel()

	t.Run("Should serialize empty window-manager shortcuts as an object", func(t *testing.T) {
		t.Parallel()

		config := aghconfig.DefaultWindowManagerConfig()
		config.Shortcuts = nil
		service := &stubSettingsService{
			GetSectionFn: func(
				_ context.Context,
				req settingspkg.SectionRequest,
			) (settingspkg.SectionEnvelope, error) {
				if req.Section != settingspkg.SectionWindowManager {
					return settingspkg.SectionEnvelope{}, errors.New("unexpected section")
				}
				return settingspkg.SectionEnvelope{
					Section:         settingspkg.SectionWindowManager,
					Scope:           settingspkg.ScopeGlobal,
					AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal},
					WindowManager: &settingspkg.WindowManagerSection{
						Config: config,
					},
				}, nil
			},
		}
		fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/api/settings/window-manager",
			nil,
		)
		if got, want := resp.Code, http.StatusOK; got != want {
			t.Fatalf("status = %d, want %d; body=%s", got, want, resp.Body.String())
		}

		var payload contract.SettingsWindowManagerResponse
		decodeJSON(t, resp.Body.Bytes(), &payload)
		if payload.Config.Shortcuts == nil {
			t.Fatalf("config.shortcuts = nil, want empty object; body=%s", resp.Body.String())
		}
		if got := len(payload.Config.Shortcuts); got != 0 {
			t.Fatalf("len(config.shortcuts) = %d, want 0", got)
		}
	})

	service := &stubSettingsService{
		GetSectionFn: func(_ context.Context, req settingspkg.SectionRequest) (settingspkg.SectionEnvelope, error) {
			envelope := settingspkg.SectionEnvelope{
				Section:         req.Section,
				Scope:           req.Scope,
				WorkspaceID:     req.WorkspaceID,
				AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal},
			}
			switch req.Section {
			case settingspkg.SectionMemory:
				envelope.Memory = &settingspkg.MemorySection{
					Config: aghconfig.MemoryConfig{
						Enabled: true,
						Dream: aghconfig.DreamConfig{
							CheckInterval: time.Hour,
						},
					},
				}
			case settingspkg.SectionRoles:
				roles := aghconfig.DefaultRolesConfig()
				roles.Dream.Model = "claude-opus"
				roles.AutoTitle.FallbackChain = []aghconfig.RoleFallback{{
					Provider: "openai", Model: "gpt-5-mini", ReasoningEffort: "medium",
				}}
				envelope.Roles = &settingspkg.RolesSection{Config: roles}
			case settingspkg.SectionSkills:
				envelope.Skills = &settingspkg.SkillsSection{
					Config: aghconfig.SkillsConfig{
						Enabled:      true,
						PollInterval: time.Minute,
						Marketplace:  aghconfig.MarketplaceConfig{Registry: "clawhub"},
					},
				}
			case settingspkg.SectionAutomation:
				envelope.Automation = &settingspkg.AutomationSection{
					Config: settingspkg.AutomationSettings{
						Enabled:           true,
						Timezone:          "UTC",
						MaxConcurrentJobs: 1,
						DefaultFireLimit:  automationmodel.FireLimitConfig{Max: 1, Window: "1m"},
					},
				}
			case settingspkg.SectionNetwork:
				envelope.Network = &settingspkg.NetworkSection{
					Config: aghconfig.NetworkConfig{
						Enabled: true,
					},
				}
			case settingspkg.SectionHooksExtensions:
				envelope.HooksExtensions = &settingspkg.HooksExtensionsSection{
					Extensions: aghconfig.ExtensionsConfig{
						Marketplace: aghconfig.ExtensionsMarketplaceConfig{Registry: "github"},
					},
				}
			default:
				return settingspkg.SectionEnvelope{}, errors.New("unexpected section")
			}
			return envelope, nil
		},
		ListCollectionFn: func(_ context.Context, req settingspkg.CollectionRequest) (settingspkg.CollectionEnvelope, error) {
			envelope := settingspkg.CollectionEnvelope{
				Collection:      req.Collection,
				Scope:           req.Scope,
				WorkspaceID:     req.WorkspaceID,
				AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal},
			}
			switch req.Collection {
			case settingspkg.CollectionSandboxes:
				envelope.Sandboxes = []settingspkg.SandboxItem{{
					Name: "local",
					Profile: aghconfig.SandboxProfile{
						Backend: "local",
					},
					SourceMetadata: settingspkg.SourceMetadata{
						EffectiveSource: settingspkg.SourceRef{
							Kind:  settingspkg.SourceKindGlobalConfig,
							Scope: settingspkg.ScopeGlobal,
						},
						AvailableTargets: []settingspkg.WriteTargetKind{
							settingspkg.WriteTargetGlobalConfig,
						},
					},
				}}
			default:
				return settingspkg.CollectionEnvelope{}, errors.New("unexpected collection")
			}
			return envelope, nil
		},
		DeleteItemFn: func(_ context.Context, req settingspkg.CollectionItemDeleteRequest) (settingspkg.MutationResult, error) {
			return settingspkg.MutationResult{
				Section:  settingspkg.SectionName(req.Collection),
				Scope:    req.Scope,
				Behavior: settingspkg.MutationBehaviorAppliedNow,
				Applied:  true,
			}, nil
		},
	}
	fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)

	for _, path := range []string{
		"/api/settings/memory",
		"/api/settings/roles",
		"/api/settings/skills",
		"/api/settings/automation",
		"/api/settings/network",
		"/api/settings/hooks-extensions",
		"/api/settings/sandboxes",
	} {
		resp := performRequest(t, fixture.Engine, http.MethodGet, path, nil)
		if got, want := resp.Code, http.StatusOK; got != want {
			t.Fatalf("%s status = %d, want %d; body=%s", path, got, want, resp.Body.String())
		}
	}

	rolesResp := performRequest(t, fixture.Engine, http.MethodGet, "/api/settings/roles", nil)
	var roles contract.SettingsRolesResponse
	decodeJSON(t, rolesResp.Body.Bytes(), &roles)
	if rolesResp.Code != http.StatusOK || roles.Section != contract.SettingsSectionRoles ||
		roles.Config.Dream.Model != "claude-opus" ||
		len(roles.Config.AutoTitle.FallbackChain) != 1 ||
		roles.Config.AutoTitle.FallbackChain[0].Model != "gpt-5-mini" {
		t.Fatalf("GET settings roles = status %d payload %#v", rolesResp.Code, roles)
	}

	for _, path := range []string{
		"/api/settings/providers/openai",
		"/api/settings/sandboxes/local",
		"/api/settings/hooks/capture",
	} {
		resp := performRequest(t, fixture.Engine, http.MethodDelete, path, nil)
		if got, want := resp.Code, http.StatusOK; got != want {
			t.Fatalf("%s status = %d, want %d; body=%s", path, got, want, resp.Body.String())
		}
	}
}

func TestSettingsRejectsAgentNameOutsideSkills(t *testing.T) {
	t.Parallel()

	service := &stubSettingsService{}
	fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)

	resp := performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/api/settings/general?agent_name=coder",
		nil,
	)
	if got, want := resp.Code, http.StatusBadRequest; got != want {
		t.Fatalf("status = %d, want %d; body=%s", got, want, resp.Body.String())
	}
	if service.GetSectionCalls != 0 {
		t.Fatalf("GetSectionCalls = %d, want 0", service.GetSectionCalls)
	}

	var payload contract.ErrorPayload
	decodeJSON(t, resp.Body.Bytes(), &payload)
	if !strings.Contains(payload.Error, "agent_name is only supported for skills") {
		t.Fatalf("payload.Error = %q, want agent_name validation", payload.Error)
	}
}

func TestSettingsHandlersReturnServiceUnavailableWithoutInjectedDependencies(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	homePaths := testutil.NewTestHomePaths(t)
	cfg := aghconfig.DefaultWithHome(homePaths)
	engine := gin.New()
	engine.Use(gin.Recovery())
	registerSettingsRoutes(engine, core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName: "api-core-http",
		HomePaths:     homePaths,
		Config:        cfg,
		Logger:        testutil.DiscardLogger(),
	}))

	tests := []struct {
		method string
		path   string
		body   []byte
	}{
		{method: http.MethodGet, path: "/api/settings/general"},
		{
			method: http.MethodPatch,
			path:   "/api/settings/general",
			body: mustJSON(t, contract.UpdateSettingsGeneralRequest{
				Config: contract.SettingsGeneralConfigPayload{
					Defaults: contract.SettingsDefaultsPayload{Agent: "coder"},
					Limits:   contract.SettingsLimitsPayload{MaxConcurrentAgents: 2},
					Permissions: contract.SettingsPermissionsPayload{
						Mode: contract.SettingsPermissionModeApproveReads,
					},
					SessionTimeout: "30m",
					HTTP:           contract.SettingsHTTPPayload{Host: "127.0.0.1", Port: 2123},
					Daemon:         contract.SettingsDaemonPayload{Socket: "/tmp/agh.sock"},
				},
			}),
		},
		{method: http.MethodGet, path: "/api/settings/providers"},
		{
			method: http.MethodPut,
			path:   "/api/settings/mcp-servers/server-a",
			body: mustJSON(t, contract.PutSettingsMCPServerRequest{
				Server: contract.SettingsMCPServerPayload{Name: "server-a", Command: "mcpd"},
			}),
		},
		{method: http.MethodDelete, path: "/api/settings/hooks/capture"},
		{method: http.MethodPost, path: "/api/settings/actions/restart", body: []byte(`{}`)},
		{method: http.MethodGet, path: "/api/settings/actions/restart/op-123"},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			t.Parallel()

			resp := performRequest(t, engine, tc.method, tc.path, tc.body)
			if got, want := resp.Code, http.StatusServiceUnavailable; got != want {
				t.Fatalf("status = %d, want %d; body=%s", got, want, resp.Body.String())
			}
		})
	}
}

func TestGetSettingsProviderMissingResourceReturnsNotFound(t *testing.T) {
	t.Parallel()

	service := &stubSettingsService{
		ListCollectionFn: func(context.Context, settingspkg.CollectionRequest) (settingspkg.CollectionEnvelope, error) {
			return settingspkg.CollectionEnvelope{
				Collection: settingspkg.CollectionProviders,
				Scope:      settingspkg.ScopeGlobal,
			}, nil
		},
	}
	fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)

	resp := performRequest(t, fixture.Engine, http.MethodGet, "/api/settings/providers/missing", nil)
	if got, want := resp.Code, http.StatusNotFound; got != want {
		t.Fatalf("status = %d, want %d; body=%s", got, want, resp.Body.String())
	}
}

func TestGetSettingsGeneralUnsupportedScopeReturnsConflict(t *testing.T) {
	t.Parallel()

	service := &stubSettingsService{
		GetSectionFn: func(context.Context, settingspkg.SectionRequest) (settingspkg.SectionEnvelope, error) {
			return settingspkg.SectionEnvelope{}, fmt.Errorf(
				"%w: %s",
				settingspkg.ErrConflict,
				`settings: section "general" does not support workspace scope`,
			)
		},
	}
	fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)

	resp := performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/api/settings/general?scope=workspace&workspace_id=ws-1",
		nil,
	)
	if got, want := resp.Code, http.StatusConflict; got != want {
		t.Fatalf("status = %d, want %d; body=%s", got, want, resp.Body.String())
	}
	if got := service.LastGetSectionRequest.Scope; got != settingspkg.ScopeWorkspace {
		t.Fatalf("LastGetSectionRequest.Scope = %q, want %q", got, settingspkg.ScopeWorkspace)
	}
	if got := service.LastGetSectionRequest.WorkspaceID; got != "ws-1" {
		t.Fatalf("LastGetSectionRequest.WorkspaceID = %q, want ws-1", got)
	}
}

func TestTriggerSettingsRestartUsesActionHandlerInsteadOfSettingsMutation(t *testing.T) {
	t.Parallel()

	service := &stubSettingsService{}
	restartController := &stubSettingsRestartController{
		RequestFn: func(context.Context) (core.SettingsRestartOperation, error) {
			return core.SettingsRestartOperation{
				OperationID:        "op-123",
				Status:             "stopping",
				ActiveSessionCount: 3,
			}, nil
		},
	}
	fixture := newSettingsHandlerFixture(t, "api-core-http", service, restartController)

	resp := performRequest(t, fixture.Engine, http.MethodPost, "/api/settings/actions/restart", []byte(`{}`))
	if got, want := resp.Code, http.StatusAccepted; got != want {
		t.Fatalf("status = %d, want %d; body=%s", got, want, resp.Body.String())
	}
	if restartController.RequestCalls != 1 {
		t.Fatalf("RequestCalls = %d, want 1", restartController.RequestCalls)
	}
	if service.UpdateSectionCalls != 0 {
		t.Fatalf("UpdateSectionCalls = %d, want 0", service.UpdateSectionCalls)
	}

	var payload contract.RestartActionResponse
	decodeJSON(t, resp.Body.Bytes(), &payload)
	if payload.OperationID != "op-123" {
		t.Fatalf("payload.OperationID = %q, want op-123", payload.OperationID)
	}
	if payload.StatusURL != "/api/settings/actions/restart/op-123" {
		t.Fatalf("payload.StatusURL = %q, want restart polling URL", payload.StatusURL)
	}
	if payload.ActiveSessionCount != 3 {
		t.Fatalf("payload.ActiveSessionCount = %d, want 3", payload.ActiveSessionCount)
	}
}

func TestGetSettingsRestartStatusReturnsPersistedOperationShape(t *testing.T) {
	t.Parallel()

	completedAt := time.Date(2026, 4, 17, 18, 50, 0, 0, time.UTC)
	restartController := &stubSettingsRestartController{
		StatusFn: func(_ context.Context, operationID string) (core.SettingsRestartOperation, error) {
			return core.SettingsRestartOperation{
				OperationID:        operationID,
				Status:             "ready",
				OldPID:             4100,
				OldStartedAt:       time.Date(2026, 4, 17, 18, 0, 0, 0, time.UTC),
				OldSocketPath:      "/tmp/agh.sock",
				NewPID:             4200,
				ActiveSessionCount: 5,
				StartedAt:          time.Date(2026, 4, 17, 18, 45, 0, 0, time.UTC),
				UpdatedAt:          completedAt,
				CompletedAt:        &completedAt,
			}, nil
		},
	}
	fixture := newSettingsHandlerFixture(t, "api-core-http", &stubSettingsService{}, restartController)

	resp := performRequest(t, fixture.Engine, http.MethodGet, "/api/settings/actions/restart/op-ready", nil)
	if got, want := resp.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d; body=%s", got, want, resp.Body.String())
	}
	if restartController.LastOperationID != "op-ready" {
		t.Fatalf("LastOperationID = %q, want op-ready", restartController.LastOperationID)
	}

	var payload contract.RestartActionStatus
	decodeJSON(t, resp.Body.Bytes(), &payload)
	if payload.OperationID != "op-ready" || payload.NewPID != 4200 || payload.ActiveSessionCount != 5 {
		t.Fatalf("payload = %#v", payload)
	}
	if payload.CompletedAt == nil || !payload.CompletedAt.Equal(completedAt) {
		t.Fatalf("payload.CompletedAt = %v, want %v", payload.CompletedAt, completedAt)
	}
}

func TestInstallSettingsMCPServerMapsStrictRequestAndRedactedResponse(t *testing.T) {
	newService := func() *stubSettingsService {
		return &stubSettingsService{
			InstallMCPCatalogFn: func(
				_ context.Context,
				req settingspkg.MCPCatalogInstallRequest,
			) (settingspkg.MCPCatalogInstallResult, error) {
				return settingspkg.MCPCatalogInstallResult{
					Item: settingspkg.MCPServerItem{
						Name:          req.Name,
						Transport:     aghconfig.MCPServerTransportStdio,
						Command:       "npx",
						SecretEnvKeys: []string{"GITHUB_TOKEN"},
						Auth: aghconfig.MCPAuthConfig{
							Type: aghconfig.MCPAuthTypeOAuth2PKCE,
						},
						ClientSecretConfigured: true,
						Scope:                  req.Scope,
						WorkspaceID:            req.WorkspaceID,
						CatalogEntry:           req.EntryID,
						CatalogVersion:         "1.2.3",
					},
					Apply: settingspkg.ApplyResult{
						Record: settingspkg.ApplyRecord{
							ID:         "cfgapp-mcp-install",
							Generation: 4,
							Lifecycle:  lifecycle.LiveAdd,
							Status:     lifecycle.StatusApplied,
							ActiveHash: "sha256:mcp-install",
						},
						Section:     settingspkg.SectionName(settingspkg.CollectionMCPServers),
						Scope:       req.Scope,
						WriteTarget: settingspkg.WriteTargetWorkspaceMCPSidecar,
						WorkspaceID: req.WorkspaceID,
						Applied:     true,
						NextAction:  lifecycle.NextActionNone,
					},
					NextStep: settingspkg.MCPCatalogInstallNextStepNone,
				}, nil
			},
		}
	}

	t.Run("Should map the strict request without returning secret bindings", func(t *testing.T) {
		t.Parallel()

		service := newService()
		fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)
		body := mustJSON(t, contract.InstallSettingsMCPServerRequest{
			EntryID:     "github",
			Name:        "github",
			Scope:       contract.SettingsWorkspaceScopeWorkspace,
			WorkspaceID: "ws-1",
			Values: &contract.SettingsMCPCatalogInstallValuesPayload{
				Env: map[string]contract.SettingsMCPSecretInputPayload{
					"GITHUB_TOKEN": {Value: "write-only-secret"},
				},
			},
		})
		resp := performRequest(t, fixture.Engine, http.MethodPost, "/api/settings/mcp-servers/install", body)
		if got, want := resp.Code, http.StatusOK; got != want {
			t.Fatalf("POST status = %d, want %d; body=%s", got, want, resp.Body.String())
		}
		if got, want := service.LastMCPCatalogInstall.EntryID, "github"; got != want {
			t.Fatalf("install EntryID = %q, want %q", got, want)
		}
		if got, want := service.LastMCPCatalogInstall.Scope, settingspkg.ScopeWorkspace; got != want {
			t.Fatalf("install Scope = %q, want %q", got, want)
		}
		if got, want := service.LastMCPCatalogInstall.WorkspaceID, "ws-1"; got != want {
			t.Fatalf("install WorkspaceID = %q, want %q", got, want)
		}
		if got, want := service.LastMCPCatalogInstall.Values.Env["GITHUB_TOKEN"].Value, "write-only-secret"; got != want {
			t.Fatalf("install secret input = %q, want mapped write-only value", got)
		}
		for _, forbidden := range []string{
			"write-only-secret",
			"vault:mcp/ws/ws-1/github/env/GITHUB_TOKEN",
			"vault:mcp/ws/ws-1/github/oauth/client-secret",
			"client_secret_ref",
		} {
			if strings.Contains(resp.Body.String(), forbidden) {
				t.Fatalf("response leaked secret material %q: %s", forbidden, resp.Body.String())
			}
		}
		var payload contract.InstallSettingsMCPServerResponse
		decodeJSON(t, resp.Body.Bytes(), &payload)
		if got := payload.MCPServer.SecretEnvKeys; len(got) != 1 || got[0] != "GITHUB_TOKEN" {
			t.Fatalf("response secret env keys = %#v, want [GITHUB_TOKEN]", got)
		}
		if payload.MCPServer.Auth == nil || !payload.MCPServer.Auth.ClientSecretConfigured {
			t.Fatalf("response auth = %#v, want configured client-secret presence", payload.MCPServer.Auth)
		}
		if got, want := payload.NextStep, contract.SettingsMCPInstallNextStepNone; got != want {
			t.Fatalf("response next_step = %q, want %q", got, want)
		}
		if !payload.Apply.Applied ||
			payload.Apply.Lifecycle != contract.SettingsApplyLifecycleLiveAdd ||
			payload.Apply.ApplyRecordID != "cfgapp-mcp-install" ||
			payload.Apply.ActiveGeneration != 4 {
			t.Fatalf("response apply = %#v, want applied live-add generation 4", payload.Apply)
		}
	})

	t.Run("Should reject a client override of a feed-locked field", func(t *testing.T) {
		t.Parallel()

		service := newService()
		fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/api/settings/mcp-servers/install",
			[]byte(`{"entry_id":"github","scope":"global","values":{},"command":"operator-command"}`),
		)
		if got, want := response.Code, http.StatusBadRequest; got != want {
			t.Fatalf("override status = %d, want %d; body=%s", got, want, response.Body.String())
		}
		body := response.Body.String()
		if !strings.Contains(body, "unknown_field") || !strings.Contains(body, "command") {
			t.Fatalf("override response = %s, want normalized unknown_field with command detail", body)
		}
		if service.InstallMCPCatalogCalls != 0 {
			t.Fatalf("InstallMCPCatalogCalls = %d, want 0 after strict decode failure", service.InstallMCPCatalogCalls)
		}
	})

	t.Run("Should accept null values for an input-free catalog entry", func(t *testing.T) {
		t.Parallel()

		service := newService()
		fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/api/settings/mcp-servers/install",
			[]byte(`{"entry_id":"linear","name":"linear","scope":"global","values":null}`),
		)
		if got, want := response.Code, http.StatusOK; got != want {
			t.Fatalf("null values status = %d, want %d; body=%s", got, want, response.Body.String())
		}
		if service.InstallMCPCatalogCalls != 1 {
			t.Fatalf("InstallMCPCatalogCalls = %d, want 1", service.InstallMCPCatalogCalls)
		}
		if service.LastMCPCatalogInstall.Values.Env != nil ||
			service.LastMCPCatalogInstall.Values.OAuthClientSecret != nil {
			t.Fatalf("install values = %#v, want empty values", service.LastMCPCatalogInstall.Values)
		}
	})

	t.Run("Should reject omitted values required by the public contract", func(t *testing.T) {
		t.Parallel()

		service := newService()
		fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/api/settings/mcp-servers/install",
			[]byte(`{"entry_id":"linear","name":"linear","scope":"global"}`),
		)
		if got, want := response.Code, http.StatusBadRequest; got != want {
			t.Fatalf("omitted values status = %d, want %d; body=%s", got, want, response.Body.String())
		}
		if service.InstallMCPCatalogCalls != 0 {
			t.Fatalf("InstallMCPCatalogCalls = %d, want 0", service.InstallMCPCatalogCalls)
		}
	})
}

func TestSettingsMCPServerMutationsPreserveScopeWorkspaceTargetAndMutationMetadata(t *testing.T) {
	t.Parallel()

	service := &stubSettingsService{
		PutCollectionItemFn: func(_ context.Context, req settingspkg.CollectionItemPutRequest) (settingspkg.MutationResult, error) {
			return settingspkg.MutationResult{
				Section:         settingspkg.SectionName(req.Collection),
				Scope:           req.Scope,
				WriteTarget:     settingspkg.WriteTargetWorkspaceMCPSidecar,
				WorkspaceID:     req.WorkspaceID,
				Behavior:        settingspkg.MutationBehaviorRestartRequired,
				Applied:         false,
				RestartRequired: true,
				RestartScope:    "daemon",
				Warnings:        []string{"restart required"},
			}, nil
		},
		DeleteItemFn: func(_ context.Context, req settingspkg.CollectionItemDeleteRequest) (settingspkg.MutationResult, error) {
			return settingspkg.MutationResult{
				Section:         settingspkg.SectionName(req.Collection),
				Scope:           req.Scope,
				WriteTarget:     settingspkg.WriteTargetWorkspaceMCPSidecar,
				WorkspaceID:     req.WorkspaceID,
				Behavior:        settingspkg.MutationBehaviorRestartRequired,
				Applied:         false,
				RestartRequired: true,
				RestartScope:    "daemon",
			}, nil
		},
	}
	fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)

	putBody := mustJSON(t, contract.PutSettingsMCPServerRequest{
		Server: contract.SettingsMCPServerPayload{
			Name:    "server-a",
			Command: "mcpd",
			Args:    []string{"serve"},
			SecretEnv: map[string]string{
				"TOKEN": "vault:mcp/server-a/env/TOKEN",
			},
		},
		SecretValues: &contract.SettingsMCPSecretValuesPayload{
			SecretEnv: map[string]string{"TOKEN": "server-token"},
		},
		PreserveSecrets: &contract.SettingsMCPSecretPreservationPayload{
			SecretEnv:         []string{"EXISTING_TOKEN"},
			OAuthClientSecret: true,
		},
		PreserveEnv: []string{"PROJECT"},
	})
	putResp := performRequest(
		t,
		fixture.Engine,
		http.MethodPut,
		"/api/settings/mcp-servers/server-a?scope=workspace&workspace_id=ws-123&target=sidecar",
		putBody,
	)
	if got, want := putResp.Code, http.StatusOK; got != want {
		t.Fatalf("PUT status = %d, want %d; body=%s", got, want, putResp.Body.String())
	}
	if got := service.LastPutCollectionRequest.Scope; got != settingspkg.ScopeWorkspace {
		t.Fatalf("LastPutCollectionRequest.Scope = %q, want %q", got, settingspkg.ScopeWorkspace)
	}
	if got := service.LastPutCollectionRequest.WorkspaceID; got != "ws-123" {
		t.Fatalf("LastPutCollectionRequest.WorkspaceID = %q, want ws-123", got)
	}
	if got := service.LastPutCollectionRequest.Target; got != settingspkg.TargetSidecar {
		t.Fatalf("LastPutCollectionRequest.Target = %q, want %q", got, settingspkg.TargetSidecar)
	}
	if service.LastPutCollectionRequest.MCPServer == nil ||
		service.LastPutCollectionRequest.MCPServer.Name != "server-a" {
		t.Fatalf(
			"LastPutCollectionRequest.MCPServer = %#v, want populated request payload",
			service.LastPutCollectionRequest.MCPServer,
		)
	}
	if got, want := service.LastPutCollectionRequest.MCPSecrets.SecretEnv["TOKEN"], "server-token"; got != want {
		t.Fatalf("LastPutCollectionRequest.MCPSecrets.SecretEnv[TOKEN] = %q, want %q", got, want)
	}
	preservedSecretEnv := service.LastPutCollectionRequest.MCPSecretPreservation.SecretEnv
	if len(preservedSecretEnv) != 1 || preservedSecretEnv[0] != "EXISTING_TOKEN" {
		t.Fatalf(
			"LastPutCollectionRequest.MCPSecretPreservation.SecretEnv = %#v, want [EXISTING_TOKEN]",
			preservedSecretEnv,
		)
	}
	if !service.LastPutCollectionRequest.MCPSecretPreservation.OAuthClientSecret {
		t.Fatal("LastPutCollectionRequest.MCPSecretPreservation.OAuthClientSecret = false, want true")
	}
	gotEnvPreservation := service.LastPutCollectionRequest.MCPEnvPreservation
	wantEnvPreservation := []string{"PROJECT"}
	if !reflect.DeepEqual(gotEnvPreservation, wantEnvPreservation) {
		t.Fatalf(
			"LastPutCollectionRequest.MCPEnvPreservation = %#v, want %#v",
			gotEnvPreservation,
			wantEnvPreservation,
		)
	}
	if strings.Contains(putResp.Body.String(), "server-token") {
		t.Fatalf("PUT response leaked raw secret value: %s", putResp.Body.String())
	}

	var putPayload contract.SettingsApplyResponse
	decodeJSON(t, putResp.Body.Bytes(), &putPayload)
	if putPayload.ApplyRecordID == "" ||
		putPayload.Applied ||
		!putPayload.RestartRequired ||
		putPayload.NextAction != contract.SettingsApplyNextActionRestartDaemon {
		t.Fatalf("putPayload = %#v", putPayload)
	}

	deleteResp := performRequest(
		t,
		fixture.Engine,
		http.MethodDelete,
		"/api/settings/mcp-servers/server-a?scope=workspace&workspace_id=ws-123&target=sidecar",
		nil,
	)
	if got, want := deleteResp.Code, http.StatusOK; got != want {
		t.Fatalf("DELETE status = %d, want %d; body=%s", got, want, deleteResp.Body.String())
	}
	if got := service.LastDeleteRequest.Scope; got != settingspkg.ScopeWorkspace {
		t.Fatalf("LastDeleteRequest.Scope = %q, want %q", got, settingspkg.ScopeWorkspace)
	}
	if got := service.LastDeleteRequest.WorkspaceID; got != "ws-123" {
		t.Fatalf("LastDeleteRequest.WorkspaceID = %q, want ws-123", got)
	}
	if got := service.LastDeleteRequest.Target; got != settingspkg.TargetSidecar {
		t.Fatalf("LastDeleteRequest.Target = %q, want %q", got, settingspkg.TargetSidecar)
	}
}

func TestListSettingsApplyRecordsReturnsBlockedDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("Should return blocked apply records with redacted diagnostics", func(t *testing.T) {
		t.Parallel()

		diagnostic := diagnostics.NewItem(
			"config.apply.restart_required",
			diagnosticcontract.CodeConfigRestartRequired,
			diagnosticcontract.CategoryConfig,
			"Daemon restart required",
			"Restart required for token=super-secret",
			diagnosticcontract.SeverityWarn,
			diagnosticcontract.FreshnessLive,
		)
		service := &stubSettingsService{
			ListApplyRecordsFn: func(
				_ context.Context,
				filter settingspkg.ApplyRecordFilter,
			) ([]settingspkg.ApplyRecord, error) {
				if got, want := filter.Status, lifecycle.StatusBlocked; got != want {
					t.Fatalf("filter.Status = %q, want %q", got, want)
				}
				return []settingspkg.ApplyRecord{{
					ID:          "cfgapp-1",
					DesiredHash: "sha256:desired",
					ActiveHash:  "sha256:active",
					Generation:  7,
					Actor:       "http",
					DiffClass:   lifecycle.DiffClassRestartRequired,
					Status:      lifecycle.StatusBlocked,
					Lifecycle:   lifecycle.RestartRequired,
					NextAction:  lifecycle.NextActionRestartDaemon,
					Diagnostics: []diagnosticcontract.DiagnosticItem{diagnostic},
					CreatedAt:   time.Unix(1, 0).UTC(),
					UpdatedAt:   time.Unix(2, 0).UTC(),
				}}, nil
			},
		}
		fixture := newSettingsHandlerFixture(t, "api-core-http", service, nil)

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/api/settings/apply?status=blocked", nil)
		if got, want := resp.Code, http.StatusOK; got != want {
			t.Fatalf("GET status = %d, want %d; body=%s", got, want, resp.Body.String())
		}
		var payload contract.ConfigApplyRecordsResponse
		decodeJSON(t, resp.Body.Bytes(), &payload)
		if len(payload.Entries) != 1 {
			t.Fatalf("entries len = %d, want 1", len(payload.Entries))
		}
		entry := payload.Entries[0]
		if got, want := entry.Status, contract.ConfigApplyStatusBlocked; got != want {
			t.Fatalf("entry.Status = %q, want %q", got, want)
		}
		if len(entry.Diagnostics) != 1 {
			t.Fatalf("entry.Diagnostics len = %d, want 1", len(entry.Diagnostics))
		}
		if strings.Contains(entry.Diagnostics[0].Message, "super-secret") {
			t.Fatalf("diagnostic message leaked secret: %q", entry.Diagnostics[0].Message)
		}
	})
}

func TestSettingsHandlersBehaveIdenticallyAcrossTransportShims(t *testing.T) {
	t.Parallel()

	observabilityEnvelope := settingspkg.SectionEnvelope{
		Section:         settingspkg.SectionObservability,
		Scope:           settingspkg.ScopeGlobal,
		AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal},
		Observability: &settingspkg.ObservabilitySection{
			Config: aghconfig.ObservabilityConfig{
				Enabled:        true,
				RetentionDays:  7,
				MaxGlobalBytes: 1024,
				Transcripts: aghconfig.ObservabilityTranscriptConfig{
					Enabled:            true,
					SegmentBytes:       2048,
					MaxBytesPerSession: 4096,
				},
			},
			Runtime: settingspkg.ObservabilityRuntimeStatus{
				Available:          true,
				Status:             "ok",
				GlobalDBSizeBytes:  10,
				SessionDBSizeBytes: 20,
				ActiveSessions:     2,
				ActiveAgents:       1,
				UptimeSeconds:      300,
			},
			LogTailSupport: settingspkg.CapabilityStatus{Available: true},
		},
	}
	serviceFactory := func() *stubSettingsService {
		return &stubSettingsService{
			GetSectionFn: func(_ context.Context, req settingspkg.SectionRequest) (settingspkg.SectionEnvelope, error) {
				if req.Section != settingspkg.SectionObservability {
					return settingspkg.SectionEnvelope{}, errors.New("unexpected section")
				}
				return observabilityEnvelope, nil
			},
		}
	}
	restartFactory := func() *stubSettingsRestartController {
		return &stubSettingsRestartController{
			RequestFn: func(context.Context) (core.SettingsRestartOperation, error) {
				return core.SettingsRestartOperation{
					OperationID:        "op-shared",
					Status:             "stopping",
					ActiveSessionCount: 2,
				}, nil
			},
			StatusFn: func(_ context.Context, operationID string) (core.SettingsRestartOperation, error) {
				return core.SettingsRestartOperation{
					OperationID:        operationID,
					Status:             "starting",
					OldPID:             100,
					OldStartedAt:       time.Date(2026, 4, 17, 18, 0, 0, 0, time.UTC),
					OldSocketPath:      "/tmp/agh.sock",
					ActiveSessionCount: 2,
					StartedAt:          time.Date(2026, 4, 17, 18, 44, 0, 0, time.UTC),
					UpdatedAt:          time.Date(2026, 4, 17, 18, 44, 30, 0, time.UTC),
				}, nil
			},
		}
	}

	httpFixture := newSettingsHandlerFixture(t, "httpapi", serviceFactory(), restartFactory())
	udsFixture := newSettingsHandlerFixture(t, "udsapi", serviceFactory(), restartFactory())

	for _, path := range []string{
		"/api/settings/observability",
		"/api/settings/actions/restart",
		"/api/settings/actions/restart/op-shared",
	} {
		var body []byte
		method := http.MethodGet
		if path == "/api/settings/actions/restart" {
			method = http.MethodPost
			body = []byte(`{}`)
		}

		httpResp := performRequest(t, httpFixture.Engine, method, path, body)
		udsResp := performRequest(t, udsFixture.Engine, method, path, body)
		if httpResp.Code != udsResp.Code {
			t.Fatalf("%s status mismatch: http=%d uds=%d", path, httpResp.Code, udsResp.Code)
		}
		if httpResp.Body.String() != udsResp.Body.String() {
			t.Fatalf("%s body mismatch:\nhttp=%s\nuds=%s", path, httpResp.Body.String(), udsResp.Body.String())
		}
	}

	var observability contract.SettingsObservabilityResponse
	decodeJSON(
		t,
		performRequest(t, httpFixture.Engine, http.MethodGet, "/api/settings/observability", nil).Body.Bytes(),
		&observability,
	)
	if !observability.LogTail.Available || observability.LogTail.StreamURL != "/api/settings/observability/log-tail" {
		t.Fatalf("observability.LogTail = %#v, want SSE metadata", observability.LogTail)
	}
	if observability.LogTail.Transport != contract.SettingsStreamTransportSSE {
		t.Fatalf(
			"observability.LogTail.Transport = %q, want %q",
			observability.LogTail.Transport,
			contract.SettingsStreamTransportSSE,
		)
	}
}

func TestStreamSettingsObservabilityLogTailEmitsSSEEvent(t *testing.T) {
	t.Parallel()

	fixture := newSettingsHandlerFixture(t, "api-core-http", &stubSettingsService{}, nil)
	if err := os.WriteFile(fixture.HomePaths.LogFile, []byte{}, 0o644); err != nil {
		t.Fatalf("WriteFile(log) error = %v", err)
	}

	server := httptest.NewServer(fixture.Engine)
	defer server.Close()

	reqCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(
		reqCtx,
		http.MethodGet,
		server.URL+"/api/settings/observability/log-tail",
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() error = %v", err)
	}

	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	defer resp.Body.Close()

	appendLine(t, fixture.HomePaths.LogFile, "daemon restarted cleanly\n")

	reader := bufio.NewReader(resp.Body)
	first := readStreamLine(t, reader)
	second := readStreamLine(t, reader)
	cancel()

	if first != "event: log" {
		t.Fatalf("first SSE line = %q, want event header", first)
	}
	if !strings.Contains(second, `"line":"daemon restarted cleanly"`) {
		t.Fatalf("second SSE line = %q, want log payload", second)
	}
	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()

	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return payload
}

func decodeJSON(t *testing.T, body []byte, dest any) {
	t.Helper()
	if err := json.Unmarshal(body, dest); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", string(body), err)
	}
}

func appendLine(t *testing.T, path string, line string) {
	t.Helper()

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatalf("OpenFile(%s) error = %v", path, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, bytes.NewBufferString(line)); err != nil {
		t.Fatalf("write log line error = %v", err)
	}
	if err := file.Sync(); err != nil {
		t.Fatalf("file.Sync() error = %v", err)
	}
}

func readStreamLine(t *testing.T, reader *bufio.Reader) string {
	t.Helper()

	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := reader.ReadString('\n')
		ch <- result{line: strings.TrimRight(line, "\r\n"), err: err}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			t.Fatalf("ReadString() error = %v", res.err)
		}
		return res.line
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SSE line")
		return ""
	}
}

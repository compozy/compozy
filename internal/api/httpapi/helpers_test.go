package httpapi

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	"github.com/compozy/agh/internal/session"
	settingspkg "github.com/compozy/agh/internal/settings"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/vault"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

type stubSessionManager = testutil.StubSessionManager
type stubObserver = testutil.StubObserver
type stubTaskManager = testutil.StubTaskManager
type stubBridgeService = testutil.StubBridgeService
type stubResourceService = testutil.StubResourceService
type stubWorkspaceService = testutil.StubWorkspaceService
type sseRecord = testutil.SSERecord

type stubVaultService struct{}

func (stubVaultService) GetMetadata(context.Context, string) (vault.Metadata, error) {
	return vault.Metadata{}, vault.ErrSecretNotFound
}

func (stubVaultService) ListMetadata(context.Context, string) ([]vault.Metadata, error) {
	return nil, nil
}

func (stubVaultService) PutSecret(context.Context, string, string, string) (vault.Metadata, error) {
	return vault.Metadata{}, nil
}

func (stubVaultService) DeleteSecret(context.Context, string) error {
	return nil
}

type stubExtensionService struct {
	ListFn             func(context.Context) ([]contract.ExtensionPayload, error)
	MarketplaceTrustFn func(context.Context, extensionpkg.MarketplaceTrustEvidence) (contract.ExtensionTrustReportPayload, error)
	InstallFn          func(context.Context, contract.InstallExtensionRequest, taskpkg.ActorContext) (contract.ExtensionPayload, error)
	UpdateFn           func(context.Context, string, contract.UpdateExtensionRequest, taskpkg.ActorContext) (contract.ManagedExtensionUpdatePayload, error)
	RemoveFn           func(context.Context, string, taskpkg.ActorContext) (contract.ManagedExtensionRemovePayload, error)
	EnableFn           func(context.Context, string, taskpkg.ActorContext) (contract.ExtensionPayload, error)
	DisableFn          func(context.Context, string, taskpkg.ActorContext) (contract.ExtensionPayload, error)
	StatusFn           func(context.Context, string) (contract.ExtensionPayload, error)
	ProvenanceFn       func(context.Context, string) (contract.ExtensionProvenancePayload, error)
}

func (s stubExtensionService) List(ctx context.Context) ([]contract.ExtensionPayload, error) {
	if s.ListFn == nil {
		return nil, nil
	}
	return s.ListFn(ctx)
}

func (s stubExtensionService) MarketplaceTrust(
	ctx context.Context,
	evidence extensionpkg.MarketplaceTrustEvidence,
) (contract.ExtensionTrustReportPayload, error) {
	if s.MarketplaceTrustFn == nil {
		return extensionpkg.MarketplaceEntryTrustReport(evidence, false)
	}
	return s.MarketplaceTrustFn(ctx, evidence)
}

func (s stubExtensionService) Install(
	ctx context.Context,
	req contract.InstallExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if s.InstallFn == nil {
		return contract.ExtensionPayload{}, nil
	}
	return s.InstallFn(ctx, req, actor)
}

func (s stubExtensionService) Update(
	ctx context.Context,
	name string,
	req contract.UpdateExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ManagedExtensionUpdatePayload, error) {
	if s.UpdateFn == nil {
		return contract.ManagedExtensionUpdatePayload{}, nil
	}
	return s.UpdateFn(ctx, name, req, actor)
}

func (s stubExtensionService) Remove(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ManagedExtensionRemovePayload, error) {
	if s.RemoveFn == nil {
		return contract.ManagedExtensionRemovePayload{}, nil
	}
	return s.RemoveFn(ctx, name, actor)
}

func (s stubExtensionService) Enable(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if s.EnableFn == nil {
		return contract.ExtensionPayload{}, nil
	}
	return s.EnableFn(ctx, name, actor)
}

func (s stubExtensionService) Disable(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if s.DisableFn == nil {
		return contract.ExtensionPayload{}, nil
	}
	return s.DisableFn(ctx, name, actor)
}

func (s stubExtensionService) Status(ctx context.Context, name string) (contract.ExtensionPayload, error) {
	if s.StatusFn == nil {
		return contract.ExtensionPayload{}, nil
	}
	return s.StatusFn(ctx, name)
}

func (s stubExtensionService) Provenance(
	ctx context.Context,
	name string,
) (contract.ExtensionProvenancePayload, error) {
	if s.ProvenanceFn == nil {
		return contract.ExtensionProvenancePayload{}, nil
	}
	return s.ProvenanceFn(ctx, name)
}

type stubSettingsService struct {
	GetSectionFn                func(context.Context, settingspkg.SectionRequest) (settingspkg.SectionEnvelope, error)
	UpdateSectionFn             func(context.Context, settingspkg.SectionUpdateRequest) (settingspkg.MutationResult, error)
	ApplySectionFn              func(context.Context, settingspkg.SectionUpdateRequest) (settingspkg.ApplyResult, error)
	ListCollectionFn            func(context.Context, settingspkg.CollectionRequest) (settingspkg.CollectionEnvelope, error)
	PutCollectionItemFn         func(context.Context, settingspkg.CollectionItemPutRequest) (settingspkg.MutationResult, error)
	ApplyCollectionItemFn       func(context.Context, settingspkg.CollectionItemPutRequest) (settingspkg.ApplyResult, error)
	ApplyModelCurationFn        func(context.Context, settingspkg.ProviderModelCurationRequest) (settingspkg.ProviderModelCurationResult, error)
	InstallMCPCatalogFn         func(context.Context, settingspkg.MCPCatalogInstallRequest) (settingspkg.MCPCatalogInstallResult, error)
	BeginMCPAuthFn              func(context.Context, settingspkg.MCPAuthBeginRequest) (mcpauth.BeginResult, error)
	ExchangeMCPAuthFn           func(context.Context, settingspkg.MCPAuthExchangeRequest) (mcpauth.Status, error)
	CompleteMCPAuthFn           func(context.Context, string) (mcpauth.Status, error)
	LogoutMCPAuthFn             func(context.Context, settingspkg.MCPAuthTargetRequest) (mcpauth.Status, error)
	DeleteCollectionItemFn      func(context.Context, settingspkg.CollectionItemDeleteRequest) (settingspkg.MutationResult, error)
	ApplyCollectionDeleteFn     func(context.Context, settingspkg.CollectionItemDeleteRequest) (settingspkg.ApplyResult, error)
	ReloadFn                    func(context.Context) (settingspkg.ApplyResult, error)
	ListApplyRecordsFn          func(context.Context, settingspkg.ApplyRecordFilter) ([]settingspkg.ApplyRecord, error)
	LastGetSectionRequest       settingspkg.SectionRequest
	LastUpdateSectionRequest    settingspkg.SectionUpdateRequest
	LastListCollectionRequest   settingspkg.CollectionRequest
	LastPutCollectionRequest    settingspkg.CollectionItemPutRequest
	LastMCPCatalogInstall       settingspkg.MCPCatalogInstallRequest
	LastDeleteCollectionRequest settingspkg.CollectionItemDeleteRequest
	LastApplyRecordFilter       settingspkg.ApplyRecordFilter
}

func (s *stubSettingsService) GetSection(
	ctx context.Context,
	req settingspkg.SectionRequest,
) (settingspkg.SectionEnvelope, error) {
	s.LastGetSectionRequest = req
	if s.GetSectionFn == nil {
		return settingsTestSectionEnvelope(req.Section, req.Scope, req.WorkspaceID), nil
	}
	return s.GetSectionFn(ctx, req)
}

func (s *stubSettingsService) UpdateSection(
	ctx context.Context,
	req settingspkg.SectionUpdateRequest,
) (settingspkg.MutationResult, error) {
	s.LastUpdateSectionRequest = req
	if s.UpdateSectionFn == nil {
		return settingspkg.MutationResult{
			Section:         req.Section,
			Scope:           req.Scope,
			WorkspaceID:     req.WorkspaceID,
			Behavior:        settingspkg.MutationBehaviorRestartRequired,
			RestartRequired: true,
		}, nil
	}
	return s.UpdateSectionFn(ctx, req)
}

func (s *stubSettingsService) ApplySection(
	ctx context.Context,
	req settingspkg.SectionUpdateRequest,
) (settingspkg.ApplyResult, error) {
	s.LastUpdateSectionRequest = req
	if s.ApplySectionFn == nil {
		return settingsTestApplyResultForScope(req.Section, req.Scope, req.WorkspaceID), nil
	}
	return s.ApplySectionFn(ctx, req)
}

func (s *stubSettingsService) ListCollection(
	ctx context.Context,
	req settingspkg.CollectionRequest,
) (settingspkg.CollectionEnvelope, error) {
	s.LastListCollectionRequest = req
	if s.ListCollectionFn == nil {
		return settingsTestCollectionEnvelope(req.Collection, req.Scope, req.WorkspaceID), nil
	}
	return s.ListCollectionFn(ctx, req)
}

func (s *stubSettingsService) PutCollectionItem(
	ctx context.Context,
	req settingspkg.CollectionItemPutRequest,
) (settingspkg.MutationResult, error) {
	s.LastPutCollectionRequest = req
	if s.PutCollectionItemFn == nil {
		return settingspkg.MutationResult{
			Section:         settingspkg.SectionName(req.Collection),
			Scope:           req.Scope,
			WorkspaceID:     req.WorkspaceID,
			Behavior:        settingspkg.MutationBehaviorRestartRequired,
			RestartRequired: true,
		}, nil
	}
	return s.PutCollectionItemFn(ctx, req)
}

func (s *stubSettingsService) ApplyCollectionItem(
	ctx context.Context,
	req settingspkg.CollectionItemPutRequest,
) (settingspkg.ApplyResult, error) {
	s.LastPutCollectionRequest = req
	if s.ApplyCollectionItemFn == nil {
		return settingsTestApplyResultForScope(settingspkg.SectionName(req.Collection), req.Scope, req.WorkspaceID), nil
	}
	return s.ApplyCollectionItemFn(ctx, req)
}

func (s *stubSettingsService) ApplyProviderModelCuration(
	ctx context.Context,
	req settingspkg.ProviderModelCurationRequest,
) (settingspkg.ProviderModelCurationResult, error) {
	if s.ApplyModelCurationFn == nil {
		return settingspkg.ProviderModelCurationResult{}, nil
	}
	return s.ApplyModelCurationFn(ctx, req)
}

func (s *stubSettingsService) InstallMCPCatalog(
	ctx context.Context,
	req settingspkg.MCPCatalogInstallRequest,
) (settingspkg.MCPCatalogInstallResult, error) {
	s.LastMCPCatalogInstall = req
	if s.InstallMCPCatalogFn == nil {
		return settingspkg.MCPCatalogInstallResult{}, nil
	}
	return s.InstallMCPCatalogFn(ctx, req)
}

func (s *stubSettingsService) BeginMCPAuth(
	ctx context.Context,
	req settingspkg.MCPAuthBeginRequest,
) (mcpauth.BeginResult, error) {
	if s.BeginMCPAuthFn == nil {
		return mcpauth.BeginResult{}, nil
	}
	return s.BeginMCPAuthFn(ctx, req)
}

func (s *stubSettingsService) ExchangeMCPAuth(
	ctx context.Context,
	req settingspkg.MCPAuthExchangeRequest,
) (mcpauth.Status, error) {
	if s.ExchangeMCPAuthFn == nil {
		return mcpauth.Status{}, nil
	}
	return s.ExchangeMCPAuthFn(ctx, req)
}

func (s *stubSettingsService) CompleteMCPAuthCallback(
	ctx context.Context,
	callbackURL string,
) (mcpauth.Status, error) {
	if s.CompleteMCPAuthFn == nil {
		return mcpauth.Status{}, nil
	}
	return s.CompleteMCPAuthFn(ctx, callbackURL)
}

func (s *stubSettingsService) LogoutMCPAuth(
	ctx context.Context,
	req settingspkg.MCPAuthTargetRequest,
) (mcpauth.Status, error) {
	if s.LogoutMCPAuthFn == nil {
		return mcpauth.Status{}, nil
	}
	return s.LogoutMCPAuthFn(ctx, req)
}

func (s *stubSettingsService) DeleteCollectionItem(
	ctx context.Context,
	req settingspkg.CollectionItemDeleteRequest,
) (settingspkg.MutationResult, error) {
	s.LastDeleteCollectionRequest = req
	if s.DeleteCollectionItemFn == nil {
		return settingspkg.MutationResult{
			Section:         settingspkg.SectionName(req.Collection),
			Scope:           req.Scope,
			WorkspaceID:     req.WorkspaceID,
			Behavior:        settingspkg.MutationBehaviorRestartRequired,
			RestartRequired: true,
		}, nil
	}
	return s.DeleteCollectionItemFn(ctx, req)
}

func (s *stubSettingsService) ApplyCollectionDelete(
	ctx context.Context,
	req settingspkg.CollectionItemDeleteRequest,
) (settingspkg.ApplyResult, error) {
	s.LastDeleteCollectionRequest = req
	if s.ApplyCollectionDeleteFn == nil {
		return settingsTestApplyResultForScope(settingspkg.SectionName(req.Collection), req.Scope, req.WorkspaceID), nil
	}
	return s.ApplyCollectionDeleteFn(ctx, req)
}

func (s *stubSettingsService) Reload(ctx context.Context) (settingspkg.ApplyResult, error) {
	if s.ReloadFn == nil {
		return settingsTestApplyResult(""), nil
	}
	return s.ReloadFn(ctx)
}

func (s *stubSettingsService) ListApplyRecords(
	ctx context.Context,
	filter settingspkg.ApplyRecordFilter,
) ([]settingspkg.ApplyRecord, error) {
	s.LastApplyRecordFilter = filter
	if s.ListApplyRecordsFn == nil {
		return nil, nil
	}
	return s.ListApplyRecordsFn(ctx, filter)
}

func settingsTestApplyResult(section settingspkg.SectionName) settingspkg.ApplyResult {
	return settingsTestApplyResultForScope(section, settingspkg.ScopeGlobal, "")
}

func settingsTestApplyResultForScope(
	section settingspkg.SectionName,
	scope settingspkg.ScopeKind,
	workspaceID string,
) settingspkg.ApplyResult {
	return settingspkg.ApplyResult{
		Section:     section,
		Scope:       scope,
		WorkspaceID: workspaceID,
		Applied:     true,
		NextAction:  "none",
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

type stubSettingsRestartController struct {
	RequestRestartFn      func(context.Context) (core.SettingsRestartOperation, error)
	GetRestartOperationFn func(context.Context, string) (core.SettingsRestartOperation, error)
	RequestRestartCalls   int
	GetRestartOperationID string
}

func (s *stubSettingsRestartController) RequestRestart(ctx context.Context) (core.SettingsRestartOperation, error) {
	s.RequestRestartCalls++
	if s.RequestRestartFn == nil {
		return core.SettingsRestartOperation{
			OperationID:        "op-123",
			Status:             "pending",
			ActiveSessionCount: 1,
		}, nil
	}
	return s.RequestRestartFn(ctx)
}

func (s *stubSettingsRestartController) GetRestartOperation(
	ctx context.Context,
	operationID string,
) (core.SettingsRestartOperation, error) {
	s.GetRestartOperationID = operationID
	if s.GetRestartOperationFn == nil {
		return core.SettingsRestartOperation{
			OperationID:        operationID,
			Status:             "ready",
			ActiveSessionCount: 1,
			StartedAt:          time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
			UpdatedAt:          time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC),
		}, nil
	}
	return s.GetRestartOperationFn(ctx, operationID)
}

func newTestHandlers(
	t *testing.T,
	manager core.SessionManager,
	observer core.Observer,
	homePaths aghconfig.HomePaths,
) *Handlers {
	t.Helper()
	return newTestHandlersWithAutomationBridgesTasksAndWorkspace(
		t,
		manager,
		observer,
		nil,
		&stubTaskManager{},
		nil,
		stubWorkspaceService{},
		homePaths,
	)
}

func newTestHandlersWithBridges(
	t *testing.T,
	manager core.SessionManager,
	observer core.Observer,
	bridges core.BridgeService,
	workspaces core.WorkspaceService,
	homePaths aghconfig.HomePaths,
) *Handlers {
	t.Helper()
	return newTestHandlersWithAutomationBridgesTasksAndWorkspace(
		t,
		manager,
		observer,
		nil,
		&stubTaskManager{},
		bridges,
		workspaces,
		homePaths,
	)
}

func newTestHandlersWithAutomationBridgesTasksAndWorkspace(
	t *testing.T,
	manager core.SessionManager,
	observer core.Observer,
	automation core.AutomationManager,
	tasks core.TaskService,
	bridges core.BridgeService,
	workspaces core.WorkspaceService,
	homePaths aghconfig.HomePaths,
) *Handlers {
	t.Helper()

	cfg := testConfigWithDisabledNetwork(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = 2123
	manager = defaultTestSessionManager(manager)
	workspaces = defaultTestWorkspaceService(workspaces)

	return newHandlers(&handlerConfig{
		sessions:       manager,
		sessionCatalog: defaultTestSessionCatalog(manager),
		tasks:          tasks,
		observer:       observer,
		automation:     automation,
		bridges:        bridges,
		workspaces:     workspaces,
		staticFS:       mustStaticFS(t),
		homePaths:      homePaths,
		config:         cfg,
		boundHost:      cfg.HTTP.Host,
		logger:         discardLogger(),
		startedAt:      time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		now:            func() time.Time { return time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC) },
		pollInterval:   5 * time.Millisecond,
		agentLoader:    aghconfig.LoadAgentDef,
		httpPort:       cfg.HTTP.Port,
	})
}

func defaultTestSessionManager(manager core.SessionManager) core.SessionManager {
	stub, ok := manager.(stubSessionManager)
	if !ok || stub.StatusFn != nil {
		return manager
	}
	stub.StatusFn = func(ctx context.Context, id string) (*session.Info, error) {
		trimmedID := strings.TrimSpace(id)
		if stub.ListAllFn != nil {
			infos, err := stub.ListAllFn(ctx)
			if err != nil {
				return nil, err
			}
			for _, info := range infos {
				if info != nil && strings.TrimSpace(info.ID) == trimmedID {
					return info, nil
				}
			}
			return nil, session.ErrSessionNotFound
		}
		info := newSessionInfo(trimmedID)
		info.WorkspaceID = "ws-workspace"
		info.Workspace = "/workspace"
		info.State = session.StateActive
		return info, nil
	}
	return stub
}

func defaultTestSessionCatalog(manager core.SessionManager) core.SessionCatalog {
	catalog, ok := manager.(core.SessionCatalog)
	if !ok {
		return nil
	}
	return catalog
}

func defaultTestWorkspaceService(workspaces core.WorkspaceService) core.WorkspaceService {
	stub, ok := workspaces.(stubWorkspaceService)
	if !ok || stub.ResolveFn != nil {
		return workspaces
	}
	stub.ResolveFn = func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
		if ref != "ws-workspace" {
			return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
		}
		return workspacepkg.ResolvedWorkspace{
			Workspace:   workspacepkg.Workspace{ID: "ws-workspace", RootDir: "/workspace", Name: "Workspace"},
			WorkspaceID: "ws-workspace",
		}, nil
	}
	return stub
}

func newTestHandlersWithWorkspace(
	t *testing.T,
	manager core.SessionManager,
	observer core.Observer,
	workspaces core.WorkspaceService,
	homePaths aghconfig.HomePaths,
) *Handlers {
	t.Helper()

	return newTestHandlersWithBridges(t, manager, observer, nil, workspaces, homePaths)
}

func newTestHandlersWithResources(
	t *testing.T,
	manager core.SessionManager,
	observer core.Observer,
	resources core.ResourceService,
	homePaths aghconfig.HomePaths,
) *Handlers {
	t.Helper()

	cfg := testConfigWithDisabledNetwork(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = 2123

	return newHandlers(&handlerConfig{
		sessions:       manager,
		sessionCatalog: defaultTestSessionCatalog(manager),
		tasks:          &stubTaskManager{},
		observer:       observer,
		resources:      resources,
		workspaces:     stubWorkspaceService{},
		staticFS:       mustStaticFS(t),
		homePaths:      homePaths,
		config:         cfg,
		boundHost:      cfg.HTTP.Host,
		logger:         discardLogger(),
		startedAt:      time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		now:            func() time.Time { return time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC) },
		pollInterval:   5 * time.Millisecond,
		agentLoader:    aghconfig.LoadAgentDef,
		httpPort:       cfg.HTTP.Port,
	})
}

func newTestHandlersWithResourcesAndAuth(
	t *testing.T,
	manager core.SessionManager,
	observer core.Observer,
	resources core.ResourceService,
	auth ...gin.HandlerFunc,
) *Handlers {
	t.Helper()

	homePaths := newTestHomePaths(t)
	cfg := testConfigWithDisabledNetwork(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = 2123

	return newHandlers(&handlerConfig{
		sessions:       manager,
		sessionCatalog: defaultTestSessionCatalog(manager),
		tasks:          &stubTaskManager{},
		observer:       observer,
		resources:      resources,
		workspaces:     stubWorkspaceService{},
		staticFS:       mustStaticFS(t),
		homePaths:      homePaths,
		config:         cfg,
		boundHost:      cfg.HTTP.Host,
		logger:         discardLogger(),
		startedAt:      time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		now:            func() time.Time { return time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC) },
		pollInterval:   5 * time.Millisecond,
		agentLoader:    aghconfig.LoadAgentDef,
		httpPort:       cfg.HTTP.Port,
		resourceAuth:   append([]gin.HandlerFunc(nil), auth...),
	})
}

func newTestHandlersWithSettingsAndExtensions(
	t *testing.T,
	boundHost string,
	settings core.SettingsService,
	restart core.SettingsRestartController,
	extensions ExtensionService,
	homePaths aghconfig.HomePaths,
) *Handlers {
	t.Helper()

	cfg := testConfigWithDisabledNetwork(homePaths)
	cfg.HTTP.Host = boundHost
	cfg.HTTP.Port = 2123

	return newHandlers(&handlerConfig{
		sessions:        stubSessionManager{},
		tasks:           &stubTaskManager{},
		observer:        stubObserver{},
		workspaces:      stubWorkspaceService{},
		settings:        settings,
		settingsRestart: restart,
		extensions:      extensions,
		staticFS:        mustStaticFS(t),
		homePaths:       homePaths,
		config:          cfg,
		boundHost:       cfg.HTTP.Host,
		logger:          discardLogger(),
		startedAt:       time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		now:             func() time.Time { return time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC) },
		pollInterval:    5 * time.Millisecond,
		agentLoader:     aghconfig.LoadAgentDef,
		httpPort:        cfg.HTTP.Port,
	})
}

func newTestRouter(t *testing.T, handlers *Handlers) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(requestLoggingMiddleware(discardLogger()))
	boundHost := "127.0.0.1"
	if handlers != nil && handlers.boundHost != "" {
		boundHost = handlers.boundHost
	}
	engine.Use(corsMiddleware(boundHost))
	engine.Use(requestBodyLimitMiddleware(maxAPIRequestBodyBytes))
	engine.Use(errorMiddleware())
	RegisterRoutes(engine, handlers)
	return engine
}

func mustStaticFS(t *testing.T) fs.FS {
	t.Helper()

	staticFS, err := newStaticFS()
	if err != nil {
		t.Fatalf("newStaticFS() error = %v", err)
	}

	return staticFS.fs
}

func newTestHomePaths(t *testing.T) aghconfig.HomePaths {
	t.Helper()
	return testutil.NewTestHomePaths(t)
}

func testConfigWithDisabledNetwork(homePaths aghconfig.HomePaths) aghconfig.Config {
	return testutil.ConfigWithDisabledNetwork(homePaths)
}

func writeAgentDef(t *testing.T, homePaths aghconfig.HomePaths, name string) {
	t.Helper()
	testutil.WriteAgentDef(t, homePaths, name)
}

func newSessionInfo(id string) *session.Info {
	return testutil.NewSessionInfo(id)
}

func newSession(id string) *session.Session {
	return testutil.NewSession(id)
}

func performRequest(t *testing.T, engine http.Handler, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	return testutil.PerformRequest(t, engine, method, path, body)
}

func performRequestWithHeaders(
	t *testing.T,
	engine http.Handler,
	method, path string,
	body []byte,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	return testutil.PerformRequestWithHeaders(t, engine, method, path, body, headers)
}

func decodeJSONResponse(t *testing.T, recorder *httptest.ResponseRecorder, dest any) {
	t.Helper()
	testutil.DecodeJSONResponse(t, recorder, dest)
}

func mustJSONBody(t *testing.T, value any) []byte {
	t.Helper()
	return testutil.MustJSONBody(t, value)
}

func parseSSE(t *testing.T, body string) []sseRecord {
	t.Helper()
	return testutil.ParseSSE(t, body)
}

func freeTCPPort(t *testing.T) int {
	t.Helper()

	var listenConfig net.ListenConfig
	ln, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(:0) error = %v", err)
	}
	defer func() {
		_ = ln.Close()
	}()

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener addr type = %T, want *net.TCPAddr", ln.Addr())
	}
	return tcpAddr.Port
}

func mustURL(host string, port int, path string) string {
	return fmt.Sprintf("http://%s:%d%s", host, port, path)
}

func discardLogger() *slog.Logger {
	return testutil.DiscardLogger()
}

func settingsTestSectionEnvelope(
	section settingspkg.SectionName,
	scope settingspkg.ScopeKind,
	workspaceID string,
) settingspkg.SectionEnvelope {
	envelope := settingspkg.SectionEnvelope{
		Section:         section,
		Scope:           scope,
		WorkspaceID:     workspaceID,
		AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal},
	}
	switch section {
	case settingspkg.SectionGeneral:
		envelope.General = &settingspkg.GeneralSection{}
	case settingspkg.SectionMemory:
		envelope.Memory = &settingspkg.MemorySection{}
	case settingspkg.SectionSkills:
		envelope.Skills = &settingspkg.SkillsSection{}
	case settingspkg.SectionAutomation:
		envelope.Automation = &settingspkg.AutomationSection{}
	case settingspkg.SectionNetwork:
		envelope.Network = &settingspkg.NetworkSection{}
	case settingspkg.SectionObservability:
		envelope.Observability = &settingspkg.ObservabilitySection{}
	case settingspkg.SectionHooksExtensions:
		envelope.HooksExtensions = &settingspkg.HooksExtensionsSection{}
	}
	return envelope
}

func settingsTestCollectionEnvelope(
	collection settingspkg.CollectionName,
	scope settingspkg.ScopeKind,
	workspaceID string,
) settingspkg.CollectionEnvelope {
	envelope := settingspkg.CollectionEnvelope{
		Collection:      collection,
		Scope:           scope,
		WorkspaceID:     workspaceID,
		AvailableScopes: []settingspkg.ScopeKind{settingspkg.ScopeGlobal},
	}
	switch collection {
	case settingspkg.CollectionProviders:
		envelope.Providers = []settingspkg.ProviderItem{{
			Name:     "demo",
			Settings: settingspkg.ProviderSettings{Command: "codex"},
		}}
	case settingspkg.CollectionMCPServers:
		envelope.MCPServers = []settingspkg.MCPServerItem{{
			Name:    "server-a",
			Command: "mcpd",
			Scope:   scope,
		}}
	case settingspkg.CollectionSandboxes:
		envelope.Sandboxes = []settingspkg.SandboxItem{{
			Name:    "demo",
			Profile: aghconfig.SandboxProfile{Backend: "local"},
		}}
	case settingspkg.CollectionHooks:
		envelope.Hooks = []settingspkg.HookItem{}
	}
	return envelope
}

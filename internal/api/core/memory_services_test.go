package core_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	"github.com/compozy/agh/internal/session"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestMemoryExtractorHandlersUseInjectedService(t *testing.T) {
	t.Parallel()

	t.Run("Should use the injected memory extractor service", func(t *testing.T) {
		t.Parallel()

		extractor := &stubMemoryExtractorService{
			status: contract.MemoryExtractorStatusPayload{
				Status:                 contract.MemoryExtractorStateRunning,
				QueuedSessions:         2,
				InFlightSessions:       1,
				ActiveProviderSessions: 1,
				DroppedTurns:           3,
				CoalescedTurns:         4,
				SkippedTurns:           5,
				BackpressuredSessions:  6,
				FailureCount:           1,
			},
			failures: []contract.MemoryExtractorFailurePayload{{
				ID:        "failure-1",
				SessionID: "sess-1",
				Reason:    "decode",
				Path:      "/tmp/failure.json",
				CreatedAt: time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC),
			}},
			retry: contract.MemoryExtractorRetryResponse{Retried: 1},
			drain: contract.MemoryExtractorDrainResponse{
				DrainedAt: time.Date(2026, 5, 5, 12, 1, 0, 0, time.UTC),
			},
		}
		engine := newMemoryServiceRouter(t, &core.BaseHandlerConfig{MemoryExtractor: extractor})

		statusResp := performRequest(t, engine, http.MethodGet, "/memory/extractor/status", nil)
		if statusResp.Code != http.StatusOK {
			t.Fatalf("status code = %d, want %d", statusResp.Code, http.StatusOK)
		}
		var statusPayload contract.MemoryExtractorStatusResponse
		decodeJSON(t, statusResp.Body.Bytes(), &statusPayload)
		if got := statusPayload.Extractor.Status; got != contract.MemoryExtractorStateRunning {
			t.Fatalf("Extractor.Status = %q, want running", got)
		}
		if got := statusPayload.Extractor.FailureCount; got != 1 {
			t.Fatalf("Extractor.FailureCount = %d, want 1", got)
		}
		if got := statusPayload.Extractor.SkippedTurns; got != 5 {
			t.Fatalf("Extractor.SkippedTurns = %d, want 5", got)
		}
		if got := statusPayload.Extractor.ActiveProviderSessions; got != 1 {
			t.Fatalf("Extractor.ActiveProviderSessions = %d, want 1", got)
		}
		if got := statusPayload.Extractor.BackpressuredSessions; got != 6 {
			t.Fatalf("Extractor.BackpressuredSessions = %d, want 6", got)
		}

		failuresResp := performRequest(t, engine, http.MethodGet, "/memory/extractor/failures", nil)
		if failuresResp.Code != http.StatusOK {
			t.Fatalf("failures status code = %d, want %d", failuresResp.Code, http.StatusOK)
		}
		var failuresPayload contract.MemoryExtractorFailuresResponse
		decodeJSON(t, failuresResp.Body.Bytes(), &failuresPayload)
		if len(failuresPayload.Failures) != 1 || failuresPayload.Failures[0].ID != "failure-1" {
			t.Fatalf("Failures = %#v, want failure-1", failuresPayload.Failures)
		}

		retryResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/memory/extractor/retry",
			[]byte(`{"failure_id":"failure-1"}`),
		)
		if retryResp.Code != http.StatusOK {
			t.Fatalf("retry status code = %d, want %d", retryResp.Code, http.StatusOK)
		}
		var retryPayload contract.MemoryExtractorRetryResponse
		decodeJSON(t, retryResp.Body.Bytes(), &retryPayload)
		if retryPayload.Retried != 1 || extractor.retryReq.FailureID != "failure-1" {
			t.Fatalf("Retry response=%#v request=%#v, want failure-1 retried", retryPayload, extractor.retryReq)
		}

		drainResp := performRequest(t, engine, http.MethodPost, "/memory/extractor/drain", nil)
		if drainResp.Code != http.StatusOK {
			t.Fatalf("drain status code = %d, want %d", drainResp.Code, http.StatusOK)
		}
		var drainPayload contract.MemoryExtractorDrainResponse
		decodeJSON(t, drainResp.Body.Bytes(), &drainPayload)
		if drainPayload.DrainedAt.IsZero() || !extractor.drainCalled {
			t.Fatalf("Drain payload=%#v called=%t, want daemon service drain", drainPayload, extractor.drainCalled)
		}
	})
}

func TestMemoryProviderHandlersUseInjectedService(t *testing.T) {
	t.Parallel()

	t.Run("Should use the injected memory provider service", func(t *testing.T) {
		t.Parallel()

		providers := &stubMemoryProviderService{
			provider: contract.MemoryProviderPayload{
				Name:    "local",
				Status:  contract.MemoryProviderStateActive,
				Active:  true,
				Builtin: true,
			},
		}
		engine := newMemoryServiceRouter(t, &core.BaseHandlerConfig{MemoryProviders: providers})

		listResp := performRequest(t, engine, http.MethodGet, "/memory/providers?workspace_id=ws-1", nil)
		if listResp.Code != http.StatusOK {
			t.Fatalf("provider list status code = %d, want %d", listResp.Code, http.StatusOK)
		}
		var listPayload contract.MemoryProviderListResponse
		decodeJSON(t, listResp.Body.Bytes(), &listPayload)
		if len(listPayload.Providers) != 1 || listPayload.Providers[0].Name != "local" {
			t.Fatalf("Providers = %#v, want local", listPayload.Providers)
		}
		if providers.workspaceID != "ws-1" {
			t.Fatalf("workspaceID = %q, want ws-1", providers.workspaceID)
		}

		selectResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/memory/providers/select?workspace_id=ws-2",
			[]byte(`{"name":"local"}`),
		)
		if selectResp.Code != http.StatusOK {
			t.Fatalf("provider select status code = %d, want %d", selectResp.Code, http.StatusOK)
		}
		var selectPayload contract.MemoryProviderResponse
		decodeJSON(t, selectResp.Body.Bytes(), &selectPayload)
		if selectPayload.Provider.Name != "local" || providers.selectedName != "local" ||
			providers.workspaceID != "ws-2" {
			t.Fatalf(
				"select payload=%#v selected=%q workspace=%q, want local/ws-2",
				selectPayload,
				providers.selectedName,
				providers.workspaceID,
			)
		}

		enableResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/memory/providers/local/enable?workspace_id=ws-3",
			[]byte(`{"reason":"maintenance"}`),
		)
		if enableResp.Code != http.StatusOK {
			t.Fatalf("provider enable status code = %d, want %d", enableResp.Code, http.StatusOK)
		}
		var enablePayload contract.MemoryProviderLifecycleResponse
		decodeJSON(t, enableResp.Body.Bytes(), &enablePayload)
		if !enablePayload.Changed || providers.selectedName != "local" || providers.workspaceID != "ws-3" ||
			providers.reason != "maintenance" {
			t.Fatalf(
				"enable payload=%#v selected=%q workspace=%q reason=%q, want local/ws-3/maintenance",
				enablePayload,
				providers.selectedName,
				providers.workspaceID,
				providers.reason,
			)
		}

		disableResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/memory/providers/local/disable?workspace_id=ws-4",
			[]byte(`{"reason":"cooldown"}`),
		)
		if disableResp.Code != http.StatusOK {
			t.Fatalf("provider disable status code = %d, want %d", disableResp.Code, http.StatusOK)
		}
		var disablePayload contract.MemoryProviderLifecycleResponse
		decodeJSON(t, disableResp.Body.Bytes(), &disablePayload)
		if !disablePayload.Changed || providers.selectedName != "local" || providers.workspaceID != "ws-4" ||
			providers.reason != "cooldown" {
			t.Fatalf(
				"disable payload=%#v selected=%q workspace=%q reason=%q, want local/ws-4/cooldown",
				disablePayload,
				providers.selectedName,
				providers.workspaceID,
				providers.reason,
			)
		}
	})
}

func TestMemorySessionLedgerHandlersUseInjectedService(t *testing.T) {
	t.Parallel()

	t.Run("Should use the injected memory session ledger service", func(t *testing.T) {
		t.Parallel()

		ledger := &stubMemorySessionLedgerService{
			response: contract.MemorySessionLedgerResponse{
				Meta: contract.MemorySessionLedgerMetaPayload{
					Version:   1,
					SessionID: "sess-1",
					Path:      "/tmp/sess-1/ledger.jsonl",
					Checksum:  "abc123",
					CreatedAt: time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC),
				},
				Events: []contract.MemorySessionLedgerEntryPayload{{
					Sequence:  1,
					EventType: "agent_message",
					EmittedAt: time.Date(2026, 5, 5, 12, 0, 1, 0, time.UTC),
				}},
			},
			replay: contract.MemorySessionReplayResponse{
				SessionID: "sess-1",
				Events: []contract.MemorySessionLedgerEntryPayload{{
					Sequence:  1,
					EventType: "agent_message",
				}},
			},
		}
		engine := newMemoryServiceRouter(t, &core.BaseHandlerConfig{MemorySessionLedger: ledger})

		getResp := performRequest(
			t,
			engine,
			http.MethodGet,
			"/workspaces/ws-workspace/memory/sessions/sess-1/ledger",
			nil,
		)
		if getResp.Code != http.StatusOK {
			t.Fatalf("ledger status code = %d, want %d", getResp.Code, http.StatusOK)
		}
		var getPayload contract.MemorySessionLedgerResponse
		decodeJSON(t, getResp.Body.Bytes(), &getPayload)
		if getPayload.Meta.SessionID != "sess-1" || ledger.sessionID != "sess-1" {
			t.Fatalf("ledger payload=%#v session=%q, want sess-1", getPayload.Meta, ledger.sessionID)
		}

		replayResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/workspaces/ws-workspace/memory/sessions/sess-1/replay",
			[]byte(`{"include_tool_events":true}`),
		)
		if replayResp.Code != http.StatusOK {
			t.Fatalf("replay status code = %d, want %d", replayResp.Code, http.StatusOK)
		}
		var replayPayload contract.MemorySessionReplayResponse
		decodeJSON(t, replayResp.Body.Bytes(), &replayPayload)
		if replayPayload.SessionID != "sess-1" || !ledger.replayReq.IncludeToolEvents {
			t.Fatalf("replay payload=%#v request=%#v, want include tool events", replayPayload, ledger.replayReq)
		}
	})
}

func TestMemorySessionLedgerHandlersRejectForeignWorkspaceSession(t *testing.T) {
	t.Parallel()

	t.Run("Should reject foreign workspace before ledger service access", func(t *testing.T) {
		tests := []struct {
			name   string
			method string
			path   string
			body   []byte
		}{
			{
				name:   "Should reject ledger read",
				method: http.MethodGet,
				path:   "/workspaces/ws-workspace/memory/sessions/sess-foreign/ledger",
			},
			{
				name:   "Should reject replay",
				method: http.MethodPost,
				path:   "/workspaces/ws-workspace/memory/sessions/sess-foreign/replay",
				body:   []byte(`{"include_tool_events":true}`),
			},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				ledger := &stubMemorySessionLedgerService{}
				foreign := testutil.NewSessionInfo("sess-foreign")
				foreign.WorkspaceID = "ws-other"
				engine := newMemoryServiceRouter(t, &core.BaseHandlerConfig{
					MemorySessionLedger: ledger,
					Sessions: testutil.StubSessionManager{
						StatusFn: func(_ context.Context, _ string) (*session.Info, error) {
							return foreign, nil
						},
					},
				})

				resp := performRequest(t, engine, tc.method, tc.path, tc.body)
				if resp.Code != http.StatusNotFound {
					t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusNotFound, resp.Body.String())
				}
				if ledger.totalCalls() != 0 {
					t.Fatalf("ledger calls = %d, want 0", ledger.totalCalls())
				}
			})
		}
	})
}

func newMemoryServiceRouter(t *testing.T, cfg *core.BaseHandlerConfig) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	homePaths := testutil.NewTestHomePaths(t)
	runtimeConfig := testConfigWithDisabledNetwork(homePaths)
	cfg.HomePaths = homePaths
	cfg.Config = runtimeConfig
	cfg.Logger = testutil.DiscardLogger()
	cfg.Now = func() time.Time {
		return time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	}
	if cfg.Sessions == nil {
		cfg.Sessions = testutil.StubSessionManager{
			StatusFn: func(_ context.Context, id string) (*session.Info, error) {
				return testutil.NewSessionInfo(id), nil
			},
		}
	}
	if cfg.Workspaces == nil {
		cfg.Workspaces = testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				if ref != "ws-workspace" {
					return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
				}
				return workspacepkg.ResolvedWorkspace{
					Workspace:   workspacepkg.Workspace{ID: "ws-workspace", RootDir: "/workspace", Name: "Workspace"},
					WorkspaceID: "ws-workspace",
				}, nil
			},
		}
	}
	handlers := core.NewBaseHandlers(cfg)
	engine := gin.New()
	engine.GET("/memory/extractor/status", handlers.GetMemoryExtractorStatus)
	engine.GET("/memory/extractor/failures", handlers.ListMemoryExtractorFailures)
	engine.POST("/memory/extractor/retry", handlers.RetryMemoryExtractor)
	engine.POST("/memory/extractor/drain", handlers.DrainMemoryExtractor)
	engine.GET("/memory/providers", handlers.ListMemoryProviders)
	engine.POST("/memory/providers/select", handlers.SelectMemoryProvider)
	engine.POST("/memory/providers/:provider_name/enable", handlers.EnableMemoryProvider)
	engine.POST("/memory/providers/:provider_name/disable", handlers.DisableMemoryProvider)
	engine.GET("/workspaces/:workspace_id/memory/sessions/:session_id/ledger", handlers.GetMemorySessionLedger)
	engine.POST("/workspaces/:workspace_id/memory/sessions/:session_id/replay", handlers.ReplayMemorySession)
	return engine
}

type stubMemoryExtractorService struct {
	status      contract.MemoryExtractorStatusPayload
	failures    []contract.MemoryExtractorFailurePayload
	retry       contract.MemoryExtractorRetryResponse
	drain       contract.MemoryExtractorDrainResponse
	retryReq    contract.MemoryExtractorRetryRequest
	drainCalled bool
}

func (s *stubMemoryExtractorService) Status(context.Context) (contract.MemoryExtractorStatusPayload, error) {
	return s.status, nil
}

func (s *stubMemoryExtractorService) ListFailures(
	context.Context,
) ([]contract.MemoryExtractorFailurePayload, error) {
	return s.failures, nil
}

func (s *stubMemoryExtractorService) Retry(
	_ context.Context,
	req contract.MemoryExtractorRetryRequest,
) (contract.MemoryExtractorRetryResponse, error) {
	s.retryReq = req
	return s.retry, nil
}

func (s *stubMemoryExtractorService) Drain(context.Context) (contract.MemoryExtractorDrainResponse, error) {
	s.drainCalled = true
	return s.drain, nil
}

type stubMemoryProviderService struct {
	provider     contract.MemoryProviderPayload
	workspaceID  string
	selectedName string
	reason       string
}

func (s *stubMemoryProviderService) List(
	_ context.Context,
	workspaceID string,
) ([]contract.MemoryProviderPayload, error) {
	s.workspaceID = workspaceID
	return []contract.MemoryProviderPayload{s.provider}, nil
}

func (s *stubMemoryProviderService) Get(
	_ context.Context,
	workspaceID string,
	_ string,
) (contract.MemoryProviderPayload, error) {
	s.workspaceID = workspaceID
	return s.provider, nil
}

func (s *stubMemoryProviderService) Select(
	_ context.Context,
	workspaceID string,
	name string,
) (contract.MemoryProviderPayload, error) {
	s.workspaceID = workspaceID
	s.selectedName = name
	return s.provider, nil
}

func (s *stubMemoryProviderService) Enable(
	_ context.Context,
	workspaceID string,
	name string,
	reason string,
) (contract.MemoryProviderLifecycleResponse, error) {
	s.workspaceID = workspaceID
	s.selectedName = name
	s.reason = reason
	return contract.MemoryProviderLifecycleResponse{Provider: s.provider, Changed: true}, nil
}

func (s *stubMemoryProviderService) Disable(
	_ context.Context,
	workspaceID string,
	name string,
	reason string,
) (contract.MemoryProviderLifecycleResponse, error) {
	s.workspaceID = workspaceID
	s.selectedName = name
	s.reason = reason
	return contract.MemoryProviderLifecycleResponse{Provider: s.provider, Changed: true}, nil
}

type stubMemorySessionLedgerService struct {
	response    contract.MemorySessionLedgerResponse
	replay      contract.MemorySessionReplayResponse
	sessionID   string
	replayReq   contract.MemorySessionReplayRequest
	getCalls    int
	replayCalls int
}

func (s *stubMemorySessionLedgerService) Get(
	_ context.Context,
	sessionID string,
) (contract.MemorySessionLedgerResponse, error) {
	s.getCalls++
	s.sessionID = sessionID
	return s.response, nil
}

func (s *stubMemorySessionLedgerService) Replay(
	_ context.Context,
	sessionID string,
	req contract.MemorySessionReplayRequest,
) (contract.MemorySessionReplayResponse, error) {
	s.replayCalls++
	s.sessionID = sessionID
	s.replayReq = req
	return s.replay, nil
}

func (s *stubMemorySessionLedgerService) totalCalls() int {
	return s.getCalls + s.replayCalls
}

func (s *stubMemorySessionLedgerService) Prune(
	context.Context,
	contract.MemorySessionsPruneRequest,
) (contract.MemorySessionsPruneResponse, error) {
	return contract.MemorySessionsPruneResponse{}, nil
}

func (s *stubMemorySessionLedgerService) Repair(context.Context) (contract.MemorySessionsRepairResponse, error) {
	return contract.MemorySessionsRepairResponse{}, nil
}

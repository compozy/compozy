package core_test

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/observe"
	"github.com/gin-gonic/gin"
)

func TestBridgeHandlersExposeDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("Should expose diagnostics from bridge routes secrets status and health", func(t *testing.T) {
		t.Parallel()

		gin.SetMode(gin.TestMode)
		bridge := bridgepkg.BridgeInstance{
			ID:            "brg-diagnostics",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "ext-telegram",
			DisplayName:   "Support",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusAuthRequired,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			Degradation: &bridgepkg.BridgeDegradation{
				Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
				Message: "provider rejected credentials",
			},
		}
		provider := bridgepkg.BridgeProvider{
			Platform:      "telegram",
			ExtensionName: "ext-telegram",
			Enabled:       false,
			HealthMessage: "provider is disabled by policy",
			SecretSlots: []bridgepkg.BridgeSecretSlot{
				{Name: "bot_token", Required: true},
			},
		}
		homePaths := testutil.NewTestHomePaths(t)
		cfg := aghconfig.DefaultWithHome(homePaths)
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName:                "api-core-test",
			MaskInternalErrors:           false,
			IncludeSessionWorkspaceInSSE: true,
			Sessions:                     testutil.StubSessionManager{},
			Observer: testutil.StubObserver{
				QueryBridgeHealthFn: func(context.Context) ([]observe.BridgeInstanceHealth, error) {
					return []observe.BridgeInstanceHealth{{
						BridgeInstanceID:      bridge.ID,
						Status:                bridgepkg.BridgeStatusAuthRequired,
						RouteCount:            0,
						DeliveryFailuresTotal: 2,
						AuthFailuresTotal:     1,
						LastError:             "temporary gateway timeout",
					}}, nil
				},
			},
			Bridges: testutil.StubBridgeService{
				ListInstancesFn: func(context.Context) ([]bridgepkg.BridgeInstance, error) {
					return []bridgepkg.BridgeInstance{bridge}, nil
				},
				GetInstanceFn: func(context.Context, string) (*bridgepkg.BridgeInstance, error) {
					return &bridge, nil
				},
				ListProvidersFn: func(context.Context) ([]bridgepkg.BridgeProvider, error) {
					return []bridgepkg.BridgeProvider{provider}, nil
				},
				ListSecretBindingsFn: func(context.Context, string) ([]bridgepkg.BridgeSecretBinding, error) {
					return nil, nil
				},
			},
			Workspaces: testutil.StubWorkspaceService{},
			HomePaths:  homePaths,
			Config:     cfg,
			Logger:     testutil.DiscardLogger(),
			StartedAt:  time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
			Now: func() time.Time {
				return time.Date(2026, 5, 19, 12, 0, 1, 0, time.UTC)
			},
			HTTPPort: cfg.HTTP.Port,
		})
		engine := gin.New()
		engine.GET("/bridges", handlers.ListBridges)
		engine.GET("/bridges/:id", handlers.GetBridge)

		listResp := performRequest(t, engine, http.MethodGet, "/bridges", nil)
		if listResp.Code != http.StatusOK {
			t.Fatalf("list status = %d body=%s", listResp.Code, listResp.Body.String())
		}
		var listPayload contract.BridgesResponse
		testutil.DecodeJSONResponse(t, listResp, &listPayload)
		assertBridgeDiagnosticKinds(t, listPayload.BridgeHealth[bridge.ID].Diagnostics)

		getResp := performRequest(t, engine, http.MethodGet, "/bridges/"+bridge.ID, nil)
		if getResp.Code != http.StatusOK {
			t.Fatalf("get status = %d body=%s", getResp.Code, getResp.Body.String())
		}
		var getPayload contract.BridgeResponse
		testutil.DecodeJSONResponse(t, getResp, &getPayload)
		assertBridgeDiagnosticKinds(t, getPayload.Health.Diagnostics)
	})

	t.Run("Should load the provider catalog once for bridge list diagnostics", func(t *testing.T) {
		t.Parallel()

		gin.SetMode(gin.TestMode)
		var listProvidersCalls atomic.Int32
		bridges := []bridgepkg.BridgeInstance{
			{
				ID:            "brg-1",
				Scope:         bridgepkg.ScopeGlobal,
				Platform:      "telegram",
				ExtensionName: "ext-telegram",
				DisplayName:   "Support 1",
				Enabled:       true,
				Status:        bridgepkg.BridgeStatusReady,
				RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			},
			{
				ID:            "brg-2",
				Scope:         bridgepkg.ScopeGlobal,
				Platform:      "telegram",
				ExtensionName: "ext-telegram",
				DisplayName:   "Support 2",
				Enabled:       true,
				Status:        bridgepkg.BridgeStatusReady,
				RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			},
		}
		provider := bridgepkg.BridgeProvider{
			Platform:      "telegram",
			ExtensionName: "ext-telegram",
			Enabled:       true,
			SecretSlots: []bridgepkg.BridgeSecretSlot{
				{Name: "bot_token", Required: true},
			},
		}
		homePaths := testutil.NewTestHomePaths(t)
		cfg := aghconfig.DefaultWithHome(homePaths)
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName:                "api-core-test",
			MaskInternalErrors:           false,
			IncludeSessionWorkspaceInSSE: true,
			Sessions:                     testutil.StubSessionManager{},
			Observer: testutil.StubObserver{
				QueryBridgeHealthFn: func(context.Context) ([]observe.BridgeInstanceHealth, error) {
					return []observe.BridgeInstanceHealth{
						{BridgeInstanceID: "brg-1", Status: bridgepkg.BridgeStatusReady},
						{BridgeInstanceID: "brg-2", Status: bridgepkg.BridgeStatusReady},
					}, nil
				},
			},
			Bridges: testutil.StubBridgeService{
				ListInstancesFn: func(context.Context) ([]bridgepkg.BridgeInstance, error) {
					return bridges, nil
				},
				ListProvidersFn: func(context.Context) ([]bridgepkg.BridgeProvider, error) {
					listProvidersCalls.Add(1)
					return []bridgepkg.BridgeProvider{provider}, nil
				},
				ListSecretBindingsFn: func(context.Context, string) ([]bridgepkg.BridgeSecretBinding, error) {
					return nil, nil
				},
			},
			Workspaces: testutil.StubWorkspaceService{},
			HomePaths:  homePaths,
			Config:     cfg,
			Logger:     testutil.DiscardLogger(),
			StartedAt:  time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
			Now: func() time.Time {
				return time.Date(2026, 5, 19, 12, 0, 1, 0, time.UTC)
			},
			HTTPPort: cfg.HTTP.Port,
		})
		engine := gin.New()
		engine.GET("/bridges", handlers.ListBridges)

		listResp := performRequest(t, engine, http.MethodGet, "/bridges", nil)
		if listResp.Code != http.StatusOK {
			t.Fatalf("list status = %d body=%s", listResp.Code, listResp.Body.String())
		}
		if got, want := listProvidersCalls.Load(), int32(1); got != want {
			t.Fatalf("ListProviders() calls = %d, want %d", got, want)
		}
	})

	t.Run("Should return base health when bridge list diagnostics enrichment fails", func(t *testing.T) {
		t.Parallel()

		gin.SetMode(gin.TestMode)
		bridge := bridgepkg.BridgeInstance{
			ID:            "brg-core",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "ext-telegram",
			DisplayName:   "Support",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusDegraded,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			Degradation: &bridgepkg.BridgeDegradation{
				Reason:  bridgepkg.BridgeDegradationReasonRateLimited,
				Message: "provider throttled",
			},
		}
		homePaths := testutil.NewTestHomePaths(t)
		cfg := aghconfig.DefaultWithHome(homePaths)
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			TransportName:                "api-core-test",
			MaskInternalErrors:           false,
			IncludeSessionWorkspaceInSSE: true,
			Sessions:                     testutil.StubSessionManager{},
			Observer: testutil.StubObserver{
				QueryBridgeHealthFn: func(context.Context) ([]observe.BridgeInstanceHealth, error) {
					return []observe.BridgeInstanceHealth{{
						BridgeInstanceID:      bridge.ID,
						Status:                bridgepkg.BridgeStatusDegraded,
						RouteCount:            2,
						DeliveryFailuresTotal: 3,
						LastError:             "adapter unavailable",
					}}, nil
				},
			},
			Bridges: testutil.StubBridgeService{
				ListInstancesFn: func(context.Context) ([]bridgepkg.BridgeInstance, error) {
					return []bridgepkg.BridgeInstance{bridge}, nil
				},
				ListProvidersFn: func(context.Context) ([]bridgepkg.BridgeProvider, error) {
					return nil, errors.New("provider catalog unavailable")
				},
			},
			Workspaces: testutil.StubWorkspaceService{},
			HomePaths:  homePaths,
			Config:     cfg,
			Logger:     testutil.DiscardLogger(),
			StartedAt:  time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
			Now: func() time.Time {
				return time.Date(2026, 5, 19, 12, 0, 1, 0, time.UTC)
			},
			HTTPPort: cfg.HTTP.Port,
		})
		engine := gin.New()
		engine.GET("/bridges", handlers.ListBridges)

		listResp := performRequest(t, engine, http.MethodGet, "/bridges", nil)
		if listResp.Code != http.StatusOK {
			t.Fatalf("list status = %d body=%s", listResp.Code, listResp.Body.String())
		}
		var payload contract.BridgesResponse
		testutil.DecodeJSONResponse(t, listResp, &payload)

		health := payload.BridgeHealth[bridge.ID]
		if got, want := health.BridgeInstanceID, bridge.ID; got != want {
			t.Fatalf("bridge_health instance_id = %q, want %q", got, want)
		}
		if got, want := health.Status, bridgepkg.BridgeStatusDegraded; got != want {
			t.Fatalf("bridge_health status = %q, want %q", got, want)
		}
		if got, want := health.RouteCount, 2; got != want {
			t.Fatalf("bridge_health route_count = %d, want %d", got, want)
		}
		if len(health.Diagnostics) != 0 {
			t.Fatalf("bridge_health diagnostics = %#v, want empty on enrichment failure", health.Diagnostics)
		}
		if health.Degradation == nil || health.Degradation.Reason != bridgepkg.BridgeDegradationReasonRateLimited {
			t.Fatalf("bridge_health degradation = %#v, want cloned instance degradation", health.Degradation)
		}
	})
}

func assertBridgeDiagnosticKinds(t *testing.T, diagnostics []bridgepkg.BridgeDiagnostic) {
	t.Helper()

	byKind := make(map[bridgepkg.BridgeDiagnosticKind]bridgepkg.BridgeDiagnostic, len(diagnostics))
	for _, diagnostic := range diagnostics {
		byKind[diagnostic.Kind] = diagnostic
	}
	for _, kind := range []bridgepkg.BridgeDiagnosticKind{
		bridgepkg.BridgeDiagnosticKindUnsupportedCapability,
		bridgepkg.BridgeDiagnosticKindMissingToken,
		bridgepkg.BridgeDiagnosticKindUnknownDestination,
		bridgepkg.BridgeDiagnosticKindPermissionDenied,
		bridgepkg.BridgeDiagnosticKindTransientDeliveryFailure,
	} {
		if _, ok := byKind[kind]; !ok {
			t.Fatalf("diagnostics missing kind %q: %#v", kind, diagnostics)
		}
	}
	if got := byKind[bridgepkg.BridgeDiagnosticKindMissingToken].SecretSlot; got != "bot_token" {
		t.Fatalf("missing token secret slot = %q, want bot_token", got)
	}
}

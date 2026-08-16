package core_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	apitestutil "github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

func TestBaseHandlersAgentNotify(t *testing.T) {
	t.Parallel()

	t.Run("Should bind the trusted caller identity and return the delivery outcome", func(t *testing.T) {
		t.Parallel()
		var got session.NotifyRequest
		manager := &notifyStubSessionManager{}
		manager.StatusFn = func(context.Context, string) (*session.Info, error) {
			return agentSpawnCallerInfo(), nil
		}
		manager.publish = func(_ context.Context, req session.NotifyRequest) (session.NotifyResult, error) {
			got = req
			return session.NotifyResult{Outcome: session.NotifyOutcomeDelivered}, nil
		}
		response := performAgentNotify(t, manager, []byte(`{"title":"Done","body":"No blockers"}`), true)
		if response.Code != http.StatusOK {
			t.Fatalf("status/body = %d/%s, want 200", response.Code, response.Body.String())
		}
		if got.SessionID != "sess-parent" || got.WorkspaceID != "ws-1" ||
			got.Title != "Done" || got.Body != "No blockers" {
			t.Fatalf("notify request = %#v, want trusted identity plus body", got)
		}
		var payload contract.AgentNotifyResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(response) error = %v", err)
		}
		if payload.Outcome != string(session.NotifyOutcomeDelivered) {
			t.Fatalf("outcome = %q, want delivered", payload.Outcome)
		}
	})

	t.Run("Should report subscriber-accounted delivered and no-client outcomes", func(t *testing.T) {
		t.Parallel()

		noClientRuntime := newNotifyRuntimeManager(t)
		noClientManager := notifyRouteManager(noClientRuntime)
		noClient := performAgentNotify(t, noClientManager, []byte(`{"title":"No client"}`), true)
		assertAgentNotifyOutcome(t, noClient, session.NotifyOutcomeNoClient)

		deliveredRuntime := newNotifyRuntimeManager(t)
		events, cancel, err := deliveredRuntime.SubscribeSessionCatalogEvents(context.Background())
		if err != nil {
			t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
		}
		defer cancel()
		deliveredManager := notifyRouteManager(deliveredRuntime)
		delivered := performAgentNotify(t, deliveredManager, []byte(`{"title":"Delivered"}`), true)
		assertAgentNotifyOutcome(t, delivered, session.NotifyOutcomeDelivered)
		rateLimited := performAgentNotify(t, deliveredManager, []byte(`{"title":"Again"}`), true)
		assertAgentNotifyOutcome(t, rateLimited, session.NotifyOutcomeRateLimited)
		var rateLimitedPayload contract.AgentNotifyResponse
		if err := json.Unmarshal(rateLimited.Body.Bytes(), &rateLimitedPayload); err != nil {
			t.Fatalf("json.Unmarshal(rate-limited response) error = %v", err)
		}
		if rateLimitedPayload.RetryAfterMS <= 0 {
			t.Fatalf("retry_after_ms = %d, want positive delay", rateLimitedPayload.RetryAfterMS)
		}
		event := <-events
		if event.Name != session.CatalogEventNameOperatorNotification || event.OperatorNotification == nil {
			t.Fatalf("catalog event = %#v, want operator notification", event)
		}
	})

	t.Run("Should reject notification cap violations as unprocessable", func(t *testing.T) {
		t.Parallel()
		manager := &notifyStubSessionManager{}
		manager.StatusFn = func(context.Context, string) (*session.Info, error) {
			return agentSpawnCallerInfo(), nil
		}
		manager.publish = func(context.Context, session.NotifyRequest) (session.NotifyResult, error) {
			return session.NotifyResult{}, session.ErrInvalidNotification
		}
		response := performAgentNotify(t, manager, []byte(`{"title":"too long"}`), true)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status/body = %d/%s, want 422", response.Code, response.Body.String())
		}
	})

	t.Run("Should require agent identity", func(t *testing.T) {
		t.Parallel()
		manager := &notifyStubSessionManager{}
		response := performAgentNotify(t, manager, []byte(`{"title":"Done"}`), false)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("status/body = %d/%s, want 401", response.Code, response.Body.String())
		}
	})
}

type notifyStubSessionManager struct {
	apitestutil.StubSessionManager
	publish func(context.Context, session.NotifyRequest) (session.NotifyResult, error)
}

func (s *notifyStubSessionManager) PublishOperatorNotification(
	ctx context.Context,
	req session.NotifyRequest,
) (session.NotifyResult, error) {
	return s.publish(ctx, req)
}

func performAgentNotify(
	t *testing.T,
	manager *notifyStubSessionManager,
	body []byte,
	withIdentity bool,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName: "uds-test", Sessions: manager, StreamDone: make(chan struct{}),
	})
	router.POST("/api/agent/notify", handlers.AgentNotify)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodPost, "/api/agent/notify", bytes.NewReader(body),
	)
	if withIdentity {
		req.Header.Set(agentidentity.HeaderSessionID, "sess-parent")
		req.Header.Set(agentidentity.HeaderAgent, "coder")
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

func newNotifyRuntimeManager(t *testing.T) *session.Manager {
	t.Helper()
	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	manager, err := session.NewManager(
		session.WithHomePaths(homePaths),
		session.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	)
	if err != nil {
		t.Fatalf("session.NewManager() error = %v", err)
	}
	return manager
}

func notifyRouteManager(runtime *session.Manager) *notifyStubSessionManager {
	manager := &notifyStubSessionManager{}
	manager.StatusFn = func(context.Context, string) (*session.Info, error) {
		return agentSpawnCallerInfo(), nil
	}
	manager.publish = runtime.PublishOperatorNotification
	return manager
}

func assertAgentNotifyOutcome(
	t *testing.T,
	response *httptest.ResponseRecorder,
	want session.NotifyOutcome,
) {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("status/body = %d/%s, want 200", response.Code, response.Body.String())
	}
	var payload contract.AgentNotifyResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(response) error = %v", err)
	}
	if payload.Outcome != string(want) {
		t.Fatalf("outcome = %q, want %q", payload.Outcome, want)
	}
}

//go:build integration

package bridgesdk

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

func TestRuntimeIntegrationBootsAndIngestsThroughHostAPI(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hostConn, runtimeConn := net.Pipe()
	defer hostConn.Close()
	defer runtimeConn.Close()

	runtime, err := NewRuntime(RuntimeConfig{
		ExtensionInfo: subprocess.InitializeExtensionInfo{
			Name:       "telegram-adapter",
			Version:    "1.0.0",
			SDKName:    "bridgesdk",
			SDKVersion: "test",
		},
		Check: testCheckHandler,
		Deliver: func(_ context.Context, session *Session, request bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error) {
			return session.AckDelivery(request, "remote-1", "")
		},
	})
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}

	hostPeer := NewPeer(hostConn, hostConn)
	var mu sync.Mutex
	var ingested bridgepkg.InboundMessageEnvelope
	if err := hostPeer.Handle("bridges/messages/ingest", func(_ context.Context, raw json.RawMessage) (any, error) {
		var envelope bridgepkg.InboundMessageEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, err
		}
		mu.Lock()
		ingested = envelope
		mu.Unlock()
		return bridgepkg.BridgesMessagesIngestResult{
			SessionID:    "sess-1",
			RouteCreated: true,
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-1",
				BridgeInstanceID: "brg-1",
				PeerID:           "peer-1",
			},
		}, nil
	}); err != nil {
		t.Fatalf("hostPeer.Handle(ingest) error = %v", err)
	}

	go func() {
		_ = runtime.Serve(ctx, runtimeConn, runtimeConn)
	}()
	go func() {
		_ = hostPeer.Serve(ctx)
	}()

	var response subprocess.InitializeResponse
	if err := hostPeer.Call(ctx, "initialize", testInitializeRequest(), &response); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	session := runtime.Session()
	if session == nil {
		t.Fatal("runtime.Session() = nil, want non-nil")
	}

	result, err := session.HostAPI().IngestBridgeMessage(ctx, testInboundEnvelope("idem-1", "msg-1", "hello"))
	if err != nil {
		t.Fatalf("HostAPI().IngestBridgeMessage() error = %v", err)
	}
	if got, want := result.SessionID, "sess-1"; got != want {
		t.Fatalf("result.SessionID = %q, want %q", got, want)
	}

	mu.Lock()
	defer mu.Unlock()
	if got, want := ingested.PlatformMessageID, "msg-1"; got != want {
		t.Fatalf("ingested.PlatformMessageID = %q, want %q", got, want)
	}
}

func TestRuntimeIntegrationRejectsInvalidIngressWithoutInvokingMapper(t *testing.T) {
	t.Parallel()

	called := 0
	handler, err := NewWebhookHandler(WebhookGuardConfig{
		AllowedMethods:      []string{http.MethodPost},
		AllowedContentTypes: []string{"application/json"},
		MaxBodyBytes:        8,
	}, func(_ http.ResponseWriter, _ *http.Request, _ WebhookRequest) error {
		called++
		return nil
	})
	if err != nil {
		t.Fatalf("NewWebhookHandler() error = %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	response, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("GET webhook error = %v", err)
	}
	response.Body.Close()
	if got, want := response.StatusCode, http.StatusMethodNotAllowed; got != want {
		t.Fatalf("GET status = %d, want %d", got, want)
	}

	response, err = http.Post(server.URL, "text/plain", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST invalid content type error = %v", err)
	}
	response.Body.Close()
	if got, want := response.StatusCode, http.StatusUnsupportedMediaType; got != want {
		t.Fatalf("POST invalid content type status = %d, want %d", got, want)
	}

	response, err = http.Post(server.URL, "application/json", strings.NewReader(`{"too_big":true}`))
	if err != nil {
		t.Fatalf("POST oversized error = %v", err)
	}
	response.Body.Close()
	if got, want := response.StatusCode, http.StatusRequestEntityTooLarge; got != want {
		t.Fatalf("POST oversized status = %d, want %d", got, want)
	}

	if got := called; got != 0 {
		t.Fatalf("provider mapping calls = %d, want 0", got)
	}
}

func TestRuntimeIntegrationReportsAuthAndRateLimitRecovery(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hostConn, runtimeConn := net.Pipe()
	defer hostConn.Close()
	defer runtimeConn.Close()

	runtime, err := NewRuntime(RuntimeConfig{
		ExtensionInfo: subprocess.InitializeExtensionInfo{
			Name:       "telegram-adapter",
			Version:    "1.0.0",
			SDKName:    "bridgesdk",
			SDKVersion: "test",
		},
		Check: testCheckHandler,
		Deliver: func(_ context.Context, session *Session, request bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error) {
			return session.AckDelivery(request, "remote-1", "")
		},
	})
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}

	hostPeer := NewPeer(hostConn, hostConn)
	var mu sync.Mutex
	var reports []bridgepkg.BridgesInstancesReportStateParams
	if err := hostPeer.Handle(
		"bridges/instances/report_state",
		func(_ context.Context, raw json.RawMessage) (any, error) {
			var params bridgepkg.BridgesInstancesReportStateParams
			if err := json.Unmarshal(raw, &params); err != nil {
				return nil, err
			}
			mu.Lock()
			reports = append(reports, params)
			mu.Unlock()

			instance := testBridgeInstance(params.BridgeInstanceID)
			instance.Status = params.Status
			instance.Degradation = params.Degradation
			return instance, nil
		},
	); err != nil {
		t.Fatalf("hostPeer.Handle(report_state) error = %v", err)
	}

	go func() {
		_ = runtime.Serve(ctx, runtimeConn, runtimeConn)
	}()
	go func() {
		_ = hostPeer.Serve(ctx)
	}()

	var response subprocess.InitializeResponse
	if err := hostPeer.Call(ctx, "initialize", testInitializeRequest(), &response); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	session := runtime.Session()
	if session == nil {
		t.Fatal("runtime.Session() = nil, want non-nil")
	}

	authUpdated, authRecovery, err := session.ReportClassifiedError(ctx, "brg-1", ClassifyError(&AuthError{
		Err: errors.New("invalid token"),
	}))
	if err != nil {
		t.Fatalf("ReportClassifiedError(auth) error = %v", err)
	}
	if authUpdated == nil || authUpdated.Status != bridgepkg.BridgeStatusAuthRequired {
		t.Fatalf("authUpdated.Status = %#v, want auth_required", authUpdated)
	}
	if authRecovery.Retry {
		t.Fatal("authRecovery.Retry = true, want false")
	}

	rateUpdated, rateRecovery, err := session.ReportClassifiedError(ctx, "brg-1", ClassifyError(&RateLimitError{
		Err:        errors.New("too many requests"),
		RetryAfter: time.Second,
	}))
	if err != nil {
		t.Fatalf("ReportClassifiedError(rate_limit) error = %v", err)
	}
	if rateUpdated == nil || rateUpdated.Status != bridgepkg.BridgeStatusDegraded {
		t.Fatalf("rateUpdated.Status = %#v, want degraded", rateUpdated)
	}
	if !rateRecovery.Retry {
		t.Fatal("rateRecovery.Retry = false, want true")
	}
	if got, want := rateRecovery.RetryAfter, time.Second; got != want {
		t.Fatalf("rateRecovery.RetryAfter = %s, want %s", got, want)
	}

	mu.Lock()
	defer mu.Unlock()
	if got, want := len(reports), 2; got != want {
		t.Fatalf("len(reports) = %d, want %d", got, want)
	}
	if got, want := reports[0].Status, bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("reports[0].Status = %q, want %q", got, want)
	}
	if reports[0].Degradation == nil || reports[0].Degradation.Reason != bridgepkg.BridgeDegradationReasonAuthFailed {
		t.Fatalf("reports[0].Degradation = %#v, want auth_failed", reports[0].Degradation)
	}
	if got, want := reports[1].Status, bridgepkg.BridgeStatusDegraded; got != want {
		t.Fatalf("reports[1].Status = %q, want %q", got, want)
	}
	if reports[1].Degradation == nil || reports[1].Degradation.Reason != bridgepkg.BridgeDegradationReasonRateLimited {
		t.Fatalf("reports[1].Degradation = %#v, want rate_limited", reports[1].Degradation)
	}
}

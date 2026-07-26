//go:build integration

package httpapi

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/testutil"
)

type blockingHTTPDeliveryTransport struct {
	releaseCh chan struct{}
}

func (t *blockingHTTPDeliveryTransport) DeliverBridge(
	ctx context.Context,
	_ string,
	req bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	if t != nil && req.Event.EventType == bridgepkg.DeliveryEventTypeStart {
		select {
		case <-t.releaseCh:
		case <-ctx.Done():
			return bridgepkg.DeliveryAck{}, ctx.Err()
		}
	}
	return bridgepkg.DeliveryAck{
		DeliveryID: req.Event.DeliveryID,
		Seq:        req.Event.Seq,
	}, nil
}

func TestHTTPBridgeCreateReturnsPersistedPayload(t *testing.T) {
	t.Parallel()

	runtime := newIntegrationRuntime(t)

	resp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/bridges"),
		[]byte(
			`{"scope":"global","platform":"telegram","extension_name":"ext-telegram","display_name":"Support","enabled":true,"dm_policy":"pairing","routing_policy":{"include_peer":true},"provider_config":{"mode":"bot","tenant":"acme"},"delivery_defaults":{"peer_id":"peer-default","mode":"reply","parse_mode":"MarkdownV2","progress":{"tool_progress":"all","grouping":"accumulate","typing":true,"reactions":false}}}`,
		),
		nil,
	)
	if resp.StatusCode != http.StatusCreated {
		body := mustReadAll(t, resp.Body)
		t.Fatalf("create bridge status = %d, want %d; body=%s", resp.StatusCode, http.StatusCreated, body)
	}

	var payload contract.BridgeResponse
	decodeHTTPJSON(t, resp, &payload)
	if payload.Bridge.ID == "" || payload.Bridge.Platform != "telegram" ||
		payload.Bridge.ExtensionName != "ext-telegram" {
		t.Fatalf("payload.Bridge = %#v", payload.Bridge)
	}
	if payload.Bridge.DMPolicy != bridgepkg.BridgeDMPolicyPairing {
		t.Fatalf("payload.Bridge.DMPolicy = %q, want %q", payload.Bridge.DMPolicy, bridgepkg.BridgeDMPolicyPairing)
	}
	if got, want := string(payload.Bridge.ProviderConfig), `{"mode":"bot","tenant":"acme"}`; got != want {
		t.Fatalf("payload.Bridge.ProviderConfig = %s, want %s", got, want)
	}
	if got, want := string(payload.Bridge.DeliveryDefaults), `{"mode":"reply","parse_mode":"MarkdownV2","peer_id":"peer-default","progress":{"tool_progress":"all","grouping":"accumulate","typing":true,"reactions":false}}`; got != want {
		t.Fatalf("payload.Bridge.DeliveryDefaults = %s, want %s", got, want)
	}

	stored, err := runtime.registry.GetBridgeInstance(context.Background(), payload.Bridge.ID)
	if err != nil {
		t.Fatalf("runtime.registry.GetBridgeInstance() error = %v", err)
	}
	if stored.DisplayName != "Support" || stored.Status != bridgepkg.BridgeStatusStarting {
		t.Fatalf("stored instance = %#v", stored)
	}
	if got, want := string(stored.ProviderConfig), `{"mode":"bot","tenant":"acme"}`; got != want {
		t.Fatalf("stored.ProviderConfig = %s, want %s", got, want)
	}
	if got, want := string(stored.DeliveryDefaults), `{"mode":"reply","parse_mode":"MarkdownV2","peer_id":"peer-default","progress":{"tool_progress":"all","grouping":"accumulate","typing":true,"reactions":false}}`; got != want {
		t.Fatalf("stored.DeliveryDefaults = %s, want %s", got, want)
	}

	detail := getHTTPBridge(t, runtime, payload.Bridge.ID)
	if got, want := string(detail.Bridge.ProviderConfig), `{"mode":"bot","tenant":"acme"}`; got != want {
		t.Fatalf("detail.Bridge.ProviderConfig = %s, want %s", got, want)
	}
	if got, want := string(detail.Bridge.DeliveryDefaults), `{"mode":"reply","parse_mode":"MarkdownV2","peer_id":"peer-default","progress":{"tool_progress":"all","grouping":"accumulate","typing":true,"reactions":false}}`; got != want {
		t.Fatalf("detail.Bridge.DeliveryDefaults = %s, want %s", got, want)
	}
}

func TestHTTPBridgeProvidersExposeOperatorMetadata(t *testing.T) {
	t.Parallel()

	runtime := newIntegrationRuntime(t)
	runtime.bridges.providers = []bridgepkg.BridgeProvider{{
		Platform:      "telegram",
		ExtensionName: "telegram-reference",
		DisplayName:   "Telegram",
		Description:   "Reference Telegram bridge adapter",
		SecretSlots: []bridgepkg.BridgeSecretSlot{{
			Name:        "bot_token",
			Description: "Bot token",
			Required:    true,
		}},
		ConfigSchema: &bridgepkg.BridgeProviderConfigSchema{
			Schema:  "agh.bridge.telegram",
			Version: "v1",
		},
		Enabled:       true,
		State:         "active",
		Health:        "healthy",
		HealthMessage: "connected",
	}}

	resp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/bridges/providers"),
		nil,
		nil,
	)
	if resp.StatusCode != http.StatusOK {
		body := mustReadAll(t, resp.Body)
		t.Fatalf("provider list status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, body)
	}

	var payload contract.BridgeProvidersResponse
	decodeHTTPJSON(t, resp, &payload)
	if got, want := len(payload.Providers), 1; got != want {
		t.Fatalf("len(providers) = %d, want %d", got, want)
	}
	if len(payload.Providers[0].SecretSlots) != 1 || payload.Providers[0].SecretSlots[0].Name != "bot_token" {
		t.Fatalf("providers[0].SecretSlots = %#v", payload.Providers[0].SecretSlots)
	}
	if payload.Providers[0].ConfigSchema == nil || payload.Providers[0].ConfigSchema.Schema != "agh.bridge.telegram" {
		t.Fatalf("providers[0].ConfigSchema = %#v", payload.Providers[0].ConfigSchema)
	}
	if payload.Providers[0].HealthMessage != "connected" {
		t.Fatalf("providers[0].HealthMessage = %q, want %q", payload.Providers[0].HealthMessage, "connected")
	}
}

func TestHTTPBridgeRoutesEndpointReturnsOnlyRequestedInstanceRoutes(t *testing.T) {
	t.Parallel()

	runtime := newIntegrationRuntime(t)

	first := createIntegrationBridge(t, runtime, bridgepkg.CreateInstanceRequest{
		ID:            "brg-http-a",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "ext-telegram",
		DisplayName:   "A",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	second := createIntegrationBridge(t, runtime, bridgepkg.CreateInstanceRequest{
		ID:            "brg-http-b",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "ext-telegram",
		DisplayName:   "B",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})

	upsertIntegrationBridgeRoute(t, runtime, bridgepkg.BridgeRoute{
		BridgeInstanceID: first.ID,
		Scope:            first.Scope,
		PeerID:           "peer-a",
		SessionID:        "sess-a",
		AgentName:        "coder",
		LastActivityAt:   time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC),
	})
	upsertIntegrationBridgeRoute(t, runtime, bridgepkg.BridgeRoute{
		BridgeInstanceID: second.ID,
		Scope:            second.Scope,
		PeerID:           "peer-b",
		SessionID:        "sess-b",
		AgentName:        "coder",
		LastActivityAt:   time.Date(2026, 4, 11, 12, 1, 0, 0, time.UTC),
	})

	resp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/bridges/"+first.ID+"/routes"),
		nil,
		nil,
	)
	if resp.StatusCode != http.StatusOK {
		body := mustReadAll(t, resp.Body)
		t.Fatalf("routes status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, body)
	}

	var payload contract.BridgeRoutesResponse
	decodeHTTPJSON(t, resp, &payload)
	if got, want := len(payload.Routes), 1; got != want {
		t.Fatalf("len(routes) = %d, want %d", got, want)
	}
	if payload.Routes[0].BridgeInstanceID != first.ID || payload.Routes[0].PeerID != "peer-a" {
		t.Fatalf("routes = %#v", payload.Routes)
	}
}

func TestHTTPBridgeTestDeliveryResolvesTargetWithoutLiveAdapter(t *testing.T) {
	t.Parallel()

	runtime := newIntegrationRuntime(t)

	instance := createIntegrationBridge(t, runtime, bridgepkg.CreateInstanceRequest{
		ID:               "brg-http-test-delivery",
		Scope:            bridgepkg.ScopeGlobal,
		Platform:         "telegram",
		ExtensionName:    "ext-telegram",
		DisplayName:      "Test Delivery",
		Enabled:          true,
		Status:           bridgepkg.BridgeStatusReady,
		RoutingPolicy:    bridgepkg.RoutingPolicy{IncludePeer: true, IncludeThread: true},
		DeliveryDefaults: []byte(`{"peer_id":"peer-default","mode":"reply"}`),
	})

	resp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/bridges/"+instance.ID+"/test-delivery"),
		[]byte(`{"message":"hello","target":{"thread_id":"thread-1"}}`),
		nil,
	)
	if resp.StatusCode != http.StatusOK {
		body := mustReadAll(t, resp.Body)
		t.Fatalf("test delivery status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, body)
	}

	var payload contract.BridgeTestDeliveryResponse
	decodeHTTPJSON(t, resp, &payload)
	if payload.Status != "resolved" || payload.DeliveryTarget.BridgeInstanceID != instance.ID {
		t.Fatalf("payload = %#v", payload)
	}
	if payload.DeliveryTarget.PeerID != "peer-default" || payload.DeliveryTarget.ThreadID != "thread-1" ||
		payload.DeliveryTarget.Mode != bridgepkg.DeliveryModeReply {
		t.Fatalf("delivery target = %#v", payload.DeliveryTarget)
	}
}

func TestHTTPObserveHealthIncludesBridgeMetricsAndPreservesSessionFields(t *testing.T) {
	t.Parallel()

	runtime := newIntegrationRuntime(t)

	createSessionResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/sessions"),
		[]byte(`{"agent_name":"coder","workspace_path":"`+runtime.workspace+`"}`),
		nil,
	)
	if createSessionResp.StatusCode != http.StatusCreated {
		body := mustReadAll(t, createSessionResp.Body)
		t.Fatalf("create session status = %d, want %d; body=%s", createSessionResp.StatusCode, http.StatusCreated, body)
	}

	instance := createIntegrationBridge(t, runtime, bridgepkg.CreateInstanceRequest{
		ID:            "brg-http-health",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "ext-telegram",
		DisplayName:   "Health",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	upsertIntegrationBridgeRoute(t, runtime, bridgepkg.BridgeRoute{
		BridgeInstanceID: instance.ID,
		Scope:            instance.Scope,
		PeerID:           "peer-health",
		SessionID:        "sess-health",
		AgentName:        "coder",
		LastActivityAt:   time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC),
	})
	runtime.observer.RecordBridgeAuthFailure(instance.ID)

	resp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/status"),
		nil,
		nil,
	)
	if resp.StatusCode != http.StatusOK {
		body := mustReadAll(t, resp.Body)
		t.Fatalf("health status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, body)
	}

	var payload contract.StatusPayload
	decodeHTTPJSON(t, resp, &payload)
	if got, want := payload.Health.Status, "ok"; got != want {
		t.Fatalf("health.status = %q, want %q", got, want)
	}
	if got, want := payload.Health.ActiveSessions, 1; got != want {
		t.Fatalf("health.active_sessions = %d, want %d", got, want)
	}
	if got, want := payload.Health.Bridges.TotalInstances, 1; got != want {
		t.Fatalf("health.bridges.total_instances = %d, want %d", got, want)
	}
	if got, want := payload.Health.Bridges.RouteCount, 1; got != want {
		t.Fatalf("health.bridges.route_count = %d, want %d", got, want)
	}
	if got, want := payload.Health.Bridges.StatusCounts.Ready, 1; got != want {
		t.Fatalf("health.bridges.status_counts.ready = %d, want %d", got, want)
	}
	if got, want := payload.Health.Bridges.AuthFailuresTotal, 1; got != want {
		t.Fatalf("health.bridges.auth_failures_total = %d, want %d", got, want)
	}
}

func TestHTTPBridgeDetailShowsAuthRequiredStatusAndHealth(t *testing.T) {
	t.Parallel()

	runtime := newIntegrationRuntime(t)

	instance := createIntegrationBridge(t, runtime, bridgepkg.CreateInstanceRequest{
		ID:            "brg-http-auth",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "ext-telegram",
		DisplayName:   "Auth",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	if _, err := runtime.bridges.UpdateInstanceState(testutil.Context(t), bridgepkg.UpdateInstanceStateRequest{
		ID:      instance.ID,
		Enabled: true,
		Status:  bridgepkg.BridgeStatusAuthRequired,
	}); err != nil {
		t.Fatalf("runtime.bridges.UpdateInstanceState() error = %v", err)
	}
	runtime.observer.RecordBridgeAuthFailure(instance.ID)

	resp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/bridges/"+instance.ID),
		nil,
		nil,
	)
	if resp.StatusCode != http.StatusOK {
		body := mustReadAll(t, resp.Body)
		t.Fatalf("get bridge status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, body)
	}

	var payload contract.BridgeResponse
	decodeHTTPJSON(t, resp, &payload)
	if got, want := payload.Bridge.Status, bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("bridge.status = %q, want %q", got, want)
	}
	if got, want := payload.Health.Status, bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("health.status = %q, want %q", got, want)
	}
	if got, want := payload.Health.AuthFailuresTotal, 1; got != want {
		t.Fatalf("health.auth_failures_total = %d, want %d", got, want)
	}
}

func TestHTTPBridgeDetailReportsBacklogAndClearsAfterDeliveryCompletes(t *testing.T) {
	t.Parallel()

	runtime := newIntegrationRuntime(t)

	instance := createIntegrationBridge(t, runtime, bridgepkg.CreateInstanceRequest{
		ID:            "brg-http-backlog",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "ext-telegram",
		DisplayName:   "Backlog",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})

	transport := &blockingHTTPDeliveryTransport{releaseCh: make(chan struct{})}
	runtime.bridges.Broker().SetTransport(transport)
	registration := registerIntegrationDelivery(
		t,
		runtime,
		instance,
		"sess-http-backlog",
		"turn-http-backlog",
		"peer-http-backlog",
	)
	if err := runtime.bridges.Broker().
		Deliver(testutil.Context(t), integrationDeliveryEvent(registration, 1, bridgepkg.DeliveryEventTypeStart, "hello", false)); err != nil {
		t.Fatalf("Broker().Deliver(start) error = %v", err)
	}
	if err := runtime.bridges.Broker().
		Deliver(testutil.Context(t), integrationDeliveryEvent(registration, 2, bridgepkg.DeliveryEventTypeDelta, "hello again", false)); err != nil {
		t.Fatalf("Broker().Deliver(delta) error = %v", err)
	}

	waitForHTTPCondition(t, func() bool {
		bridge := getHTTPBridge(t, runtime, instance.ID)
		return bridge.Health.DeliveryBacklog == 1
	})

	bridge := getHTTPBridge(t, runtime, instance.ID)
	if got, want := bridge.Health.DeliveryBacklog, 1; got != want {
		t.Fatalf("bridge.health.delivery_backlog = %d, want %d", got, want)
	}
	health := getHTTPHealth(t, runtime)
	if got, want := health.Health.Bridges.DeliveryBacklog, 1; got != want {
		t.Fatalf("health.bridges.delivery_backlog = %d, want %d", got, want)
	}

	close(transport.releaseCh)
	waitForHTTPCondition(t, func() bool {
		return getHTTPBridge(t, runtime, instance.ID).Health.DeliveryBacklog == 0
	})

	bridge = getHTTPBridge(t, runtime, instance.ID)
	if bridge.Health.DeliveryBacklog != 0 {
		t.Fatalf("bridge.health.delivery_backlog = %d, want 0", bridge.Health.DeliveryBacklog)
	}
	if bridge.Health.LastSuccessAt == nil {
		t.Fatal("bridge.health.last_success_at = nil, want successful delivery timestamp")
	}
}

func createIntegrationBridge(
	t *testing.T,
	runtime integrationRuntime,
	req bridgepkg.CreateInstanceRequest,
) *bridgepkg.BridgeInstance {
	t.Helper()

	instance, err := runtime.bridges.CreateInstance(testutil.Context(t), req)
	if err != nil {
		t.Fatalf("runtime.bridges.CreateInstance() error = %v", err)
	}
	return instance
}

func upsertIntegrationBridgeRoute(t *testing.T, runtime integrationRuntime, route bridgepkg.BridgeRoute) {
	t.Helper()

	if _, err := runtime.bridges.UpsertRoute(testutil.Context(t), route); err != nil {
		t.Fatalf("runtime.bridges.UpsertRoute() error = %v", err)
	}
}

func registerIntegrationDelivery(
	t *testing.T,
	runtime integrationRuntime,
	instance *bridgepkg.BridgeInstance,
	sessionID string,
	turnID string,
	peerID string,
) bridgepkg.DeliverySnapshot {
	t.Helper()

	snapshot, err := runtime.bridges.Broker().
		RegisterPromptDelivery(testutil.Context(t), bridgepkg.PromptDeliveryRegistration{
			SessionID:     sessionID,
			TurnID:        turnID,
			ExtensionName: instance.ExtensionName,
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            instance.Scope,
				WorkspaceID:      instance.WorkspaceID,
				BridgeInstanceID: instance.ID,
				PeerID:           peerID,
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: instance.ID,
				PeerID:           peerID,
				Mode:             bridgepkg.DeliveryModeReply,
			},
		})
	if err != nil {
		t.Fatalf("RegisterPromptDelivery(%s) error = %v", instance.ID, err)
	}
	return *snapshot
}

func integrationDeliveryEvent(
	snapshot bridgepkg.DeliverySnapshot,
	seq int64,
	eventType bridgepkg.DeliveryEventType,
	text string,
	final bool,
) bridgepkg.DeliveryEvent {
	return bridgepkg.DeliveryEvent{
		DeliveryID:       snapshot.DeliveryID,
		BridgeInstanceID: snapshot.BridgeInstanceID,
		RoutingKey:       snapshot.RoutingKey,
		DeliveryTarget:   snapshot.DeliveryTarget,
		Seq:              seq,
		EventType:        eventType,
		Content:          bridgepkg.MessageContent{Text: text},
		Final:            final,
	}
}

func getHTTPBridge(t *testing.T, runtime integrationRuntime, bridgeID string) contract.BridgeResponse {
	t.Helper()

	resp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/bridges/"+bridgeID),
		nil,
		nil,
	)
	if resp.StatusCode != http.StatusOK {
		body := mustReadAll(t, resp.Body)
		t.Fatalf("get bridge status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, body)
	}
	var payload contract.BridgeResponse
	decodeHTTPJSON(t, resp, &payload)
	return payload
}

func getHTTPHealth(t *testing.T, runtime integrationRuntime) contract.StatusPayload {
	t.Helper()

	resp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/status"),
		nil,
		nil,
	)
	if resp.StatusCode != http.StatusOK {
		body := mustReadAll(t, resp.Body)
		t.Fatalf("get health status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, body)
	}
	var payload contract.StatusPayload
	decodeHTTPJSON(t, resp, &payload)
	return payload
}

func waitForHTTPCondition(t *testing.T, fn func() bool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(testutil.Context(t), 2*time.Second)
	defer cancel()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if fn() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("condition did not become true before timeout")
		case <-ticker.C:
		}
	}
}

func mustReadAll(t *testing.T, body io.ReadCloser) string {
	t.Helper()

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v", err)
	}
	if err := body.Close(); err != nil {
		t.Fatalf("body.Close() error = %v", err)
	}
	return string(data)
}

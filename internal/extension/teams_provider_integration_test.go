//go:build integration

package extensionpkg_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/compozy/agh/internal/acp"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	extensiontest "github.com/compozy/agh/internal/extensiontest"
	observepkg "github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/subprocess"
)

const (
	teamsProviderListenAddrEnv     = "AGH_BRIDGE_TEAMS_LISTEN_ADDR"
	teamsProviderServiceURLEnv     = "AGH_BRIDGE_TEAMS_SERVICE_URL"
	teamsProviderOpenIDMetadataEnv = "AGH_BRIDGE_TEAMS_OPENID_METADATA_URL"
	teamsProviderTokenURLEnv       = "AGH_BRIDGE_TEAMS_TOKEN_URL"
	teamsProviderLoopbackAuthEnv   = "AGH_BRIDGE_TEAMS_ALLOW_LOOPBACK_AUTH_FOR_TESTING"
)

var (
	buildTeamsProviderOnce sync.Once
	buildTeamsProviderErr  error
)

func TestTeamsProviderLaunchNegotiatesBridgeRuntime(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildTeamsProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newTeamsProviderAPIServer(t)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: teamsProviderExtensionDir(repoRoot),
		Platform:     "teams",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{
			teamsManagedInstanceConfig(
				"brg-teams-a",
				"11111111-2222-3333-4444-555555555555",
				bridgepkg.RoutingPolicy{IncludeGroup: true, IncludeThread: true},
			),
			teamsManagedInstanceConfig(
				"brg-teams-b",
				"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				bridgepkg.RoutingPolicy{IncludePeer: true, IncludeThread: true},
			),
		},
		ExtraEnv: map[string]string{
			teamsProviderListenAddrEnv:     listenAddr,
			teamsProviderServiceURLEnv:     mockAPI.ServiceURL(),
			teamsProviderOpenIDMetadataEnv: mockAPI.MetadataURL(),
			teamsProviderTokenURLEnv:       mockAPI.TokenURL(),
			teamsProviderLoopbackAuthEnv:   "1",
		},
		StartTime: time.Date(2026, 4, 15, 19, 0, 0, 0, time.UTC),
	})

	harness.WaitForHandshake(t, 10*time.Second)
	expectedInstanceIDs := []string{"brg-teams-a", "brg-teams-b"}
	states := harness.WaitForStates(t, 10*time.Second, func(states []extensiontest.StateRecord) bool {
		return teamsProviderStatesReady(states, expectedInstanceIDs...)
	})
	for _, instanceID := range expectedInstanceIDs {
		instanceID := instanceID
		t.Run("ShouldReportReadyStateFor_"+instanceID, func(t *testing.T) {
			t.Parallel()
			state, ok := teamsProviderLastStateForInstance(states, instanceID)
			if !ok {
				t.Fatalf("adapter state for %q missing after wait: %#v", instanceID, states)
			}
			if got, want := state.Status.Normalize(), bridgepkg.BridgeStatusReady; got != want {
				t.Fatalf(
					"adapter state for %q = %q (error=%q, degradation=%#v), want %q",
					instanceID,
					got,
					state.Error,
					state.Instance.Degradation,
					want,
				)
			}
		})
	}

	report := harness.Report(t)
	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "teams",
		Platform:                  "teams",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{
			{
				InstanceID:          "brg-teams-a",
				ExtensionName:       "teams",
				BoundSecretNames:    []string{"app_id", "app_password", "app_tenant_id"},
				ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
			},
			{
				InstanceID:          "brg-teams-b",
				ExtensionName:       "teams",
				BoundSecretNames:    []string{"app_id", "app_password", "app_tenant_id"},
				ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
			},
		},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	if report.Ownership == nil {
		t.Fatal("ownership marker = nil, want provider ownership evidence")
	}
	if got, want := len(report.Ownership.Fetched), 2; got != want {
		t.Fatalf("len(report.Ownership.Fetched) = %d, want %d", got, want)
	}

	health := harness.ObserveHealth(t)
	if got, want := health.Bridges.StatusCounts.Ready, 2; got != want {
		t.Fatalf("observe.Health().Bridges.StatusCounts.Ready = %d, want %d", got, want)
	}
}

func TestTeamsProviderIngressAndDeliveryConformance(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildTeamsProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newTeamsProviderAPIServer(t)
	startTime := time.Date(2026, 4, 15, 19, 5, 0, 0, time.UTC)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: teamsProviderExtensionDir(repoRoot),
		Platform:     "teams",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{teamsManagedInstanceConfig(
			"brg-teams",
			"11111111-2222-3333-4444-555555555555",
			bridgepkg.RoutingPolicy{IncludeGroup: true, IncludeThread: true},
		)},
		Driver: extensiontest.NewScriptedPromptDriver(startTime, []extensiontest.ScriptedPromptEvent{
			{Type: acp.EventTypeAgentMessage, Text: "hello"},
			{Type: acp.EventTypeAgentMessage, Text: " world"},
			{Type: acp.EventTypeDone},
		}),
		ExtraEnv: map[string]string{
			teamsProviderListenAddrEnv:     listenAddr,
			teamsProviderServiceURLEnv:     mockAPI.ServiceURL(),
			teamsProviderOpenIDMetadataEnv: mockAPI.MetadataURL(),
			teamsProviderTokenURLEnv:       mockAPI.TokenURL(),
			teamsProviderLoopbackAuthEnv:   "1",
		},
		StartTime: startTime,
	})

	harness.WaitForHandshake(t, 10*time.Second)
	states := harness.WaitForStates(t, 10*time.Second, func(states []extensiontest.StateRecord) bool {
		return len(states) > 0
	})
	if got, want := states[len(states)-1].Status.Normalize(), bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf(
			"last adapter state = %q (error=%q, degradation=%#v), want %q",
			got,
			states[len(states)-1].Error,
			states[len(states)-1].Instance.Degradation,
			want,
		)
	}

	webhookURL := fmt.Sprintf("http://%s/teams/%s", listenAddr, harness.Instances[0].ID)
	postTeamsProviderWebhook(
		t,
		mockAPI,
		webhookURL,
		"app-id",
		teamsProviderMessageWebhook(mockAPI.ServiceURL(), "Need a summary"),
	)
	postTeamsProviderWebhook(t, mockAPI, webhookURL, "app-id", teamsProviderInvokeWebhook(mockAPI.ServiceURL()))
	postTeamsProviderWebhook(t, mockAPI, webhookURL, "app-id", teamsProviderReactionWebhook(mockAPI.ServiceURL()))

	ingests := harness.WaitForIngests(t, 10*time.Second, func(records []extensiontest.IngestRecord) bool {
		return len(records) >= 4 && strings.TrimSpace(records[len(records)-1].Result.SessionID) != ""
	})
	deliveries := harness.WaitForDeliveries(t, 10*time.Second, func(records []extensiontest.DeliveryRecord) bool {
		return len(records) > 0 &&
			normalizeDeliveryEventType(
				records[len(records)-1].Request.Event.EventType,
			) == bridgepkg.DeliveryEventTypeFinal
	})
	report := harness.Report(t)

	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "teams",
		Platform:                  "teams",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		RequireDelivery:           true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "teams",
			BoundSecretNames:    []string{"app_id", "app_password", "app_tenant_id"},
			ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	message := findIngestByFamily(t, ingests, bridgepkg.InboundEventFamilyMessage)
	if got, want := message.Envelope.GroupID, "19:channel@thread.tacv2"; got != want {
		t.Fatalf("message group id = %q, want %q", got, want)
	}
	if got, want := message.Envelope.ThreadID, teamsProviderExpectedThreadID(
		"19:channel@thread.tacv2;messageid=activity-1",
		mockAPI.ServiceURL(),
	); got != want {
		t.Fatalf("message thread id = %q, want %q", got, want)
	}
	if got, want := message.Envelope.Content.Text, "Need a summary"; got != want {
		t.Fatalf("message text = %q, want %q", got, want)
	}

	action := findIngestByFamily(t, ingests, bridgepkg.InboundEventFamilyAction)
	if action.Envelope.Action == nil {
		t.Fatal("action envelope missing action payload")
	}
	if got, want := action.Envelope.Action.ActionID, "approve"; got != want {
		t.Fatalf("action.ActionID = %q, want %q", got, want)
	}

	reaction := findIngestByFamily(t, ingests, bridgepkg.InboundEventFamilyReaction)
	if reaction.Envelope.Reaction == nil {
		t.Fatal("reaction envelope missing reaction payload")
	}
	if got, want := reaction.Envelope.Reaction.Emoji, "like"; got != want {
		t.Fatalf("reaction.Emoji = %q, want %q", got, want)
	}
	if !reaction.Envelope.Reaction.Added {
		t.Fatal("reaction.Added = false, want true for the first reaction ingest")
	}

	if len(deliveries) < 2 {
		t.Fatalf("len(deliveries) = %d, want at least 2", len(deliveries))
	}
	if got, want := normalizeDeliveryEventType(
		deliveries[0].Request.Event.EventType,
	), bridgepkg.DeliveryEventTypeStart; got != want {
		t.Fatalf("first delivery event type = %q, want %q", got, want)
	}
	if got, want := normalizeDeliveryEventType(
		deliveries[len(deliveries)-1].Request.Event.EventType,
	), bridgepkg.DeliveryEventTypeFinal; got != want {
		t.Fatalf("last delivery event type = %q, want %q", got, want)
	}

	calls := mockAPI.Calls()
	if !teamsProviderCallsContainPath(calls, http.MethodPost, "/oauth2/v2.0/token") {
		t.Fatalf("mock api calls = %#v, want %s %s", calls, http.MethodPost, "/oauth2/v2.0/token")
	}
	if !teamsProviderCallsContainPathPrefix(calls, http.MethodPost, "/v3/conversations/") {
		t.Fatalf("mock api calls = %#v, want outbound POST activity", calls)
	}
	if !teamsProviderCallsContainPathPrefix(calls, http.MethodPut, "/v3/conversations/") {
		t.Fatalf("mock api calls = %#v, want outbound PUT activity update", calls)
	}

	row := waitForBridgeHealth(t, 10*time.Second, harness, func(health observepkg.BridgeInstanceHealth) bool {
		return health.Status.Normalize() == bridgepkg.BridgeStatusReady && health.RouteCount >= 1
	})
	if row.RouteCount < 1 {
		t.Fatalf("bridge health route_count = %d, want at least 1 after ingress", row.RouteCount)
	}
}

func TestTeamsProviderInvalidTenantConfigReportsDegradedState(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildTeamsProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)

	instanceConfig := teamsManagedInstanceConfig(
		"brg-teams-bad",
		"not-a-tenant",
		bridgepkg.RoutingPolicy{IncludePeer: true},
	)
	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir:     teamsProviderExtensionDir(repoRoot),
		Platform:         "teams",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{instanceConfig},
		ExtraEnv: map[string]string{
			teamsProviderListenAddrEnv: listenAddr,
		},
		StartTime: time.Date(2026, 4, 15, 19, 10, 0, 0, time.UTC),
	})

	harness.WaitForHandshake(t, 10*time.Second)
	states := harness.WaitForStates(t, 10*time.Second, func(states []extensiontest.StateRecord) bool {
		return len(states) > 0
	})
	last := states[len(states)-1]
	if got, want := last.Status.Normalize(), bridgepkg.BridgeStatusDegraded; got != want {
		t.Fatalf("last adapter state = %q (error=%q), want %q", got, last.Error, want)
	}

	report := harness.Report(t)
	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "teams",
		Platform:                  "teams",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "teams",
			BoundSecretNames:    []string{"app_id", "app_password", "app_tenant_id"},
			ExpectedFinalStatus: bridgepkg.BridgeStatusDegraded,
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	var instance *bridgepkg.BridgeInstance
	waitForCondition(t, 10*time.Second, "bridge instance degraded after invalid tenant config", func() bool {
		loaded, err := harness.Bridges.GetInstance(context.Background(), harness.Instances[0].ID)
		if err != nil {
			return false
		}
		instance = loaded
		return loaded.Status.Normalize() == bridgepkg.BridgeStatusDegraded &&
			loaded.Degradation != nil &&
			loaded.Degradation.Reason == bridgepkg.BridgeDegradationReasonTenantConfigInvalid
	})
	if instance == nil {
		t.Fatal("degraded bridge instance = nil, want persisted degraded state")
	}
}

func teamsProviderExtensionDir(repoRoot string) string {
	return filepath.Join(repoRoot, "extensions", "bridges", "teams")
}

func buildTeamsProvider(t *testing.T, repoRoot string) {
	t.Helper()

	buildTeamsProviderOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(
			ctx,
			"go",
			"build",
			"-o",
			"./extensions/bridges/teams/bin/teams",
			"./extensions/bridges/teams",
		)
		cmd.Dir = repoRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildTeamsProviderErr = fmt.Errorf("go build teams provider: %w\n%s", err, string(output))
		}
	})
	if buildTeamsProviderErr != nil {
		t.Fatal(buildTeamsProviderErr)
	}
}

func teamsManagedInstanceConfig(
	instanceID string,
	tenantID string,
	routing bridgepkg.RoutingPolicy,
) extensiontest.ManagedInstanceConfig {
	return extensiontest.ManagedInstanceConfig{
		ID:            instanceID,
		DisplayName:   "Teams",
		RoutingPolicy: routing,
		BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
			{BindingName: "app_id", Kind: "token", Value: "app-id"},
			{BindingName: "app_password", Kind: "token", Value: "app-password"},
			{BindingName: "app_tenant_id", Kind: "token", Value: tenantID},
		},
	}
}

func teamsProviderStatesReady(states []extensiontest.StateRecord, instanceIDs ...string) bool {
	for _, instanceID := range instanceIDs {
		state, ok := teamsProviderLastStateForInstance(states, instanceID)
		if !ok || state.Status.Normalize() != bridgepkg.BridgeStatusReady {
			return false
		}
	}
	return true
}

func teamsProviderLastStateForInstance(
	states []extensiontest.StateRecord,
	instanceID string,
) (extensiontest.StateRecord, bool) {
	target := strings.TrimSpace(instanceID)
	for i := len(states) - 1; i >= 0; i-- {
		stateID := strings.TrimSpace(states[i].BridgeInstanceID)
		if stateID == "" {
			stateID = strings.TrimSpace(states[i].Instance.ID)
		}
		if stateID == target {
			return states[i], true
		}
	}
	return extensiontest.StateRecord{}, false
}

func teamsProviderMessageWebhook(serviceURL string, text string) map[string]any {
	return map[string]any{
		"type":       "message",
		"id":         "activity-1",
		"channelId":  "msteams",
		"serviceUrl": serviceURL,
		"timestamp":  "2026-04-15T19:05:00Z",
		"text":       text,
		"from":       map[string]any{"id": "29:user-1", "name": "Alice Example"},
		"recipient":  map[string]any{"id": "28:bot", "name": "Bridge Bot"},
		"conversation": map[string]any{
			"id":               "19:channel@thread.tacv2;messageid=activity-1",
			"conversationType": "channel",
			"tenantId":         "11111111-2222-3333-4444-555555555555",
		},
	}
}

func teamsProviderInvokeWebhook(serviceURL string) map[string]any {
	return map[string]any{
		"type":       "invoke",
		"id":         "activity-2",
		"channelId":  "msteams",
		"serviceUrl": serviceURL,
		"timestamp":  "2026-04-15T19:05:01Z",
		"from":       map[string]any{"id": "29:user-1", "name": "Alice Example"},
		"recipient":  map[string]any{"id": "28:bot", "name": "Bridge Bot"},
		"conversation": map[string]any{
			"id":               "19:channel@thread.tacv2;messageid=activity-1",
			"conversationType": "channel",
			"tenantId":         "11111111-2222-3333-4444-555555555555",
		},
		"value": map[string]any{
			"action": map[string]any{
				"data": map[string]any{
					"actionId": "approve",
					"value":    "yes",
				},
			},
		},
	}
}

func teamsProviderReactionWebhook(serviceURL string) map[string]any {
	return map[string]any{
		"type":       "messageReaction",
		"id":         "activity-3",
		"channelId":  "msteams",
		"serviceUrl": serviceURL,
		"timestamp":  "2026-04-15T19:05:02Z",
		"from":       map[string]any{"id": "29:user-1", "name": "Alice Example"},
		"conversation": map[string]any{
			"id":               "19:channel@thread.tacv2;messageid=activity-1",
			"conversationType": "channel",
			"tenantId":         "11111111-2222-3333-4444-555555555555",
		},
		"reactionsAdded":   []map[string]any{{"type": "like"}},
		"reactionsRemoved": []map[string]any{{"type": "sad"}},
	}
}

func postTeamsProviderWebhook(
	t *testing.T,
	server *teamsProviderAPIServer,
	webhookURL string,
	appID string,
	payload any,
) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(payload) error = %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(body))
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+server.SignedToken(t, appID, server.ServiceURL()))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		responseBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			t.Fatalf("io.ReadAll(response body) error = %v", readErr)
		}
		if resp.StatusCode == http.StatusOK {
			return
		}
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusServiceUnavailable {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		t.Fatalf(
			"webhook status = %d, want %d; body=%q",
			resp.StatusCode,
			http.StatusOK,
			strings.TrimSpace(string(responseBody)),
		)
	}

	t.Fatalf("webhook %s did not become ready before timeout", webhookURL)
}

func teamsProviderExpectedThreadID(conversationID string, serviceURL string) string {
	encodedConversationID := base64.RawURLEncoding.EncodeToString([]byte(strings.TrimSpace(conversationID)))
	encodedServiceURL := base64.RawURLEncoding.EncodeToString(
		[]byte(strings.TrimRight(strings.TrimSpace(serviceURL), "/")),
	)
	return "teams:" + encodedConversationID + ":" + encodedServiceURL
}

func teamsProviderCallsContainPath(calls []teamsProviderAPICall, method string, path string) bool {
	for _, call := range calls {
		if call.Method == method && call.Path == path {
			return true
		}
	}
	return false
}

func teamsProviderCallsContainPathPrefix(calls []teamsProviderAPICall, method string, prefix string) bool {
	for _, call := range calls {
		if call.Method == method && strings.HasPrefix(call.Path, prefix) {
			return true
		}
	}
	return false
}

type teamsProviderAPIServer struct {
	server     *httptest.Server
	privateKey *rsa.PrivateKey
	keyID      string

	mu       sync.Mutex
	apiCalls []teamsProviderAPICall
	nextID   int
}

type teamsProviderAPICall struct {
	Method string
	Path   string
	Body   map[string]any
}

func newTeamsProviderAPIServer(t *testing.T) *teamsProviderAPIServer {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}

	srv := &teamsProviderAPIServer{
		privateKey: privateKey,
		keyID:      "teams-integration-key",
		nextID:     900,
	}

	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/openid/.well-known/openidconfiguration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":   "https://api.botframework.com",
				"jwks_uri": serverURL + "/openid/keys",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/openid/keys":
			pub := privateKey.PublicKey
			_ = json.NewEncoder(w).Encode(map[string]any{
				"keys": []map[string]any{{
					"kty":          "RSA",
					"kid":          srv.keyID,
					"x5t":          srv.keyID,
					"n":            base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
					"e":            base64.RawURLEncoding.EncodeToString(teamsProviderBigEndianExponent(pub.E)),
					"endorsements": []string{"msteams"},
				}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/oauth2/v2.0/token":
			srv.recordCall(r.Method, r.URL.Path, teamsProviderDecodeJSONBody(r.Body))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token_type":   "Bearer",
				"expires_in":   3600,
				"access_token": "bot-access-token",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v3/conversations":
			body := teamsProviderDecodeJSONBody(r.Body)
			srv.recordCall(r.Method, r.URL.Path, body)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "a:created-conversation"})
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/activities"):
			body := teamsProviderDecodeJSONBody(r.Body)
			srv.recordCall(r.Method, r.URL.Path, body)
			srv.mu.Lock()
			id := fmt.Sprintf("activity-%d", srv.nextID)
			srv.nextID++
			srv.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id})
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/activities/"):
			srv.recordCall(r.Method, r.URL.Path, teamsProviderDecodeJSONBody(r.Body))
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "updated"})
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/activities/"):
			srv.recordCall(r.Method, r.URL.Path, map[string]any{})
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "unknown path"})
		}
	}))
	serverURL = server.URL
	srv.server = server
	t.Cleanup(server.Close)
	return srv
}

func (s *teamsProviderAPIServer) recordCall(method string, path string, body map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apiCalls = append(s.apiCalls, teamsProviderAPICall{Method: method, Path: path, Body: body})
}

func (s *teamsProviderAPIServer) Calls() []teamsProviderAPICall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]teamsProviderAPICall, len(s.apiCalls))
	copy(out, s.apiCalls)
	return out
}

func (s *teamsProviderAPIServer) ServiceURL() string {
	return s.server.URL
}

func (s *teamsProviderAPIServer) MetadataURL() string {
	return s.server.URL + "/openid/.well-known/openidconfiguration"
}

func (s *teamsProviderAPIServer) TokenURL() string {
	return s.server.URL + "/oauth2/v2.0/token"
}

func (s *teamsProviderAPIServer) SignedToken(t *testing.T, appID string, serviceURL string) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":        "https://api.botframework.com",
		"aud":        appID,
		"serviceUrl": strings.TrimRight(strings.TrimSpace(serviceURL), "/"),
		"exp":        time.Now().UTC().Add(time.Hour).Unix(),
		"nbf":        time.Now().UTC().Add(-time.Minute).Unix(),
		"iat":        time.Now().UTC().Unix(),
	})
	token.Header["kid"] = s.keyID
	signed, err := token.SignedString(s.privateKey)
	if err != nil {
		t.Fatalf("token.SignedString() error = %v", err)
	}
	return signed
}

func teamsProviderDecodeJSONBody(body io.ReadCloser) map[string]any {
	defer func() { _ = body.Close() }()
	if body == nil {
		return map[string]any{}
	}
	out := map[string]any{}
	_ = json.NewDecoder(body).Decode(&out)
	return out
}

func teamsProviderBigEndianExponent(e int) []byte {
	if e == 0 {
		return []byte{0}
	}
	buf := make([]byte, 0, 4)
	for e > 0 {
		buf = append([]byte{byte(e & 0xff)}, buf...)
		e >>= 8
	}
	return buf
}

func teamsProviderExpectedRemoteMessageID(conversationID string, serviceURL string, activityID string) string {
	payload, _ := json.Marshal(map[string]string{
		"conversation_id": strings.TrimSpace(conversationID),
		"service_url":     strings.TrimRight(strings.TrimSpace(serviceURL), "/"),
		"activity_id":     strings.TrimSpace(activityID),
	})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func teamsProviderBigIntFromBytes(raw []byte) *big.Int {
	return new(big.Int).SetBytes(raw)
}

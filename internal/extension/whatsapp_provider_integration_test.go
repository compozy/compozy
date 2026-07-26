//go:build integration

package extensionpkg_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	extensiontest "github.com/compozy/agh/internal/extensiontest"
	observepkg "github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/subprocess"
)

const (
	whatsappProviderListenAddrEnv = "AGH_BRIDGE_WHATSAPP_LISTEN_ADDR"
	whatsappProviderAPIBaseEnv    = "AGH_BRIDGE_WHATSAPP_API_BASE_URL"
)

var (
	buildWhatsAppProviderOnce sync.Once
	buildWhatsAppProviderErr  error
)

func TestWhatsAppProviderLaunchNegotiatesBridgeRuntime(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildWhatsAppProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newWhatsAppProviderAPIServer(t, whatsappProviderAPIServerConfig{})

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: whatsappProviderExtensionDir(repoRoot),
		Platform:     "whatsapp",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{{
			ID:          "brg-whatsapp",
			DisplayName: "WhatsApp",
			RoutingPolicy: bridgepkg.RoutingPolicy{
				IncludePeer: true,
			},
			ProviderConfig: map[string]any{
				"phone_number_id": "123456789",
			},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "access_token", Kind: "token", Value: "access-token"},
				{BindingName: "app_secret", Kind: "token", Value: "app-secret"},
				{BindingName: "verify_token", Kind: "token", Value: "verify-token"},
			},
		}},
		ExtraEnv: map[string]string{
			whatsappProviderListenAddrEnv: listenAddr,
			whatsappProviderAPIBaseEnv:    mockAPI.URL(),
		},
		StartTime: time.Date(2026, 4, 15, 18, 0, 0, 0, time.UTC),
	})

	harness.WaitForHandshake(t, 10*time.Second)
	states := harness.WaitForStates(t, 10*time.Second, func(states []extensiontest.StateRecord) bool {
		return len(states) > 0
	})
	if got, want := states[len(states)-1].Status.Normalize(), bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("last adapter state = %q (error=%q), want %q", got, states[len(states)-1].Error, want)
	}

	report := harness.Report(t)
	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "whatsapp",
		Platform:                  "whatsapp",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "whatsapp",
			BoundSecretNames:    []string{"access_token", "app_secret", "verify_token"},
			ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	row := waitForBridgeHealth(t, 10*time.Second, harness, func(health observepkg.BridgeInstanceHealth) bool {
		return health.Status.Normalize() == bridgepkg.BridgeStatusReady
	})
	if got, want := row.RouteCount, 0; got != want {
		t.Fatalf("bridge health route_count = %d, want %d before ingress", got, want)
	}
}

func TestWhatsAppProviderIngressAndDeliveryConformance(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildWhatsAppProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newWhatsAppProviderAPIServer(t, whatsappProviderAPIServerConfig{})
	startTime := time.Date(2026, 4, 15, 18, 5, 0, 0, time.UTC)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: whatsappProviderExtensionDir(repoRoot),
		Platform:     "whatsapp",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{{
			ID:          "brg-whatsapp",
			DisplayName: "WhatsApp",
			RoutingPolicy: bridgepkg.RoutingPolicy{
				IncludePeer: true,
			},
			ProviderConfig: map[string]any{
				"phone_number_id": "123456789",
			},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "access_token", Kind: "token", Value: "access-token"},
				{BindingName: "app_secret", Kind: "token", Value: "app-secret"},
				{BindingName: "verify_token", Kind: "token", Value: "verify-token"},
			},
		}},
		Driver: extensiontest.NewScriptedPromptDriver(startTime, []extensiontest.ScriptedPromptEvent{
			{Type: acp.EventTypeAgentMessage, Text: "hello"},
			{Type: acp.EventTypeAgentMessage, Text: " world"},
			{Type: acp.EventTypeDone},
		}),
		ExtraEnv: map[string]string{
			whatsappProviderListenAddrEnv: listenAddr,
			whatsappProviderAPIBaseEnv:    mockAPI.URL(),
		},
		StartTime: startTime,
	})

	harness.WaitForHandshake(t, 10*time.Second)
	states := harness.WaitForStates(t, 10*time.Second, func(states []extensiontest.StateRecord) bool {
		return len(states) > 0
	})
	if got, want := states[len(states)-1].Status.Normalize(), bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("last adapter state = %q (error=%q), want %q", got, states[len(states)-1].Error, want)
	}

	webhookURL := fmt.Sprintf("http://%s/whatsapp/%s", listenAddr, harness.Instances[0].ID)
	postWhatsAppProviderWebhook(
		t,
		webhookURL,
		"app-secret",
		whatsappProviderInboundWebhook("123456789", "Need a summary"),
	)

	ingests := harness.WaitForIngests(t, 10*time.Second, func(records []extensiontest.IngestRecord) bool {
		return len(records) > 0 && strings.TrimSpace(records[len(records)-1].Result.SessionID) != ""
	})
	deliveries := harness.WaitForDeliveries(t, 10*time.Second, func(records []extensiontest.DeliveryRecord) bool {
		return len(records) > 0 &&
			normalizeDeliveryEventType(
				records[len(records)-1].Request.Event.EventType,
			) == bridgepkg.DeliveryEventTypeFinal
	})
	report := harness.Report(t)

	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "whatsapp",
		Platform:                  "whatsapp",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		RequireDelivery:           true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "whatsapp",
			BoundSecretNames:    []string{"access_token", "app_secret", "verify_token"},
			ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	if got, want := len(ingests), 1; got != want {
		t.Fatalf("len(ingests) = %d, want %d", got, want)
	}
	if got, want := ingests[0].Envelope.PeerID, "15551234567"; got != want {
		t.Fatalf("ingest envelope peer id = %q, want %q", got, want)
	}
	if got, want := ingests[0].Envelope.Content.Text, "Need a summary"; got != want {
		t.Fatalf("ingest envelope text = %q, want %q", got, want)
	}
	if len(deliveries) < 2 {
		t.Fatalf("len(deliveries) = %d, want at least 2", len(deliveries))
	}

	calls := mockAPI.Calls()
	if len(calls) < 3 {
		t.Fatalf("len(mock api calls) = %d, want at least 3", len(calls))
	}
	if got, want := calls[0].Path, "/v21.0/123456789"; got != want {
		t.Fatalf("calls[0].Path = %q, want %q", got, want)
	}
	if got, want := calls[len(calls)-2].Path, "/v21.0/123456789/messages"; got != want {
		t.Fatalf("delivery path = %q, want %q", calls[len(calls)-2].Path, want)
	}
	if got, want := calls[len(calls)-2].Body["to"], "15551234567"; got != want {
		t.Fatalf("delivery to = %#v, want %q", calls[len(calls)-2].Body["to"], want)
	}

	row := waitForBridgeHealth(t, 10*time.Second, harness, func(health observepkg.BridgeInstanceHealth) bool {
		return health.Status.Normalize() == bridgepkg.BridgeStatusReady && health.RouteCount == 1
	})
	if got, want := row.RouteCount, 1; got != want {
		t.Fatalf("bridge health route_count = %d, want %d", got, want)
	}
}

func TestWhatsAppProviderRateLimitReportsDegradedState(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildWhatsAppProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newWhatsAppProviderAPIServer(t, whatsappProviderAPIServerConfig{FailFirstSendWith429: true})
	startTime := time.Date(2026, 4, 15, 18, 10, 0, 0, time.UTC)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: whatsappProviderExtensionDir(repoRoot),
		Platform:     "whatsapp",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{{
			ID:          "brg-whatsapp",
			DisplayName: "WhatsApp",
			RoutingPolicy: bridgepkg.RoutingPolicy{
				IncludePeer: true,
			},
			ProviderConfig: map[string]any{
				"phone_number_id": "123456789",
			},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "access_token", Kind: "token", Value: "access-token"},
				{BindingName: "app_secret", Kind: "token", Value: "app-secret"},
				{BindingName: "verify_token", Kind: "token", Value: "verify-token"},
			},
		}},
		Driver: extensiontest.NewScriptedPromptDriver(startTime, []extensiontest.ScriptedPromptEvent{
			{Type: acp.EventTypeAgentMessage, Text: "hello"},
			{Type: acp.EventTypeDone},
		}),
		ExtraEnv: map[string]string{
			whatsappProviderListenAddrEnv: listenAddr,
			whatsappProviderAPIBaseEnv:    mockAPI.URL(),
		},
		StartTime: startTime,
	})

	harness.WaitForHandshake(t, 10*time.Second)
	waitForBridgeAdapterState(t, harness, bridgepkg.BridgeStatusReady)
	webhookURL := fmt.Sprintf("http://%s/whatsapp/%s", listenAddr, harness.Instances[0].ID)
	postWhatsAppProviderWebhook(
		t,
		webhookURL,
		"app-secret",
		whatsappProviderInboundWebhook("123456789", "Trigger rate limit"),
	)

	states := harness.WaitForStates(t, 10*time.Second, func(records []extensiontest.StateRecord) bool {
		return stateRecordsContainDegradation(
			records,
			bridgepkg.BridgeStatusDegraded,
			bridgepkg.BridgeDegradationReasonRateLimited,
		)
	})
	if !stateRecordsContainDegradation(
		states,
		bridgepkg.BridgeStatusDegraded,
		bridgepkg.BridgeDegradationReasonRateLimited,
	) {
		t.Fatalf("state markers = %#v, want degraded rate-limited state report", states)
	}
}

func whatsappProviderExtensionDir(repoRoot string) string {
	return filepath.Join(repoRoot, "extensions", "bridges", "whatsapp")
}

func buildWhatsAppProvider(t *testing.T, repoRoot string) {
	t.Helper()

	buildWhatsAppProviderOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(
			ctx,
			"go",
			"build",
			"-o",
			"./extensions/bridges/whatsapp/bin/whatsapp",
			"./extensions/bridges/whatsapp",
		)
		cmd.Dir = repoRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildWhatsAppProviderErr = fmt.Errorf("go build whatsapp provider: %w\n%s", err, string(output))
		}
	})
	if buildWhatsAppProviderErr != nil {
		t.Fatal(buildWhatsAppProviderErr)
	}
}

func whatsappProviderInboundWebhook(phoneNumberID string, text string) map[string]any {
	return map[string]any{
		"object": "whatsapp_business_account",
		"entry": []map[string]any{{
			"id": "waba-1",
			"changes": []map[string]any{{
				"field": "messages",
				"value": map[string]any{
					"messaging_product": "whatsapp",
					"metadata": map[string]any{
						"display_phone_number": "+15551234567",
						"phone_number_id":      phoneNumberID,
					},
					"contacts": []map[string]any{{
						"profile": map[string]any{"name": "Alice Example"},
						"wa_id":   "15551234567",
					}},
					"messages": []map[string]any{{
						"from":      "15551234567",
						"id":        "wamid.abc123",
						"timestamp": strconv.FormatInt(time.Date(2026, 4, 15, 18, 5, 0, 0, time.UTC).Unix(), 10),
						"type":      "text",
						"text":      map[string]any{"body": text},
					}},
				},
			}},
		}},
	}
}

func postWhatsAppProviderWebhook(t *testing.T, url string, appSecret string, payload any) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(payload) error = %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hub-Signature-256", signWhatsAppPayload(body, appSecret))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		payload, readErr := io.ReadAll(resp.Body)
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
			strings.TrimSpace(string(payload)),
		)
	}

	t.Fatalf("webhook %s did not become ready before timeout", url)
}

func signWhatsAppPayload(body []byte, appSecret string) string {
	mac := hmac.New(sha256.New, []byte(appSecret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

type whatsappProviderAPIServerConfig struct {
	FailFirstSendWith429 bool
}

type whatsappProviderAPIServer struct {
	server        *httptest.Server
	mu            sync.Mutex
	calls         []whatsappProviderAPICall
	nextMessageID int
	sendCount     int
	config        whatsappProviderAPIServerConfig
}

type whatsappProviderAPICall struct {
	Path string
	Body map[string]any
}

func newWhatsAppProviderAPIServer(t *testing.T, cfg whatsappProviderAPIServerConfig) *whatsappProviderAPIServer {
	t.Helper()

	srv := &whatsappProviderAPIServer{nextMessageID: 700, config: cfg}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}

		srv.mu.Lock()
		srv.calls = append(srv.calls, whatsappProviderAPICall{Path: r.URL.Path, Body: body})
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages") {
			srv.sendCount++
		}
		sendCount := srv.sendCount
		srv.mu.Unlock()

		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/123456789"):
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "123456789"})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/123456789/messages") && cfg.FailFirstSendWith429 && sendCount == 1:
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"message": "rate limited",
					"code":    130429,
				},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/123456789/messages"):
			srv.mu.Lock()
			messageID := fmt.Sprintf("wamid.%d", srv.nextMessageID)
			srv.nextMessageID++
			srv.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{
				"messages": []map[string]any{{"id": messageID}},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"message": "unknown method",
					"code":    http.StatusNotFound,
				},
			})
		}
	}))
	srv.server = server
	t.Cleanup(server.Close)
	return srv
}

func (s *whatsappProviderAPIServer) URL() string {
	return s.server.URL
}

func (s *whatsappProviderAPIServer) Calls() []whatsappProviderAPICall {
	s.mu.Lock()
	defer s.mu.Unlock()

	cloned := make([]whatsappProviderAPICall, len(s.calls))
	copy(cloned, s.calls)
	return cloned
}

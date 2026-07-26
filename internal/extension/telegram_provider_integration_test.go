//go:build integration

package extensionpkg_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
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
	telegramProviderListenAddrEnv = "AGH_BRIDGE_TELEGRAM_LISTEN_ADDR"
	telegramProviderAPIBaseEnv    = "AGH_BRIDGE_TELEGRAM_API_BASE_URL"
	telegramWebhookSecretHeader   = "X-Telegram-Bot-Api-Secret-Token"
)

var (
	buildTelegramProviderOnce sync.Once
	buildTelegramProviderErr  error
)

func TestTelegramProviderLaunchNegotiatesBridgeRuntime(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildTelegramProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newTelegramProviderAPIServer(t)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: telegramProviderExtensionDir(repoRoot),
		ManagedInstances: []extensiontest.ManagedInstanceConfig{{
			ID:            "brg-telegram",
			DisplayName:   "Telegram",
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludeThread: true, IncludeGroup: true},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "bot_token", Kind: "token", Value: "telegram-bot-token"},
				{BindingName: "webhook_secret", Kind: "token", Value: "top-secret"},
			},
		}},
		ExtraEnv: map[string]string{
			telegramProviderListenAddrEnv: listenAddr,
			telegramProviderAPIBaseEnv:    mockAPI.URL(),
		},
		StartTime: time.Date(2026, 4, 15, 15, 0, 0, 0, time.UTC),
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
		Provider:                  "telegram",
		Platform:                  "telegram",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "telegram",
			BoundSecretNames:    []string{"bot_token", "webhook_secret"},
			ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	if report.Ownership == nil {
		t.Fatal("ownership marker = nil, want provider ownership evidence")
	}
	if got, want := len(report.Ownership.Fetched), 1; got != want {
		t.Fatalf("len(report.Ownership.Fetched) = %d, want %d", got, want)
	}

	row := waitForBridgeHealth(t, 10*time.Second, harness, func(health observepkg.BridgeInstanceHealth) bool {
		return health.Status.Normalize() == bridgepkg.BridgeStatusReady
	})
	if got, want := row.RouteCount, 0; got != want {
		t.Fatalf("bridge health route_count = %d, want %d before ingress", got, want)
	}
}

func TestTelegramProviderIngressAndDeliveryConformance(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildTelegramProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newTelegramProviderAPIServer(t)
	startTime := time.Date(2026, 4, 15, 15, 5, 0, 0, time.UTC)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: telegramProviderExtensionDir(repoRoot),
		ManagedInstances: []extensiontest.ManagedInstanceConfig{{
			ID:            "brg-telegram",
			DisplayName:   "Telegram",
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludeThread: true, IncludeGroup: true},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "bot_token", Kind: "token", Value: "telegram-bot-token"},
				{BindingName: "webhook_secret", Kind: "token", Value: "top-secret"},
			},
		}},
		Driver: extensiontest.NewScriptedPromptDriver(startTime, []extensiontest.ScriptedPromptEvent{
			{Type: acp.EventTypeAgentMessage, Text: "hello"},
			{Type: acp.EventTypeAgentMessage, Text: " world"},
			{Type: acp.EventTypeDone},
		}),
		ExtraEnv: map[string]string{
			telegramProviderListenAddrEnv: listenAddr,
			telegramProviderAPIBaseEnv:    mockAPI.URL(),
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

	postTelegramProviderWebhook(
		t,
		fmt.Sprintf("http://%s/telegram/%s", listenAddr, harness.Instances[0].ID),
		"top-secret",
		telegramProviderInboundUpdate(startTime),
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
		Provider:                  "telegram",
		Platform:                  "telegram",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		RequireDelivery:           true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "telegram",
			BoundSecretNames:    []string{"bot_token", "webhook_secret"},
			ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	if got, want := len(ingests), 1; got != want {
		t.Fatalf("len(ingests) = %d, want %d", got, want)
	}
	if got, want := ingests[0].Envelope.GroupID, "-100777"; got != want {
		t.Fatalf("ingest envelope group id = %q, want %q", got, want)
	}
	if got, want := ingests[0].Envelope.ThreadID, "654"; got != want {
		t.Fatalf("ingest envelope thread id = %q, want %q", got, want)
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
	if got, want := calls[len(calls)-2].Method, "sendMessage"; got != want {
		t.Fatalf("delivery send method = %q, want %q", got, want)
	}
	if got, want := calls[len(calls)-1].Method, "editMessageText"; got != want {
		t.Fatalf("delivery edit method = %q, want %q", got, want)
	}
	if got, want := calls[len(calls)-2].Body["chat_id"], "-100777"; got != want {
		t.Fatalf("sendMessage chat_id = %#v, want %q", calls[len(calls)-2].Body["chat_id"], want)
	}
	if got, want := int(calls[len(calls)-2].Body["message_thread_id"].(float64)), 654; got != want {
		t.Fatalf("sendMessage message_thread_id = %d, want %d", got, want)
	}

	row := waitForBridgeHealth(t, 10*time.Second, harness, func(health observepkg.BridgeInstanceHealth) bool {
		return health.Status.Normalize() == bridgepkg.BridgeStatusReady && health.RouteCount == 1
	})
	if got, want := row.RouteCount, 1; got != want {
		t.Fatalf("bridge health route_count = %d, want %d", got, want)
	}
}

func TestTelegramProviderRestartResumesActiveDelivery(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildTelegramProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newTelegramProviderAPIServer(t)
	startTime := time.Date(2026, 4, 15, 15, 10, 0, 0, time.UTC)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: telegramProviderExtensionDir(repoRoot),
		ManagedInstances: []extensiontest.ManagedInstanceConfig{{
			ID:            "brg-telegram",
			DisplayName:   "Telegram",
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludeThread: true, IncludeGroup: true},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "bot_token", Kind: "token", Value: "telegram-bot-token"},
				{BindingName: "webhook_secret", Kind: "token", Value: "top-secret"},
			},
		}},
		Driver: extensiontest.NewScriptedPromptDriver(startTime, []extensiontest.ScriptedPromptEvent{
			{Type: acp.EventTypeAgentMessage, Text: "hello"},
			{Type: acp.EventTypeDone},
		}),
		StartTime:                startTime,
		CrashOnceOnFirstDelivery: true,
		BrokerOptions: []bridgepkg.DeliveryBrokerOption{
			bridgepkg.WithDeliveryBrokerRetryDelay(20 * time.Millisecond),
		},
		ExtraEnv: map[string]string{
			telegramProviderListenAddrEnv: listenAddr,
			telegramProviderAPIBaseEnv:    mockAPI.URL(),
		},
	})

	harness.WaitForHandshake(t, 10*time.Second)
	states := harness.WaitForStates(t, 10*time.Second, func(states []extensiontest.StateRecord) bool {
		return len(states) > 0
	})
	if got, want := states[len(states)-1].Status.Normalize(), bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("last adapter state = %q (error=%q), want %q", got, states[len(states)-1].Error, want)
	}

	postTelegramProviderWebhook(
		t,
		fmt.Sprintf("http://%s/telegram/%s", listenAddr, harness.Instances[0].ID),
		"top-secret",
		telegramProviderInboundUpdate(startTime),
	)

	deliveries := harness.WaitForDeliveries(t, 10*time.Second, func(records []extensiontest.DeliveryRecord) bool {
		for _, record := range records {
			if normalizeDeliveryEventType(record.Request.Event.EventType) == bridgepkg.DeliveryEventTypeResume {
				return true
			}
		}
		return false
	})
	report := harness.Report(t)

	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "telegram",
		Platform:                  "telegram",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		RequireDelivery:           true,
		RequireResume:             true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "telegram",
			BoundSecretNames:    []string{"bot_token", "webhook_secret"},
			ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	if len(deliveries) < 2 {
		t.Fatalf("len(deliveries) = %d, want at least 2", len(deliveries))
	}
	resume := findDeliveryRecord(t, deliveries, bridgepkg.DeliveryEventTypeResume)
	if resume.Request.Snapshot == nil {
		t.Fatal("resume delivery snapshot = nil, want resumable state")
	}
	if resume.PID == deliveries[0].PID {
		t.Fatalf("resume pid = %d, want a restarted provider process different from %d", resume.PID, deliveries[0].PID)
	}
	if !mockAPI.ContainsMethod("sendMessage") {
		t.Fatal("mock telegram api did not record sendMessage after restart")
	}
}

func telegramProviderExtensionDir(repoRoot string) string {
	return filepath.Join(repoRoot, "extensions", "bridges", "telegram")
}

func buildTelegramProvider(t *testing.T, repoRoot string) {
	t.Helper()

	buildTelegramProviderOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(
			ctx,
			"go",
			"build",
			"-o",
			"./extensions/bridges/telegram/bin/telegram",
			"./extensions/bridges/telegram",
		)
		cmd.Dir = repoRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildTelegramProviderErr = fmt.Errorf("go build telegram provider: %w\n%s", err, string(output))
		}
	})
	if buildTelegramProviderErr != nil {
		t.Fatal(buildTelegramProviderErr)
	}
}

func telegramProviderInboundUpdate(now time.Time) map[string]any {
	return map[string]any{
		"update_id": 9001,
		"message": map[string]any{
			"message_id":        321,
			"message_thread_id": 654,
			"date":              now.Unix(),
			"chat": map[string]any{
				"id":       -100777,
				"type":     "supergroup",
				"title":    "ops",
				"is_forum": true,
			},
			"from": map[string]any{
				"id":         888,
				"username":   "alice",
				"first_name": "Alice",
				"last_name":  "Example",
			},
			"text": "Need a summary",
		},
	}
}

func postTelegramProviderWebhook(t *testing.T, url string, secret string, payload any) {
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
		req.Header.Set(telegramWebhookSecretHeader, secret)

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

func reserveIntegrationListenAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("ln.Close() error = %v", err)
	}
	return addr
}

type telegramProviderAPIServer struct {
	server        *httptest.Server
	mu            sync.Mutex
	calls         []telegramProviderAPICall
	nextMessageID int64
}

type telegramProviderAPICall struct {
	Method string
	Body   map[string]any
}

func newTelegramProviderAPIServer(t *testing.T) *telegramProviderAPIServer {
	t.Helper()

	srv := &telegramProviderAPIServer{nextMessageID: 700}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := filepath.Base(r.URL.Path)
		body := map[string]any{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}

		srv.mu.Lock()
		srv.calls = append(srv.calls, telegramProviderAPICall{Method: method, Body: body})
		srv.mu.Unlock()

		switch method {
		case "getMe":
			writeTelegramProviderAPIResponse(t, w, map[string]any{"id": 1, "username": "aghbot"})
		case "sendMessage":
			srv.mu.Lock()
			messageID := srv.nextMessageID
			srv.nextMessageID++
			srv.mu.Unlock()
			writeTelegramProviderAPIResponse(t, w, map[string]any{"message_id": messageID})
		case "editMessageText", "deleteMessage":
			writeTelegramProviderAPIResponse(t, w, true)
		default:
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":          false,
				"error_code":  http.StatusNotFound,
				"description": "unknown method",
			})
		}
	}))
	srv.server = server
	t.Cleanup(server.Close)
	return srv
}

func (s *telegramProviderAPIServer) URL() string {
	return s.server.URL
}

func (s *telegramProviderAPIServer) Calls() []telegramProviderAPICall {
	s.mu.Lock()
	defer s.mu.Unlock()

	cloned := make([]telegramProviderAPICall, len(s.calls))
	copy(cloned, s.calls)
	return cloned
}

func (s *telegramProviderAPIServer) ContainsMethod(method string) bool {
	for _, call := range s.Calls() {
		if strings.TrimSpace(call.Method) == strings.TrimSpace(method) {
			return true
		}
	}
	return false
}

func writeTelegramProviderAPIResponse(t *testing.T, w http.ResponseWriter, result any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"ok":     true,
		"result": result,
	}); err != nil {
		t.Fatalf("json.NewEncoder().Encode() error = %v", err)
	}
}

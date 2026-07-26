//go:build integration

package extensionpkg_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
	discordProviderListenAddrEnv = "AGH_BRIDGE_DISCORD_LISTEN_ADDR"
	discordProviderAPIBaseEnv    = "AGH_BRIDGE_DISCORD_API_BASE_URL"
	discordWebhookAckBudget      = 3 * time.Second
)

var (
	buildDiscordProviderOnce sync.Once
	buildDiscordProviderErr  error
)

func TestDiscordProviderLaunchNegotiatesBridgeRuntime(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildDiscordProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newDiscordProviderAPIServer(t)
	publicKey, _ := discordProviderTestKeys()

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: discordProviderExtensionDir(repoRoot),
		Platform:     "discord",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{{
			ID:            "brg-discord",
			DisplayName:   "Discord",
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludeGroup: true, IncludeThread: true},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "bot_token", Kind: "token", Value: "discord-bot-token"},
				{BindingName: "public_key", Kind: "token", Value: publicKey},
			},
		}},
		ExtraEnv: map[string]string{
			discordProviderListenAddrEnv: listenAddr,
			discordProviderAPIBaseEnv:    mockAPI.URL(),
		},
		StartTime: time.Date(2026, 4, 15, 17, 0, 0, 0, time.UTC),
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
		Provider:                  "discord",
		Platform:                  "discord",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "discord",
			BoundSecretNames:    []string{"bot_token", "public_key"},
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

func TestDiscordProviderIngressAndDeliveryConformance(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildDiscordProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newDiscordProviderAPIServer(t)
	startTime := time.Date(2026, 4, 15, 17, 5, 0, 0, time.UTC)
	publicKey, privateKey := discordProviderTestKeys()

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: discordProviderExtensionDir(repoRoot),
		Platform:     "discord",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{{
			ID:            "brg-discord",
			DisplayName:   "Discord",
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludeGroup: true, IncludeThread: true},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "bot_token", Kind: "token", Value: "discord-bot-token"},
				{BindingName: "public_key", Kind: "token", Value: publicKey},
			},
		}},
		Driver: extensiontest.NewScriptedPromptDriver(startTime, []extensiontest.ScriptedPromptEvent{
			{Type: acp.EventTypeAgentMessage, Text: "hello"},
			{Type: acp.EventTypeAgentMessage, Text: " world"},
			{Type: acp.EventTypeDone},
		}),
		ExtraEnv: map[string]string{
			discordProviderListenAddrEnv: listenAddr,
			discordProviderAPIBaseEnv:    mockAPI.URL(),
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

	webhookURL := fmt.Sprintf("http://%s/discord/%s", listenAddr, harness.Instances[0].ID)
	status, _, elapsed := postDiscordSignedJSON(
		t,
		webhookURL,
		privateKey,
		startTime,
		discordProviderMessageEventWebhook(startTime),
	)
	if got, want := status, http.StatusNoContent; got != want {
		t.Fatalf("message webhook status = %d, want %d", got, want)
	}
	if elapsed > discordWebhookAckBudget {
		t.Fatalf("message webhook ack took %s, want <= %s", elapsed, discordWebhookAckBudget)
	}

	status, body, elapsed := postDiscordSignedJSON(
		t,
		webhookURL,
		privateKey,
		startTime,
		discordProviderCommandInteraction(),
	)
	if got, want := status, http.StatusOK; got != want {
		t.Fatalf("command interaction status = %d, want %d", got, want)
	}
	if elapsed > discordWebhookAckBudget {
		t.Fatalf("command interaction ack took %s, want <= %s", elapsed, discordWebhookAckBudget)
	}
	if got, want := strings.TrimSpace(body), `{"type":5}`; got != want {
		t.Fatalf("command interaction body = %s, want %s", got, want)
	}

	status, body, elapsed = postDiscordSignedJSON(
		t,
		webhookURL,
		privateKey,
		startTime,
		discordProviderActionInteraction(),
	)
	if got, want := status, http.StatusOK; got != want {
		t.Fatalf("action interaction status = %d, want %d", got, want)
	}
	if elapsed > discordWebhookAckBudget {
		t.Fatalf("action interaction ack took %s, want <= %s", elapsed, discordWebhookAckBudget)
	}
	if got, want := strings.TrimSpace(body), `{"type":6}`; got != want {
		t.Fatalf("action interaction body = %s, want %s", got, want)
	}

	status, _, elapsed = postDiscordSignedJSON(
		t,
		webhookURL,
		privateKey,
		startTime,
		discordProviderReactionEventWebhook(),
	)
	if got, want := status, http.StatusNoContent; got != want {
		t.Fatalf("reaction webhook status = %d, want %d", got, want)
	}
	if elapsed > discordWebhookAckBudget {
		t.Fatalf("reaction webhook ack took %s, want <= %s", elapsed, discordWebhookAckBudget)
	}

	ingests := harness.WaitForIngests(t, 10*time.Second, func(records []extensiontest.IngestRecord) bool {
		if len(records) < 4 {
			return false
		}
		for _, record := range records {
			if strings.TrimSpace(record.Result.SessionID) == "" {
				return false
			}
		}
		return true
	})
	deliveries := harness.WaitForDeliveries(t, 10*time.Second, func(records []extensiontest.DeliveryRecord) bool {
		return len(records) > 0 &&
			normalizeDeliveryEventType(
				records[len(records)-1].Request.Event.EventType,
			) == bridgepkg.DeliveryEventTypeFinal
	})
	report := harness.Report(t)

	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "discord",
		Platform:                  "discord",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		RequireDelivery:           true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "discord",
			BoundSecretNames:    []string{"bot_token", "public_key"},
			ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	message := findIngestByFamily(t, ingests, bridgepkg.InboundEventFamilyMessage)
	if got, want := message.Envelope.GroupID, "channel-1"; got != want {
		t.Fatalf("message group id = %q, want %q", got, want)
	}
	if got, want := message.Envelope.ThreadID, "thread-1"; got != want {
		t.Fatalf("message thread id = %q, want %q", got, want)
	}
	if got, want := message.Envelope.Content.Text, "Need a summary"; got != want {
		t.Fatalf("message text = %q, want %q", got, want)
	}

	command := findIngestByFamily(t, ingests, bridgepkg.InboundEventFamilyCommand)
	if command.Envelope.Command == nil {
		t.Fatal("command envelope missing command payload")
	}
	if got, want := command.Envelope.Command.Command, "/agh summarize"; got != want {
		t.Fatalf("command.Command = %q, want %q", got, want)
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
	if got, want := reaction.Envelope.Reaction.Emoji, ":thumbsup:"; got != want {
		t.Fatalf("reaction.Emoji = %q, want %q", got, want)
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
	if len(calls) < 3 {
		t.Fatalf("len(mock api calls) = %d, want at least 3", len(calls))
	}
	if calls[0].Path != "/users/@me" {
		t.Fatalf("first api path = %q, want /users/@me", calls[0].Path)
	}
	if !discordProviderCallsContainPath(calls, "/channels/thread-1/messages") {
		t.Fatalf("mock api calls = %#v, want /channels/thread-1/messages", calls)
	}
	if !discordProviderCallsContainPath(calls, "/channels/thread-1/messages/discord-msg-1") {
		t.Fatalf("mock api calls = %#v, want PATCH /channels/thread-1/messages/discord-msg-1", calls)
	}

	row := waitForBridgeHealth(t, 10*time.Second, harness, func(health observepkg.BridgeInstanceHealth) bool {
		return health.Status.Normalize() == bridgepkg.BridgeStatusReady && health.RouteCount >= 1
	})
	if row.RouteCount < 1 {
		t.Fatalf("bridge health route_count = %d, want at least 1 after ingress", row.RouteCount)
	}
}

func discordProviderExtensionDir(repoRoot string) string {
	return filepath.Join(repoRoot, "extensions", "bridges", "discord")
}

func buildDiscordProvider(t *testing.T, repoRoot string) {
	t.Helper()

	buildDiscordProviderOnce.Do(func() {
		cmd := exec.Command(
			"go",
			"build",
			"-o",
			filepath.Join(repoRoot, "extensions", "bridges", "discord", "bin", "discord"),
			".",
		)
		cmd.Dir = filepath.Join(repoRoot, "extensions", "bridges", "discord")
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildDiscordProviderErr = fmt.Errorf("go build discord provider: %w\n%s", err, string(output))
		}
	})
	if buildDiscordProviderErr != nil {
		t.Fatal(buildDiscordProviderErr)
	}
}

func discordProviderTestKeys() (string, ed25519.PrivateKey) {
	seed := bytes.Repeat([]byte{7}, ed25519.SeedSize)
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	return hex.EncodeToString(publicKey), privateKey
}

func discordProviderMessageEventWebhook(now time.Time) map[string]any {
	return map[string]any{
		"type": 1,
		"event": map[string]any{
			"id":        "evt-msg-1",
			"type":      "MESSAGE_CREATE",
			"timestamp": now.Format(time.RFC3339Nano),
			"data": map[string]any{
				"id":           "msg-in-1",
				"channel_id":   "thread-1",
				"guild_id":     "guild-1",
				"parent_id":    "channel-1",
				"channel_type": 11,
				"content":      "Need a summary",
				"timestamp":    now.Format(time.RFC3339Nano),
				"author": map[string]any{
					"id":          "user-1",
					"username":    "alice",
					"global_name": "Alice",
				},
			},
		},
	}
}

func discordProviderCommandInteraction() map[string]any {
	return map[string]any{
		"id":             "ixn-cmd-1",
		"type":           2,
		"token":          "ixn-token-1",
		"application_id": "app-1",
		"guild_id":       "guild-1",
		"channel_id":     "thread-1",
		"channel": map[string]any{
			"id":        "thread-1",
			"type":      11,
			"parent_id": "channel-1",
		},
		"member": map[string]any{
			"user": map[string]any{
				"id":          "user-1",
				"username":    "alice",
				"global_name": "Alice",
			},
		},
		"data": map[string]any{
			"name": "agh",
			"options": []map[string]any{{
				"name": "summarize",
				"type": 1,
				"options": []map[string]any{{
					"name":  "topic",
					"value": "release notes",
				}},
			}},
		},
	}
}

func discordProviderActionInteraction() map[string]any {
	return map[string]any{
		"id":             "ixn-action-1",
		"type":           3,
		"token":          "ixn-token-2",
		"application_id": "app-1",
		"guild_id":       "guild-1",
		"channel_id":     "thread-1",
		"channel": map[string]any{
			"id":        "thread-1",
			"type":      11,
			"parent_id": "channel-1",
		},
		"member": map[string]any{
			"user": map[string]any{
				"id":          "user-1",
				"username":    "alice",
				"global_name": "Alice",
			},
		},
		"message": map[string]any{
			"id": "provider-msg-1",
		},
		"data": map[string]any{
			"custom_id":      "approve",
			"component_type": 2,
			"values":         []string{"yes"},
		},
	}
}

func discordProviderReactionEventWebhook() map[string]any {
	return map[string]any{
		"type": 1,
		"event": map[string]any{
			"id":        "evt-react-1",
			"type":      "MESSAGE_REACTION_ADD",
			"timestamp": time.Date(2026, 4, 15, 17, 5, 2, 0, time.UTC).Format(time.RFC3339Nano),
			"data": map[string]any{
				"channel_id":   "thread-1",
				"guild_id":     "guild-1",
				"parent_id":    "channel-1",
				"channel_type": 11,
				"message_id":   "provider-msg-1",
				"user_id":      "user-1",
				"emoji": map[string]any{
					"name": "thumbsup",
				},
			},
		},
	}
}

func postDiscordSignedJSON(
	t *testing.T,
	webhookURL string,
	privateKey ed25519.PrivateKey,
	_ time.Time,
	payload any,
) (int, string, time.Duration) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	timestamp := fmt.Sprintf("%d", time.Now().UTC().Unix())
	message := append([]byte(timestamp), body...)
	signature := ed25519.Sign(privateKey, message)

	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature-Timestamp", timestamp)
	req.Header.Set("X-Signature-Ed25519", hex.EncodeToString(signature))

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http.Do() error = %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	return resp.StatusCode, string(data), time.Since(start)
}

type discordProviderAPICall struct {
	Method string
	Path   string
}

type discordProviderAPIServer struct {
	server *httptest.Server
	mu     sync.Mutex
	calls  []discordProviderAPICall
}

func newDiscordProviderAPIServer(t *testing.T) *discordProviderAPIServer {
	t.Helper()

	mock := &discordProviderAPIServer{}
	mock.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mock.mu.Lock()
		mock.calls = append(mock.calls, discordProviderAPICall{
			Method: r.Method,
			Path:   r.URL.Path,
		})
		mock.mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/users/@me":
			writeDiscordProviderJSON(t, w, http.StatusOK, map[string]any{
				"id":       "app-1",
				"username": "agh",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/channels/thread-1/messages":
			writeDiscordProviderJSON(t, w, http.StatusOK, map[string]any{
				"id": "discord-msg-1",
			})
		case r.Method == http.MethodPatch && r.URL.Path == "/channels/thread-1/messages/discord-msg-1":
			writeDiscordProviderJSON(t, w, http.StatusOK, map[string]any{
				"id": "discord-msg-1",
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/channels/thread-1/messages/discord-msg-1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected discord api call: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(mock.server.Close)
	return mock
}

func (s *discordProviderAPIServer) URL() string {
	return s.server.URL
}

func (s *discordProviderAPIServer) Calls() []discordProviderAPICall {
	s.mu.Lock()
	defer s.mu.Unlock()
	calls := make([]discordProviderAPICall, len(s.calls))
	copy(calls, s.calls)
	return calls
}

func discordProviderCallsContainPath(calls []discordProviderAPICall, path string) bool {
	for _, call := range calls {
		if call.Path == path {
			return true
		}
	}
	return false
}

func writeDiscordProviderJSON(t *testing.T, w http.ResponseWriter, status int, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("json.Encode() error = %v", err)
	}
}

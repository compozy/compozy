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
	slackProviderListenAddrEnv = "AGH_BRIDGE_SLACK_LISTEN_ADDR"
	slackProviderAPIBaseEnv    = "AGH_BRIDGE_SLACK_API_BASE_URL"
	slackSignatureVersion      = "v0"
)

var (
	buildSlackProviderOnce sync.Once
	buildSlackProviderErr  error
)

func TestSlackProviderLaunchNegotiatesBridgeRuntime(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildSlackProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newSlackProviderAPIServer(t)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: slackProviderExtensionDir(repoRoot),
		Platform:     "slack",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{{
			ID:            "brg-slack",
			DisplayName:   "Slack",
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludeGroup: true},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "bot_token", Kind: "token", Value: "xoxb-slack-token"},
				{BindingName: "signing_secret", Kind: "token", Value: "top-secret"},
			},
		}},
		ExtraEnv: map[string]string{
			slackProviderListenAddrEnv: listenAddr,
			slackProviderAPIBaseEnv:    mockAPI.URL(),
		},
		StartTime: time.Date(2026, 4, 15, 16, 0, 0, 0, time.UTC),
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
		Provider:                  "slack",
		Platform:                  "slack",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "slack",
			BoundSecretNames:    []string{"bot_token", "signing_secret"},
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

func TestSlackProviderIngressInteractionsAndDeliveryConformance(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildSlackProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newSlackProviderAPIServer(t)
	startTime := time.Date(2026, 4, 15, 16, 5, 0, 0, time.UTC)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: slackProviderExtensionDir(repoRoot),
		Platform:     "slack",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{{
			ID:            "brg-slack",
			DisplayName:   "Slack",
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludeGroup: true},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "bot_token", Kind: "token", Value: "xoxb-slack-token"},
				{BindingName: "signing_secret", Kind: "token", Value: "top-secret"},
			},
		}},
		Driver: extensiontest.NewScriptedPromptDriver(startTime, []extensiontest.ScriptedPromptEvent{
			{Type: acp.EventTypeAgentMessage, Text: "hello"},
			{Type: acp.EventTypeAgentMessage, Text: " world"},
			{Type: acp.EventTypeDone},
		}),
		ExtraEnv: map[string]string{
			slackProviderListenAddrEnv: listenAddr,
			slackProviderAPIBaseEnv:    mockAPI.URL(),
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

	webhookURL := fmt.Sprintf("http://%s/slack/%s", listenAddr, harness.Instances[0].ID)
	postSlackProviderJSONWebhook(t, webhookURL, "top-secret", startTime, slackProviderMessageWebhook(startTime))
	postSlackProviderFormWebhook(
		t,
		webhookURL,
		"top-secret",
		startTime,
		"token=t&team_id=T1&channel_id=C123&channel_name=general&user_id=U123&user_name=alice&command=%2Fagh&text=summarize&trigger_id=1337.42",
	)
	postSlackProviderFormWebhook(
		t,
		webhookURL,
		"top-secret",
		startTime,
		"payload="+urlQueryEscape(slackProviderBlockActionsPayload()),
	)
	postSlackProviderJSONWebhook(t, webhookURL, "top-secret", startTime, slackProviderReactionWebhook())

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
		Provider:                  "slack",
		Platform:                  "slack",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		RequireDelivery:           true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "slack",
			BoundSecretNames:    []string{"bot_token", "signing_secret"},
			ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
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

	message := findIngestByFamily(t, ingests, bridgepkg.InboundEventFamilyMessage)
	if got, want := message.Envelope.GroupID, "C123"; got != want {
		t.Fatalf("message group id = %q, want %q", got, want)
	}
	if got, want := message.Envelope.ThreadID, "1775866805.100000"; got != want {
		t.Fatalf("message thread id = %q, want %q", got, want)
	}
	if got, want := message.Envelope.Content.Text, "Need a summary"; got != want {
		t.Fatalf("message text = %q, want %q", got, want)
	}

	command := findIngestByFamily(t, ingests, bridgepkg.InboundEventFamilyCommand)
	if command.Envelope.Command == nil {
		t.Fatal("command envelope missing command payload")
	}
	if got, want := command.Envelope.Command.Command, "/agh"; got != want {
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

	calls := mockAPI.Calls()
	if len(calls) < 3 {
		t.Fatalf("len(mock api calls) = %d, want at least 3", len(calls))
	}
	if got, want := calls[0].Method, "auth.test"; got != want {
		t.Fatalf("calls[0].Method = %q, want %q", got, want)
	}
	if !slackProviderCallsContainMethod(calls, "chat.postMessage") {
		t.Fatalf("mock api calls = %#v, want chat.postMessage", calls)
	}
	if !slackProviderCallsContainMethod(calls, "chat.update") {
		t.Fatalf("mock api calls = %#v, want chat.update", calls)
	}

	row := waitForBridgeHealth(t, 10*time.Second, harness, func(health observepkg.BridgeInstanceHealth) bool {
		return health.Status.Normalize() == bridgepkg.BridgeStatusReady && health.RouteCount >= 1
	})
	if row.RouteCount < 1 {
		t.Fatalf("bridge health route_count = %d, want at least 1 after ingress", row.RouteCount)
	}
}

func slackProviderExtensionDir(repoRoot string) string {
	return filepath.Join(repoRoot, "extensions", "bridges", "slack")
}

func buildSlackProvider(t *testing.T, repoRoot string) {
	t.Helper()

	buildSlackProviderOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(
			ctx,
			"go",
			"build",
			"-o",
			"./extensions/bridges/slack/bin/slack",
			"./extensions/bridges/slack",
		)
		cmd.Dir = repoRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildSlackProviderErr = fmt.Errorf("go build slack provider: %w\n%s", err, string(output))
		}
	})
	if buildSlackProviderErr != nil {
		t.Fatal(buildSlackProviderErr)
	}
}

func slackProviderMessageWebhook(now time.Time) map[string]any {
	return map[string]any{
		"type":       "event_callback",
		"team_id":    "T1",
		"event_id":   "EvMessage",
		"event_time": now.Unix(),
		"event": map[string]any{
			"type":         "message",
			"channel":      "C123",
			"channel_type": "channel",
			"user":         "U123",
			"username":     "alice",
			"text":         "Need a summary",
			"ts":           "1775866805.100000",
			"thread_ts":    "1775866805.100000",
		},
	}
}

func slackProviderReactionWebhook() map[string]any {
	return map[string]any{
		"type":     "event_callback",
		"team_id":  "T1",
		"event_id": "EvReaction",
		"event": map[string]any{
			"type":      "reaction_added",
			"user":      "U123",
			"reaction":  "thumbsup",
			"event_ts":  "1775866805.300000",
			"item_user": "U999",
			"item": map[string]any{
				"type":    "message",
				"channel": "C123",
				"ts":      "1775866805.100000",
			},
		},
	}
}

func slackProviderBlockActionsPayload() string {
	return `{"type":"block_actions","trigger_id":"trigger-1","response_url":"https://hooks.slack.test/action","channel":{"id":"C123"},"container":{"type":"message","channel_id":"C123","message_ts":"1775866805.100000","thread_ts":"1775866805.100000"},"message":{"ts":"1775866805.100000","thread_ts":"1775866805.100000"},"user":{"id":"U123","username":"alice"},"actions":[{"type":"button","action_id":"approve","block_id":"primary","value":"yes","action_ts":"1775866805.200000"}]}`
}

func postSlackProviderJSONWebhook(t *testing.T, endpoint string, secret string, now time.Time, payload any) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(payload) error = %v", err)
	}
	postSignedSlackProviderRequest(t, endpoint, secret, now, "application/json", body)
}

func postSlackProviderFormWebhook(t *testing.T, endpoint string, secret string, now time.Time, body string) {
	t.Helper()
	postSignedSlackProviderRequest(t, endpoint, secret, now, "application/x-www-form-urlencoded", []byte(body))
}

func postSignedSlackProviderRequest(
	t *testing.T,
	endpoint string,
	secret string,
	now time.Time,
	contentType string,
	body []byte,
) {
	t.Helper()

	signingTime := time.Now().UTC()
	if now.IsZero() {
		now = signingTime
	}
	timestamp := fmt.Sprintf("%d", signingTime.Unix())
	signature := slackProviderSignature(secret, timestamp, body)
	deadline := time.Now().Add(10 * time.Second)

	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("X-Slack-Request-Timestamp", timestamp)
		req.Header.Set("X-Slack-Signature", signature)

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

	t.Fatalf("webhook %s did not become ready before timeout", endpoint)
}

func findIngestByFamily(
	t *testing.T,
	records []extensiontest.IngestRecord,
	family bridgepkg.InboundEventFamily,
) extensiontest.IngestRecord {
	t.Helper()

	want := strings.TrimSpace(string(family))
	for _, record := range records {
		if strings.TrimSpace(string(record.Envelope.EventFamily)) == want {
			return record
		}
	}
	t.Fatalf("ingest records did not contain family %q", string(family))
	return extensiontest.IngestRecord{}
}

func slackProviderSignature(secret string, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	_, _ = mac.Write([]byte(slackSignatureVersion + ":" + strings.TrimSpace(timestamp) + ":"))
	_, _ = mac.Write(body)
	return slackSignatureVersion + "=" + hex.EncodeToString(mac.Sum(nil))
}

func urlQueryEscape(value string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		" ", "%20",
		"\"", "%22",
		"{", "%7B",
		"}", "%7D",
		":", "%3A",
		",", "%2C",
		"/", "%2F",
		"[", "%5B",
		"]", "%5D",
	)
	return replacer.Replace(value)
}

type slackProviderAPIServer struct {
	server *httptest.Server
	mu     sync.Mutex
	calls  []slackProviderAPICall
	nextTS string
}

type slackProviderAPICall struct {
	Method string
	Body   map[string]any
}

func newSlackProviderAPIServer(t *testing.T) *slackProviderAPIServer {
	t.Helper()

	srv := &slackProviderAPIServer{nextTS: "1775866805.900000"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := strings.TrimPrefix(r.URL.Path, "/")
		body := map[string]any{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}

		srv.mu.Lock()
		srv.calls = append(srv.calls, slackProviderAPICall{Method: method, Body: body})
		nextTS := srv.nextTS
		srv.mu.Unlock()

		switch method {
		case "auth.test":
			writeSlackProviderAPIResponse(t, w, map[string]any{"ok": true, "bot_id": "B1", "user_id": "U1"})
		case "chat.postMessage":
			writeSlackProviderAPIResponse(t, w, map[string]any{"ok": true, "ts": nextTS})
		case "chat.update", "chat.delete":
			writeSlackProviderAPIResponse(t, w, map[string]any{"ok": true})
		default:
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":    false,
				"error": "unknown_method",
			})
		}
	}))
	srv.server = server
	t.Cleanup(server.Close)
	return srv
}

func (s *slackProviderAPIServer) URL() string {
	return s.server.URL
}

func (s *slackProviderAPIServer) Calls() []slackProviderAPICall {
	s.mu.Lock()
	defer s.mu.Unlock()

	cloned := make([]slackProviderAPICall, len(s.calls))
	copy(cloned, s.calls)
	return cloned
}

func slackProviderCallsContainMethod(calls []slackProviderAPICall, method string) bool {
	for _, call := range calls {
		if strings.TrimSpace(call.Method) == strings.TrimSpace(method) {
			return true
		}
	}
	return false
}

func writeSlackProviderAPIResponse(t *testing.T, w http.ResponseWriter, payload map[string]any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("json.NewEncoder().Encode() error = %v", err)
	}
}

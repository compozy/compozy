//go:build integration

package extensionpkg_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strconv"
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
	gchatProviderListenAddrEnv  = "AGH_BRIDGE_GCHAT_LISTEN_ADDR"
	gchatProviderAPIBaseEnv     = "AGH_BRIDGE_GCHAT_API_BASE_URL"
	gchatProviderTokenURLEnv    = "AGH_BRIDGE_GCHAT_TOKEN_URL"
	gchatProviderDirectCertsEnv = "AGH_BRIDGE_GCHAT_DIRECT_CERTS_URL"
	gchatProviderPubSubCertsEnv = "AGH_BRIDGE_GCHAT_PUBSUB_CERTS_URL"

	gchatProviderDirectIssuer = "chat@system.gserviceaccount.com"
	gchatProviderPubSubIssuer = "https://accounts.google.com"
)

var (
	buildGChatProviderOnce sync.Once
	buildGChatProviderErr  error
)

func TestGChatProviderLaunchNegotiatesBridgeRuntime(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildGChatProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newGChatProviderAPIServer(t)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: gchatProviderExtensionDir(repoRoot),
		Platform:     "gchat",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{{
			ID:            "brg-gchat",
			DisplayName:   "Google Chat",
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludeGroup: true, IncludeThread: true},
			ProviderConfig: map[string]any{
				"mode": "hybrid",
				"verification": map[string]any{
					"pubsub_audience":              "https://example.test/pubsub",
					"pubsub_service_account_email": "push@example.iam.gserviceaccount.com",
				},
			},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "credentials_json", Kind: "json", Value: gchatProviderCredentialsJSON(t)},
				{BindingName: "project_number", Kind: "token", Value: "123456789"},
			},
		}},
		ExtraEnv: map[string]string{
			gchatProviderListenAddrEnv:  listenAddr,
			gchatProviderAPIBaseEnv:     mockAPI.URL(),
			gchatProviderTokenURLEnv:    mockAPI.TokenURL(),
			gchatProviderDirectCertsEnv: mockAPI.DirectCertsURL(),
			gchatProviderPubSubCertsEnv: mockAPI.PubSubCertsURL(),
		},
		StartTime: time.Date(2026, 4, 15, 20, 30, 0, 0, time.UTC),
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
		Provider:                  "gchat",
		Platform:                  "gchat",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "gchat",
			BoundSecretNames:    []string{"credentials_json", "project_number"},
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

func TestGChatProviderIngressAndDeliveryConformance(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildGChatProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newGChatProviderAPIServer(t)
	startTime := time.Date(2026, 4, 15, 20, 35, 0, 0, time.UTC)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: gchatProviderExtensionDir(repoRoot),
		Platform:     "gchat",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{{
			ID:            "brg-gchat",
			DisplayName:   "Google Chat",
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludeGroup: true, IncludeThread: true},
			ProviderConfig: map[string]any{
				"mode": "hybrid",
				"verification": map[string]any{
					"pubsub_audience":              "https://example.test/pubsub",
					"pubsub_service_account_email": "push@example.iam.gserviceaccount.com",
				},
			},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "credentials_json", Kind: "json", Value: gchatProviderCredentialsJSON(t)},
				{BindingName: "project_number", Kind: "token", Value: "123456789"},
			},
		}},
		Driver: extensiontest.NewScriptedPromptDriver(startTime, []extensiontest.ScriptedPromptEvent{
			{Type: acp.EventTypeAgentMessage, Text: "hello"},
			{Type: acp.EventTypeAgentMessage, Text: " world"},
			{Type: acp.EventTypeDone},
		}),
		ExtraEnv: map[string]string{
			gchatProviderListenAddrEnv:  listenAddr,
			gchatProviderAPIBaseEnv:     mockAPI.URL(),
			gchatProviderTokenURLEnv:    mockAPI.TokenURL(),
			gchatProviderDirectCertsEnv: mockAPI.DirectCertsURL(),
			gchatProviderPubSubCertsEnv: mockAPI.PubSubCertsURL(),
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

	webhookURL := fmt.Sprintf("http://%s/gchat/%s", listenAddr, harness.Instances[0].ID)
	postGChatProviderWebhook(
		t,
		webhookURL,
		mockAPI.signDirectToken(t, "123456789"),
		gchatProviderDirectMessagePayload(startTime),
	)
	postGChatProviderWebhook(
		t,
		webhookURL,
		mockAPI.signPubSubToken(t, "https://example.test/pubsub", "push@example.iam.gserviceaccount.com"),
		gchatProviderPubSubReactionPayload(),
	)

	ingests := harness.WaitForIngests(t, 10*time.Second, func(records []extensiontest.IngestRecord) bool {
		return len(records) >= 2
	})
	deliveries := harness.WaitForDeliveries(t, 10*time.Second, func(records []extensiontest.DeliveryRecord) bool {
		return len(records) > 0 &&
			normalizeDeliveryEventType(
				records[len(records)-1].Request.Event.EventType,
			) == bridgepkg.DeliveryEventTypeFinal
	})
	report := harness.Report(t)

	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "gchat",
		Platform:                  "gchat",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		RequireDelivery:           true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{{
			InstanceID:          harness.Instances[0].ID,
			ExtensionName:       "gchat",
			BoundSecretNames:    []string{"credentials_json", "project_number"},
			ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
		}},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	message := findIngestByFamily(t, ingests, bridgepkg.InboundEventFamilyMessage)
	if got, want := message.Envelope.GroupID, "spaces/AAA"; got != want {
		t.Fatalf("message group id = %q, want %q", got, want)
	}
	if got, want := message.Envelope.ThreadID, gchatExpectedThreadID(
		"spaces/AAA",
		"spaces/AAA/threads/thread-1",
		false,
	); got != want {
		t.Fatalf("message thread id = %q, want %q", got, want)
	}
	if got, want := message.Envelope.Content.Text, "Need a summary"; got != want {
		t.Fatalf("message text = %q, want %q", got, want)
	}

	reaction := findIngestByFamily(t, ingests, bridgepkg.InboundEventFamilyReaction)
	if reaction.Envelope.Reaction == nil {
		t.Fatal("reaction envelope missing reaction payload")
	}
	if got, want := reaction.Envelope.Reaction.Emoji, "👍"; got != want {
		t.Fatalf("reaction emoji = %q, want %q", got, want)
	}
	if got, want := reaction.Envelope.ThreadID, gchatExpectedThreadID(
		"spaces/AAA",
		"spaces/AAA/threads/thread-react",
		false,
	); got != want {
		t.Fatalf("reaction thread id = %q, want %q", got, want)
	}

	if len(deliveries) < 2 {
		t.Fatalf("len(deliveries) = %d, want at least 2", len(deliveries))
	}

	calls := mockAPI.Calls()
	if !gchatProviderCallsContain(calls, http.MethodPost, "/oauth2/token") {
		t.Fatalf("mock api calls = %#v, want %s %s", calls, http.MethodPost, "/oauth2/token")
	}
	if !gchatProviderCallsContain(calls, http.MethodPost, "/v1/spaces/AAA/messages") {
		t.Fatalf("mock api calls = %#v, want delivery POST", calls)
	}
	if !gchatProviderCallsContain(calls, http.MethodPatch, "/v1/spaces/AAA/messages/msg-1") {
		t.Fatalf("mock api calls = %#v, want delivery PATCH", calls)
	}

	row := waitForBridgeHealth(t, 10*time.Second, harness, func(health observepkg.BridgeInstanceHealth) bool {
		return health.Status.Normalize() == bridgepkg.BridgeStatusReady && health.RouteCount >= 1
	})
	if row.RouteCount < 1 {
		t.Fatalf("bridge health route_count = %d, want at least 1", row.RouteCount)
	}
}

func gchatProviderExtensionDir(repoRoot string) string {
	return filepath.Join(repoRoot, "extensions", "bridges", "gchat")
}

func buildGChatProvider(t *testing.T, repoRoot string) {
	t.Helper()

	buildGChatProviderOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(
			ctx,
			"go",
			"build",
			"-o",
			"./extensions/bridges/gchat/bin/gchat",
			"./extensions/bridges/gchat",
		)
		cmd.Dir = repoRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildGChatProviderErr = fmt.Errorf("go build gchat provider: %w\n%s", err, string(output))
		}
	})
	if buildGChatProviderErr != nil {
		t.Fatal(buildGChatProviderErr)
	}
}

func postGChatProviderWebhook(t *testing.T, endpoint string, bearerToken string, payload []byte) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(bearerToken))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
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
			strings.TrimSpace(string(body)),
		)
	}

	t.Fatalf("webhook %s did not become ready before timeout", endpoint)
}

func gchatProviderDirectMessagePayload(now time.Time) []byte {
	payload, err := json.Marshal(map[string]any{
		"chat": map[string]any{
			"eventTime": now.Format(time.RFC3339Nano),
			"messagePayload": map[string]any{
				"space": map[string]any{
					"name": "spaces/AAA",
					"type": "SPACE",
				},
				"message": map[string]any{
					"name":         "spaces/AAA/messages/msg-direct",
					"argumentText": "Need a summary",
					"createTime":   now.Format(time.RFC3339Nano),
					"sender": map[string]any{
						"name":        "users/123",
						"displayName": "Alice Example",
						"email":       "alice@example.com",
					},
					"thread": map[string]any{
						"name": "spaces/AAA/threads/thread-1",
					},
				},
			},
		},
	})
	if err != nil {
		panic(err)
	}
	return payload
}

func gchatProviderPubSubReactionPayload() []byte {
	payload, err := json.Marshal(map[string]any{
		"reaction": map[string]any{
			"name": "spaces/AAA/messages/msg-react/reactions/rxn-1",
			"emoji": map[string]any{
				"unicode": "👍",
			},
			"user": map[string]any{
				"name":        "users/456",
				"displayName": "Dave",
			},
		},
	})
	if err != nil {
		panic(err)
	}

	push, err := json.Marshal(map[string]any{
		"message": map[string]any{
			"data":        base64.StdEncoding.EncodeToString(payload),
			"messageId":   "pubsub-1",
			"publishTime": "2026-04-15T20:35:01Z",
			"attributes": map[string]string{
				"ce-type":    "google.workspace.chat.reaction.v1.created",
				"ce-subject": "//chat.googleapis.com/spaces/AAA",
				"ce-time":    "2026-04-15T20:35:01Z",
			},
		},
		"subscription": "projects/test/subscriptions/gchat",
	})
	if err != nil {
		panic(err)
	}
	return push
}

func gchatProviderCredentialsJSON(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	privateKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	encoded, err := json.Marshal(map[string]string{
		"client_email": "bot@example.iam.gserviceaccount.com",
		"private_key":  string(privateKey),
		"token_uri":    "https://oauth2.googleapis.com/token",
	})
	if err != nil {
		t.Fatalf("json.Marshal(credentials) error = %v", err)
	}
	return string(encoded)
}

func gchatExpectedThreadID(spaceName string, threadName string, isDM bool) string {
	trimmedSpace := strings.TrimSpace(spaceName)
	if trimmedSpace == "" {
		return ""
	}

	encodedThread := ""
	if strings.TrimSpace(threadName) != "" {
		encodedThread = ":" + base64.RawURLEncoding.EncodeToString([]byte(strings.TrimSpace(threadName)))
	}
	dmSuffix := ""
	if isDM {
		dmSuffix = ":dm"
	}
	return "gchat:" + trimmedSpace + encodedThread + dmSuffix
}

func gchatProviderCallsContain(calls []gchatProviderAPICall, method string, path string) bool {
	for _, call := range calls {
		if strings.EqualFold(strings.TrimSpace(call.Method), strings.TrimSpace(method)) &&
			strings.TrimSpace(call.Path) == strings.TrimSpace(path) {
			return true
		}
	}
	return false
}

type gchatProviderAPIServer struct {
	server         *httptest.Server
	mu             sync.Mutex
	calls          []gchatProviderAPICall
	directKey      *rsa.PrivateKey
	pubSubKey      *rsa.PrivateKey
	directCertPEM  string
	pubSubCertPEM  string
	messageCounter int
	messageStore   map[string]map[string]any
}

type gchatProviderAPICall struct {
	Method string
	Path   string
	Body   map[string]any
}

func newGChatProviderAPIServer(t *testing.T) *gchatProviderAPIServer {
	t.Helper()

	directKey, directCertPEM := generateGChatProviderRSAKeyAndCert(t)
	pubSubKey, pubSubCertPEM := generateGChatProviderRSAKeyAndCert(t)

	srv := &gchatProviderAPIServer{
		directKey:     directKey,
		pubSubKey:     pubSubKey,
		directCertPEM: directCertPEM,
		pubSubCertPEM: pubSubCertPEM,
		messageStore: map[string]map[string]any{
			"spaces/AAA/messages/msg-react": {
				"name": "spaces/AAA/messages/msg-react",
				"space": map[string]any{
					"name": "spaces/AAA",
					"type": "SPACE",
				},
				"thread": map[string]any{
					"name": "spaces/AAA/threads/thread-react",
				},
			},
		},
	}

	srv.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/direct-certs":
			_ = json.NewEncoder(w).Encode(map[string]string{"direct-kid": srv.directCertPEM})
			return
		case r.Method == http.MethodGet && r.URL.Path == "/pubsub-certs":
			_ = json.NewEncoder(w).Encode(map[string]string{"pubsub-kid": srv.pubSubCertPEM})
			return
		case r.Method == http.MethodPost && r.URL.Path == "/oauth2/token":
			srv.recordCall(r.Method, r.URL.Path, map[string]any{"grant_type": "jwt-bearer"})
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "token-123",
				"expires_in":   3600,
				"token_type":   "Bearer",
			})
			return
		}

		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			http.NotFound(w, r)
			return
		}

		body := map[string]any{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		srv.recordCall(r.Method, r.URL.Path, body)

		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages"):
			srv.mu.Lock()
			srv.messageCounter++
			name := "spaces/AAA/messages/msg-" + strconv.Itoa(srv.messageCounter)
			threadName := "spaces/AAA/threads/thread-created"
			if thread, ok := body["thread"].(map[string]any); ok {
				if value, ok := thread["name"].(string); ok && strings.TrimSpace(value) != "" {
					threadName = strings.TrimSpace(value)
				}
			}
			srv.messageStore[name] = map[string]any{
				"name": name,
				"space": map[string]any{
					"name": "spaces/AAA",
					"type": "SPACE",
				},
				"thread": map[string]any{
					"name": threadName,
				},
			}
			srv.mu.Unlock()

			_ = json.NewEncoder(w).Encode(map[string]any{
				"name": name,
				"thread": map[string]any{
					"name": threadName,
				},
			})
			return
		case r.Method == http.MethodPatch:
			name := strings.TrimPrefix(r.URL.Path, "/v1/")
			_ = json.NewEncoder(w).Encode(map[string]any{"name": name})
			return
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
			return
		case r.Method == http.MethodGet:
			name := strings.TrimPrefix(r.URL.Path, "/v1/")
			srv.mu.Lock()
			message, ok := srv.messageStore[name]
			srv.mu.Unlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(message)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	t.Cleanup(srv.server.Close)
	return srv
}

func (s *gchatProviderAPIServer) URL() string {
	return s.server.URL
}

func (s *gchatProviderAPIServer) TokenURL() string {
	return s.server.URL + "/oauth2/token"
}

func (s *gchatProviderAPIServer) DirectCertsURL() string {
	return s.server.URL + "/direct-certs"
}

func (s *gchatProviderAPIServer) PubSubCertsURL() string {
	return s.server.URL + "/pubsub-certs"
}

func (s *gchatProviderAPIServer) Calls() []gchatProviderAPICall {
	s.mu.Lock()
	defer s.mu.Unlock()

	cloned := make([]gchatProviderAPICall, len(s.calls))
	copy(cloned, s.calls)
	return cloned
}

func (s *gchatProviderAPIServer) signDirectToken(t *testing.T, audience string) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": gchatProviderDirectIssuer,
		"aud": audience,
		"iat": time.Now().UTC().Add(-time.Minute).Unix(),
		"exp": time.Now().UTC().Add(time.Hour).Unix(),
	})
	token.Header["kid"] = "direct-kid"
	signed, err := token.SignedString(s.directKey)
	if err != nil {
		t.Fatalf("token.SignedString(direct) error = %v", err)
	}
	return signed
}

func (s *gchatProviderAPIServer) signPubSubToken(t *testing.T, audience string, email string) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":            gchatProviderPubSubIssuer,
		"aud":            audience,
		"email":          email,
		"email_verified": true,
		"iat":            time.Now().UTC().Add(-time.Minute).Unix(),
		"exp":            time.Now().UTC().Add(time.Hour).Unix(),
	})
	token.Header["kid"] = "pubsub-kid"
	signed, err := token.SignedString(s.pubSubKey)
	if err != nil {
		t.Fatalf("token.SignedString(pubsub) error = %v", err)
	}
	return signed
}

func (s *gchatProviderAPIServer) recordCall(method string, path string, body map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, gchatProviderAPICall{
		Method: method,
		Path:   path,
		Body:   body,
	})
}

func generateGChatProviderRSAKeyAndCert(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		NotBefore:    time.Now().UTC().Add(-time.Hour),
		NotAfter:     time.Now().UTC().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("x509.CreateCertificate() error = %v", err)
	}
	return key, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

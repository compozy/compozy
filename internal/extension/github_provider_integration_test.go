//go:build integration

package extensionpkg_test

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
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
	"github.com/compozy/agh/internal/subprocess"
)

const (
	githubProviderListenAddrEnv = "AGH_BRIDGE_GITHUB_LISTEN_ADDR"
	githubProviderAPIBaseEnv    = "AGH_BRIDGE_GITHUB_API_BASE_URL"
	githubProviderWebhookSecret = "top-secret"
)

var (
	buildGitHubProviderOnce sync.Once
	buildGitHubProviderErr  error
)

func TestGitHubProviderLaunchNegotiatesBridgeRuntime(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildGitHubProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newGitHubProviderAPIServer(t)
	privateKey := githubProviderTestPrivateKey(t)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: githubProviderExtensionDir(repoRoot),
		Platform:     "github",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{
			githubPATManagedInstance(listenAddr),
			githubAppManagedInstance(listenAddr, privateKey),
		},
		ExtraEnv: map[string]string{
			githubProviderListenAddrEnv: listenAddr,
			githubProviderAPIBaseEnv:    mockAPI.URL(),
		},
		StartTime: time.Date(2026, 4, 15, 22, 10, 0, 0, time.UTC),
	})

	waitForGitHubReadyStates(t, harness, []string{"brg-github-pat", "brg-github-app"})

	report := harness.Report(t)
	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "github",
		Platform:                  "github",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{
			{
				InstanceID:          "brg-github-pat",
				ExtensionName:       "github",
				BoundSecretNames:    []string{"webhook_secret", "token"},
				ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
			},
			{
				InstanceID:          "brg-github-app",
				ExtensionName:       "github",
				BoundSecretNames:    []string{"webhook_secret", "app_id", "private_key"},
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
}

func TestGitHubProviderSharedWebhookIngressAndDeliveryConformance(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildGitHubProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newGitHubProviderAPIServer(t)
	privateKey := githubProviderTestPrivateKey(t)
	startTime := time.Date(2026, 4, 15, 22, 15, 0, 0, time.UTC)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: githubProviderExtensionDir(repoRoot),
		Platform:     "github",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{
			githubPATManagedInstance(listenAddr),
			githubAppManagedInstance(listenAddr, privateKey),
		},
		Driver: extensiontest.NewScriptedPromptDriver(startTime, []extensiontest.ScriptedPromptEvent{
			{Type: acp.EventTypeAgentMessage, Text: "hello"},
			{Type: acp.EventTypeAgentMessage, Text: " world"},
			{Type: acp.EventTypeDone},
		}),
		ExtraEnv: map[string]string{
			githubProviderListenAddrEnv: listenAddr,
			githubProviderAPIBaseEnv:    mockAPI.URL(),
		},
		StartTime: startTime,
	})

	waitForGitHubReadyStates(t, harness, []string{"brg-github-pat", "brg-github-app"})

	webhookURL := fmt.Sprintf("http://%s/github", listenAddr)
	postGitHubProviderWebhook(
		t,
		webhookURL,
		githubProviderWebhookSecret,
		"issue_comment",
		githubIssueCommentWebhookPayload(startTime),
	)
	postGitHubProviderWebhook(
		t,
		webhookURL,
		githubProviderWebhookSecret,
		"pull_request_review_comment",
		githubReviewCommentWebhookPayload(startTime),
	)

	ingests := harness.WaitForIngests(t, 10*time.Second, func(records []extensiontest.IngestRecord) bool {
		if len(records) < 2 {
			return false
		}
		seen := map[string]bool{}
		for _, record := range records {
			if strings.TrimSpace(record.Result.SessionID) == "" {
				continue
			}
			seen[strings.TrimSpace(record.Envelope.BridgeInstanceID)] = true
		}
		return seen["brg-github-pat"] && seen["brg-github-app"]
	})
	deliveries := harness.WaitForDeliveries(t, 10*time.Second, func(records []extensiontest.DeliveryRecord) bool {
		if len(records) < 4 {
			return false
		}
		finals := 0
		for _, record := range records {
			if normalizeDeliveryEventType(record.Request.Event.EventType) == bridgepkg.DeliveryEventTypeFinal {
				finals++
			}
		}
		return finals >= 2
	})
	report := harness.Report(t)

	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "github",
		Platform:                  "github",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		RequireDelivery:           true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{
			{
				InstanceID:          "brg-github-pat",
				ExtensionName:       "github",
				BoundSecretNames:    []string{"webhook_secret", "token"},
				ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
			},
			{
				InstanceID:          "brg-github-app",
				ExtensionName:       "github",
				BoundSecretNames:    []string{"webhook_secret", "app_id", "private_key"},
				ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
			},
		},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	patIngest := githubFindIngestByInstance(t, ingests, "brg-github-pat")
	if got, want := patIngest.Envelope.GroupID, "acme/app-one"; got != want {
		t.Fatalf("PAT ingest group id = %q, want %q", got, want)
	}
	if got, want := patIngest.Envelope.ThreadID, "github:acme/app-one:issue:42"; got != want {
		t.Fatalf("PAT ingest thread id = %q, want %q", got, want)
	}
	if got, want := patIngest.Envelope.Content.Text, "Need a summary for PAT"; got != want {
		t.Fatalf("PAT ingest text = %q, want %q", got, want)
	}

	appIngest := githubFindIngestByInstance(t, ingests, "brg-github-app")
	if got, want := appIngest.Envelope.GroupID, "acme/app-two"; got != want {
		t.Fatalf("App ingest group id = %q, want %q", got, want)
	}
	if got, want := appIngest.Envelope.ThreadID, "github:acme/app-two:7:rc:300"; got != want {
		t.Fatalf("App ingest thread id = %q, want %q", got, want)
	}
	if got, want := appIngest.Envelope.Content.Text, "Need a summary for review"; got != want {
		t.Fatalf("App ingest text = %q, want %q", got, want)
	}

	if got, want := len(deliveries) >= 4, true; got != want {
		t.Fatalf("len(deliveries) = %d, want at least 4", len(deliveries))
	}

	calls := mockAPI.Calls()
	if !githubProviderCallsContain(calls, http.MethodPost, "/repos/acme/app-one/issues/42/comments") {
		t.Fatalf("mock api calls = %#v, want PAT issue comment POST", calls)
	}
	if !githubProviderCallsContain(calls, http.MethodPatch, "/repos/acme/app-one/issues/comments/910") {
		t.Fatalf("mock api calls = %#v, want PAT issue comment PATCH", calls)
	}
	if !githubProviderCallsContain(calls, http.MethodPost, "/repos/acme/app-two/pulls/7/comments/300/replies") {
		t.Fatalf("mock api calls = %#v, want app review reply POST", calls)
	}
	if !githubProviderCallsContain(calls, http.MethodPatch, "/repos/acme/app-two/pulls/comments/920") {
		t.Fatalf("mock api calls = %#v, want app review comment PATCH", calls)
	}
	if !githubProviderCallsContain(calls, http.MethodPost, "/app/installations/9002/access_tokens") {
		t.Fatalf("mock api calls = %#v, want app installation token exchange", calls)
	}
}

func githubProviderExtensionDir(repoRoot string) string {
	return filepath.Join(repoRoot, "extensions", "bridges", "github")
}

func buildGitHubProvider(t *testing.T, repoRoot string) {
	t.Helper()

	buildGitHubProviderOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(
			ctx,
			"go",
			"build",
			"-o",
			"./extensions/bridges/github/bin/github",
			"./extensions/bridges/github",
		)
		cmd.Dir = repoRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildGitHubProviderErr = fmt.Errorf(
				"go build github provider: %w\n%s",
				err,
				strings.TrimSpace(string(output)),
			)
		}
	})

	if buildGitHubProviderErr != nil {
		t.Fatal(buildGitHubProviderErr)
	}
}

func githubPATManagedInstance(listenAddr string) extensiontest.ManagedInstanceConfig {
	return extensiontest.ManagedInstanceConfig{
		ID:            "brg-github-pat",
		DisplayName:   "GitHub PAT",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludeGroup: true, IncludeThread: true},
		ProviderConfig: map[string]any{
			"mode": "pat",
			"repository": map[string]any{
				"full_name": "acme/app-one",
			},
			"webhook": map[string]any{
				"listen_addr": listenAddr,
				"path":        "/github",
			},
		},
		BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
			{BindingName: "webhook_secret", Kind: "token", Value: githubProviderWebhookSecret},
			{BindingName: "token", Kind: "token", Value: "ghp-test-token"},
		},
	}
}

func githubAppManagedInstance(listenAddr string, privateKey string) extensiontest.ManagedInstanceConfig {
	return extensiontest.ManagedInstanceConfig{
		ID:            "brg-github-app",
		DisplayName:   "GitHub App",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludeGroup: true, IncludeThread: true},
		ProviderConfig: map[string]any{
			"mode":            "app",
			"installation_id": 9002,
			"repository": map[string]any{
				"full_name": "acme/app-two",
			},
			"webhook": map[string]any{
				"listen_addr": listenAddr,
				"path":        "/github",
			},
		},
		BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
			{BindingName: "webhook_secret", Kind: "token", Value: githubProviderWebhookSecret},
			{BindingName: "app_id", Kind: "token", Value: "12345"},
			{BindingName: "private_key", Kind: "token", Value: privateKey},
		},
	}
}

func waitForGitHubReadyStates(t *testing.T, harness *extensiontest.Harness, instanceIDs []string) {
	t.Helper()

	expected := make(map[string]struct{}, len(instanceIDs))
	for _, instanceID := range instanceIDs {
		expected[strings.TrimSpace(instanceID)] = struct{}{}
	}

	harness.WaitForHandshake(t, 10*time.Second)
	harness.WaitForStates(t, 10*time.Second, func(states []extensiontest.StateRecord) bool {
		ready := map[string]bridgepkg.BridgeStatus{}
		for _, state := range states {
			ready[strings.TrimSpace(state.BridgeInstanceID)] = state.Status.Normalize()
		}
		for instanceID := range expected {
			if ready[instanceID] != bridgepkg.BridgeStatusReady {
				return false
			}
		}
		return true
	})
}

func githubFindIngestByInstance(
	t *testing.T,
	records []extensiontest.IngestRecord,
	instanceID string,
) extensiontest.IngestRecord {
	t.Helper()

	for _, record := range records {
		if strings.TrimSpace(record.Envelope.BridgeInstanceID) == strings.TrimSpace(instanceID) {
			return record
		}
	}
	t.Fatalf("ingest records did not contain instance %q", instanceID)
	return extensiontest.IngestRecord{}
}

func postGitHubProviderWebhook(t *testing.T, webhookURL string, secret string, event string, payload any) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, webhookURL, strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", event)
	req.Header.Set("X-Hub-Signature-256", githubProviderSignature(secret, body))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("webhook request error = %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("webhook status = %d, want %d (body=%s)", got, want, strings.TrimSpace(string(bodyBytes)))
	}
}

func githubProviderSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func githubIssueCommentWebhookPayload(now time.Time) map[string]any {
	return map[string]any{
		"action": "created",
		"comment": map[string]any{
			"id":         101,
			"body":       "Need a summary for PAT",
			"created_at": now.Format(time.RFC3339),
			"user": map[string]any{
				"id":    1,
				"login": "alice",
				"type":  "User",
			},
		},
		"issue": map[string]any{
			"number": 42,
		},
		"repository": map[string]any{
			"name":      "app-one",
			"full_name": "acme/app-one",
			"owner": map[string]any{
				"login": "acme",
			},
		},
		"sender": map[string]any{
			"id":    1,
			"login": "alice",
			"type":  "User",
		},
	}
}

func githubReviewCommentWebhookPayload(now time.Time) map[string]any {
	return map[string]any{
		"action": "created",
		"comment": map[string]any{
			"id":             301,
			"in_reply_to_id": 300,
			"body":           "Need a summary for review",
			"path":           "main.go",
			"created_at":     now.Format(time.RFC3339),
			"user": map[string]any{
				"id":    2,
				"login": "bob",
				"type":  "User",
			},
		},
		"pull_request": map[string]any{
			"number": 7,
		},
		"installation": map[string]any{
			"id": 9002,
		},
		"repository": map[string]any{
			"name":      "app-two",
			"full_name": "acme/app-two",
			"owner": map[string]any{
				"login": "acme",
			},
		},
		"sender": map[string]any{
			"id":    2,
			"login": "bob",
			"type":  "User",
		},
	}
}

type githubProviderAPIServer struct {
	server *httptest.Server

	mu    sync.Mutex
	calls []githubProviderAPICall
}

type githubProviderAPICall struct {
	Method string
	Path   string
	Auth   string
	Body   string
}

func newGitHubProviderAPIServer(t *testing.T) *githubProviderAPIServer {
	t.Helper()

	mock := &githubProviderAPIServer{}
	mock.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		mock.mu.Lock()
		mock.calls = append(mock.calls, githubProviderAPICall{
			Method: r.Method,
			Path:   r.URL.Path,
			Auth:   r.Header.Get("Authorization"),
			Body:   string(bodyBytes),
		})
		mock.mu.Unlock()

		switch r.URL.Path {
		case "/user":
			_, _ = io.WriteString(w, `{"id":1,"login":"bridge-bot"}`)
		case "/app/installations/9002/access_tokens":
			_, _ = io.WriteString(w, `{"token":"inst-token","expires_at":"2026-04-15T23:00:00Z"}`)
		case "/repos/acme/app-one/issues/42/comments":
			_, _ = io.WriteString(w, `{"id":910,"body":"hello"}`)
		case "/repos/acme/app-one/issues/comments/910":
			_, _ = io.WriteString(w, `{"id":910,"body":"hello world"}`)
		case "/repos/acme/app-two/pulls/7/comments/300/replies":
			_, _ = io.WriteString(w, `{"id":920,"body":"hello"}`)
		case "/repos/acme/app-two/pulls/comments/920":
			_, _ = io.WriteString(w, `{"id":920,"body":"hello world"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(mock.server.Close)
	return mock
}

func (m *githubProviderAPIServer) URL() string {
	return m.server.URL
}

func (m *githubProviderAPIServer) Calls() []githubProviderAPICall {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := make([]githubProviderAPICall, len(m.calls))
	copy(cloned, m.calls)
	return cloned
}

func githubProviderCallsContain(calls []githubProviderAPICall, method string, path string) bool {
	for _, call := range calls {
		if call.Method == method && call.Path == path {
			return true
		}
	}
	return false
}

func githubProviderTestPrivateKey(t *testing.T) string {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	return string(pem.EncodeToMemory(block))
}

//go:build integration

package extensionpkg_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	linearProviderListenAddrEnv = "AGH_BRIDGE_LINEAR_LISTEN_ADDR"
	linearProviderAPIBaseEnv    = "AGH_BRIDGE_LINEAR_API_BASE_URL"
	linearProviderTokenURLEnv   = "AGH_BRIDGE_LINEAR_TOKEN_URL"
	linearProviderWebhookSecret = "linear-webhook-secret"
)

var (
	buildLinearProviderOnce sync.Once
	buildLinearProviderErr  error
)

func TestLinearProviderLaunchNegotiatesBridgeRuntime(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildLinearProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newLinearProviderAPIServer(t)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: linearProviderExtensionDir(repoRoot),
		Platform:     "linear",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{
			linearCommentsManagedInstance(listenAddr),
			linearAgentManagedInstance(listenAddr),
		},
		ExtraEnv: map[string]string{
			linearProviderListenAddrEnv: listenAddr,
			linearProviderAPIBaseEnv:    mockAPI.URL(),
			linearProviderTokenURLEnv:   mockAPI.TokenURL(),
		},
		StartTime: time.Date(2026, 4, 15, 22, 20, 0, 0, time.UTC),
	})

	waitForLinearReadyStates(t, harness, []string{"brg-linear-comments", "brg-linear-agent"})

	report := harness.Report(t)
	if err := extensiontest.ValidateConformance(report, extensiontest.ConformanceExpectation{
		Provider:                  "linear",
		Platform:                  "linear",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{
			{
				InstanceID:          "brg-linear-comments",
				ExtensionName:       "linear",
				BoundSecretNames:    []string{"webhook_secret", "api_key"},
				ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
			},
			{
				InstanceID:          "brg-linear-agent",
				ExtensionName:       "linear",
				BoundSecretNames:    []string{"webhook_secret", "client_id", "client_secret"},
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

func TestLinearProviderSharedWebhookIngressAndDeliveryConformance(t *testing.T) {
	repoRoot := telegramReferenceRepoRoot(t)
	buildLinearProvider(t, repoRoot)

	listenAddr := reserveIntegrationListenAddr(t)
	mockAPI := newLinearProviderAPIServer(t)
	startTime := time.Date(2026, 4, 15, 22, 25, 0, 0, time.UTC)

	harness := extensiontest.NewHarness(t, extensiontest.HarnessConfig{
		ExtensionDir: linearProviderExtensionDir(repoRoot),
		Platform:     "linear",
		ManagedInstances: []extensiontest.ManagedInstanceConfig{
			linearCommentsManagedInstance(listenAddr),
			linearAgentManagedInstance(listenAddr),
		},
		Driver: extensiontest.NewScriptedPromptDriver(startTime, []extensiontest.ScriptedPromptEvent{
			{Type: acp.EventTypeAgentMessage, Text: "hello"},
			{Type: acp.EventTypeAgentMessage, Text: " world"},
			{Type: acp.EventTypeDone},
		}),
		ExtraEnv: map[string]string{
			linearProviderListenAddrEnv: listenAddr,
			linearProviderAPIBaseEnv:    mockAPI.URL(),
			linearProviderTokenURLEnv:   mockAPI.TokenURL(),
		},
		StartTime: startTime,
	})

	waitForLinearReadyStates(t, harness, []string{"brg-linear-comments", "brg-linear-agent"})

	webhookURL := fmt.Sprintf("http://%s/linear", listenAddr)
	webhookTime := time.Now().UTC()
	postLinearProviderWebhook(t, webhookURL, linearCommentWebhookBody(webhookTime))
	postLinearProviderWebhook(t, webhookURL, linearAgentSessionWebhookBody(webhookTime))

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
		return seen["brg-linear-comments"] && seen["brg-linear-agent"]
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
		Provider:                  "linear",
		Platform:                  "linear",
		RequireOwnedInstanceList:  true,
		RequireOwnedInstanceFetch: true,
		RequireStateReport:        true,
		RequireDelivery:           true,
		ManagedInstances: []extensiontest.ManagedInstanceExpectation{
			{
				InstanceID:          "brg-linear-comments",
				ExtensionName:       "linear",
				BoundSecretNames:    []string{"webhook_secret", "api_key"},
				ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
			},
			{
				InstanceID:          "brg-linear-agent",
				ExtensionName:       "linear",
				BoundSecretNames:    []string{"webhook_secret", "client_id", "client_secret"},
				ExpectedFinalStatus: bridgepkg.BridgeStatusReady,
			},
		},
	}); err != nil {
		t.Fatalf("ValidateConformance() error = %v", err)
	}

	commentIngest := linearFindIngestByInstance(t, ingests, "brg-linear-comments")
	if got, want := commentIngest.Envelope.GroupID, "issue-comments"; got != want {
		t.Fatalf("comment ingest group id = %q, want %q", got, want)
	}
	if got, want := commentIngest.Envelope.ThreadID, "linear:issue-comments:c:comment-root"; got != want {
		t.Fatalf("comment ingest thread id = %q, want %q", got, want)
	}
	if got, want := commentIngest.Envelope.Content.Text, "Need a summary for comments"; got != want {
		t.Fatalf("comment ingest text = %q, want %q", got, want)
	}

	agentIngest := linearFindIngestByInstance(t, ingests, "brg-linear-agent")
	if got, want := agentIngest.Envelope.GroupID, "issue-agent"; got != want {
		t.Fatalf("agent ingest group id = %q, want %q", got, want)
	}
	if got, want := agentIngest.Envelope.ThreadID, "linear:issue-agent:c:comment-agent-root:s:session-agent"; got != want {
		t.Fatalf("agent ingest thread id = %q, want %q", got, want)
	}
	if got, want := agentIngest.Envelope.Content.Text, "Need a summary for agent sessions"; got != want {
		t.Fatalf("agent ingest text = %q, want %q", got, want)
	}

	if got, want := len(deliveries) >= 4, true; got != want {
		t.Fatalf("len(deliveries) = %d, want at least 4", len(deliveries))
	}

	calls := mockAPI.Calls()
	if !linearProviderCallsContain(calls, "commentCreate") {
		t.Fatalf("mock api calls = %#v, want commentCreate", calls)
	}
	if !linearProviderCallsContain(calls, "commentUpdate") {
		t.Fatalf("mock api calls = %#v, want commentUpdate", calls)
	}
	if !linearProviderCallsContain(calls, "agentActivityCreate") {
		t.Fatalf("mock api calls = %#v, want agentActivityCreate", calls)
	}
	if !linearProviderCallsContain(calls, "oauth_token") {
		t.Fatalf("mock api calls = %#v, want oauth_token", calls)
	}
}

func linearProviderExtensionDir(repoRoot string) string {
	return filepath.Join(repoRoot, "extensions", "bridges", "linear")
}

func buildLinearProvider(t *testing.T, repoRoot string) {
	t.Helper()

	buildLinearProviderOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(
			ctx,
			"go",
			"build",
			"-o",
			"./extensions/bridges/linear/bin/linear",
			"./extensions/bridges/linear",
		)
		cmd.Dir = repoRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildLinearProviderErr = fmt.Errorf(
				"go build linear provider: %w\n%s",
				err,
				strings.TrimSpace(string(output)),
			)
		}
	})

	if buildLinearProviderErr != nil {
		t.Fatal(buildLinearProviderErr)
	}
}

func linearCommentsManagedInstance(listenAddr string) extensiontest.ManagedInstanceConfig {
	return extensiontest.ManagedInstanceConfig{
		ID:            "brg-linear-comments",
		DisplayName:   "Linear Comments",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludeGroup: true, IncludeThread: true},
		ProviderConfig: map[string]any{
			"organization_id": "org-comments",
			"mode":            "comments",
			"auth_mode":       "api_key",
			"webhook": map[string]any{
				"listen_addr": listenAddr,
				"path":        "/linear",
			},
		},
		BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
			{BindingName: "webhook_secret", Kind: "token", Value: linearProviderWebhookSecret},
			{BindingName: "api_key", Kind: "token", Value: "linear-api-key-comments"},
		},
	}
}

func linearAgentManagedInstance(listenAddr string) extensiontest.ManagedInstanceConfig {
	return extensiontest.ManagedInstanceConfig{
		ID:            "brg-linear-agent",
		DisplayName:   "Linear Agent Sessions",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludeGroup: true, IncludeThread: true},
		ProviderConfig: map[string]any{
			"organization_id": "org-agent",
			"mode":            "agent_sessions",
			"auth_mode":       "oauth",
			"webhook": map[string]any{
				"listen_addr": listenAddr,
				"path":        "/linear",
			},
		},
		BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
			{BindingName: "webhook_secret", Kind: "token", Value: linearProviderWebhookSecret},
			{BindingName: "client_id", Kind: "token", Value: "linear-client-id"},
			{BindingName: "client_secret", Kind: "token", Value: "linear-client-secret"},
		},
	}
}

func waitForLinearReadyStates(t *testing.T, harness *extensiontest.Harness, instanceIDs []string) {
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

func linearFindIngestByInstance(
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

func postLinearProviderWebhook(t *testing.T, webhookURL string, payload map[string]any) {
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
	req.Header.Set("linear-signature", linearProviderSignature(linearProviderWebhookSecret, body))

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

func linearProviderSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func linearCommentWebhookBody(now time.Time) map[string]any {
	return map[string]any{
		"type":             "Comment",
		"action":           "create",
		"createdAt":        now.Format(time.RFC3339),
		"organizationId":   "org-comments",
		"url":              "https://linear.app/acme/issue/TEST-1#comment-reply-1",
		"webhookId":        "webhook-comment-1",
		"webhookTimestamp": now.UnixMilli(),
		"data": map[string]any{
			"id":        "comment-reply-1",
			"body":      "Need a summary for comments",
			"issueId":   "issue-comments",
			"userId":    "user-comment-1",
			"createdAt": now.Format(time.RFC3339),
			"updatedAt": now.Format(time.RFC3339),
			"parentId":  "comment-root",
			"user": map[string]any{
				"id":   "user-comment-1",
				"name": "Alice Example",
				"url":  "https://linear.app/acme/profiles/alice",
			},
		},
		"actor": map[string]any{
			"id":   "user-comment-1",
			"name": "Alice Example",
			"type": "user",
		},
	}
}

func linearAgentSessionWebhookBody(now time.Time) map[string]any {
	return map[string]any{
		"type":             "AgentSessionEvent",
		"action":           "prompted",
		"createdAt":        now.Format(time.RFC3339),
		"appUserId":        "bot-agent",
		"organizationId":   "org-agent",
		"webhookId":        "webhook-agent-1",
		"webhookTimestamp": now.UnixMilli(),
		"promptContext":    "TEST-2\n\n@get-bot Hello there",
		"agentSession": map[string]any{
			"id":              "session-agent",
			"appUserId":       "bot-agent",
			"issueId":         "issue-agent",
			"commentId":       "comment-agent-root",
			"sourceCommentId": "comment-agent-source",
		},
		"agentActivity": map[string]any{
			"id":        "activity-agent-1",
			"body":      "Need a summary for agent sessions",
			"createdAt": now.Format(time.RFC3339),
			"updatedAt": now.Format(time.RFC3339),
			"content": map[string]any{
				"type": "prompt",
				"body": "Need a summary for agent sessions",
			},
		},
		"actor": map[string]any{
			"id":   "user-agent-1",
			"name": "Bob Example",
			"url":  "https://linear.app/acme/profiles/bob",
			"type": "user",
		},
	}
}

type linearProviderAPICall struct {
	Authorization string
	Operation     string
	Path          string
	Variables     map[string]any
}

type linearProviderAPIServer struct {
	server *httptest.Server

	mu                 sync.Mutex
	calls              []linearProviderAPICall
	commentCreateCount int
	agentActivityCount int
}

func newLinearProviderAPIServer(t *testing.T) *linearProviderAPIServer {
	t.Helper()

	mock := &linearProviderAPIServer{}
	mock.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			bodyBytes, _ := io.ReadAll(r.Body)
			_ = r.Body.Close()
			values, _ := url.ParseQuery(string(bodyBytes))
			mock.recordCall(linearProviderAPICall{
				Operation: "oauth_token",
				Path:      r.URL.Path,
				Variables: map[string]any{
					"grant_type": values.Get("grant_type"),
					"scope":      values.Get("scope"),
				},
			})
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "oauth-access-token",
				"expires_in":   3600,
			})
			return
		case "/graphql":
			payload := map[string]any{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_ = r.Body.Close()
			query, _ := payload["query"].(string)
			variables, _ := payload["variables"].(map[string]any)
			authHeader := r.Header.Get("Authorization")

			switch {
			case strings.Contains(query, "LinearProviderViewer"):
				orgID := "org-comments"
				viewerID := "bot-comment"
				if authHeader == "Bearer oauth-access-token" {
					orgID = "org-agent"
					viewerID = "bot-agent"
				}
				mock.recordCall(linearProviderAPICall{
					Authorization: authHeader,
					Operation:     "viewer",
					Path:          r.URL.Path,
				})
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"viewer": map[string]any{
							"id":          viewerID,
							"displayName": "Linear Bot",
							"organization": map[string]any{
								"id": orgID,
							},
						},
					},
				})
				return
			case strings.Contains(query, "LinearProviderCreateComment"):
				mock.mu.Lock()
				mock.commentCreateCount++
				count := mock.commentCreateCount
				mock.mu.Unlock()
				commentID := fmt.Sprintf("comment-created-%d", count)
				mock.recordCall(linearProviderAPICall{
					Authorization: authHeader,
					Operation:     "commentCreate",
					Path:          r.URL.Path,
					Variables:     variables,
				})
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"commentCreate": map[string]any{
							"success": true,
							"comment": map[string]any{
								"id":        commentID,
								"body":      variables["body"],
								"parentId":  variables["parentId"],
								"url":       "https://linear.app/comment/" + commentID,
								"createdAt": "2026-04-15T22:25:01Z",
								"updatedAt": "2026-04-15T22:25:01Z",
								"issue": map[string]any{
									"id": variables["issueId"],
								},
							},
						},
					},
				})
				return
			case strings.Contains(query, "LinearProviderUpdateComment"):
				mock.recordCall(linearProviderAPICall{
					Authorization: authHeader,
					Operation:     "commentUpdate",
					Path:          r.URL.Path,
					Variables:     variables,
				})
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"commentUpdate": map[string]any{
							"success": true,
							"comment": map[string]any{
								"id":        variables["id"],
								"body":      variables["body"],
								"url":       "https://linear.app/comment/" + fmt.Sprint(variables["id"]),
								"createdAt": "2026-04-15T22:25:01Z",
								"updatedAt": "2026-04-15T22:25:02Z",
								"issue": map[string]any{
									"id": "issue-comments",
								},
							},
						},
					},
				})
				return
			case strings.Contains(query, "LinearProviderCreateAgentActivity"):
				mock.mu.Lock()
				mock.agentActivityCount++
				count := mock.agentActivityCount
				mock.mu.Unlock()
				mock.recordCall(linearProviderAPICall{
					Authorization: authHeader,
					Operation:     "agentActivityCreate",
					Path:          r.URL.Path,
					Variables:     variables,
				})
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"agentActivityCreate": map[string]any{
							"success": true,
							"agentActivity": map[string]any{
								"id": fmt.Sprintf("activity-%d", count),
								"sourceComment": map[string]any{
									"id": fmt.Sprintf("agent-comment-%d", count),
								},
							},
						},
					},
				})
				return
			default:
				http.Error(w, "unexpected graphql operation", http.StatusBadRequest)
				return
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(mock.server.Close)
	return mock
}

func (m *linearProviderAPIServer) URL() string {
	return m.server.URL
}

func (m *linearProviderAPIServer) TokenURL() string {
	return m.server.URL + "/oauth/token"
}

func (m *linearProviderAPIServer) Calls() []linearProviderAPICall {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := make([]linearProviderAPICall, len(m.calls))
	copy(cloned, m.calls)
	return cloned
}

func (m *linearProviderAPIServer) recordCall(call linearProviderAPICall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, call)
}

func linearProviderCallsContain(calls []linearProviderAPICall, operation string) bool {
	for _, call := range calls {
		if strings.TrimSpace(call.Operation) == strings.TrimSpace(operation) {
			return true
		}
	}
	return false
}

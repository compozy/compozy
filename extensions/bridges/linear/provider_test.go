package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

func TestResolveLinearInstanceConfigValidatesProviderOwnedModes(t *testing.T) {
	t.Parallel()

	managed := linearTestManagedInstance("brg-linear")
	managed.Instance.ProviderConfig = []byte(`{
		"organization_id":"org-comments",
		"mode":"comments",
		"auth_mode":"api_key",
		"webhook":{"listen_addr":"127.0.0.1:9999","path":"linear"}
	}`)

	cfg := resolveLinearInstanceConfig(managed, instanceSecretValues{
		webhookSecret: "webhook-secret",
		apiKey:        "linear-api-key",
	}, resolveLinearEnv{
		apiBaseURL: "https://api.linear.app/api/",
		tokenURL:   "https://api.linear.app/oauth/token/",
	})
	if cfg.configError != nil {
		t.Fatalf("resolveLinearInstanceConfig(valid comments/api_key) configError = %v", cfg.configError)
	}
	if got, want := cfg.organizationID, "org-comments"; got != want {
		t.Fatalf("organizationID = %q, want %q", got, want)
	}
	if got, want := cfg.mode, linearModeComments; got != want {
		t.Fatalf("mode = %q, want %q", got, want)
	}
	if got, want := cfg.authMode, linearAuthModeAPIKey; got != want {
		t.Fatalf("authMode = %q, want %q", got, want)
	}
	if got, want := cfg.webhookPath, "/linear"; got != want {
		t.Fatalf("webhookPath = %q, want %q", got, want)
	}
	if got, want := cfg.apiBaseURL, "https://api.linear.app/api"; got != want {
		t.Fatalf("apiBaseURL = %q, want %q", got, want)
	}

	managed.Instance.ProviderConfig = []byte(`{
		"organization_id":"org-agent",
		"mode":"agent-sessions",
		"auth_mode":"oauth",
		"webhook":{"listen_addr":"127.0.0.1:9999","path":"/linear"}
	}`)
	cfg = resolveLinearInstanceConfig(managed, instanceSecretValues{
		webhookSecret: "webhook-secret",
		clientID:      "client-id",
		clientSecret:  "client-secret",
	}, resolveLinearEnv{
		tokenURL: "https://api.linear.app/oauth/token",
	})
	if cfg.configError != nil {
		t.Fatalf("resolveLinearInstanceConfig(valid agent_sessions/oauth) configError = %v", cfg.configError)
	}
	if got, want := cfg.mode, linearModeAgentSessions; got != want {
		t.Fatalf("mode = %q, want %q", got, want)
	}
	if got, want := cfg.authMode, linearAuthModeOAuth; got != want {
		t.Fatalf("authMode = %q, want %q", got, want)
	}
	if !validLinearCredentialedURL("http://127.0.0.1:3000") {
		t.Fatal("validLinearCredentialedURL(loopback http) = false, want true")
	}
	if validLinearCredentialedURL("http://169.254.169.254/latest/meta-data") {
		t.Fatal("validLinearCredentialedURL(link-local http) = true, want false")
	}
	if validLinearCredentialedURL("https://evil.example/graphql") {
		t.Fatal("validLinearCredentialedURL(untrusted https host) = true, want false")
	}

	invalidCases := []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "missing organization",
			payload: `{"mode":"comments","auth_mode":"api_key"}`,
			want:    "organization_id",
		},
		{
			name:    "missing mode",
			payload: `{"organization_id":"org-1","auth_mode":"api_key"}`,
			want:    "mode",
		},
		{
			name:    "missing auth mode",
			payload: `{"organization_id":"org-1","mode":"comments"}`,
			want:    "auth_mode",
		},
		{
			name:    "unsupported mode",
			payload: `{"organization_id":"org-1","mode":"unsupported","auth_mode":"api_key"}`,
			want:    "unsupported provider_config.mode",
		},
	}

	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			managed := linearTestManagedInstance("brg-" + strings.ReplaceAll(tt.name, " ", "-"))
			managed.Instance.ProviderConfig = []byte(tt.payload)
			cfg := resolveLinearInstanceConfig(managed, instanceSecretValues{}, resolveLinearEnv{})
			if cfg.configError == nil || !strings.Contains(cfg.configError.Error(), tt.want) {
				t.Fatalf("configError = %v, want substring %q", cfg.configError, tt.want)
			}
		})
	}
}

func TestDetermineLinearInitialStateValidatesConfiguredAuthModes(t *testing.T) {
	t.Parallel()

	provider := &linearProvider{
		apiFactory: func(cfg resolvedInstanceConfig) linearAPI {
			return linearFakeAPI{
				viewer: &linearViewer{
					ID:             "bot-user-id",
					DisplayName:    "Linear Bot",
					OrganizationID: cfg.organizationID,
				},
			}
		},
	}

	ctx := context.Background()

	_, status, degradation, err := provider.determineInitialState(ctx, resolvedInstanceConfig{
		instanceID:     "brg-api-key",
		organizationID: "org-comments",
		mode:           linearModeComments,
		authMode:       linearAuthModeAPIKey,
		webhookSecret:  "webhook-secret",
	})
	if err == nil {
		t.Fatal("determineInitialState(missing api key) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("missing api key status = %q, want %q", got, want)
	}
	if degradation == nil || degradation.Reason != bridgepkg.BridgeDegradationReasonAuthFailed {
		t.Fatalf("missing api key degradation = %#v, want auth_failed", degradation)
	}

	updated, status, degradation, err := provider.determineInitialState(ctx, resolvedInstanceConfig{
		instanceID:     "brg-api-key",
		organizationID: "org-comments",
		mode:           linearModeComments,
		authMode:       linearAuthModeAPIKey,
		webhookSecret:  "webhook-secret",
		apiKey:         "linear-api-key",
	})
	if err != nil {
		t.Fatalf("determineInitialState(valid api key) error = %v", err)
	}
	if got, want := status, bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("valid api key status = %q, want %q", got, want)
	}
	if degradation != nil {
		t.Fatalf("valid api key degradation = %#v, want nil", degradation)
	}
	if got, want := updated.botUserID, "bot-user-id"; got != want {
		t.Fatalf("botUserID = %q, want %q", got, want)
	}

	_, status, degradation, err = provider.determineInitialState(ctx, resolvedInstanceConfig{
		instanceID:     "brg-oauth",
		organizationID: "org-agent",
		mode:           linearModeAgentSessions,
		authMode:       linearAuthModeOAuth,
		webhookSecret:  "webhook-secret",
		clientID:       "client-id",
	})
	if err == nil {
		t.Fatal("determineInitialState(missing client secret) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("missing oauth credentials status = %q, want %q", got, want)
	}
	if degradation == nil || degradation.Reason != bridgepkg.BridgeDegradationReasonAuthFailed {
		t.Fatalf("missing oauth credentials degradation = %#v, want auth_failed", degradation)
	}

	provider.apiFactory = func(resolvedInstanceConfig) linearAPI {
		return linearFakeAPI{
			viewer: &linearViewer{
				ID:             "bot-user-id",
				DisplayName:    "Linear Bot",
				OrganizationID: "wrong-org",
			},
		}
	}
	_, status, degradation, err = provider.determineInitialState(ctx, resolvedInstanceConfig{
		instanceID:     "brg-mismatch",
		organizationID: "org-agent",
		mode:           linearModeAgentSessions,
		authMode:       linearAuthModeOAuth,
		webhookSecret:  "webhook-secret",
		clientID:       "client-id",
		clientSecret:   "client-secret",
	})
	if err == nil {
		t.Fatal("determineInitialState(org mismatch) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusDegraded; got != want {
		t.Fatalf("org mismatch status = %q, want %q", got, want)
	}
	if degradation == nil || degradation.Reason != bridgepkg.BridgeDegradationReasonTenantConfigInvalid {
		t.Fatalf("org mismatch degradation = %#v, want tenant_config_invalid", degradation)
	}
}

func TestMapLinearWebhookPayloadsPreserveRoutingIdentity(t *testing.T) {
	t.Parallel()

	managed := linearTestManagedInstance("brg-linear")
	now := time.Date(2026, 4, 15, 21, 0, 0, 0, time.UTC)

	commentMapped, ignored, err := mapLinearCommentCreated(linearCommentWebhookPayload{
		Type:             "Comment",
		Action:           "create",
		OrganizationID:   "org-comments",
		WebhookID:        "webhook-comment",
		WebhookTimestamp: now.UnixMilli(),
		URL:              "https://linear.app/test/issue/TEST-1#comment-reply-1",
		Data: linearCommentData{
			ID:        "reply-1",
			Body:      "Need a summary",
			IssueID:   "issue-123",
			UserID:    "user-1",
			CreatedAt: now.Format(time.RFC3339),
			UpdatedAt: now.Format(time.RFC3339),
			ParentID:  "root-comment",
			User: linearActor{
				ID:   "user-1",
				Name: "Alice Example",
				URL:  "https://linear.app/acme/profiles/alice",
			},
		},
		Actor: linearActor{
			ID:   "user-1",
			Name: "Alice Example",
		},
	}, managed, now)
	if err != nil {
		t.Fatalf("mapLinearCommentCreated() error = %v", err)
	}
	if ignored {
		t.Fatal("mapLinearCommentCreated() ignored = true, want false")
	}
	if got, want := commentMapped.Envelope.GroupID, "issue-123"; got != want {
		t.Fatalf("comment group id = %q, want %q", got, want)
	}
	if got, want := commentMapped.Envelope.ThreadID, "linear:issue-123:c:root-comment"; got != want {
		t.Fatalf("comment thread id = %q, want %q", got, want)
	}
	if got, want := commentMapped.Envelope.PlatformMessageID, "reply-1"; got != want {
		t.Fatalf("comment platform message id = %q, want %q", got, want)
	}

	agentMapped, ignored, err := mapLinearAgentSessionEvent(linearAgentSessionWebhookPayload{
		Type:             "AgentSessionEvent",
		Action:           "prompted",
		OrganizationID:   "org-agent",
		WebhookID:        "webhook-agent",
		WebhookTimestamp: now.UnixMilli(),
		PromptContext:    "TEST-1\n\n@get-bot Hello there",
		AgentSession: linearAgentSession{
			ID:              "session-123",
			AppUserID:       "bot-user-id",
			IssueID:         "issue-456",
			CommentID:       "comment-root",
			SourceCommentID: "comment-source",
		},
		AgentActivity: &linearAgentActivityPayload{
			ID:        "activity-1",
			Body:      "Hello there",
			CreatedAt: now.Format(time.RFC3339),
			Content: struct {
				Type string `json:"type,omitempty"`
				Body string `json:"body,omitempty"`
			}{
				Type: "prompt",
				Body: "Hello there",
			},
		},
		Actor: linearActor{
			ID:   "user-2",
			Name: "Bob Example",
			URL:  "https://linear.app/acme/profiles/bob",
		},
	}, &managed, now, "bot-user-id")
	if err != nil {
		t.Fatalf("mapLinearAgentSessionEvent(prompted) error = %v", err)
	}
	if ignored {
		t.Fatal("mapLinearAgentSessionEvent(prompted) ignored = true, want false")
	}
	if got, want := agentMapped.Envelope.GroupID, "issue-456"; got != want {
		t.Fatalf("agent group id = %q, want %q", got, want)
	}
	if got, want := agentMapped.Envelope.ThreadID, "linear:issue-456:c:comment-root:s:session-123"; got != want {
		t.Fatalf("agent thread id = %q, want %q", got, want)
	}
	if got, want := agentMapped.Envelope.PlatformMessageID, "comment-source"; got != want {
		t.Fatalf("agent platform message id = %q, want %q", got, want)
	}
}

func TestVerifyLinearWebhookSignatureAndTimestamp(t *testing.T) {
	t.Parallel()

	body := []byte(`{"type":"Comment","organizationId":"org-1"}`)
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.test/linear",
		strings.NewReader(string(body)),
	)
	req.Header.Set("linear-signature", linearSignature("super-secret", body))

	candidates := []resolvedInstanceConfig{
		{instanceID: "brg-1", organizationID: "org-1", mode: linearModeComments, webhookSecret: "super-secret"},
	}
	if err := verifyLinearWebhookSignature(req, body, candidates); err != nil {
		t.Fatalf("verifyLinearWebhookSignature(valid) error = %v", err)
	}

	req.Header.Set("linear-signature", "bad-signature")
	if err := verifyLinearWebhookSignature(req, body, candidates); err == nil {
		t.Fatal("verifyLinearWebhookSignature(invalid) error = nil, want non-nil")
	}

	t.Run("Should scope signature candidates by organization for shared webhook path", func(t *testing.T) {
		t.Parallel()

		sharedPathBody := []byte(`{"type":"Comment","organizationId":"org-b"}`)
		sharedPathReq := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"http://example.test/linear",
			strings.NewReader(string(sharedPathBody)),
		)
		sharedPathCandidates := []resolvedInstanceConfig{
			{instanceID: "brg-a", organizationID: "org-a", mode: linearModeComments, webhookSecret: "secret-a"},
			{instanceID: "brg-b", organizationID: "org-b", mode: linearModeComments, webhookSecret: "secret-b"},
		}
		scopedCandidates, err := selectLinearWebhookSignatureCandidates(sharedPathBody, sharedPathCandidates)
		if err != nil {
			t.Fatalf("selectLinearWebhookSignatureCandidates(shared path) error = %v", err)
		}
		if got, want := len(scopedCandidates), 1; got != want {
			t.Fatalf("len(scopedCandidates) = %d, want %d", got, want)
		}
		if got, want := scopedCandidates[0].instanceID, "brg-b"; got != want {
			t.Fatalf("scopedCandidates[0].instanceID = %q, want %q", got, want)
		}

		sharedPathReq.Header.Set("linear-signature", linearSignature("secret-b", sharedPathBody))
		if err := verifyLinearWebhookSignature(sharedPathReq, sharedPathBody, scopedCandidates); err != nil {
			t.Fatalf("verifyLinearWebhookSignature(shared path valid) error = %v", err)
		}
		sharedPathReq.Header.Set("linear-signature", linearSignature("secret-a", sharedPathBody))
		if err := verifyLinearWebhookSignature(sharedPathReq, sharedPathBody, scopedCandidates); err == nil {
			t.Fatal("verifyLinearWebhookSignature(shared path cross-instance) error = nil, want non-nil")
		}
	})

	now := time.Date(2026, 4, 15, 21, 5, 0, 0, time.UTC)
	if err := validateLinearWebhookTimestamp(now.Add(-30*time.Second).UnixMilli(), now); err != nil {
		t.Fatalf("validateLinearWebhookTimestamp(within skew) error = %v", err)
	}
	if err := validateLinearWebhookTimestamp(now.Add(-2*time.Minute).UnixMilli(), now); err == nil {
		t.Fatal("validateLinearWebhookTimestamp(stale) error = nil, want non-nil")
	}
}

func TestExecuteLinearDeliveryCommentAndAgentSessionModes(t *testing.T) {
	t.Parallel()

	api := &recordingLinearAPI{
		viewer: &linearViewer{
			ID:             "bot-user-id",
			OrganizationID: "org-comments",
		},
	}

	commentStart := linearTestDeliveryRequest(
		"brg-linear-comments",
		"delivery-comment",
		1,
		bridgepkg.DeliveryEventTypeStart,
		linearThreadRef{
			IssueID:       "issue-123",
			RootCommentID: "root-comment",
		},
		"hello",
		linearModeComments,
	)
	commentAck, state, err := executeLinearDelivery(context.Background(), api, resolvedInstanceConfig{
		mode: linearModeComments,
	}, commentStart, deliveryState{})
	if err != nil {
		t.Fatalf("executeLinearDelivery(comment start) error = %v", err)
	}
	if got, want := commentAck.RemoteMessageID, "comment-created-1"; got != want {
		t.Fatalf("comment start remote id = %q, want %q", got, want)
	}

	commentFinal := commentStart
	commentFinal.Event.Seq = 2
	commentFinal.Event.EventType = bridgepkg.DeliveryEventTypeFinal
	commentFinal.Event.Final = true
	commentFinal.Event.Content.Text = "hello world"
	commentAck, state, err = executeLinearDelivery(context.Background(), api, resolvedInstanceConfig{
		mode: linearModeComments,
	}, commentFinal, state)
	if err != nil {
		t.Fatalf("executeLinearDelivery(comment final) error = %v", err)
	}
	if got, want := commentAck.RemoteMessageID, "comment-created-1"; got != want {
		t.Fatalf("comment final remote id = %q, want %q", got, want)
	}
	if got, want := len(api.updatedComments), 1; got != want {
		t.Fatalf("len(updatedComments) = %d, want %d", got, want)
	}

	commentDelete := commentFinal
	commentDelete.Event.Seq = 3
	commentDelete.Event.EventType = bridgepkg.DeliveryEventTypeDelete
	commentDelete.Event.Operation = bridgepkg.DeliveryOperationDelete
	commentDelete.Event.Reference = &bridgepkg.DeliveryMessageReference{RemoteMessageID: commentAck.RemoteMessageID}
	commentDelete.Event.Content.Text = ""
	if _, _, err := executeLinearDelivery(context.Background(), api, resolvedInstanceConfig{
		mode: linearModeComments,
	}, commentDelete, state); err != nil {
		t.Fatalf("executeLinearDelivery(comment delete) error = %v", err)
	}
	if got, want := api.deletedComments, []string{"comment-created-1"}; !equalStrings(got, want) {
		t.Fatalf("deletedComments = %#v, want %#v", got, want)
	}

	agentStart := linearTestDeliveryRequest(
		"brg-linear-agent",
		"delivery-agent",
		1,
		bridgepkg.DeliveryEventTypeStart,
		linearThreadRef{
			IssueID:        "issue-456",
			RootCommentID:  "comment-root",
			AgentSessionID: "session-123",
		},
		"hello",
		linearModeAgentSessions,
	)
	agentAck, agentState, err := executeLinearDelivery(context.Background(), api, resolvedInstanceConfig{
		mode: linearModeAgentSessions,
	}, agentStart, deliveryState{})
	if err != nil {
		t.Fatalf("executeLinearDelivery(agent start) error = %v", err)
	}
	if got, want := agentAck.RemoteMessageID, "agent-comment-1"; got != want {
		t.Fatalf("agent start remote id = %q, want %q", got, want)
	}

	agentDelta := agentStart
	agentDelta.Event.Seq = 2
	agentDelta.Event.EventType = bridgepkg.DeliveryEventTypeDelta
	agentDelta.Event.Content.Text = "hello world"
	agentAck, agentState, err = executeLinearDelivery(context.Background(), api, resolvedInstanceConfig{
		mode: linearModeAgentSessions,
	}, agentDelta, agentState)
	if err != nil {
		t.Fatalf("executeLinearDelivery(agent delta) error = %v", err)
	}
	if got, want := agentAck.RemoteMessageID, "agent-comment-2"; got != want {
		t.Fatalf("agent delta remote id = %q, want %q", got, want)
	}

	agentFinal := agentDelta
	agentFinal.Event.Seq = 3
	agentFinal.Event.EventType = bridgepkg.DeliveryEventTypeFinal
	agentFinal.Event.Final = true
	finalAck, agentState, err := executeLinearDelivery(context.Background(), api, resolvedInstanceConfig{
		mode: linearModeAgentSessions,
	}, agentFinal, agentState)
	if err != nil {
		t.Fatalf("executeLinearDelivery(agent final no-op) error = %v", err)
	}
	if got, want := finalAck.RemoteMessageID, agentAck.RemoteMessageID; got != want {
		t.Fatalf("agent final remote id = %q, want %q", got, want)
	}
	if got, want := finalAck.ReplaceRemoteMessageID, agentAck.RemoteMessageID; got != want {
		t.Fatalf("agent final replace remote id = %q, want %q", got, want)
	}
	if got, want := api.agentActivities, []string{"hello", " world"}; !equalStrings(got, want) {
		t.Fatalf("agentActivities = %#v, want %#v", got, want)
	}

	failOpen := linearTestDeliveryRequest(
		"brg-linear-agent",
		"delivery-agent-fail-open",
		1,
		bridgepkg.DeliveryEventTypeError,
		linearThreadRef{
			IssueID:        "issue-456",
			RootCommentID:  "comment-root",
			AgentSessionID: "session-123",
		},
		"session stopped before delivery completed",
		linearModeAgentSessions,
	)
	failOpen.Event.Final = true
	failOpen.Event.Error = &bridgepkg.DeliveryErrorDetail{
		Message: "session stopped before delivery completed",
	}
	if _, _, err := executeLinearDelivery(context.Background(), api, resolvedInstanceConfig{
		mode: linearModeAgentSessions,
	}, failOpen, deliveryState{}); err != nil {
		t.Fatalf("executeLinearDelivery(agent fail-open terminal post) error = %v", err)
	}
	if got := api.agentActivities[len(api.agentActivities)-1]; got != "session stopped before delivery completed" {
		t.Fatalf("last agent activity = %q, want visible fail-open terminal post", got)
	}

	agentDelete := agentFinal
	agentDelete.Event.Seq = 4
	agentDelete.Event.EventType = bridgepkg.DeliveryEventTypeDelete
	agentDelete.Event.Operation = bridgepkg.DeliveryOperationDelete
	agentDelete.Event.Reference = &bridgepkg.DeliveryMessageReference{RemoteMessageID: finalAck.RemoteMessageID}
	agentDelete.Event.Content.Text = ""
	if _, _, err := executeLinearDelivery(context.Background(), api, resolvedInstanceConfig{
		mode: linearModeAgentSessions,
	}, agentDelete, agentState); err == nil {
		t.Fatal("executeLinearDelivery(agent delete) error = nil, want non-nil")
	}
}

func TestLinearProviderHandleBridgesDeliverCommittedOutcome(t *testing.T) {
	t.Run("Should evict delivery state after a committed provider mutation", func(t *testing.T) {
		t.Parallel()

		provider, err := newLinearProvider(io.Discard)
		if err != nil {
			t.Fatalf("newLinearProvider() error = %v", err)
		}
		t.Cleanup(func() {
			provider.lifecycle.Stop()
			if err := provider.http.Shutdown(context.Background()); err != nil {
				t.Errorf("provider HTTP shutdown error = %v", err)
			}
		})
		provider.routes.Replace(map[string]resolvedInstanceConfig{
			"brg-linear": {
				instanceID: "brg-linear",
				mode:       linearModeComments,
			},
		}, nil)
		committedAPI := &recordingLinearAPI{
			updateErr: &bridgesdk.CommittedMutationError{Err: errors.New("linear result unavailable")},
		}
		provider.apiFactory = func(resolvedInstanceConfig) linearAPI { return committedAPI }

		request := linearTestDeliveryRequest(
			"brg-linear",
			"delivery-committed",
			2,
			bridgepkg.DeliveryEventTypeFinal,
			linearThreadRef{IssueID: "issue-123", RootCommentID: "root-comment"},
			"hello world",
			linearModeComments,
		)
		request.Event.Final = true
		key := deliveryStateKey("brg-linear", request.Event.DeliveryID)
		provider.deliveries.Store(key, deliveryState{
			LastSeq:         1,
			LastContent:     "hello",
			RemoteMessageID: "comment-created-1",
		})

		_, err = provider.handleBridgesDeliver(context.Background(), nil, request)
		var committedErr *bridgesdk.CommittedMutationError
		if !errors.As(err, &committedErr) {
			t.Fatalf("handleBridgesDeliver() error = %T, want CommittedMutationError", err)
		}
		if got, want := len(committedAPI.updatedComments), 1; got != want {
			t.Fatalf("provider mutations = %d, want %d", got, want)
		}
		if state, ok := provider.deliveries.Load(key); ok {
			t.Fatalf("deliveries[%q] = %#v, want evicted", key, state)
		}
	})
}

func TestLinearClientAPIKeyAndOAuthRequests(t *testing.T) {
	t.Parallel()

	type graphQLCall struct {
		Authorization string
		Query         string
		Variables     map[string]any
	}

	var mu sync.Mutex
	graphQLCalls := make([]graphQLCall, 0)
	tokenBodies := make([]url.Values, 0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			bodyBytes, readErr := io.ReadAll(r.Body)
			closeErr := r.Body.Close()
			if closeErr != nil {
				t.Errorf("close Linear OAuth request body: %v", closeErr)
			}
			if readErr != nil {
				t.Errorf("read Linear OAuth request body: %v", readErr)
				http.Error(w, "read request body", http.StatusBadRequest)
				return
			}
			values, parseErr := url.ParseQuery(string(bodyBytes))
			if parseErr != nil {
				t.Errorf("parse Linear OAuth request body: %v", parseErr)
				http.Error(w, "parse request body", http.StatusBadRequest)
				return
			}
			mu.Lock()
			tokenBodies = append(tokenBodies, values)
			mu.Unlock()
			if err := json.NewEncoder(w).Encode(map[string]any{
				"access_token": "oauth-access-token",
				"expires_in":   3600,
			}); err != nil {
				t.Errorf("encode Linear OAuth response: %v", err)
			}
			return
		case "/graphql":
			payload := linearGraphQLRequest{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := r.Body.Close(); err != nil {
				t.Errorf("close Linear GraphQL request body: %v", err)
			}

			mu.Lock()
			graphQLCalls = append(graphQLCalls, graphQLCall{
				Authorization: r.Header.Get("Authorization"),
				Query:         payload.Query,
				Variables:     payload.Variables,
			})
			mu.Unlock()

			switch {
			case strings.Contains(payload.Query, "LinearProviderViewer"):
				orgID := "org-comments"
				viewerID := "bot-comment"
				if r.Header.Get("Authorization") == "Bearer oauth-access-token" {
					orgID = "org-agent"
					viewerID = "bot-agent"
				}
				if err := json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"viewer": map[string]any{
							"id":          viewerID,
							"displayName": "Linear Bot",
							"organization": map[string]any{
								"id": orgID,
							},
						},
					},
				}); err != nil {
					t.Errorf("encode Linear viewer response: %v", err)
				}
			case strings.Contains(payload.Query, "LinearProviderCreateComment"):
				if err := json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"commentCreate": map[string]any{
							"success": true,
							"comment": map[string]any{
								"id":        "comment-created-1",
								"body":      payload.Variables["body"],
								"parentId":  payload.Variables["parentId"],
								"url":       "https://linear.app/comment/comment-created-1",
								"createdAt": "2026-04-15T21:00:00Z",
								"updatedAt": "2026-04-15T21:00:00Z",
								"issue": map[string]any{
									"id": payload.Variables["issueId"],
								},
							},
						},
					},
				}); err != nil {
					t.Errorf("encode Linear create comment response: %v", err)
				}
			case strings.Contains(payload.Query, "LinearProviderUpdateComment"):
				if err := json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"commentUpdate": map[string]any{
							"success": true,
							"comment": map[string]any{
								"id":        payload.Variables["id"],
								"body":      payload.Variables["body"],
								"url":       "https://linear.app/comment/comment-created-1",
								"createdAt": "2026-04-15T21:00:00Z",
								"updatedAt": "2026-04-15T21:01:00Z",
								"issue": map[string]any{
									"id": "issue-123",
								},
							},
						},
					},
				}); err != nil {
					t.Errorf("encode Linear update comment response: %v", err)
				}
			case strings.Contains(payload.Query, "LinearProviderDeleteComment"):
				if err := json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"commentDelete": map[string]any{"success": true},
					},
				}); err != nil {
					t.Errorf("encode Linear delete comment response: %v", err)
				}
			case strings.Contains(payload.Query, "LinearProviderCreateAgentActivity"):
				if err := json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"agentActivityCreate": map[string]any{
							"success": true,
							"agentActivity": map[string]any{
								"id": "activity-1",
								"sourceComment": map[string]any{
									"id": "agent-comment-1",
								},
							},
						},
					},
				}); err != nil {
					t.Errorf("encode Linear agent activity response: %v", err)
				}
			default:
				http.Error(w, "unexpected query", http.StatusBadRequest)
			}
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	apiKeyClient := &linearClient{
		cfg: resolvedInstanceConfig{
			authMode:    linearAuthModeAPIKey,
			apiKey:      "linear-api-key",
			apiBaseURL:  server.URL,
			webhookPath: "/linear",
		},
		httpClient: server.Client(),
		now:        func() time.Time { return time.Date(2026, 4, 15, 21, 0, 0, 0, time.UTC) },
	}
	viewer, err := apiKeyClient.ValidateAuth(context.Background())
	if err != nil {
		t.Fatalf("ValidateAuth(api_key) error = %v", err)
	}
	if got, want := viewer.OrganizationID, "org-comments"; got != want {
		t.Fatalf("api_key viewer organization = %q, want %q", got, want)
	}
	if _, err := apiKeyClient.CreateComment(context.Background(), "issue-123", "hello", "root-comment"); err != nil {
		t.Fatalf("CreateComment(api_key) error = %v", err)
	}
	if _, err := apiKeyClient.UpdateComment(context.Background(), "comment-created-1", "hello world"); err != nil {
		t.Fatalf("UpdateComment(api_key) error = %v", err)
	}
	if err := apiKeyClient.DeleteComment(context.Background(), "comment-created-1"); err != nil {
		t.Fatalf("DeleteComment(api_key) error = %v", err)
	}

	oauthClient := &linearClient{
		cfg: resolvedInstanceConfig{
			authMode:        linearAuthModeOAuth,
			mode:            linearModeAgentSessions,
			apiBaseURL:      server.URL,
			oauthTokenURL:   server.URL + "/oauth/token",
			clientID:        "client-id",
			clientSecret:    "client-secret",
			oauthTokenCache: &linearOAuthTokenCache{},
		},
		httpClient: server.Client(),
		now:        func() time.Time { return time.Date(2026, 4, 15, 21, 0, 0, 0, time.UTC) },
	}
	viewer, err = oauthClient.ValidateAuth(context.Background())
	if err != nil {
		t.Fatalf("ValidateAuth(oauth) error = %v", err)
	}
	if got, want := viewer.OrganizationID, "org-agent"; got != want {
		t.Fatalf("oauth viewer organization = %q, want %q", got, want)
	}
	if _, err := oauthClient.CreateAgentActivity(context.Background(), "session-123", "hello"); err != nil {
		t.Fatalf("CreateAgentActivity(oauth) error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(tokenBodies) == 0 {
		t.Fatal("oauth token endpoint was not called")
	}
	if got, want := tokenBodies[0].Get("grant_type"), "client_credentials"; got != want {
		t.Fatalf("grant_type = %q, want %q", got, want)
	}
	if got, want := tokenBodies[0].Get(
		"scope",
	), "read,write,comments:create,issues:create,app:mentionable"; got != want {
		t.Fatalf("scope = %q, want %q", got, want)
	}
	if len(graphQLCalls) < 5 {
		t.Fatalf("len(graphQLCalls) = %d, want at least 5", len(graphQLCalls))
	}
	if got, want := graphQLCalls[0].Authorization, "Bearer linear-api-key"; got != want {
		t.Fatalf("api key auth header = %q, want %q", got, want)
	}
	if got, want := graphQLCalls[len(graphQLCalls)-1].Authorization, "Bearer oauth-access-token"; got != want {
		t.Fatalf("oauth auth header = %q, want %q", got, want)
	}
}

func TestLinearClientClassifiesHTTPFailures(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/graphql":
			if strings.Contains(r.Header.Get("Authorization"), "bad-token") {
				http.Error(w, `{"message":"forbidden"}`, http.StatusForbidden)
				return
			}
			http.Error(w, `{"message":"rate limited"}`, http.StatusTooManyRequests)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &linearClient{
		cfg: resolvedInstanceConfig{
			authMode:   linearAuthModeAPIKey,
			apiKey:     "bad-token",
			apiBaseURL: server.URL,
		},
		httpClient: server.Client(),
		now:        func() time.Time { return time.Now().UTC() },
	}
	if _, err := client.ValidateAuth(context.Background()); err == nil {
		t.Fatal("ValidateAuth(403) error = nil, want non-nil")
	} else {
		var authErr *bridgesdk.AuthError
		if !errors.As(err, &authErr) {
			t.Fatalf("ValidateAuth() error = %#v, want auth error", err)
		}
	}

	client.cfg.apiKey = "okay-token"
	if _, err := client.CreateComment(context.Background(), "issue-1", "hello", ""); err == nil {
		t.Fatal("CreateComment(429) error = nil, want non-nil")
	} else {
		var rateErr *bridgesdk.RateLimitError
		if !errors.As(err, &rateErr) {
			t.Fatalf("CreateComment() error = %#v, want rate limit error", err)
		}
	}

	t.Run("Should preserve auth classification when the GraphQL error body read and close fail", func(t *testing.T) {
		t.Parallel()

		readErr := errors.New("Linear GraphQL error body read failed")
		closeErr := errors.New("Linear GraphQL error body close failed")
		body := &linearErrorResponseBody{
			content:  []byte("Linear access denied"),
			readErr:  readErr,
			closeErr: closeErr,
		}
		client := &linearClient{
			cfg: resolvedInstanceConfig{
				authMode:   linearAuthModeAPIKey,
				apiKey:     "linear-api-key",
				apiBaseURL: "http://127.0.0.1",
			},
			httpClient: &http.Client{Transport: linearErrorResponseTransport{
				statusCode: http.StatusForbidden,
				body:       body,
			}},
			now: func() time.Time { return time.Now().UTC() },
		}

		_, err := client.CreateComment(t.Context(), "issue-1", "hello", "")
		if !errors.Is(err, readErr) {
			t.Fatalf("CreateComment() error = %v, want response body read failure", err)
		}
		if !errors.Is(err, closeErr) {
			t.Fatalf("CreateComment() error = %v, want response body close failure", err)
		}
		var authErr *bridgesdk.AuthError
		if !errors.As(err, &authErr) {
			t.Fatalf("CreateComment() error = %T %v, want AuthError", err, err)
		}
		if got, want := bridgesdk.ClassifyError(err).Class, bridgesdk.ErrorClassAuth; got != want {
			t.Fatalf("CreateComment() error class = %q, want %q", got, want)
		}
		if !strings.Contains(authErr.Error(), "Linear access denied") {
			t.Fatalf("CreateComment() auth error = %q, want partial provider body", authErr.Error())
		}
		if bridgesdk.IsCommittedMutation(err) {
			t.Fatalf("CreateComment() error = %v, want pre-commit rejection", err)
		}
		if !body.closed {
			t.Fatal("response body closed = false after error classification")
		}
	})

	t.Run("Should preserve transient OAuth classification when response close fails", func(t *testing.T) {
		t.Parallel()

		closeErr := errors.New("Linear OAuth response body close failed")
		body := &linearErrorResponseBody{
			content:  []byte("Linear OAuth unavailable"),
			closeErr: closeErr,
		}
		client := &linearClient{
			cfg: resolvedInstanceConfig{
				authMode:        linearAuthModeOAuth,
				oauthTokenURL:   "http://127.0.0.1/oauth/token",
				clientID:        "client-id",
				clientSecret:    "client-secret",
				oauthTokenCache: &linearOAuthTokenCache{},
			},
			httpClient: &http.Client{Transport: linearErrorResponseTransport{
				statusCode: http.StatusServiceUnavailable,
				body:       body,
			}},
			now: func() time.Time { return time.Now().UTC() },
		}

		_, err := client.ensureOAuthToken(t.Context())
		if !errors.Is(err, closeErr) {
			t.Fatalf("ensureOAuthToken() error = %v, want response body close failure", err)
		}
		var transientErr *bridgesdk.TransientError
		if !errors.As(err, &transientErr) {
			t.Fatalf("ensureOAuthToken() error = %T %v, want TransientError", err, err)
		}
		if got, want := bridgesdk.ClassifyError(err).Class, bridgesdk.ErrorClassTransient; got != want {
			t.Fatalf("ensureOAuthToken() error class = %q, want %q", got, want)
		}
		if !strings.Contains(transientErr.Error(), "Linear OAuth unavailable") {
			t.Fatalf("ensureOAuthToken() transient error = %q, want provider body", transientErr.Error())
		}
		if !body.closed {
			t.Fatal("response body closed = false after error classification")
		}
	})

	for _, testCase := range []struct {
		name          string
		body          string
		wantCommitted bool
	}{
		{
			name:          "Should not retry a committed create with truncated GraphQL JSON",
			body:          `{"data":{"commentCreate":`,
			wantCommitted: true,
		},
		{
			name:          "Should not retry a committed create without a comment id",
			body:          `{"data":{"commentCreate":{"success":true,"comment":{}}}}`,
			wantCommitted: true,
		},
		{
			name:          "Should preserve an explicit GraphQL mutation rejection as pre-commit",
			body:          `{"data":{"commentCreate":{"success":false}}}`,
			wantCommitted: false,
		},
		{
			name: "Should not retry a mutation when partial GraphQL data also carries errors",
			body: `{"data":{"commentCreate":{"success":true,"comment":{"id":"comment-1"}}},` +
				`"errors":[{"message":"nested resolver failed"}]}`,
			wantCommitted: true,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			attempts := 0
			var attemptsMu sync.Mutex
			committedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attemptsMu.Lock()
				attempts++
				attemptsMu.Unlock()
				w.WriteHeader(http.StatusOK)
				if _, err := io.WriteString(w, testCase.body); err != nil {
					t.Errorf("write GraphQL response: %v", err)
				}
			}))
			defer committedServer.Close()

			committedClient := &linearClient{
				cfg: resolvedInstanceConfig{
					authMode:   linearAuthModeAPIKey,
					apiKey:     "linear-api-key",
					apiBaseURL: committedServer.URL,
				},
				httpClient: committedServer.Client(),
				now:        func() time.Time { return time.Now().UTC() },
			}

			_, err := committedClient.CreateComment(context.Background(), "issue-1", "hello", "")
			if err == nil {
				t.Fatal("CreateComment(result unavailable) error = nil, want non-nil")
			}
			var committedErr *bridgesdk.CommittedMutationError
			if got := errors.As(err, &committedErr); got != testCase.wantCommitted {
				t.Fatalf("CreateComment() committed error = %v, want %v; error = %T", got, testCase.wantCommitted, err)
			}
			attemptsMu.Lock()
			gotAttempts := attempts
			attemptsMu.Unlock()
			if got, want := gotAttempts, 1; got != want {
				t.Fatalf("mutation attempts = %d, want %d", got, want)
			}
		})
	}
}

func TestLinearClientRejectsCredentialedRedirects(t *testing.T) {
	t.Parallel()

	t.Run("Should not follow GraphQL redirects", func(t *testing.T) {
		t.Parallel()

		var (
			mu       sync.Mutex
			evilHits int
		)
		evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			mu.Lock()
			evilHits++
			mu.Unlock()
			http.Error(w, "unexpected redirect follow", http.StatusInternalServerError)
		}))
		defer evil.Close()

		trusted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/graphql" {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, evil.URL+"/graphql", http.StatusTemporaryRedirect)
		}))
		defer trusted.Close()

		client := &linearClient{
			cfg: resolvedInstanceConfig{
				authMode:   linearAuthModeAPIKey,
				apiKey:     "linear-api-key",
				apiBaseURL: trusted.URL,
			},
			httpClient: trusted.Client(),
			now:        func() time.Time { return time.Now().UTC() },
		}
		if _, err := client.ValidateAuth(context.Background()); err == nil {
			t.Fatal("ValidateAuth(redirect) error = nil, want non-nil")
		}

		mu.Lock()
		got := evilHits
		mu.Unlock()
		if got != 0 {
			t.Fatalf("redirect target hits = %d, want 0", got)
		}
	})

	t.Run("Should not follow OAuth token redirects", func(t *testing.T) {
		t.Parallel()

		var (
			mu       sync.Mutex
			evilHits int
		)
		evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			mu.Lock()
			evilHits++
			mu.Unlock()
			http.Error(w, "unexpected redirect follow", http.StatusInternalServerError)
		}))
		defer evil.Close()

		trusted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/oauth/token" {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, evil.URL+"/oauth/token", http.StatusTemporaryRedirect)
		}))
		defer trusted.Close()

		client := &linearClient{
			cfg: resolvedInstanceConfig{
				authMode:        linearAuthModeOAuth,
				apiBaseURL:      trusted.URL,
				oauthTokenURL:   trusted.URL + "/oauth/token",
				clientID:        "client-id",
				clientSecret:    "client-secret",
				oauthTokenCache: &linearOAuthTokenCache{},
			},
			httpClient: trusted.Client(),
			now:        func() time.Time { return time.Now().UTC() },
		}
		if got := client.authToken(context.Background()); got != "" {
			t.Fatalf("authToken(redirect) = %q, want empty", got)
		}

		mu.Lock()
		got := evilHits
		mu.Unlock()
		if got != 0 {
			t.Fatalf("redirect target hits = %d, want 0", got)
		}
	})

	t.Run("Should reject an OAuth redirect status even when it carries a valid token body", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusFound)
			if _, err := io.WriteString(w, `{"access_token":"redirect-token","expires_in":3600}`); err != nil {
				t.Errorf("write redirect OAuth response: %v", err)
			}
		}))
		defer server.Close()

		client := &linearClient{
			cfg: resolvedInstanceConfig{
				authMode:        linearAuthModeOAuth,
				oauthTokenURL:   server.URL,
				clientID:        "client-id",
				clientSecret:    "client-secret",
				oauthTokenCache: &linearOAuthTokenCache{},
			},
			httpClient: server.Client(),
			now:        func() time.Time { return time.Now().UTC() },
		}
		if _, err := client.ensureOAuthToken(t.Context()); err == nil {
			t.Fatal("ensureOAuthToken(302) error = nil, want non-2xx rejection")
		}
	})
}

type linearFakeAPI struct {
	viewer *linearViewer
	err    error
}

type linearErrorResponseTransport struct {
	statusCode int
	body       io.ReadCloser
}

func (t linearErrorResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: t.statusCode,
		Header:     make(http.Header),
		Body:       t.body,
		Request:    req,
	}, nil
}

type linearErrorResponseBody struct {
	content  []byte
	readErr  error
	closeErr error
	reads    int
	closed   bool
}

func (b *linearErrorResponseBody) Read(buffer []byte) (int, error) {
	b.reads++
	if b.reads > 1 {
		return 0, io.EOF
	}
	return copy(buffer, b.content), b.readErr
}

func (b *linearErrorResponseBody) Close() error {
	b.closed = true
	return b.closeErr
}

func (f linearFakeAPI) ValidateAuth(context.Context) (*linearViewer, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.viewer, nil
}

func (f linearFakeAPI) CreateComment(context.Context, string, string, string) (*linearComment, error) {
	return nil, f.err
}

func (f linearFakeAPI) UpdateComment(context.Context, string, string) (*linearComment, error) {
	return nil, f.err
}

func (f linearFakeAPI) DeleteComment(context.Context, string) error {
	return f.err
}

func (f linearFakeAPI) CreateAgentActivity(context.Context, string, string) (*linearAgentActivity, error) {
	return nil, f.err
}

type recordingLinearAPI struct {
	viewer          *linearViewer
	updateErr       error
	createdComments []string
	updatedComments []string
	deletedComments []string
	agentActivities []string
}

func (a *recordingLinearAPI) ValidateAuth(context.Context) (*linearViewer, error) {
	return a.viewer, nil
}

func (a *recordingLinearAPI) CreateComment(_ context.Context, _ string, body string, _ string) (*linearComment, error) {
	a.createdComments = append(a.createdComments, body)
	return &linearComment{ID: "comment-created-1", Body: body}, nil
}

func (a *recordingLinearAPI) UpdateComment(_ context.Context, commentID string, body string) (*linearComment, error) {
	a.updatedComments = append(a.updatedComments, commentID+":"+body)
	if a.updateErr != nil {
		return nil, a.updateErr
	}
	return &linearComment{ID: commentID, Body: body}, nil
}

func (a *recordingLinearAPI) DeleteComment(_ context.Context, commentID string) error {
	a.deletedComments = append(a.deletedComments, commentID)
	return nil
}

func (a *recordingLinearAPI) CreateAgentActivity(
	_ context.Context,
	_ string,
	body string,
) (*linearAgentActivity, error) {
	a.agentActivities = append(a.agentActivities, body)
	return &linearAgentActivity{
		ID: "activity-" + string(rune(len(a.agentActivities)+'0')),
		SourceComment: &struct {
			ID string `json:"id"`
		}{
			ID: "agent-comment-" + string(rune(len(a.agentActivities)+'0')),
		},
	}, nil
}

func linearTestManagedInstance(id string) subprocess.InitializeBridgeManagedInstance {
	return subprocess.InitializeBridgeManagedInstance{
		Instance: bridgepkg.BridgeInstance{
			ID:          id,
			Scope:       bridgepkg.ScopeWorkspace,
			WorkspaceID: "ws-linear",
		},
	}
}

func linearTestDeliveryRequest(
	instanceID string,
	deliveryID string,
	seq int64,
	eventType bridgepkg.DeliveryEventType,
	thread linearThreadRef,
	text string,
	_ string,
) bridgepkg.DeliveryRequest {
	threadID := encodeLinearThreadID(thread)
	return bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       deliveryID,
			BridgeInstanceID: instanceID,
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-linear",
				BridgeInstanceID: instanceID,
				GroupID:          thread.IssueID,
				ThreadID:         threadID,
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: instanceID,
				GroupID:          thread.IssueID,
				ThreadID:         threadID,
				Mode:             bridgepkg.DeliveryModeReply,
			},
			Seq:       seq,
			EventType: eventType,
			Content:   bridgepkg.MessageContent{Text: text},
			Operation: bridgepkg.DeliveryOperationPost,
		},
	}
}

func equalStrings(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for idx := range got {
		if got[idx] != want[idx] {
			return false
		}
	}
	return true
}

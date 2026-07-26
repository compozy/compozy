package main

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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/subprocess"
)

type githubHTTPDoerFunc func(*http.Request) (*http.Response, error)

func (fn githubHTTPDoerFunc) Do(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestMapGitHubWebhookCommentsAndThreadIDs(t *testing.T) {
	t.Parallel()

	managed := subprocess.InitializeBridgeManagedInstance{
		Instance: bridgepkg.BridgeInstance{
			ID:          "brg-github",
			Scope:       bridgepkg.ScopeWorkspace,
			WorkspaceID: "ws-github",
		},
	}
	now := time.Date(2026, 4, 15, 21, 0, 0, 0, time.UTC)

	prItem, err := mapGitHubIssueComment(githubIssuePayload{
		Action: "created",
		Comment: githubIssueComment{
			ID:        101,
			Body:      "Need a summary",
			CreatedAt: now.Format(time.RFC3339),
			User:      githubUser{ID: 1, Login: "alice", Type: "User"},
		},
		Issue: struct {
			Number      int64 `json:"number,omitempty"`
			PullRequest *struct {
				URL string `json:"url,omitempty"`
			} `json:"pull_request,omitempty"`
		}{
			Number: 42,
			PullRequest: &struct {
				URL string `json:"url,omitempty"`
			}{URL: "https://api.github.com/repos/acme/app/pulls/42"},
		},
		Repository: githubRepository{
			Name:     "app",
			FullName: "acme/app",
			Owner:    githubUser{Login: "acme"},
		},
		Installation: &githubInstallation{ID: 7001},
	}, managed, now)
	if err != nil {
		t.Fatalf("mapGitHubIssueComment(pr) error = %v", err)
	}
	if got, want := prItem.Envelope.GroupID, "acme/app"; got != want {
		t.Fatalf("pr group id = %q, want %q", got, want)
	}
	if got, want := prItem.Envelope.ThreadID, "github:acme/app:42"; got != want {
		t.Fatalf("pr thread id = %q, want %q", got, want)
	}
	if got, want := prItem.InstallationID, int64(7001); got != want {
		t.Fatalf("pr installation id = %d, want %d", got, want)
	}

	issueItem, err := mapGitHubIssueComment(githubIssuePayload{
		Action: "created",
		Comment: githubIssueComment{
			ID:        102,
			Body:      "Issue comment",
			CreatedAt: now.Format(time.RFC3339),
			User:      githubUser{ID: 2, Login: "bob", Type: "User"},
		},
		Issue: struct {
			Number      int64 `json:"number,omitempty"`
			PullRequest *struct {
				URL string `json:"url,omitempty"`
			} `json:"pull_request,omitempty"`
		}{
			Number: 11,
		},
		Repository: githubRepository{
			Name:     "app",
			FullName: "acme/app",
			Owner:    githubUser{Login: "acme"},
		},
	}, managed, now)
	if err != nil {
		t.Fatalf("mapGitHubIssueComment(issue) error = %v", err)
	}
	if got, want := issueItem.Envelope.ThreadID, "github:acme/app:issue:11"; got != want {
		t.Fatalf("issue thread id = %q, want %q", got, want)
	}

	reviewItem, err := mapGitHubReviewComment(githubReviewPayload{
		Action: "created",
		Comment: githubReviewComment{
			ID:          301,
			InReplyToID: 300,
			Body:        "Line comment",
			Path:        "main.go",
			CreatedAt:   now.Format(time.RFC3339),
			User:        githubUser{ID: 3, Login: "carol", Type: "User"},
		},
		PullRequest: struct {
			Number int64 `json:"number,omitempty"`
		}{Number: 42},
		Repository: githubRepository{
			Name:     "app",
			FullName: "acme/app",
			Owner:    githubUser{Login: "acme"},
		},
		Installation: &githubInstallation{ID: 7002},
	}, managed, now)
	if err != nil {
		t.Fatalf("mapGitHubReviewComment() error = %v", err)
	}
	if got, want := reviewItem.Envelope.ThreadID, "github:acme/app:42:rc:300"; got != want {
		t.Fatalf("review thread id = %q, want %q", got, want)
	}
	if got, want := reviewItem.Envelope.PlatformMessageID, "301"; got != want {
		t.Fatalf("review platform message id = %q, want %q", got, want)
	}
	if !strings.Contains(string(reviewItem.Envelope.ProviderMetadata), `"root_review_comment_id":300`) {
		t.Fatalf("review provider metadata = %s, want root review comment id", reviewItem.Envelope.ProviderMetadata)
	}

	decoded, err := decodeGitHubThreadID(reviewItem.Envelope.ThreadID)
	if err != nil {
		t.Fatalf("decodeGitHubThreadID() error = %v", err)
	}
	if got, want := decoded.ReviewCommentID, int64(300); got != want {
		t.Fatalf("decoded review comment id = %d, want %d", got, want)
	}
}

func TestDetermineGitHubInitialStateValidatesPATAndAppModes(t *testing.T) {
	t.Parallel()

	provider, err := newGitHubProvider(io.Discard)
	if err != nil {
		t.Fatalf("newGitHubProvider() error = %v", err)
	}
	provider.apiFactory = func(_ resolvedInstanceConfig) githubAPI {
		return &fakeGitHubAPI{viewer: &githubViewer{ID: 77, Login: "bridge-bot"}}
	}

	ctx := context.Background()

	_, status, degradation, err := provider.determineInitialState(ctx, &resolvedInstanceConfig{
		instanceID:    "brg-pat",
		mode:          githubModePAT,
		repoOwner:     "acme",
		repoName:      "app",
		repoFullName:  "acme/app",
		webhookSecret: "secret",
	})
	if err == nil {
		t.Fatal("determineInitialState(missing PAT token) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("PAT missing token status = %q, want %q", got, want)
	}
	if degradation == nil || degradation.Reason != bridgepkg.BridgeDegradationReasonAuthFailed {
		t.Fatalf("PAT missing token degradation = %#v, want auth_failed", degradation)
	}

	updated, status, degradation, err := provider.determineInitialState(ctx, &resolvedInstanceConfig{
		instanceID:    "brg-pat",
		mode:          githubModePAT,
		repoOwner:     "acme",
		repoName:      "app",
		repoFullName:  "acme/app",
		webhookSecret: "secret",
		token:         "ghp-token",
	})
	if err != nil {
		t.Fatalf("determineInitialState(valid PAT) error = %v", err)
	}
	if got, want := status, bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("valid PAT status = %q, want %q", got, want)
	}
	if degradation != nil {
		t.Fatalf("valid PAT degradation = %#v, want nil", degradation)
	}
	if got, want := updated.botLogin, "bridge-bot"; got != want {
		t.Fatalf("valid PAT bot login = %q, want %q", got, want)
	}

	_, status, degradation, err = provider.determineInitialState(ctx, &resolvedInstanceConfig{
		instanceID:    "brg-app",
		mode:          githubModeApp,
		repoOwner:     "acme",
		repoName:      "app",
		repoFullName:  "acme/app",
		webhookSecret: "secret",
	})
	if err == nil {
		t.Fatal("determineInitialState(missing app credentials) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("missing app credentials status = %q, want %q", got, want)
	}
	if degradation == nil || degradation.Reason != bridgepkg.BridgeDegradationReasonAuthFailed {
		t.Fatalf("missing app credentials degradation = %#v, want auth_failed", degradation)
	}

	privateKey := mustGitHubTestPrivateKey(t)
	updated, status, degradation, err = provider.determineInitialState(ctx, &resolvedInstanceConfig{
		instanceID:    "brg-app",
		mode:          githubModeApp,
		repoOwner:     "acme",
		repoName:      "app",
		repoFullName:  "acme/app",
		webhookSecret: "secret",
		appID:         "12345",
		privateKey:    privateKey,
	})
	if err != nil {
		t.Fatalf("determineInitialState(app without installation) error = %v", err)
	}
	if got, want := status, bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("app without installation status = %q, want %q", got, want)
	}
	if degradation != nil {
		t.Fatalf("app without installation degradation = %#v, want nil", degradation)
	}
	if got := updated.botLogin; got != "" {
		t.Fatalf("app without installation bot login = %q, want empty", got)
	}

	updated, status, degradation, err = provider.determineInitialState(ctx, &resolvedInstanceConfig{
		instanceID:     "brg-app",
		mode:           githubModeApp,
		repoOwner:      "acme",
		repoName:       "app",
		repoFullName:   "acme/app",
		webhookSecret:  "secret",
		appID:          "12345",
		privateKey:     privateKey,
		installationID: 9001,
	})
	if err != nil {
		t.Fatalf("determineInitialState(app with installation) error = %v", err)
	}
	if got, want := status, bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("app with installation status = %q, want %q", got, want)
	}
	if degradation != nil {
		t.Fatalf("app with installation degradation = %#v, want nil", degradation)
	}
	if got, want := updated.botLogin, "bridge-bot"; got != want {
		t.Fatalf("app with installation bot login = %q, want %q", got, want)
	}
}

func TestVerifyGitHubWebhookSignatureAndRouteSelection(t *testing.T) {
	t.Parallel()

	payload := githubIssuePayload{
		Action: "created",
		Comment: githubIssueComment{
			ID:        101,
			Body:      "hello",
			CreatedAt: "2026-04-15T21:05:00Z",
			User:      githubUser{ID: 1, Login: "alice", Type: "User"},
		},
		Issue: struct {
			Number      int64 `json:"number,omitempty"`
			PullRequest *struct {
				URL string `json:"url,omitempty"`
			} `json:"pull_request,omitempty"`
		}{Number: 42},
		Repository: githubRepository{
			Name:     "app",
			FullName: "acme/app",
			Owner:    githubUser{Login: "acme"},
		},
		Installation: &githubInstallation{ID: 9001},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	signature := signGitHubTestBody("super-secret", body)
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.test/github",
		strings.NewReader(string(body)),
	)
	req.Header.Set("X-Hub-Signature-256", signature)

	candidates := []resolvedInstanceConfig{
		{instanceID: "brg-a", repoFullName: "acme/app", webhookSecret: "super-secret", webhookPath: "/github"},
		{instanceID: "brg-b", repoFullName: "acme/other", webhookSecret: "super-secret", webhookPath: "/github"},
	}
	if err := verifyGitHubWebhookSignature(context.Background(), req, body, candidates); err != nil {
		t.Fatalf("verifyGitHubWebhookSignature() error = %v", err)
	}

	cfg, ok, err := selectGitHubIssueConfig(candidates, payload)
	if err != nil {
		t.Fatalf("selectGitHubIssueConfig() error = %v", err)
	}
	if !ok {
		t.Fatal("selectGitHubIssueConfig() ok = false, want true")
	}
	if got, want := cfg.instanceID, "brg-a"; got != want {
		t.Fatalf("selected instance id = %q, want %q", got, want)
	}

	badReq := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.test/github",
		strings.NewReader(string(body)),
	)
	badReq.Header.Set("X-Hub-Signature-256", "sha256=deadbeef")
	if err := verifyGitHubWebhookSignature(context.Background(), badReq, body, candidates); err == nil {
		t.Fatal("verifyGitHubWebhookSignature(invalid) error = nil, want non-nil")
	}
}

func TestGitHubProviderRejectsSharedPathWebhookSignedForDifferentInstance(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 21, 0, 0, 0, time.UTC)
	managed := []subprocess.InitializeBridgeManagedInstance{
		{Instance: bridgepkg.BridgeInstance{ID: "brg-github-1", Scope: bridgepkg.ScopeWorkspace, WorkspaceID: "ws-1"}},
		{Instance: bridgepkg.BridgeInstance{ID: "brg-github-2", Scope: bridgepkg.ScopeWorkspace, WorkspaceID: "ws-1"}},
	}
	ingested := make([]bridgepkg.InboundMessageEnvelope, 0)
	session := newGitHubTestSession(t, managed, func(_ context.Context, method string, params any, result any) error {
		switch method {
		case "bridges/messages/ingest":
			ingested = append(ingested, params.(bridgepkg.InboundMessageEnvelope))
			*(result.(*bridgepkg.BridgesMessagesIngestResult)) = bridgepkg.BridgesMessagesIngestResult{}
			return nil
		case "bridges/instances/report_state":
			report := params.(bridgepkg.BridgesInstancesReportStateParams)
			*(result.(*bridgepkg.BridgeInstance)) = bridgepkg.BridgeInstance{
				ID:     report.BridgeInstanceID,
				Status: report.Status,
			}
			return nil
		default:
			return errors.New("unexpected method: " + method)
		}
	})

	provider := newGitHubProviderForTest(t, session)
	provider.now = func() time.Time { return now }
	provider.routes.Replace(map[string]resolvedInstanceConfig{
		"brg-github-1": {
			managed:       managed[0],
			instanceID:    "brg-github-1",
			repoOwner:     "acme",
			repoName:      "app-one",
			repoFullName:  "acme/app-one",
			webhookPath:   "/github/shared",
			webhookSecret: "secret-one",
			botLogin:      "bridge-bot",
			dedup:         bridgesdk.NewDedupCache(5*time.Minute, 100),
		},
		"brg-github-2": {
			managed:       managed[1],
			instanceID:    "brg-github-2",
			repoOwner:     "acme",
			repoName:      "app-two",
			repoFullName:  "acme/app-two",
			webhookPath:   "/github/shared",
			webhookSecret: "secret-two",
			botLogin:      "bridge-bot",
			dedup:         bridgesdk.NewDedupCache(5*time.Minute, 100),
		},
	}, nil)
	provider.rateLimiter = bridgesdk.NewFixedWindowRateLimiter(20, time.Minute)
	provider.inFlightLimiter = bridgesdk.NewInFlightLimiter(4)

	body := mustJSON(t, githubIssuePayload{
		Action: "created",
		Comment: githubIssueComment{
			ID:        101,
			Body:      "Need a summary",
			CreatedAt: now.Format(time.RFC3339),
			User:      githubUser{ID: 1, Login: "alice", Type: "User"},
		},
		Issue: struct {
			Number      int64 `json:"number,omitempty"`
			PullRequest *struct {
				URL string `json:"url,omitempty"`
			} `json:"pull_request,omitempty"`
		}{
			Number: 42,
			PullRequest: &struct {
				URL string `json:"url,omitempty"`
			}{URL: "https://api.github.com/repos/acme/app-two/pulls/42"},
		},
		Repository:   githubRepository{Name: "app-two", FullName: "acme/app-two", Owner: githubUser{Login: "acme"}},
		Installation: &githubInstallation{ID: 9002},
		Sender:       githubUser{ID: 1, Login: "alice", Type: "User"},
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.test/github/shared",
		strings.NewReader(string(body)),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "issue_comment")
	req.Header.Set("X-Hub-Signature-256", signGitHubTestBody("secret-one", body))
	provider.serveWebhookHTTP(recorder, req)

	if got, want := recorder.Code, http.StatusUnauthorized; got != want {
		t.Fatalf("shared path webhook status = %d, want %d", got, want)
	}
	if got := len(ingested); got != 0 {
		t.Fatalf("len(ingested) = %d, want 0", got)
	}
}

func TestExecuteGitHubDeliveryIssueReviewDeleteAndResume(t *testing.T) {
	t.Parallel()

	cfg := resolvedInstanceConfig{
		instanceID:   "brg-github",
		mode:         githubModeApp,
		repoOwner:    "acme",
		repoName:     "app",
		repoFullName: "acme/app",
	}
	api := &fakeGitHubAPI{
		viewer:              &githubViewer{Login: "bridge-bot"},
		nextIssueCommentID:  500,
		nextReviewCommentID: 600,
	}

	startReq := bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "delivery-1",
			BridgeInstanceID: "brg-github",
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-github",
				BridgeInstanceID: "brg-github",
				GroupID:          "acme/app",
				ThreadID:         "github:acme/app:42",
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: "brg-github",
				GroupID:          "acme/app",
				ThreadID:         "github:acme/app:42",
				Mode:             bridgepkg.DeliveryModeReply,
			},
			Seq:       1,
			EventType: bridgepkg.DeliveryEventTypeStart,
			Content:   bridgepkg.MessageContent{Text: "hello"},
		},
	}
	startAck, state, err := executeGitHubDelivery(context.Background(), api, &cfg, startReq, deliveryState{}, 9001)
	if err != nil {
		t.Fatalf("executeGitHubDelivery(start issue) error = %v", err)
	}
	if got, want := startAck.RemoteMessageID, "issue:500"; got != want {
		t.Fatalf("start issue remote id = %q, want %q", got, want)
	}

	finalReq := startReq
	finalReq.Event.Seq = 2
	finalReq.Event.EventType = bridgepkg.DeliveryEventTypeFinal
	finalReq.Event.Final = true
	finalReq.Event.Content.Text = "hello world"
	finalAck, state, err := executeGitHubDelivery(context.Background(), api, &cfg, finalReq, state, 9001)
	if err != nil {
		t.Fatalf("executeGitHubDelivery(final issue) error = %v", err)
	}
	if got, want := finalAck.RemoteMessageID, "issue:500"; got != want {
		t.Fatalf("final issue remote id = %q, want %q", got, want)
	}
	if len(api.issueUpdates) != 1 {
		t.Fatalf("len(issueUpdates) = %d, want 1", len(api.issueUpdates))
	}

	deleteReq := finalReq
	deleteReq.Event.Seq = 3
	deleteReq.Event.EventType = bridgepkg.DeliveryEventTypeDelete
	deleteReq.Event.Operation = bridgepkg.DeliveryOperationDelete
	deleteReq.Event.Final = true
	deleteReq.Event.Reference = &bridgepkg.DeliveryMessageReference{RemoteMessageID: finalAck.RemoteMessageID}
	if _, _, err := executeGitHubDelivery(context.Background(), api, &cfg, deleteReq, state, 9001); err != nil {
		t.Fatalf("executeGitHubDelivery(delete issue) error = %v", err)
	}
	if got, want := api.deletedIssueCommentIDs, []int64{500}; !equalInt64s(got, want) {
		t.Fatalf("deleted issue ids = %#v, want %#v", got, want)
	}

	reviewReq := bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "delivery-2",
			BridgeInstanceID: "brg-github",
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-github",
				BridgeInstanceID: "brg-github",
				GroupID:          "acme/app",
				ThreadID:         "github:acme/app:42:rc:200",
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: "brg-github",
				GroupID:          "acme/app",
				ThreadID:         "github:acme/app:42:rc:200",
				Mode:             bridgepkg.DeliveryModeReply,
			},
			Seq:       1,
			EventType: bridgepkg.DeliveryEventTypeStart,
			Content:   bridgepkg.MessageContent{Text: "review reply"},
		},
	}
	reviewAck, _, err := executeGitHubDelivery(context.Background(), api, &cfg, reviewReq, deliveryState{}, 9002)
	if err != nil {
		t.Fatalf("executeGitHubDelivery(start review) error = %v", err)
	}
	if got, want := reviewAck.RemoteMessageID, "review:600"; got != want {
		t.Fatalf("start review remote id = %q, want %q", got, want)
	}

	resumeReq := reviewReq
	resumeReq.Event.DeliveryID = "delivery-3"
	resumeReq.Event.Seq = 1
	resumeReq.Event.EventType = bridgepkg.DeliveryEventTypeResume
	resumeReq.Event.Resume = &bridgepkg.DeliveryResumeState{LatestEventType: bridgepkg.DeliveryEventTypeFinal}
	resumeReq.Snapshot = &bridgepkg.DeliverySnapshot{
		DeliveryID:       "delivery-3",
		SessionID:        "sess-1",
		TurnID:           "turn-1",
		BridgeInstanceID: "brg-github",
		RoutingKey:       reviewReq.Event.RoutingKey,
		DeliveryTarget:   reviewReq.Event.DeliveryTarget,
		LatestSeq:        1,
		LatestEventType:  bridgepkg.DeliveryEventTypeFinal,
		CurrentContent:   bridgepkg.MessageContent{Text: "resume text"},
		RemoteMessageID:  reviewAck.RemoteMessageID,
		Final:            true,
		UpdatedAt:        time.Date(2026, 4, 15, 21, 10, 0, 0, time.UTC),
	}
	resumeAck, _, err := executeGitHubDelivery(context.Background(), api, &cfg, resumeReq, deliveryState{}, 9002)
	if err != nil {
		t.Fatalf("executeGitHubDelivery(resume review) error = %v", err)
	}
	if got, want := resumeAck.RemoteMessageID, "review:600"; got != want {
		t.Fatalf("resume review remote id = %q, want %q", got, want)
	}
	if len(api.reviewUpdates) == 0 {
		t.Fatal("reviewUpdates = 0, want at least 1")
	}
}

func TestGitHubClientPATAndAppRequests(t *testing.T) {
	t.Parallel()

	privateKey := mustGitHubTestPrivateKey(t)
	var mu sync.Mutex
	type recordedRequest struct {
		Method string
		Path   string
		Auth   string
		Body   string
	}
	requests := make([]recordedRequest, 0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, readErr := io.ReadAll(r.Body)
		closeErr := r.Body.Close()
		if closeErr != nil {
			t.Errorf("close GitHub API request body: %v", closeErr)
		}
		if readErr != nil {
			t.Errorf("read GitHub API request body: %v", readErr)
			http.Error(w, "read request body", http.StatusBadRequest)
			return
		}
		mu.Lock()
		requests = append(requests, recordedRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Auth:   r.Header.Get("Authorization"),
			Body:   string(bodyBytes),
		})
		mu.Unlock()

		switch r.URL.Path {
		case "/user":
			if _, err := io.WriteString(w, `{"id":1,"login":"bridge-bot"}`); err != nil {
				t.Errorf("write GitHub viewer response: %v", err)
			}
		case "/app/installations/9001/access_tokens":
			if _, err := io.WriteString(w, `{"token":"inst-token","expires_at":"2026-04-15T23:00:00Z"}`); err != nil {
				t.Errorf("write GitHub installation token response: %v", err)
			}
		case "/repos/acme/app/issues/42/comments":
			if _, err := io.WriteString(w, `{"id":501,"body":"hello"}`); err != nil {
				t.Errorf("write GitHub issue comment response: %v", err)
			}
		case "/repos/acme/app/pulls/42/comments/200/replies":
			if _, err := io.WriteString(w, `{"id":601,"body":"review"}`); err != nil {
				t.Errorf("write GitHub review reply response: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	patClient := &githubClient{
		cfg: resolvedInstanceConfig{
			mode:       githubModePAT,
			apiBaseURL: server.URL,
			repoOwner:  "acme",
			repoName:   "app",
			token:      "ghp-test",
		},
		httpClient: server.Client(),
		now:        func() time.Time { return time.Date(2026, 4, 15, 21, 0, 0, 0, time.UTC) },
	}
	viewer, err := patClient.ValidateAuth(context.Background(), 0)
	if err != nil {
		t.Fatalf("ValidateAuth(PAT) error = %v", err)
	}
	if got, want := viewer.Login, "bridge-bot"; got != want {
		t.Fatalf("PAT viewer login = %q, want %q", got, want)
	}

	if _, err := patClient.CreateIssueComment(context.Background(), 42, "hello", 0); err != nil {
		t.Fatalf("CreateIssueComment(PAT) error = %v", err)
	}

	appClient := &githubClient{
		cfg: resolvedInstanceConfig{
			mode:       githubModeApp,
			apiBaseURL: server.URL,
			repoOwner:  "acme",
			repoName:   "app",
			appID:      "12345",
			privateKey: privateKey,
		},
		httpClient: server.Client(),
		now:        func() time.Time { return time.Date(2026, 4, 15, 21, 0, 0, 0, time.UTC) },
	}
	if _, err := appClient.CreateReviewCommentReply(context.Background(), 42, 200, "review", 9001); err != nil {
		t.Fatalf("CreateReviewCommentReply(app) error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) < 4 {
		t.Fatalf("len(requests) = %d, want at least 4", len(requests))
	}
	if got, want := requests[0].Auth, "Bearer ghp-test"; got != want {
		t.Fatalf("PAT auth header = %q, want %q", got, want)
	}
	if got, want := requests[1].Path, "/repos/acme/app/issues/42/comments"; got != want {
		t.Fatalf("issue comment path = %q, want %q", got, want)
	}
	if got, want := requests[2].Path, "/app/installations/9001/access_tokens"; got != want {
		t.Fatalf("access token path = %q, want %q", got, want)
	}
	if !strings.HasPrefix(requests[2].Auth, "Bearer ") || requests[2].Auth == "Bearer " {
		t.Fatalf("app jwt auth header = %q, want non-empty bearer token", requests[2].Auth)
	}
	if got, want := requests[3].Auth, "Bearer inst-token"; got != want {
		t.Fatalf("installation auth header = %q, want %q", got, want)
	}
}

func TestGitHubClientRefreshesInstallationTokenWhenInstallationChanges(t *testing.T) {
	t.Run("Should request a new installation token when installation changes", func(t *testing.T) {
		t.Parallel()

		privateKey := mustGitHubTestPrivateKey(t)
		var mu sync.Mutex
		type recordedRequest struct {
			Path string
			Auth string
		}
		requests := make([]recordedRequest, 0, 4)

		client := &githubClient{
			cfg: resolvedInstanceConfig{
				mode:       githubModeApp,
				apiBaseURL: "https://api.github.test",
				appID:      "12345",
				privateKey: privateKey,
			},
			httpClient: githubHTTPDoerFunc(func(r *http.Request) (*http.Response, error) {
				mu.Lock()
				requests = append(requests, recordedRequest{
					Path: r.URL.Path,
					Auth: r.Header.Get("Authorization"),
				})
				mu.Unlock()

				body := ""
				switch r.URL.Path {
				case "/app/installations/9002/access_tokens":
					body = `{"token":"inst-token-9002","expires_at":"2026-04-15T23:00:00Z"}`
				case "/app/installations/9003/access_tokens":
					body = `{"token":"inst-token-9003","expires_at":"2026-04-15T23:00:00Z"}`
				case "/user":
					body = `{"id":1,"login":"bridge-bot"}`
				default:
					return &http.Response{
						StatusCode: http.StatusNotFound,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader("not found")),
					}, nil
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			}),
			now: func() time.Time { return time.Date(2026, 4, 15, 21, 0, 0, 0, time.UTC) },
		}

		if _, err := client.ValidateAuth(context.Background(), 9002); err != nil {
			t.Fatalf("ValidateAuth(9002) error = %v", err)
		}
		if _, err := client.ValidateAuth(context.Background(), 9003); err != nil {
			t.Fatalf("ValidateAuth(9003) error = %v", err)
		}

		mu.Lock()
		defer mu.Unlock()
		if got, want := len(requests), 4; got != want {
			t.Fatalf("len(requests) = %d, want %d", got, want)
		}
		if got, want := requests[0].Path, "/app/installations/9002/access_tokens"; got != want {
			t.Fatalf("first access token path = %q, want %q", got, want)
		}
		if got, want := requests[1].Auth, "Bearer inst-token-9002"; got != want {
			t.Fatalf("first user auth header = %q, want %q", got, want)
		}
		if got, want := requests[2].Path, "/app/installations/9003/access_tokens"; got != want {
			t.Fatalf("second access token path = %q, want %q", got, want)
		}
		if got, want := requests[3].Auth, "Bearer inst-token-9003"; got != want {
			t.Fatalf("second user auth header = %q, want %q", got, want)
		}
	})
}

func TestGitHubClientClassifiesHTTPFailures(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			http.Error(w, `{"message":"rate limit exceeded"}`, http.StatusTooManyRequests)
		case "/repos/acme/app/issues/42/comments":
			http.Error(w, `{"message":"bad request"}`, http.StatusUnprocessableEntity)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &githubClient{
		cfg: resolvedInstanceConfig{
			mode:       githubModePAT,
			apiBaseURL: server.URL,
			repoOwner:  "acme",
			repoName:   "app",
			token:      "ghp-test",
		},
		httpClient: server.Client(),
		now:        func() time.Time { return time.Date(2026, 4, 15, 21, 0, 0, 0, time.UTC) },
	}

	if _, err := client.ValidateAuth(context.Background(), 0); err == nil {
		t.Fatal("ValidateAuth(rate limited) error = nil, want non-nil")
	} else {
		var rateErr *bridgesdk.RateLimitError
		if !errors.As(err, &rateErr) {
			t.Fatalf("ValidateAuth() error = %#v, want rate limit error", err)
		}
	}

	if _, err := client.CreateIssueComment(context.Background(), 42, "oops", 0); err == nil {
		t.Fatal("CreateIssueComment(422) error = nil, want non-nil")
	} else {
		var permanentErr *bridgesdk.PermanentError
		if !errors.As(err, &permanentErr) {
			t.Fatalf("CreateIssueComment() error = %#v, want permanent error", err)
		}
		if committedErr, ok := errors.AsType[*bridgesdk.CommittedMutationError](err); ok && committedErr != nil {
			t.Fatalf("CreateIssueComment() error = %#v, want pre-commit rejection", err)
		}
	}

	for _, testCase := range []struct {
		name       string
		statusCode int
	}{
		{name: "Should refuse a credentialed 301 redirect", statusCode: http.StatusMovedPermanently},
		{name: "Should refuse a credentialed 302 redirect", statusCode: http.StatusFound},
		{name: "Should refuse a credentialed 303 redirect", statusCode: http.StatusSeeOther},
		{name: "Should refuse a credentialed 307 redirect", statusCode: http.StatusTemporaryRedirect},
		{name: "Should refuse a credentialed 308 redirect", statusCode: http.StatusPermanentRedirect},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var targetHits atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				targetHits.Add(1)
				if err := json.NewEncoder(w).Encode(githubIssueComment{ID: 901}); err != nil {
					t.Errorf("encode redirect target GitHub response: %v", err)
				}
			}))
			defer target.Close()

			redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", target.URL)
				w.WriteHeader(testCase.statusCode)
				if _, err := io.WriteString(w, `{"message":"redirect refused"}`); err != nil {
					t.Errorf("write redirect GitHub response: %v", err)
				}
				if got, want := r.Method, http.MethodPost; got != want {
					t.Errorf("redirect source method = %q, want %q", got, want)
				}
			}))
			defer redirect.Close()

			client := &githubClient{
				cfg: resolvedInstanceConfig{
					mode:       githubModePAT,
					apiBaseURL: redirect.URL,
					repoOwner:  "acme",
					repoName:   "app",
					token:      "ghp-test",
				},
				httpClient: redirect.Client(),
			}
			_, err := client.CreateIssueComment(t.Context(), 42, "hello", 0)
			var permanentErr *bridgesdk.PermanentError
			if !errors.As(err, &permanentErr) {
				t.Fatalf(
					"CreateIssueComment(%d) error = %T %v, want first-response PermanentError",
					testCase.statusCode,
					err,
					err,
				)
			}
			if bridgesdk.IsCommittedMutation(err) {
				t.Fatalf("CreateIssueComment(%d) error = %v, want pre-commit rejection", testCase.statusCode, err)
			}
			if got := targetHits.Load(); got != 0 {
				t.Fatalf("redirect target hits = %d, want 0", got)
			}
		})
	}

	t.Run("Should preserve rate-limit classification when the error body read fails", func(t *testing.T) {
		t.Parallel()

		readErr := errors.New("GitHub error body read failed")
		body := &githubPartialErrorResponseBody{
			content: []byte(`{"message":"GitHub rate limit exceeded"}`),
			readErr: readErr,
		}
		client := &githubClient{
			cfg: resolvedInstanceConfig{
				mode:       githubModePAT,
				apiBaseURL: "https://api.github.test",
				repoOwner:  "acme",
				repoName:   "app",
				token:      "ghp-test",
			},
			httpClient: githubHTTPDoerFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header:     http.Header{"Retry-After": []string{"4"}},
					Body:       body,
					Request:    req,
				}, nil
			}),
		}

		_, err := client.CreateIssueComment(t.Context(), 42, "hello", 0)
		if !errors.Is(err, readErr) {
			t.Fatalf("CreateIssueComment() error = %v, want response body read failure", err)
		}
		var rateLimitErr *bridgesdk.RateLimitError
		if !errors.As(err, &rateLimitErr) || rateLimitErr.RetryAfter != 4*time.Second {
			t.Fatalf("CreateIssueComment() error = %v, want rate limit with four-second retry", err)
		}
		if got, want := bridgesdk.ClassifyError(err).Class, bridgesdk.ErrorClassRateLimit; got != want {
			t.Fatalf("CreateIssueComment() error class = %q, want %q", got, want)
		}
		if !strings.Contains(rateLimitErr.Error(), "GitHub rate limit exceeded") {
			t.Fatalf("CreateIssueComment() rate-limit error = %q, want partial provider body", rateLimitErr.Error())
		}
		if bridgesdk.IsCommittedMutation(err) {
			t.Fatalf("CreateIssueComment() error = %v, want pre-commit rejection", err)
		}
		if !body.closed {
			t.Fatal("response body closed = false after error classification")
		}
	})

	for _, testCase := range []struct {
		name string
		body string
	}{
		{name: "Should not retry a committed create with truncated JSON", body: `{"id":`},
		{name: "Should not retry a committed create without a comment id", body: `{}`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			attempts := 0
			committedClient := &githubClient{
				cfg: resolvedInstanceConfig{
					mode:       githubModePAT,
					apiBaseURL: "https://api.github.test",
					repoOwner:  "acme",
					repoName:   "app",
					token:      "ghp-test",
				},
				httpClient: githubHTTPDoerFunc(func(*http.Request) (*http.Response, error) {
					attempts++
					return &http.Response{
						StatusCode: http.StatusCreated,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(testCase.body)),
					}, nil
				}),
			}

			_, err := committedClient.CreateIssueComment(context.Background(), 42, "hello", 0)
			if err == nil {
				t.Fatal("CreateIssueComment(committed result unavailable) error = nil, want non-nil")
			}
			var committedErr *bridgesdk.CommittedMutationError
			if !errors.As(err, &committedErr) {
				t.Fatalf("CreateIssueComment() error = %T, want CommittedMutationError", err)
			}
			if got, want := attempts, 1; got != want {
				t.Fatalf("mutation attempts = %d, want %d", got, want)
			}
		})
	}
}

func TestGitHubProviderAfterInitializeSyncsOwnedInstancesAndReportsState(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 21, 20, 0, 0, time.UTC)
	privateKey := mustGitHubTestPrivateKey(t)
	managed := []subprocess.InitializeBridgeManagedInstance{
		{
			Instance: bridgepkg.BridgeInstance{
				ID:          "brg-github-1",
				Scope:       bridgepkg.ScopeWorkspace,
				WorkspaceID: "ws-1",
				DMPolicy:    bridgepkg.BridgeDMPolicyOpen,
				ProviderConfig: mustJSON(t, map[string]any{
					"mode": githubModePAT,
					"repository": map[string]any{
						"full_name": "acme/app-one",
					},
					"webhook": map[string]any{
						"listen_addr": "127.0.0.1:0",
						"path":        "/github/app-one",
					},
				}),
			},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "webhook_secret", Value: "secret"},
				{BindingName: "token", Value: "ghp-token"},
			},
		},
		{
			Instance: bridgepkg.BridgeInstance{
				ID:          "brg-github-2",
				Scope:       bridgepkg.ScopeWorkspace,
				WorkspaceID: "ws-2",
				DMPolicy:    bridgepkg.BridgeDMPolicyOpen,
				ProviderConfig: mustJSON(t, map[string]any{
					"mode":            githubModeApp,
					"installation_id": 9002,
					"repository": map[string]any{
						"full_name": "acme/app-two",
					},
					"webhook": map[string]any{
						"listen_addr": "127.0.0.1:0",
						"path":        "/github/app-two",
					},
				}),
			},
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
				{BindingName: "webhook_secret", Value: "secret"},
				{BindingName: "app_id", Value: "12345"},
				{BindingName: "private_key", Value: privateKey},
			},
		},
	}

	reported := make([]bridgepkg.BridgesInstancesReportStateParams, 0)
	session := newGitHubTestSession(t, managed, func(_ context.Context, method string, params any, result any) error {
		switch method {
		case "bridges/instances/list":
			items := make([]bridgepkg.BridgeInstance, 0, len(managed))
			for _, item := range managed {
				items = append(items, item.Instance)
			}
			target := result.(*[]bridgepkg.BridgeInstance)
			*target = items
			return nil
		case "bridges/instances/get":
			targetParams := params.(bridgepkg.BridgeInstanceTargetParams)
			for _, item := range managed {
				if item.Instance.ID == targetParams.BridgeInstanceID {
					*(result.(*bridgepkg.BridgeInstance)) = item.Instance
					return nil
				}
			}
			return errors.New("missing instance")
		case "bridges/instances/report_state":
			report := params.(bridgepkg.BridgesInstancesReportStateParams)
			reported = append(reported, report)
			*(result.(*bridgepkg.BridgeInstance)) = bridgepkg.BridgeInstance{
				ID:            report.BridgeInstanceID,
				Status:        report.Status,
				Platform:      "github",
				ExtensionName: "github",
				DisplayName:   "GitHub",
				Scope:         bridgepkg.ScopeWorkspace,
				WorkspaceID:   "ws",
			}
			return nil
		default:
			return errors.New("unexpected method: " + method)
		}
	})

	provider, err := newGitHubProvider(io.Discard)
	if err != nil {
		t.Fatalf("newGitHubProvider() error = %v", err)
	}
	provider.now = func() time.Time { return now }
	provider.apiFactory = func(cfg resolvedInstanceConfig) githubAPI {
		return &fakeGitHubAPI{viewer: &githubViewer{Login: cfg.repoName + "-bot"}}
	}
	t.Cleanup(func() {
		provider.lifecycle.Stop()
		if err := provider.http.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})

	if err := provider.lifecycle.Initialize(context.Background(), session); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	select {
	case <-provider.lifecycle.Initialized():
	case <-t.Context().Done():
		t.Fatal("provider initialization did not finish")
	}

	if got, want := len(provider.routes.Snapshot()), 2; got != want {
		t.Fatalf("len(provider.routes) = %d, want %d", got, want)
	}
	if got, want := len(reported), 2; got != want {
		t.Fatalf("len(reported) = %d, want %d", got, want)
	}
	if provider.http.Address() == "" {
		t.Fatal("provider HTTP address is empty after initialization")
	}
	if cfg, ok := provider.routes.Get("brg-github-2"); !ok {
		t.Fatal("provider.routes missing brg-github-2")
	} else if got, want := cfg.botLogin, "app-two-bot"; got != want {
		t.Fatalf("resolved bot login = %q, want %q", got, want)
	}
}

func TestGitHubProviderServeWebhookHTTPSharedEndpointIngestsMultipleInstances(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 21, 25, 0, 0, time.UTC)
	managed := []subprocess.InitializeBridgeManagedInstance{
		{
			Instance: bridgepkg.BridgeInstance{
				ID:          "brg-github-1",
				Scope:       bridgepkg.ScopeWorkspace,
				WorkspaceID: "ws-1",
			},
		},
		{
			Instance: bridgepkg.BridgeInstance{
				ID:          "brg-github-2",
				Scope:       bridgepkg.ScopeWorkspace,
				WorkspaceID: "ws-2",
			},
		},
	}

	ingested := make([]bridgepkg.InboundMessageEnvelope, 0)
	reported := make([]bridgepkg.BridgesInstancesReportStateParams, 0)
	session := newGitHubTestSession(t, managed, func(_ context.Context, method string, params any, result any) error {
		switch method {
		case "bridges/messages/ingest":
			envelope := params.(bridgepkg.InboundMessageEnvelope)
			ingested = append(ingested, envelope)
			*(result.(*bridgepkg.BridgesMessagesIngestResult)) = bridgepkg.BridgesMessagesIngestResult{
				SessionID: "sess-" + envelope.BridgeInstanceID,
				RoutingKey: bridgepkg.RoutingKey{
					Scope:            envelope.Scope,
					WorkspaceID:      envelope.WorkspaceID,
					BridgeInstanceID: envelope.BridgeInstanceID,
					GroupID:          envelope.GroupID,
					ThreadID:         envelope.ThreadID,
				},
			}
			return nil
		case "bridges/instances/report_state":
			report := params.(bridgepkg.BridgesInstancesReportStateParams)
			reported = append(reported, report)
			*(result.(*bridgepkg.BridgeInstance)) = bridgepkg.BridgeInstance{
				ID:     report.BridgeInstanceID,
				Status: report.Status,
			}
			return nil
		default:
			return errors.New("unexpected method: " + method)
		}
	})

	provider := newGitHubProviderForTest(t, session)
	provider.now = func() time.Time { return now }
	provider.routes.Replace(map[string]resolvedInstanceConfig{
		"brg-github-1": {
			managed:       managed[0],
			instanceID:    "brg-github-1",
			repoOwner:     "acme",
			repoName:      "app-one",
			repoFullName:  "acme/app-one",
			webhookPath:   "/github/app-one",
			webhookSecret: "secret",
			botLogin:      "bridge-bot",
			dedup:         bridgesdk.NewDedupCache(5*time.Minute, 100),
		},
		"brg-github-2": {
			managed:       managed[1],
			instanceID:    "brg-github-2",
			repoOwner:     "acme",
			repoName:      "app-two",
			repoFullName:  "acme/app-two",
			webhookPath:   "/github/app-two",
			webhookSecret: "secret",
			botLogin:      "bridge-bot",
			dedup:         bridgesdk.NewDedupCache(5*time.Minute, 100),
		},
	}, nil)
	provider.rateLimiter = bridgesdk.NewFixedWindowRateLimiter(20, time.Minute)
	provider.inFlightLimiter = bridgesdk.NewInFlightLimiter(4)

	first := mustJSON(t, githubIssuePayload{
		Action: "created",
		Comment: githubIssueComment{
			ID:        101,
			Body:      "Need a summary",
			CreatedAt: now.Format(time.RFC3339),
			User:      githubUser{ID: 1, Login: "alice", Type: "User"},
		},
		Issue: struct {
			Number      int64 `json:"number,omitempty"`
			PullRequest *struct {
				URL string `json:"url,omitempty"`
			} `json:"pull_request,omitempty"`
		}{
			Number: 42,
			PullRequest: &struct {
				URL string `json:"url,omitempty"`
			}{URL: "https://api.github.com/repos/acme/app-one/pulls/42"},
		},
		Repository:   githubRepository{Name: "app-one", FullName: "acme/app-one", Owner: githubUser{Login: "acme"}},
		Installation: &githubInstallation{ID: 9001},
		Sender:       githubUser{ID: 1, Login: "alice", Type: "User"},
	})
	second := mustJSON(t, githubReviewPayload{
		Action: "created",
		Comment: githubReviewComment{
			ID:        301,
			Body:      "Line comment",
			Path:      "main.go",
			CreatedAt: now.Format(time.RFC3339),
			User:      githubUser{ID: 2, Login: "bob", Type: "User"},
		},
		PullRequest: struct {
			Number int64 `json:"number,omitempty"`
		}{Number: 7},
		Repository:   githubRepository{Name: "app-two", FullName: "acme/app-two", Owner: githubUser{Login: "acme"}},
		Installation: &githubInstallation{ID: 9002},
		Sender:       githubUser{ID: 2, Login: "bob", Type: "User"},
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.test/github/app-one",
		strings.NewReader(string(first)),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "issue_comment")
	req.Header.Set("X-Hub-Signature-256", signGitHubTestBody("secret", first))
	provider.serveWebhookHTTP(recorder, req)
	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("issue webhook status = %d, want %d", got, want)
	}

	recorder = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.test/github/app-two",
		strings.NewReader(string(second)),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "pull_request_review_comment")
	req.Header.Set("X-Hub-Signature-256", signGitHubTestBody("secret", second))
	provider.serveWebhookHTTP(recorder, req)
	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("review webhook status = %d, want %d", got, want)
	}

	if got, want := len(ingested), 2; got != want {
		t.Fatalf("len(ingested) = %d, want %d", got, want)
	}
	if got, want := ingested[0].BridgeInstanceID, "brg-github-1"; got != want {
		t.Fatalf("first ingest instance = %q, want %q", got, want)
	}
	if got, want := ingested[1].BridgeInstanceID, "brg-github-2"; got != want {
		t.Fatalf("second ingest instance = %q, want %q", got, want)
	}
	if got, want := provider.cachedInstallationID("acme/app-two"), int64(9002); got != want {
		t.Fatalf("cached installation id = %d, want %d", got, want)
	}
	if len(reported) == 0 {
		t.Fatal("reported state transitions = 0, want at least 1 ready report")
	}
}

func TestGitHubProviderDefaultAPIFactoryReusesClientPerInstance(t *testing.T) {
	provider, err := newGitHubProvider(io.Discard)
	if err != nil {
		t.Fatalf("newGitHubProvider() error = %v", err)
	}

	cfg := resolvedInstanceConfig{
		instanceID:   "brg-github",
		apiBaseURL:   githubDefaultAPIBaseURL,
		mode:         githubModeApp,
		repoOwner:    "acme",
		repoName:     "app",
		repoFullName: "acme/app",
	}
	first := provider.apiFactory(cfg)
	second := provider.apiFactory(cfg)
	if first != second {
		t.Fatal("apiFactory() returned different clients for the same instance, want reuse")
	}

	other := provider.apiFactory(resolvedInstanceConfig{
		instanceID:   "brg-github-2",
		apiBaseURL:   githubDefaultAPIBaseURL,
		mode:         githubModeApp,
		repoOwner:    "acme",
		repoName:     "other",
		repoFullName: "acme/other",
	})
	if first == other {
		t.Fatal("apiFactory() reused a client across different instances, want per-instance isolation")
	}
}

func TestGitHubProviderReconcileAllowsSharedWebhookPaths(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv(githubListenAddrEnv, "127.0.0.1:0")
	t.Setenv(bridgesdk.AdapterStartsPathEnv, filepath.Join(tmpDir, "starts.log"))

	provider, err := newGitHubProvider(io.Discard)
	if err != nil {
		t.Fatalf("newGitHubProvider() error = %v", err)
	}
	t.Cleanup(func() {
		provider.lifecycle.Stop()
		if err := provider.http.Shutdown(context.Background()); err != nil {
			t.Errorf("provider HTTP shutdown error = %v", err)
		}
	})
	provider.apiFactory = func(cfg resolvedInstanceConfig) githubAPI {
		return &fakeGitHubAPI{viewer: &githubViewer{Login: cfg.repoName + "-bot"}}
	}

	session := newGitHubTestSession(t, nil, func(_ context.Context, method string, _ any, result any) error {
		switch method {
		case "bridges/instances/list":
			*(result.(*[]bridgepkg.BridgeInstance)) = nil
			return nil
		case "bridges/instances/get":
			*(result.(*bridgepkg.BridgeInstance)) = bridgepkg.BridgeInstance{}
			return nil
		case "bridges/instances/report_state":
			*(result.(*bridgepkg.BridgeInstance)) = bridgepkg.BridgeInstance{}
			return nil
		default:
			return errors.New("unexpected method: " + method)
		}
	})

	first := subprocess.InitializeBridgeManagedInstance{
		Instance: bridgepkg.BridgeInstance{
			ID:          "brg-github-1",
			Scope:       bridgepkg.ScopeWorkspace,
			WorkspaceID: "ws-1",
			ProviderConfig: mustJSON(t, map[string]any{
				"mode": "pat",
				"webhook": map[string]any{
					"path": "/shared",
				},
				"repository": map[string]any{
					"full_name": "acme/app-one",
				},
			}),
		},
		BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
			{BindingName: "webhook_secret", Value: "secret-one"},
			{BindingName: "token", Value: "ghp-one"},
		},
	}
	second := subprocess.InitializeBridgeManagedInstance{
		Instance: bridgepkg.BridgeInstance{
			ID:          "brg-github-2",
			Scope:       bridgepkg.ScopeWorkspace,
			WorkspaceID: "ws-1",
			ProviderConfig: mustJSON(t, map[string]any{
				"mode": "pat",
				"webhook": map[string]any{
					"path": "/shared",
				},
				"repository": map[string]any{
					"full_name": "acme/app-two",
				},
			}),
		},
		BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
			{BindingName: "webhook_secret", Value: "secret-two"},
			{BindingName: "token", Value: "ghp-two"},
		},
	}

	configs := provider.reconcileInstanceConfigs(
		context.Background(),
		session,
		[]subprocess.InitializeBridgeManagedInstance{first, second},
	)
	if got, want := len(configs), 2; got != want {
		t.Fatalf("len(configs) = %d, want %d", got, want)
	}
	if configs[0].configError != nil {
		t.Fatalf("configs[0].configError = %v, want nil for shared webhook path", configs[0].configError)
	}
	if configs[1].configError != nil {
		t.Fatalf("configs[1].configError = %v, want nil for shared webhook path", configs[1].configError)
	}

	duplicateRepository := second
	duplicateRepository.Instance.ID = "brg-github-duplicate"
	duplicateRepository.Instance.ProviderConfig = mustJSON(t, map[string]any{
		"mode": "pat",
		"webhook": map[string]any{
			"path": "/duplicate-repository",
		},
		"repository": map[string]any{
			"full_name": "acme/app-one",
		},
	})
	configs = provider.reconcileInstanceConfigs(
		context.Background(),
		session,
		[]subprocess.InitializeBridgeManagedInstance{first, duplicateRepository},
	)
	if got := configs[1].configError; got == nil || !strings.Contains(got.Error(), "already owned") {
		t.Fatalf("duplicate repository configError = %v, want ownership conflict", got)
	}
}

func TestGitHubProviderHandleBridgesDeliverReportsReadyAndErrors(t *testing.T) {
	t.Parallel()

	managed := []subprocess.InitializeBridgeManagedInstance{
		{
			Instance: bridgepkg.BridgeInstance{
				ID:          "brg-github",
				Scope:       bridgepkg.ScopeWorkspace,
				WorkspaceID: "ws-1",
			},
		},
	}
	reported := make([]bridgepkg.BridgesInstancesReportStateParams, 0)
	session := newGitHubTestSession(t, managed, func(_ context.Context, method string, params any, result any) error {
		if method != "bridges/instances/report_state" {
			return errors.New("unexpected method: " + method)
		}
		report := params.(bridgepkg.BridgesInstancesReportStateParams)
		reported = append(reported, report)
		*(result.(*bridgepkg.BridgeInstance)) = bridgepkg.BridgeInstance{
			ID:     report.BridgeInstanceID,
			Status: report.Status,
		}
		return nil
	})

	successAPI := &fakeGitHubAPI{nextIssueCommentID: 800}
	provider := newGitHubProviderForTest(t, session)
	provider.now = func() time.Time { return time.Date(2026, 4, 15, 21, 30, 0, 0, time.UTC) }
	provider.routes.Replace(map[string]resolvedInstanceConfig{
		"brg-github": {
			managed:       managed[0],
			instanceID:    "brg-github",
			mode:          githubModePAT,
			repoOwner:     "acme",
			repoName:      "app",
			repoFullName:  "acme/app",
			webhookSecret: "secret",
			token:         "ghp-token",
		},
	}, nil)
	provider.apiFactory = func(resolvedInstanceConfig) githubAPI { return successAPI }

	req := bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "delivery-1",
			BridgeInstanceID: "brg-github",
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-1",
				BridgeInstanceID: "brg-github",
				GroupID:          "acme/app",
				ThreadID:         "github:acme/app:42",
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: "brg-github",
				GroupID:          "acme/app",
				ThreadID:         "github:acme/app:42",
				Mode:             bridgepkg.DeliveryModeReply,
			},
			Seq:       1,
			EventType: bridgepkg.DeliveryEventTypeStart,
			Content:   bridgepkg.MessageContent{Text: "hello"},
		},
	}
	ack, err := provider.handleBridgesDeliver(context.Background(), session, req)
	if err != nil {
		t.Fatalf("handleBridgesDeliver(success) error = %v", err)
	}
	if got, want := ack.RemoteMessageID, "issue:800"; got != want {
		t.Fatalf("success remote id = %q, want %q", got, want)
	}
	if len(reported) == 0 || reported[len(reported)-1].Status != bridgepkg.BridgeStatusReady {
		t.Fatalf("reported statuses = %#v, want trailing ready state", reported)
	}

	errorProvider := newGitHubProviderForTest(t, session)
	errorProvider.now = provider.now
	errorProvider.routes.Replace(provider.routes.Snapshot(), nil)
	errorProvider.apiFactory = func(resolvedInstanceConfig) githubAPI {
		return &fakeGitHubErrorAPI{err: &bridgesdk.AuthError{Err: errors.New("bad auth")}}
	}
	if _, err := errorProvider.handleBridgesDeliver(context.Background(), session, req); err == nil {
		t.Fatal("handleBridgesDeliver(error) error = nil, want non-nil")
	}
	if got, want := errorProvider.lastError, "bad auth"; !strings.Contains(got, want) {
		t.Fatalf("lastError = %q, want substring %q", got, want)
	}

	t.Run("Should evict delivery state after a committed provider mutation", func(t *testing.T) {
		t.Parallel()

		committedAPI := &fakeGitHubAPI{
			updateErr: &bridgesdk.CommittedMutationError{Err: errors.New("github result unavailable")},
		}
		committedProvider := newGitHubProviderForTest(t, nil)
		committedProvider.routes.Replace(provider.routes.Snapshot(), nil)
		committedProvider.apiFactory = func(resolvedInstanceConfig) githubAPI { return committedAPI }

		committedRequest := req
		committedRequest.Event.DeliveryID = "delivery-committed"
		committedRequest.Event.Seq = 2
		committedRequest.Event.EventType = bridgepkg.DeliveryEventTypeFinal
		committedRequest.Event.Final = true
		key := deliveryStateKey("brg-github", committedRequest.Event.DeliveryID)
		committedProvider.deliveries.Store(key, deliveryState{
			LastSeq:         1,
			RemoteMessageID: "issue:700",
		})

		_, err := committedProvider.handleBridgesDeliver(context.Background(), nil, committedRequest)
		var committedErr *bridgesdk.CommittedMutationError
		if !errors.As(err, &committedErr) {
			t.Fatalf("handleBridgesDeliver() error = %T, want CommittedMutationError", err)
		}
		if got, want := len(committedAPI.issueUpdates), 1; got != want {
			t.Fatalf("provider mutations = %d, want %d", got, want)
		}
		if state, ok := committedProvider.deliveries.Load(key); ok {
			t.Fatalf("deliveries[%q] = %#v, want evicted", key, state)
		}
	})
}

func TestGitHubUtilityHelpers(t *testing.T) {
	t.Parallel()

	if got, want := installationIDFromMetadata(
		mustJSON(t, map[string]any{"installation_id": 77}),
	), int64(
		77,
	); got != want {
		t.Fatalf("installationIDFromMetadata(valid) = %d, want %d", got, want)
	}
	if got := installationIDFromMetadata(json.RawMessage(`{`)); got != 0 {
		t.Fatalf("installationIDFromMetadata(invalid) = %d, want 0", got)
	}

	fallback := time.Date(2026, 4, 15, 21, 40, 0, 0, time.UTC)
	if got := normalizeGitHubReceivedAt(
		fallback,
		"2026-04-15T21:41:00Z",
	); !got.Equal(
		time.Date(2026, 4, 15, 21, 41, 0, 0, time.UTC),
	) {
		t.Fatalf("normalizeGitHubReceivedAt(valid) = %s, want parsed time", got)
	}
	if got := normalizeGitHubReceivedAt(fallback, "not-a-time"); !got.Equal(fallback) {
		t.Fatalf("normalizeGitHubReceivedAt(fallback) = %s, want %s", got, fallback)
	}
	if got := normalizeGitHubReceivedAt(time.Time{}, "not-a-time"); got.IsZero() {
		t.Fatal("normalizeGitHubReceivedAt(zero fallback) returned zero time")
	}

	if owner, name, fullName, err := normalizeGitHubRepository("", "", " acme/app "); err != nil {
		t.Fatalf("normalizeGitHubRepository(full_name) error = %v", err)
	} else if owner != "acme" || name != "app" || fullName != "acme/app" {
		t.Fatalf("normalizeGitHubRepository(full_name) = %q/%q/%q, want acme/app/acme/app", owner, name, fullName)
	}
	if owner, name, fullName, err := normalizeGitHubRepository("acme", "app", ""); err != nil {
		t.Fatalf("normalizeGitHubRepository(owner,name) error = %v", err)
	} else if owner != "acme" || name != "app" || fullName != "acme/app" {
		t.Fatalf("normalizeGitHubRepository(owner,name) = %q/%q/%q, want acme/app/acme/app", owner, name, fullName)
	}
	if _, _, _, err := normalizeGitHubRepository("", "", "bad"); err == nil {
		t.Fatal("normalizeGitHubRepository(invalid full_name) error = nil, want non-nil")
	}
	if _, _, _, err := normalizeGitHubRepository("", "", ""); err == nil {
		t.Fatal("normalizeGitHubRepository(missing repo) error = nil, want non-nil")
	}

	if ref, err := parseGitHubRemoteCommentRef("issue:123"); err != nil {
		t.Fatalf("parseGitHubRemoteCommentRef(issue) error = %v", err)
	} else if ref.Kind != "issue" || ref.CommentID != 123 {
		t.Fatalf("issue ref = %#v, want issue/123", ref)
	}
	if ref, err := parseGitHubRemoteCommentRef("review:456"); err != nil {
		t.Fatalf("parseGitHubRemoteCommentRef(review) error = %v", err)
	} else if ref.Kind != "review" || ref.CommentID != 456 {
		t.Fatalf("review ref = %#v, want review/456", ref)
	}
	if _, err := parseGitHubRemoteCommentRef(""); err == nil {
		t.Fatal("parseGitHubRemoteCommentRef(empty) error = nil, want non-nil")
	}
	if _, err := parseGitHubRemoteCommentRef("note:1"); err == nil {
		t.Fatal("parseGitHubRemoteCommentRef(bad kind) error = nil, want non-nil")
	}
}

func TestGitHubClientUpdateDeleteAndCredentialValidation(t *testing.T) {
	t.Parallel()

	privateKey := mustGitHubTestPrivateKey(t)
	var mu sync.Mutex
	requestCounts := map[string]int{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCounts[r.Method+" "+r.URL.Path]++
		mu.Unlock()

		switch r.URL.Path {
		case "/repos/acme/app/issues/comments/700":
			if r.Method == http.MethodPatch {
				if _, err := io.WriteString(w, `{}`); err != nil {
					t.Errorf("write GitHub issue update response: %v", err)
				}
				return
			}
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		case "/app/installations/9001/access_tokens":
			if _, err := io.WriteString(w, `{"token":"inst-token","expires_at":"2026-04-15T23:00:00Z"}`); err != nil {
				t.Errorf("write refreshed GitHub installation token response: %v", err)
			}
			return
		case "/repos/acme/app/pulls/comments/800":
			if r.Method == http.MethodPatch {
				if _, err := io.WriteString(w, `{}`); err != nil {
					t.Errorf("write GitHub review update response: %v", err)
				}
				return
			}
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	patClient := &githubClient{
		cfg: resolvedInstanceConfig{
			mode:       githubModePAT,
			apiBaseURL: server.URL,
			repoOwner:  "acme",
			repoName:   "app",
			token:      "ghp-token",
		},
		httpClient: server.Client(),
		now:        func() time.Time { return time.Date(2026, 4, 15, 21, 0, 0, 0, time.UTC) },
	}
	issueComment, err := patClient.UpdateIssueComment(context.Background(), 700, "updated", 0)
	if err != nil {
		t.Fatalf("UpdateIssueComment() error = %v", err)
	}
	if got, want := issueComment.ID, int64(700); got != want {
		t.Fatalf("UpdateIssueComment id = %d, want %d", got, want)
	}
	if err := patClient.DeleteIssueComment(context.Background(), 700, 0); err != nil {
		t.Fatalf("DeleteIssueComment() error = %v", err)
	}

	appClient := &githubClient{
		cfg: resolvedInstanceConfig{
			mode:       githubModeApp,
			apiBaseURL: server.URL,
			repoOwner:  "acme",
			repoName:   "app",
			appID:      "12345",
			privateKey: privateKey,
		},
		httpClient: server.Client(),
		now:        func() time.Time { return time.Date(2026, 4, 15, 21, 0, 0, 0, time.UTC) },
	}
	reviewComment, err := appClient.UpdateReviewComment(context.Background(), 800, "updated", 9001)
	if err != nil {
		t.Fatalf("UpdateReviewComment() error = %v", err)
	}
	if got, want := reviewComment.ID, int64(800); got != want {
		t.Fatalf("UpdateReviewComment id = %d, want %d", got, want)
	}
	if err := appClient.DeleteReviewComment(context.Background(), 800, 9001); err != nil {
		t.Fatalf("DeleteReviewComment() error = %v", err)
	}

	if err := validateGitHubAppCredentials(&resolvedInstanceConfig{appID: "", privateKey: privateKey}); err == nil {
		t.Fatal("validateGitHubAppCredentials(missing appID) error = nil, want non-nil")
	}
	if err := validateGitHubAppCredentials(&resolvedInstanceConfig{appID: "abc", privateKey: privateKey}); err == nil {
		t.Fatal("validateGitHubAppCredentials(non-numeric appID) error = nil, want non-nil")
	}
	if err := validateGitHubAppCredentials(&resolvedInstanceConfig{appID: "12345", privateKey: "bad"}); err == nil {
		t.Fatal("validateGitHubAppCredentials(bad key) error = nil, want non-nil")
	}
	if err := validateGitHubAppCredentials(&resolvedInstanceConfig{
		appID:      "12345",
		privateKey: privateKey,
	}); err != nil {
		t.Fatalf("validateGitHubAppCredentials(valid) error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if got, want := requestCounts[http.MethodPatch+" /repos/acme/app/issues/comments/700"], 1; got != want {
		t.Fatalf("issue patch count = %d, want %d", got, want)
	}
	if got, want := requestCounts[http.MethodDelete+" /repos/acme/app/issues/comments/700"], 1; got != want {
		t.Fatalf("issue delete count = %d, want %d", got, want)
	}
	if got, want := requestCounts[http.MethodPatch+" /repos/acme/app/pulls/comments/800"], 1; got != want {
		t.Fatalf("review patch count = %d, want %d", got, want)
	}
	if got, want := requestCounts[http.MethodDelete+" /repos/acme/app/pulls/comments/800"], 1; got != want {
		t.Fatalf("review delete count = %d, want %d", got, want)
	}
	if got, want := requestCounts[http.MethodPost+" /app/installations/9001/access_tokens"], 1; got != want {
		t.Fatalf("installation token count = %d, want %d", got, want)
	}
}

func TestGitHubProviderLifecycleRunAndRetryHelpers(t *testing.T) {
	tmpDir := t.TempDir()
	handshakePath := filepath.Join(tmpDir, "handshake.json")
	shutdownPath := filepath.Join(tmpDir, "shutdown.log")
	startsPath := filepath.Join(tmpDir, "starts.log")

	t.Setenv(bridgesdk.AdapterHandshakePathEnv, handshakePath)
	t.Setenv(bridgesdk.AdapterShutdownPathEnv, shutdownPath)
	t.Setenv(bridgesdk.AdapterStartsPathEnv, startsPath)

	provider, err := newGitHubProvider(io.Discard)
	if err != nil {
		t.Fatalf("newGitHubProvider() error = %v", err)
	}

	session := newGitHubTestSession(t, nil, func(_ context.Context, method string, _ any, result any) error {
		if method != "bridges/instances/list" {
			return errors.New("unexpected method: " + method)
		}
		*(result.(*[]bridgepkg.BridgeInstance)) = nil
		return nil
	})
	if err := provider.lifecycle.Initialize(context.Background(), session); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	select {
	case <-provider.lifecycle.Initialized():
	case <-t.Context().Done():
		t.Fatal("provider initialization did not finish")
	}

	handshakeRaw, err := os.ReadFile(handshakePath)
	if err != nil {
		t.Fatalf("os.ReadFile(handshake) error = %v", err)
	}
	if !strings.Contains(string(handshakeRaw), `"provider":"github"`) {
		t.Fatalf("handshake marker = %s, want github runtime metadata", handshakeRaw)
	}
	if err := provider.healthCheck(); err != nil {
		t.Fatalf("healthCheck(initial) error = %v, want nil", err)
	}
	provider.setLastError(errors.New("boom"))
	if err := provider.healthCheck(); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("healthCheck(lastError) = %v, want boom", err)
	}
	provider.clearLastError()

	if err := provider.startServer("127.0.0.1:0"); err != nil {
		t.Fatalf("startServer() error = %v", err)
	}
	if err := provider.lifecycle.Shutdown(
		context.Background(),
		session,
		subprocess.ShutdownRequest{DeadlineMS: 250},
	); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	shutdownRaw, err := os.ReadFile(shutdownPath)
	if err != nil {
		t.Fatalf("os.ReadFile(shutdown) error = %v", err)
	}
	if !strings.Contains(string(shutdownRaw), "pid=") {
		t.Fatalf("shutdown marker = %q, want pid marker", string(shutdownRaw))
	}

	if err := run([]string{"bogus"}, strings.NewReader(""), io.Discard, io.Discard); err == nil {
		t.Fatal("run(unsupported) error = nil, want non-nil")
	}
	if err := provider.serve(strings.NewReader(""), io.Discard); err != nil {
		t.Fatalf("provider.serve(empty stdin) error = %v, want nil", err)
	}
	if err := runServe(strings.NewReader(""), io.Discard, io.Discard); err != nil {
		t.Fatalf("runServe(empty stdin) error = %v, want nil", err)
	}
}

func TestGitHubProviderResolveDeliveryInstallationAndWebhookBranches(t *testing.T) {
	t.Parallel()

	provider := newGitHubProviderForTest(t, nil)

	cfg := resolvedInstanceConfig{
		instanceID:   "brg-github",
		mode:         githubModeApp,
		repoFullName: "acme/app",
	}
	if got, err := provider.resolveDeliveryInstallationID(
		&resolvedInstanceConfig{mode: githubModePAT},
		bridgepkg.DeliveryRequest{},
	); err != nil ||
		got != 0 {
		t.Fatalf("resolveDeliveryInstallationID(PAT) = (%d, %v), want (0, nil)", got, err)
	}
	if got, err := provider.resolveDeliveryInstallationID(
		&resolvedInstanceConfig{mode: githubModeApp, installationID: 9001},
		bridgepkg.DeliveryRequest{},
	); err != nil ||
		got != 9001 {
		t.Fatalf("resolveDeliveryInstallationID(config) = (%d, %v), want (9001, nil)", got, err)
	}

	request := bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			ProviderMetadata: mustJSON(t, map[string]any{"installation_id": 9002}),
		},
	}
	if got, err := provider.resolveDeliveryInstallationID(&cfg, request); err != nil || got != 9002 {
		t.Fatalf("resolveDeliveryInstallationID(event metadata) = (%d, %v), want (9002, nil)", got, err)
	}
	if got := provider.cachedInstallationID("acme/app"); got != 9002 {
		t.Fatalf("cached installation id = %d, want 9002", got)
	}

	request = bridgepkg.DeliveryRequest{
		Snapshot: &bridgepkg.DeliverySnapshot{
			ProviderMetadata: mustJSON(t, map[string]any{"installation_id": 9003}),
		},
	}
	if got, err := provider.resolveDeliveryInstallationID(&cfg, request); err != nil || got != 9003 {
		t.Fatalf("resolveDeliveryInstallationID(snapshot metadata) = (%d, %v), want (9003, nil)", got, err)
	}

	provider.storeInstallationID("acme/app", 9004)
	if got, err := provider.resolveDeliveryInstallationID(
		&cfg,
		bridgepkg.DeliveryRequest{},
	); err != nil || got != 9004 {
		t.Fatalf("resolveDeliveryInstallationID(cache) = (%d, %v), want (9004, nil)", got, err)
	}

	if _, err := provider.resolveDeliveryInstallationID(&resolvedInstanceConfig{
		instanceID:   "brg-github",
		mode:         githubModeApp,
		repoFullName: "acme/other",
	}, bridgepkg.DeliveryRequest{}); err == nil {
		t.Fatal("resolveDeliveryInstallationID(missing) error = nil, want non-nil")
	}

	session := newGitHubTestSession(t, []subprocess.InitializeBridgeManagedInstance{{
		Instance: bridgepkg.BridgeInstance{ID: "brg-github", Scope: bridgepkg.ScopeWorkspace, WorkspaceID: "ws-1"},
	}}, func(_ context.Context, method string, params any, result any) error {
		if method != "bridges/messages/ingest" {
			return errors.New("unexpected method: " + method)
		}
		envelope := params.(bridgepkg.InboundMessageEnvelope)
		*(result.(*bridgepkg.BridgesMessagesIngestResult)) = bridgepkg.BridgesMessagesIngestResult{
			SessionID: "sess-" + envelope.BridgeInstanceID,
		}
		return nil
	})
	setUnexportedField(t, provider.lifecycle, "session", session)
	webhookConfig := resolvedInstanceConfig{
		managed: subprocess.InitializeBridgeManagedInstance{
			Instance: bridgepkg.BridgeInstance{ID: "brg-github", Scope: bridgepkg.ScopeWorkspace, WorkspaceID: "ws-1"},
		},
		instanceID:    "brg-github",
		repoOwner:     "acme",
		repoName:      "app",
		repoFullName:  "acme/app",
		webhookSecret: "secret",
		webhookPath:   "/github",
		botLogin:      "bridge-bot",
		dedup:         bridgesdk.NewDedupCache(5*time.Minute, 100),
	}
	provider.routes.Replace(map[string]resolvedInstanceConfig{
		"brg-github": webhookConfig,
	}, nil)

	writeWebhook := func(event string, payload any) (int, string, error) {
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		recorder := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"http://example.test/github",
			strings.NewReader(string(body)),
		)
		req.Header.Set("X-GitHub-Event", event)
		req.Header.Set("X-Hub-Signature-256", signGitHubTestBody(webhookConfig.webhookSecret, body))
		err = provider.handleWebhookRequest(
			recorder,
			req,
			[]resolvedInstanceConfig{webhookConfig},
			bridgesdk.WebhookRequest{
				Body:       body,
				ReceivedAt: time.Date(2026, 4, 15, 21, 50, 0, 0, time.UTC),
			},
		)
		return recorder.Code, recorder.Body.String(), err
	}

	if status, body, err := writeWebhook(
		"ping",
		map[string]any{},
	); err != nil || status != http.StatusOK ||
		body != "pong" {
		t.Fatalf("ping webhook = (%d, %q, %v), want (200, pong, nil)", status, body, err)
	}

	if status, body, err := writeWebhook(
		"workflow_run",
		map[string]any{},
	); err != nil || status != http.StatusOK ||
		body != "ok" {
		t.Fatalf("unknown webhook = (%d, %q, %v), want (200, ok, nil)", status, body, err)
	}

	if status, body, err := writeWebhook("issue_comment", map[string]any{
		"action": "edited",
		"comment": map[string]any{
			"id":         1,
			"body":       "ignored",
			"created_at": "2026-04-15T21:50:00Z",
			"user":       map[string]any{"id": 1, "login": "alice"},
		},
		"issue":      map[string]any{"number": 42},
		"repository": map[string]any{"full_name": "acme/app", "name": "app", "owner": map[string]any{"login": "acme"}},
		"sender":     map[string]any{"id": 1, "login": "alice"},
	}); err != nil || status != http.StatusOK || body != "ok" {
		t.Fatalf("edited issue webhook = (%d, %q, %v), want (200, ok, nil)", status, body, err)
	}

	if status, body, err := writeWebhook("issue_comment", map[string]any{
		"action": "created",
		"comment": map[string]any{
			"id":         2,
			"body":       "self",
			"created_at": "2026-04-15T21:50:00Z",
			"user":       map[string]any{"id": 2, "login": "bridge-bot"},
		},
		"issue":      map[string]any{"number": 42},
		"repository": map[string]any{"full_name": "acme/app", "name": "app", "owner": map[string]any{"login": "acme"}},
		"sender":     map[string]any{"id": 2, "login": "bridge-bot"},
	}); err != nil || status != http.StatusOK || body != "ok" {
		t.Fatalf("self issue webhook = (%d, %q, %v), want (200, ok, nil)", status, body, err)
	}

	if status, body, err := writeWebhook("issue_comment", map[string]any{
		"action": "created",
		"comment": map[string]any{
			"id":         3,
			"body":       "other repo",
			"created_at": "2026-04-15T21:50:00Z",
			"user":       map[string]any{"id": 3, "login": "alice"},
		},
		"issue": map[string]any{"number": 42},
		"repository": map[string]any{
			"full_name": "acme/other",
			"name":      "other",
			"owner":     map[string]any{"login": "acme"},
		},
		"sender": map[string]any{"id": 3, "login": "alice"},
	}); err != nil || status != http.StatusOK || body != "ignored" {
		t.Fatalf("unmatched issue webhook = (%d, %q, %v), want (200, ignored, nil)", status, body, err)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.test/github",
		strings.NewReader("{"),
	)
	req.Header.Set("X-GitHub-Event", "issue_comment")
	if err := provider.handleWebhookRequest(
		recorder,
		req,
		[]resolvedInstanceConfig{webhookConfig},
		bridgesdk.WebhookRequest{
			Body:       []byte("{"),
			ReceivedAt: time.Date(2026, 4, 15, 21, 50, 0, 0, time.UTC),
		},
	); err == nil {
		t.Fatal("invalid issue_comment payload error = nil, want non-nil")
	}
}

func TestGitHubAdditionalHelpersAndErrorClassification(t *testing.T) {
	t.Parallel()

	privateKey := mustGitHubTestPrivateKey(t)

	block, _ := pem.Decode([]byte(privateKey))
	if block == nil {
		t.Fatal("pem.Decode(privateKey) = nil")
	}
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(mustParseGitHubTestKey(t, privateKey))
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey() error = %v", err)
	}
	pkcs8 := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Bytes}))
	if _, err := parseGitHubPrivateKey(pkcs8); err != nil {
		t.Fatalf("parseGitHubPrivateKey(pkcs8) error = %v", err)
	}
	if _, err := signGitHubAppJWT("12345", privateKey, time.Date(2026, 4, 15, 22, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("signGitHubAppJWT() error = %v", err)
	}

	if got, err := joinGitHubURL("https://api.github.com/root/", "/repos/acme/app"); err != nil {
		t.Fatalf("joinGitHubURL(valid) error = %v", err)
	} else if got != "https://api.github.com/repos/acme/app" {
		t.Fatalf("joinGitHubURL(valid) = %q, want github repo endpoint", got)
	}
	if _, err := joinGitHubURL("http://[::1", "/repos/acme/app"); err == nil {
		t.Fatal("joinGitHubURL(invalid base) error = nil, want non-nil")
	}

	if got, want := normalizeURL(" https://api.github.com/root/ "), "https://api.github.com/root"; got != want {
		t.Fatalf("normalizeURL() = %q, want %q", got, want)
	}
	if got, want := parseRetryAfter("15"), 15*time.Second; got != want {
		t.Fatalf("parseRetryAfter(valid) = %s, want %s", got, want)
	}
	if got := parseRetryAfter("bogus"); got != 0 {
		t.Fatalf("parseRetryAfter(invalid) = %s, want 0", got)
	}

	authClient := &githubClient{cfg: resolvedInstanceConfig{mode: githubModePAT}}
	if _, err := authClient.authHeader(context.Background(), 0); err == nil {
		t.Fatal("authHeader(PAT missing token) error = nil, want non-nil")
	}
	authClient.cfg = resolvedInstanceConfig{mode: "other"}
	if _, err := authClient.authHeader(context.Background(), 0); err == nil {
		t.Fatal("authHeader(unsupported mode) error = nil, want non-nil")
	}

	if err := classifyGitHubHTTPError(http.StatusUnauthorized, "", `{"message":"nope"}`); err == nil {
		t.Fatal("classifyGitHubHTTPError(401) error = nil, want non-nil")
	} else {
		var authErr *bridgesdk.AuthError
		if !errors.As(err, &authErr) {
			t.Fatalf("401 classification = %#v, want auth error", err)
		}
	}
	if err := classifyGitHubHTTPError(http.StatusForbidden, "9", `{"message":"rate limit exceeded"}`); err == nil {
		t.Fatal("classifyGitHubHTTPError(403 rate limit) error = nil, want non-nil")
	} else {
		var rateErr *bridgesdk.RateLimitError
		if !errors.As(err, &rateErr) || rateErr.RetryAfter != 9*time.Second {
			t.Fatalf("403 rate limit classification = %#v, want retry-after 9s", err)
		}
	}
	if err := classifyGitHubHTTPError(http.StatusForbidden, "", `{"message":"forbidden"}`); err == nil {
		t.Fatal("classifyGitHubHTTPError(403 auth) error = nil, want non-nil")
	} else {
		var authErr *bridgesdk.AuthError
		if !errors.As(err, &authErr) {
			t.Fatalf("403 auth classification = %#v, want auth error", err)
		}
	}
	if err := classifyGitHubHTTPError(http.StatusBadGateway, "", `{"message":"upstream failed"}`); err == nil {
		t.Fatal("classifyGitHubHTTPError(502) error = nil, want non-nil")
	} else {
		var transientErr *bridgesdk.TransientError
		if !errors.As(err, &transientErr) {
			t.Fatalf("502 classification = %#v, want transient error", err)
		}
	}
	if err := classifyGitHubHTTPError(http.StatusUnprocessableEntity, "", `{"error":"bad input"}`); err == nil {
		t.Fatal("classifyGitHubHTTPError(422) error = nil, want non-nil")
	} else {
		var permanentErr *bridgesdk.PermanentError
		if !errors.As(err, &permanentErr) {
			t.Fatalf("422 classification = %#v, want permanent error", err)
		}
	}

	if got, want := extractGitHubErrorMessage(`{"error":"bad input"}`), "bad input"; got != want {
		t.Fatalf("extractGitHubErrorMessage(error field) = %q, want %q", got, want)
	}
	if got, err := readGitHubResponseBody(errReader{}); err == nil {
		t.Fatal("readGitHubResponseBody(errReader) error = nil, want non-nil")
	} else if got != "" {
		t.Fatalf("readGitHubResponseBody(errReader) = %q, want empty", got)
	}

	waitProvider := newGitHubProviderForTest(t, nil)
	go func() {
		time.Sleep(20 * time.Millisecond)
		waitProvider.routes.Replace(map[string]resolvedInstanceConfig{
			"brg-github": {instanceID: "brg-github"},
		}, nil)
		waitProvider.lifecycle.MarkRoutesReady()
	}()
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer waitCancel()
	if cfg, err := waitProvider.waitForInstanceConfig(
		waitCtx,
		"brg-github",
	); err != nil ||
		cfg.instanceID != "brg-github" {
		t.Fatalf("waitForInstanceConfig(available later) = (%#v, %v), want brg-github", cfg, err)
	}

	stopProvider := newGitHubProviderForTest(t, nil)
	stopProvider.lifecycle.Stop()
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer stopCancel()
	if _, err := stopProvider.waitForInstanceConfig(stopCtx, "missing"); err == nil {
		t.Fatal("waitForInstanceConfig(stopped) error = nil, want non-nil")
	}
}

func TestGitHubProviderStoreDeliveryStateEvictsTerminalEntries(t *testing.T) {
	provider := newGitHubProviderForTest(t, nil)
	startEvent := bridgepkg.DeliveryEvent{EventType: bridgepkg.DeliveryEventTypeStart}
	finalEvent := bridgepkg.DeliveryEvent{EventType: bridgepkg.DeliveryEventTypeFinal}

	provider.storeDeliveryState(
		"brg-github",
		"delivery-1",
		startEvent,
		deliveryState{LastSeq: 1, RemoteMessageID: "issue:1"},
	)
	if got := provider.deliveryState("brg-github", "delivery-1").RemoteMessageID; got != "issue:1" {
		t.Fatalf("deliveryState(start) = %q, want %q", got, "issue:1")
	}

	provider.storeDeliveryState(
		"brg-github",
		"delivery-1",
		finalEvent,
		deliveryState{LastSeq: 2, RemoteMessageID: "issue:1"},
	)
	if got := provider.deliveryState("brg-github", "delivery-1"); got != (deliveryState{}) {
		t.Fatalf("deliveryState(final) = %#v, want empty after eviction", got)
	}
}

func TestGitHubListenErrorsProjectOnlyOntoValidConfigs(t *testing.T) {
	t.Run("Should report missing and failed listener startup without replacing prior errors", func(t *testing.T) {
		t.Parallel()

		provider := newGitHubProviderForTest(t, nil)
		prior := errors.New("prior config error")
		configs := []resolvedInstanceConfig{{instanceID: "brg-valid"}, {instanceID: "brg-invalid", configError: prior}}
		provider.applyGitHubListenErrors(configs, "")
		if configs[0].configError == nil || !strings.Contains(configs[0].configError.Error(), "listen address") {
			t.Fatalf("missing listener error = %v", configs[0].configError)
		}
		if !errors.Is(configs[1].configError, prior) {
			t.Fatalf("prior config error = %v, want preserved", configs[1].configError)
		}

		configs = []resolvedInstanceConfig{{instanceID: "brg-valid"}}
		provider.applyGitHubListenErrors(configs, "invalid-listen-address")
		if configs[0].configError == nil || !strings.Contains(configs[0].configError.Error(), "listen") {
			t.Fatalf("listener startup error = %v", configs[0].configError)
		}
	})
}

func signGitHubTestBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write(body); err != nil {
		panic("write GitHub signature body: " + err.Error())
	}
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return payload
}

func newGitHubTestSession(
	t *testing.T,
	managed []subprocess.InitializeBridgeManagedInstance,
	call bridgesdk.CallFunc,
) *bridgesdk.Session {
	t.Helper()

	request := subprocess.InitializeRequest{
		ProtocolVersion: "1.0",
		Capabilities: subprocess.InitializeCapabilities{
			Provides: []string{"bridge.adapter"},
			GrantedActions: []extensionprotocol.HostAPIMethod{
				"bridges/instances/list",
				"bridges/instances/get",
				"bridges/instances/report_state",
				"bridges/messages/ingest",
			},
		},
		Methods: subprocess.InitializeMethods{
			ExtensionServices: []string{"bridges/deliver"},
		},
		Runtime: subprocess.InitializeRuntime{
			Bridge: &subprocess.InitializeBridgeRuntime{
				RuntimeVersion:   subprocess.InitializeBridgeRuntimeVersion2,
				Purpose:          subprocess.BridgeRuntimePurposeService,
				Provider:         "github",
				Platform:         "github",
				ManagedInstances: managed,
			},
		},
	}

	session := &bridgesdk.Session{}
	setUnexportedField(t, session, "request", request)
	setUnexportedField(t, session, "response", subprocess.InitializeResponse{})
	setUnexportedField(t, session, "host", bridgesdk.NewHostAPIClientFromCall(call))
	setUnexportedField(t, session, "cache", bridgesdk.NewInstanceCache(request.Runtime.Bridge))
	setUnexportedField(t, session, "now", func() time.Time { return time.Date(2026, 4, 15, 21, 0, 0, 0, time.UTC) })
	return session
}

func newGitHubProviderForTest(t *testing.T, session *bridgesdk.Session) *githubProvider {
	t.Helper()

	provider, err := newGitHubProvider(io.Discard)
	if err != nil {
		t.Fatalf("newGitHubProvider() error = %v", err)
	}
	if session != nil {
		setUnexportedField(t, provider.lifecycle, "session", session)
	}
	t.Cleanup(func() {
		provider.lifecycle.Stop()
		if err := provider.http.Shutdown(context.Background()); err != nil {
			t.Errorf("provider HTTP shutdown error = %v", err)
		}
	})
	return provider
}

func setUnexportedField(t *testing.T, target any, fieldName string, value any) {
	t.Helper()

	elem := reflect.ValueOf(target).Elem()
	field := elem.FieldByName(fieldName)
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}

func mustGitHubTestPrivateKey(t *testing.T) string {
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

func mustParseGitHubTestKey(t *testing.T, value string) *rsa.PrivateKey {
	t.Helper()

	key, err := parseGitHubPrivateKey(value)
	if err != nil {
		t.Fatalf("parseGitHubPrivateKey() error = %v", err)
	}
	return key
}

func equalInt64s(got []int64, want []int64) bool {
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

type fakeGitHubAPI struct {
	viewer                  *githubViewer
	validateErr             error
	updateErr               error
	nextIssueCommentID      int64
	nextReviewCommentID     int64
	issueUpdates            []int64
	reviewUpdates           []int64
	deletedIssueCommentIDs  []int64
	deletedReviewCommentIDs []int64
}

func (f *fakeGitHubAPI) ValidateAuth(context.Context, int64) (*githubViewer, error) {
	if f.validateErr != nil {
		return nil, f.validateErr
	}
	return f.viewer, nil
}

func (f *fakeGitHubAPI) CreateIssueComment(context.Context, int64, string, int64) (*githubIssueComment, error) {
	if f.nextIssueCommentID == 0 {
		f.nextIssueCommentID = 500
	}
	comment := &githubIssueComment{ID: f.nextIssueCommentID}
	f.nextIssueCommentID++
	return comment, nil
}

func (f *fakeGitHubAPI) CreateReviewCommentReply(
	context.Context,
	int64,
	int64,
	string,
	int64,
) (*githubReviewComment, error) {
	if f.nextReviewCommentID == 0 {
		f.nextReviewCommentID = 600
	}
	comment := &githubReviewComment{ID: f.nextReviewCommentID}
	f.nextReviewCommentID++
	return comment, nil
}

func (f *fakeGitHubAPI) UpdateIssueComment(
	_ context.Context,
	commentID int64,
	_ string,
	_ int64,
) (*githubIssueComment, error) {
	f.issueUpdates = append(f.issueUpdates, commentID)
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return &githubIssueComment{ID: commentID}, nil
}

func (f *fakeGitHubAPI) UpdateReviewComment(
	_ context.Context,
	commentID int64,
	_ string,
	_ int64,
) (*githubReviewComment, error) {
	f.reviewUpdates = append(f.reviewUpdates, commentID)
	return &githubReviewComment{ID: commentID}, nil
}

func (f *fakeGitHubAPI) DeleteIssueComment(_ context.Context, commentID int64, _ int64) error {
	f.deletedIssueCommentIDs = append(f.deletedIssueCommentIDs, commentID)
	return nil
}

func (f *fakeGitHubAPI) DeleteReviewComment(_ context.Context, commentID int64, _ int64) error {
	f.deletedReviewCommentIDs = append(f.deletedReviewCommentIDs, commentID)
	return nil
}

type fakeGitHubErrorAPI struct {
	err error
}

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("boom")
}

type githubPartialErrorResponseBody struct {
	content []byte
	readErr error
	reads   int
	closed  bool
}

func (b *githubPartialErrorResponseBody) Read(buffer []byte) (int, error) {
	b.reads++
	if b.reads > 1 {
		return 0, io.EOF
	}
	return copy(buffer, b.content), b.readErr
}

func (b *githubPartialErrorResponseBody) Close() error {
	b.closed = true
	return nil
}

func (f *fakeGitHubErrorAPI) ValidateAuth(context.Context, int64) (*githubViewer, error) {
	return nil, f.err
}

func (f *fakeGitHubErrorAPI) CreateIssueComment(context.Context, int64, string, int64) (*githubIssueComment, error) {
	return nil, f.err
}

func (f *fakeGitHubErrorAPI) CreateReviewCommentReply(
	context.Context,
	int64,
	int64,
	string,
	int64,
) (*githubReviewComment, error) {
	return nil, f.err
}

func (f *fakeGitHubErrorAPI) UpdateIssueComment(context.Context, int64, string, int64) (*githubIssueComment, error) {
	return nil, f.err
}

func (f *fakeGitHubErrorAPI) UpdateReviewComment(context.Context, int64, string, int64) (*githubReviewComment, error) {
	return nil, f.err
}

func (f *fakeGitHubErrorAPI) DeleteIssueComment(context.Context, int64, int64) error {
	return f.err
}

func (f *fakeGitHubErrorAPI) DeleteReviewComment(context.Context, int64, int64) error {
	return f.err
}

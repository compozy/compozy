package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func TestGitHubWebhookRateLimitKey(t *testing.T) {
	t.Run("Should rate limit the same client host across different source ports", func(t *testing.T) {
		t.Parallel()

		const webhookSecret = "super-secret"
		provider, err := newGitHubProvider(io.Discard)
		if err != nil {
			t.Fatalf("newGitHubProvider() error = %v", err)
		}
		provider.now = func() time.Time { return time.Date(2026, 5, 16, 21, 45, 0, 0, time.UTC) }
		provider.rateLimiter = bridgesdk.NewFixedWindowRateLimiter(1, time.Minute)
		provider.inFlightLimiter = bridgesdk.NewInFlightLimiter(4)
		provider.routes.Replace(map[string]resolvedInstanceConfig{"brg-github": {
			instanceID:    "brg-github",
			webhookPath:   "/github",
			webhookSecret: webhookSecret,
		}}, nil)

		first := serveGitHubPingWebhook(t, provider, webhookSecret, "203.0.113.9:10001")
		if got, want := first.Code, http.StatusOK; got != want {
			t.Fatalf("first status = %d, want %d", got, want)
		}
		if got, want := strings.TrimSpace(first.Body.String()), "pong"; got != want {
			t.Fatalf("first body = %q, want %q", got, want)
		}

		second := serveGitHubPingWebhook(t, provider, webhookSecret, "203.0.113.9:10002")
		if got, want := second.Code, http.StatusTooManyRequests; got != want {
			t.Fatalf("second status = %d, want %d", got, want)
		}
		if got, want := strings.TrimSpace(
			second.Body.String(),
		), http.StatusText(
			http.StatusTooManyRequests,
		); got != want {
			t.Fatalf("second body = %q, want %q", got, want)
		}
	})
}

func TestGitHubDeliveryResumeIdempotency(t *testing.T) {
	t.Run("Should return stable ack for same sequence resume with existing provider state", func(t *testing.T) {
		t.Parallel()

		cfg := resolvedInstanceConfig{
			instanceID:   "brg-github",
			mode:         githubModeApp,
			repoOwner:    "acme",
			repoName:     "app",
			repoFullName: "acme/app",
		}
		api := &fakeGitHubAPI{
			viewer:             &githubViewer{Login: "bridge-bot"},
			nextIssueCommentID: 500,
		}

		startReq := testGitHubIssueDeliveryRequest(bridgepkg.DeliveryEventTypeStart, 1, false)
		startAck, state, err := executeGitHubDelivery(context.Background(), api, &cfg, startReq, deliveryState{}, 9001)
		if err != nil {
			t.Fatalf("executeGitHubDelivery(start) error = %v", err)
		}
		if got, want := startAck.RemoteMessageID, "issue:500"; got != want {
			t.Fatalf("start remote id = %q, want %q", got, want)
		}

		finalReq := testGitHubIssueDeliveryRequest(bridgepkg.DeliveryEventTypeFinal, 2, true)
		finalAck, state, err := executeGitHubDelivery(context.Background(), api, &cfg, finalReq, state, 9001)
		if err != nil {
			t.Fatalf("executeGitHubDelivery(final) error = %v", err)
		}
		if got, want := finalAck.RemoteMessageID, "issue:500"; got != want {
			t.Fatalf("final remote id = %q, want %q", got, want)
		}
		issueUpdates := len(api.issueUpdates)

		resumeReq := finalReq
		resumeReq.Event.EventType = bridgepkg.DeliveryEventTypeResume
		resumeReq.Event.Resume = &bridgepkg.DeliveryResumeState{LatestEventType: bridgepkg.DeliveryEventTypeFinal}
		resumeReq.Snapshot = &bridgepkg.DeliverySnapshot{
			DeliveryID:             resumeReq.Event.DeliveryID,
			SessionID:              "sess-1",
			TurnID:                 "turn-1",
			BridgeInstanceID:       resumeReq.Event.BridgeInstanceID,
			RoutingKey:             resumeReq.Event.RoutingKey,
			DeliveryTarget:         resumeReq.Event.DeliveryTarget,
			LatestSeq:              resumeReq.Event.Seq,
			LatestEventType:        bridgepkg.DeliveryEventTypeFinal,
			CurrentContent:         resumeReq.Event.Content,
			RemoteMessageID:        finalAck.RemoteMessageID,
			ReplaceRemoteMessageID: finalAck.ReplaceRemoteMessageID,
			Final:                  true,
			UpdatedAt:              time.Date(2026, 5, 16, 21, 50, 0, 0, time.UTC),
		}

		resumeAck, resumedState, err := executeGitHubDelivery(context.Background(), api, &cfg, resumeReq, state, 9001)
		if err != nil {
			t.Fatalf("executeGitHubDelivery(resume) error = %v", err)
		}
		if got, want := resumeAck.RemoteMessageID, finalAck.RemoteMessageID; got != want {
			t.Fatalf("resume remote id = %q, want %q", got, want)
		}
		if got, want := resumeAck.ReplaceRemoteMessageID, finalAck.ReplaceRemoteMessageID; got != want {
			t.Fatalf("resume replace remote id = %q, want %q", got, want)
		}
		if got, want := resumeAck.Seq, finalReq.Event.Seq; got != want {
			t.Fatalf("resume seq = %d, want %d", got, want)
		}
		if got := len(api.issueUpdates); got != issueUpdates {
			t.Fatalf("len(issueUpdates) = %d, want unchanged %d", got, issueUpdates)
		}
		if resumedState != state {
			t.Fatalf("resumed state = %#v, want unchanged %#v", resumedState, state)
		}
	})
}

func serveGitHubPingWebhook(
	t *testing.T,
	provider *githubProvider,
	secret string,
	remoteAddr string,
) *httptest.ResponseRecorder {
	t.Helper()

	body := []byte(`{}`)
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.test/github",
		strings.NewReader(string(body)),
	)
	req.RemoteAddr = remoteAddr
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-Hub-Signature-256", signGitHubTestBody(secret, body))
	recorder := httptest.NewRecorder()
	provider.serveWebhookHTTP(recorder, req)
	return recorder
}

func testGitHubIssueDeliveryRequest(
	eventType bridgepkg.DeliveryEventType,
	seq int64,
	final bool,
) bridgepkg.DeliveryRequest {
	return bridgepkg.DeliveryRequest{
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
			Seq:       seq,
			EventType: eventType,
			Content:   bridgepkg.MessageContent{Text: "hello world"},
			Final:     final,
		},
	}
}

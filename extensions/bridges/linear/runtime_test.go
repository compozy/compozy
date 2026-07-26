package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/subprocess"
)

func TestRuntimeInitializeStartsServerAndWritesMarkers(t *testing.T) {
	env := setLinearProviderTestEnv(t)
	listenAddr := reserveLinearListenAddr(t)
	now := time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC)

	runtime, hostPeer, cleanup := newLinearRuntimePeerPair(t)
	defer cleanup()
	runtime.now = func() time.Time { return now }

	runtime.apiFactory = func(cfg resolvedInstanceConfig) linearAPI {
		return &recordingLinearAPI{
			viewer: &linearViewer{
				ID:             "bot-" + cfg.mode,
				DisplayName:    "Linear Bot",
				OrganizationID: cfg.organizationID,
			},
		}
	}

	managed := []subprocess.InitializeBridgeManagedInstance{
		linearRuntimeManagedInstance(
			now,
			"brg-linear-comments",
			"org-comments",
			linearModeComments,
			linearAuthModeAPIKey,
			listenAddr,
		),
		linearRuntimeManagedInstance(
			now,
			"brg-linear-agent",
			"org-agent",
			linearModeAgentSessions,
			linearAuthModeOAuth,
			listenAddr,
		),
	}
	mustHandleLinearLifecycle(t, hostPeer, managed...)

	if err := hostPeer.Call(
		context.Background(),
		"initialize",
		linearInitializeRequest(now, managed...),
		nil,
	); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	handshake := waitForLinearJSONFile[bridgesdk.InitializeMarker](t, env.HandshakePath)
	if got, want := handshake.Request.Runtime.Bridge.Provider, "linear"; got != want {
		t.Fatalf("handshake provider = %q, want %q", got, want)
	}

	ownership := waitForLinearJSONFile[bridgesdk.OwnershipMarker](t, env.OwnershipPath)
	if got, want := len(ownership.Fetched), 2; got != want {
		t.Fatalf("len(ownership.Fetched) = %d, want %d", got, want)
	}

	states := waitForLinearJSONLinesFile[bridgesdk.StateMarker](
		t,
		env.StatePath,
		func(items []bridgesdk.StateMarker) bool { return len(items) >= 2 },
	)
	for _, state := range states[:2] {
		if got, want := state.Status.Normalize(), bridgepkg.BridgeStatusReady; got != want {
			t.Fatalf("state.Status = %q, want %q", got, want)
		}
	}

	waitForLinearCondition(t, func() bool {
		runtime.mu.RLock()
		defer runtime.mu.RUnlock()
		return strings.TrimSpace(runtime.http.Address()) != ""
	})
}

func TestWebhookIngressRejectsInvalidSignatureAndIngestsSupportedModes(t *testing.T) {
	env := setLinearProviderTestEnv(t)
	listenAddr := reserveLinearListenAddr(t)
	now := time.Date(2026, 4, 15, 13, 5, 0, 0, time.UTC)

	runtime, hostPeer, cleanup := newLinearRuntimePeerPair(t)
	defer cleanup()
	runtime.now = func() time.Time { return now }

	runtime.apiFactory = func(cfg resolvedInstanceConfig) linearAPI {
		botID := "bot-comments"
		if cfg.mode == linearModeAgentSessions {
			botID = "bot-agent"
		}
		return &recordingLinearAPI{
			viewer: &linearViewer{
				ID:             botID,
				DisplayName:    "Linear Bot",
				OrganizationID: cfg.organizationID,
			},
		}
	}

	managed := []subprocess.InitializeBridgeManagedInstance{
		linearRuntimeManagedInstance(
			now,
			"brg-linear-comments",
			"org-comments",
			linearModeComments,
			linearAuthModeAPIKey,
			listenAddr,
		),
		linearRuntimeManagedInstance(
			now,
			"brg-linear-agent",
			"org-agent",
			linearModeAgentSessions,
			linearAuthModeOAuth,
			listenAddr,
		),
	}
	mustHandleLinearLifecycle(t, hostPeer, managed...)

	var (
		mu       sync.Mutex
		ingested []bridgepkg.InboundMessageEnvelope
	)
	mustHandleLinear(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
		func(_ context.Context, params json.RawMessage) (any, error) {
			var envelope bridgepkg.InboundMessageEnvelope
			if err := json.Unmarshal(params, &envelope); err != nil {
				return nil, err
			}
			mu.Lock()
			ingested = append(ingested, envelope)
			mu.Unlock()
			return bridgepkg.BridgesMessagesIngestResult{
				SessionID:    "sess-" + envelope.BridgeInstanceID,
				RouteCreated: true,
				RoutingKey: bridgepkg.RoutingKey{
					Scope:            envelope.Scope,
					WorkspaceID:      envelope.WorkspaceID,
					BridgeInstanceID: envelope.BridgeInstanceID,
					PeerID:           envelope.PeerID,
					ThreadID:         envelope.ThreadID,
					GroupID:          envelope.GroupID,
				},
			}, nil
		},
	)

	if err := hostPeer.Call(
		context.Background(),
		"initialize",
		linearInitializeRequest(now, managed...),
		nil,
	); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	waitForLinearCondition(t, func() bool {
		runtime.mu.RLock()
		defer runtime.mu.RUnlock()
		return strings.TrimSpace(runtime.http.Address()) != ""
	})

	webhookURL := "http://" + linearRuntimeServerAddr(runtime) + "/linear"

	invalidPayload := linearCommentWebhookBodyForTest(
		now,
		"org-comments",
		"user-1",
		"reply-1",
		"root-comment",
		"Need a summary",
	)
	resp := postLinearTestWebhook(t, webhookURL, invalidPayload, "wrong-secret")
	if got, want := resp.StatusCode, http.StatusUnauthorized; got != want {
		t.Fatalf("invalid webhook status = %d, want %d", got, want)
	}
	_ = resp.Body.Close()

	commentPayload := linearCommentWebhookBodyForTest(
		now,
		"org-comments",
		"user-1",
		"reply-1",
		"root-comment",
		"Need a summary",
	)
	resp = postLinearTestWebhook(t, webhookURL, commentPayload, linearProviderWebhookSecretValue)
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("comment webhook status = %d, want %d", got, want)
	}
	_ = resp.Body.Close()

	agentPayload := linearAgentSessionWebhookBodyForTest(
		now,
		"org-agent",
		"session-123",
		"comment-root",
		"comment-source",
		"Prompt text",
	)
	resp = postLinearTestWebhook(t, webhookURL, agentPayload, linearProviderWebhookSecretValue)
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("agent webhook status = %d, want %d", got, want)
	}
	_ = resp.Body.Close()

	selfCommentPayload := linearCommentWebhookBodyForTest(
		now,
		"org-comments",
		"bot-comments",
		"reply-self",
		"root-comment",
		"Ignore me",
	)
	resp = postLinearTestWebhook(t, webhookURL, selfCommentPayload, linearProviderWebhookSecretValue)
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("self comment webhook status = %d, want %d", got, want)
	}
	_ = resp.Body.Close()

	records := waitForLinearJSONLinesFile[bridgesdk.IngestMarker](
		t,
		env.IngestPath,
		func(items []bridgesdk.IngestMarker) bool { return len(items) == 2 },
	)
	if got, want := records[0].Envelope.ThreadID, "linear:issue-comments:c:root-comment"; got != want {
		t.Fatalf("comment thread id = %q, want %q", got, want)
	}
	if got, want := records[1].Envelope.ThreadID, "linear:issue-agent:c:comment-root:s:session-123"; got != want {
		t.Fatalf("agent thread id = %q, want %q", got, want)
	}

	mu.Lock()
	defer mu.Unlock()
	if got, want := len(ingested), 2; got != want {
		t.Fatalf("len(ingested) = %d, want %d", got, want)
	}
}

func TestWebhookIngressRejectsCrossInstanceSignatureOnSharedPath(t *testing.T) {
	t.Run("Should reject cross-instance signature on shared webhook path", func(t *testing.T) {
		env := setLinearProviderTestEnv(t)
		listenAddr := reserveLinearListenAddr(t)
		now := time.Date(2026, 4, 15, 13, 10, 0, 0, time.UTC)

		runtime, hostPeer, cleanup := newLinearRuntimePeerPair(t)
		defer cleanup()
		runtime.now = func() time.Time { return now }
		runtime.apiFactory = func(cfg resolvedInstanceConfig) linearAPI {
			return &recordingLinearAPI{
				viewer: &linearViewer{
					ID:             "bot-" + cfg.instanceID,
					DisplayName:    "Linear Bot",
					OrganizationID: cfg.organizationID,
				},
			}
		}

		managed := []subprocess.InitializeBridgeManagedInstance{
			linearRuntimeManagedInstanceWithWebhookSecret(
				now,
				"brg-linear-a",
				"org-a",
				linearModeComments,
				linearAuthModeAPIKey,
				listenAddr,
				"secret-a",
			),
			linearRuntimeManagedInstanceWithWebhookSecret(
				now,
				"brg-linear-b",
				"org-b",
				linearModeComments,
				linearAuthModeAPIKey,
				listenAddr,
				"secret-b",
			),
		}
		mustHandleLinearLifecycle(t, hostPeer, managed...)

		var (
			mu       sync.Mutex
			ingested []bridgepkg.InboundMessageEnvelope
		)
		mustHandleLinear(
			t,
			hostPeer,
			string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
			func(_ context.Context, params json.RawMessage) (any, error) {
				var envelope bridgepkg.InboundMessageEnvelope
				if err := json.Unmarshal(params, &envelope); err != nil {
					return nil, err
				}
				mu.Lock()
				ingested = append(ingested, envelope)
				mu.Unlock()
				return bridgepkg.BridgesMessagesIngestResult{
					SessionID:    "sess-" + envelope.BridgeInstanceID,
					RouteCreated: true,
					RoutingKey: bridgepkg.RoutingKey{
						Scope:            envelope.Scope,
						WorkspaceID:      envelope.WorkspaceID,
						BridgeInstanceID: envelope.BridgeInstanceID,
						PeerID:           envelope.PeerID,
						ThreadID:         envelope.ThreadID,
						GroupID:          envelope.GroupID,
					},
				}, nil
			},
		)

		if err := hostPeer.Call(
			context.Background(),
			"initialize",
			linearInitializeRequest(now, managed...),
			nil,
		); err != nil {
			t.Fatalf("hostPeer.Call(initialize) error = %v", err)
		}

		waitForLinearCondition(t, func() bool {
			runtime.mu.RLock()
			defer runtime.mu.RUnlock()
			return strings.TrimSpace(runtime.http.Address()) != ""
		})

		webhookURL := "http://" + linearRuntimeServerAddr(runtime) + "/linear"
		payload := linearCommentWebhookBodyForTest(
			now,
			"org-b",
			"user-1",
			"reply-1",
			"root-comment",
			"Need a summary",
		)
		resp := postLinearTestWebhook(t, webhookURL, payload, "secret-a")
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("io.ReadAll(resp.Body) error = %v", err)
		}
		if err := resp.Body.Close(); err != nil {
			t.Fatalf("resp.Body.Close() error = %v", err)
		}
		if got, want := resp.StatusCode, http.StatusUnauthorized; got != want {
			t.Fatalf("cross-instance webhook status = %d, want %d", got, want)
		}
		if got, want := strings.TrimSpace(string(body)), http.StatusText(http.StatusUnauthorized); got != want {
			t.Fatalf("cross-instance webhook body = %q, want %q", got, want)
		}

		mu.Lock()
		defer mu.Unlock()
		if got, want := len(ingested), 0; got != want {
			t.Fatalf("len(ingested) = %d, want %d", got, want)
		}
		if _, err := os.Stat(env.IngestPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(ingestPath) error = %v, want not-exist", err)
		}
	})
}

func TestRuntimeDeliveriesRecordMarkersForSupportedModes(t *testing.T) {
	env := setLinearProviderTestEnv(t)
	listenAddr := reserveLinearListenAddr(t)

	runtime, hostPeer, cleanup := newLinearRuntimePeerPair(t)
	defer cleanup()

	commentAPI := &recordingLinearAPI{
		viewer: &linearViewer{
			ID:             "bot-comments",
			DisplayName:    "Linear Bot",
			OrganizationID: "org-comments",
		},
	}
	agentAPI := &recordingLinearAPI{
		viewer: &linearViewer{
			ID:             "bot-agent",
			DisplayName:    "Linear Bot",
			OrganizationID: "org-agent",
		},
	}
	runtime.apiFactory = func(cfg resolvedInstanceConfig) linearAPI {
		if cfg.mode == linearModeAgentSessions {
			return agentAPI
		}
		return commentAPI
	}

	now := time.Date(2026, 4, 15, 13, 10, 0, 0, time.UTC)
	managed := []subprocess.InitializeBridgeManagedInstance{
		linearRuntimeManagedInstance(
			now,
			"brg-linear-comments",
			"org-comments",
			linearModeComments,
			linearAuthModeAPIKey,
			listenAddr,
		),
		linearRuntimeManagedInstance(
			now,
			"brg-linear-agent",
			"org-agent",
			linearModeAgentSessions,
			linearAuthModeOAuth,
			listenAddr,
		),
	}
	mustHandleLinearLifecycle(t, hostPeer, managed...)

	if err := hostPeer.Call(
		context.Background(),
		"initialize",
		linearInitializeRequest(now, managed...),
		nil,
	); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	waitForLinearCondition(t, func() bool {
		_, err := runtime.configForInstance("brg-linear-agent")
		return err == nil
	})

	commentStart := linearTestDeliveryRequest(
		"brg-linear-comments",
		"delivery-comment",
		1,
		bridgepkg.DeliveryEventTypeStart,
		linearThreadRef{
			IssueID:       "issue-comments",
			RootCommentID: "root-comment",
		},
		"hello",
		linearModeComments,
	)
	var commentStartAck bridgepkg.DeliveryAck
	if err := hostPeer.Call(context.Background(), "bridges/deliver", commentStart, &commentStartAck); err != nil {
		t.Fatalf("hostPeer.Call(comment start) error = %v", err)
	}

	commentFinal := commentStart
	commentFinal.Event.Seq = 2
	commentFinal.Event.EventType = bridgepkg.DeliveryEventTypeFinal
	commentFinal.Event.Final = true
	commentFinal.Event.Content.Text = "hello world"
	var commentFinalAck bridgepkg.DeliveryAck
	if err := hostPeer.Call(context.Background(), "bridges/deliver", commentFinal, &commentFinalAck); err != nil {
		t.Fatalf("hostPeer.Call(comment final) error = %v", err)
	}

	commentDelete := commentFinal
	commentDelete.Event.Seq = 3
	commentDelete.Event.EventType = bridgepkg.DeliveryEventTypeDelete
	commentDelete.Event.Operation = bridgepkg.DeliveryOperationDelete
	commentDelete.Event.Reference = &bridgepkg.DeliveryMessageReference{
		RemoteMessageID: commentFinalAck.RemoteMessageID,
	}
	commentDelete.Event.Content.Text = ""
	var commentDeleteAck bridgepkg.DeliveryAck
	if err := hostPeer.Call(context.Background(), "bridges/deliver", commentDelete, &commentDeleteAck); err != nil {
		t.Fatalf("hostPeer.Call(comment delete) error = %v", err)
	}

	agentStart := linearTestDeliveryRequest(
		"brg-linear-agent",
		"delivery-agent",
		1,
		bridgepkg.DeliveryEventTypeStart,
		linearThreadRef{
			IssueID:        "issue-agent",
			RootCommentID:  "comment-root",
			AgentSessionID: "session-123",
		},
		"hello",
		linearModeAgentSessions,
	)
	var agentStartAck bridgepkg.DeliveryAck
	if err := hostPeer.Call(context.Background(), "bridges/deliver", agentStart, &agentStartAck); err != nil {
		t.Fatalf("hostPeer.Call(agent start) error = %v", err)
	}

	agentFinal := agentStart
	agentFinal.Event.Seq = 2
	agentFinal.Event.EventType = bridgepkg.DeliveryEventTypeFinal
	agentFinal.Event.Final = true
	agentFinal.Event.Content.Text = "hello world"
	var agentFinalAck bridgepkg.DeliveryAck
	if err := hostPeer.Call(context.Background(), "bridges/deliver", agentFinal, &agentFinalAck); err != nil {
		t.Fatalf("hostPeer.Call(agent final) error = %v", err)
	}

	records := waitForLinearJSONLinesFile[bridgesdk.DeliveryMarker](
		t,
		env.DeliveryPath,
		func(items []bridgesdk.DeliveryMarker) bool { return len(items) >= 5 },
	)
	if got, want := len(records), 5; got != want {
		t.Fatalf("len(records) = %d, want %d", got, want)
	}
	if got, want := commentFinalAck.ReplaceRemoteMessageID, commentStartAck.RemoteMessageID; got != want {
		t.Fatalf("comment final ReplaceRemoteMessageID = %q, want %q", got, want)
	}
	if got, want := commentDeleteAck.RemoteMessageID, commentFinalAck.RemoteMessageID; got != want {
		t.Fatalf("comment delete RemoteMessageID = %q, want %q", got, want)
	}
	if got, want := agentFinalAck.ReplaceRemoteMessageID, agentStartAck.RemoteMessageID; got != want {
		t.Fatalf("agent final ReplaceRemoteMessageID = %q, want %q", got, want)
	}
	if got, want := commentAPI.updatedComments, []string{"comment-created-1:hello world"}; !equalStrings(got, want) {
		t.Fatalf("commentAPI.updatedComments = %#v, want %#v", got, want)
	}
	if got, want := commentAPI.deletedComments, []string{"comment-created-1"}; !equalStrings(got, want) {
		t.Fatalf("commentAPI.deletedComments = %#v, want %#v", got, want)
	}
	if got, want := agentAPI.agentActivities, []string{"hello", " world"}; !equalStrings(got, want) {
		t.Fatalf("agentAPI.agentActivities = %#v, want %#v", got, want)
	}

	state := runtime.deliveryState("brg-linear-agent", "delivery-agent")
	if got, want := state.RemoteMessageID, agentFinalAck.RemoteMessageID; got != want {
		t.Fatalf("runtime.deliveryState() remote id = %q, want %q", got, want)
	}
}

func TestRuntimeProgressDeliveryAcknowledgesWithoutPlatformSideEffects(t *testing.T) {
	// not parallel: the provider bootstrap uses t.Setenv for process-wide runtime configuration.
	t.Run("Should ack progress without creating a Linear API client", func(t *testing.T) {
		setLinearProviderTestEnv(t)

		runtime, hostPeer, cleanup := newLinearRuntimePeerPair(t)
		defer cleanup()
		apiFactoryCalls := 0
		runtime.apiFactory = func(resolvedInstanceConfig) linearAPI {
			apiFactoryCalls++
			return &recordingLinearAPI{}
		}
		mustHandleLinearLifecycle(t, hostPeer)
		if err := hostPeer.Call(
			context.Background(),
			"initialize",
			linearInitializeRequest(
				time.Date(2026, 4, 15, 13, 20, 0, 0, time.UTC),
				linearRuntimeManagedInstance(
					time.Date(2026, 4, 15, 13, 20, 0, 0, time.UTC),
					"brg-linear-progress-noop",
					"org-progress",
					linearModeComments,
					linearAuthModeAPIKey,
					"127.0.0.1:0",
				),
			),
			nil,
		); err != nil {
			t.Fatalf("hostPeer.Call(initialize) error = %v", err)
		}

		request := linearTestDeliveryRequest(
			"brg-linear-progress-noop",
			"delivery-linear-progress-noop",
			1,
			bridgepkg.DeliveryEventTypeProgress,
			linearThreadRef{IssueID: "issue-progress", RootCommentID: "comment-progress"},
			"",
			linearModeComments,
		)
		request.Event.Progress = &bridgepkg.ToolProgress{
			ToolCallID: "tool-call-linear-noop",
			ToolID:     "agh__search",
			Phase:      bridgepkg.ToolProgressPhaseStarted,
			Label:      "Search",
			Index:      1,
		}
		var ack bridgepkg.DeliveryAck
		if err := hostPeer.Call(context.Background(), "bridges/deliver", request, &ack); err != nil {
			t.Fatalf("hostPeer.Call(progress) error = %v", err)
		}
		if got, want := ack.DeliveryID, request.Event.DeliveryID; got != want {
			t.Fatalf("ack delivery id = %q, want %q", got, want)
		}
		if got, want := ack.Seq, request.Event.Seq; got != want {
			t.Fatalf("ack seq = %d, want %d", got, want)
		}
		if ack.RemoteMessageID != "" || ack.ReplaceRemoteMessageID != "" {
			t.Fatalf("progress ack = %#v, want no platform handles", ack)
		}
		if got := apiFactoryCalls; got != 0 {
			t.Fatalf("apiFactory calls = %d, want 0", got)
		}
	})
}

func TestWaitHealthAndHelperUtilities(t *testing.T) {
	t.Parallel()

	waitProvider, err := newLinearProvider(io.Discard)
	if err != nil {
		t.Fatalf("newLinearProvider(wait) error = %v", err)
	}
	t.Cleanup(waitProvider.lifecycle.Stop)
	go func() {
		time.Sleep(20 * time.Millisecond)
		waitProvider.routes.Replace(map[string]resolvedInstanceConfig{
			"brg-1": {
				instanceID:     "brg-1",
				webhookPath:    "/linear",
				organizationID: "org-1",
				mode:           linearModeComments,
			},
		}, nil)
	}()
	cfg, err := waitProvider.waitForInstanceConfig(context.Background(), "brg-1", 200*time.Millisecond)
	if err != nil {
		t.Fatalf("waitForInstanceConfig() error = %v", err)
	}
	if got, want := cfg.instanceID, "brg-1"; got != want {
		t.Fatalf("cfg.instanceID = %q, want %q", got, want)
	}
	if got, want := len(waitProvider.configsForPath("/linear")), 1; got != want {
		t.Fatalf("len(configsForPath) = %d, want %d", got, want)
	}
	if _, err := waitProvider.configForInstance("missing"); err == nil {
		t.Fatal("configForInstance(missing) error = nil, want non-nil")
	}

	waitProvider.storeDeliveryState("brg-1", "delivery-1", deliveryState{RemoteMessageID: "remote-1"})
	if got, want := waitProvider.deliveryState("brg-1", "delivery-1").RemoteMessageID, "remote-1"; got != want {
		t.Fatalf("deliveryState().RemoteMessageID = %q, want %q", got, want)
	}

	waitProvider.setLastError(errors.New("boom"))
	if err := waitProvider.healthCheck(); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("healthCheck() error = %v, want boom", err)
	}
	waitProvider.clearLastError()
	if err := waitProvider.healthCheck(); err != nil {
		t.Fatalf("healthCheck(clear) error = %v", err)
	}

	commentCfg := resolvedInstanceConfig{
		organizationID: "org-1",
		mode:           linearModeComments,
		apiBaseURL:     "https://linear.example",
	}
	if got, want := commentCfg.ownershipKey(), "org-1|comments"; got != want {
		t.Fatalf("ownershipKey() = %q, want %q", got, want)
	}
	if got, want := commentCfg.graphqlURL(), "https://linear.example/graphql"; got != want {
		t.Fatalf("graphqlURL() = %q, want %q", got, want)
	}

	selected, ok, err := selectLinearConfig([]resolvedInstanceConfig{commentCfg}, "org-1", linearModeComments)
	if err != nil || !ok || selected.organizationID != "org-1" {
		t.Fatalf("selectLinearConfig() = (%#v, %v, %v), want selected org-1", selected, ok, err)
	}
	if _, _, err := selectLinearConfig(
		[]resolvedInstanceConfig{commentCfg, commentCfg},
		"org-1",
		linearModeComments,
	); err == nil {
		t.Fatal("selectLinearConfig(duplicate) error = nil, want non-nil")
	}

	if !linearCommentIsSelf(resolvedInstanceConfig{botUserID: "bot-1"}, linearCommentWebhookPayload{
		Data:  linearCommentData{UserID: "bot-1"},
		Actor: linearActor{ID: "bot-1"},
	}) {
		t.Fatal("linearCommentIsSelf() = false, want true")
	}
	if linearCommentIsSelf(resolvedInstanceConfig{botUserID: "bot-1"}, linearCommentWebhookPayload{
		Data: linearCommentData{UserID: "user-1"},
	}) {
		t.Fatal("linearCommentIsSelf(other user) = true, want false")
	}

	if got, want := deliveryStateKey("brg-1", "delivery-1"), "brg-1|delivery-1"; got != want {
		t.Fatalf("deliveryStateKey() = %q, want %q", got, want)
	}
	if got, want := actorID(&linearActor{ID: "user-1"}), "user-1"; got != want {
		t.Fatalf("actorID() = %q, want %q", got, want)
	}
	if got, want := actorName(&linearActor{Name: "Alice"}), "Alice"; got != want {
		t.Fatalf("actorName() = %q, want %q", got, want)
	}
	if got, want := actorURL(
		&linearActor{URL: "https://linear.app/u/alice"},
	), "https://linear.app/u/alice"; got != want {
		t.Fatalf("actorURL() = %q, want %q", got, want)
	}

	rec := httptest.NewRecorder()
	if err := writeWebhookText(rec, http.StatusAccepted, "queued"); err != nil {
		t.Fatalf("writeWebhookText() error = %v", err)
	}
	if got, want := rec.Code, http.StatusAccepted; got != want {
		t.Fatalf("writeWebhookText status = %d, want %d", got, want)
	}
	if got, want := strings.TrimSpace(rec.Body.String()), "queued"; got != want {
		t.Fatalf("writeWebhookText body = %q, want %q", got, want)
	}
	if err := classifyLinearTransportError(context.Canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("classifyLinearTransportError(context.Canceled) = %v, want context.Canceled", err)
	}
}

func TestHandleShutdownAndHandleBridgesDeliverErrorPaths(t *testing.T) {
	env := setLinearProviderTestEnv(t)

	provider, err := newLinearProvider(io.Discard)
	if err != nil {
		t.Fatalf("newLinearProvider() error = %v", err)
	}
	if err := provider.startServer(reserveLinearListenAddr(t)); err != nil {
		t.Fatalf("startServer() error = %v", err)
	}

	provider.lifecycle.Stop()
	_, err = provider.handleBridgesDeliver(context.Background(), nil, linearTestDeliveryRequest(
		"missing-instance",
		"delivery-missing",
		1,
		bridgepkg.DeliveryEventTypeStart,
		linearThreadRef{IssueID: "issue-1", RootCommentID: "root-1"},
		"hello",
		linearModeComments,
	))
	if err == nil {
		t.Fatal("handleBridgesDeliver(missing config) error = nil, want non-nil")
	}
	records := waitForLinearJSONLinesFile[bridgesdk.DeliveryMarker](
		t,
		env.DeliveryPath,
		func(items []bridgesdk.DeliveryMarker) bool { return len(items) == 1 },
	)
	if strings.TrimSpace(records[0].Error) == "" {
		t.Fatalf("delivery marker = %#v, want recorded error", records[0])
	}

	if err := provider.lifecycle.Shutdown(
		context.Background(),
		nil,
		subprocess.ShutdownRequest{DeadlineMS: 100},
	); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	lines := waitForLinearNonEmptyLines(t, env.ShutdownPath)
	if len(lines) == 0 || !strings.Contains(lines[0], "pid=") {
		t.Fatalf("shutdown marker lines = %#v, want pid line", lines)
	}
	select {
	case <-provider.lifecycle.StopChannel():
	default:
		t.Fatal("provider lifecycle is not stopped after shutdown")
	}
}

func TestDetermineInitialStateAdditionalBranches(t *testing.T) {
	t.Parallel()

	provider := &linearProvider{
		apiFactory: func(resolvedInstanceConfig) linearAPI {
			return linearFakeAPI{
				err: &bridgesdk.RateLimitError{
					Err:        errors.New("rate limited"),
					RetryAfter: time.Minute,
				},
			}
		},
	}

	ctx := context.Background()

	_, status, degradation, err := provider.determineInitialState(ctx, resolvedInstanceConfig{
		instanceID:  "brg-config-error",
		configError: errors.New("bad config"),
	})
	if err == nil {
		t.Fatal("determineInitialState(config error) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusDegraded; got != want {
		t.Fatalf("config error status = %q, want %q", got, want)
	}
	if degradation == nil || degradation.Reason != bridgepkg.BridgeDegradationReasonTenantConfigInvalid {
		t.Fatalf("config error degradation = %#v, want tenant_config_invalid", degradation)
	}

	_, status, degradation, err = provider.determineInitialState(ctx, resolvedInstanceConfig{
		instanceID:     "brg-missing-secret",
		organizationID: "org-1",
		mode:           linearModeComments,
		authMode:       linearAuthModeAPIKey,
		apiKey:         "linear-api-key",
	})
	if err == nil {
		t.Fatal("determineInitialState(missing webhook secret) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("missing webhook secret status = %q, want %q", got, want)
	}
	if degradation == nil || degradation.Reason != bridgepkg.BridgeDegradationReasonAuthFailed {
		t.Fatalf("missing webhook secret degradation = %#v, want auth_failed", degradation)
	}

	_, status, degradation, err = provider.determineInitialState(ctx, resolvedInstanceConfig{
		instanceID:     "brg-rate-limited",
		organizationID: "org-1",
		mode:           linearModeComments,
		authMode:       linearAuthModeAPIKey,
		webhookSecret:  "webhook-secret",
		apiKey:         "linear-api-key",
	})
	if err == nil {
		t.Fatal("determineInitialState(rate limited) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusDegraded; got != want {
		t.Fatalf("rate limited status = %q, want %q", got, want)
	}
	if degradation == nil || degradation.Reason != bridgepkg.BridgeDegradationReasonRateLimited {
		t.Fatalf("rate limited degradation = %#v, want rate_limited", degradation)
	}
}

func TestHandleWebhookRequestBranchesAndThreadDecoding(t *testing.T) {
	t.Parallel()

	provider, err := newLinearProvider(io.Discard)
	if err != nil {
		t.Fatalf("newLinearProvider() error = %v", err)
	}

	commentManaged := linearRuntimeManagedInstance(
		time.Now().UTC(),
		"brg-comments",
		"org-comments",
		linearModeComments,
		linearAuthModeAPIKey,
		"127.0.0.1:0",
	)
	commentCfg := resolvedInstanceConfig{
		instanceID:      "brg-comments",
		organizationID:  "org-comments",
		mode:            linearModeComments,
		webhookPath:     "/linear",
		webhookSecret:   linearProviderWebhookSecretValue,
		botUserID:       "bot-comments",
		dedup:           bridgesdk.NewDedupCache(5*time.Minute, 4000),
		oauthTokenCache: &linearOAuthTokenCache{},
		managed:         &commentManaged,
	}
	agentManaged := linearRuntimeManagedInstance(
		time.Now().UTC(),
		"brg-agent",
		"org-agent",
		linearModeAgentSessions,
		linearAuthModeOAuth,
		"127.0.0.1:0",
	)
	agentCfg := resolvedInstanceConfig{
		instanceID:      "brg-agent",
		organizationID:  "org-agent",
		mode:            linearModeAgentSessions,
		webhookPath:     "/linear",
		webhookSecret:   linearProviderWebhookSecretValue,
		botUserID:       "bot-agent",
		dedup:           bridgesdk.NewDedupCache(5*time.Minute, 4000),
		oauthTokenCache: &linearOAuthTokenCache{},
		managed:         &agentManaged,
	}

	rec := httptest.NewRecorder()
	err = provider.handleWebhookRequest(rec, nil, []resolvedInstanceConfig{commentCfg}, bridgesdk.WebhookRequest{
		Body:       []byte(`{"type":"Comment"`),
		ReceivedAt: time.Date(2026, 4, 15, 13, 20, 0, 0, time.UTC),
	})
	var httpErr *bridgesdk.HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("handleWebhookRequest(invalid json) error = %#v, want 400 http error", err)
	}

	rec = httptest.NewRecorder()
	now := time.Date(2026, 4, 15, 13, 20, 0, 0, time.UTC)
	unknownBody := mustJSONMarshal(t, map[string]any{
		"type":             "Unknown",
		"organizationId":   "org-comments",
		"webhookTimestamp": now.UnixMilli(),
	})
	if err := provider.handleWebhookRequest(
		rec,
		nil,
		[]resolvedInstanceConfig{commentCfg},
		bridgesdk.WebhookRequest{Body: unknownBody, ReceivedAt: now},
	); err != nil {
		t.Fatalf("handleWebhookRequest(unknown type) error = %v", err)
	}
	if got, want := strings.TrimSpace(rec.Body.String()), "ok"; got != want {
		t.Fatalf("unknown type body = %q, want %q", got, want)
	}

	rec = httptest.NewRecorder()
	ignoredBody := mustJSONMarshal(
		t,
		linearCommentWebhookBodyForTest(now, "other-org", "user-1", "reply-2", "root-comment", "ignored"),
	)
	if err := provider.handleWebhookRequest(
		rec,
		nil,
		[]resolvedInstanceConfig{commentCfg},
		bridgesdk.WebhookRequest{Body: ignoredBody, ReceivedAt: now},
	); err != nil {
		t.Fatalf("handleWebhookRequest(ignored org) error = %v", err)
	}
	if got, want := strings.TrimSpace(rec.Body.String()), "ignored"; got != want {
		t.Fatalf("ignored org body = %q, want %q", got, want)
	}

	rec = httptest.NewRecorder()
	commentUpdate := linearCommentWebhookBodyForTest(
		now,
		"org-comments",
		"user-1",
		"reply-3",
		"root-comment",
		"updated",
	)
	commentUpdate["action"] = "update"
	updateBody := mustJSONMarshal(t, commentUpdate)
	if err := provider.handleWebhookRequest(
		rec,
		nil,
		[]resolvedInstanceConfig{commentCfg},
		bridgesdk.WebhookRequest{Body: updateBody, ReceivedAt: now},
	); err != nil {
		t.Fatalf("handleWebhookRequest(comment update) error = %v", err)
	}
	if got, want := strings.TrimSpace(rec.Body.String()), "ok"; got != want {
		t.Fatalf("comment update body = %q, want %q", got, want)
	}

	rec = httptest.NewRecorder()
	agentInvalid := mustJSONMarshal(t, map[string]any{
		"type":             "AgentSessionEvent",
		"action":           "created",
		"organizationId":   "org-agent",
		"webhookTimestamp": now.UnixMilli(),
		"agentSession": map[string]any{
			"id":        "session-1",
			"issueId":   "issue-1",
			"commentId": "comment-1",
		},
	})
	err = provider.handleWebhookRequest(
		rec,
		nil,
		[]resolvedInstanceConfig{agentCfg},
		bridgesdk.WebhookRequest{Body: agentInvalid, ReceivedAt: now},
	)
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("handleWebhookRequest(invalid agent payload) error = %#v, want 400 http error", err)
	}

	if got, err := decodeLinearThreadID(
		"linear:issue-1:s:session-1",
	); err != nil || got.IssueID != "issue-1" ||
		got.AgentSessionID != "session-1" {
		t.Fatalf("decodeLinearThreadID(issue-session) = (%#v, %v), want issue-1/session-1", got, err)
	}
	if got, err := decodeLinearThreadID(
		"linear:issue-1",
	); err != nil || got.IssueID != "issue-1" || got.RootCommentID != "" ||
		got.AgentSessionID != "" {
		t.Fatalf("decodeLinearThreadID(issue) = (%#v, %v), want issue only", got, err)
	}
	if _, err := decodeLinearThreadID("nope"); err == nil {
		t.Fatal("decodeLinearThreadID(invalid) error = nil, want non-nil")
	}

	createdMapped, ignored, err := mapLinearAgentSessionEvent(linearAgentSessionWebhookPayload{
		Type:             "AgentSessionEvent",
		Action:           "created",
		OrganizationID:   "org-agent",
		WebhookID:        "webhook-created",
		WebhookTimestamp: now.UnixMilli(),
		AgentSession: linearAgentSession{
			ID:              "session-created",
			AppUserID:       "bot-agent",
			IssueID:         "issue-created",
			CommentID:       "comment-created",
			SourceCommentID: "comment-created",
			Comment: &linearSessionComment{
				ID:     "comment-created",
				Body:   "Start here",
				UserID: "user-created",
			},
			Creator: &linearActor{
				ID:   "user-created",
				Name: "Alice Example",
				URL:  "https://linear.app/acme/profiles/alice",
			},
		},
	}, agentCfg.managed, now, "bot-agent")
	if err != nil {
		t.Fatalf("mapLinearAgentSessionEvent(created) error = %v", err)
	}
	if ignored {
		t.Fatal("mapLinearAgentSessionEvent(created) ignored = true, want false")
	}
	if got, want := createdMapped.Envelope.ThreadID, "linear:issue-created:c:comment-created:s:session-created"; got != want {
		t.Fatalf("created thread id = %q, want %q", got, want)
	}

	_, ignored, err = mapLinearAgentSessionEvent(linearAgentSessionWebhookPayload{
		Type:             "AgentSessionEvent",
		Action:           "completed",
		OrganizationID:   "org-agent",
		WebhookTimestamp: now.UnixMilli(),
	}, agentCfg.managed, now, "bot-agent")
	if err != nil {
		t.Fatalf("mapLinearAgentSessionEvent(ignored action) error = %v", err)
	}
	if !ignored {
		t.Fatal("mapLinearAgentSessionEvent(ignored action) ignored = false, want true")
	}
}

func TestExecuteLinearDeliveryEdgeCasesAndClassifiers(t *testing.T) {
	t.Parallel()

	api := &recordingLinearAPI{
		viewer: &linearViewer{
			ID:             "bot-comments",
			OrganizationID: "org-comments",
		},
	}

	commentReq := linearTestDeliveryRequest(
		"brg-linear-comments",
		"delivery-comment-edge",
		1,
		bridgepkg.DeliveryEventTypeResume,
		linearThreadRef{
			IssueID:       "issue-comments",
			RootCommentID: "root-comment",
		},
		"",
		linearModeComments,
	)
	commentReq.Event.Reference = &bridgepkg.DeliveryMessageReference{RemoteMessageID: "remote-comment"}
	ack, state, err := executeLinearDelivery(
		context.Background(),
		api,
		resolvedInstanceConfig{mode: linearModeComments},
		commentReq,
		deliveryState{},
	)
	if err != nil {
		t.Fatalf("executeLinearDelivery(comment resume) error = %v", err)
	}
	if got, want := ack.RemoteMessageID, "remote-comment"; got != want {
		t.Fatalf("comment resume remote id = %q, want %q", got, want)
	}
	if got, want := state.RemoteMessageID, "remote-comment"; got != want {
		t.Fatalf("comment resume state remote id = %q, want %q", got, want)
	}

	deleteReq := commentReq
	deleteReq.Event.EventType = bridgepkg.DeliveryEventTypeDelete
	deleteReq.Event.Operation = bridgepkg.DeliveryOperationDelete
	deleteReq.Event.Reference = nil
	if _, _, err := executeLinearDelivery(
		context.Background(),
		api,
		resolvedInstanceConfig{mode: linearModeComments},
		deleteReq,
		deliveryState{},
	); err == nil {
		t.Fatal("executeLinearDelivery(comment delete missing remote id) error = nil, want non-nil")
	}

	if _, _, err := executeLinearDelivery(
		context.Background(),
		api,
		resolvedInstanceConfig{mode: "unsupported"},
		commentReq,
		deliveryState{},
	); err == nil {
		t.Fatal("executeLinearDelivery(unsupported mode) error = nil, want non-nil")
	}
	if _, _, err := executeLinearDelivery(
		context.Background(),
		api,
		resolvedInstanceConfig{mode: linearModeComments},
		commentReq,
		deliveryState{LastSeq: 2},
	); err == nil {
		t.Fatal("executeLinearDelivery(out of order) error = nil, want non-nil")
	}

	agentReq := linearTestDeliveryRequest(
		"brg-linear-agent",
		"delivery-agent-edge",
		1,
		bridgepkg.DeliveryEventTypeResume,
		linearThreadRef{
			IssueID:        "issue-agent",
			RootCommentID:  "comment-root",
			AgentSessionID: "session-123",
		},
		"hello",
		linearModeAgentSessions,
	)
	agentReq.Event.Reference = &bridgepkg.DeliveryMessageReference{RemoteMessageID: "remote-agent"}
	ack, state, err = executeLinearDelivery(
		context.Background(),
		api,
		resolvedInstanceConfig{mode: linearModeAgentSessions},
		agentReq,
		deliveryState{},
	)
	if err != nil {
		t.Fatalf("executeLinearDelivery(agent resume) error = %v", err)
	}
	if got, want := ack.RemoteMessageID, "remote-agent"; got != want {
		t.Fatalf("agent resume remote id = %q, want %q", got, want)
	}
	if got, want := state.RemoteMessageID, "remote-agent"; got != want {
		t.Fatalf("agent resume state remote id = %q, want %q", got, want)
	}

	agentEdit := agentReq
	agentEdit.Event.Operation = bridgepkg.DeliveryOperationEdit
	if _, _, err := executeLinearDelivery(
		context.Background(),
		api,
		resolvedInstanceConfig{mode: linearModeAgentSessions},
		agentEdit,
		deliveryState{},
	); err == nil {
		t.Fatal("executeLinearDelivery(agent edit) error = nil, want non-nil")
	}

	if err := classifyLinearTransportError(nil); err != nil {
		t.Fatalf("classifyLinearTransportError(nil) = %v, want nil", err)
	}
	if _, ok := classifyLinearHTTPError(http.StatusUnauthorized, []byte("forbidden")).(*bridgesdk.AuthError); !ok {
		t.Fatalf("classifyLinearHTTPError(401) did not return auth error")
	}
	if _, ok := classifyLinearHTTPError(http.StatusTooManyRequests, []byte("rate limited")).(*bridgesdk.RateLimitError); !ok {
		t.Fatalf("classifyLinearHTTPError(429) did not return rate limit error")
	}
	if _, ok := classifyLinearHTTPError(http.StatusBadGateway, []byte("unavailable")).(*bridgesdk.TransientError); !ok {
		t.Fatalf("classifyLinearHTTPError(502) did not return transient error")
	}
	if _, ok := classifyLinearHTTPError(http.StatusBadRequest, []byte("bad request")).(*bridgesdk.PermanentError); !ok {
		t.Fatalf("classifyLinearHTTPError(400) did not return permanent error")
	}
	if httpErr, ok := classifyLinearHTTPError(http.StatusRequestTimeout, nil).(*bridgesdk.HTTPError); !ok ||
		httpErr.StatusCode != http.StatusRequestTimeout {
		t.Fatalf("classifyLinearHTTPError(408) = %#v, want request-timeout http error", httpErr)
	}
	if _, ok := classifyLinearTransportError(context.DeadlineExceeded).(*bridgesdk.HTTPError); !ok {
		t.Fatalf("classifyLinearTransportError(deadline) did not return http error")
	}
	if _, ok := classifyLinearTransportError(context.Canceled).(*bridgesdk.TransientError); !ok {
		t.Fatalf("classifyLinearTransportError(canceled) did not return transient error")
	}
	if _, ok := classifyLinearTransportError(&net.DNSError{IsTimeout: true}).(*bridgesdk.HTTPError); !ok {
		t.Fatalf("classifyLinearTransportError(timeout) did not return http error")
	}
	if _, ok := classifyLinearTransportError(errors.New("boom")).(*bridgesdk.TransientError); !ok {
		t.Fatalf("classifyLinearTransportError(generic) did not return transient error")
	}
	if got, want := issueThreadIDFromGroup("", "issue-fallback"), "linear:issue-fallback"; got != want {
		t.Fatalf("issueThreadIDFromGroup() = %q, want %q", got, want)
	}
	if got, want := linearUserName("https://linear.app/acme/profiles/alice"), "alice"; got != want {
		t.Fatalf("linearUserName() = %q, want %q", got, want)
	}
}

func TestLinearRunHelpers(t *testing.T) {
	t.Parallel()

	if err := run([]string{"bad"}, strings.NewReader(""), io.Discard, io.Discard); err == nil {
		t.Fatal("run(unsupported) error = nil, want non-nil")
	}
	if err := runServe(strings.NewReader(""), io.Discard, io.Discard); err != nil {
		t.Fatalf("runServe(empty input) error = %v", err)
	}
}

func TestNewLinearProviderDefaultsAndNotFoundWebhook(t *testing.T) {
	t.Parallel()

	provider, err := newLinearProvider(nil)
	if err != nil {
		t.Fatalf("newLinearProvider(nil) error = %v", err)
	}
	if provider.stderr == nil {
		t.Fatal("newLinearProvider(nil) stderr = nil, want non-nil writer")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"http://example.test/not-linear",
		http.NoBody,
	)
	provider.serveWebhookHTTP(rec, req)
	if got, want := rec.Code, http.StatusNotFound; got != want {
		t.Fatalf("serveWebhookHTTP(not found) status = %d, want %d", got, want)
	}
}

const linearProviderWebhookSecretValue = "linear-webhook-secret"

func newLinearRuntimePeerPair(t *testing.T) (*linearProvider, *bridgesdk.Peer, func()) {
	t.Helper()

	hostConn, runtimeConn := net.Pipe()
	runtime, err := newLinearProvider(io.Discard)
	if err != nil {
		t.Fatalf("newLinearProvider() error = %v", err)
	}

	hostPeer := bridgesdk.NewPeer(hostConn, hostConn)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 2)
	go func() { errCh <- runtime.serve(runtimeConn, runtimeConn) }()
	go func() { errCh <- hostPeer.Serve(ctx) }()

	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			cancel()
			runtime.lifecycle.Stop()
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
			if err := runtime.http.Shutdown(shutdownCtx); err != nil {
				t.Errorf("runtime HTTP shutdown error = %v", err)
			}
			shutdownCancel()
			if err := hostConn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("host connection close error = %v", err)
			}
			if err := runtimeConn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("runtime connection close error = %v", err)
			}
			for range 2 {
				err := <-errCh
				if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, net.ErrClosed) {
					continue
				}
				if strings.Contains(err.Error(), "closed") {
					continue
				}
				t.Fatalf("runtime peer serve error = %v", err)
			}
			waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
			defer waitCancel()
			if err := runtime.lifecycle.Wait(waitCtx); err != nil {
				t.Errorf("runtime lifecycle wait error = %v", err)
			}
		})
	}

	return runtime, hostPeer, cleanup
}

func mustHandleLinear(t *testing.T, peer *bridgesdk.Peer, method string, handler bridgesdk.RPCHandler) {
	t.Helper()
	if err := peer.Handle(method, handler); err != nil {
		t.Fatalf("peer.Handle(%q) error = %v", method, err)
	}
}

func mustHandleLinearLifecycle(
	t *testing.T,
	peer *bridgesdk.Peer,
	managed ...subprocess.InitializeBridgeManagedInstance,
) {
	t.Helper()

	mustHandleLinear(
		t,
		peer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesList),
		func(context.Context, json.RawMessage) (any, error) {
			instances := make([]bridgepkg.BridgeInstance, 0, len(managed))
			for _, item := range managed {
				instances = append(instances, item.Instance)
			}
			return instances, nil
		},
	)
	mustHandleLinear(
		t,
		peer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
		func(_ context.Context, params json.RawMessage) (any, error) {
			var payload bridgepkg.BridgeInstanceTargetParams
			if err := json.Unmarshal(params, &payload); err != nil {
				return nil, err
			}
			for _, item := range managed {
				if item.Instance.ID == payload.BridgeInstanceID {
					return item.Instance, nil
				}
			}
			return nil, errors.New("unexpected instance")
		},
	)
	mustHandleLinear(
		t,
		peer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		func(_ context.Context, params json.RawMessage) (any, error) {
			var payload bridgepkg.BridgesInstancesReportStateParams
			if err := json.Unmarshal(params, &payload); err != nil {
				return nil, err
			}
			for _, item := range managed {
				if item.Instance.ID == payload.BridgeInstanceID {
					instance := item.Instance
					instance.Status = payload.Status
					instance.Degradation = payload.Degradation
					return instance, nil
				}
			}
			return nil, errors.New("unexpected state instance")
		},
	)
}

func linearRuntimeManagedInstance(
	now time.Time,
	instanceID string,
	organizationID string,
	mode string,
	authMode string,
	listenAddr string,
) subprocess.InitializeBridgeManagedInstance {
	providerConfig := fmt.Sprintf(`{
		"organization_id": %q,
		"mode": %q,
		"auth_mode": %q,
		"webhook": {
			"listen_addr": %q,
			"path": "/linear"
		}
	}`, organizationID, mode, authMode, listenAddr)

	secrets := []subprocess.InitializeBridgeBoundSecret{
		{BindingName: "webhook_secret", Kind: "token", Value: linearProviderWebhookSecretValue},
	}
	switch authMode {
	case linearAuthModeAPIKey:
		secrets = append(
			secrets,
			subprocess.InitializeBridgeBoundSecret{BindingName: "api_key", Kind: "token", Value: "linear-api-key"},
		)
	case linearAuthModeOAuth:
		secrets = append(
			secrets,
			subprocess.InitializeBridgeBoundSecret{BindingName: "client_id", Kind: "token", Value: "linear-client-id"},
			subprocess.InitializeBridgeBoundSecret{
				BindingName: "client_secret",
				Kind:        "token",
				Value:       "linear-client-secret",
			},
		)
	}

	return subprocess.InitializeBridgeManagedInstance{
		Instance: bridgepkg.BridgeInstance{
			ID:             instanceID,
			Scope:          bridgepkg.ScopeWorkspace,
			WorkspaceID:    "ws-linear",
			Platform:       "linear",
			ExtensionName:  "linear",
			DisplayName:    "Linear",
			Source:         bridgepkg.BridgeInstanceSourceDynamic,
			Enabled:        true,
			Status:         bridgepkg.BridgeStatusReady,
			RoutingPolicy:  bridgepkg.RoutingPolicy{IncludeThread: true, IncludeGroup: true},
			ProviderConfig: []byte(providerConfig),
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		BoundSecrets: secrets,
	}
}

func linearRuntimeManagedInstanceWithWebhookSecret(
	now time.Time,
	instanceID string,
	organizationID string,
	mode string,
	authMode string,
	listenAddr string,
	webhookSecret string,
) subprocess.InitializeBridgeManagedInstance {
	managed := linearRuntimeManagedInstance(now, instanceID, organizationID, mode, authMode, listenAddr)
	for idx := range managed.BoundSecrets {
		if managed.BoundSecrets[idx].BindingName == "webhook_secret" {
			managed.BoundSecrets[idx].Value = webhookSecret
			break
		}
	}
	return managed
}

func linearInitializeRequest(
	_ time.Time,
	managed ...subprocess.InitializeBridgeManagedInstance,
) subprocess.InitializeRequest {
	return subprocess.InitializeRequest{
		ProtocolVersion:          "1",
		SupportedProtocolVersion: []string{"1"},
		AGHVersion:               "0.5.0",
		SessionNonce:             "nonce-test",
		Extension: subprocess.InitializeExtension{
			Name:       "linear",
			Version:    "0.1.0",
			SourceTier: "user",
		},
		Capabilities: subprocess.InitializeCapabilities{
			Provides: []string{"bridge.adapter"},
			GrantedActions: []extensionprotocol.HostAPIMethod{
				extensionprotocol.HostAPIMethodBridgesInstancesList,
				extensionprotocol.HostAPIMethodBridgesInstancesGet,
				extensionprotocol.HostAPIMethodBridgesInstancesReportState,
				extensionprotocol.HostAPIMethodBridgesMessagesIngest,
			},
			GrantedSecurity: []string{"bridge.read", "bridge.write"},
		},
		Methods: subprocess.InitializeMethods{
			ExtensionServices: []string{"bridges/deliver", "health_check", "shutdown"},
		},
		Runtime: subprocess.InitializeRuntime{
			HealthCheckIntervalMS: 30_000,
			HealthCheckTimeoutMS:  5_000,
			ShutdownTimeoutMS:     5_000,
			DefaultHookTimeoutMS:  5_000,
			Bridge: &subprocess.InitializeBridgeRuntime{
				RuntimeVersion:   subprocess.InitializeBridgeRuntimeVersion2,
				Purpose:          subprocess.BridgeRuntimePurposeService,
				Provider:         "linear",
				Platform:         "linear",
				ManagedInstances: managed,
			},
		},
	}
}

func setLinearProviderTestEnv(t *testing.T) bridgesdk.AdapterMarkerPaths {
	t.Helper()

	root := filepath.Join(t.TempDir(), "markers")
	env := bridgesdk.AdapterMarkerPaths{
		HandshakePath: filepath.Join(root, "handshake.json"),
		OwnershipPath: filepath.Join(root, "ownership.json"),
		StatePath:     filepath.Join(root, "state.jsonl"),
		DeliveryPath:  filepath.Join(root, "delivery.jsonl"),
		IngestPath:    filepath.Join(root, "ingest.jsonl"),
		StartsPath:    filepath.Join(root, "starts.log"),
		ShutdownPath:  filepath.Join(root, "shutdown.log"),
		CrashOncePath: filepath.Join(root, "crash-once.json"),
	}

	t.Setenv(bridgesdk.AdapterHandshakePathEnv, env.HandshakePath)
	t.Setenv(bridgesdk.AdapterOwnershipPathEnv, env.OwnershipPath)
	t.Setenv(bridgesdk.AdapterStatePathEnv, env.StatePath)
	t.Setenv(bridgesdk.AdapterDeliveryPathEnv, env.DeliveryPath)
	t.Setenv(bridgesdk.AdapterIngestPathEnv, env.IngestPath)
	t.Setenv(bridgesdk.AdapterStartsPathEnv, env.StartsPath)
	t.Setenv(bridgesdk.AdapterShutdownPathEnv, env.ShutdownPath)
	t.Setenv(bridgesdk.AdapterCrashOncePathEnv, "")

	return env
}

func reserveLinearListenAddr(t *testing.T) string {
	t.Helper()

	var listenConfig net.ListenConfig
	ln, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenConfig.Listen() error = %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("ln.Close() error = %v", err)
	}
	return addr
}

func linearRuntimeServerAddr(runtime *linearProvider) string {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return runtime.http.Address()
}

func linearCommentWebhookBodyForTest(
	now time.Time,
	organizationID string,
	userID string,
	commentID string,
	parentID string,
	body string,
) map[string]any {
	return map[string]any{
		"type":             "Comment",
		"action":           "create",
		"organizationId":   organizationID,
		"webhookId":        "webhook-comment-" + commentID,
		"webhookTimestamp": now.UnixMilli(),
		"url":              "https://linear.app/acme/issue/TEST-1#" + commentID,
		"data": map[string]any{
			"id":        commentID,
			"body":      body,
			"issueId":   "issue-comments",
			"userId":    userID,
			"createdAt": now.Format(time.RFC3339),
			"updatedAt": now.Format(time.RFC3339),
			"parentId":  parentID,
			"user": map[string]any{
				"id":   userID,
				"name": "Alice Example",
				"url":  "https://linear.app/acme/profiles/alice",
			},
		},
		"actor": map[string]any{
			"id":   userID,
			"name": "Alice Example",
		},
	}
}

func linearAgentSessionWebhookBodyForTest(
	now time.Time,
	organizationID string,
	sessionID string,
	commentID string,
	sourceCommentID string,
	body string,
) map[string]any {
	return map[string]any{
		"type":             "AgentSessionEvent",
		"action":           "prompted",
		"organizationId":   organizationID,
		"webhookId":        "webhook-agent-" + sessionID,
		"webhookTimestamp": now.UnixMilli(),
		"promptContext":    "TEST-2\n\n@get-bot " + body,
		"agentSession": map[string]any{
			"id":              sessionID,
			"appUserId":       "bot-agent",
			"issueId":         "issue-agent",
			"commentId":       commentID,
			"sourceCommentId": sourceCommentID,
		},
		"agentActivity": map[string]any{
			"id":        "activity-1",
			"body":      body,
			"createdAt": now.Format(time.RFC3339),
			"content": map[string]any{
				"type": "prompt",
				"body": body,
			},
		},
		"actor": map[string]any{
			"id":   "user-agent",
			"name": "Bob Example",
			"url":  "https://linear.app/acme/profiles/bob",
		},
	}
}

func postLinearTestWebhook(t *testing.T, webhookURL string, payload map[string]any, secret string) *http.Response {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(payload) error = %v", err)
	}
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		webhookURL,
		strings.NewReader(string(body)),
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("linear-signature", linearSignature(secret, body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http.DefaultClient.Do() error = %v", err)
	}
	return resp
}

func waitForLinearJSONFile[T any](t *testing.T, path string) T {
	t.Helper()

	var item T
	waitForLinearCondition(t, func() bool {
		payload, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		return json.Unmarshal(payload, &item) == nil
	})
	return item
}

func waitForLinearJSONLinesFile[T any](t *testing.T, path string, predicate func([]T) bool) []T {
	t.Helper()

	var items []T
	waitForLinearCondition(t, func() bool {
		payload, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		lines := linearNonEmptyLines(string(payload))
		decoded := make([]T, 0, len(lines))
		for _, line := range lines {
			var item T
			if err := json.Unmarshal([]byte(line), &item); err != nil {
				return false
			}
			decoded = append(decoded, item)
		}
		items = decoded
		return predicate(items)
	})
	return items
}

func waitForLinearNonEmptyLines(t *testing.T, path string) []string {
	t.Helper()

	var lines []string
	waitForLinearCondition(t, func() bool {
		payload, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		lines = linearNonEmptyLines(string(payload))
		return len(lines) > 0
	})
	return lines
}

func waitForLinearCondition(t *testing.T, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition did not succeed before timeout")
}

func linearNonEmptyLines(input string) []string {
	lines := strings.Split(input, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return filtered
}

func mustJSONMarshal(t *testing.T, value any) []byte {
	t.Helper()

	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return payload
}

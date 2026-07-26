package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/subprocess"
)

func TestWebhookIngressTimeout(t *testing.T) {
	// not parallel: setLinearProviderTestEnv mutates process environment for marker paths.
	t.Run("Should time out stalled host API and release in-flight slot", func(t *testing.T) {
		setLinearProviderTestEnv(t)
		listenAddr := reserveLinearListenAddr(t)
		now := time.Date(2026, 4, 15, 13, 8, 0, 0, time.UTC)

		runtime, hostPeer, cleanup := newLinearRuntimePeerPair(t)
		defer cleanup()
		runtime.now = func() time.Time { return now }
		runtime.webhookIngressTimeout = 50 * time.Millisecond
		runtime.inFlight = bridgesdk.NewInFlightLimiter(1)
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
		}
		mustHandleLinearLifecycle(t, hostPeer, managed...)

		release := make(chan struct{})
		var releaseOnce sync.Once
		t.Cleanup(func() {
			releaseOnce.Do(func() { close(release) })
		})

		var (
			mu          sync.Mutex
			ingestCalls int
		)
		mustHandleLinear(
			t,
			hostPeer,
			string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
			func(ctx context.Context, params json.RawMessage) (any, error) {
				var envelope bridgepkg.InboundMessageEnvelope
				if err := json.Unmarshal(params, &envelope); err != nil {
					return nil, err
				}
				mu.Lock()
				ingestCalls++
				mu.Unlock()

				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-release:
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
				}
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
			"org-comments",
			"user-1",
			"reply-timeout",
			"root-comment",
			"Need a bounded ingress call",
		)
		client := &http.Client{Timeout: time.Second}

		first := postLinearTestWebhookWithClient(t, client, webhookURL, payload, linearProviderWebhookSecretValue)
		firstBody := readLinearTestResponseBody(t, first)
		if got, want := first.StatusCode, http.StatusGatewayTimeout; got != want {
			t.Fatalf("stalled webhook status = %d body = %q, want %d", got, firstBody, want)
		}
		if !strings.Contains(firstBody, "timed out") {
			t.Fatalf("stalled webhook body = %q, want timeout text", firstBody)
		}

		releaseOnce.Do(func() { close(release) })
		secondPayload := linearCommentWebhookBodyForTest(
			now,
			"org-comments",
			"user-1",
			"reply-after-timeout",
			"root-comment",
			"Proceed after timeout",
		)
		second := postLinearTestWebhookWithClient(
			t,
			client,
			webhookURL,
			secondPayload,
			linearProviderWebhookSecretValue,
		)
		secondBody := readLinearTestResponseBody(t, second)
		if got, want := second.StatusCode, http.StatusOK; got != want {
			t.Fatalf("second webhook status = %d body = %q, want %d", got, secondBody, want)
		}

		mu.Lock()
		gotCalls := ingestCalls
		mu.Unlock()
		if gotCalls < 2 {
			t.Fatalf("ingest calls = %d, want at least 2 to prove the in-flight slot was released", gotCalls)
		}
	})
}

func postLinearTestWebhookWithClient(
	t *testing.T,
	client *http.Client,
	webhookURL string,
	payload map[string]any,
	secret string,
) *http.Response {
	t.Helper()
	if client == nil {
		t.Fatal("postLinearTestWebhookWithClient client is nil")
	}

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
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("http.Client.Do() error = %v", err)
	}
	return resp
}

func readLinearTestResponseBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	if resp == nil {
		t.Fatal("readLinearTestResponseBody response is nil")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("io.ReadAll(resp.Body) error = %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("resp.Body.Close() error = %v", err)
	}
	return strings.TrimSpace(string(body))
}

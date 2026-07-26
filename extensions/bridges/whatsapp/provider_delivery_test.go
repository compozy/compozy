package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/subprocess"
)

func TestWhatsAppHealthStateIsolation(t *testing.T) {
	// not parallel: setProviderTestEnv mutates process environment for marker paths.
	t.Run("Should keep unresolved instance health error after another instance succeeds", func(t *testing.T) {
		setProviderTestEnv(t)
		listenAddr := reserveListenAddr(t)
		t.Setenv(whatsappListenAddrEnv, listenAddr)

		now := time.Date(2026, 5, 16, 23, 10, 0, 0, time.UTC)
		runtime, hostPeer, cleanup := newRuntimePeerPair(t)
		defer cleanup()
		runtime.apiFactory = func(resolvedInstanceConfig) whatsappAPI {
			return &fakeWhatsAppAPI{nextMessageID: 100}
		}

		managed := []subprocess.InitializeBridgeManagedInstance{
			testBridgeRuntime(now, "brg-1"),
			testBridgeRuntime(now, "brg-2"),
		}
		mustHandleLifecycle(t, hostPeer, managed...)
		recordWhatsAppIngests(t, hostPeer)
		initializeWhatsAppRuntimeForDeliveryTest(t, runtime, hostPeer, now, managed...)

		runtime.setLastError(errors.New("whatsapp: delivery failed for brg-1"))
		if err := runtime.dispatchInboundEnvelope(
			context.Background(),
			"brg-2",
			testWhatsAppInboundEnvelope(now, "brg-2", "wamid.health", "healthy peer"),
		); err != nil {
			t.Fatalf("dispatchInboundEnvelope(brg-2) error = %v", err)
		}

		err := runtime.healthCheck()
		if err == nil {
			t.Fatal("healthCheck() error = nil, want unresolved brg-1 health error")
		}
		if !strings.Contains(err.Error(), "brg-1") {
			t.Fatalf("healthCheck() error = %q, want brg-1 failure", err.Error())
		}
	})
}

func TestWhatsAppInboundBatchRichPayloadPreservation(t *testing.T) {
	// not parallel: setProviderTestEnv mutates process environment for marker paths.
	t.Run("Should dispatch rich batched envelopes individually without losing metadata", func(t *testing.T) {
		setProviderTestEnv(t)
		listenAddr := reserveListenAddr(t)
		t.Setenv(whatsappListenAddrEnv, listenAddr)

		now := time.Date(2026, 5, 16, 23, 20, 0, 0, time.UTC)
		runtime, hostPeer, cleanup := newRuntimePeerPair(t)
		defer cleanup()
		runtime.apiFactory = func(resolvedInstanceConfig) whatsappAPI {
			return &fakeWhatsAppAPI{nextMessageID: 200}
		}

		managed := testBridgeRuntime(now, "brg-1")
		mustHandleLifecycle(t, hostPeer, managed)
		ingested := recordWhatsAppIngests(t, hostPeer)
		initializeWhatsAppRuntimeForDeliveryTest(t, runtime, hostPeer, now, managed)

		batch := bridgesdk.InboundBatch{
			Key: "batch-rich",
			Items: []bridgepkg.InboundMessageEnvelope{
				testWhatsAppInboundEnvelope(now, "brg-1", "wamid.1", "first"),
				func() bridgepkg.InboundMessageEnvelope {
					envelope := testWhatsAppInboundEnvelope(
						now.Add(time.Second),
						"brg-1",
						"wamid.2",
						"second",
					)
					envelope.Attachments = []bridgepkg.MessageAttachment{
						{
							ID:       "att-2",
							Name:     "invoice.pdf",
							MIMEType: "application/pdf",
							URL:      "https://cdn.example/invoice.pdf",
						},
					}
					envelope.ProviderMetadata = json.RawMessage(
						[]byte(`{"wamid":"wamid.2","attachment_count":1}`),
					)
					return envelope
				}(),
			},
			CreatedAt: now,
			UpdatedAt: now.Add(time.Second),
		}

		if err := runtime.dispatchInboundBatch(context.Background(), "brg-1", batch); err != nil {
			t.Fatalf("dispatchInboundBatch() error = %v", err)
		}

		items := ingested()
		if got, want := len(items), 2; got != want {
			t.Fatalf("len(ingested) = %d, want %d", got, want)
		}
		if got, want := items[0].Content.Text, "first"; got != want {
			t.Fatalf("items[0].Content.Text = %q, want %q", got, want)
		}
		if got, want := items[1].Content.Text, "second"; got != want {
			t.Fatalf("items[1].Content.Text = %q, want %q", got, want)
		}
		if got, want := len(items[1].Attachments), 1; got != want {
			t.Fatalf("len(items[1].Attachments) = %d, want %d", got, want)
		}
		if got, want := items[1].Attachments[0].ID, "att-2"; got != want {
			t.Fatalf("items[1].Attachments[0].ID = %q, want %q", got, want)
		}
		if got := string(items[1].ProviderMetadata); !strings.Contains(got, `"attachment_count":1`) {
			t.Fatalf("items[1].ProviderMetadata = %s, want attachment count", got)
		}
	})
}

func TestWhatsAppOutboundMessagePreservation(t *testing.T) {
	// not parallel: this file also contains Setenv-backed runtime tests.
	t.Run("Should preserve leading and trailing whitespace when sending text", func(t *testing.T) {
		api := &fakeWhatsAppAPI{nextMessageID: 300}
		cfg := resolvedInstanceConfig{
			instanceID:    "brg-1",
			phoneNumberID: "phone-1",
		}
		text := "  keep leading whitespace\ntrailing whitespace  "
		req := testDeliveryRequest(
			"brg-1",
			"delivery-whitespace",
			1,
			bridgepkg.DeliveryEventTypeStart,
			false,
			text,
		)

		if _, _, err := executeWhatsAppDelivery(
			context.Background(),
			api,
			cfg,
			req,
			deliveryState{},
		); err != nil {
			t.Fatalf("executeWhatsAppDelivery() error = %v", err)
		}
		if got, want := len(api.requests), 1; got != want {
			t.Fatalf("len(api.requests) = %d, want %d", got, want)
		}
		if got := api.requests[0].Text.Body; got != text {
			t.Fatalf("sent text body = %q, want exact original %q", got, text)
		}
	})

	t.Run("Should preserve Unicode text in rune-bounded chunks and acknowledge the last message", func(t *testing.T) {
		const maxMessageRunes = 4_096

		api := &fakeWhatsAppAPI{nextMessageID: 400}
		cfg := resolvedInstanceConfig{instanceID: "brg-1", phoneNumberID: "phone-1"}
		text := "  " + strings.Repeat("🙂", maxMessageRunes) + "tail\n "
		request := testDeliveryRequest(
			"brg-1",
			"delivery-unicode-chunks",
			1,
			bridgepkg.DeliveryEventTypeStart,
			false,
			text,
		)
		wantChunks := bridgesdk.ChunkMessage(text, maxMessageRunes, nil)

		ack, _, err := executeWhatsAppDelivery(
			context.Background(),
			api,
			cfg,
			request,
			deliveryState{},
		)
		if err != nil {
			t.Fatalf("executeWhatsAppDelivery() error = %v", err)
		}
		if got, want := len(api.requests), len(wantChunks); got != want {
			t.Fatalf("SendTextMessage calls = %d, want %d", got, want)
		}
		for index, wantChunk := range wantChunks {
			got := api.requests[index].Text.Body
			if got != wantChunk {
				t.Fatalf(
					"chunk %d rune length = %d, want exact chunk rune length %d",
					index,
					len([]rune(got)),
					len([]rune(wantChunk)),
				)
			}
			if gotRunes := len([]rune(got)); gotRunes > maxMessageRunes {
				t.Fatalf("chunk %d rune length = %d, want <= %d", index, gotRunes, maxMessageRunes)
			}
		}
		wantRemoteID := fmt.Sprintf("wamid.%d", 400+len(wantChunks)-1)
		if ack.RemoteMessageID != wantRemoteID {
			t.Fatalf("ack.RemoteMessageID = %q, want %q", ack.RemoteMessageID, wantRemoteID)
		}
	})
}

func initializeWhatsAppRuntimeForDeliveryTest(
	t *testing.T,
	runtime *whatsappProvider,
	hostPeer *bridgesdk.Peer,
	now time.Time,
	managed ...subprocess.InitializeBridgeManagedInstance,
) {
	t.Helper()

	if err := hostPeer.Call(
		context.Background(),
		"initialize",
		testInitializeRequest(now, managed...),
		nil,
	); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}
	select {
	case <-runtime.lifecycle.Initialized():
	case <-t.Context().Done():
		t.Fatal("provider initialization did not finish")
	}
	for _, item := range managed {
		if _, ok := runtime.routes.Get(item.Instance.ID); !ok {
			t.Fatalf("route %q missing after initialization", item.Instance.ID)
		}
	}
}

func recordWhatsAppIngests(
	t *testing.T,
	hostPeer *bridgesdk.Peer,
) func() []bridgepkg.InboundMessageEnvelope {
	t.Helper()

	var (
		mu       sync.Mutex
		ingested []bridgepkg.InboundMessageEnvelope
	)
	mustHandle(
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

	return func() []bridgepkg.InboundMessageEnvelope {
		mu.Lock()
		defer mu.Unlock()
		cloned := make([]bridgepkg.InboundMessageEnvelope, len(ingested))
		copy(cloned, ingested)
		return cloned
	}
}

func testWhatsAppInboundEnvelope(
	receivedAt time.Time,
	instanceID string,
	messageID string,
	text string,
) bridgepkg.InboundMessageEnvelope {
	return bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID:  instanceID,
		Scope:             bridgepkg.ScopeWorkspace,
		WorkspaceID:       "ws-whatsapp",
		PeerID:            "15551234567",
		PlatformMessageID: messageID,
		ReceivedAt:        receivedAt,
		Sender: bridgepkg.MessageSender{
			ID:          "15551234567",
			DisplayName: "Alice",
		},
		Content:        bridgepkg.MessageContent{Text: text},
		EventFamily:    bridgepkg.InboundEventFamilyMessage,
		IdempotencyKey: "whatsapp:" + instanceID + ":" + messageID,
	}
}

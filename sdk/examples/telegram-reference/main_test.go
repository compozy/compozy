package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/subprocess"
)

func TestMapTelegramUpdateToInboundEnvelope(t *testing.T) {
	timestamp := time.Date(2026, 4, 11, 4, 30, 0, 0, time.UTC)
	bridgeRuntime := testBridgeRuntime(timestamp, "brg-telegram-reference")

	envelope, err := mapTelegramUpdate(telegramUpdate{
		UpdateID:         9001,
		BridgeInstanceID: bridgeRuntime.Instance.ID,
		Message: &telegramMessage{
			MessageID:       321,
			MessageThreadID: 654,
			Date:            timestamp.Unix(),
			Chat: telegramChat{
				ID:    777,
				Type:  "supergroup",
				Title: "ops",
			},
			From: telegramUser{
				ID:        888,
				Username:  "alice",
				FirstName: "Alice",
				LastName:  "Example",
			},
			Text: "  Need a summary  ",
		},
	}, bridgeRuntime, func() time.Time {
		return timestamp.Add(2 * time.Hour)
	})
	if err != nil {
		t.Fatalf("mapTelegramUpdate() error = %v", err)
	}

	if got, want := envelope.BridgeInstanceID, bridgeRuntime.Instance.ID; got != want {
		t.Fatalf("envelope.BridgeInstanceID = %q, want %q", got, want)
	}
	if got, want := envelope.Scope, bridgeRuntime.Instance.Scope; got != want {
		t.Fatalf("envelope.Scope = %q, want %q", got, want)
	}
	if got, want := envelope.WorkspaceID, bridgeRuntime.Instance.WorkspaceID; got != want {
		t.Fatalf("envelope.WorkspaceID = %q, want %q", got, want)
	}
	if got, want := envelope.PeerID, "777"; got != want {
		t.Fatalf("envelope.PeerID = %q, want %q", got, want)
	}
	if got, want := envelope.ThreadID, "654"; got != want {
		t.Fatalf("envelope.ThreadID = %q, want %q", got, want)
	}
	if got, want := envelope.PlatformMessageID, "321"; got != want {
		t.Fatalf("envelope.PlatformMessageID = %q, want %q", got, want)
	}
	if got, want := envelope.ReceivedAt, timestamp; !got.Equal(want) {
		t.Fatalf("envelope.ReceivedAt = %s, want %s", got.Format(time.RFC3339Nano), want.Format(time.RFC3339Nano))
	}
	if got, want := envelope.Sender.ID, "888"; got != want {
		t.Fatalf("envelope.Sender.ID = %q, want %q", got, want)
	}
	if got, want := envelope.Sender.Username, "alice"; got != want {
		t.Fatalf("envelope.Sender.Username = %q, want %q", got, want)
	}
	if got, want := envelope.Sender.DisplayName, "Alice Example"; got != want {
		t.Fatalf("envelope.Sender.DisplayName = %q, want %q", got, want)
	}
	if got, want := envelope.Content.Text, "Need a summary"; got != want {
		t.Fatalf("envelope.Content.Text = %q, want %q", got, want)
	}
	if got, want := envelope.IdempotencyKey, "telegram:brg-telegram-reference:9001"; got != want {
		t.Fatalf("envelope.IdempotencyKey = %q, want %q", got, want)
	}
}

func TestMapTelegramUpdateToInboundEnvelopeEditAndReplyFamilies(t *testing.T) {
	t.Run("Should map edited message with target text and original timestamp", func(t *testing.T) {
		t.Parallel()

		originalTimestamp := time.Date(2026, 4, 11, 4, 30, 0, 0, time.UTC)
		editTimestamp := originalTimestamp.Add(5 * time.Minute)
		bridgeRuntime := testBridgeRuntime(originalTimestamp, "brg-telegram-reference")
		update := mustDecodeTelegramUpdate(t, `{
			"update_id": 9002,
			"edited_message": {
				"message_id": 321,
				"message_thread_id": 654,
				"date": `+fmt.Sprint(originalTimestamp.Unix())+`,
				"edit_date": `+fmt.Sprint(editTimestamp.Unix())+`,
				"chat": {"id": 777, "type": "supergroup", "title": "ops"},
				"from": {"id": 888, "username": "alice", "first_name": "Alice", "last_name": "Example"},
				"text": "  Need an edited summary  "
			}
		}`)

		envelope, err := mapTelegramUpdate(update, bridgeRuntime, func() time.Time {
			return editTimestamp.Add(time.Hour)
		})
		if err != nil {
			t.Fatalf("mapTelegramUpdate() error = %v", err)
		}
		if err := envelope.Validate(); err != nil {
			t.Fatalf("envelope.Validate() error = %v", err)
		}
		if got, want := envelope.EventFamily, bridgepkg.InboundEventFamilyEdit; got != want {
			t.Fatalf("envelope.EventFamily = %q, want %q", got, want)
		}
		if envelope.Edit == nil {
			t.Fatal("envelope.Edit = nil, want typed edit payload")
		}
		if got, want := envelope.Edit.MessageID, "321"; got != want {
			t.Fatalf("envelope.Edit.MessageID = %q, want %q", got, want)
		}
		if got, want := envelope.Edit.NewText, "Need an edited summary"; got != want {
			t.Fatalf("envelope.Edit.NewText = %q, want %q", got, want)
		}
		if got, want := envelope.Edit.OriginalTimestamp, originalTimestamp; !got.Equal(want) {
			t.Fatalf("envelope.Edit.OriginalTimestamp = %s, want %s", got, want)
		}
		if got, want := envelope.Edit.Operation, bridgepkg.InboundEditOperationUpdated; got != want {
			t.Fatalf("envelope.Edit.Operation = %q, want %q", got, want)
		}
		if got, want := envelope.ReceivedAt, editTimestamp; !got.Equal(want) {
			t.Fatalf("envelope.ReceivedAt = %s, want %s", got, want)
		}
		if envelope.PlatformMessageID != "" || envelope.Content.Text != "" {
			t.Fatalf(
				"edit message fields = platform_message_id %q content %q, want empty",
				envelope.PlatformMessageID,
				envelope.Content.Text,
			)
		}
	})

	t.Run("Should map edited channel post without requiring a user sender", func(t *testing.T) {
		t.Parallel()

		originalTimestamp := time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC)
		editTimestamp := originalTimestamp.Add(time.Minute)
		bridgeRuntime := testBridgeRuntime(originalTimestamp, "brg-telegram-reference")
		update := mustDecodeTelegramUpdate(t, `{
			"update_id": 9003,
			"edited_channel_post": {
				"message_id": 654,
				"date": `+fmt.Sprint(originalTimestamp.Unix())+`,
				"edit_date": `+fmt.Sprint(editTimestamp.Unix())+`,
				"chat": {"id": -100777, "type": "channel", "title": "Release channel"},
				"sender_chat": {"id": -100777, "type": "channel", "title": "Release channel"},
				"text": "Release notes updated"
			}
		}`)

		envelope, err := mapTelegramUpdate(update, bridgeRuntime, func() time.Time {
			return editTimestamp.Add(time.Hour)
		})
		if err != nil {
			t.Fatalf("mapTelegramUpdate() error = %v", err)
		}
		if err := envelope.Validate(); err != nil {
			t.Fatalf("envelope.Validate() error = %v", err)
		}
		if got, want := envelope.Edit.MessageID, "654"; got != want {
			t.Fatalf("envelope.Edit.MessageID = %q, want %q", got, want)
		}
		if got, want := envelope.Sender.ID, "-100777"; got != want {
			t.Fatalf("envelope.Sender.ID = %q, want %q", got, want)
		}
		if got, want := envelope.Sender.DisplayName, "Release channel"; got != want {
			t.Fatalf("envelope.Sender.DisplayName = %q, want %q", got, want)
		}
	})

	t.Run("Should map reply context from the embedded Telegram parent message", func(t *testing.T) {
		t.Parallel()

		timestamp := time.Date(2026, 4, 11, 5, 30, 0, 0, time.UTC)
		bridgeRuntime := testBridgeRuntime(timestamp, "brg-telegram-reference")
		update := mustDecodeTelegramUpdate(t, `{
			"update_id": 9004,
			"message": {
				"message_id": 322,
				"date": `+fmt.Sprint(timestamp.Unix())+`,
				"chat": {"id": 777, "type": "supergroup", "title": "ops"},
				"from": {"id": 999, "username": "bob", "first_name": "Bob"},
				"text": "Follow up on that",
				"reply_to_message": {
					"message_id": 321,
					"date": `+fmt.Sprint(timestamp.Add(-time.Minute).Unix())+`,
					"chat": {"id": 777, "type": "supergroup", "title": "ops"},
					"from": {"id": 888, "username": "alice", "first_name": "Alice", "last_name": "Example"},
					"text": "Need an edited summary"
				}
			}
		}`)

		envelope, err := mapTelegramUpdate(update, bridgeRuntime, nil)
		if err != nil {
			t.Fatalf("mapTelegramUpdate() error = %v", err)
		}
		if err := envelope.Validate(); err != nil {
			t.Fatalf("envelope.Validate() error = %v", err)
		}
		if got, want := envelope.EventFamily, bridgepkg.InboundEventFamilyMessage; got != want {
			t.Fatalf("envelope.EventFamily = %q, want %q", got, want)
		}
		if got, want := envelope.ReplyToText, "Need an edited summary"; got != want {
			t.Fatalf("envelope.ReplyToText = %q, want %q", got, want)
		}
		if got, want := envelope.ReplyToAuthorID, "888"; got != want {
			t.Fatalf("envelope.ReplyToAuthorID = %q, want %q", got, want)
		}
		if got, want := envelope.ReplyToAuthorName, "Alice Example"; got != want {
			t.Fatalf("envelope.ReplyToAuthorName = %q, want %q", got, want)
		}
	})

	t.Run("Should reject ambiguous and unsupported Telegram update payloads", func(t *testing.T) {
		t.Parallel()

		bridgeRuntime := testBridgeRuntime(time.Date(2026, 4, 11, 6, 0, 0, 0, time.UTC), "brg-telegram-reference")
		cases := []struct {
			name string
			raw  string
		}{
			{
				name: "Should reject an update without a supported message payload",
				raw:  `{"update_id":9005}`,
			},
			{
				name: "Should reject an update with both message and edited message payloads",
				raw: `{
					"update_id":9006,
					"message":{"message_id":1,"date":1775887200,"chat":{"id":777},"text":"new"},
					"edited_message":{"message_id":1,"date":1775887200,"edit_date":1775887260,"chat":{"id":777},"text":"edit"}
				}`,
			},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				update := mustDecodeTelegramUpdate(t, testCase.raw)
				if _, err := mapTelegramUpdate(update, bridgeRuntime, nil); err == nil {
					t.Fatalf("mapTelegramUpdate(%#v) error = nil, want unsupported-shape failure", update)
				}
			})
		}
	})
}

func mustDecodeTelegramUpdate(t *testing.T, raw string) telegramUpdate {
	t.Helper()

	var update telegramUpdate
	if err := json.Unmarshal([]byte(raw), &update); err != nil {
		t.Fatalf("json.Unmarshal(telegram update) error = %v", err)
	}
	return update
}

func TestBoundSecretValueReadsOnlyBoundLaunchCredentials(t *testing.T) {
	bridgeRuntime := testBridgeRuntime(time.Date(2026, 4, 11, 4, 45, 0, 0, time.UTC), "brg-telegram-reference")
	bridgeRuntime.BoundSecrets = []subprocess.InitializeBridgeBoundSecret{
		{BindingName: "bot_token", Kind: "token", Value: "  telegram-token  "},
		{BindingName: "webhook_secret", Kind: "token", Value: "webhook-secret"},
	}

	value, ok := boundSecretValue(bridgeRuntime, " bot_token ")
	if !ok {
		t.Fatal("boundSecretValue(bot_token) ok = false, want true")
	}
	if got, want := value, "telegram-token"; got != want {
		t.Fatalf("boundSecretValue(bot_token) = %q, want %q", got, want)
	}

	if got, ok := boundSecretValue(bridgeRuntime, "runtime/vault/read"); ok || got != "" {
		t.Fatalf("boundSecretValue(runtime/vault/read) = (%q, %t), want empty/false", got, ok)
	}
	if got, ok := boundSecretValue(bridgeRuntime, "missing"); ok || got != "" {
		t.Fatalf("boundSecretValue(missing) = (%q, %t), want empty/false", got, ok)
	}
}

func TestResolveManagedInstanceRequiresExplicitSelectionForMultiplexedProvider(t *testing.T) {
	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 11, 6, 55, 0, 0, time.UTC)
	managed := []subprocess.InitializeBridgeManagedInstance{
		testBridgeRuntime(now, "brg-1"),
		testBridgeRuntime(now, "brg-2"),
	}
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesList),
		func(context.Context, json.RawMessage) (any, error) {
			return []bridgepkg.BridgeInstance{managed[0].Instance, managed[1].Instance}, nil
		},
	)
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
		func(_ context.Context, raw json.RawMessage) (any, error) {
			var params bridgepkg.BridgeInstanceTargetParams
			if err := json.Unmarshal(raw, &params); err != nil {
				return nil, err
			}
			switch params.BridgeInstanceID {
			case "brg-1":
				return managed[0].Instance, nil
			case "brg-2":
				return managed[1].Instance, nil
			default:
				return nil, errors.New("unexpected instance")
			}
		},
	)
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		func(_ context.Context, raw json.RawMessage) (any, error) {
			var params bridgepkg.BridgesInstancesReportStateParams
			if err := json.Unmarshal(raw, &params); err != nil {
				return nil, err
			}
			switch params.BridgeInstanceID {
			case "brg-1":
				instance := managed[0].Instance
				instance.Status = params.Status
				return instance, nil
			case "brg-2":
				instance := managed[1].Instance
				instance.Status = params.Status
				return instance, nil
			default:
				return nil, errors.New("unexpected instance")
			}
		},
	)

	if err := hostPeer.Call(
		context.Background(),
		"initialize",
		testInitializeRequest(now, managed...),
		nil,
	); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	session := runtime.sdk.Session()
	if _, err := resolveManagedInstance(session, ""); err == nil {
		t.Fatal("resolveManagedInstance(empty) error = nil, want explicit selection failure")
	}

	resolved, err := resolveManagedInstance(session, "brg-2")
	if err != nil {
		t.Fatalf("resolveManagedInstance(brg-2) error = %v", err)
	}
	if got, want := resolved.Instance.ID, "brg-2"; got != want {
		t.Fatalf("resolved.Instance.ID = %q, want %q", got, want)
	}
}

func TestAckDeliveryPreservesOrderedRemoteAndReplacementIDsPerInstance(t *testing.T) {
	runtime, err := newTelegramReferenceRuntime(io.Discard)
	if err != nil {
		t.Fatalf("newTelegramReferenceRuntime() error = %v", err)
	}

	startAck, err := runtime.ackDelivery(
		testDeliveryRequest("brg-1", "delivery-1", 1, bridgepkg.DeliveryEventTypeStart, false),
	)
	if err != nil {
		t.Fatalf("ackDelivery(start) error = %v", err)
	}
	if got, want := startAck.RemoteMessageID, "telegram:brg-1:delivery-1:1"; got != want {
		t.Fatalf("start ack remote_message_id = %q, want %q", got, want)
	}

	deltaAck, err := runtime.ackDelivery(
		testDeliveryRequest("brg-1", "delivery-1", 2, bridgepkg.DeliveryEventTypeDelta, false),
	)
	if err != nil {
		t.Fatalf("ackDelivery(delta) error = %v", err)
	}
	if got, want := deltaAck.ReplaceRemoteMessageID, startAck.RemoteMessageID; got != want {
		t.Fatalf("delta ack replace_remote_message_id = %q, want %q", got, want)
	}

	otherAck, err := runtime.ackDelivery(
		testDeliveryRequest("brg-2", "delivery-1", 1, bridgepkg.DeliveryEventTypeStart, false),
	)
	if err != nil {
		t.Fatalf("ackDelivery(other instance) error = %v", err)
	}
	if got, want := otherAck.RemoteMessageID, "telegram:brg-2:delivery-1:1"; got != want {
		t.Fatalf("other ack remote_message_id = %q, want %q", got, want)
	}
}

func TestAckDeliveryRejectsOutOfOrderSequence(t *testing.T) {
	runtime, err := newTelegramReferenceRuntime(io.Discard)
	if err != nil {
		t.Fatalf("newTelegramReferenceRuntime() error = %v", err)
	}

	if _, err := runtime.ackDelivery(
		testDeliveryRequest("brg-1", "delivery-2", 1, bridgepkg.DeliveryEventTypeStart, false),
	); err != nil {
		t.Fatalf("ackDelivery(start) error = %v", err)
	}
	if _, err := runtime.ackDelivery(
		testDeliveryRequest("brg-1", "delivery-2", 1, bridgepkg.DeliveryEventTypeDelta, false),
	); err == nil {
		t.Fatal("ackDelivery(out-of-order) error = nil, want failure")
	}
}

func TestRunRejectsUnsupportedCommand(t *testing.T) {
	err := run([]string{"bogus"}, strings.NewReader(""), io.Discard, io.Discard)
	if err == nil {
		t.Fatal("run(unsupported) error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "unsupported command") {
		t.Fatalf("run(unsupported) error = %v, want unsupported command", err)
	}
}

func TestRunServeReturnsOnEOFAndWritesStartMarker(t *testing.T) {
	env := setAdapterTestEnv(t)

	if err := run(nil, strings.NewReader(""), io.Discard, io.Discard); err != nil {
		t.Fatalf("run(serve) error = %v", err)
	}

	lines := waitForNonEmptyLines(t, env.startsPath)
	if len(lines) == 0 || !strings.Contains(lines[0], "pid=") {
		t.Fatalf("start marker lines = %#v, want pid entry", lines)
	}
}

func TestRuntimeInitializeWritesOwnershipAndPerInstanceStateMarkers(t *testing.T) {
	env := setAdapterTestEnv(t)
	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 11, 7, 0, 0, 0, time.UTC)
	managed := []subprocess.InitializeBridgeManagedInstance{
		testBridgeRuntime(now, "brg-1"),
		testBridgeRuntime(now, "brg-2"),
	}
	managed[0].BoundSecrets = []subprocess.InitializeBridgeBoundSecret{
		{BindingName: "bot_token", Kind: "token", Value: "telegram-bot-token"},
	}

	listedIDs := make([]string, 0)
	gotIDs := make([]string, 0)
	reportedStatuses := make([]bridgepkg.BridgesInstancesReportStateParams, 0)
	var mu sync.Mutex

	instanceByID := map[string]bridgepkg.BridgeInstance{
		"brg-1": managed[0].Instance,
		"brg-2": managed[1].Instance,
	}
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesList),
		func(context.Context, json.RawMessage) (any, error) {
			mu.Lock()
			listedIDs = append(listedIDs, "list")
			mu.Unlock()
			return []bridgepkg.BridgeInstance{instanceByID["brg-1"], instanceByID["brg-2"]}, nil
		},
	)
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
		func(_ context.Context, params json.RawMessage) (any, error) {
			var payload bridgepkg.BridgeInstanceTargetParams
			if err := json.Unmarshal(params, &payload); err != nil {
				return nil, err
			}
			mu.Lock()
			gotIDs = append(gotIDs, payload.BridgeInstanceID)
			mu.Unlock()
			return instanceByID[payload.BridgeInstanceID], nil
		},
	)
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		func(_ context.Context, params json.RawMessage) (any, error) {
			var payload bridgepkg.BridgesInstancesReportStateParams
			if err := json.Unmarshal(params, &payload); err != nil {
				return nil, err
			}
			instance := instanceByID[payload.BridgeInstanceID]
			instance.Status = payload.Status
			mu.Lock()
			reportedStatuses = append(reportedStatuses, payload)
			mu.Unlock()
			return instance, nil
		},
	)

	if err := hostPeer.Call(
		context.Background(),
		"initialize",
		testInitializeRequest(now, managed...),
		nil,
	); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	handshake := waitForJSONFile[initializeMarker](t, env.handshakePath)
	if got, want := len(handshake.Request.Runtime.Bridge.ManagedInstances), 2; got != want {
		t.Fatalf("len(handshake managed instances) = %d, want %d", got, want)
	}

	ownership := waitForJSONFile[ownershipMarker](t, env.ownershipPath)
	if got, want := len(ownership.Listed), 2; got != want {
		t.Fatalf("len(ownership.Listed) = %d, want %d", got, want)
	}
	if got, want := len(ownership.Fetched), 2; got != want {
		t.Fatalf("len(ownership.Fetched) = %d, want %d", got, want)
	}

	states := waitForJSONLinesFile[stateMarker](t, env.statePath, func(items []stateMarker) bool {
		return len(items) >= 2
	})
	if got, want := states[0].BridgeInstanceID, "brg-1"; got != want {
		t.Fatalf("states[0].BridgeInstanceID = %q, want %q", got, want)
	}
	if got, want := states[0].Status.Normalize(), bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("states[0].Status = %q, want %q", got, want)
	}
	if got, want := states[1].Status.Normalize(), bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("states[1].Status = %q, want %q", got, want)
	}

	mu.Lock()
	defer mu.Unlock()
	if got, want := len(listedIDs), 1; got != want {
		t.Fatalf("len(listedIDs) = %d, want %d", got, want)
	}
	if got, want := strings.Join(gotIDs, ","), "brg-1,brg-2"; got != want {
		t.Fatalf("gotIDs = %q, want %q", got, want)
	}
	if got, want := len(reportedStatuses), 2; got != want {
		t.Fatalf("len(reportedStatuses) = %d, want %d", got, want)
	}

	_ = runtime
}

func TestRuntimePollsInboundUpdatesAndRetriesNotInitialized(t *testing.T) {
	env := setAdapterTestEnv(t)
	_, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 11, 7, 5, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-telegram-reference")
	managed.BoundSecrets = []subprocess.InitializeBridgeBoundSecret{
		{BindingName: "bot_token", Kind: "token", Value: "telegram-bot-token"},
	}

	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesList),
		func(context.Context, json.RawMessage) (any, error) {
			return []bridgepkg.BridgeInstance{managed.Instance}, nil
		},
	)
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
		func(context.Context, json.RawMessage) (any, error) {
			return managed.Instance, nil
		},
	)
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		func(_ context.Context, params json.RawMessage) (any, error) {
			var payload bridgepkg.BridgesInstancesReportStateParams
			if err := json.Unmarshal(params, &payload); err != nil {
				return nil, err
			}
			instance := managed.Instance
			instance.Status = payload.Status
			return instance, nil
		},
	)

	var ingestCalls atomic.Int64
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
		func(_ context.Context, params json.RawMessage) (any, error) {
			if ingestCalls.Add(1) == 1 {
				return nil, subprocess.NewRPCError(rpcCodeNotInitialized, "Not initialized", nil)
			}

			var envelope bridgepkg.InboundMessageEnvelope
			if err := json.Unmarshal(params, &envelope); err != nil {
				return nil, err
			}
			return bridgepkg.BridgesMessagesIngestResult{
				SessionID:    "sess-1",
				RouteCreated: true,
				RoutingKey: bridgepkg.RoutingKey{
					Scope:            envelope.Scope,
					WorkspaceID:      envelope.WorkspaceID,
					BridgeInstanceID: envelope.BridgeInstanceID,
					PeerID:           envelope.PeerID,
					ThreadID:         envelope.ThreadID,
				},
			}, nil
		},
	)

	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	update := telegramUpdate{
		BridgeInstanceID: managed.Instance.ID,
		UpdateID:         9002,
		Message: &telegramMessage{
			MessageID:       654,
			MessageThreadID: 987,
			Date:            now.Unix(),
			Chat:            telegramChat{ID: 777},
			From:            telegramUser{ID: 888, Username: "alice"},
			Caption:         "caption fallback",
		},
	}
	if err := appendJSONLine(env.updatesPath, update); err != nil {
		t.Fatalf("appendJSONLine(update) error = %v", err)
	}

	ingests := waitForJSONLinesFile[ingestMarker](t, env.ingestPath, func(items []ingestMarker) bool {
		return len(items) > 0 && strings.TrimSpace(items[len(items)-1].Result.SessionID) != ""
	})
	if got, want := ingests[len(ingests)-1].Envelope.Content.Text, "caption fallback"; got != want {
		t.Fatalf("ingest envelope text = %q, want %q", got, want)
	}
	if got := ingestCalls.Load(); got < 2 {
		t.Fatalf("ingest host call attempts = %d, want retry after not initialized", got)
	}
}

func TestRuntimeDeliveryWritesAckAndManagedInstanceErrors(t *testing.T) {
	env := setAdapterTestEnv(t)
	_, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 11, 7, 10, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-telegram-reference")
	managed.BoundSecrets = []subprocess.InitializeBridgeBoundSecret{
		{BindingName: "bot_token", Kind: "token", Value: "telegram-bot-token"},
	}

	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesList),
		func(context.Context, json.RawMessage) (any, error) {
			return []bridgepkg.BridgeInstance{managed.Instance}, nil
		},
	)
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
		func(context.Context, json.RawMessage) (any, error) {
			return managed.Instance, nil
		},
	)
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		func(_ context.Context, params json.RawMessage) (any, error) {
			var payload bridgepkg.BridgesInstancesReportStateParams
			if err := json.Unmarshal(params, &payload); err != nil {
				return nil, err
			}
			instance := managed.Instance
			instance.Status = payload.Status
			return instance, nil
		},
	)

	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	var ack bridgepkg.DeliveryAck
	if err := hostPeer.Call(
		context.Background(),
		"bridges/deliver",
		testDeliveryRequest(managed.Instance.ID, "delivery-3", 1, bridgepkg.DeliveryEventTypeStart, false),
		&ack,
	); err != nil {
		t.Fatalf("hostPeer.Call(bridges/deliver) error = %v", err)
	}
	if got, want := ack.RemoteMessageID, "telegram:brg-telegram-reference:delivery-3:1"; got != want {
		t.Fatalf("ack.RemoteMessageID = %q, want %q", got, want)
	}

	if err := hostPeer.Call(
		context.Background(),
		"bridges/deliver",
		testDeliveryRequest("brg-unowned", "delivery-4", 1, bridgepkg.DeliveryEventTypeStart, false),
		nil,
	); err == nil {
		t.Fatal("hostPeer.Call(bridges/deliver unowned) error = nil, want failure")
	}

	records := waitForJSONLinesFile[deliveryMarker](t, env.deliveryPath, func(items []deliveryMarker) bool {
		return len(items) >= 2
	})
	if records[0].Ack == nil {
		t.Fatal("first delivery marker ack = nil, want ack")
	}
	if records[1].Ack != nil || strings.TrimSpace(records[1].Error) == "" {
		t.Fatalf("second delivery marker = %#v, want recorded error without ack", records[1])
	}
}

func TestRuntimeInitializeWritesOwnershipErrorAndStillReportsState(t *testing.T) {
	env := setAdapterTestEnv(t)
	_, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 11, 7, 12, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-telegram-reference")
	managed.BoundSecrets = []subprocess.InitializeBridgeBoundSecret{
		{BindingName: "bot_token", Kind: "token", Value: "telegram-bot-token"},
	}

	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesList),
		func(context.Context, json.RawMessage) (any, error) {
			return []bridgepkg.BridgeInstance{managed.Instance}, nil
		},
	)
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
		func(context.Context, json.RawMessage) (any, error) {
			return nil, subprocess.NewRPCError(-32601, "Method not found", nil)
		},
	)
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		func(_ context.Context, params json.RawMessage) (any, error) {
			var payload bridgepkg.BridgesInstancesReportStateParams
			if err := json.Unmarshal(params, &payload); err != nil {
				return nil, err
			}
			instance := managed.Instance
			instance.Status = payload.Status
			return instance, nil
		},
	)

	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	ownership := waitForJSONFile[ownershipMarker](t, env.ownershipPath)
	if got := strings.TrimSpace(ownership.Error); got == "" {
		t.Fatal("ownership.Error = empty, want recorded get failure")
	}

	states := waitForJSONLinesFile[stateMarker](t, env.statePath, func(items []stateMarker) bool {
		return len(items) > 0
	})
	if got, want := states[len(states)-1].Status.Normalize(), bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("last state status = %q, want %q", got, want)
	}
}

func TestRetryHostCallReturnsContextError(t *testing.T) {
	runtime, err := newTelegramReferenceRuntime(io.Discard)
	if err != nil {
		t.Fatalf("newTelegramReferenceRuntime() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runtime.retryHostCall(ctx, func(context.Context) error {
		return subprocess.NewRPCError(rpcCodeNotInitialized, "Not initialized", nil)
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("retryHostCall() error = %v, want context.Canceled", err)
	}
}

func TestHealthCheckReflectsLastErrorAndHandleShutdownWritesMarker(t *testing.T) {
	env := setAdapterTestEnv(t)
	runtime, err := newTelegramReferenceRuntime(io.Discard)
	if err != nil {
		t.Fatalf("newTelegramReferenceRuntime() error = %v", err)
	}

	runtime.setLastError(errors.New("boom"))
	if err := runtime.healthCheck(); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("healthCheck() error = %v, want boom", err)
	}

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-runtime.stopCh
	}()
	if err := runtime.handleShutdown(
		context.Background(),
		nil,
		subprocess.ShutdownRequest{DeadlineMS: 50},
	); err != nil {
		t.Fatalf("handleShutdown() error = %v", err)
	}
	lines := waitForNonEmptyLines(t, env.shutdownPath)
	if len(lines) == 0 || !strings.Contains(lines[0], "pid=") {
		t.Fatalf("shutdown marker lines = %#v, want pid entry", lines)
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not close stopCh before timeout")
	}
}

func TestUtilityHelpers(t *testing.T) {
	if _, err := mapTelegramUpdate(telegramUpdate{}, testBridgeRuntime(time.Now().UTC(), "brg-1"), nil); err == nil {
		t.Fatal("mapTelegramUpdate(nil message) error = nil, want failure")
	}
	if got, want := deliveryStateKey(" brg-1 ", " dlv-1 "), "brg-1:dlv-1"; got != want {
		t.Fatalf("deliveryStateKey() = %q, want %q", got, want)
	}
	if got, want := optionalTelegramID(0), ""; got != want {
		t.Fatalf("optionalTelegramID(0) = %q, want empty", got)
	}
	if !isNotInitializedRPCError(subprocess.NewRPCError(rpcCodeNotInitialized, "Not initialized", nil)) {
		t.Fatal("isNotInitializedRPCError() = false, want true")
	}
	if isNotInitializedRPCError(errors.New("boom")) {
		t.Fatal("isNotInitializedRPCError(non-rpc) = true, want false")
	}

	sideEffectRoot := tempSideEffectDir(t, "agh-telegram-reference-utils-")
	target := filepath.Join(sideEffectRoot, "crash-once.json")
	if !shouldCrashOnce(target) {
		t.Fatal("shouldCrashOnce(missing file) = false, want true")
	}
	if err := os.WriteFile(target, []byte("ok"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", target, err)
	}
	if shouldCrashOnce(target) {
		t.Fatal("shouldCrashOnce(existing file) = true, want false")
	}

	markerPath := filepath.Join(sideEffectRoot, "markers", "lines.log")
	if err := appendMarkerLine(markerPath, " hello "); err != nil {
		t.Fatalf("appendMarkerLine() error = %v", err)
	}
	if got, want := waitForNonEmptyLines(t, markerPath)[0], "hello"; got != want {
		t.Fatalf("marker line = %q, want %q", got, want)
	}

	jsonlPath := filepath.Join(sideEffectRoot, "markers", "data.jsonl")
	if err := appendJSONLine(jsonlPath, map[string]string{"hello": "world"}); err != nil {
		t.Fatalf("appendJSONLine() error = %v", err)
	}
	if got := waitForNonEmptyLines(t, jsonlPath); len(got) != 1 || !strings.Contains(got[0], `"hello":"world"`) {
		t.Fatalf("jsonl lines = %#v, want encoded payload", got)
	}

	jsonPath := filepath.Join(sideEffectRoot, "markers", "state.json")
	if err := writeJSONFile(jsonPath, map[string]string{"status": "ready"}); err != nil {
		t.Fatalf("writeJSONFile() error = %v", err)
	}
	if got := waitForNonEmptyLines(t, jsonPath); len(got) != 1 || !strings.Contains(got[0], `"status":"ready"`) {
		t.Fatalf("json file lines = %#v, want encoded payload", got)
	}

	lines := nonEmptyLines("\n one \n\n two \n")
	if got, want := strings.Join(lines, ","), "one,two"; got != want {
		t.Fatalf("nonEmptyLines() = %q, want %q", got, want)
	}
}

func newRuntimePeerPair(t *testing.T) (*telegramReferenceRuntime, *bridgesdk.Peer, func()) {
	t.Helper()

	hostConn, runtimeConn := net.Pipe()
	runtime, err := newTelegramReferenceRuntime(io.Discard)
	if err != nil {
		t.Fatalf("newTelegramReferenceRuntime() error = %v", err)
	}

	hostPeer := bridgesdk.NewPeer(hostConn, hostConn)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 2)
	go func() { errCh <- runtime.serve(ctx, runtimeConn, runtimeConn) }()
	go func() { errCh <- hostPeer.Serve(ctx) }()

	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			cancel()
			runtime.stop()
			if err := hostConn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("hostConn.Close() error = %v", err)
			}
			if err := runtimeConn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("runtimeConn.Close() error = %v", err)
			}
			runtime.wg.Wait()
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
		})
	}

	return runtime, hostPeer, cleanup
}

func mustHandle(t *testing.T, peer *bridgesdk.Peer, method string, handler bridgesdk.RPCHandler) {
	t.Helper()
	if err := peer.Handle(method, handler); err != nil {
		t.Fatalf("peer.Handle(%q) error = %v", method, err)
	}
}

func testBridgeRuntime(now time.Time, instanceID string) subprocess.InitializeBridgeManagedInstance {
	return subprocess.InitializeBridgeManagedInstance{
		Instance: bridgepkg.BridgeInstance{
			ID:            instanceID,
			Scope:         bridgepkg.ScopeWorkspace,
			WorkspaceID:   "ws-telegram",
			Platform:      "telegram",
			ExtensionName: "telegram-reference",
			DisplayName:   "Telegram Reference",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true, IncludeThread: true},
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}
}

func testInitializeRequest(
	_ time.Time,
	managed ...subprocess.InitializeBridgeManagedInstance,
) subprocess.InitializeRequest {
	return subprocess.InitializeRequest{
		ProtocolVersion:          "1",
		SupportedProtocolVersion: []string{"1"},
		AGHVersion:               "0.5.0",
		SessionNonce:             "nonce-test",
		Extension: subprocess.InitializeExtension{
			Name:       "telegram-reference",
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
				Provider:         "telegram-reference",
				Platform:         "telegram",
				ManagedInstances: managed,
			},
		},
	}
}

func testDeliveryRequest(
	instanceID string,
	deliveryID string,
	seq int64,
	eventType bridgepkg.DeliveryEventType,
	final bool,
) bridgepkg.DeliveryRequest {
	return bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       deliveryID,
			BridgeInstanceID: instanceID,
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-telegram",
				BridgeInstanceID: instanceID,
				PeerID:           "peer-1",
				ThreadID:         "thread-1",
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: instanceID,
				PeerID:           "peer-1",
				ThreadID:         "thread-1",
				Mode:             bridgepkg.DeliveryModeReply,
			},
			Seq:       seq,
			EventType: eventType,
			Content:   bridgepkg.MessageContent{Text: "hello"},
			Final:     final,
		},
	}
}

func setAdapterTestEnv(t *testing.T) adapterEnv {
	// not parallel: mutates process environment for adapter marker paths.
	t.Helper()

	root := tempSideEffectDir(t, "agh-telegram-reference-markers-")
	env := adapterEnv{
		handshakePath: filepath.Join(root, "handshake.json"),
		ownershipPath: filepath.Join(root, "ownership.json"),
		statePath:     filepath.Join(root, "state.jsonl"),
		deliveryPath:  filepath.Join(root, "delivery.jsonl"),
		ingestPath:    filepath.Join(root, "ingest.jsonl"),
		updatesPath:   filepath.Join(root, "updates.jsonl"),
		startsPath:    filepath.Join(root, "starts.log"),
		shutdownPath:  filepath.Join(root, "shutdown.log"),
		crashOncePath: filepath.Join(root, "crash-once.json"),
	}

	t.Setenv(adapterHandshakeEnv, env.handshakePath)
	t.Setenv(adapterOwnershipEnv, env.ownershipPath)
	t.Setenv(adapterStateEnv, env.statePath)
	t.Setenv(adapterDeliveryEnv, env.deliveryPath)
	t.Setenv(adapterIngestEnv, env.ingestPath)
	t.Setenv(adapterUpdatesEnv, env.updatesPath)
	t.Setenv(adapterStartsEnv, env.startsPath)
	t.Setenv(adapterShutdownEnv, env.shutdownPath)
	t.Setenv(adapterCrashOnceEnv, "")

	return env
}

func tempSideEffectDir(t *testing.T, pattern string) string {
	t.Helper()

	root, err := os.MkdirTemp("", pattern)
	if err != nil {
		t.Fatalf("os.MkdirTemp(%q) error = %v", pattern, err)
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("telegram-reference side-effect directory retained at %s", root)
			return
		}
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("os.RemoveAll(%q) error = %v", root, err)
		}
	})
	return root
}

func waitForNonEmptyLines(t *testing.T, path string) []string {
	t.Helper()

	var lines []string
	waitForCondition(t, func() bool {
		payload, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		lines = nonEmptyLines(string(payload))
		return len(lines) > 0
	})
	return lines
}

func waitForJSONFile[T any](t *testing.T, path string) T {
	t.Helper()

	var item T
	waitForCondition(t, func() bool {
		payload, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		return json.Unmarshal(payload, &item) == nil
	})
	return item
}

func waitForJSONLinesFile[T any](t *testing.T, path string, predicate func([]T) bool) []T {
	t.Helper()

	var items []T
	waitForCondition(t, func() bool {
		payload, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		lines := nonEmptyLines(string(payload))
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

func waitForCondition(t *testing.T, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition did not succeed before timeout")
}

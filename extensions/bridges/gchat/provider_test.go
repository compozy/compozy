package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/subprocess"
)

func TestGChatProgressRendering(t *testing.T) {
	t.Parallel()

	t.Run("Should acknowledge default-off progress without calling the Google Chat API", func(t *testing.T) {
		t.Parallel()

		provider, err := newGChatProvider(io.Discard)
		if err != nil {
			t.Fatalf("newGChatProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testBridgeRuntime(t, time.Unix(8_300, 0).UTC(), "brg-progress-off")
		setGChatProviderRoute(provider, managed.Instance.ID, &resolvedInstanceConfig{
			managed:    managed,
			instanceID: managed.Instance.ID,
		})
		apiCalls := 0
		provider.apiFactory = func(*resolvedInstanceConfig) gchatAPI {
			apiCalls++
			return &fakeGChatAPI{}
		}

		request := testGChatProgressRequest(
			managed.Instance.ID,
			"delivery-progress-off",
			1,
			"call-off",
			"agh__terminal",
			"Inspecting",
		)
		ack, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			request,
		)
		if err != nil {
			t.Fatalf("handleBridgesProgress() error = %v", err)
		}
		if ack.DeliveryID != request.Event.DeliveryID || ack.Seq != request.Event.Seq {
			t.Fatalf("progress ack = %#v, want delivery id %q seq %d", ack, request.Event.DeliveryID, request.Event.Seq)
		}
		if ack.RemoteMessageID != "" || ack.ReplaceRemoteMessageID != "" {
			t.Fatalf("progress ack remote ids = (%q, %q), want empty", ack.RemoteMessageID, ack.ReplaceRemoteMessageID)
		}
		if apiCalls != 0 {
			t.Fatalf("Google Chat API factory calls = %d, want zero", apiCalls)
		}

		provider.storeDeliveryState(
			managed.Instance.ID,
			request.Event.DeliveryID,
			request.Event,
			deliveryState{RemoteMessageID: "spaces/AAA/messages/stale"},
		)
		final := request
		final.Event.Seq = 2
		final.Event.EventType = bridgepkg.DeliveryEventTypeFinal
		final.Event.Progress = nil
		final.Event.Final = true
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			final,
		); err != nil {
			t.Fatalf("handleBridgesProgress(default-off lifecycle final) error = %v", err)
		}
		_, deliveryExists := provider.deliveries.Load(deliveryStateKey(managed.Instance.ID, request.Event.DeliveryID))
		if deliveryExists {
			t.Fatal("default-off lifecycle-only delivery state still exists, want terminal removal")
		}
	})

	t.Run("Should retain failed progress for retry and report unhealthy", func(t *testing.T) {
		t.Parallel()

		provider, err := newGChatProvider(io.Discard)
		if err != nil {
			t.Fatalf("newGChatProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testBridgeRuntime(t, time.Unix(8_325, 0).UTC(), "brg-progress-failure")
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"all","grouping":"separate","typing":false,"reactions":false}}`,
		)
		setGChatProviderRoute(provider, managed.Instance.ID, &resolvedInstanceConfig{
			managed:    managed,
			instanceID: managed.Instance.ID,
		})
		provider.apiFactory = func(*resolvedInstanceConfig) gchatAPI {
			return authFailingGChatAPI{}
		}

		request := testGChatProgressRequest(
			managed.Instance.ID,
			"delivery-progress-failure",
			1,
			"call-failure",
			"agh__terminal",
			"Inspecting",
		)
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			request,
		); err == nil {
			t.Fatal("handleBridgesProgress() error = nil, want failed create error")
		}
		if provider.deliveryState(managed.Instance.ID, request.Event.DeliveryID).Progress == nil {
			t.Fatal("failed progress dispatcher = nil, want retained dispatcher for retry")
		}
		if err := provider.healthCheck(); err == nil {
			t.Fatal("healthCheck() error = nil, want reported progress failure")
		}
	})

	t.Run("Should detach pending progress after opt-out even when provider config is invalid", func(t *testing.T) {
		t.Parallel()

		provider, err := newGChatProvider(io.Discard)
		if err != nil {
			t.Fatalf("newGChatProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testBridgeRuntime(t, time.Unix(8_350, 0).UTC(), "brg-progress-disabled")
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"all","grouping":"accumulate","typing":false,"reactions":false}}`,
		)
		setGChatProviderRoute(provider, managed.Instance.ID, &resolvedInstanceConfig{
			managed:    managed,
			instanceID: managed.Instance.ID,
		})
		api := &fakeGChatAPI{}
		provider.apiFactory = func(*resolvedInstanceConfig) gchatAPI { return api }

		first := testGChatProgressRequest(
			managed.Instance.ID,
			"delivery-progress-disabled",
			1,
			"call-1",
			"agh__terminal",
			"Inspecting",
		)
		second := testGChatProgressRequest(
			managed.Instance.ID,
			"delivery-progress-disabled",
			2,
			"call-2",
			"agh__task_list",
			"Reading tasks",
		)
		for _, request := range []bridgepkg.DeliveryRequest{first, second} {
			if _, err := provider.handleBridgesProgress(
				context.Background(),
				&bridgesdk.Session{},
				request,
			); err != nil {
				t.Fatalf("handleBridgesProgress(seq=%d) error = %v", request.Event.Seq, err)
			}
		}
		if provider.deliveryState(managed.Instance.ID, first.Event.DeliveryID).Progress == nil {
			t.Fatal("progress dispatcher before disabling = nil, want active dispatcher")
		}

		provider.routes.Update(managed.Instance.ID, func(cfg resolvedInstanceConfig) resolvedInstanceConfig {
			cfg.managed.Instance.DeliveryDefaults = json.RawMessage(
				`{"progress":{"tool_progress":"off","grouping":"accumulate","typing":false,"reactions":false}}`,
			)
			cfg.configError = errors.New("gchat: provider config became invalid")
			return cfg
		})
		disabled := second
		disabled.Event.Seq = 3
		disabled.Event.Progress.ToolCallID = "call-3"
		disabled.Event.Progress.Index = 3
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			disabled,
		); err != nil {
			t.Fatalf("handleBridgesProgress(disabled) error = %v", err)
		}

		if provider.deliveryState(managed.Instance.ID, first.Event.DeliveryID).Progress != nil {
			t.Fatal("progress dispatcher after disabling != nil, want detached dispatcher")
		}
		api.mu.Lock()
		updates := len(api.updates)
		api.mu.Unlock()
		if updates != 0 {
			t.Fatalf("Google Chat progress updates after disabling = %d, want zero", updates)
		}
	})

	t.Run("Should flush an opted-in plain-text progress message before the final answer", func(t *testing.T) {
		t.Parallel()

		provider, err := newGChatProvider(io.Discard)
		if err != nil {
			t.Fatalf("newGChatProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testBridgeRuntime(t, time.Unix(8_400, 0).UTC(), "brg-progress-on")
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"all","grouping":"accumulate","typing":false,"reactions":false}}`,
		)
		setGChatProviderRoute(provider, managed.Instance.ID, &resolvedInstanceConfig{
			managed:    managed,
			instanceID: managed.Instance.ID,
		})
		api := &fakeGChatAPI{}
		provider.apiFactory = func(*resolvedInstanceConfig) gchatAPI { return api }

		first := testGChatProgressRequest(
			managed.Instance.ID,
			"delivery-progress-on",
			1,
			"call-1",
			"agh__terminal",
			"Inspecting",
		)
		second := testGChatProgressRequest(
			managed.Instance.ID,
			"delivery-progress-on",
			2,
			"call-2",
			"agh__task_list",
			"Reading tasks",
		)
		for _, request := range []bridgepkg.DeliveryRequest{first, second} {
			if _, err := provider.handleBridgesProgress(
				context.Background(),
				&bridgesdk.Session{},
				request,
			); err != nil {
				t.Fatalf("handleBridgesProgress(seq=%d) error = %v", request.Event.Seq, err)
			}
		}

		final := first
		final.Event.Seq = 3
		final.Event.EventType = bridgepkg.DeliveryEventTypeFinal
		final.Event.Progress = nil
		final.Event.Content = bridgepkg.MessageContent{Text: "Final answer"}
		final.Event.Final = true
		if _, err := provider.handleBridgesDeliver(
			context.Background(),
			&bridgesdk.Session{},
			final,
		); err != nil {
			t.Fatalf("handleBridgesDeliver(final) error = %v", err)
		}
		_, deliveryExists := provider.deliveries.Load(deliveryStateKey(managed.Instance.ID, first.Event.DeliveryID))
		if deliveryExists {
			t.Fatal("terminal delivery state still exists, want terminal removal")
		}

		api.mu.Lock()
		messages := append([]gchatCreateMessageRequest(nil), api.messages...)
		updates := append([]gchatUpdateMessageRequest(nil), api.updates...)
		api.mu.Unlock()
		if got, want := len(messages), 2; got != want {
			t.Fatalf("Google Chat create calls = %d, want %d", got, want)
		}
		if got, want := messages[0].Text, "Inspecting"; got != want {
			t.Fatalf("progress message text = %q, want %q", got, want)
		}
		if got, want := messages[0].SpaceName, "spaces/AAA"; got != want {
			t.Fatalf("progress message space = %q, want %q", got, want)
		}
		if got, want := messages[0].ThreadName, "spaces/AAA/threads/thread-1"; got != want {
			t.Fatalf("progress message thread = %q, want %q", got, want)
		}
		if got, want := len(updates), 1; got != want {
			t.Fatalf("Google Chat progress patch calls = %d, want %d", got, want)
		}
		if got, want := updates[0].MessageName, "spaces/AAA/messages/msg-1"; got != want {
			t.Fatalf("progress patch message name = %q, want %q", got, want)
		}
		if got, want := updates[0].Text, "Inspecting\nReading tasks"; got != want {
			t.Fatalf("progress patch text = %q, want %q", got, want)
		}
		if got, want := messages[1].Text, "Final answer"; got != want {
			t.Fatalf("final message text = %q, want %q", got, want)
		}
	})

	t.Run("Should deliver final text once after a committed progress flush", func(t *testing.T) {
		t.Parallel()

		provider, err := newGChatProvider(io.Discard)
		if err != nil {
			t.Fatalf("newGChatProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testBridgeRuntime(t, time.Unix(8_425, 0).UTC(), "brg-progress-committed-flush")
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"all","grouping":"accumulate","typing":false,"reactions":false}}`,
		)
		setGChatProviderRoute(provider, managed.Instance.ID, &resolvedInstanceConfig{
			managed:    managed,
			instanceID: managed.Instance.ID,
		})
		api := &fakeGChatAPI{
			updateErrorText: "Inspecting\nReading tasks",
			updateErr: &bridgesdk.CommittedMutationError{
				Err: errors.New("Google Chat progress update result unavailable"),
			},
		}
		provider.apiFactory = func(*resolvedInstanceConfig) gchatAPI { return api }

		first := testGChatProgressRequest(
			managed.Instance.ID,
			"delivery-progress-committed-flush",
			1,
			"call-1",
			"agh__terminal",
			"Inspecting",
		)
		second := testGChatProgressRequest(
			managed.Instance.ID,
			first.Event.DeliveryID,
			2,
			"call-2",
			"agh__task_list",
			"Reading tasks",
		)
		for _, request := range []bridgepkg.DeliveryRequest{first, second} {
			if _, err := provider.handleBridgesProgress(
				context.Background(),
				&bridgesdk.Session{},
				request,
			); err != nil {
				t.Fatalf("handleBridgesProgress(seq=%d) error = %v", request.Event.Seq, err)
			}
		}

		final := first
		final.Event.Seq = 3
		final.Event.EventType = bridgepkg.DeliveryEventTypeFinal
		final.Event.Progress = nil
		final.Event.Content = bridgepkg.MessageContent{Text: "Final answer"}
		final.Event.Final = true
		if _, err := provider.handleBridgesDeliver(
			context.Background(),
			&bridgesdk.Session{},
			final,
		); err != nil {
			t.Fatalf("handleBridgesDeliver(final) error = %v", err)
		}

		api.mu.Lock()
		createAttempts := append([]gchatCreateMessageRequest(nil), api.createAttempts...)
		updates := append([]gchatUpdateMessageRequest(nil), api.updates...)
		api.mu.Unlock()
		if got, want := countGChatCreateRequests(createAttempts, "Final answer"), 1; got != want {
			t.Fatalf("Google Chat final create attempts = %d, want %d", got, want)
		}
		if got, want := len(updates), 1; got != want {
			t.Fatalf("Google Chat committed progress update attempts = %d, want %d", got, want)
		}
		if _, exists := provider.deliveries.Load(
			deliveryStateKey(managed.Instance.ID, first.Event.DeliveryID),
		); exists {
			t.Fatal("terminal delivery state still exists after final text")
		}
	})

	t.Run("Should remove and close delivery state after committed final text", func(t *testing.T) {
		t.Parallel()

		provider, err := newGChatProvider(io.Discard)
		if err != nil {
			t.Fatalf("newGChatProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testBridgeRuntime(t, time.Unix(8_437, 0).UTC(), "brg-final-committed")
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"all","grouping":"accumulate","typing":false,"reactions":false}}`,
		)
		setGChatProviderRoute(provider, managed.Instance.ID, &resolvedInstanceConfig{
			managed:    managed,
			instanceID: managed.Instance.ID,
		})
		api := &fakeGChatAPI{
			createErrorText: "Final answer",
			createErr: &bridgesdk.CommittedMutationError{
				Err: errors.New("Google Chat final create result unavailable"),
			},
		}
		provider.apiFactory = func(*resolvedInstanceConfig) gchatAPI { return api }

		progress := testGChatProgressRequest(
			managed.Instance.ID,
			"delivery-final-committed",
			1,
			"call-1",
			"agh__terminal",
			"Inspecting",
		)
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			progress,
		); err != nil {
			t.Fatalf("handleBridgesProgress() error = %v", err)
		}
		dispatcher := provider.deliveryState(managed.Instance.ID, progress.Event.DeliveryID).Progress
		if dispatcher == nil {
			t.Fatal("progress dispatcher = nil before final delivery")
		}

		final := progress
		final.Event.Seq = 2
		final.Event.EventType = bridgepkg.DeliveryEventTypeFinal
		final.Event.Progress = nil
		final.Event.Content = bridgepkg.MessageContent{Text: "Final answer"}
		final.Event.Final = true
		_, err = provider.handleBridgesDeliver(
			context.Background(),
			&bridgesdk.Session{},
			final,
		)
		var committedErr *bridgesdk.CommittedMutationError
		if !errors.As(err, &committedErr) {
			t.Fatalf("handleBridgesDeliver(final) error = %T %v, want CommittedMutationError", err, err)
		}
		if _, exists := provider.deliveries.Load(
			deliveryStateKey(managed.Instance.ID, progress.Event.DeliveryID),
		); exists {
			t.Fatal("delivery state still exists after committed final text")
		}
		if err := dispatcher.Flush(context.Background()); err == nil || !strings.Contains(err.Error(), "closed") {
			t.Fatalf("removed progress dispatcher Flush() error = %v, want closed dispatcher", err)
		}

		api.mu.Lock()
		createAttempts := append([]gchatCreateMessageRequest(nil), api.createAttempts...)
		api.mu.Unlock()
		if got, want := countGChatCreateRequests(createAttempts, "Final answer"), 1; got != want {
			t.Fatalf("Google Chat committed final create attempts = %d, want %d", got, want)
		}
	})

	t.Run("Should ignore unsupported affordances and remove lifecycle-only state", func(t *testing.T) {
		t.Parallel()

		provider, err := newGChatProvider(io.Discard)
		if err != nil {
			t.Fatalf("newGChatProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testBridgeRuntime(t, time.Unix(8_450, 0).UTC(), "brg-progress-lifecycle")
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"all","grouping":"separate","typing":true,"reactions":true}}`,
		)
		setGChatProviderRoute(provider, managed.Instance.ID, &resolvedInstanceConfig{
			managed:    managed,
			instanceID: managed.Instance.ID,
		})
		api := &fakeGChatAPI{}
		provider.apiFactory = func(*resolvedInstanceConfig) gchatAPI { return api }

		request := testGChatProgressRequest(
			managed.Instance.ID,
			"delivery-progress-lifecycle",
			1,
			"call-lifecycle",
			"agh__terminal",
			"Inspecting",
		)
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			request,
		); err != nil {
			t.Fatalf("handleBridgesProgress() error = %v", err)
		}
		final := request
		final.Event.Seq = 2
		final.Event.EventType = bridgepkg.DeliveryEventTypeFinal
		final.Event.Progress = nil
		final.Event.Final = true
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			final,
		); err != nil {
			t.Fatalf("handleBridgesProgress(lifecycle final) error = %v", err)
		}

		api.mu.Lock()
		messages := append([]gchatCreateMessageRequest(nil), api.messages...)
		updates := append([]gchatUpdateMessageRequest(nil), api.updates...)
		api.mu.Unlock()
		if got, want := len(messages), 1; got != want {
			t.Fatalf("Google Chat progress create calls = %d, want %d", got, want)
		}
		if got, want := messages[0].Text, "Inspecting"; got != want {
			t.Fatalf("Google Chat progress text = %q, want %q", got, want)
		}
		if len(updates) != 0 {
			t.Fatalf("Google Chat progress patch calls = %d, want zero", len(updates))
		}
		_, deliveryExists := provider.deliveries.Load(deliveryStateKey(managed.Instance.ID, request.Event.DeliveryID))
		if deliveryExists {
			t.Fatal("lifecycle-only delivery state still exists, want terminal removal")
		}

		staleDeliveryID := "delivery-progress-lifecycle-stale"
		provider.deliveries.Store(deliveryStateKey(managed.Instance.ID, staleDeliveryID), deliveryState{
			RemoteMessageID: "spaces/AAA/messages/stale",
		})
		staleFinal := final
		staleFinal.Event.DeliveryID = staleDeliveryID
		staleFinal.Event.Seq = 3
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			staleFinal,
		); err != nil {
			t.Fatalf("handleBridgesProgress(stale lifecycle final) error = %v", err)
		}
		_, staleExists := provider.deliveries.Load(deliveryStateKey(managed.Instance.ID, staleDeliveryID))
		if staleExists {
			t.Fatal("dispatcher-free lifecycle-only state still exists, want terminal removal")
		}
	})
}

func TestMapGChatDirectAndPubSubPayloads(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 20, 0, 0, 0, time.UTC)
	managed := testBridgeRuntime(t, now, "brg-gchat")

	directMessage, ok, err := mapDirectMessageEvent(mustUnmarshalGChatEvent(t, `{
		"chat": {
			"eventTime": "`+now.Format(time.RFC3339Nano)+`",
			"messagePayload": {
				"space": {"name": "spaces/AAA", "type": "SPACE"},
				"message": {
					"name": "spaces/AAA/messages/msg-1",
					"argumentText": "Need a summary",
					"createTime": "`+now.Format(time.RFC3339Nano)+`",
					"sender": {
						"name": "users/123",
						"displayName": "Alice Example",
						"email": "alice@example.com"
					},
					"thread": {"name": "spaces/AAA/threads/thread-1"}
				}
			}
		}
	}`), managed, now, nil)
	if err != nil {
		t.Fatalf("mapDirectMessageEvent() error = %v", err)
	}
	if !ok {
		t.Fatal("mapDirectMessageEvent() ok = false, want true")
	}
	if got, want := directMessage.Envelope.GroupID, "spaces/AAA"; got != want {
		t.Fatalf("direct message group id = %q, want %q", got, want)
	}
	if got, want := directMessage.Envelope.Content.Text, "Need a summary"; got != want {
		t.Fatalf("direct message text = %q, want %q", got, want)
	}
	if got, want := directMessage.Envelope.ThreadID, encodeGChatThreadID(gchatThreadRef{
		SpaceName:  "spaces/AAA",
		ThreadName: "spaces/AAA/threads/thread-1",
	}); got != want {
		t.Fatalf("direct message thread id = %q, want %q", got, want)
	}

	action, ok, err := mapDirectActionEvent(mustUnmarshalGChatEvent(t, `{
		"chat": {
			"buttonClickedPayload": {
				"space": {"name": "spaces/AAA", "type": "SPACE"},
				"message": {
					"name": "spaces/AAA/messages/msg-2",
					"thread": {"name": "spaces/AAA/threads/thread-2"}
				},
				"user": {
					"name": "users/234",
					"displayName": "Bob"
				}
			}
		},
		"commonEventObject": {
			"parameters": {
				"actionId": "approve",
				"value": "yes"
			}
		}
	}`), managed, now)
	if err != nil {
		t.Fatalf("mapDirectActionEvent() error = %v", err)
	}
	if !ok {
		t.Fatal("mapDirectActionEvent() ok = false, want true")
	}
	if got, want := action.Envelope.EventFamily, bridgepkg.InboundEventFamilyAction; got != want {
		t.Fatalf("action family = %q, want %q", got, want)
	}
	if got, want := action.Envelope.Action.ActionID, "approve"; got != want {
		t.Fatalf("action id = %q, want %q", got, want)
	}

	pubsubMessage, err := mapPubSubMessageEvent(gchatWorkspaceEventNotification{
		EventType:      "google.workspace.chat.message.v1.created",
		EventTime:      now.Format(time.RFC3339Nano),
		TargetResource: "//chat.googleapis.com/spaces/DM1",
		Message: &gchatMessage{
			Name:       "spaces/DM1/messages/msg-3",
			Text:       "hello from dm",
			CreateTime: now.Format(time.RFC3339Nano),
			Sender:     gchatUser{Name: "users/345", DisplayName: "Carol", Email: "carol@example.com"},
			Space:      &gchatSpace{Name: "spaces/DM1", Type: "DM"},
		},
	}, managed, now, nil)
	if err != nil {
		t.Fatalf("mapPubSubMessageEvent() error = %v", err)
	}
	if got, want := pubsubMessage.Envelope.PeerID, "spaces/DM1"; got != want {
		t.Fatalf("pubsub message peer id = %q, want %q", got, want)
	}
	if !pubsubMessage.Direct {
		t.Fatal("pubsub message Direct = false, want true")
	}

	api := &fakeGChatAPI{
		messagesMap: map[string]gchatMessage{
			"spaces/AAA/messages/msg-4": {
				Name:   "spaces/AAA/messages/msg-4",
				Space:  &gchatSpace{Name: "spaces/AAA", Type: "SPACE"},
				Thread: &gchatThread{Name: "spaces/AAA/threads/thread-4"},
			},
		},
	}
	reaction, err := mapPubSubReactionEvent(context.Background(), api, gchatWorkspaceEventNotification{
		EventType:      "google.workspace.chat.reaction.v1.created",
		EventTime:      now.Format(time.RFC3339Nano),
		TargetResource: "//chat.googleapis.com/spaces/AAA",
		Reaction: &gchatReaction{
			Name: "spaces/AAA/messages/msg-4/reactions/reaction-1",
			Emoji: &struct {
				Unicode string `json:"unicode,omitempty"`
			}{Unicode: "👍"},
			User: &gchatUser{Name: "users/456", DisplayName: "Dave"},
		},
	}, managed, now)
	if err != nil {
		t.Fatalf("mapPubSubReactionEvent() error = %v", err)
	}
	if got, want := reaction.Envelope.EventFamily, bridgepkg.InboundEventFamilyReaction; got != want {
		t.Fatalf("reaction family = %q, want %q", got, want)
	}
	if got, want := reaction.Envelope.GroupID, "spaces/AAA"; got != want {
		t.Fatalf("reaction group id = %q, want %q", got, want)
	}
	if got, want := reaction.Envelope.ThreadID, encodeGChatThreadID(gchatThreadRef{
		SpaceName:  "spaces/AAA",
		ThreadName: "spaces/AAA/threads/thread-4",
	}); got != want {
		t.Fatalf("reaction thread id = %q, want %q", got, want)
	}
}

func TestMapGChatReplyContext(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 12, 17, 0, 0, 0, time.UTC)
	managed := testBridgeRuntime(t, now, "brg-gchat-replies")
	managed.Instance.Scope = bridgepkg.ScopeWorkspace
	managed.Instance.WorkspaceID = "ws-a"
	parents := bridgesdk.NewParentMessageCache(0)

	t.Run("Should fill quoted reply context from the scoped parent cache", func(t *testing.T) {
		t.Parallel()

		root, ok, err := mapDirectMessageEvent(mustUnmarshalGChatEvent(t, `{
			"chat": {
				"eventTime": "`+now.Format(time.RFC3339Nano)+`",
				"messagePayload": {
					"space": {"name": "spaces/AAA", "type": "SPACE"},
					"message": {
						"name": "spaces/AAA/messages/root",
						"text": "original proposal",
						"createTime": "`+now.Format(time.RFC3339Nano)+`",
						"sender": {"name": "users/parent", "displayName": "Bob"},
						"thread": {"name": "spaces/AAA/threads/thread-1"}
					}
				}
			}
		}`), managed, now, parents)
		if err != nil || !ok {
			t.Fatalf("mapDirectMessageEvent(root) = (ok=%v, err=%v), want mapped", ok, err)
		}
		if root.Envelope.PlatformMessageID != "spaces/AAA/messages/root" {
			t.Fatalf("root PlatformMessageID = %q, want cached message id", root.Envelope.PlatformMessageID)
		}
		parents.RememberEnvelope(root.Envelope)

		replyEvent := mustUnmarshalGChatEvent(t, `{
			"chat": {
				"eventTime": "`+now.Add(time.Minute).Format(time.RFC3339Nano)+`",
				"messagePayload": {
					"space": {"name": "spaces/AAA", "type": "SPACE"},
					"message": {
						"name": "spaces/AAA/messages/reply",
						"text": "I agree",
						"createTime": "`+now.Add(time.Minute).Format(time.RFC3339Nano)+`",
						"sender": {"name": "users/reply", "displayName": "Alice"},
						"thread": {"name": "spaces/AAA/threads/thread-1"},
						"quotedMessageMetadata": {
							"name": "spaces/AAA/messages/root",
							"quoteType": "REPLY"
						}
					}
				}
			}
		}`)
		reply, ok, err := mapDirectMessageEvent(replyEvent, managed, now.Add(time.Minute), parents)
		if err != nil || !ok {
			t.Fatalf("mapDirectMessageEvent(reply) = (ok=%v, err=%v), want mapped", ok, err)
		}
		if reply.Envelope.ReplyToText != "original proposal" ||
			reply.Envelope.ReplyToAuthorID != "users/parent" ||
			reply.Envelope.ReplyToAuthorName != "Bob" {
			t.Fatalf(
				"reply context = (%q, %q, %q), want cached parent",
				reply.Envelope.ReplyToText,
				reply.Envelope.ReplyToAuthorID,
				reply.Envelope.ReplyToAuthorName,
			)
		}
		if shouldBatchGChatMessage(replyEvent.Chat.MessagePayload.Message) {
			t.Fatal("shouldBatchGChatMessage(quoted reply) = true, want false")
		}

		inlineEvent := mustUnmarshalGChatEvent(t, `{
			"chat": {
				"eventTime": "`+now.Add(2*time.Minute).Format(time.RFC3339Nano)+`",
				"messagePayload": {
					"space": {"name": "spaces/AAA", "type": "SPACE"},
					"message": {
						"name": "spaces/AAA/messages/inline-reply",
						"text": "I agree from a cold process",
						"createTime": "`+now.Add(2*time.Minute).Format(time.RFC3339Nano)+`",
						"sender": {"name": "users/reply", "displayName": "Alice"},
						"thread": {"name": "spaces/AAA/threads/thread-1"},
						"quotedMessageMetadata": {
							"name": "spaces/AAA/messages/root",
							"quoteType": "REPLY",
							"quotedMessageSnapshot": {
								"name": "spaces/AAA/messages/root",
								"text": "provider snapshot proposal",
								"sender": "users/parent"
							}
						}
					}
				}
			}
		}`)
		inline, ok, err := mapDirectMessageEvent(
			inlineEvent,
			managed,
			now.Add(2*time.Minute),
			bridgesdk.NewParentMessageCache(0),
		)
		if err != nil || !ok {
			t.Fatalf("mapDirectMessageEvent(inline reply) = (ok=%v, err=%v), want mapped", ok, err)
		}
		if inline.Envelope.ReplyToText != "provider snapshot proposal" ||
			inline.Envelope.ReplyToAuthorID != "users/parent" ||
			inline.Envelope.ReplyToAuthorName != "" {
			t.Fatalf(
				"inline reply context = (%q, %q, %q), want provider snapshot without display name",
				inline.Envelope.ReplyToText,
				inline.Envelope.ReplyToAuthorID,
				inline.Envelope.ReplyToAuthorName,
			)
		}

		cold, ok, err := mapDirectMessageEvent(
			replyEvent,
			managed,
			now.Add(time.Minute),
			bridgesdk.NewParentMessageCache(0),
		)
		if err != nil || !ok {
			t.Fatalf("mapDirectMessageEvent(cold reply) = (ok=%v, err=%v), want mapped", ok, err)
		}
		if cold.Envelope.ReplyToText != "" ||
			cold.Envelope.ReplyToAuthorID != "" ||
			cold.Envelope.ReplyToAuthorName != "" {
			t.Fatalf("cold reply context = %#v, want empty cache-miss fields", cold.Envelope)
		}
	})
}

func TestAllowGChatDirectMessagePoliciesAndModeValidation(t *testing.T) {
	t.Parallel()

	user := gchatUserIdentity{ID: "users/123", Username: "alice@example.com", DisplayName: "Alice"}

	if !allowGChatDirectMessage(&resolvedInstanceConfig{dmPolicy: bridgepkg.BridgeDMPolicyOpen}, user, true) {
		t.Fatal("allowGChatDirectMessage(open) = false, want true")
	}
	if !allowGChatDirectMessage(&resolvedInstanceConfig{
		dmPolicy:       bridgepkg.BridgeDMPolicyAllowlist,
		allowUserIDs:   map[string]struct{}{"users/123": {}},
		allowUsernames: map[string]struct{}{"alice@example.com": {}},
	}, user, true) {
		t.Fatal("allowGChatDirectMessage(allowlist) = false, want true")
	}
	if !allowGChatDirectMessage(&resolvedInstanceConfig{
		dmPolicy:        bridgepkg.BridgeDMPolicyPairing,
		pairedUsernames: map[string]struct{}{"alice@example.com": {}},
	}, user, true) {
		t.Fatal("allowGChatDirectMessage(pairing) = false, want true")
	}
	if allowGChatDirectMessage(&resolvedInstanceConfig{dmPolicy: bridgepkg.BridgeDMPolicyAllowlist}, user, true) {
		t.Fatal("allowGChatDirectMessage(rejected) = true, want false")
	}
	if !validGChatMode(gchatModeDirect) || !validGChatMode(gchatModePubSub) || !validGChatMode(gchatModeHybrid) {
		t.Fatal("validGChatMode() rejected a supported mode")
	}
	if validGChatMode("unknown") {
		t.Fatal("validGChatMode(unknown) = true, want false")
	}
}

func TestExecuteGChatDeliveryPostEditDeleteAndResume(t *testing.T) {
	t.Parallel()

	api := &fakeGChatAPI{}

	startReq := testDeliveryRequest("brg-gchat", "delivery-1", 1, bridgepkg.DeliveryEventTypeStart, false)
	startAck, state, err := executeGChatDelivery(context.Background(), api, startReq, deliveryState{})
	if err != nil {
		t.Fatalf("executeGChatDelivery(start) error = %v", err)
	}
	if got, want := startAck.RemoteMessageID, "spaces/AAA/messages/msg-1"; got != want {
		t.Fatalf("startAck.RemoteMessageID = %q, want %q", got, want)
	}

	finalReq := testDeliveryRequest("brg-gchat", "delivery-1", 2, bridgepkg.DeliveryEventTypeFinal, true)
	finalAck, state, err := executeGChatDelivery(context.Background(), api, finalReq, state)
	if err != nil {
		t.Fatalf("executeGChatDelivery(final) error = %v", err)
	}
	if got, want := finalAck.ReplaceRemoteMessageID, "spaces/AAA/messages/msg-1"; got != want {
		t.Fatalf("finalAck.ReplaceRemoteMessageID = %q, want %q", got, want)
	}

	deleteReq := testDeleteRequest("brg-gchat", "delivery-1", 3, finalAck.RemoteMessageID)
	deleteAck, _, err := executeGChatDelivery(context.Background(), api, deleteReq, state)
	if err != nil {
		t.Fatalf("executeGChatDelivery(delete) error = %v", err)
	}
	if got, want := deleteAck.RemoteMessageID, finalAck.RemoteMessageID; got != want {
		t.Fatalf("deleteAck.RemoteMessageID = %q, want %q", got, want)
	}

	resumeReq := testDeliveryRequest("brg-gchat", "delivery-2", 1, bridgepkg.DeliveryEventTypeResume, true)
	resumeReq.Event.Resume = &bridgepkg.DeliveryResumeState{LatestEventType: bridgepkg.DeliveryEventTypeFinal}
	resumeReq.Snapshot = &bridgepkg.DeliverySnapshot{
		DeliveryID:       "delivery-2",
		SessionID:        "sess-1",
		TurnID:           "turn-1",
		BridgeInstanceID: "brg-gchat",
		RoutingKey:       resumeReq.Event.RoutingKey,
		DeliveryTarget:   resumeReq.Event.DeliveryTarget,
		LatestSeq:        1,
		LatestEventType:  bridgepkg.DeliveryEventTypeFinal,
		CurrentContent:   bridgepkg.MessageContent{Text: "hello"},
		Final:            true,
		UpdatedAt:        time.Date(2026, 4, 15, 20, 5, 0, 0, time.UTC),
	}
	resumeAck, _, err := executeGChatDelivery(context.Background(), api, resumeReq, deliveryState{})
	if err != nil {
		t.Fatalf("executeGChatDelivery(resume) error = %v", err)
	}
	if got, want := resumeAck.RemoteMessageID, "spaces/AAA/messages/msg-2"; got != want {
		t.Fatalf("resumeAck.RemoteMessageID = %q, want %q", got, want)
	}
}

func TestExecuteGChatDeliveryChunks(t *testing.T) {
	t.Parallel()

	const maxMessageBytes = 32_000

	t.Run("Should update an explicit reference when local delivery state is empty", func(t *testing.T) {
		t.Parallel()

		api := &fakeGChatAPI{}
		request := testDeliveryRequest(
			"brg-gchat",
			"delivery-explicit-edit",
			1,
			bridgepkg.DeliveryEventTypeDelta,
			false,
		)
		const referencedMessage = "spaces/AAA/messages/referenced"
		request.Event.Operation = bridgepkg.DeliveryOperationEdit
		request.Event.Reference = &bridgepkg.DeliveryMessageReference{RemoteMessageID: referencedMessage}
		request.Event.Content.Text = "edited text"

		ack, state, err := executeGChatDelivery(
			context.Background(),
			api,
			request,
			deliveryState{},
		)
		if err != nil {
			t.Fatalf("executeGChatDelivery() error = %v", err)
		}
		if got, want := len(api.updates), 1; got != want {
			t.Fatalf("UpdateMessage calls = %d, want %d", got, want)
		}
		if got := len(api.messages); got != 0 {
			t.Fatalf("CreateMessage calls = %d, want 0", got)
		}
		if got, want := api.updates[0].MessageName, referencedMessage; got != want {
			t.Fatalf("updated message = %q, want %q", got, want)
		}
		if got, want := api.updates[0].Text, request.Event.Content.Text; got != want {
			t.Fatalf("updated text = %q, want %q", got, want)
		}
		if ack.RemoteMessageID != referencedMessage || ack.ReplaceRemoteMessageID != referencedMessage {
			t.Fatalf(
				"ack remote ids = current:%q replace:%q, want %q",
				ack.RemoteMessageID,
				ack.ReplaceRemoteMessageID,
				referencedMessage,
			)
		}
		if state.RemoteMessageID != referencedMessage || state.ReplaceRemoteMessageID != referencedMessage {
			t.Fatalf(
				"state remote ids = current:%q replace:%q, want %q",
				state.RemoteMessageID,
				state.ReplaceRemoteMessageID,
				referencedMessage,
			)
		}
	})

	t.Run("Should create one preview then materialize byte-bounded continuations on final", func(t *testing.T) {
		t.Parallel()

		api := &fakeGChatAPI{}
		text := strings.Repeat("🙂", 9_000)
		startRequest := testDeliveryRequest(
			"brg-gchat",
			"delivery-chunked-create",
			1,
			bridgepkg.DeliveryEventTypeStart,
			false,
		)
		startRequest.Event.Content.Text = text
		wantChunks := bridgesdk.ChunkMessage(text, maxMessageBytes, func(value string) int {
			return len(value)
		})

		startAck, startState, err := executeGChatDelivery(
			context.Background(),
			api,
			startRequest,
			deliveryState{},
		)
		if err != nil {
			t.Fatalf("executeGChatDelivery(start) error = %v", err)
		}
		if got, want := len(api.messages), 1; got != want {
			t.Fatalf("CreateMessage calls = %d, want %d", got, want)
		}
		if got, want := api.messages[0].Text, wantChunks[0]; got != want {
			t.Fatalf("preview text length = %d, want first chunk length %d", len(got), len(want))
		}
		if got, want := startAck.RemoteMessageID, "spaces/AAA/messages/msg-1"; got != want {
			t.Fatalf("start ack.RemoteMessageID = %q, want %q", got, want)
		}

		finalRequest := testDeliveryRequest(
			"brg-gchat",
			"delivery-chunked-create",
			2,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		finalRequest.Event.Content.Text = text
		finalAck, finalState, err := executeGChatDelivery(
			context.Background(),
			api,
			finalRequest,
			startState,
		)
		if err != nil {
			t.Fatalf("executeGChatDelivery(final) error = %v", err)
		}
		if got, want := len(api.updates), 1; got != want {
			t.Fatalf("UpdateMessage calls = %d, want %d", got, want)
		}
		if got, want := api.updates[0].Text, wantChunks[0]; got != want {
			t.Fatalf("final preview text length = %d, want first chunk length %d", len(got), len(want))
		}
		if got, want := len(api.messages), len(wantChunks); got != want {
			t.Fatalf("total CreateMessage calls = %d, want %d", got, want)
		}
		for index, wantChunk := range wantChunks[1:] {
			got := api.messages[index+1]
			if got.Text != wantChunk {
				t.Fatalf(
					"continuation %d text length = %d, want exact chunk length %d",
					index,
					len(got.Text),
					len(wantChunk),
				)
			}
			if len(got.Text) > maxMessageBytes {
				t.Fatalf("continuation %d length = %d, want <= %d", index, len(got.Text), maxMessageBytes)
			}
			if got.SpaceName != "spaces/AAA" || got.ThreadName != "spaces/AAA/threads/thread-1" {
				t.Fatalf(
					"continuation %d target = (%q, %q), want original space and thread",
					index,
					got.SpaceName,
					got.ThreadName,
				)
			}
		}
		wantRemoteID := "spaces/AAA/messages/msg-" + strconv.Itoa(len(wantChunks))
		if finalAck.RemoteMessageID != wantRemoteID || finalState.RemoteMessageID != wantRemoteID {
			t.Fatalf(
				"final remote ids = ack:%q state:%q, want %q",
				finalAck.RemoteMessageID,
				finalState.RemoteMessageID,
				wantRemoteID,
			)
		}
	})

	t.Run("Should keep an oversized cumulative preview in one edited message", func(t *testing.T) {
		t.Parallel()

		api := &fakeGChatAPI{}
		text := strings.Repeat("preview ", 5_000)
		request := testDeliveryRequest(
			"brg-gchat",
			"delivery-chunked-preview",
			2,
			bridgepkg.DeliveryEventTypeDelta,
			false,
		)
		request.Event.Content.Text = text
		wantChunks := bridgesdk.ChunkMessage(text, maxMessageBytes, func(value string) int {
			return len(value)
		})
		remoteID := "spaces/AAA/messages/preview"

		ack, state, err := executeGChatDelivery(
			context.Background(),
			api,
			request,
			deliveryState{LastSeq: 1, RemoteMessageID: remoteID},
		)
		if err != nil {
			t.Fatalf("executeGChatDelivery() error = %v", err)
		}
		if got, want := len(api.updates), 1; got != want {
			t.Fatalf("UpdateMessage calls = %d, want %d", got, want)
		}
		if got := len(api.messages); got != 0 {
			t.Fatalf("CreateMessage calls = %d, want 0 for a non-terminal preview", got)
		}
		if got, want := api.updates[0].Text, wantChunks[0]; got != want {
			t.Fatalf(
				"UpdateMessage text length = %d, want first chunk length %d",
				len(got),
				len(want),
			)
		}
		if ack.RemoteMessageID != remoteID || state.RemoteMessageID != remoteID {
			t.Fatalf(
				"active remote ids = ack:%q state:%q, want %q",
				ack.RemoteMessageID,
				state.RemoteMessageID,
				remoteID,
			)
		}
	})

	t.Run("Should edit the active message then post final continuations and acknowledge the last", func(t *testing.T) {
		t.Parallel()

		api := &fakeGChatAPI{}
		text := strings.Repeat("final ", 12_000)
		request := testDeliveryRequest(
			"brg-gchat",
			"delivery-chunked-final",
			2,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		request.Event.Content.Text = text
		wantChunks := bridgesdk.ChunkMessage(text, maxMessageBytes, func(value string) int {
			return len(value)
		})
		remoteID := "spaces/AAA/messages/preview"

		ack, state, err := executeGChatDelivery(
			context.Background(),
			api,
			request,
			deliveryState{LastSeq: 1, RemoteMessageID: remoteID},
		)
		if err != nil {
			t.Fatalf("executeGChatDelivery() error = %v", err)
		}
		if got, want := len(api.updates), 1; got != want {
			t.Fatalf("UpdateMessage calls = %d, want %d", got, want)
		}
		if got, want := api.updates[0].MessageName, remoteID; got != want {
			t.Fatalf("updated message = %q, want %q", got, want)
		}
		if got, want := api.updates[0].Text, wantChunks[0]; got != want {
			t.Fatalf(
				"updated text length = %d, want first chunk length %d",
				len(got),
				len(want),
			)
		}
		if got, want := len(api.messages), len(wantChunks)-1; got != want {
			t.Fatalf("continuation CreateMessage calls = %d, want %d", got, want)
		}
		for index, wantChunk := range wantChunks[1:] {
			got := api.messages[index]
			if got.Text != wantChunk {
				t.Fatalf(
					"continuation %d text length = %d, want exact chunk length %d",
					index,
					len(got.Text),
					len(wantChunk),
				)
			}
			if got.SpaceName != "spaces/AAA" || got.ThreadName != "spaces/AAA/threads/thread-1" {
				t.Fatalf(
					"continuation %d target = (%q, %q), want original space and thread",
					index,
					got.SpaceName,
					got.ThreadName,
				)
			}
		}
		wantRemoteID := "spaces/AAA/messages/msg-" + strconv.Itoa(len(wantChunks)-1)
		if ack.RemoteMessageID != wantRemoteID || state.RemoteMessageID != wantRemoteID {
			t.Fatalf(
				"final remote ids = ack:%q state:%q, want %q",
				ack.RemoteMessageID,
				state.RemoteMessageID,
				wantRemoteID,
			)
		}
		if ack.ReplaceRemoteMessageID != remoteID || state.ReplaceRemoteMessageID != remoteID {
			t.Fatalf(
				"replace remote ids = ack:%q state:%q, want %q",
				ack.ReplaceRemoteMessageID,
				state.ReplaceRemoteMessageID,
				remoteID,
			)
		}
	})

	t.Run("Should resume at the first unsent chunk after exhausting Retry-After attempts", func(t *testing.T) {
		t.Parallel()

		request := testDeliveryRequest(
			"brg-gchat",
			"delivery-partial-chunk",
			1,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		request.Event.Content.Text = strings.Repeat("partial Google Chat delivery ", 2_000)
		chunks := gchatDeliveryChunks(request.Event.Content.Text)
		if len(chunks) < 2 {
			t.Fatalf("gchatDeliveryChunks() returned %d chunk, want at least 2", len(chunks))
		}

		api := &fakeGChatAPI{
			createErrorText: chunks[1],
			createErr: &bridgesdk.RateLimitError{
				Err:        errors.New("rate limited continuation"),
				RetryAfter: time.Nanosecond,
			},
		}
		_, partialState, err := executeGChatDelivery(context.Background(), api, request, deliveryState{})
		if err == nil {
			t.Fatal("executeGChatDelivery(partial) error = nil, want exhausted rate limit")
		}
		if got, want := countGChatCreateRequests(
			api.createAttempts,
			chunks[1],
		), bridgesdk.DefaultRetryConfig().Attempts; got != want {
			t.Fatalf("failed continuation attempts = %d, want %d", got, want)
		}
		if got, want := countGChatCreateRequests(api.messages, chunks[0]), 1; got != want {
			t.Fatalf("successful first-chunk creates = %d, want %d", got, want)
		}

		api.createErr = nil
		resume := testGChatResumeRequest(request)
		if _, _, err := executeGChatDelivery(context.Background(), api, resume, partialState); err != nil {
			t.Fatalf("executeGChatDelivery(resume) error = %v", err)
		}
		if got, want := countGChatCreateRequests(api.messages, chunks[0]), 1; got != want {
			t.Fatalf("successful first-chunk creates after resume = %d, want %d", got, want)
		}
		if got, want := len(api.messages), len(chunks); got != want {
			t.Fatalf("successful chunk creates = %d, want %d", got, want)
		}
	})

	t.Run("Should stop after a committed create response omits its message name", func(t *testing.T) {
		t.Parallel()

		api := &fakeGChatAPI{createEmptyMessageName: true}
		request := testDeliveryRequest(
			"brg-gchat",
			"delivery-empty-name",
			1,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)

		ack, state, err := executeGChatDelivery(
			context.Background(),
			api,
			request,
			deliveryState{},
		)
		if got, want := bridgesdk.ClassifyError(err).Class, bridgesdk.ErrorClassPermanent; got != want {
			t.Fatalf("executeGChatDelivery() error class = %q, want %q: %v", got, want, err)
		}
		var committedErr *bridgesdk.CommittedMutationError
		if !errors.As(err, &committedErr) {
			t.Fatalf("executeGChatDelivery() error = %T, want CommittedMutationError", err)
		}
		if got, want := len(api.createAttempts), 1; got != want {
			t.Fatalf("create attempts = %d, want %d", got, want)
		}
		if ack != (bridgepkg.DeliveryAck{}) || state.LastSeq != 0 || state.RemoteMessageID != "" {
			t.Fatalf("unacknowledged delivery = ack:%#v state:%#v, want no advanced state", ack, state)
		}
	})

	t.Run("Should reject an invalid delivery request before calling Google Chat", func(t *testing.T) {
		t.Parallel()

		api := &fakeGChatAPI{}
		request := testDeliveryRequest(
			"brg-gchat",
			"delivery-invalid",
			1,
			bridgepkg.DeliveryEventTypeStart,
			false,
		)
		request.Event.BridgeInstanceID = "other-instance"

		if _, _, err := executeGChatDelivery(
			context.Background(),
			api,
			request,
			deliveryState{},
		); err == nil {
			t.Fatal("executeGChatDelivery() error = nil, want invalid request error")
		}
		if len(api.messages) != 0 || len(api.updates) != 0 {
			t.Fatalf("Google Chat calls = creates:%d updates:%d, want zero", len(api.messages), len(api.updates))
		}
	})
}

func TestVerifyGChatBearerTokens(t *testing.T) {
	t.Parallel()

	server := newGChatProviderTestServer(t)
	defer server.Close()

	directReq := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.test/gchat",
		strings.NewReader(`{"chat":{}}`),
	)
	directReq.Header.Set("Authorization", "Bearer "+server.signDirectToken(t, "123456789"))
	if err := verifyDirectBearerToken(context.Background(), directReq, &resolvedInstanceConfig{
		projectNumber:  "123456789",
		directIssuer:   gchatDefaultDirectIssuer,
		directCertsURL: server.DirectCertsURL(),
	}); err != nil {
		t.Fatalf("verifyDirectBearerToken(valid) error = %v", err)
	}
	if err := verifyDirectBearerToken(context.Background(), directReq, &resolvedInstanceConfig{
		projectNumber:  "wrong",
		directIssuer:   gchatDefaultDirectIssuer,
		directCertsURL: server.DirectCertsURL(),
	}); err == nil {
		t.Fatal("verifyDirectBearerToken(wrong audience) error = nil, want non-nil")
	}

	pubsubReq := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.test/gchat",
		strings.NewReader(`{"message":{"data":"e30="},"subscription":"sub"}`),
	)
	pubsubReq.Header.Set(
		"Authorization",
		"Bearer "+server.signPubSubToken(
			t,
			"https://example.test/pubsub",
			"push@example.iam.gserviceaccount.com",
			true,
		),
	)
	if err := verifyPubSubBearerToken(context.Background(), pubsubReq, &resolvedInstanceConfig{
		pubsubAudience:            "https://example.test/pubsub",
		pubsubIssuer:              gchatDefaultPubSubIssuerURL,
		pubsubCertsURL:            server.PubSubCertsURL(),
		pubsubServiceAccountEmail: "push@example.iam.gserviceaccount.com",
	}); err != nil {
		t.Fatalf("verifyPubSubBearerToken(valid) error = %v", err)
	}
	if err := verifyPubSubBearerToken(context.Background(), pubsubReq, &resolvedInstanceConfig{
		pubsubAudience:            "https://example.test/pubsub",
		pubsubIssuer:              gchatDefaultPubSubIssuerURL,
		pubsubCertsURL:            server.PubSubCertsURL(),
		pubsubServiceAccountEmail: "wrong@example.iam.gserviceaccount.com",
	}); err == nil {
		t.Fatal("verifyPubSubBearerToken(wrong email) error = nil, want non-nil")
	}

	unverifiedReq := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.test/gchat",
		strings.NewReader(`{"message":{"data":"e30="},"subscription":"sub"}`),
	)
	unverifiedReq.Header.Set(
		"Authorization",
		"Bearer "+server.signPubSubToken(
			t,
			"https://example.test/pubsub",
			"push@example.iam.gserviceaccount.com",
			false,
		),
	)
	if err := verifyPubSubBearerToken(context.Background(), unverifiedReq, &resolvedInstanceConfig{
		pubsubAudience:            "https://example.test/pubsub",
		pubsubIssuer:              gchatDefaultPubSubIssuerURL,
		pubsubCertsURL:            server.PubSubCertsURL(),
		pubsubServiceAccountEmail: "push@example.iam.gserviceaccount.com",
	}); err == nil {
		t.Fatal("verifyPubSubBearerToken(unverified email) error = nil, want non-nil")
	}
}

func TestHandleBridgesDeliverKeepsLastErrorWhenReadyReportFails(t *testing.T) {
	t.Setenv(gchatListenAddrEnv, reserveListenAddr(t))

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	runtime.apiFactory = func(*resolvedInstanceConfig) gchatAPI {
		return &fakeGChatAPI{}
	}

	now := time.Date(2026, 4, 15, 20, 11, 0, 0, time.UTC)
	managed := testBridgeRuntime(t, now, "brg-gchat")

	var reportMu sync.Mutex
	reportCalls := 0

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

			reportMu.Lock()
			reportCalls++
			callNumber := reportCalls
			reportMu.Unlock()

			if callNumber > 1 {
				return nil, subprocess.NewRPCError(-32099, "report ready failed", nil)
			}

			instance := managed.Instance
			instance.Status = payload.Status
			instance.Degradation = payload.Degradation
			return instance, nil
		},
	)

	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}
	if _, err := runtime.waitForInstanceConfig(managed.Instance.ID, time.Second); err != nil {
		t.Fatalf("waitForInstanceConfig() error = %v", err)
	}

	waitForCondition(t, func() bool {
		reportMu.Lock()
		defer reportMu.Unlock()
		return reportCalls >= 1
	})

	session := runtime.currentSession()
	if session == nil {
		t.Fatal("runtime.currentSession() = nil, want non-nil")
	}

	runtime.lifecycle.Host().ForgetStatus(managed.Instance.ID)

	ack, err := runtime.handleBridgesDeliver(
		context.Background(),
		session,
		testDeliveryRequest(managed.Instance.ID, "delivery-ready-state", 1, bridgepkg.DeliveryEventTypeStart, false),
	)
	if err != nil {
		t.Fatalf("handleBridgesDeliver() error = %v", err)
	}
	if got, want := ack.RemoteMessageID, "spaces/AAA/messages/msg-1"; got != want {
		t.Fatalf("ack.RemoteMessageID = %q, want %q", got, want)
	}
	if err := runtime.healthCheck(); err == nil || !strings.Contains(err.Error(), "report ready failed") {
		t.Fatalf("healthCheck() error = %v, want readiness report failure", err)
	}
}

func TestRuntimeInitializeWebhookAndDeliveryFlow(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mockAPI := newGChatProviderTestServer(t)
	t.Setenv(gchatListenAddrEnv, listenAddr)
	t.Setenv(gchatAPIBaseEnv, mockAPI.URL())
	t.Setenv(gchatTokenURLEnv, mockAPI.TokenURL())
	t.Setenv(gchatDirectCertsEnv, mockAPI.DirectCertsURL())
	t.Setenv(gchatPubSubCertsEnv, mockAPI.PubSubCertsURL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 20, 10, 0, 0, time.UTC)
	managed := testBridgeRuntime(t, now, "brg-gchat")
	managed.Instance.ProviderConfig = mustJSON(t, map[string]any{
		"mode": "hybrid",
		"webhook": map[string]any{
			"listen_addr": listenAddr,
			"path":        "/gchat/brg-gchat",
		},
		"verification": map[string]any{
			"pubsub_audience":              "https://example.test/pubsub",
			"pubsub_service_account_email": "push@example.iam.gserviceaccount.com",
		},
	})
	managed.BoundSecrets = []subprocess.InitializeBridgeBoundSecret{
		{BindingName: "credentials_json", Kind: "json", Value: testCredentialsJSON(t)},
		{BindingName: "project_number", Kind: "token", Value: "123456789"},
	}

	var ingested []bridgepkg.InboundMessageEnvelope
	var mu sync.Mutex

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
			instance.Degradation = payload.Degradation
			return instance, nil
		},
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
				SessionID:    "sess-1",
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

	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	handshake := waitForJSONFile[bridgesdk.InitializeMarker](t, env.HandshakePath)
	if got, want := handshake.Request.Runtime.Bridge.Provider, "gchat"; got != want {
		t.Fatalf("handshake provider = %q, want %q", got, want)
	}
	waitForCondition(t, func() bool {
		return strings.TrimSpace(runtime.http.Address()) != ""
	})

	serverAddr := runtime.http.Address()
	webhookURL := "http://" + serverAddr + "/gchat/brg-gchat"

	directReq, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		webhookURL,
		strings.NewReader(directWebhookPayload()),
	)
	if err != nil {
		t.Fatalf("http.NewRequest(direct) error = %v", err)
	}
	directReq.Header.Set("Content-Type", "application/json")
	directReq.Header.Set("Authorization", "Bearer "+mockAPI.signDirectToken(t, "123456789"))
	directResp, err := http.DefaultClient.Do(directReq)
	if err != nil {
		t.Fatalf("http.DefaultClient.Do(direct) error = %v", err)
	}
	defer func() {
		if err := directResp.Body.Close(); err != nil {
			t.Errorf("direct response body close error = %v", err)
		}
	}()
	if got, want := directResp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("direct webhook status = %d, want %d", got, want)
	}

	pubsubReq, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		webhookURL,
		strings.NewReader(pubSubReactionPayload(t)),
	)
	if err != nil {
		t.Fatalf("http.NewRequest(pubsub) error = %v", err)
	}
	pubsubReq.Header.Set("Content-Type", "application/json")
	pubsubReq.Header.Set(
		"Authorization",
		"Bearer "+mockAPI.signPubSubToken(
			t,
			"https://example.test/pubsub",
			"push@example.iam.gserviceaccount.com",
			true,
		),
	)
	pubsubResp, err := http.DefaultClient.Do(pubsubReq)
	if err != nil {
		t.Fatalf("http.DefaultClient.Do(pubsub) error = %v", err)
	}
	defer func() {
		if err := pubsubResp.Body.Close(); err != nil {
			t.Errorf("Pub/Sub response body close error = %v", err)
		}
	}()
	if got, want := pubsubResp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("pubsub webhook status = %d, want %d", got, want)
	}

	ingests := waitForJSONLinesFile[bridgesdk.IngestMarker](
		t,
		env.IngestPath,
		func(items []bridgesdk.IngestMarker) bool { return len(items) >= 2 },
	)
	if got, want := ingests[0].Envelope.EventFamily, bridgepkg.InboundEventFamilyMessage; got != want {
		t.Fatalf("ingests[0].Envelope.EventFamily = %q, want %q", got, want)
	}
	if got, want := ingests[1].Envelope.EventFamily, bridgepkg.InboundEventFamilyReaction; got != want {
		t.Fatalf("ingests[1].Envelope.EventFamily = %q, want %q", got, want)
	}

	var ack bridgepkg.DeliveryAck
	if err := hostPeer.Call(
		context.Background(),
		"bridges/deliver",
		testDeliveryRequest("brg-gchat", "delivery-1", 1, bridgepkg.DeliveryEventTypeStart, false),
		&ack,
	); err != nil {
		t.Fatalf("hostPeer.Call(start delivery) error = %v", err)
	}
	if err := hostPeer.Call(
		context.Background(),
		"bridges/deliver",
		testDeliveryRequest("brg-gchat", "delivery-1", 2, bridgepkg.DeliveryEventTypeFinal, true),
		&ack,
	); err != nil {
		t.Fatalf("hostPeer.Call(final delivery) error = %v", err)
	}
	records := waitForJSONLinesFile[bridgesdk.DeliveryMarker](
		t,
		env.DeliveryPath,
		func(items []bridgesdk.DeliveryMarker) bool { return len(items) >= 2 },
	)
	if records[0].Ack == nil || records[1].Ack == nil {
		t.Fatalf("delivery markers = %#v, want recorded acks", records)
	}
	if len(mockAPI.Calls()) < 3 {
		t.Fatalf("len(mock api calls) = %d, want at least 3", len(mockAPI.Calls()))
	}
	mu.Lock()
	if got, want := len(ingested), 2; got != want {
		t.Fatalf("len(ingested) = %d, want %d", got, want)
	}
	mu.Unlock()
}

func TestRuntimePubSubMessageAndDirectDeliveryPaths(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mockAPI := newGChatProviderTestServer(t)
	t.Setenv(gchatListenAddrEnv, listenAddr)
	t.Setenv(gchatAPIBaseEnv, mockAPI.URL())
	t.Setenv(gchatTokenURLEnv, mockAPI.TokenURL())
	t.Setenv(gchatDirectCertsEnv, mockAPI.DirectCertsURL())
	t.Setenv(gchatPubSubCertsEnv, mockAPI.PubSubCertsURL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 20, 12, 0, 0, time.UTC)
	managed := testBridgeRuntime(t, now, "brg-gchat")
	managed.Instance.ProviderConfig = mustJSON(t, map[string]any{
		"mode": "hybrid",
		"webhook": map[string]any{
			"listen_addr": listenAddr,
			"path":        "/gchat/brg-gchat",
		},
		"verification": map[string]any{
			"pubsub_audience":              "https://example.test/pubsub",
			"pubsub_service_account_email": "push@example.iam.gserviceaccount.com",
		},
	})

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
			instance.Degradation = payload.Degradation
			return instance, nil
		},
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
			return bridgepkg.BridgesMessagesIngestResult{
				SessionID: "sess-2",
				RoutingKey: bridgepkg.RoutingKey{
					Scope:            envelope.Scope,
					WorkspaceID:      envelope.WorkspaceID,
					BridgeInstanceID: envelope.BridgeInstanceID,
					GroupID:          envelope.GroupID,
					ThreadID:         envelope.ThreadID,
					PeerID:           envelope.PeerID,
				},
			}, nil
		},
	)

	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	cfg, err := runtime.waitForInstanceConfig("brg-gchat", time.Second)
	if err != nil {
		t.Fatalf("waitForInstanceConfig() error = %v", err)
	}
	if cfg.configError != nil {
		t.Fatalf("waitForInstanceConfig() configError = %v, want nil", cfg.configError)
	}
	session := runtime.currentSession()
	if session == nil {
		t.Fatal("runtime.currentSession() = nil, want non-nil")
	}

	recorder := httptest.NewRecorder()
	err = runtime.handlePubSubWebhook(context.Background(), recorder, &cfg, bridgesdk.WebhookRequest{
		Body:       []byte(pubSubMessagePayload(t, now)),
		ReceivedAt: now,
	})
	if err != nil {
		t.Fatalf("handlePubSubWebhook(message) error = %v", err)
	}
	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("handlePubSubWebhook(message) status = %d, want %d", got, want)
	}

	badDeleteReq := testDeleteRequest("brg-gchat", "delivery-error", 1, "")
	if _, err := runtime.handleBridgesDeliver(context.Background(), session, badDeleteReq); err == nil {
		t.Fatal("handleBridgesDeliver(delete without remote) error = nil, want non-nil")
	}

	startReq := testDeliveryRequest("brg-gchat", "delivery-delete", 1, bridgepkg.DeliveryEventTypeStart, false)
	ack, err := runtime.handleBridgesDeliver(context.Background(), session, startReq)
	if err != nil {
		t.Fatalf("handleBridgesDeliver(start) error = %v", err)
	}
	deleteReq := testDeleteRequest("brg-gchat", "delivery-delete", 2, ack.RemoteMessageID)
	deleteAck, err := runtime.handleBridgesDeliver(context.Background(), session, deleteReq)
	if err != nil {
		t.Fatalf("handleBridgesDeliver(delete) error = %v", err)
	}
	if got, want := deleteAck.RemoteMessageID, ack.RemoteMessageID; got != want {
		t.Fatalf("deleteAck.RemoteMessageID = %q, want %q", got, want)
	}

	deliveries := waitForJSONLinesFile[bridgesdk.DeliveryMarker](
		t,
		env.DeliveryPath,
		func(items []bridgesdk.DeliveryMarker) bool { return len(items) >= 3 },
	)
	if deliveries[0].Error == "" || deliveries[1].Ack == nil || deliveries[2].Ack == nil {
		t.Fatalf("delivery markers = %#v, want error, start ack, delete ack", deliveries)
	}

	ingests := waitForJSONLinesFile[bridgesdk.IngestMarker](
		t,
		env.IngestPath,
		func(items []bridgesdk.IngestMarker) bool { return len(items) >= 1 },
	)
	if got, want := ingests[len(ingests)-1].Envelope.EventFamily, bridgepkg.InboundEventFamilyMessage; got != want {
		t.Fatalf("last ingest event family = %q, want %q", got, want)
	}
}

func TestGChatHealthAndRouteWaitHelpers(t *testing.T) {
	t.Parallel()

	provider, err := newGChatProvider(io.Discard)
	if err != nil {
		t.Fatalf("newGChatProvider() error = %v", err)
	}
	provider.setLastError(errors.New("boom"))
	if err := provider.healthCheck(); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("healthCheck() error = %v, want boom", err)
	}
	provider.clearLastError()
	if err := provider.healthCheck(); err != nil {
		t.Fatalf("healthCheck() after clear error = %v", err)
	}

	waitDone := make(chan resolvedInstanceConfig, 1)
	go func() {
		cfg, waitErr := provider.waitForInstanceConfig("brg-gchat", 200*time.Millisecond)
		if waitErr == nil {
			waitDone <- cfg
		}
	}()
	setGChatProviderRoute(provider, "brg-gchat", &resolvedInstanceConfig{instanceID: "brg-gchat"})
	select {
	case cfg := <-waitDone:
		if got, want := cfg.instanceID, "brg-gchat"; got != want {
			t.Fatalf("waitForInstanceConfig instanceID = %q, want %q", got, want)
		}
	case <-t.Context().Done():
		t.Fatal("waitForInstanceConfig() did not wake")
	}
}

func TestResolveInstanceConfigAndInitialState(t *testing.T) {
	server := newGChatProviderTestServer(t)
	defer server.Close()

	provider, err := newGChatProvider(io.Discard)
	if err != nil {
		t.Fatalf("newGChatProvider() error = %v", err)
	}
	provider.apiFactory = func(cfg *resolvedInstanceConfig) gchatAPI {
		return &gchatBotClient{cfg: *cfg}
	}

	now := time.Date(2026, 4, 15, 20, 20, 0, 0, time.UTC)
	managed := testBridgeRuntime(t, now, "brg-gchat")
	managed.Instance.ProviderConfig = mustJSON(t, map[string]any{
		"mode": "hybrid",
		"webhook": map[string]any{
			"path": "/custom/gchat",
		},
		"verification": map[string]any{
			"pubsub_audience":              "https://example.test/pubsub",
			"pubsub_service_account_email": "push@example.iam.gserviceaccount.com",
		},
		"dm": map[string]any{
			"allow_usernames": []string{" Alice@example.com ", "@alice@example.com"},
		},
		"batching": map[string]any{
			"delay_ms":        5,
			"split_delay_ms":  2,
			"split_threshold": 3,
		},
	})

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

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
			instance.Degradation = payload.Degradation
			return instance, nil
		},
	)
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
		func(context.Context, json.RawMessage) (any, error) {
			return bridgepkg.BridgesMessagesIngestResult{SessionID: "sess-1"}, nil
		},
	)

	t.Setenv(gchatListenAddrEnv, reserveListenAddr(t))
	t.Setenv(gchatAPIBaseEnv, server.URL())
	t.Setenv(gchatTokenURLEnv, server.TokenURL())
	t.Setenv(gchatDirectCertsEnv, server.DirectCertsURL())
	t.Setenv(gchatPubSubCertsEnv, server.PubSubCertsURL())

	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	session := runtime.currentSession()
	if session == nil {
		t.Fatal("runtime.currentSession() = nil, want non-nil")
	}

	cfg := provider.resolveInstanceConfig(session, managed)
	if cfg.configError != nil {
		t.Fatalf("resolveInstanceConfig() configError = %v", cfg.configError)
	}
	if got, want := cfg.apiBaseURL, server.URL(); got != want {
		t.Fatalf("cfg.apiBaseURL = %q, want %q", got, want)
	}
	if got, want := cfg.tokenURL, server.TokenURL(); got != want {
		t.Fatalf("cfg.tokenURL = %q, want %q", got, want)
	}
	if got, want := cfg.directCertsURL, server.DirectCertsURL(); got != want {
		t.Fatalf("cfg.directCertsURL = %q, want %q", got, want)
	}
	if got, want := cfg.pubsubCertsURL, server.PubSubCertsURL(); got != want {
		t.Fatalf("cfg.pubsubCertsURL = %q, want %q", got, want)
	}
	waitForCondition(t, func() bool {
		return runtime.http.Address() != ""
	})
	if got, want := cfg.webhookPath, "/custom/gchat"; got != want {
		t.Fatalf("cfg.webhookPath = %q, want %q", got, want)
	}
	if got, want := cfg.mode, gchatModeHybrid; got != want {
		t.Fatalf("cfg.mode = %q, want %q", got, want)
	}
	if cfg.batcher == nil {
		t.Fatal("cfg.batcher = nil, want configured batcher")
	}
	defer cfg.batcher.Close()
	if _, ok := cfg.allowUsernames["alice@example.com"]; !ok {
		t.Fatalf("cfg.allowUsernames = %#v, want alice@example.com", cfg.allowUsernames)
	}

	status, degradation, err := provider.determineInitialState(context.Background(), &cfg)
	if err != nil {
		t.Fatalf("determineInitialState(ready) error = %v", err)
	}
	if got, want := status, bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("determineInitialState(ready) status = %q, want %q", got, want)
	}
	if degradation != nil {
		t.Fatalf("determineInitialState(ready) degradation = %#v, want nil", degradation)
	}

	degradedStatus, degraded, err := provider.determineInitialState(context.Background(), &resolvedInstanceConfig{
		configError: errors.New("bad config"),
	})
	if err == nil {
		t.Fatal("determineInitialState(configError) error = nil, want non-nil")
	}
	if got, want := degradedStatus, bridgepkg.BridgeStatusDegraded; got != want {
		t.Fatalf("determineInitialState(configError) status = %q, want %q", got, want)
	}
	if degraded == nil || degraded.Reason != bridgepkg.BridgeDegradationReasonTenantConfigInvalid {
		t.Fatalf("determineInitialState(configError) degradation = %#v, want tenant config invalid", degraded)
	}

	authStatus, authDegradation, err := provider.determineInitialState(context.Background(), &resolvedInstanceConfig{})
	if err == nil {
		t.Fatal("determineInitialState(missing creds) error = nil, want non-nil")
	}
	if got, want := authStatus, bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("determineInitialState(missing creds) status = %q, want %q", got, want)
	}
	if authDegradation == nil || authDegradation.Reason != bridgepkg.BridgeDegradationReasonAuthFailed {
		t.Fatalf("determineInitialState(missing creds) degradation = %#v, want auth failed", authDegradation)
	}

	managedMissingProject := testBridgeRuntime(t, now, "brg-missing-project")
	managedMissingProject.BoundSecrets = []subprocess.InitializeBridgeBoundSecret{
		{BindingName: "credentials_json", Kind: "json", Value: testCredentialsJSON(t)},
	}
	cfgMissingProject := provider.resolveInstanceConfig(session, managedMissingProject)
	if cfgMissingProject.configError == nil ||
		!strings.Contains(cfgMissingProject.configError.Error(), "project_number") {
		t.Fatalf(
			"resolveInstanceConfig(missing project) configError = %v, want project_number error",
			cfgMissingProject.configError,
		)
	}

	authProvider, err := newGChatProvider(io.Discard)
	if err != nil {
		t.Fatalf("newGChatProvider(authProvider) error = %v", err)
	}
	authProvider.apiFactory = func(*resolvedInstanceConfig) gchatAPI {
		return authFailingGChatAPI{}
	}
	authRequiredStatus, authRequiredDegradation, err := authProvider.determineInitialState(context.Background(), &cfg)
	if err == nil {
		t.Fatal("determineInitialState(auth failure) error = nil, want non-nil")
	}
	if got, want := authRequiredStatus, bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("determineInitialState(auth failure) status = %q, want %q", got, want)
	}
	if authRequiredDegradation == nil || authRequiredDegradation.Reason != bridgepkg.BridgeDegradationReasonAuthFailed {
		t.Fatalf("determineInitialState(auth failure) degradation = %#v, want auth failed", authRequiredDegradation)
	}

	rateLimitProvider, err := newGChatProvider(io.Discard)
	if err != nil {
		t.Fatalf("newGChatProvider(rateLimitProvider) error = %v", err)
	}
	rateLimitProvider.apiFactory = func(*resolvedInstanceConfig) gchatAPI {
		return rateLimitFailingGChatAPI{}
	}
	rateLimitedStatus, rateLimitedDegradation, err := rateLimitProvider.determineInitialState(
		context.Background(),
		&cfg,
	)
	if err == nil {
		t.Fatal("determineInitialState(rate limit) error = nil, want non-nil")
	}
	if got, want := rateLimitedStatus, bridgepkg.BridgeStatusDegraded; got != want {
		t.Fatalf("determineInitialState(rate limit) status = %q, want %q", got, want)
	}
	if rateLimitedDegradation == nil || rateLimitedDegradation.Reason != bridgepkg.BridgeDegradationReasonRateLimited {
		t.Fatalf("determineInitialState(rate limit) degradation = %#v, want rate limited", rateLimitedDegradation)
	}

	managedUnsupportedMode := testBridgeRuntime(t, now, "brg-unsupported-mode")
	managedUnsupportedMode.Instance.ProviderConfig = mustJSON(t, map[string]any{"mode": "weird"})
	cfgUnsupportedMode := provider.resolveInstanceConfig(session, managedUnsupportedMode)
	if cfgUnsupportedMode.configError == nil ||
		!strings.Contains(cfgUnsupportedMode.configError.Error(), "unsupported") {
		t.Fatalf(
			"resolveInstanceConfig(unsupported mode) configError = %v, want unsupported mode",
			cfgUnsupportedMode.configError,
		)
	}

	managedPubSubMissingAudience := testBridgeRuntime(t, now, "brg-pubsub-missing")
	managedPubSubMissingAudience.Instance.ProviderConfig = mustJSON(t, map[string]any{"mode": "pubsub"})
	cfgPubSubMissingAudience := provider.resolveInstanceConfig(session, managedPubSubMissingAudience)
	if cfgPubSubMissingAudience.configError == nil ||
		!strings.Contains(cfgPubSubMissingAudience.configError.Error(), "pubsub_audience") {
		t.Fatalf(
			"resolveInstanceConfig(pubsub missing audience) configError = %v, want pubsub_audience error",
			cfgPubSubMissingAudience.configError,
		)
	}

	managedBadConfig := testBridgeRuntime(t, now, "brg-bad-config")
	managedBadConfig.Instance.ProviderConfig = []byte(`{`)
	cfgBadConfig := provider.resolveInstanceConfig(session, managedBadConfig)
	if cfgBadConfig.configError == nil ||
		!strings.Contains(cfgBadConfig.configError.Error(), "decode provider_config") {
		t.Fatalf(
			"resolveInstanceConfig(bad provider_config) configError = %v, want decode error",
			cfgBadConfig.configError,
		)
	}

	t.Setenv(gchatDirectCertsEnv, "")
	t.Setenv(gchatPubSubCertsEnv, "")

	managedBlockedCerts := testBridgeRuntime(t, now, "brg-blocked-certs")
	managedBlockedCerts.Instance.ProviderConfig = mustJSON(t, map[string]any{
		"mode": "pubsub",
		"verification": map[string]any{
			"pubsub_audience":              "https://example.test/pubsub",
			"pubsub_service_account_email": "push@example.iam.gserviceaccount.com",
			"pubsub_certs_url":             "https://evil.example/certs",
		},
	})
	cfgBlockedCerts := provider.resolveInstanceConfig(session, managedBlockedCerts)
	if cfgBlockedCerts.configError == nil ||
		!strings.Contains(cfgBlockedCerts.configError.Error(), "pubsub_certs_url") {
		t.Fatalf(
			"resolveInstanceConfig(blocked cert host) configError = %v, want pubsub_certs_url allowlist error",
			cfgBlockedCerts.configError,
		)
	}

	managedAllowedCerts := testBridgeRuntime(t, now, "brg-allowed-certs")
	managedAllowedCerts.Instance.ProviderConfig = mustJSON(t, map[string]any{
		"mode": "pubsub",
		"verification": map[string]any{
			"pubsub_audience":              "https://example.test/pubsub",
			"pubsub_service_account_email": "push@example.iam.gserviceaccount.com",
			"pubsub_certs_url":             gchatDefaultPubSubCertsURL,
		},
	})
	cfgAllowedCerts := provider.resolveInstanceConfig(session, managedAllowedCerts)
	if cfgAllowedCerts.configError != nil {
		t.Fatalf("resolveInstanceConfig(allowed cert host) configError = %v, want nil", cfgAllowedCerts.configError)
	}
	if got, want := cfgAllowedCerts.pubsubCertsURL, gchatDefaultPubSubCertsURL; got != want {
		t.Fatalf("cfgAllowedCerts.pubsubCertsURL = %q, want %q", got, want)
	}
}

func TestGChatTransportAndClassificationHelpers(t *testing.T) {
	t.Run(
		"Should use exact Google Chat transport contracts and classify failures",
		testGChatTransportAndClassificationHelpers,
	)
	t.Run("Should preserve auth classification when the error body read fails", func(t *testing.T) {
		t.Parallel()

		readErr := errors.New("Google Chat error body read failed")
		body := &gchatPartialErrorResponseBody{
			content: []byte("Google Chat access denied"),
			readErr: readErr,
		}
		client := &gchatBotClient{
			cfg: resolvedInstanceConfig{apiBaseURL: "https://chat.googleapis.com"},
			httpClient: &http.Client{Transport: gchatRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Header:     make(http.Header),
					Body:       body,
					Request:    req,
				}, nil
			})},
			cachedToken: "cached-token",
			tokenExpiry: time.Now().UTC().Add(time.Hour),
		}

		_, err := client.GetMessage(t.Context(), "spaces/AAA/messages/msg-auth")
		if !errors.Is(err, readErr) {
			t.Fatalf("GetMessage() error = %v, want response body read failure", err)
		}
		var authErr *bridgesdk.AuthError
		if !errors.As(err, &authErr) {
			t.Fatalf("GetMessage() error = %T %v, want AuthError", err, err)
		}
		if got, want := bridgesdk.ClassifyError(err).Class, bridgesdk.ErrorClassAuth; got != want {
			t.Fatalf("GetMessage() error class = %q, want %q", got, want)
		}
		if !strings.Contains(authErr.Error(), "Google Chat access denied") {
			t.Fatalf("GetMessage() auth error = %q, want partial provider body", authErr.Error())
		}
		if got, want := body.reads, 2; got != want {
			t.Fatalf("response body reads = %d, want %d for read plus deferred drain", got, want)
		}
		if !body.closed {
			t.Fatal("response body closed = false after error classification")
		}
	})
	t.Run("Should stop after a successful create returns truncated JSON", func(t *testing.T) {
		t.Parallel()

		attempts := 0
		var attemptsMu sync.Mutex
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attemptsMu.Lock()
			attempts++
			attemptsMu.Unlock()
			w.WriteHeader(http.StatusOK)
			if _, err := io.WriteString(w, `{"name":`); err != nil {
				t.Errorf("write Google Chat response: %v", err)
			}
		}))
		defer server.Close()

		client := &gchatBotClient{
			cfg:         resolvedInstanceConfig{apiBaseURL: server.URL},
			httpClient:  server.Client(),
			cachedToken: "cached-token",
			tokenExpiry: time.Now().UTC().Add(time.Hour),
		}
		_, err := createGChatDeliveryMessage(t.Context(), client, gchatCreateMessageRequest{
			SpaceName: "spaces/AAA",
			Text:      "hello",
		})
		if err == nil {
			t.Fatal("createGChatDeliveryMessage(truncated response) error = nil, want non-nil")
		}
		var committedErr *bridgesdk.CommittedMutationError
		if !errors.As(err, &committedErr) {
			t.Fatalf("createGChatDeliveryMessage() error = %T, want CommittedMutationError", err)
		}
		attemptsMu.Lock()
		gotAttempts := attempts
		attemptsMu.Unlock()
		if got, want := gotAttempts, 1; got != want {
			t.Fatalf("transport attempts = %d, want %d", got, want)
		}
	})
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
				if err := json.NewEncoder(w).Encode(gchatSentMessage{
					Name: "spaces/AAA/messages/msg-redirect",
				}); err != nil {
					t.Errorf("encode redirect target Google Chat response: %v", err)
				}
			}))
			defer target.Close()

			redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", target.URL)
				w.WriteHeader(testCase.statusCode)
				if _, err := io.WriteString(w, `{"error":{"message":"redirect refused"}}`); err != nil {
					t.Errorf("write redirect Google Chat response: %v", err)
				}
				if got, want := r.Method, http.MethodPost; got != want {
					t.Errorf("redirect source method = %q, want %q", got, want)
				}
			}))
			defer redirect.Close()

			client := &gchatBotClient{
				cfg:         resolvedInstanceConfig{apiBaseURL: redirect.URL},
				httpClient:  redirect.Client(),
				cachedToken: "cached-token",
				tokenExpiry: time.Now().UTC().Add(time.Hour),
			}
			_, err := client.CreateMessage(t.Context(), gchatCreateMessageRequest{
				SpaceName: "spaces/AAA",
				Text:      "hello",
			})
			var httpErr *bridgesdk.HTTPError
			if !errors.As(err, &httpErr) || httpErr.StatusCode != testCase.statusCode {
				t.Fatalf(
					"CreateMessage(%d) error = %T %v, want first-response HTTPError",
					testCase.statusCode,
					err,
					err,
				)
			}
			if bridgesdk.IsCommittedMutation(err) {
				t.Fatalf("CreateMessage(%d) error = %v, want pre-commit rejection", testCase.statusCode, err)
			}
			if got := targetHits.Load(); got != 0 {
				t.Fatalf("redirect target hits = %d, want 0", got)
			}
		})
	}
	t.Run("Should reject trailing JSON in an OAuth token response", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if _, err := io.WriteString(w, `{"access_token":"gchat-token","expires_in":3600} {}`); err != nil {
				t.Errorf("write Google Chat token response: %v", err)
			}
		}))
		defer server.Close()

		client := &gchatBotClient{cfg: resolvedInstanceConfig{
			tokenURL:    server.URL,
			credentials: mustCredentials(t),
		}}
		if _, err := client.accessToken(t.Context()); err == nil {
			t.Fatal("accessToken(trailing JSON) error = nil, want strict decode failure")
		}
	})
}

func testGChatTransportAndClassificationHelpers(t *testing.T) {
	t.Helper()
	t.Parallel()

	server := newGChatProviderTestServer(t)
	defer server.Close()

	credentials := mustCredentials(t)
	client := &gchatBotClient{cfg: resolvedInstanceConfig{
		apiBaseURL:  server.URL(),
		tokenURL:    server.TokenURL(),
		credentials: credentials,
	}}

	if err := client.ValidateAuth(context.Background()); err != nil {
		t.Fatalf("ValidateAuth() error = %v", err)
	}
	created, err := client.CreateMessage(context.Background(), gchatCreateMessageRequest{
		SpaceName:  "spaces/AAA",
		ThreadName: "spaces/AAA/threads/thread-created",
		Text:       "hello",
	})
	if err != nil {
		t.Fatalf("CreateMessage() error = %v", err)
	}
	if strings.TrimSpace(created.Name) == "" {
		t.Fatal("CreateMessage() name = empty, want remote message id")
	}
	if _, err := client.UpdateMessage(context.Background(), gchatUpdateMessageRequest{
		MessageName: created.Name,
		Text:        "updated",
	}); err != nil {
		t.Fatalf("UpdateMessage() error = %v", err)
	}
	updatePath := "/v1/" + created.Name
	if !slices.ContainsFunc(server.Calls(), func(call gchatAPICall) bool {
		return call.Method == http.MethodPatch &&
			call.Path == updatePath &&
			stringValue(call.Body[providerTextKey]) == "updated"
	}) {
		t.Fatalf("Google Chat API calls = %#v, want PATCH %s with updated text", server.Calls(), updatePath)
	}
	if err := client.DeleteMessage(context.Background(), created.Name); err != nil {
		t.Fatalf("DeleteMessage() error = %v", err)
	}
	if _, err := client.GetMessage(context.Background(), "spaces/AAA/messages/msg-react"); err != nil {
		t.Fatalf("GetMessage() error = %v", err)
	}
	if err := client.callJSON(
		context.Background(),
		http.MethodGet,
		"/v1/missing",
		nil,
		nil,
		&map[string]any{},
		bridgesdk.HTTPResponseNoCommit,
	); err == nil {
		t.Fatal("callJSON(missing) error = nil, want non-nil")
	}

	if got, want := parseRetryAfter("7"), 7*time.Second; got != want {
		t.Fatalf("parseRetryAfter() = %s, want %s", got, want)
	}
	if got := parseRetryAfter("nope"); got != 0 {
		t.Fatalf("parseRetryAfter(nope) = %s, want 0", got)
	}

	if _, ok := classifyGChatHTTPError(http.StatusUnauthorized, "", `{"error":{"message":"denied"}}`).(*bridgesdk.AuthError); !ok {
		t.Fatalf(
			"classifyGChatHTTPError(401) = %T, want *bridgesdk.AuthError",
			classifyGChatHTTPError(http.StatusUnauthorized, "", `{"error":{"message":"denied"}}`),
		)
	}
	if rateErr, ok := classifyGChatHTTPError(http.StatusTooManyRequests, "9", "").(*bridgesdk.RateLimitError); !ok ||
		rateErr.RetryAfter != 9*time.Second {
		t.Fatalf(
			"classifyGChatHTTPError(429) = %#v, want rate limit with retry-after 9s",
			classifyGChatHTTPError(http.StatusTooManyRequests, "9", ""),
		)
	}
	if serverErr, ok := classifyGChatHTTPError(http.StatusServiceUnavailable, "", "").(*bridgesdk.HTTPError); !ok ||
		bridgesdk.ClassifyError(serverErr).Class != bridgesdk.ErrorClassServerError {
		t.Fatalf(
			"classifyGChatHTTPError(503) = %#v, want server_error HTTPError",
			classifyGChatHTTPError(http.StatusServiceUnavailable, "", ""),
		)
	}
	if httpErr, ok := classifyGChatHTTPError(http.StatusTeapot, "", "").(*bridgesdk.HTTPError); !ok ||
		httpErr.StatusCode != http.StatusTeapot {
		t.Fatalf(
			"classifyGChatHTTPError(418) = %#v, want HTTP 418 error",
			classifyGChatHTTPError(http.StatusTeapot, "", ""),
		)
	}

	if got, want := normalizeWebhookPath("gchat/test"), "/gchat/test"; got != want {
		t.Fatalf("normalizeWebhookPath() = %q, want %q", got, want)
	}
	if got, want := normalizeURL(" https://example.test/path/ "), "https://example.test/path"; got != want {
		t.Fatalf("normalizeURL() = %q, want %q", got, want)
	}
	if got := buildIdentitySet([]string{" Alice ", "@ALICE", ""}); len(got) != 1 {
		t.Fatalf("buildIdentitySet() = %#v, want single normalized entry", got)
	}
	if !issuerMatches("https://accounts.google.com", "accounts.google.com", "https://accounts.google.com") {
		t.Fatal("issuerMatches() = false, want true")
	}
}

func TestGChatWebhookHandlersUseRequestContext(t *testing.T) {
	server := newGChatProviderTestServer(t)
	defer server.Close()

	now := time.Date(2026, 4, 15, 21, 45, 0, 0, time.UTC)
	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	managed := testBridgeRuntime(t, now, "brg-gchat-ctx")
	managed.Instance.ProviderConfig = mustJSON(t, map[string]any{
		"mode": "hybrid",
		"verification": map[string]any{
			"pubsub_audience":              "https://example.test/pubsub",
			"pubsub_service_account_email": "push@example.iam.gserviceaccount.com",
		},
	})

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
			instance.Degradation = payload.Degradation
			return instance, nil
		},
	)
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
		func(ctx context.Context, _ json.RawMessage) (any, error) {
			if !errors.Is(ctx.Err(), context.Canceled) {
				t.Fatalf("bridges/messages/ingest ctx.Err() = %v, want context.Canceled", ctx.Err())
			}
			return nil, context.Canceled
		},
	)

	t.Setenv(gchatListenAddrEnv, reserveListenAddr(t))
	t.Setenv(gchatAPIBaseEnv, server.URL())
	t.Setenv(gchatTokenURLEnv, server.TokenURL())
	t.Setenv(gchatDirectCertsEnv, server.DirectCertsURL())
	t.Setenv(gchatPubSubCertsEnv, server.PubSubCertsURL())
	runtime.apiFactory = func(cfg *resolvedInstanceConfig) gchatAPI {
		client := &gchatBotClient{
			cfg: *cfg,
			httpClient: &http.Client{
				Timeout: 10 * time.Second,
			},
		}
		return &contextCheckingGChatAPI{
			t:            t,
			validateAuth: client.ValidateAuth,
			message: gchatMessage{
				Name:  "spaces/AAA/messages/msg-react",
				Space: &gchatSpace{Name: "spaces/AAA", Type: "SPACE"},
			},
		}
	}

	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	cfg, err := runtime.waitForInstanceConfig("brg-gchat-ctx", time.Second)
	if err != nil {
		t.Fatalf("waitForInstanceConfig() error = %v", err)
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	recorder := httptest.NewRecorder()
	err = runtime.handleDirectWebhook(
		canceledCtx,
		recorder,
		&cfg,
		bridgesdk.WebhookRequest{Body: []byte(directWebhookPayload()), ReceivedAt: now},
	)
	var httpErr *bridgesdk.HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("handleDirectWebhook(canceled context) error = %v, want HTTP 500", err)
	}

	recorder = httptest.NewRecorder()
	err = runtime.handlePubSubWebhook(
		canceledCtx,
		recorder,
		&cfg,
		bridgesdk.WebhookRequest{Body: []byte(pubSubReactionPayload(t)), ReceivedAt: now},
	)
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("handlePubSubWebhook(canceled context) error = %v, want HTTP 500", err)
	}
}

func TestGoogleX509KeyCacheReusesFreshKeysAndFallsBackToStaleEntries(t *testing.T) {
	server := newGChatProviderTestServer(t)
	url := server.DirectCertsURL()
	cache := newGoogleX509KeyCache(&http.Client{Timeout: time.Second}, time.Minute, time.Now)

	first, err := cache.fetch(context.Background(), url)
	if err != nil {
		t.Fatalf("cache.fetch(first) error = %v", err)
	}
	second, err := cache.fetch(context.Background(), url)
	if err != nil {
		t.Fatalf("cache.fetch(second) error = %v", err)
	}
	if len(first) == 0 || len(second) == 0 {
		t.Fatal("cache.fetch() returned no keys, want cached keys")
	}
	if got, want := server.DirectCertHits(), 1; got != want {
		t.Fatalf("DirectCertHits() = %d, want %d after cache reuse", got, want)
	}

	cache.entries[url] = googleX509KeyCacheEntry{
		keys:      first,
		expiresAt: time.Now().Add(-time.Second),
	}
	server.Close()

	stale, err := cache.fetch(context.Background(), url)
	if err != nil {
		t.Fatalf("cache.fetch(stale fallback) error = %v", err)
	}
	if len(stale) == 0 {
		t.Fatal("cache.fetch(stale fallback) = empty, want cached keys")
	}
}

func TestGoogleX509KeyCacheUsesBoundedClientTimeout(t *testing.T) {
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		if err := json.NewEncoder(w).Encode(map[string]string{"kid": "bad"}); err != nil {
			t.Errorf("encode slow X.509 response: %v", err)
		}
	}))
	defer slowServer.Close()

	cache := newGoogleX509KeyCache(&http.Client{Timeout: 20 * time.Millisecond}, time.Minute, time.Now)
	if _, err := cache.fetch(context.Background(), slowServer.URL); err == nil {
		t.Fatal("cache.fetch(timeout) error = nil, want non-nil")
	}
}

func TestGChatPayloadAndRoutingHelpers(t *testing.T) {
	t.Parallel()

	if got, want := detectGChatWebhookShape([]byte(`{"chat":{}}`)), gchatModeDirect; got != want {
		t.Fatalf("detectGChatWebhookShape(direct) = %q, want %q", got, want)
	}
	if got, want := detectGChatWebhookShape(
		[]byte(`{"subscription":"sub","message":{"data":"e30="}}`),
	), gchatModePubSub; got != want {
		t.Fatalf("detectGChatWebhookShape(pubsub) = %q, want %q", got, want)
	}
	if got := detectGChatWebhookShape([]byte(`{"invalid":true}`)); got != "" {
		t.Fatalf("detectGChatWebhookShape(invalid) = %q, want empty", got)
	}

	decoded, err := decodePubSubMessage(gchatPubSubPushMessage{
		Subscription: "sub",
		Message: gchatPubSubInner{
			Data: base64.StdEncoding.EncodeToString([]byte(`{"message":{"name":"spaces/AAA/messages/msg-1"}}`)),
			Attributes: map[string]string{
				"ce-type":    "google.workspace.chat.message.v1.created",
				"ce-subject": "//chat.googleapis.com/spaces/AAA",
				"ce-time":    "2026-04-15T20:00:00Z",
			},
		},
	})
	if err != nil {
		t.Fatalf("decodePubSubMessage(valid) error = %v", err)
	}
	if got, want := decoded.EventType, "google.workspace.chat.message.v1.created"; got != want {
		t.Fatalf("decoded.EventType = %q, want %q", got, want)
	}
	if _, err := decodePubSubMessage(gchatPubSubPushMessage{Message: gchatPubSubInner{Data: "%%%"}}); err == nil {
		t.Fatal("decodePubSubMessage(invalid base64) error = nil, want non-nil")
	}

	threadID := encodeGChatThreadID(gchatThreadRef{SpaceName: "spaces/AAA", ThreadName: "spaces/AAA/threads/thread-1"})
	target, err := resolveGChatDeliveryTarget(bridgepkg.DeliveryEvent{
		DeliveryTarget: bridgepkg.DeliveryTarget{ThreadID: threadID},
	})
	if err != nil {
		t.Fatalf("resolveGChatDeliveryTarget(thread) error = %v", err)
	}
	if got, want := target.SpaceName, "spaces/AAA"; got != want {
		t.Fatalf("resolveGChatDeliveryTarget(thread) space = %q, want %q", got, want)
	}
	if got, want := target.ThreadName, "spaces/AAA/threads/thread-1"; got != want {
		t.Fatalf("resolveGChatDeliveryTarget(thread) name = %q, want %q", got, want)
	}
	target, err = resolveGChatDeliveryTarget(bridgepkg.DeliveryEvent{
		DeliveryTarget: bridgepkg.DeliveryTarget{GroupID: "spaces/FALLBACK"},
	})
	if err != nil {
		t.Fatalf("resolveGChatDeliveryTarget(group) error = %v", err)
	}
	if got, want := target.SpaceName, "spaces/FALLBACK"; got != want {
		t.Fatalf("resolveGChatDeliveryTarget(group) space = %q, want %q", got, want)
	}
	if _, err := resolveGChatDeliveryTarget(bridgepkg.DeliveryEvent{}); err == nil {
		t.Fatal("resolveGChatDeliveryTarget(empty) error = nil, want non-nil")
	}

	server := newGChatProviderTestServer(t)
	defer server.Close()
	directReq := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.test/gchat",
		strings.NewReader(`{"chat":{}}`),
	)
	directReq.Header.Set("Authorization", "Bearer "+server.signDirectToken(t, "123456789"))
	if err := verifyGChatWebhookBearer(context.Background(), directReq, []byte(`{"chat":{}}`), &resolvedInstanceConfig{
		mode:           gchatModeDirect,
		projectNumber:  "123456789",
		directIssuer:   gchatDefaultDirectIssuer,
		directCertsURL: server.DirectCertsURL(),
	}); err != nil {
		t.Fatalf("verifyGChatWebhookBearer(direct) error = %v", err)
	}

	pubsubReq := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.test/gchat",
		strings.NewReader(`{"subscription":"sub","message":{"data":"e30="}}`),
	)
	pubsubReq.Header.Set(
		"Authorization",
		"Bearer "+server.signPubSubToken(
			t,
			"https://example.test/pubsub",
			"push@example.iam.gserviceaccount.com",
			true,
		),
	)
	if err := verifyGChatWebhookBearer(
		context.Background(),
		pubsubReq,
		[]byte(`{"subscription":"sub","message":{"data":"e30="}}`),
		&resolvedInstanceConfig{
			mode:                      gchatModePubSub,
			pubsubAudience:            "https://example.test/pubsub",
			pubsubIssuer:              gchatDefaultPubSubIssuerURL,
			pubsubCertsURL:            server.PubSubCertsURL(),
			pubsubServiceAccountEmail: "push@example.iam.gserviceaccount.com",
		},
	); err != nil {
		t.Fatalf("verifyGChatWebhookBearer(pubsub) error = %v", err)
	}

	if err := verifyGChatWebhookBearer(
		context.Background(),
		httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"http://example.test/gchat",
			strings.NewReader(`{"bad":true}`),
		),
		[]byte(`{"bad":true}`),
		&resolvedInstanceConfig{mode: gchatModeHybrid},
	); err == nil {
		t.Fatal("verifyGChatWebhookBearer(invalid) error = nil, want non-nil")
	}

	if got, want := normalizeReceivedAt(
		time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		"2026-04-15T01:02:03Z",
	), time.Date(
		2026,
		4,
		15,
		1,
		2,
		3,
		0,
		time.UTC,
	); !got.Equal(
		want,
	) {
		t.Fatalf("normalizeReceivedAt(parsed) = %s, want %s", got, want)
	}
}

func TestGChatWebhookAndBatchErrorPaths(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 20, 25, 0, 0, time.UTC)
	server := newGChatProviderTestServer(t)
	defer server.Close()

	provider, err := newGChatProvider(io.Discard)
	if err != nil {
		t.Fatalf("newGChatProvider() error = %v", err)
	}
	provider.apiFactory = func(*resolvedInstanceConfig) gchatAPI {
		return &fakeGChatAPI{
			messagesMap: map[string]gchatMessage{
				"spaces/AAA/messages/msg-react": {
					Name:   "spaces/AAA/messages/msg-react",
					Space:  &gchatSpace{Name: "spaces/AAA", Type: "SPACE"},
					Thread: &gchatThread{Name: "spaces/AAA/threads/thread-react"},
				},
			},
		}
	}

	cfg := resolvedInstanceConfig{
		managed:                   testBridgeRuntime(t, now, "brg-gchat"),
		instanceID:                "brg-gchat",
		mode:                      gchatModeHybrid,
		projectNumber:             "123456789",
		directIssuer:              gchatDefaultDirectIssuer,
		directCertsURL:            server.DirectCertsURL(),
		pubsubAudience:            "https://example.test/pubsub",
		pubsubIssuer:              gchatDefaultPubSubIssuerURL,
		pubsubCertsURL:            server.PubSubCertsURL(),
		pubsubServiceAccountEmail: "push@example.iam.gserviceaccount.com",
		dedup:                     bridgesdk.NewDedupCache(5*time.Minute, 100),
		rateLimiter:               bridgesdk.NewFixedWindowRateLimiter(10, time.Minute),
		inFlightLimiter:           bridgesdk.NewInFlightLimiter(2),
	}

	recorder := httptest.NewRecorder()
	err = provider.handleWebhookRequest(
		recorder,
		httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"http://example.test/gchat",
			strings.NewReader(`{"bad":true}`),
		),
		&cfg,
		bridgesdk.WebhookRequest{Body: []byte(`{"bad":true}`), ReceivedAt: now},
	)
	var httpErr *bridgesdk.HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("handleWebhookRequest(invalid) error = %v, want HTTP 400", err)
	}

	recorder = httptest.NewRecorder()
	err = provider.handleDirectWebhook(
		context.Background(),
		recorder,
		&cfg,
		bridgesdk.WebhookRequest{Body: []byte(`{`), ReceivedAt: now},
	)
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("handleDirectWebhook(invalid json) error = %v, want HTTP 400", err)
	}

	recorder = httptest.NewRecorder()
	err = provider.handleDirectWebhook(
		context.Background(),
		recorder,
		&cfg,
		bridgesdk.WebhookRequest{Body: []byte(`{"chat":null}`), ReceivedAt: now},
	)
	if err != nil {
		t.Fatalf("handleDirectWebhook(nil chat) error = %v", err)
	}
	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("handleDirectWebhook(nil chat) status = %d, want %d", got, want)
	}

	actionPayload := mustJSON(t, map[string]any{
		"chat": map[string]any{
			"buttonClickedPayload": map[string]any{
				"space": map[string]any{
					"name": "spaces/AAA",
					"type": "SPACE",
				},
				"message": map[string]any{
					"name": "spaces/AAA/messages/msg-action",
					"thread": map[string]any{
						"name": "spaces/AAA/threads/thread-action",
					},
				},
				"user": map[string]any{
					"name":        "users/123",
					"displayName": "Alice",
				},
			},
		},
		"commonEventObject": map[string]any{
			"parameters": map[string]string{
				"actionId": "approve",
				"value":    "yes",
			},
		},
	})
	recorder = httptest.NewRecorder()
	err = provider.handleDirectWebhook(
		context.Background(),
		recorder,
		&cfg,
		bridgesdk.WebhookRequest{Body: actionPayload, ReceivedAt: now},
	)
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("handleDirectWebhook(uninitialized session) error = %v, want HTTP 500", err)
	}

	recorder = httptest.NewRecorder()
	err = provider.handlePubSubWebhook(
		context.Background(),
		recorder,
		&cfg,
		bridgesdk.WebhookRequest{Body: []byte(`{"message":{"data":"%%%"}}`), ReceivedAt: now},
	)
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("handlePubSubWebhook(invalid payload) error = %v, want HTTP 400", err)
	}

	recorder = httptest.NewRecorder()
	err = provider.handlePubSubWebhook(
		context.Background(),
		recorder,
		&cfg,
		bridgesdk.WebhookRequest{Body: []byte(pubSubReactionPayload(t)), ReceivedAt: now},
	)
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("handlePubSubWebhook(uninitialized session) error = %v, want HTTP 500", err)
	}

	batchErr := provider.dispatchInboundBatch(context.Background(), "brg-gchat", bridgesdk.InboundBatch{
		Items: []bridgepkg.InboundMessageEnvelope{
			{
				BridgeInstanceID: "brg-gchat",
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-gchat",
				Content:          bridgepkg.MessageContent{Text: "hello"},
				Attachments:      []bridgepkg.MessageAttachment{{Name: "file-1"}},
				IdempotencyKey:   "item-1",
			},
			{
				BridgeInstanceID: "brg-gchat",
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-gchat",
				Content:          bridgepkg.MessageContent{Text: "world"},
				Attachments:      []bridgepkg.MessageAttachment{{Name: "file-2"}},
				IdempotencyKey:   "item-2",
			},
		},
	})
	if batchErr == nil || !strings.Contains(batchErr.Error(), "not initialized") {
		t.Fatalf("dispatchInboundBatch() error = %v, want runtime session is not initialized", batchErr)
	}
}

func TestRunRejectsUnsupportedCommands(t *testing.T) {
	t.Parallel()

	err := run([]string{"nope"}, strings.NewReader(""), io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), `unsupported command "nope"`) {
		t.Fatalf("run(unsupported) error = %v, want unsupported command", err)
	}
}

func TestGChatSmallHelperBranches(t *testing.T) {
	t.Parallel()

	if _, err := parseRSAPrivateKey("not-a-key"); err == nil {
		t.Fatal("parseRSAPrivateKey(invalid) error = nil, want non-nil")
	}

	fallback := time.Date(2026, 4, 15, 3, 4, 5, 0, time.UTC)
	if got := normalizeReceivedAt(fallback, "not-a-time"); !got.Equal(fallback) {
		t.Fatalf("normalizeReceivedAt(invalid) = %s, want fallback %s", got, fallback)
	}

	attachments := normalizeGChatAttachments([]gchatAttachment{{
		Name:        "attachments/1",
		ContentName: "report.txt",
		ContentType: "text/plain",
		DownloadURI: "https://example.test/report.txt",
	}})
	if got, want := len(attachments), 1; got != want {
		t.Fatalf("len(normalizeGChatAttachments()) = %d, want %d", got, want)
	}
	if got, want := attachments[0].Name, "report.txt"; got != want {
		t.Fatalf("normalizeGChatAttachments()[0].Name = %q, want %q", got, want)
	}
}

func TestHandleBridgesDeliverRejectsUnknownInstance(t *testing.T) {
	deliveryPath := filepath.Join(t.TempDir(), "delivery.jsonl")
	t.Setenv(bridgesdk.AdapterDeliveryPathEnv, deliveryPath)
	provider, err := newGChatProvider(io.Discard)
	if err != nil {
		t.Fatalf("newGChatProvider() error = %v", err)
	}

	_, err = provider.handleBridgesDeliver(
		context.Background(),
		nil,
		testDeliveryRequest("missing", "delivery-1", 1, bridgepkg.DeliveryEventTypeStart, false),
	)
	if err == nil || !strings.Contains(err.Error(), "unmanaged instance") {
		t.Fatalf("handleBridgesDeliver() error = %v, want unmanaged instance error", err)
	}
	payload, readErr := os.ReadFile(deliveryPath)
	if readErr != nil {
		t.Fatalf("os.ReadFile(delivery marker) error = %v", readErr)
	}
	if !strings.Contains(string(payload), "unmanaged instance") {
		t.Fatalf("delivery marker = %q, want unmanaged instance error", string(payload))
	}
}

func TestHandleBridgesDeliverRejectsConfigError(t *testing.T) {
	deliveryPath := filepath.Join(t.TempDir(), "delivery.jsonl")
	t.Setenv(bridgesdk.AdapterDeliveryPathEnv, deliveryPath)
	provider, err := newGChatProvider(io.Discard)
	if err != nil {
		t.Fatalf("newGChatProvider() error = %v", err)
	}
	setGChatProviderRoute(provider, "brg-gchat", &resolvedInstanceConfig{
		instanceID: "brg-gchat",
		configError: errors.New(
			"gchat: provider_config.verification.direct_certs_url host \"example.test\" is not allowed",
		),
	})

	_, err = provider.handleBridgesDeliver(
		context.Background(),
		nil,
		testDeliveryRequest("brg-gchat", "delivery-1", 1, bridgepkg.DeliveryEventTypeStart, false),
	)
	if err == nil || !strings.Contains(err.Error(), "is not allowed") {
		t.Fatalf("handleBridgesDeliver() error = %v, want configError", err)
	}
	payload, readErr := os.ReadFile(deliveryPath)
	if readErr != nil {
		t.Fatalf("os.ReadFile(delivery marker) error = %v", readErr)
	}
	if !strings.Contains(string(payload), "is not allowed") {
		t.Fatalf("delivery marker = %q, want configError", string(payload))
	}
}

func TestReconcileInstanceConfigsDetectsSharedWebhookPaths(t *testing.T) {
	now := time.Date(2026, 4, 15, 20, 28, 0, 0, time.UTC)
	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	seed := testBridgeRuntime(t, now, "seed")
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesList),
		func(context.Context, json.RawMessage) (any, error) {
			return []bridgepkg.BridgeInstance{seed.Instance}, nil
		},
	)
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
		func(context.Context, json.RawMessage) (any, error) {
			return seed.Instance, nil
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
			instance := seed.Instance
			instance.Status = payload.Status
			instance.Degradation = payload.Degradation
			return instance, nil
		},
	)
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
		func(context.Context, json.RawMessage) (any, error) {
			return bridgepkg.BridgesMessagesIngestResult{SessionID: "sess-1"}, nil
		},
	)

	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, seed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	session := runtime.currentSession()
	if session == nil {
		t.Fatal("runtime.currentSession() = nil, want non-nil")
	}

	provider, err := newGChatProvider(io.Discard)
	if err != nil {
		t.Fatalf("newGChatProvider() error = %v", err)
	}
	defer provider.lifecycle.Stop()
	provider.apiFactory = func(*resolvedInstanceConfig) gchatAPI { return &fakeGChatAPI{} }

	first := testBridgeRuntime(t, now, "brg-one")
	first.Instance.ProviderConfig = mustJSON(t, map[string]any{
		"mode": "pubsub",
		"webhook": map[string]any{
			"path": "/shared",
		},
		"verification": map[string]any{
			"pubsub_audience":              "https://example.test/pubsub",
			"pubsub_service_account_email": "push@example.iam.gserviceaccount.com",
		},
	})
	second := testBridgeRuntime(t, now, "brg-two")
	second.Instance.ProviderConfig = mustJSON(t, map[string]any{
		"mode": "pubsub",
		"webhook": map[string]any{
			"path": "/shared",
		},
		"verification": map[string]any{
			"pubsub_audience":              "https://example.test/pubsub",
			"pubsub_service_account_email": "push@example.iam.gserviceaccount.com",
		},
	})

	configs, _ := prepareGChatConfigConstraints([]resolvedInstanceConfig{
		provider.resolveInstanceConfig(session, first),
		provider.resolveInstanceConfig(session, second),
	})
	if got, want := len(configs), 2; got != want {
		t.Fatalf("len(configs) = %d, want %d", got, want)
	}
	if configs[0].configError == nil || !strings.Contains(configs[0].configError.Error(), "shared") {
		t.Fatalf("configs[0].configError = %v, want shared webhook path error", configs[0].configError)
	}
	if configs[1].configError == nil || !strings.Contains(configs[1].configError.Error(), "shared") {
		t.Fatalf("configs[1].configError = %v, want shared webhook path error", configs[1].configError)
	}
	provider.routes.Replace(map[string]resolvedInstanceConfig{
		configs[0].instanceID: configs[0],
		configs[1].instanceID: configs[1],
	}, nil)
	if _, ok := provider.configForPath("/shared"); ok {
		t.Fatal("configForPath(/shared) = ok, want conflicted path rejected")
	}
	recorder := httptest.NewRecorder()
	provider.serveWebhookHTTP(
		recorder,
		httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"http://example.test/shared",
			strings.NewReader("{}"),
		),
	)
	if got, want := recorder.Code, http.StatusNotFound; got != want {
		t.Fatalf("serveWebhookHTTP() status = %d, want %d", got, want)
	}
	if got := recorder.Body.String(); !strings.Contains(got, "404 page not found") {
		t.Fatalf("serveWebhookHTTP() body = %q, want 404 response", got)
	}

	firstWithListen := testBridgeRuntime(t, now, "brg-one-listen")
	firstWithListen.Instance.ProviderConfig = mustJSON(t, map[string]any{
		"mode": "pubsub",
		"webhook": map[string]any{
			"listen_addr": "127.0.0.1:21231",
			"path":        "/first",
		},
		"verification": map[string]any{
			"pubsub_audience":              "https://example.test/pubsub",
			"pubsub_service_account_email": "push@example.iam.gserviceaccount.com",
		},
	})
	third := testBridgeRuntime(t, now, "brg-three")
	third.Instance.ProviderConfig = mustJSON(t, map[string]any{
		"mode": "pubsub",
		"webhook": map[string]any{
			"listen_addr": "127.0.0.1:21232",
			"path":        "/unique",
		},
		"verification": map[string]any{
			"pubsub_audience":              "https://example.test/pubsub",
			"pubsub_service_account_email": "push@example.iam.gserviceaccount.com",
		},
	})
	configs, requestedListen := prepareGChatConfigConstraints([]resolvedInstanceConfig{
		provider.resolveInstanceConfig(session, firstWithListen),
		provider.resolveInstanceConfig(session, third),
	})
	if got, want := requestedListen, "127.0.0.1:21231"; got != want {
		t.Fatalf("requestedListen = %q, want %q", got, want)
	}
	if configs[1].configError == nil || !strings.Contains(configs[1].configError.Error(), "incompatible listen_addr") {
		t.Fatalf("configs[1].configError = %v, want incompatible listen_addr error", configs[1].configError)
	}

	empty := provider.reconcileInstanceConfigs(context.Background(), session, nil)
	if len(empty) != 0 {
		t.Fatalf("reconcileInstanceConfigs(nil) len = %d, want 0", len(empty))
	}
}

type fakeGChatAPI struct {
	messages               []gchatCreateMessageRequest
	createAttempts         []gchatCreateMessageRequest
	createErrorText        string
	createErr              error
	createEmptyMessageName bool
	updates                []gchatUpdateMessageRequest
	updateErrorText        string
	updateErr              error
	deletes                []string
	fetched                []string
	store                  []gchatSentMessage
	mu                     sync.Mutex
	messagesMap            map[string]gchatMessage
}

type gchatRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f gchatRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type gchatPartialErrorResponseBody struct {
	content []byte
	readErr error
	reads   int
	closed  bool
}

func (b *gchatPartialErrorResponseBody) Read(buffer []byte) (int, error) {
	b.reads++
	if b.reads > 1 {
		return 0, io.EOF
	}
	return copy(buffer, b.content), b.readErr
}

func (b *gchatPartialErrorResponseBody) Close() error {
	b.closed = true
	return nil
}

type authFailingGChatAPI struct{}

func (authFailingGChatAPI) ValidateAuth(context.Context) error {
	return &bridgesdk.AuthError{Err: errors.New("bad token")}
}
func (authFailingGChatAPI) CreateMessage(context.Context, gchatCreateMessageRequest) (*gchatSentMessage, error) {
	return nil, errors.New("unsupported")
}
func (authFailingGChatAPI) UpdateMessage(context.Context, gchatUpdateMessageRequest) (*gchatSentMessage, error) {
	return nil, errors.New("unsupported")
}
func (authFailingGChatAPI) DeleteMessage(context.Context, string) error {
	return errors.New("unsupported")
}
func (authFailingGChatAPI) GetMessage(context.Context, string) (*gchatMessage, error) {
	return nil, errors.New("unsupported")
}

type rateLimitFailingGChatAPI struct{}

func (rateLimitFailingGChatAPI) ValidateAuth(context.Context) error {
	return &bridgesdk.RateLimitError{Err: errors.New("slow down"), RetryAfter: 5 * time.Second}
}
func (rateLimitFailingGChatAPI) CreateMessage(context.Context, gchatCreateMessageRequest) (*gchatSentMessage, error) {
	return nil, errors.New("unsupported")
}
func (rateLimitFailingGChatAPI) UpdateMessage(context.Context, gchatUpdateMessageRequest) (*gchatSentMessage, error) {
	return nil, errors.New("unsupported")
}
func (rateLimitFailingGChatAPI) DeleteMessage(context.Context, string) error {
	return errors.New("unsupported")
}
func (rateLimitFailingGChatAPI) GetMessage(context.Context, string) (*gchatMessage, error) {
	return nil, errors.New("unsupported")
}

func (f *fakeGChatAPI) ValidateAuth(context.Context) error { return nil }

func (f *fakeGChatAPI) CreateMessage(_ context.Context, req gchatCreateMessageRequest) (*gchatSentMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createAttempts = append(f.createAttempts, req)
	if f.createErr != nil && (f.createErrorText == "" || req.Text == f.createErrorText) {
		return nil, f.createErr
	}
	f.messages = append(f.messages, req)
	name := "spaces/AAA/messages/msg-" + strconv.Itoa(len(f.messages))
	if f.createEmptyMessageName {
		name = ""
	}
	msg := gchatSentMessage{Name: name}
	f.store = append(f.store, msg)
	return &msg, nil
}

func countGChatCreateRequests(requests []gchatCreateMessageRequest, text string) int {
	count := 0
	for _, request := range requests {
		if request.Text == text {
			count++
		}
	}
	return count
}

func testGChatResumeRequest(request bridgepkg.DeliveryRequest) bridgepkg.DeliveryRequest {
	resume := request
	resume.Event.EventType = bridgepkg.DeliveryEventTypeResume
	resume.Event.Resume = &bridgepkg.DeliveryResumeState{LatestEventType: bridgepkg.DeliveryEventTypeFinal}
	resume.Snapshot = &bridgepkg.DeliverySnapshot{
		DeliveryID:       request.Event.DeliveryID,
		SessionID:        "sess-resume",
		TurnID:           "turn-resume",
		BridgeInstanceID: request.Event.BridgeInstanceID,
		RoutingKey:       request.Event.RoutingKey,
		DeliveryTarget:   request.Event.DeliveryTarget,
		LatestSeq:        request.Event.Seq,
		LatestEventType:  bridgepkg.DeliveryEventTypeFinal,
		CurrentContent:   request.Event.Content,
		Operation:        request.Event.Operation,
		Final:            true,
		UpdatedAt:        time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC),
	}
	return resume
}

func (f *fakeGChatAPI) UpdateMessage(_ context.Context, req gchatUpdateMessageRequest) (*gchatSentMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, req)
	if f.updateErr != nil && (f.updateErrorText == "" || req.Text == f.updateErrorText) {
		return nil, f.updateErr
	}
	return &gchatSentMessage{Name: req.MessageName}, nil
}

func (f *fakeGChatAPI) DeleteMessage(_ context.Context, messageName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletes = append(f.deletes, messageName)
	return nil
}

func (f *fakeGChatAPI) GetMessage(_ context.Context, messageName string) (*gchatMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fetched = append(f.fetched, messageName)
	if msg, ok := f.messagesMap[messageName]; ok {
		msgCopy := msg
		return &msgCopy, nil
	}
	return nil, errors.New("not found")
}

type contextCheckingGChatAPI struct {
	t            *testing.T
	message      gchatMessage
	validateAuth func(context.Context) error
}

func (c *contextCheckingGChatAPI) ValidateAuth(ctx context.Context) error {
	if c.validateAuth != nil {
		return c.validateAuth(ctx)
	}
	return nil
}

func (c *contextCheckingGChatAPI) CreateMessage(context.Context, gchatCreateMessageRequest) (*gchatSentMessage, error) {
	return nil, errors.New("unsupported")
}

func (c *contextCheckingGChatAPI) UpdateMessage(context.Context, gchatUpdateMessageRequest) (*gchatSentMessage, error) {
	return nil, errors.New("unsupported")
}

func (c *contextCheckingGChatAPI) DeleteMessage(context.Context, string) error {
	return errors.New("unsupported")
}

func (c *contextCheckingGChatAPI) GetMessage(ctx context.Context, messageName string) (*gchatMessage, error) {
	c.t.Helper()
	if !errors.Is(ctx.Err(), context.Canceled) {
		c.t.Fatalf("GetMessage ctx.Err() = %v, want context.Canceled", ctx.Err())
	}
	if c.message.Name != "" {
		messageCopy := c.message
		if strings.TrimSpace(messageCopy.Name) == "" {
			messageCopy.Name = messageName
		}
		return &messageCopy, nil
	}
	return nil, context.Canceled
}

type gchatProviderTestServer struct {
	t              *testing.T
	server         *httptest.Server
	mu             sync.Mutex
	calls          []gchatAPICall
	directKey      *rsa.PrivateKey
	pubSubKey      *rsa.PrivateKey
	directCertPEM  string
	pubSubCertPEM  string
	directCertHits int
	pubSubCertHits int
	messageCounter int
	messageStore   map[string]gchatMessage
}

type gchatAPICall struct {
	Method string
	Path   string
	Body   map[string]any
}

func newGChatProviderTestServer(t *testing.T) *gchatProviderTestServer {
	t.Helper()

	directKey, directCertPEM := generateRSAKeyAndCert(t)
	pubSubKey, pubSubCertPEM := generateRSAKeyAndCert(t)

	s := &gchatProviderTestServer{
		t:             t,
		directKey:     directKey,
		pubSubKey:     pubSubKey,
		directCertPEM: directCertPEM,
		pubSubCertPEM: pubSubCertPEM,
		messageStore: map[string]gchatMessage{
			"spaces/AAA/messages/msg-react": {
				Name:   "spaces/AAA/messages/msg-react",
				Space:  &gchatSpace{Name: "spaces/AAA", Type: "SPACE"},
				Thread: &gchatThread{Name: "spaces/AAA/threads/thread-react"},
			},
		},
	}
	s.server = httptest.NewServer(http.HandlerFunc(s.serveHTTP))
	return s
}

func (s *gchatProviderTestServer) Close() { s.server.Close() }

func (s *gchatProviderTestServer) URL() string { return s.server.URL }

func (s *gchatProviderTestServer) TokenURL() string { return s.server.URL + "/oauth2/token" }

func (s *gchatProviderTestServer) DirectCertsURL() string { return s.server.URL + "/direct-certs" }

func (s *gchatProviderTestServer) PubSubCertsURL() string { return s.server.URL + "/pubsub-certs" }

func (s *gchatProviderTestServer) DirectCertHits() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.directCertHits
}

func (s *gchatProviderTestServer) PubSubCertHits() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pubSubCertHits
}

func (s *gchatProviderTestServer) Calls() []gchatAPICall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]gchatAPICall, len(s.calls))
	copy(out, s.calls)
	return out
}

func (s *gchatProviderTestServer) serveHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/direct-certs":
		s.mu.Lock()
		s.directCertHits++
		s.mu.Unlock()
		if err := json.NewEncoder(w).Encode(map[string]string{"direct-kid": s.directCertPEM}); err != nil {
			s.t.Errorf("encode direct certificate response: %v", err)
		}
		return
	case r.Method == http.MethodGet && r.URL.Path == "/pubsub-certs":
		s.mu.Lock()
		s.pubSubCertHits++
		s.mu.Unlock()
		if err := json.NewEncoder(w).Encode(map[string]string{"pubsub-kid": s.pubSubCertPEM}); err != nil {
			s.t.Errorf("encode Pub/Sub certificate response: %v", err)
		}
		return
	case r.Method == http.MethodPost && r.URL.Path == "/oauth2/token":
		if err := json.NewEncoder(w).
			Encode(gchatTokenResponse{AccessToken: "token-123", ExpiresIn: 3600, TokenType: "Bearer"}); err != nil {
			s.t.Errorf("encode OAuth token response: %v", err)
		}
		return
	}

	if strings.HasPrefix(r.URL.Path, "/v1/") {
		var body map[string]any
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
				s.t.Errorf("decode Google Chat API request: %v", err)
			}
		}
		s.mu.Lock()
		s.calls = append(s.calls, gchatAPICall{Method: r.Method, Path: r.URL.Path, Body: body})
		s.mu.Unlock()

		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages"):
			s.mu.Lock()
			s.messageCounter++
			name := "spaces/AAA/messages/msg-" + strconv.Itoa(s.messageCounter)
			threadName := ""
			if thread, ok := body["thread"].(map[string]any); ok {
				threadName, _ = thread["name"].(string)
			}
			s.messageStore[name] = gchatMessage{
				Name:   name,
				Text:   stringValue(body["text"]),
				Space:  &gchatSpace{Name: "spaces/AAA", Type: "SPACE"},
				Thread: &gchatThread{Name: firstNonEmpty(threadName, "spaces/AAA/threads/thread-created")},
			}
			s.mu.Unlock()
			if err := json.NewEncoder(w).
				Encode(gchatSentMessage{Name: name, Thread: &gchatThread{Name: firstNonEmpty(threadName, "spaces/AAA/threads/thread-created")}}); err != nil {
				s.t.Errorf("encode created Google Chat message: %v", err)
			}
			return
		case r.Method == http.MethodPatch && r.URL.Query().Get("updateMask") == providerTextKey:
			name := strings.TrimPrefix(r.URL.Path, "/v1/")
			if err := json.NewEncoder(w).Encode(gchatSentMessage{Name: name}); err != nil {
				s.t.Errorf("encode updated Google Chat message: %v", err)
			}
			return
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
			return
		case r.Method == http.MethodGet:
			name := strings.TrimPrefix(r.URL.Path, "/v1/")
			s.mu.Lock()
			msg, ok := s.messageStore[name]
			s.mu.Unlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			if err := json.NewEncoder(w).Encode(msg); err != nil {
				s.t.Errorf("encode fetched Google Chat message: %v", err)
			}
			return
		}
	}

	http.NotFound(w, r)
}

func (s *gchatProviderTestServer) signDirectToken(t *testing.T, audience string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": gchatDefaultDirectIssuer,
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

func (s *gchatProviderTestServer) signPubSubToken(
	t *testing.T,
	audience string,
	email string,
	emailVerified bool,
) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":            gchatDefaultPubSubIssuerURL,
		"aud":            audience,
		"email":          email,
		"email_verified": emailVerified,
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

func generateRSAKeyAndCert(t *testing.T) (*rsa.PrivateKey, string) {
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
	pemCert := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	return key, pemCert
}

func newRuntimePeerPair(t *testing.T) (*gchatProvider, *bridgesdk.Peer, func()) {
	t.Helper()

	hostConn, runtimeConn := net.Pipe()
	runtime, err := newGChatProvider(io.Discard)
	if err != nil {
		t.Fatalf("newGChatProvider() error = %v", err)
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
				t.Fatalf("provider HTTP shutdown error = %v", err)
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
			if err := runtime.lifecycle.Wait(context.Background()); err != nil {
				t.Fatalf("provider lifecycle wait error = %v", err)
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

func setGChatProviderRoute(
	provider *gchatProvider,
	instanceID string,
	config *resolvedInstanceConfig,
) {
	provider.routes.Replace(map[string]resolvedInstanceConfig{instanceID: *config}, nil)
}

func testBridgeRuntime(t *testing.T, now time.Time, instanceID string) subprocess.InitializeBridgeManagedInstance {
	t.Helper()

	return subprocess.InitializeBridgeManagedInstance{
		Instance: bridgepkg.BridgeInstance{
			ID:            instanceID,
			Scope:         bridgepkg.ScopeWorkspace,
			WorkspaceID:   "ws-gchat",
			Platform:      "gchat",
			ExtensionName: "gchat",
			DisplayName:   "Google Chat",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true, IncludeThread: true, IncludeGroup: true},
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
			{BindingName: "credentials_json", Kind: "json", Value: testCredentialsJSON(t)},
			{BindingName: "project_number", Kind: "token", Value: "123456789"},
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
			Name:       "gchat",
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
				Provider:         "gchat",
				Platform:         "gchat",
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
	threadID := encodeGChatThreadID(gchatThreadRef{
		SpaceName:  "spaces/AAA",
		ThreadName: "spaces/AAA/threads/thread-1",
	})
	return bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       deliveryID,
			BridgeInstanceID: instanceID,
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-gchat",
				BridgeInstanceID: instanceID,
				GroupID:          "spaces/AAA",
				ThreadID:         threadID,
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: instanceID,
				GroupID:          "spaces/AAA",
				ThreadID:         threadID,
				Mode:             bridgepkg.DeliveryModeReply,
			},
			Seq:       seq,
			EventType: eventType,
			Content:   bridgepkg.MessageContent{Text: "hello"},
			Final:     final,
		},
	}
}

func testGChatProgressRequest(
	instanceID string,
	deliveryID string,
	seq int64,
	toolCallID string,
	toolID string,
	label string,
) bridgepkg.DeliveryRequest {
	request := testDeliveryRequest(
		instanceID,
		deliveryID,
		seq,
		bridgepkg.DeliveryEventTypeProgress,
		false,
	)
	request.Event.Content = bridgepkg.MessageContent{}
	request.Event.Progress = &bridgepkg.ToolProgress{
		ToolCallID: toolCallID,
		ToolID:     toolID,
		Phase:      bridgepkg.ToolProgressPhaseStarted,
		Label:      label,
		Index:      int(seq),
	}
	return request
}

func testDeleteRequest(
	instanceID string,
	deliveryID string,
	seq int64,
	remoteMessageID string,
) bridgepkg.DeliveryRequest {
	req := testDeliveryRequest(instanceID, deliveryID, seq, bridgepkg.DeliveryEventTypeDelete, true)
	req.Event.Operation = bridgepkg.DeliveryOperationDelete
	req.Event.Reference = &bridgepkg.DeliveryMessageReference{RemoteMessageID: remoteMessageID}
	req.Event.Content = bridgepkg.MessageContent{}
	return req
}

func directWebhookPayload() string {
	return `{"chat":{"eventTime":"2026-04-15T20:10:00Z","messagePayload":{"space":{"name":"spaces/AAA","type":"SPACE"},"message":{"name":"spaces/AAA/messages/msg-direct","argumentText":"Need a summary","createTime":"2026-04-15T20:10:00Z","sender":{"name":"users/123","displayName":"Alice Example","email":"alice@example.com"},"thread":{"name":"spaces/AAA/threads/thread-1"}}}}}`
}

func pubSubReactionPayload(t *testing.T) string {
	t.Helper()

	payload := map[string]any{
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
	}
	raw := mustJSON(t, payload)
	push := map[string]any{
		"message": map[string]any{
			"data":        encodeBase64(raw),
			"messageId":   "pubsub-1",
			"publishTime": "2026-04-15T20:10:01Z",
			"attributes": map[string]any{
				"ce-type":    "google.workspace.chat.reaction.v1.created",
				"ce-subject": "//chat.googleapis.com/spaces/AAA",
				"ce-time":    "2026-04-15T20:10:01Z",
			},
		},
		"subscription": "projects/test/subscriptions/gchat",
	}
	return string(mustJSON(t, push))
}

func pubSubMessagePayload(t *testing.T, now time.Time) string {
	t.Helper()

	payload := map[string]any{
		"message": map[string]any{
			"name":       "spaces/AAA/messages/msg-pubsub",
			"text":       "hello from pubsub",
			"createTime": now.Format(time.RFC3339Nano),
			"sender": map[string]any{
				"name":        "users/234",
				"displayName": "Bob",
				"email":       "bob@example.com",
			},
			"space": map[string]any{
				"name": "spaces/AAA",
				"type": "SPACE",
			},
			"thread": map[string]any{
				"name": "spaces/AAA/threads/thread-pubsub",
			},
		},
	}
	raw := mustJSON(t, payload)
	push := map[string]any{
		"message": map[string]any{
			"data":        encodeBase64(raw),
			"messageId":   "pubsub-message-1",
			"publishTime": now.Format(time.RFC3339Nano),
			"attributes": map[string]any{
				"ce-type":    "google.workspace.chat.message.v1.created",
				"ce-subject": "//chat.googleapis.com/spaces/AAA",
				"ce-time":    now.Format(time.RFC3339Nano),
			},
		},
		"subscription": "projects/test/subscriptions/gchat",
	}
	return string(mustJSON(t, push))
}

func encodeBase64(raw []byte) string {
	return strings.TrimSpace(base64.StdEncoding.EncodeToString(raw))
}

func mustUnmarshalGChatEvent(t *testing.T, payload string) gchatEvent {
	t.Helper()

	var event gchatEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		t.Fatalf("json.Unmarshal(gchatEvent) error = %v", err)
	}
	return event
}

func mustCredentials(t *testing.T) serviceAccountCredentials {
	t.Helper()

	var credentials serviceAccountCredentials
	if err := json.Unmarshal([]byte(testCredentialsJSON(t)), &credentials); err != nil {
		t.Fatalf("json.Unmarshal(credentials) error = %v", err)
	}
	return credentials
}

func testCredentialsJSON(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	encoded, err := json.Marshal(serviceAccountCredentials{
		ClientEmail: "bot@example.iam.gserviceaccount.com",
		PrivateKey:  string(pemKey),
		TokenURI:    gchatDefaultAuthEndpointURL,
	})
	if err != nil {
		t.Fatalf("json.Marshal(credentials) error = %v", err)
	}
	return string(encoded)
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return raw
}

func setProviderTestEnv(t *testing.T) bridgesdk.AdapterMarkerPaths {
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

func reserveListenAddr(t *testing.T) string {
	t.Helper()

	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("ln.Close() error = %v", err)
	}
	return addr
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

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition did not succeed before timeout")
}

func nonEmptyLines(input string) []string {
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

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

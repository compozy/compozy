package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
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
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/subprocess"
)

func enableTeamsLoopbackCredentialedURLsForTesting(t *testing.T) {
	t.Helper()
	t.Setenv(teamsTestLoopbackAuthEnv, "1")
}

func TestTeamsProgressRendering(t *testing.T) {
	t.Parallel()

	t.Run("Should acknowledge default-off progress without calling the Teams API", func(t *testing.T) {
		t.Parallel()

		provider, err := newTeamsProvider(io.Discard)
		if err != nil {
			t.Fatalf("newTeamsProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testTeamsManagedInstance(t, time.Unix(8_000, 0).UTC(), "brg-progress-off", map[string]any{
			"service_url": "https://service.test",
		})
		setTeamsProviderRoute(provider, managed.Instance.ID, resolvedInstanceConfig{
			managed:    &managed,
			instanceID: managed.Instance.ID,
			appID:      "app-id",
			serviceURL: "https://service.test",
		})
		apiCalls := 0
		provider.apiFactory = func(resolvedInstanceConfig) teamsAPI {
			apiCalls++
			return &fakeTeamsAPI{}
		}

		request := testTeamsProgressRequest(
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
			t.Fatalf("Teams API factory calls = %d, want zero", apiCalls)
		}

		provider.storeDeliveryState(managed.Instance.ID, request.Event.DeliveryID, deliveryState{
			RemoteMessageID: "stale-message",
		})
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

		provider, err := newTeamsProvider(io.Discard)
		if err != nil {
			t.Fatalf("newTeamsProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testTeamsManagedInstance(t, time.Unix(8_025, 0).UTC(), "brg-progress-failure", map[string]any{
			"service_url": "https://service.test",
		})
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"all","grouping":"separate","typing":false,"reactions":false}}`,
		)
		setTeamsProviderRoute(provider, managed.Instance.ID, resolvedInstanceConfig{
			managed:    &managed,
			instanceID: managed.Instance.ID,
			appID:      "app-id",
			serviceURL: "https://service.test",
		})
		api := &fakeTeamsAPI{sendErr: errors.New("activity unavailable")}
		provider.apiFactory = func(resolvedInstanceConfig) teamsAPI { return api }

		request := testTeamsProgressRequest(
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
			t.Fatal("handleBridgesProgress() error = nil, want failed activity error")
		}
		if provider.deliveryState(managed.Instance.ID, request.Event.DeliveryID).Progress == nil {
			t.Fatal("failed progress dispatcher = nil, want retained dispatcher for retry")
		}
		if err := provider.healthCheck(); err == nil {
			t.Fatal("healthCheck() error = nil, want reported progress failure")
		}
	})

	t.Run("Should detach pending progress when the instance switches to off", func(t *testing.T) {
		t.Parallel()

		provider, err := newTeamsProvider(io.Discard)
		if err != nil {
			t.Fatalf("newTeamsProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testTeamsManagedInstance(t, time.Unix(8_050, 0).UTC(), "brg-progress-disabled", map[string]any{
			"service_url": "https://service.test",
		})
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"all","grouping":"accumulate","typing":false,"reactions":false}}`,
		)
		setTeamsProviderRoute(provider, managed.Instance.ID, resolvedInstanceConfig{
			managed:    &managed,
			instanceID: managed.Instance.ID,
			appID:      "app-id",
			serviceURL: "https://service.test",
		})
		api := &fakeTeamsAPI{nextActivityID: 10}
		provider.apiFactory = func(resolvedInstanceConfig) teamsAPI { return api }

		first := testTeamsProgressRequest(
			managed.Instance.ID,
			"delivery-progress-disabled",
			1,
			"call-1",
			"agh__terminal",
			"Inspecting",
		)
		second := testTeamsProgressRequest(
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

		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"off","grouping":"accumulate","typing":false,"reactions":false}}`,
		)
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
		if got := len(api.updateCalls); got != 0 {
			t.Fatalf("Teams progress updates after disabling = %d, want zero", got)
		}
	})

	t.Run("Should flush an opted-in markdown progress activity before the final answer", func(t *testing.T) {
		t.Parallel()

		provider, err := newTeamsProvider(io.Discard)
		if err != nil {
			t.Fatalf("newTeamsProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testTeamsManagedInstance(t, time.Unix(8_100, 0).UTC(), "brg-progress-on", map[string]any{
			"service_url": "https://service.test",
		})
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"all","grouping":"accumulate","typing":true,"reactions":true}}`,
		)
		setTeamsProviderRoute(provider, managed.Instance.ID, resolvedInstanceConfig{
			managed:    &managed,
			instanceID: managed.Instance.ID,
			appID:      "app-id",
			serviceURL: "https://service.test",
		})
		api := &fakeTeamsAPI{nextActivityID: 20}
		provider.apiFactory = func(resolvedInstanceConfig) teamsAPI { return api }

		first := testTeamsProgressRequest(
			managed.Instance.ID,
			"delivery-progress-on",
			1,
			"call-1",
			"agh__terminal",
			"Inspecting",
		)
		second := testTeamsProgressRequest(
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

		var messageCalls []teamsSendCall
		for _, call := range api.sendCalls {
			if call.Activity.Type == providerMessageKey {
				messageCalls = append(messageCalls, call)
			}
		}
		if got, want := len(messageCalls), 2; got != want {
			t.Fatalf("Teams message activity calls = %d, want %d", got, want)
		}
		if got, want := messageCalls[0].Activity.Text, "Inspecting"; got != want {
			t.Fatalf("progress activity text = %q, want %q", got, want)
		}
		if got, want := messageCalls[0].Activity.TextFormat, "markdown"; got != want {
			t.Fatalf("progress activity text format = %q, want %q", got, want)
		}
		if got, want := messageCalls[0].ReplyToID, "activity-parent"; got != want {
			t.Fatalf("progress activity reply id = %q, want %q", got, want)
		}
		if got, want := len(api.updateCalls), 1; got != want {
			t.Fatalf("Teams progress update calls = %d, want %d", got, want)
		}
		if got, want := api.updateCalls[0].Activity.Text, "Inspecting\nReading tasks"; got != want {
			t.Fatalf("progress update text = %q, want %q", got, want)
		}
		if got, want := api.updateCalls[0].Activity.TextFormat, "markdown"; got != want {
			t.Fatalf("progress update text format = %q, want %q", got, want)
		}
		if got, want := messageCalls[1].Activity.Text, "Final answer"; got != want {
			t.Fatalf("final activity text = %q, want %q", got, want)
		}
	})

	t.Run("Should deliver final text once after a committed progress flush", func(t *testing.T) {
		t.Parallel()

		provider, err := newTeamsProvider(io.Discard)
		if err != nil {
			t.Fatalf("newTeamsProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testTeamsManagedInstance(
			t,
			time.Unix(8_125, 0).UTC(),
			"brg-progress-committed-flush",
			map[string]any{
				"service_url": "https://service.test",
			},
		)
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"all","grouping":"accumulate","typing":false,"reactions":false}}`,
		)
		setTeamsProviderRoute(provider, managed.Instance.ID, resolvedInstanceConfig{
			managed:    &managed,
			instanceID: managed.Instance.ID,
			appID:      "app-id",
			serviceURL: "https://service.test",
		})
		api := &fakeTeamsAPI{
			nextActivityID:  40,
			updateErrorText: "Inspecting\nReading tasks",
			updateErr: &bridgesdk.CommittedMutationError{
				Err: errors.New("Teams progress update result unavailable"),
			},
		}
		provider.apiFactory = func(resolvedInstanceConfig) teamsAPI { return api }

		first := testTeamsProgressRequest(
			managed.Instance.ID,
			"delivery-progress-committed-flush",
			1,
			"call-1",
			"agh__terminal",
			"Inspecting",
		)
		second := testTeamsProgressRequest(
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

		if got, want := countTeamsSendCalls(api.sendAttempts, "Final answer"), 1; got != want {
			t.Fatalf("Teams final send attempts = %d, want %d", got, want)
		}
		if got, want := len(api.updateCalls), 1; got != want {
			t.Fatalf("Teams committed progress update attempts = %d, want %d", got, want)
		}
		if state := provider.deliveryState(managed.Instance.ID, first.Event.DeliveryID); state.Progress != nil {
			t.Fatal("terminal delivery progress dispatcher != nil after final text")
		}
	})

	t.Run("Should remove and close delivery state after committed final text", func(t *testing.T) {
		t.Parallel()

		provider, err := newTeamsProvider(io.Discard)
		if err != nil {
			t.Fatalf("newTeamsProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testTeamsManagedInstance(t, time.Unix(8_137, 0).UTC(), "brg-final-committed", map[string]any{
			"service_url": "https://service.test",
		})
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"all","grouping":"accumulate","typing":false,"reactions":false}}`,
		)
		setTeamsProviderRoute(provider, managed.Instance.ID, resolvedInstanceConfig{
			managed:    &managed,
			instanceID: managed.Instance.ID,
			appID:      "app-id",
			serviceURL: "https://service.test",
		})
		api := &fakeTeamsAPI{
			nextActivityID: 50,
			sendErrorText:  "Final answer",
			sendErr: &bridgesdk.CommittedMutationError{
				Err: errors.New("Teams final send result unavailable"),
			},
		}
		provider.apiFactory = func(resolvedInstanceConfig) teamsAPI { return api }

		progress := testTeamsProgressRequest(
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
		if got, want := countTeamsSendCalls(api.sendAttempts, "Final answer"), 1; got != want {
			t.Fatalf("Teams committed final send attempts = %d, want %d", got, want)
		}
	})

	t.Run("Should reuse one proactive conversation for typing and progress", func(t *testing.T) {
		t.Parallel()

		provider, err := newTeamsProvider(io.Discard)
		if err != nil {
			t.Fatalf("newTeamsProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)
		managed := testTeamsManagedInstance(t, time.Unix(8_200, 0).UTC(), "brg-progress-dm", map[string]any{
			"service_url": "https://service.test",
		})
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"all","grouping":"accumulate","typing":true,"reactions":false}}`,
		)
		setTeamsProviderRoute(provider, managed.Instance.ID, resolvedInstanceConfig{
			managed:     &managed,
			instanceID:  managed.Instance.ID,
			appID:       "app-id",
			appTenantID: "tenant-id",
			serviceURL:  "https://service.test",
		})
		api := &fakeTeamsAPI{createConversationID: "conversation-dm", nextActivityID: 30}
		provider.apiFactory = func(resolvedInstanceConfig) teamsAPI { return api }

		request := testTeamsProgressRequest(
			managed.Instance.ID,
			"delivery-progress-dm",
			1,
			"call-dm",
			"agh__terminal",
			"Inspecting",
		)
		request.Event.RoutingKey.GroupID = ""
		request.Event.RoutingKey.ThreadID = ""
		request.Event.RoutingKey.PeerID = "29:user"
		request.Event.DeliveryTarget.GroupID = ""
		request.Event.DeliveryTarget.ThreadID = ""
		request.Event.DeliveryTarget.PeerID = "29:user"
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			request,
		); err != nil {
			t.Fatalf("handleBridgesProgress() error = %v", err)
		}

		if got, want := len(api.createCalls), 1; got != want {
			t.Fatalf("Teams proactive conversation calls = %d, want %d", got, want)
		}
		if got, want := len(api.sendCalls), 2; got != want {
			t.Fatalf("Teams typing and progress calls = %d, want %d", got, want)
		}
		if got, want := api.sendCalls[0].Activity.Type, teamsTypingActivityType; got != want {
			t.Fatalf("Teams first progress activity type = %q, want %q", got, want)
		}
		if got, want := api.sendCalls[1].Activity.Type, providerMessageKey; got != want {
			t.Fatalf("Teams second progress activity type = %q, want %q", got, want)
		}
		for index, call := range api.sendCalls {
			if got, want := call.ConversationID, "conversation-dm"; got != want {
				t.Fatalf("Teams send call %d conversation id = %q, want %q", index, got, want)
			}
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
		_, deliveryExists := provider.deliveries.Load(deliveryStateKey(managed.Instance.ID, request.Event.DeliveryID))
		if deliveryExists {
			t.Fatal("lifecycle-only delivery state still exists, want terminal removal")
		}

		staleDeliveryID := "delivery-progress-dm-stale"
		provider.storeDeliveryState(managed.Instance.ID, staleDeliveryID, deliveryState{
			RemoteMessageID: "stale-message",
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

func TestMapTeamsActivityFamiliesAndDMPolicy(t *testing.T) {
	t.Parallel()

	cfg := resolvedInstanceConfig{
		instanceID: "brg-teams",
		managed: &subprocess.InitializeBridgeManagedInstance{
			Instance: bridgepkg.BridgeInstance{
				ID:          "brg-teams",
				Scope:       bridgepkg.ScopeWorkspace,
				WorkspaceID: "ws-teams",
			},
		},
		serviceURL: teamsDefaultServiceURL,
	}

	messageActivity := teamsActivity{
		Type:       "message",
		ID:         "activity-1",
		Text:       "<at>Bot</at> Need a summary",
		Timestamp:  "2026-04-15T18:05:00Z",
		ServiceURL: teamsDefaultServiceURL,
		ChannelID:  "msteams",
		From:       teamsChannelAccount{ID: "29:user-1", Name: "Alice Example"},
		Recipient:  teamsChannelAccount{ID: "28:bot", Name: "Bridge Bot"},
		Conversation: teamsConversation{
			ID:               "19:channel@thread.tacv2;messageid=activity-1",
			ConversationType: "channel",
			TenantID:         "11111111-2222-3333-4444-555555555555",
		},
		Attachments: []teamsAttachment{{
			ContentType: "image/png",
			ContentURL:  "https://example.test/image.png",
			Name:        "image.png",
		}},
	}
	items, err := mapTeamsActivity(messageActivity, cfg, time.Time{})
	if err != nil {
		t.Fatalf("mapTeamsActivity(message) error = %v", err)
	}
	if got, want := len(items), 1; got != want {
		t.Fatalf("len(items) = %d, want %d", got, want)
	}
	message := items[0].Envelope
	if got, want := message.EventFamily, bridgepkg.InboundEventFamilyMessage; got != want {
		t.Fatalf("message.EventFamily = %q, want %q", got, want)
	}
	if got, want := message.GroupID, "19:channel@thread.tacv2"; got != want {
		t.Fatalf("message.GroupID = %q, want %q", got, want)
	}
	if got, want := len(message.Attachments), 1; got != want {
		t.Fatalf("len(message.Attachments) = %d, want %d", got, want)
	}
	if _, err := decodeTeamsThreadID(message.ThreadID); err != nil {
		t.Fatalf("decodeTeamsThreadID(message.ThreadID) error = %v", err)
	}

	actionActivity := teamsActivity{
		Type:       "message",
		ID:         "activity-2",
		Timestamp:  "2026-04-15T18:05:01Z",
		ServiceURL: teamsDefaultServiceURL,
		From:       teamsChannelAccount{ID: "29:user-1", Name: "Alice Example"},
		Recipient:  teamsChannelAccount{ID: "28:bot", Name: "Bridge Bot"},
		Conversation: teamsConversation{
			ID:       "a:direct-conversation",
			TenantID: "11111111-2222-3333-4444-555555555555",
		},
		Value: json.RawMessage(`{"actionId":"approve","value":"yes"}`),
	}
	items, err = mapTeamsActivity(actionActivity, cfg, time.Time{})
	if err != nil {
		t.Fatalf("mapTeamsActivity(message action) error = %v", err)
	}
	if got, want := items[0].Envelope.EventFamily, bridgepkg.InboundEventFamilyAction; got != want {
		t.Fatalf("items[0].Envelope.EventFamily = %q, want %q", got, want)
	}
	if got, want := items[0].Envelope.Action.ActionID, "approve"; got != want {
		t.Fatalf("items[0].Envelope.Action.ActionID = %q, want %q", got, want)
	}
	if got, want := items[0].Envelope.PeerID, "a:direct-conversation"; got != want {
		t.Fatalf("items[0].Envelope.PeerID = %q, want %q", got, want)
	}

	invokeActivity := actionActivity
	invokeActivity.Type = "invoke"
	invokeActivity.ID = "activity-3"
	invokeActivity.Value = json.RawMessage(
		`{"action":{"data":{"actionId":"escalate","value":"high"}}}`,
	)
	items, err = mapTeamsActivity(invokeActivity, cfg, time.Time{})
	if err != nil {
		t.Fatalf("mapTeamsActivity(invoke action) error = %v", err)
	}
	if got, want := items[0].Envelope.Action.ActionID, "escalate"; got != want {
		t.Fatalf("items[0].Envelope.Action.ActionID = %q, want %q", got, want)
	}

	reactionActivity := teamsActivity{
		Type:       "messageReaction",
		ID:         "activity-4",
		Timestamp:  "2026-04-15T18:05:02Z",
		ServiceURL: teamsDefaultServiceURL,
		From:       teamsChannelAccount{ID: "29:user-1", Name: "Alice Example"},
		Conversation: teamsConversation{
			ID:               "19:channel@thread.tacv2;messageid=activity-1",
			ConversationType: "channel",
		},
		ReactionsAdded:   []teamsMessageReaction{{Type: "like"}},
		ReactionsRemoved: []teamsMessageReaction{{Type: "sad"}},
	}
	items, err = mapTeamsActivity(reactionActivity, cfg, time.Time{})
	if err != nil {
		t.Fatalf("mapTeamsActivity(reaction) error = %v", err)
	}
	if got, want := len(items), 2; got != want {
		t.Fatalf("len(reaction items) = %d, want %d", got, want)
	}
	if got, want := items[0].Envelope.Reaction.Added, true; got != want {
		t.Fatalf("items[0].Envelope.Reaction.Added = %t, want %t", got, want)
	}
	if got, want := items[1].Envelope.Reaction.Added, false; got != want {
		t.Fatalf("items[1].Envelope.Reaction.Added = %t, want %t", got, want)
	}

	user := teamsUserIdentity{
		ID:          "29:user-1",
		Username:    "alice example",
		DisplayName: "Alice Example",
	}
	if !allowTeamsDirectMessage(
		resolvedInstanceConfig{dmPolicy: bridgepkg.BridgeDMPolicyOpen},
		user,
		true,
	) {
		t.Fatal("allowTeamsDirectMessage(open) = false, want true")
	}
	if !allowTeamsDirectMessage(resolvedInstanceConfig{
		dmPolicy:       bridgepkg.BridgeDMPolicyAllowlist,
		allowUserIDs:   map[string]struct{}{"29:user-1": {}},
		allowUsernames: map[string]struct{}{"alice example": {}},
	}, user, true) {
		t.Fatal("allowTeamsDirectMessage(allowlist) = false, want true")
	}
	if !allowTeamsDirectMessage(resolvedInstanceConfig{
		dmPolicy:        bridgepkg.BridgeDMPolicyPairing,
		pairedUsernames: map[string]struct{}{"alice example": {}},
	}, user, true) {
		t.Fatal("allowTeamsDirectMessage(pairing) = false, want true")
	}
	if allowTeamsDirectMessage(
		resolvedInstanceConfig{dmPolicy: bridgepkg.BridgeDMPolicyAllowlist},
		user,
		true,
	) {
		t.Fatal("allowTeamsDirectMessage(rejected) = true, want false")
	}
}

func TestExecuteTeamsDeliveryConversationAndProactiveDM(t *testing.T) {
	t.Parallel()

	api := &fakeTeamsAPI{
		createConversationID: "a:created-conversation",
		nextActivityID:       700,
	}
	cfg := resolvedInstanceConfig{
		instanceID:  "brg-teams",
		serviceURL:  "https://smba.trafficmanager.net/teams/",
		appID:       "app-id",
		appTenantID: "11111111-2222-3333-4444-555555555555",
	}
	threadID := encodeTeamsThreadID(teamsThreadRef{
		ConversationID: "19:channel@thread.tacv2;messageid=activity-parent",
		ServiceURL:     "https://smba.trafficmanager.net/teams/",
	})
	startReq := bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "delivery-1",
			BridgeInstanceID: "brg-teams",
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-teams",
				BridgeInstanceID: "brg-teams",
				GroupID:          "19:channel@thread.tacv2",
				ThreadID:         threadID,
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: "brg-teams",
				GroupID:          "19:channel@thread.tacv2",
				ThreadID:         threadID,
				Mode:             bridgepkg.DeliveryModeReply,
			},
			Seq:       1,
			EventType: bridgepkg.DeliveryEventTypeStart,
			Content:   bridgepkg.MessageContent{Text: "hello"},
		},
	}

	startAck, state, err := executeTeamsDelivery(
		context.Background(),
		api,
		cfg,
		startReq,
		deliveryState{},
		nil,
		func(string, string) (teamsUserContext, bool) {
			return teamsUserContext{}, false
		},
	)
	if err != nil {
		t.Fatalf("executeTeamsDelivery(start) error = %v", err)
	}
	if got, want := len(api.sendCalls), 1; got != want {
		t.Fatalf("len(api.sendCalls) = %d, want %d", got, want)
	}
	if got, want := api.sendCalls[0].ReplyToID, "activity-parent"; got != want {
		t.Fatalf("api.sendCalls[0].ReplyToID = %q, want %q", got, want)
	}

	finalReq := startReq
	finalReq.Event.Seq = 2
	finalReq.Event.EventType = bridgepkg.DeliveryEventTypeFinal
	finalReq.Event.Final = true
	finalReq.Event.Content.Text = "hello world"
	finalAck, state, err := executeTeamsDelivery(
		context.Background(),
		api,
		cfg,
		finalReq,
		state,
		nil,
		func(string, string) (teamsUserContext, bool) {
			return teamsUserContext{}, false
		},
	)
	if err != nil {
		t.Fatalf("executeTeamsDelivery(final) error = %v", err)
	}
	if got, want := len(api.updateCalls), 1; got != want {
		t.Fatalf("len(api.updateCalls) = %d, want %d", got, want)
	}
	if got, want := finalAck.ReplaceRemoteMessageID, startAck.RemoteMessageID; got != want {
		t.Fatalf("finalAck.ReplaceRemoteMessageID = %q, want %q", got, want)
	}

	deleteReq := finalReq
	deleteReq.Event.Seq = 3
	deleteReq.Event.EventType = bridgepkg.DeliveryEventTypeDelete
	deleteReq.Event.Operation = bridgepkg.DeliveryOperationDelete
	deleteReq.Event.Reference = &bridgepkg.DeliveryMessageReference{
		RemoteMessageID: finalAck.RemoteMessageID,
	}
	deleteReq.Event.Content.Text = ""
	_, _, err = executeTeamsDelivery(
		context.Background(),
		api,
		cfg,
		deleteReq,
		state,
		nil,
		func(string, string) (teamsUserContext, bool) {
			return teamsUserContext{}, false
		},
	)
	if err != nil {
		t.Fatalf("executeTeamsDelivery(delete) error = %v", err)
	}
	if got, want := len(api.deleteCalls), 1; got != want {
		t.Fatalf("len(api.deleteCalls) = %d, want %d", got, want)
	}

	proactiveReq := bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "delivery-2",
			BridgeInstanceID: "brg-teams",
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-teams",
				BridgeInstanceID: "brg-teams",
				PeerID:           "29:user-2",
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: "brg-teams",
				PeerID:           "29:user-2",
				Mode:             bridgepkg.DeliveryModeDirectSend,
			},
			Seq:       1,
			EventType: bridgepkg.DeliveryEventTypeStart,
			Content:   bridgepkg.MessageContent{Text: "ping"},
		},
	}
	_, _, err = executeTeamsDelivery(
		context.Background(),
		api,
		cfg,
		proactiveReq,
		deliveryState{},
		nil,
		func(instanceID string, userID string) (teamsUserContext, bool) {
			if instanceID != "brg-teams" || userID != "29:user-2" {
				return teamsUserContext{}, false
			}
			return teamsUserContext{
				ServiceURL: "https://smba.trafficmanager.net/teams/",
				TenantID:   "11111111-2222-3333-4444-555555555555",
			}, true
		},
	)
	if err != nil {
		t.Fatalf("executeTeamsDelivery(proactive) error = %v", err)
	}
	if got, want := len(api.createCalls), 1; got != want {
		t.Fatalf("len(api.createCalls) = %d, want %d", got, want)
	}

	resumeReq := proactiveReq
	resumeReq.Event.EventType = bridgepkg.DeliveryEventTypeResume
	resumeReq.Event.Resume = &bridgepkg.DeliveryResumeState{
		LatestEventType: bridgepkg.DeliveryEventTypeFinal,
	}
	resumeReq.Snapshot = &bridgepkg.DeliverySnapshot{
		DeliveryID:       "delivery-2",
		SessionID:        "sess-1",
		TurnID:           "turn-1",
		BridgeInstanceID: "brg-teams",
		RoutingKey:       proactiveReq.Event.RoutingKey,
		DeliveryTarget:   proactiveReq.Event.DeliveryTarget,
		LatestSeq:        1,
		LastSentSeq:      1,
		LastAckedSeq:     1,
		LatestEventType:  bridgepkg.DeliveryEventTypeFinal,
		CurrentContent:   bridgepkg.MessageContent{Text: "ping"},
		RemoteMessageID:  startAck.RemoteMessageID,
		Final:            true,
		UpdatedAt:        time.Date(2026, 4, 15, 18, 10, 0, 0, time.UTC),
	}
	resumeAck, _, err := executeTeamsDelivery(
		context.Background(),
		api,
		cfg,
		resumeReq,
		deliveryState{},
		nil,
		func(string, string) (teamsUserContext, bool) {
			return teamsUserContext{}, false
		},
	)
	if err != nil {
		t.Fatalf("executeTeamsDelivery(resume) error = %v", err)
	}
	if got, want := resumeAck.RemoteMessageID, startAck.RemoteMessageID; got != want {
		t.Fatalf("resumeAck.RemoteMessageID = %q, want %q", got, want)
	}
}

func TestExecuteTeamsDeliveryChunksTerminalContent(t *testing.T) {
	t.Parallel()

	serviceURL := "https://smba.trafficmanager.net/teams/"
	conversationID := "19:channel@thread.tacv2"
	threadID := encodeTeamsThreadID(teamsThreadRef{
		ConversationID: conversationID + ";messageid=activity-parent",
		ServiceURL:     serviceURL,
	})
	cfg := resolvedInstanceConfig{
		instanceID: "brg-teams",
		serviceURL: serviceURL,
		appID:      "app-id",
	}
	routing := bridgepkg.RoutingKey{
		Scope:            bridgepkg.ScopeWorkspace,
		WorkspaceID:      "ws-teams",
		BridgeInstanceID: "brg-teams",
		GroupID:          conversationID,
		ThreadID:         threadID,
	}
	target := bridgepkg.DeliveryTarget{
		BridgeInstanceID: "brg-teams",
		GroupID:          conversationID,
		ThreadID:         threadID,
		Mode:             bridgepkg.DeliveryModeReply,
	}
	longText := strings.Repeat("teams chunk ", 3_000)
	lookup := func(string, string) (teamsUserContext, bool) {
		return teamsUserContext{}, false
	}

	t.Run("Should post terminal chunks in order and acknowledge the last activity", func(t *testing.T) {
		t.Parallel()

		api := &fakeTeamsAPI{nextActivityID: 700}
		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "delivery-create",
			BridgeInstanceID: "brg-teams",
			RoutingKey:       routing,
			DeliveryTarget:   target,
			Seq:              1,
			EventType:        bridgepkg.DeliveryEventTypeFinal,
			Final:            true,
			Content:          bridgepkg.MessageContent{Text: longText},
		}}

		ack, _, err := executeTeamsDelivery(
			context.Background(), api, cfg, request, deliveryState{}, nil, lookup,
		)
		if err != nil {
			t.Fatalf("executeTeamsDelivery() error = %v", err)
		}
		wantChunks := bridgesdk.ChunkMessage(longText, teamsMaxMessageLen, nil)
		if got, want := len(api.sendCalls), len(wantChunks); got != want {
			t.Fatalf("send calls = %d, want %d chunks", got, want)
		}
		for index, call := range api.sendCalls {
			if got, want := call.Activity.Text, wantChunks[index]; got != want {
				t.Fatalf("chunk %d text = %q, want %q", index, got, want)
			}
			if got := len([]rune(call.Activity.Text)); got > teamsMaxMessageLen {
				t.Fatalf("chunk %d length = %d, want <= %d", index, got, teamsMaxMessageLen)
			}
			if got, want := call.ConversationID, conversationID; got != want {
				t.Fatalf("chunk %d conversation = %q, want %q", index, got, want)
			}
			if got, want := call.ReplyToID, "activity-parent"; got != want {
				t.Fatalf("chunk %d reply id = %q, want %q", index, got, want)
			}
			indicator := fmt.Sprintf("\n\n(%d/%d)", index+1, len(api.sendCalls))
			if !strings.HasSuffix(call.Activity.Text, indicator) {
				t.Fatalf("chunk %d text = %q, want suffix %q", index, call.Activity.Text, indicator)
			}
		}
		ref, err := decodeRemoteMessageID(ack.RemoteMessageID)
		if err != nil {
			t.Fatalf("decodeRemoteMessageID() error = %v", err)
		}
		if got, want := ref.ActivityID, fmt.Sprintf("activity-%d", 699+len(api.sendCalls)); got != want {
			t.Fatalf("ack activity id = %q, want %q", got, want)
		}
	})

	t.Run("Should keep an overflowing non-terminal preview in one activity", func(t *testing.T) {
		t.Parallel()

		api := &fakeTeamsAPI{nextActivityID: 800}
		remoteID := testTeamsRemoteMessageID(conversationID, serviceURL, "activity-active")
		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "delivery-preview",
			BridgeInstanceID: "brg-teams",
			RoutingKey:       routing,
			DeliveryTarget:   target,
			Seq:              2,
			EventType:        bridgepkg.DeliveryEventTypeDelta,
			Operation:        bridgepkg.DeliveryOperationEdit,
			Reference:        &bridgepkg.DeliveryMessageReference{RemoteMessageID: remoteID},
			Content:          bridgepkg.MessageContent{Text: longText},
		}}

		_, _, err := executeTeamsDelivery(
			context.Background(), api, cfg, request,
			deliveryState{LastSeq: 1, RemoteMessageID: remoteID}, nil, lookup,
		)
		if err != nil {
			t.Fatalf("executeTeamsDelivery() error = %v", err)
		}
		if got, want := len(api.updateCalls), 1; got != want {
			t.Fatalf("update calls = %d, want %d", got, want)
		}
		wantPreview := bridgesdk.ChunkMessage(longText, teamsMaxMessageLen, nil)[0]
		if got := api.updateCalls[0].Activity.Text; got != wantPreview {
			t.Fatalf("preview text = %q, want %q", got, wantPreview)
		}
		if got := len([]rune(api.updateCalls[0].Activity.Text)); got > teamsMaxMessageLen {
			t.Fatalf("preview length = %d, want <= %d", got, teamsMaxMessageLen)
		}
		if !strings.Contains(api.updateCalls[0].Activity.Text, "\n\n(1/") {
			t.Fatalf("preview text = %q, want first-chunk indicator", api.updateCalls[0].Activity.Text)
		}
		if got := len(api.sendCalls); got != 0 {
			t.Fatalf("send calls = %d, want no preview continuations", got)
		}
	})

	t.Run("Should edit the active activity then post terminal continuations", func(t *testing.T) {
		t.Parallel()

		api := &fakeTeamsAPI{nextActivityID: 900}
		remoteID := testTeamsRemoteMessageID(conversationID, serviceURL, "activity-active")
		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "delivery-edit",
			BridgeInstanceID: "brg-teams",
			RoutingKey:       routing,
			DeliveryTarget:   target,
			Seq:              2,
			EventType:        bridgepkg.DeliveryEventTypeFinal,
			Final:            true,
			Operation:        bridgepkg.DeliveryOperationEdit,
			Reference:        &bridgepkg.DeliveryMessageReference{RemoteMessageID: remoteID},
			Content:          bridgepkg.MessageContent{Text: longText},
		}}

		ack, _, err := executeTeamsDelivery(
			context.Background(), api, cfg, request,
			deliveryState{LastSeq: 1, RemoteMessageID: remoteID}, nil, lookup,
		)
		if err != nil {
			t.Fatalf("executeTeamsDelivery() error = %v", err)
		}
		if got, want := len(api.updateCalls), 1; got != want {
			t.Fatalf("update calls = %d, want %d", got, want)
		}
		if len(api.sendCalls) == 0 {
			t.Fatal("send calls = 0, want terminal continuations")
		}
		wantChunks := bridgesdk.ChunkMessage(longText, teamsMaxMessageLen, nil)
		if got, want := len(api.sendCalls)+1, len(wantChunks); got != want {
			t.Fatalf("delivered chunks = %d, want %d", got, want)
		}
		if got, want := api.updateCalls[0].Activity.Text, wantChunks[0]; got != want {
			t.Fatalf("updated chunk = %q, want %q", got, want)
		}
		totalChunks := len(wantChunks)
		if got := len([]rune(api.updateCalls[0].Activity.Text)); got > teamsMaxMessageLen {
			t.Fatalf("updated chunk length = %d, want <= %d", got, teamsMaxMessageLen)
		}
		indicator := fmt.Sprintf("\n\n(1/%d)", totalChunks)
		if !strings.HasSuffix(api.updateCalls[0].Activity.Text, indicator) {
			t.Fatalf("updated chunk = %q, want suffix %q", api.updateCalls[0].Activity.Text, indicator)
		}
		for index, call := range api.sendCalls {
			if got, want := call.Activity.Text, wantChunks[index+1]; got != want {
				t.Fatalf("continuation %d = %q, want %q", index, got, want)
			}
			if got, want := call.ReplyToID, "activity-parent"; got != want {
				t.Fatalf("continuation %d reply id = %q, want %q", index, got, want)
			}
			if got := len([]rune(call.Activity.Text)); got > teamsMaxMessageLen {
				t.Fatalf("continuation %d length = %d, want <= %d", index, got, teamsMaxMessageLen)
			}
			indicator := fmt.Sprintf("\n\n(%d/%d)", index+2, totalChunks)
			if !strings.HasSuffix(call.Activity.Text, indicator) {
				t.Fatalf("continuation %d = %q, want suffix %q", index, call.Activity.Text, indicator)
			}
		}
		lastRef, err := decodeRemoteMessageID(ack.RemoteMessageID)
		if err != nil {
			t.Fatalf("decodeRemoteMessageID() error = %v", err)
		}
		if got, want := lastRef.ActivityID, fmt.Sprintf("activity-%d", 899+len(api.sendCalls)); got != want {
			t.Fatalf("ack activity id = %q, want %q", got, want)
		}
		if got := ack.ReplaceRemoteMessageID; got != remoteID {
			t.Fatalf("ReplaceRemoteMessageID = %q, want %q", got, remoteID)
		}
	})

	t.Run("Should keep root continuations at the conversation root", func(t *testing.T) {
		t.Parallel()

		api := &fakeTeamsAPI{nextActivityID: 1_000}
		rootThreadID := encodeTeamsThreadID(teamsThreadRef{
			ConversationID: conversationID,
			ServiceURL:     serviceURL,
		})
		rootRouting := routing
		rootRouting.ThreadID = rootThreadID
		rootTarget := target
		rootTarget.ThreadID = rootThreadID
		remoteID := testTeamsRemoteMessageID(conversationID, serviceURL, "activity-active")
		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "delivery-root-edit",
			BridgeInstanceID: "brg-teams",
			RoutingKey:       rootRouting,
			DeliveryTarget:   rootTarget,
			Seq:              2,
			EventType:        bridgepkg.DeliveryEventTypeFinal,
			Final:            true,
			Operation:        bridgepkg.DeliveryOperationEdit,
			Reference:        &bridgepkg.DeliveryMessageReference{RemoteMessageID: remoteID},
			Content:          bridgepkg.MessageContent{Text: longText},
		}}

		_, _, err := executeTeamsDelivery(
			context.Background(), api, cfg, request,
			deliveryState{LastSeq: 1, RemoteMessageID: remoteID}, nil, lookup,
		)
		if err != nil {
			t.Fatalf("executeTeamsDelivery() error = %v", err)
		}
		if len(api.sendCalls) == 0 {
			t.Fatal("send calls = 0, want terminal continuations")
		}
		for index, call := range api.sendCalls {
			if got := call.ReplyToID; got != "" {
				t.Fatalf("continuation %d reply id = %q, want root", index, got)
			}
		}
	})

	t.Run("Should reject a send response without an activity id", func(t *testing.T) {
		t.Parallel()

		api := &fakeTeamsAPI{sendEmptyActivityID: true}
		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "delivery-empty-id",
			BridgeInstanceID: "brg-teams",
			RoutingKey:       routing,
			DeliveryTarget:   target,
			Seq:              1,
			EventType:        bridgepkg.DeliveryEventTypeFinal,
			Final:            true,
			Content:          bridgepkg.MessageContent{Text: "hello"},
		}}

		ack, state, err := executeTeamsDelivery(
			context.Background(), api, cfg, request, deliveryState{}, nil, lookup,
		)
		if got, want := bridgesdk.ClassifyError(err).Class, bridgesdk.ErrorClassPermanent; got != want {
			t.Fatalf("executeTeamsDelivery() error class = %q, want %q: %v", got, want, err)
		}
		var committedErr *bridgesdk.CommittedMutationError
		if !errors.As(err, &committedErr) {
			t.Fatalf("executeTeamsDelivery() error = %T, want CommittedMutationError", err)
		}
		if got, want := len(api.sendAttempts), 1; got != want {
			t.Fatalf("send attempts = %d, want %d", got, want)
		}
		if ack != (bridgepkg.DeliveryAck{}) ||
			state.LastSeq != 0 ||
			state.RemoteMessageID != "" ||
			state.ReplaceRemoteMessageID != "" ||
			state.Chunks.NextChunk() != 0 {
			t.Fatalf("unacknowledged delivery = ack:%#v state:%#v, want no committed chunk", ack, state)
		}
	})

	t.Run("Should stop without an ack when a terminal continuation fails", func(t *testing.T) {
		t.Parallel()

		sendErr := errors.New("continuation unavailable")
		api := &fakeTeamsAPI{sendErr: sendErr}
		remoteID := testTeamsRemoteMessageID(conversationID, serviceURL, "activity-active")
		initialState := deliveryState{LastSeq: 1, RemoteMessageID: remoteID}
		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "delivery-continuation-failure",
			BridgeInstanceID: "brg-teams",
			RoutingKey:       routing,
			DeliveryTarget:   target,
			Seq:              2,
			EventType:        bridgepkg.DeliveryEventTypeFinal,
			Final:            true,
			Operation:        bridgepkg.DeliveryOperationEdit,
			Reference:        &bridgepkg.DeliveryMessageReference{RemoteMessageID: remoteID},
			Content:          bridgepkg.MessageContent{Text: longText},
		}}

		ack, state, err := executeTeamsDelivery(
			context.Background(), api, cfg, request, initialState, nil, lookup,
		)
		if !errors.Is(err, sendErr) {
			t.Fatalf("executeTeamsDelivery() error = %v, want wrapped continuation error", err)
		}
		if ack != (bridgepkg.DeliveryAck{}) ||
			state.LastSeq != initialState.LastSeq ||
			state.RemoteMessageID != initialState.RemoteMessageID ||
			state.ReplaceRemoteMessageID != initialState.ReplaceRemoteMessageID {
			t.Fatalf("failed delivery = ack:%#v state:%#v, want zero ack and no committed final state", ack, state)
		}
		if got, want := state.Chunks.NextChunk(), 1; got != want {
			t.Fatalf("completed chunk prefix = %d, want %d", got, want)
		}
	})

	t.Run("Should resume at the first unsent chunk after exhausting Retry-After attempts", func(t *testing.T) {
		t.Parallel()

		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "delivery-partial-chunk",
			BridgeInstanceID: "brg-teams",
			RoutingKey:       routing,
			DeliveryTarget:   target,
			Seq:              1,
			EventType:        bridgepkg.DeliveryEventTypeFinal,
			Final:            true,
			Content:          bridgepkg.MessageContent{Text: longText},
		}}
		chunks := teamsDeliveryChunks(request.Event)
		if len(chunks) < 2 {
			t.Fatalf("teamsDeliveryChunks() returned %d chunk, want at least 2", len(chunks))
		}

		api := &fakeTeamsAPI{
			nextActivityID: 1_100,
			sendErrorText:  chunks[1],
			sendErr: &bridgesdk.RateLimitError{
				Err:        errors.New("rate limited continuation"),
				RetryAfter: time.Nanosecond,
			},
		}
		_, partialState, err := executeTeamsDelivery(
			context.Background(), api, cfg, request, deliveryState{}, nil, lookup,
		)
		if err == nil {
			t.Fatal("executeTeamsDelivery(partial) error = nil, want exhausted rate limit")
		}
		if got, want := countTeamsSendCalls(
			api.sendAttempts,
			chunks[1],
		), bridgesdk.DefaultRetryConfig().Attempts; got != want {
			t.Fatalf("failed continuation attempts = %d, want %d", got, want)
		}
		if got, want := countTeamsSendCalls(api.sendCalls, chunks[0]), 1; got != want {
			t.Fatalf("successful first-chunk sends = %d, want %d", got, want)
		}

		api.sendErr = nil
		resume := testTeamsResumeRequest(request)
		if _, _, err := executeTeamsDelivery(
			context.Background(), api, cfg, resume, partialState, nil, lookup,
		); err != nil {
			t.Fatalf("executeTeamsDelivery(resume) error = %v", err)
		}
		if got, want := countTeamsSendCalls(api.sendCalls, chunks[0]), 1; got != want {
			t.Fatalf("successful first-chunk sends after resume = %d, want %d", got, want)
		}
		if got, want := len(api.sendCalls), len(chunks); got != want {
			t.Fatalf("successful chunk sends = %d, want %d", got, want)
		}
	})
}

func TestResolveInstanceConfigAndDetermineInitialState(t *testing.T) {
	mock := newTeamsProviderServer(t, teamsProviderServerConfig{})
	env := setProviderTestEnv(t)
	_ = env
	listenAddr := reserveListenAddr(t)
	t.Setenv(teamsListenAddrEnv, listenAddr)
	t.Setenv(teamsServiceURLEnv, mock.ServiceURL())
	t.Setenv(teamsOpenIDMetadataURLEnv, mock.MetadataURL())
	t.Setenv(teamsOAuthTokenURLEnvName(), mock.TokenURL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 18, 0, 0, 0, time.UTC)
	managed := testTeamsManagedInstance(t, now, "brg-1", map[string]any{
		"webhook": map[string]any{
			"listen_addr": listenAddr,
			"path":        "teams",
		},
		"batching": map[string]any{
			"delay_ms":        5,
			"split_delay_ms":  7,
			"split_threshold": 2,
		},
		"dm": map[string]any{
			"allow_user_ids":   []string{"29:user-1"},
			"allow_usernames":  []string{"Alice Example"},
			"paired_usernames": []string{"Bob Example"},
		},
	})

	mustHandleLifecycle(t, hostPeer, managed)
	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}
	states := waitForJSONLinesFile[bridgesdk.StateMarker](
		t,
		env.StatePath,
		func(items []bridgesdk.StateMarker) bool {
			return len(items) >= 1 && items[len(items)-1].Status.Normalize() == bridgepkg.BridgeStatusReady
		},
	)
	if got, want := states[len(states)-1].Status.Normalize(), bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("states[last].Status = %q, want %q", got, want)
	}
	waitForCondition(t, func() bool {
		return strings.TrimSpace(runtime.http.Address()) != ""
	})

	session := runtime.currentSession()
	if session == nil {
		t.Fatal("runtime.currentSession() = nil, want session")
	}

	cfg := runtime.resolveInstanceConfig(session, managed)
	if cfg.configError != nil {
		t.Fatalf("resolveInstanceConfig() configError = %v, want nil", cfg.configError)
	}
	defer cfg.batcher.Close()
	if got, want := cfg.serviceURL, mock.ServiceURL(); got != want {
		t.Fatalf("cfg.serviceURL = %q, want %q", got, want)
	}
	if got, want := cfg.openIDMetadataURL, mock.MetadataURL(); got != want {
		t.Fatalf("cfg.openIDMetadataURL = %q, want %q", got, want)
	}
	if got, want := cfg.tokenURL, mock.TokenURL(); got != want {
		t.Fatalf("cfg.tokenURL = %q, want %q", got, want)
	}
	if got, want := cfg.appTenantID, "11111111-2222-3333-4444-555555555555"; got != want {
		t.Fatalf("cfg.appTenantID = %q, want %q", got, want)
	}
	if cfg.batcher == nil {
		t.Fatal("cfg.batcher = nil, want non-nil")
	}

	status, degradation, err := runtime.determineInitialState(
		context.Background(),
		resolvedInstanceConfig{
			instanceID:        "bad-config",
			configError:       errors.New("bad config"),
			serviceURL:        mock.ServiceURL(),
			openIDMetadataURL: mock.MetadataURL(),
			tokenURL:          mock.TokenURL(),
		},
	)
	if err == nil {
		t.Fatal("determineInitialState(configError) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusDegraded; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if degradation == nil ||
		degradation.Reason != bridgepkg.BridgeDegradationReasonTenantConfigInvalid {
		t.Fatalf("degradation = %#v, want tenant config invalid", degradation)
	}

	status, degradation, err = runtime.determineInitialState(
		context.Background(),
		resolvedInstanceConfig{
			instanceID:        "missing-auth",
			serviceURL:        mock.ServiceURL(),
			openIDMetadataURL: mock.MetadataURL(),
			tokenURL:          mock.TokenURL(),
		},
	)
	if err == nil {
		t.Fatal("determineInitialState(missing auth) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if degradation == nil || degradation.Reason != bridgepkg.BridgeDegradationReasonAuthFailed {
		t.Fatalf("degradation = %#v, want auth failed", degradation)
	}

	runtime.apiFactory = func(resolvedInstanceConfig) teamsAPI {
		return &fakeTeamsAPI{validateErr: &bridgesdk.AuthError{Err: errors.New("bad token")}}
	}
	status, degradation, err = runtime.determineInitialState(
		context.Background(),
		resolvedInstanceConfig{
			instanceID:        "bad-auth",
			serviceURL:        mock.ServiceURL(),
			openIDMetadataURL: mock.MetadataURL(),
			tokenURL:          mock.TokenURL(),
			appID:             "app-id",
			appPassword:       "app-password",
		},
	)
	if err == nil {
		t.Fatal("determineInitialState(auth error) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if degradation == nil || degradation.Reason != bridgepkg.BridgeDegradationReasonAuthFailed {
		t.Fatalf("degradation = %#v, want auth failed classification", degradation)
	}

	runtime.apiFactory = func(resolvedInstanceConfig) teamsAPI {
		return &fakeTeamsAPI{}
	}
	status, degradation, err = runtime.determineInitialState(
		context.Background(),
		resolvedInstanceConfig{
			instanceID:        "ready",
			serviceURL:        mock.ServiceURL(),
			openIDMetadataURL: mock.MetadataURL(),
			tokenURL:          mock.TokenURL(),
			appID:             "app-id",
			appPassword:       "app-password",
		},
	)
	if err != nil {
		t.Fatalf("determineInitialState(ready) error = %v", err)
	}
	if got, want := status, bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if degradation != nil {
		t.Fatalf("degradation = %#v, want nil", degradation)
	}
}

func TestRuntimeInitializeStartsServerAndWritesMarkers(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mock := newTeamsProviderServer(t, teamsProviderServerConfig{})
	t.Setenv(teamsListenAddrEnv, listenAddr)
	t.Setenv(teamsServiceURLEnv, mock.ServiceURL())
	t.Setenv(teamsOpenIDMetadataURLEnv, mock.MetadataURL())
	t.Setenv(teamsOAuthTokenURLEnvName(), mock.TokenURL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 18, 20, 0, 0, time.UTC)
	managed := []subprocess.InitializeBridgeManagedInstance{
		testTeamsManagedInstance(
			t,
			now,
			"brg-1",
			map[string]any{},
		),
		testTeamsManagedInstance(
			t,
			now,
			"brg-2",
			map[string]any{},
		),
	}
	mustHandleLifecycle(t, hostPeer, managed...)

	if err := hostPeer.Call(
		context.Background(),
		"initialize",
		testInitializeRequest(now, managed...),
		nil,
	); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	handshake := waitForJSONFile[bridgesdk.InitializeMarker](t, env.HandshakePath)
	if got, want := handshake.Request.Runtime.Bridge.Provider, "teams"; got != want {
		t.Fatalf("handshake provider = %q, want %q", got, want)
	}
	ownership := waitForJSONFile[bridgesdk.OwnershipMarker](t, env.OwnershipPath)
	if got, want := len(ownership.Fetched), 2; got != want {
		t.Fatalf("len(ownership.Fetched) = %d, want %d", got, want)
	}
	states := waitForJSONLinesFile[bridgesdk.StateMarker](
		t,
		env.StatePath,
		func(items []bridgesdk.StateMarker) bool { return len(items) >= 2 },
	)
	if got, want := states[0].Status.Normalize(), bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("states[0].Status = %q, want %q", got, want)
	}
	waitForCondition(t, func() bool {
		return strings.TrimSpace(runtime.http.Address()) != ""
	})
}

func TestWebhookAuthorizationRejectsInvalidTokenAndIngestsActivities(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mock := newTeamsProviderServer(t, teamsProviderServerConfig{})
	t.Setenv(teamsListenAddrEnv, listenAddr)
	t.Setenv(teamsServiceURLEnv, mock.ServiceURL())
	t.Setenv(teamsOpenIDMetadataURLEnv, mock.MetadataURL())
	t.Setenv(teamsOAuthTokenURLEnvName(), mock.TokenURL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 18, 25, 0, 0, time.UTC)
	managed := testTeamsManagedInstance(t, now, "brg-1", map[string]any{})
	mustHandleLifecycle(t, hostPeer, managed)

	var ingested []bridgepkg.InboundMessageEnvelope
	var mu sync.Mutex
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
					GroupID:          envelope.GroupID,
					ThreadID:         envelope.ThreadID,
				},
			}, nil
		},
	)

	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}
	waitForCondition(t, func() bool {
		return strings.TrimSpace(runtime.http.Address()) != ""
	})

	serverAddr := runtime.http.Address()
	webhookURL := "http://" + serverAddr + "/teams/brg-1"

	invalidReq, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		webhookURL,
		strings.NewReader(teamsMessageWebhook(mock.ServiceURL(), "Need a summary")),
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(invalid) error = %v", err)
	}
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidReq.Header.Set("Authorization", "Bearer bad-token")
	invalidResp, err := http.DefaultClient.Do(invalidReq)
	if err != nil {
		t.Fatalf("http.DefaultClient.Do(invalid) error = %v", err)
	}
	if err := invalidResp.Body.Close(); err != nil {
		t.Errorf("invalid response body close error = %v", err)
	}
	if got, want := invalidResp.StatusCode, http.StatusUnauthorized; got != want {
		t.Fatalf("invalid webhook status = %d, want %d", got, want)
	}

	postTeamsWebhook(
		t,
		mock,
		webhookURL,
		"app-id",
		teamsMessageWebhook(mock.ServiceURL(), "Need a summary"),
	)
	postTeamsWebhook(t, mock, webhookURL, "app-id", teamsInvokeWebhook(mock.ServiceURL()))
	postTeamsWebhook(t, mock, webhookURL, "app-id", teamsReactionWebhook(mock.ServiceURL()))

	ingests := waitForJSONLinesFile[bridgesdk.IngestMarker](
		t,
		env.IngestPath,
		func(items []bridgesdk.IngestMarker) bool {
			return len(items) >= 4
		},
	)
	if got, want := ingests[0].Envelope.EventFamily, bridgepkg.InboundEventFamilyMessage; got != want {
		t.Fatalf("ingests[0].Envelope.EventFamily = %q, want %q", got, want)
	}
	families := map[bridgepkg.InboundEventFamily]int{}
	for _, item := range ingests {
		families[item.Envelope.EventFamily]++
	}
	if families[bridgepkg.InboundEventFamilyMessage] == 0 ||
		families[bridgepkg.InboundEventFamilyAction] == 0 ||
		families[bridgepkg.InboundEventFamilyReaction] == 0 {
		t.Fatalf("families = %#v, want message/action/reaction coverage", families)
	}
	mu.Lock()
	if got, want := len(ingested), len(ingests); got != want {
		t.Fatalf("len(ingested) = %d, want %d", got, want)
	}
	mu.Unlock()
}

func TestTeamsCredentialedRequestsRejectRedirects(t *testing.T) {
	enableTeamsLoopbackCredentialedURLsForTesting(t)

	t.Run("Should preserve rate-limit classification when the error body read fails", func(t *testing.T) {
		// not parallel: parent test configures credentialed URL policy with t.Setenv.
		readErr := errors.New("Teams error body read failed")
		body := &teamsPartialErrorResponseBody{
			content: []byte("Teams rate limit exceeded"),
			readErr: readErr,
		}
		client := &teamsBotClient{
			cfg: resolvedInstanceConfig{},
			httpClient: &http.Client{Transport: teamsRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header:     http.Header{"Retry-After": []string{"3"}},
					Body:       body,
					Request:    req,
				}, nil
			})},
			cachedToken: "cached-token",
			tokenExpiry: time.Now().UTC().Add(time.Hour),
		}

		err := client.callJSON(
			t.Context(),
			http.MethodGet,
			"https://service.test",
			"/v3/conversations/rate-limited",
			nil,
			nil,
			bridgesdk.HTTPResponseNoCommit,
		)
		if !errors.Is(err, readErr) {
			t.Fatalf("callJSON() error = %v, want response body read failure", err)
		}
		var rateLimitErr *bridgesdk.RateLimitError
		if !errors.As(err, &rateLimitErr) || rateLimitErr.RetryAfter != 3*time.Second {
			t.Fatalf("callJSON() error = %v, want rate limit with three-second retry", err)
		}
		if got, want := bridgesdk.ClassifyError(err).Class, bridgesdk.ErrorClassRateLimit; got != want {
			t.Fatalf("callJSON() error class = %q, want %q", got, want)
		}
		if !strings.Contains(rateLimitErr.Error(), "Teams rate limit exceeded") {
			t.Fatalf("callJSON() rate-limit error = %q, want partial provider body", rateLimitErr.Error())
		}
		if got, want := body.reads, 2; got != want {
			t.Fatalf("response body reads = %d, want %d for read plus deferred drain", got, want)
		}
		if !body.closed {
			t.Fatal("response body closed = false after error classification")
		}
	})

	t.Run("Should not follow OpenID metadata redirects", func(t *testing.T) {
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
			http.Redirect(w, r, evil.URL, http.StatusTemporaryRedirect)
		}))
		defer trusted.Close()

		_, err := fetchTeamsOpenIDMetadata(context.Background(), trusted.URL)
		var httpErr *bridgesdk.HTTPError
		if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusTemporaryRedirect {
			t.Fatalf(
				"fetchTeamsOpenIDMetadata(redirect) error = %v, want blocked redirect HTTP 307",
				err,
			)
		}

		mu.Lock()
		got := evilHits
		mu.Unlock()
		if got != 0 {
			t.Fatalf("redirect target hits = %d, want 0", got)
		}
	})

	t.Run("Should not follow JWKS redirects", func(t *testing.T) {
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
			http.Redirect(w, r, evil.URL, http.StatusTemporaryRedirect)
		}))
		defer trusted.Close()

		_, err := fetchTeamsJWKS(context.Background(), trusted.URL)
		var httpErr *bridgesdk.HTTPError
		if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusTemporaryRedirect {
			t.Fatalf("fetchTeamsJWKS(redirect) error = %v, want blocked redirect HTTP 307", err)
		}

		mu.Lock()
		got := evilHits
		mu.Unlock()
		if got != 0 {
			t.Fatalf("redirect target hits = %d, want 0", got)
		}
	})

	t.Run("Should not follow token redirects", func(t *testing.T) {
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
			http.Redirect(w, r, evil.URL, http.StatusTemporaryRedirect)
		}))
		defer trusted.Close()

		client := &teamsBotClient{
			cfg: resolvedInstanceConfig{
				tokenURL:    trusted.URL,
				appID:       "app-id",
				appPassword: "app-password",
			},
			httpClient: http.DefaultClient,
		}
		_, err := client.accessToken(context.Background())
		var httpErr *bridgesdk.HTTPError
		if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusTemporaryRedirect {
			t.Fatalf("accessToken(redirect) error = %v, want blocked redirect HTTP 307", err)
		}

		mu.Lock()
		got := evilHits
		mu.Unlock()
		if got != 0 {
			t.Fatalf("redirect target hits = %d, want 0", got)
		}
	})

	for _, testCase := range []struct {
		name       string
		statusCode int
	}{
		{name: "Should refuse a credentialed bot API 301 redirect", statusCode: http.StatusMovedPermanently},
		{name: "Should refuse a credentialed bot API 302 redirect", statusCode: http.StatusFound},
		{name: "Should refuse a credentialed bot API 303 redirect", statusCode: http.StatusSeeOther},
		{name: "Should refuse a credentialed bot API 307 redirect", statusCode: http.StatusTemporaryRedirect},
		{name: "Should refuse a credentialed bot API 308 redirect", statusCode: http.StatusPermanentRedirect},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			// not parallel: parent test configures credentialed URL policy with t.Setenv.
			var targetHits atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				targetHits.Add(1)
				if err := json.NewEncoder(w).Encode(teamsResourceResponse{ID: "activity-redirect"}); err != nil {
					t.Errorf("encode redirect target Teams response: %v", err)
				}
			}))
			defer target.Close()

			redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", target.URL)
				http.Error(w, "redirect refused", testCase.statusCode)
				if got, want := r.Method, http.MethodPost; got != want {
					t.Errorf("redirect source method = %q, want %q", got, want)
				}
			}))
			defer redirect.Close()

			client := &teamsBotClient{
				httpClient:  redirect.Client(),
				cachedToken: "cached-token",
				tokenExpiry: time.Now().UTC().Add(time.Hour),
			}
			_, err := client.SendActivity(
				t.Context(),
				redirect.URL,
				"conversation-1",
				"",
				teamsOutboundActivity{Type: "message", Text: "hello"},
			)
			var httpErr *bridgesdk.HTTPError
			if !errors.As(err, &httpErr) || httpErr.StatusCode != testCase.statusCode {
				t.Fatalf(
					"SendActivity(%d) error = %T %v, want first-response HTTPError",
					testCase.statusCode,
					err,
					err,
				)
			}
			if bridgesdk.IsCommittedMutation(err) {
				t.Fatalf("SendActivity(%d) error = %v, want pre-commit rejection", testCase.statusCode, err)
			}
			if got := targetHits.Load(); got != 0 {
				t.Fatalf("redirect target hits = %d, want 0", got)
			}
		})
	}

	t.Run("Should reject trailing JSON in an OAuth token response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if _, err := io.WriteString(w, `{"access_token":"teams-token","expires_in":3600} {}`); err != nil {
				t.Errorf("write Teams token response: %v", err)
			}
		}))
		defer server.Close()

		client := &teamsBotClient{
			cfg: resolvedInstanceConfig{
				tokenURL:    server.URL,
				appID:       "app-id",
				appPassword: "app-password",
			},
			httpClient: server.Client(),
		}
		if _, err := client.accessToken(t.Context()); err == nil {
			t.Fatal("accessToken(trailing JSON) error = nil, want strict decode failure")
		}
	})
}

func TestRuntimeDeliveriesCallTeamsAPI(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mock := newTeamsProviderServer(t, teamsProviderServerConfig{})
	t.Setenv(teamsListenAddrEnv, listenAddr)
	t.Setenv(teamsServiceURLEnv, mock.ServiceURL())
	t.Setenv(teamsOpenIDMetadataURLEnv, mock.MetadataURL())
	t.Setenv(teamsOAuthTokenURLEnvName(), mock.TokenURL())

	_, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 18, 30, 0, 0, time.UTC)
	managed := testTeamsManagedInstance(t, now, "brg-1", map[string]any{})
	mustHandleLifecycle(t, hostPeer, managed)

	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	threadID := encodeTeamsThreadID(teamsThreadRef{
		ConversationID: "19:channel@thread.tacv2;messageid=activity-parent",
		ServiceURL:     mock.ServiceURL(),
	})
	startReq := bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "delivery-1",
			BridgeInstanceID: "brg-1",
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-teams",
				BridgeInstanceID: "brg-1",
				GroupID:          "19:channel@thread.tacv2",
				ThreadID:         threadID,
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: "brg-1",
				GroupID:          "19:channel@thread.tacv2",
				ThreadID:         threadID,
				Mode:             bridgepkg.DeliveryModeReply,
			},
			Seq:       1,
			EventType: bridgepkg.DeliveryEventTypeStart,
			Content:   bridgepkg.MessageContent{Text: "hello"},
		},
	}
	var startAck bridgepkg.DeliveryAck
	if err := hostPeer.Call(context.Background(), "bridges/deliver", startReq, &startAck); err != nil {
		t.Fatalf("hostPeer.Call(start delivery) error = %v", err)
	}
	finalReq := startReq
	finalReq.Event.Seq = 2
	finalReq.Event.EventType = bridgepkg.DeliveryEventTypeFinal
	finalReq.Event.Final = true
	finalReq.Event.Content.Text = "hello world"
	var finalAck bridgepkg.DeliveryAck
	if err := hostPeer.Call(context.Background(), "bridges/deliver", finalReq, &finalAck); err != nil {
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
	if got, want := finalAck.ReplaceRemoteMessageID, startAck.RemoteMessageID; got != want {
		t.Fatalf("finalAck.ReplaceRemoteMessageID = %q, want %q", got, want)
	}
	if got, want := len(mock.APICalls()), 2; got < want {
		t.Fatalf("len(mock.APICalls()) = %d, want at least %d", got, want)
	}
}

func TestClassifyTeamsHTTPErrorAndHelpers(t *testing.T) {
	rate := classifyTeamsHTTPError(http.StatusTooManyRequests, "5", "slow down")
	var rateErr *bridgesdk.RateLimitError
	if !errors.As(rate, &rateErr) {
		t.Fatalf("classifyTeamsHTTPError(rate) = %T, want *RateLimitError", rate)
	}
	if got, want := rateErr.RetryAfter, 5*time.Second; got != want {
		t.Fatalf("rateErr.RetryAfter = %s, want %s", got, want)
	}

	auth := classifyTeamsHTTPError(http.StatusUnauthorized, "", "bad token")
	var authErr *bridgesdk.AuthError
	if !errors.As(auth, &authErr) {
		t.Fatalf("classifyTeamsHTTPError(auth) = %T, want *AuthError", auth)
	}

	serverErr := classifyTeamsHTTPError(http.StatusInternalServerError, "", "upstream failed")
	var serverHTTPError *bridgesdk.HTTPError
	if !errors.As(serverErr, &serverHTTPError) ||
		bridgesdk.ClassifyError(serverErr).Class != bridgesdk.ErrorClassServerError {
		t.Fatalf("classifyTeamsHTTPError(500) = %#v, want server_error HTTPError", serverErr)
	}

	if !looksLikeTeamsUserID("29:user-1") {
		t.Fatal("looksLikeTeamsUserID(29:user-1) = false, want true")
	}
	if looksLikeTeamsUserID("19:channel@thread.tacv2") {
		t.Fatal("looksLikeTeamsUserID(channel) = true, want false")
	}
	if !looksLikeTenantID("11111111-2222-3333-4444-555555555555") {
		t.Fatal("looksLikeTenantID(valid uuid) = false, want true")
	}
	if !validTeamsServiceURL("http://127.0.0.1:3000") {
		t.Fatal("validTeamsServiceURL(loopback http) = false, want true")
	}
	if validTeamsServiceURL("http://example.test") {
		t.Fatal("validTeamsServiceURL(http) = true, want false")
	}
	if !validTeamsCredentialedURL("https://login.botframework.com/v1/.well-known/openidconfiguration") {
		t.Fatal("validTeamsCredentialedURL(botframework) = false, want true")
	}
	if !validTeamsCredentialedURL("https://login.microsoftonline.com/botframework.com/oauth2/v2.0/token") {
		t.Fatal("validTeamsCredentialedURL(microsoftonline) = false, want true")
	}
	if validTeamsCredentialedURL("http://127.0.0.1:3000/openid") {
		t.Fatal("validTeamsCredentialedURL(loopback http without opt-in) = true, want false")
	}
	enableTeamsLoopbackCredentialedURLsForTesting(t)
	if !validTeamsCredentialedURL("http://127.0.0.1:3000/openid") {
		t.Fatal("validTeamsCredentialedURL(loopback http with opt-in) = false, want true")
	}
	if validTeamsCredentialedURL("https://evil.example/openid") {
		t.Fatal("validTeamsCredentialedURL(untrusted https host) = true, want false")
	}
	if validTeamsCredentialedURL("http://169.254.169.254/latest/meta-data") {
		t.Fatal("validTeamsCredentialedURL(link-local http) = true, want false")
	}
}

func TestProviderHelperStateAndShutdown(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)

	runtime, err := newTeamsProvider(io.Discard)
	if err != nil {
		t.Fatalf("newTeamsProvider() error = %v", err)
	}
	if err := runtime.startServer(listenAddr); err != nil {
		t.Fatalf("startServer() error = %v", err)
	}
	if err := runtime.healthCheck(); err != nil {
		t.Fatalf("healthCheck() error = %v, want nil", err)
	}

	runtime.setLastError(errors.New("boom"))
	if err := runtime.healthCheck(); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("healthCheck() error = %v, want boom", err)
	}
	runtime.clearLastError()
	if err := runtime.healthCheck(); err != nil {
		t.Fatalf("healthCheck() after clear error = %v, want nil", err)
	}

	runtime.recordTeamsProgressCleanupError(nil)
	if err := runtime.healthCheck(); err != nil {
		t.Fatalf("healthCheck() after nil progress cleanup error = %v, want nil", err)
	}
	runtime.recordTeamsProgressCleanupError(errors.New("cleanup failed"))
	if err := runtime.healthCheck(); err == nil || !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("healthCheck() progress cleanup error = %v, want cleanup failed", err)
	}
	runtime.clearLastError()

	runtime.storeUserContext("brg-1", teamsActivity{
		From:       teamsChannelAccount{ID: "29:user-1"},
		ServiceURL: "https://service.test",
		Conversation: teamsConversation{
			TenantID: "tenant-1",
		},
	})
	if ctx, ok := runtime.userContext("brg-1", "29:user-1"); !ok || ctx.TenantID != "tenant-1" {
		t.Fatalf("userContext() = (%#v, %t), want tenant-1", ctx, ok)
	}
	if _, ok := runtime.userContext("brg-1", "missing"); ok {
		t.Fatal("userContext(missing) = true, want false")
	}

	done := make(chan struct{})
	runtime.lifecycle.Go(func() {
		<-runtime.lifecycle.StopChannel()
		close(done)
	})

	if err := runtime.lifecycle.Shutdown(
		context.Background(),
		nil,
		subprocess.ShutdownRequest{DeadlineMS: 250},
	); err != nil {
		t.Fatalf("handleShutdown() error = %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not close stopCh before timeout")
	}

	waitForCondition(t, func() bool {
		data, err := os.ReadFile(env.ShutdownPath)
		return err == nil && strings.Contains(string(data), "pid=")
	})
}

func TestDispatchInboundBatchAndEnvelopeCoverage(t *testing.T) {
	env := setProviderTestEnv(t)
	_ = env
	listenAddr := reserveListenAddr(t)
	mock := newTeamsProviderServer(t, teamsProviderServerConfig{})
	t.Setenv(teamsListenAddrEnv, listenAddr)
	t.Setenv(teamsServiceURLEnv, mock.ServiceURL())
	t.Setenv(teamsOpenIDMetadataURLEnv, mock.MetadataURL())
	t.Setenv(teamsOAuthTokenURLEnvName(), mock.TokenURL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 19, 20, 0, 0, time.UTC)
	managed := testTeamsManagedInstance(t, now, "brg-1", map[string]any{})
	mustHandleLifecycle(t, hostPeer, managed)

	var ingested []bridgepkg.InboundMessageEnvelope
	var mu sync.Mutex
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
					GroupID:          envelope.GroupID,
					ThreadID:         envelope.ThreadID,
				},
			}, nil
		},
	)

	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}
	waitForCondition(t, func() bool {
		_, err := runtime.configForInstance("brg-1")
		return err == nil
	})

	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID: "brg-1",
		Scope:            bridgepkg.ScopeWorkspace,
		WorkspaceID:      "ws-teams",
		GroupID:          "19:channel@thread.tacv2",
		ThreadID: encodeTeamsThreadID(
			teamsThreadRef{
				ConversationID: "19:channel@thread.tacv2;messageid=activity-1",
				ServiceURL:     mock.ServiceURL(),
			},
		),
		PlatformMessageID: "activity-1",
		ReceivedAt:        now,
		Sender: bridgepkg.MessageSender{
			ID:          "29:user-1",
			Username:    "alice",
			DisplayName: "Alice",
		},
		Content:        bridgepkg.MessageContent{Text: "first"},
		EventFamily:    bridgepkg.InboundEventFamilyMessage,
		IdempotencyKey: "idem-1",
	}

	if err := runtime.dispatchInboundBatch(context.Background(), "brg-1", bridgesdk.InboundBatch{}); err != nil {
		t.Fatalf("dispatchInboundBatch(empty) error = %v", err)
	}
	if err := runtime.dispatchInboundBatch(context.Background(), "brg-1", bridgesdk.InboundBatch{
		Items: []bridgepkg.InboundMessageEnvelope{
			envelope,
			func() bridgepkg.InboundMessageEnvelope {
				clone := envelope
				clone.PlatformMessageID = "activity-2"
				clone.Content.Text = "second"
				clone.IdempotencyKey = "idem-2"
				return clone
			}(),
		},
	}); err != nil {
		t.Fatalf("dispatchInboundBatch(merged) error = %v", err)
	}

	waitForCondition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ingested) >= 1
	})
	mu.Lock()
	got := ingested[len(ingested)-1]
	mu.Unlock()
	if got.Content.Text != "first\nsecond" {
		t.Fatalf("merged text = %q, want %q", got.Content.Text, "first\nsecond")
	}
	if !strings.Contains(got.IdempotencyKey, ":batch:2") {
		t.Fatalf("merged idempotency key = %q, want batch suffix", got.IdempotencyKey)
	}

	uninitialized, err := newTeamsProvider(io.Discard)
	if err != nil {
		t.Fatalf("newTeamsProvider(uninitialized) error = %v", err)
	}
	if err := uninitialized.dispatchInboundEnvelope(
		context.Background(),
		"brg-1",
		envelope,
	); err == nil ||
		!strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("dispatchInboundEnvelope(uninitialized) error = %v, want not initialized", err)
	}
}

func TestBotClientCoverageAndWebhookGuards(t *testing.T) {
	t.Run("Should stop after a successful activity send returns truncated JSON", func(t *testing.T) {
		attempts := 0
		var attemptsMu sync.Mutex
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attemptsMu.Lock()
			attempts++
			attemptsMu.Unlock()
			w.WriteHeader(http.StatusOK)
			if _, err := io.WriteString(w, `{"id":`); err != nil {
				t.Errorf("write Teams response: %v", err)
			}
		}))
		defer server.Close()

		client := &teamsBotClient{
			cfg:         resolvedInstanceConfig{},
			httpClient:  server.Client(),
			cachedToken: "cached-token",
			tokenExpiry: time.Now().UTC().Add(time.Hour),
		}
		_, err := sendTeamsDeliveryActivity(
			t.Context(),
			client,
			server.URL,
			"conversation-1",
			"",
			teamsOutboundActivity{Type: "message", Text: "hello"},
		)
		if err == nil {
			t.Fatal("sendTeamsDeliveryActivity(truncated response) error = nil, want non-nil")
		}
		var committedErr *bridgesdk.CommittedMutationError
		if !errors.As(err, &committedErr) {
			t.Fatalf("sendTeamsDeliveryActivity() error = %T, want CommittedMutationError", err)
		}
		attemptsMu.Lock()
		gotAttempts := attempts
		attemptsMu.Unlock()
		if got, want := gotAttempts, 1; got != want {
			t.Fatalf("transport attempts = %d, want %d", got, want)
		}
	})

	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mock := newTeamsProviderServer(t, teamsProviderServerConfig{})
	t.Setenv(teamsListenAddrEnv, listenAddr)
	t.Setenv(teamsServiceURLEnv, mock.ServiceURL())
	t.Setenv(teamsOpenIDMetadataURLEnv, mock.MetadataURL())
	t.Setenv(teamsOAuthTokenURLEnvName(), mock.TokenURL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 19, 30, 0, 0, time.UTC)
	managed := testTeamsManagedInstance(t, now, "brg-1", map[string]any{})
	mustHandleLifecycle(t, hostPeer, managed)
	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}
	waitForCondition(t, func() bool {
		return strings.TrimSpace(runtime.http.Address()) != ""
	})

	webhookURL := fmt.Sprintf("http://%s/teams/%s", listenAddr, managed.Instance.ID)
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		webhookURL,
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(GET) error = %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http.DefaultClient.Do(GET) error = %v", err)
	}
	_, getReadErr := io.ReadAll(resp.Body)
	getCloseErr := resp.Body.Close()
	if getCloseErr != nil {
		t.Errorf("GET webhook response body close error = %v", getCloseErr)
	}
	if getReadErr != nil {
		t.Fatalf("read GET webhook response body: %v", getReadErr)
	}
	if got, want := resp.StatusCode, http.StatusMethodNotAllowed; got != want {
		t.Fatalf("GET webhook status = %d, want %d", got, want)
	}

	req, err = http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://"+listenAddr+"/unknown",
		strings.NewReader(`{}`),
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(unknown) error = %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http.DefaultClient.Do(unknown) error = %v", err)
	}
	_, unknownReadErr := io.ReadAll(resp.Body)
	unknownCloseErr := resp.Body.Close()
	if unknownCloseErr != nil {
		t.Errorf("unknown-path response body close error = %v", unknownCloseErr)
	}
	if unknownReadErr != nil {
		t.Fatalf("read unknown-path response body: %v", unknownReadErr)
	}
	if got, want := resp.StatusCode, http.StatusNotFound; got != want {
		t.Fatalf("unknown path status = %d, want %d", got, want)
	}

	cfg := resolvedInstanceConfig{
		appID:             "app-id",
		appPassword:       "app-password",
		serviceURL:        mock.ServiceURL(),
		tokenURL:          mock.TokenURL(),
		openIDMetadataURL: mock.MetadataURL(),
	}
	client := &teamsBotClient{cfg: cfg, httpClient: http.DefaultClient}
	created, err := client.CreateConversation(
		context.Background(),
		mock.ServiceURL(),
		teamsCreateConversationRequest{
			Bot:      teamsChannelAccount{ID: "28:bot"},
			IsGroup:  false,
			Members:  []teamsChannelAccount{{ID: "29:user-1"}},
			TenantID: "11111111-2222-3333-4444-555555555555",
		},
	)
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if got, want := created.ID, "a:created-conversation"; got != want {
		t.Fatalf("CreateConversation().ID = %q, want %q", got, want)
	}
	if err := client.DeleteActivity(
		context.Background(),
		mock.ServiceURL(),
		"conversation-1",
		"activity-1",
	); err != nil {
		t.Fatalf("DeleteActivity() error = %v", err)
	}

	calls := mock.APICalls()
	if len(calls) == 0 {
		t.Fatal("mock.APICalls() = 0, want recorded bot client calls")
	}
	if len(calls) < 2 {
		t.Fatalf("len(mock.APICalls()) = %d, want at least 2", len(calls))
	}

	waitForCondition(t, func() bool {
		data, err := os.ReadFile(env.StartsPath)
		return err == nil && strings.Contains(string(data), "listen=")
	})
}

func TestHandleBridgesDeliverCoverageAndRunCommand(t *testing.T) {
	env := setProviderTestEnv(t)
	_ = env
	listenAddr := reserveListenAddr(t)
	mock := newTeamsProviderServer(t, teamsProviderServerConfig{})
	t.Setenv(teamsListenAddrEnv, listenAddr)
	t.Setenv(teamsServiceURLEnv, mock.ServiceURL())
	t.Setenv(teamsOpenIDMetadataURLEnv, mock.MetadataURL())
	t.Setenv(teamsOAuthTokenURLEnvName(), mock.TokenURL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 19, 40, 0, 0, time.UTC)
	managed := testTeamsManagedInstance(t, now, "brg-1", map[string]any{})
	mustHandleLifecycle(t, hostPeer, managed)
	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}
	states := waitForJSONLinesFile[bridgesdk.StateMarker](
		t,
		env.StatePath,
		func(items []bridgesdk.StateMarker) bool {
			return len(items) >= 1 && items[len(items)-1].Status.Normalize() == bridgepkg.BridgeStatusReady
		},
	)
	if got, want := states[len(states)-1].Status.Normalize(), bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("states[last].Status = %q, want %q", got, want)
	}
	waitForCondition(t, func() bool {
		_, err := runtime.configForInstance("brg-1")
		return err == nil && runtime.currentSession() != nil
	})

	api := &fakeTeamsAPI{nextActivityID: 800}
	runtime.apiFactory = func(resolvedInstanceConfig) teamsAPI { return api }
	session := runtime.currentSession()
	if session == nil {
		t.Fatal("runtime.currentSession() = nil, want session")
	}

	threadID := encodeTeamsThreadID(teamsThreadRef{
		ConversationID: "19:channel@thread.tacv2;messageid=activity-parent",
		ServiceURL:     mock.ServiceURL(),
	})
	req := bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "delivery-1",
			BridgeInstanceID: "brg-1",
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-teams",
				BridgeInstanceID: "brg-1",
				GroupID:          "19:channel@thread.tacv2",
				ThreadID:         threadID,
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: "brg-1",
				GroupID:          "19:channel@thread.tacv2",
				ThreadID:         threadID,
				Mode:             bridgepkg.DeliveryModeReply,
			},
			Seq:       1,
			EventType: bridgepkg.DeliveryEventTypeStart,
			Content:   bridgepkg.MessageContent{Text: "hello"},
		},
	}

	ack, err := runtime.handleBridgesDeliver(context.Background(), session, req)
	if err != nil {
		t.Fatalf("handleBridgesDeliver(success) error = %v", err)
	}
	if strings.TrimSpace(ack.RemoteMessageID) == "" {
		t.Fatal("handleBridgesDeliver(success) remote message id = empty, want non-empty")
	}
	if got, want := len(api.sendCalls), 1; got != want {
		t.Fatalf("len(api.sendCalls) = %d, want %d", got, want)
	}

	badReq := req
	badReq.Event.BridgeInstanceID = "missing"
	_, err = runtime.handleBridgesDeliver(context.Background(), session, badReq)
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf(
			"handleBridgesDeliver(missing instance) error = %v, want missing instance error",
			err,
		)
	}

	if err := run(
		[]string{"unknown"},
		strings.NewReader(""),
		io.Discard,
		io.Discard,
	); err == nil ||
		!strings.Contains(err.Error(), "unsupported command") {
		t.Fatalf("run(unknown) error = %v, want unsupported command", err)
	}
}

func TestTeamsOpenIDAndAuthHelperCoverage(t *testing.T) {
	enableTeamsLoopbackCredentialedURLsForTesting(t)

	if _, err := fetchTeamsOpenIDMetadata(context.Background(), ""); err == nil {
		t.Fatal("fetchTeamsOpenIDMetadata(empty) error = nil, want non-nil")
	}

	metadataServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/metadata":
				if err := json.NewEncoder(w).
					Encode(map[string]any{"issuer": "https://api.botframework.com"}); err != nil {
					t.Errorf("encode Teams metadata response: %v", err)
				}
			case "/jwks":
				if err := json.NewEncoder(w).Encode(map[string]any{"keys": []any{}}); err != nil {
					t.Errorf("encode Teams JWKS response: %v", err)
				}
			default:
				w.WriteHeader(http.StatusNotFound)
				if err := json.NewEncoder(w).Encode(map[string]any{"error": "missing"}); err != nil {
					t.Errorf("encode missing Teams auth resource response: %v", err)
				}
			}
		}),
	)
	defer metadataServer.Close()

	if _, err := fetchTeamsOpenIDMetadata(
		context.Background(),
		metadataServer.URL+"/metadata",
	); err == nil ||
		!strings.Contains(err.Error(), "jwks_uri") {
		t.Fatalf("fetchTeamsOpenIDMetadata(missing jwks_uri) error = %v, want jwks_uri error", err)
	}
	if _, err := fetchTeamsJWKS(
		context.Background(),
		metadataServer.URL+"/jwks",
	); err == nil ||
		!strings.Contains(err.Error(), "omitted signing keys") {
		t.Fatalf("fetchTeamsJWKS(empty keys) error = %v, want empty key error", err)
	}

	keys := teamsJWKS{Keys: []teamsJWK{{Kid: "known"}}}
	if _, err := keys.keyByID("missing"); err == nil ||
		!strings.Contains(err.Error(), "not found") {
		t.Fatalf("keyByID(missing) error = %v, want not found", err)
	}
	if err := (teamsJWK{Endorsements: []string{"other"}}).validateEndorsement(
		"msteams",
	); err == nil ||
		!strings.Contains(err.Error(), "not endorsed") {
		t.Fatalf("validateEndorsement() error = %v, want endorsement error", err)
	}
	if _, err := (teamsJWK{
		Kty: "RSA",
		N:   base64.RawURLEncoding.EncodeToString([]byte{1}),
		E:   base64.RawURLEncoding.EncodeToString([]byte{0}),
	}).publicKey(); err == nil || !strings.Contains(err.Error(), "exponent") {
		t.Fatalf("publicKey(invalid exponent) error = %v, want exponent error", err)
	}

	mock := newTeamsProviderServer(t, teamsProviderServerConfig{})
	cfg := resolvedInstanceConfig{
		appID:             "app-id",
		serviceURL:        mock.ServiceURL(),
		openIDMetadataURL: mock.MetadataURL(),
	}
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://teams.test/webhook",
		strings.NewReader(teamsMessageWebhook(mock.ServiceURL(), "Need a summary")),
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() error = %v", err)
	}
	if err := verifyTeamsAuthorization(
		context.Background(),
		req,
		[]byte(teamsMessageWebhook(mock.ServiceURL(), "Need a summary")),
		cfg,
	); err == nil ||
		!strings.Contains(err.Error(), "bearer authorization") {
		t.Fatalf(
			"verifyTeamsAuthorization(missing auth) error = %v, want bearer authorization error",
			err,
		)
	}

	req, err = http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://teams.test/webhook",
		strings.NewReader(teamsMessageWebhook(mock.ServiceURL(), "Need a summary")),
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext(mismatch) error = %v", err)
	}
	req.Header.Set(
		"Authorization",
		"Bearer "+mock.SignedToken(t, "app-id", "https://elsewhere.test"),
	)
	if err := verifyTeamsAuthorization(
		context.Background(),
		req,
		[]byte(teamsMessageWebhook(mock.ServiceURL(), "Need a summary")),
		cfg,
	); err == nil ||
		!strings.Contains(err.Error(), "did not match") {
		t.Fatalf(
			"verifyTeamsAuthorization(service url mismatch) error = %v, want mismatch error",
			err,
		)
	}
}

func TestReconcileInstanceConfigCoverage(t *testing.T) {
	t.Setenv(teamsListenAddrEnv, "")

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 19, 50, 0, 0, time.UTC)
	managed := testTeamsManagedInstance(t, now, "brg-1", map[string]any{
		"webhook": map[string]any{
			"path": "shared",
		},
	})
	managed2 := testTeamsManagedInstance(t, now, "brg-2", map[string]any{
		"webhook": map[string]any{
			"path": "shared",
		},
	})
	mustHandleLifecycle(t, hostPeer, managed, managed2)
	if err := hostPeer.Call(
		context.Background(),
		"initialize",
		testInitializeRequest(now, managed, managed2),
		nil,
	); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}
	session := runtime.currentSession()
	if session == nil {
		t.Fatal("runtime.currentSession() = nil, want session")
	}

	configs := runtime.reconcileInstanceConfigs(
		context.Background(),
		session,
		[]subprocess.InitializeBridgeManagedInstance{managed, managed2},
	)
	if got, want := len(configs), 2; got != want {
		t.Fatalf("len(configs) = %d, want %d", got, want)
	}
	if configs[0].configError == nil ||
		!strings.Contains(configs[0].configError.Error(), "shared") {
		t.Fatalf("configs[0].configError = %v, want shared path error", configs[0].configError)
	}
	if configs[1].configError == nil ||
		!strings.Contains(configs[1].configError.Error(), "shared") {
		t.Fatalf(
			"configs[1].configError = %v, want shared path error",
			configs[1].configError,
		)
	}
	if _, ok := runtime.configForPath("/shared"); ok {
		t.Fatal("configForPath(/shared) = ok, want duplicate path to stay inactive")
	}

	badJSON := managed
	badJSON.Instance.ID = "bad-json"
	badJSON.Instance.ProviderConfig = json.RawMessage("{")
	resolved := runtime.resolveInstanceConfig(session, badJSON)
	if resolved.configError == nil ||
		!strings.Contains(resolved.configError.Error(), "decode provider_config") {
		t.Fatalf(
			"resolveInstanceConfig(bad json) error = %v, want decode provider_config error",
			resolved.configError,
		)
	}
}

func TestWebhookRejectsDuplicateSharedPath(t *testing.T) {
	t.Run("Should reject webhook requests when shared path is duplicated", func(t *testing.T) {
		t.Setenv(teamsListenAddrEnv, "")

		runtime, hostPeer, cleanup := newRuntimePeerPair(t)
		defer cleanup()

		now := time.Date(2026, 4, 15, 19, 55, 0, 0, time.UTC)
		managed := testTeamsManagedInstance(t, now, "brg-1", map[string]any{
			"webhook": map[string]any{
				"path": "shared",
			},
		})
		managed2 := testTeamsManagedInstance(t, now, "brg-2", map[string]any{
			"webhook": map[string]any{
				"path": "shared",
			},
		})
		managed2.Instance.WorkspaceID = "ws-other"
		mustHandleLifecycle(t, hostPeer, managed, managed2)

		if err := hostPeer.Call(
			context.Background(),
			"initialize",
			testInitializeRequest(now, managed, managed2),
			nil,
		); err != nil {
			t.Fatalf("hostPeer.Call(initialize) error = %v", err)
		}
		session := runtime.currentSession()
		if session == nil {
			t.Fatal("runtime.currentSession() = nil, want session")
		}
		configs := runtime.reconcileInstanceConfigs(
			context.Background(),
			session,
			[]subprocess.InitializeBridgeManagedInstance{managed, managed2},
		)
		if got, want := len(configs), 2; got != want {
			t.Fatalf("len(configs) = %d, want %d", got, want)
		}
		for idx, cfg := range configs {
			if got, want := cfg.initialStatus.Normalize(), bridgepkg.BridgeStatusDegraded; got != want {
				t.Fatalf("states[%d].Status = %q, want %q", idx, got, want)
			}
			if cfg.initialDegradation == nil ||
				cfg.initialDegradation.Reason != bridgepkg.BridgeDegradationReasonTenantConfigInvalid ||
				!strings.Contains(cfg.initialDegradation.Message, "shared") {
				t.Fatalf(
					"configs[%d].initialDegradation = %#v, want shared-path tenant config invalid",
					idx,
					cfg.initialDegradation,
				)
			}
		}
		req := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"http://teams.test/shared",
			strings.NewReader(teamsMessageWebhook(teamsDefaultServiceURL, "Need a summary")),
		)
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		runtime.serveWebhookHTTP(resp, req)

		result := resp.Result()
		body, err := io.ReadAll(result.Body)
		if err != nil {
			t.Fatalf("io.ReadAll(result.Body) error = %v", err)
		}
		if err := result.Body.Close(); err != nil {
			t.Fatalf("result.Body.Close() error = %v", err)
		}
		if got, want := result.StatusCode, http.StatusNotFound; got != want {
			t.Fatalf("duplicate webhook status = %d, want %d (body=%q)", got, want, string(body))
		}
	})
}

func TestRunServeCoverage(t *testing.T) {
	t.Parallel()

	if err := runServe(strings.NewReader(""), io.Discard, io.Discard); err != nil {
		t.Fatalf("runServe(empty stdin) error = %v", err)
	}
	if err := run(nil, strings.NewReader(""), io.Discard, io.Discard); err != nil {
		t.Fatalf("run(default serve) error = %v", err)
	}
}

func TestHandleWebhookRequestCoverage(t *testing.T) {
	t.Parallel()

	runtime, err := newTeamsProvider(io.Discard)
	if err != nil {
		t.Fatalf("newTeamsProvider() error = %v", err)
	}

	cfg := resolvedInstanceConfig{
		instanceID: "brg-1",
		dedup:      bridgesdk.NewDedupCache(5*time.Minute, 16),
	}

	recorder := httptest.NewRecorder()
	err = runtime.handleWebhookRequest(recorder, cfg, bridgesdk.WebhookRequest{
		Body:       []byte("{"),
		ReceivedAt: time.Now().UTC(),
	})
	var httpErr *bridgesdk.HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("handleWebhookRequest(invalid json) error = %v, want bad request http error", err)
	}

	recorder = httptest.NewRecorder()
	err = runtime.handleWebhookRequest(recorder, cfg, bridgesdk.WebhookRequest{
		Body: []byte(
			`{"type":"conversationUpdate","from":{"id":"29:user-1"},"serviceUrl":"https://service.test","conversation":{"tenantId":"tenant-1"}}`,
		),
	})
	if err != nil {
		t.Fatalf("handleWebhookRequest(ignored activity) error = %v", err)
	}
	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("handleWebhookRequest(ignored activity) status = %d, want %d", got, want)
	}
}

func TestRemoteMessageReferenceHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should encode and decode remote message references", func(t *testing.T) {
		t.Parallel()

		encoded := encodeRemoteMessageID(teamsRemoteMessageRef{
			ConversationID: "conversation-1",
			ServiceURL:     "https://service.test/",
			ActivityID:     "activity-1",
		})
		decoded, err := decodeRemoteMessageID(encoded)
		if err != nil {
			t.Fatalf("decodeRemoteMessageID(valid) error = %v", err)
		}
		if decoded.ServiceURL != "https://service.test" {
			t.Fatalf(
				"decodeRemoteMessageID().ServiceURL = %q, want %q",
				decoded.ServiceURL,
				"https://service.test",
			)
		}
		if _, err := decodeRemoteMessageID(
			base64.RawURLEncoding.EncodeToString(
				[]byte(`{"conversation_id":"","service_url":"https://service.test","activity_id":"activity-1"}`),
			),
		); err == nil {
			t.Fatal("decodeRemoteMessageID(incomplete) error = nil, want non-nil")
		}

		if got, want := referenceRemoteMessageID(
			&bridgepkg.DeliveryMessageReference{RemoteMessageID: " remote-id "},
		), "remote-id"; got != want {
			t.Fatalf("referenceRemoteMessageID() = %q, want %q", got, want)
		}
		if referenceRemoteMessageID(nil) != "" {
			t.Fatal("referenceRemoteMessageID(nil) != empty string")
		}
	})

	t.Run("Should fail closed when a referenced delivery id cannot be resolved", func(t *testing.T) {
		t.Parallel()

		reference := &bridgepkg.DeliveryMessageReference{DeliveryID: "delivery-1"}
		state := deliveryState{RemoteMessageID: "current-remote"}
		snapshot := &bridgepkg.DeliverySnapshot{RemoteMessageID: "snapshot-remote"}

		if got := resolveTeamsReferencedRemoteMessageID(reference, snapshot, state, nil); got != "" {
			t.Fatalf("resolveTeamsReferencedRemoteMessageID(nil lookup) = %q, want empty", got)
		}
		if got := resolveTeamsReferencedRemoteMessageID(reference, snapshot, state, func(string) (deliveryState, bool) {
			return deliveryState{}, false
		}); got != "" {
			t.Fatalf("resolveTeamsReferencedRemoteMessageID(missing reference) = %q, want empty", got)
		}
		if got := resolveTeamsReferencedRemoteMessageID(nil, snapshot, state, nil); got != "current-remote" {
			t.Fatalf("resolveTeamsReferencedRemoteMessageID(no reference) = %q, want current-remote", got)
		}
	})
}

type fakeTeamsAPI struct {
	createConversationID string
	nextActivityID       int
	validateErr          error
	sendErr              error
	sendErrors           []error
	sendErrorText        string
	sendEmptyActivityID  bool
	deleteErr            error
	updateErrorText      string
	updateErr            error

	createCalls  []teamsCreateConversationRequest
	sendCalls    []teamsSendCall
	sendAttempts []teamsSendCall
	updateCalls  []teamsUpdateCall
	deleteCalls  []teamsDeleteCall
}

type teamsRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f teamsRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type teamsPartialErrorResponseBody struct {
	content []byte
	readErr error
	reads   int
	closed  bool
}

func (b *teamsPartialErrorResponseBody) Read(buffer []byte) (int, error) {
	b.reads++
	if b.reads > 1 {
		return 0, io.EOF
	}
	return copy(buffer, b.content), b.readErr
}

func (b *teamsPartialErrorResponseBody) Close() error {
	b.closed = true
	return nil
}

type teamsSendCall struct {
	ServiceURL     string
	ConversationID string
	ReplyToID      string
	Activity       teamsOutboundActivity
}

type teamsUpdateCall struct {
	ServiceURL     string
	ConversationID string
	ActivityID     string
	Activity       teamsOutboundActivity
}

type teamsDeleteCall struct {
	ServiceURL     string
	ConversationID string
	ActivityID     string
}

func (f *fakeTeamsAPI) ValidateAuth(context.Context) error {
	return f.validateErr
}

func (f *fakeTeamsAPI) CreateConversation(
	_ context.Context,
	_ string,
	req teamsCreateConversationRequest,
) (*teamsConversationResourceResponse, error) {
	f.createCalls = append(f.createCalls, req)
	return &teamsConversationResourceResponse{ID: f.createConversationID}, nil
}

func (f *fakeTeamsAPI) SendActivity(
	_ context.Context,
	serviceURL string,
	conversationID string,
	replyToID string,
	activity teamsOutboundActivity,
) (*teamsResourceResponse, error) {
	call := teamsSendCall{
		ServiceURL:     serviceURL,
		ConversationID: conversationID,
		ReplyToID:      replyToID,
		Activity:       activity,
	}
	f.sendAttempts = append(f.sendAttempts, call)
	if len(f.sendErrors) > 0 {
		err := f.sendErrors[0]
		f.sendErrors = f.sendErrors[1:]
		return nil, err
	}
	if f.sendErr != nil && (f.sendErrorText == "" || activity.Text == f.sendErrorText) {
		return nil, f.sendErr
	}
	f.sendCalls = append(f.sendCalls, call)
	if f.sendEmptyActivityID {
		return &teamsResourceResponse{}, nil
	}
	id := fmt.Sprintf("activity-%d", f.nextActivityID)
	f.nextActivityID++
	return &teamsResourceResponse{ID: id}, nil
}

func countTeamsSendCalls(calls []teamsSendCall, text string) int {
	count := 0
	for _, call := range calls {
		if call.Activity.Text == text {
			count++
		}
	}
	return count
}

func testTeamsResumeRequest(request bridgepkg.DeliveryRequest) bridgepkg.DeliveryRequest {
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

func (f *fakeTeamsAPI) UpdateActivity(
	_ context.Context,
	serviceURL string,
	conversationID string,
	activityID string,
	activity teamsOutboundActivity,
) error {
	f.updateCalls = append(f.updateCalls, teamsUpdateCall{
		ServiceURL:     serviceURL,
		ConversationID: conversationID,
		ActivityID:     activityID,
		Activity:       activity,
	})
	if f.updateErr != nil && (f.updateErrorText == "" || activity.Text == f.updateErrorText) {
		return f.updateErr
	}
	return nil
}

func (f *fakeTeamsAPI) DeleteActivity(
	_ context.Context,
	serviceURL string,
	conversationID string,
	activityID string,
) error {
	f.deleteCalls = append(f.deleteCalls, teamsDeleteCall{
		ServiceURL:     serviceURL,
		ConversationID: conversationID,
		ActivityID:     activityID,
	})
	return f.deleteErr
}

type teamsProviderServerConfig struct{}

type teamsProviderServer struct {
	server     *httptest.Server
	privateKey *rsa.PrivateKey
	keyID      string

	mu       sync.Mutex
	apiCalls []teamsProviderAPICall
	nextID   int
}

type teamsProviderAPICall struct {
	Method string
	Path   string
	Body   map[string]any
}

func newTeamsProviderServer(t *testing.T, _ teamsProviderServerConfig) *teamsProviderServer {
	t.Helper()
	enableTeamsLoopbackCredentialedURLsForTesting(t)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	srv := &teamsProviderServer{privateKey: privateKey, keyID: "teams-test-key", nextID: 700}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/openid/.well-known/openidconfiguration":
			if err := json.NewEncoder(w).Encode(map[string]any{
				"issuer":   "https://api.botframework.com",
				"jwks_uri": srv.server.URL + "/openid/keys",
			}); err != nil {
				t.Errorf("encode Teams OpenID metadata response: %v", err)
			}
		case r.Method == http.MethodGet && r.URL.Path == "/openid/keys":
			pub := privateKey.PublicKey
			if err := json.NewEncoder(w).Encode(map[string]any{
				"keys": []map[string]any{{
					"kty":          "RSA",
					"kid":          srv.keyID,
					"x5t":          srv.keyID,
					"n":            base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
					"e":            base64.RawURLEncoding.EncodeToString(bigEndianExponent(pub.E)),
					"endorsements": []string{"msteams"},
				}},
			}); err != nil {
				t.Errorf("encode Teams signing keys response: %v", err)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/oauth2/v2.0/token":
			if err := json.NewEncoder(w).Encode(map[string]any{
				"token_type":   "Bearer",
				"expires_in":   3600,
				"access_token": "bot-access-token",
			}); err != nil {
				t.Errorf("encode Teams OAuth response: %v", err)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/v3/conversations":
			body := decodeJSONBody(t, r.Body)
			srv.recordCall(r.Method, r.URL.Path, body)
			if err := json.NewEncoder(w).Encode(map[string]any{"id": "a:created-conversation"}); err != nil {
				t.Errorf("encode Teams conversation response: %v", err)
			}
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/activities"):
			body := decodeJSONBody(t, r.Body)
			srv.recordCall(r.Method, r.URL.Path, body)
			srv.mu.Lock()
			id := fmt.Sprintf("activity-%d", srv.nextID)
			srv.nextID++
			srv.mu.Unlock()
			if err := json.NewEncoder(w).Encode(map[string]any{"id": id}); err != nil {
				t.Errorf("encode Teams activity response: %v", err)
			}
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/activities/"):
			srv.recordCall(r.Method, r.URL.Path, decodeJSONBody(t, r.Body))
			if err := json.NewEncoder(w).Encode(map[string]any{"id": "updated"}); err != nil {
				t.Errorf("encode Teams activity update response: %v", err)
			}
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/activities/"):
			srv.recordCall(r.Method, r.URL.Path, map[string]any{})
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(map[string]any{"error": "unknown path"}); err != nil {
				t.Errorf("encode unknown Teams path response: %v", err)
			}
		}
	}))
	srv.server = server
	t.Cleanup(server.Close)
	return srv
}

func (s *teamsProviderServer) recordCall(method string, path string, body map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apiCalls = append(s.apiCalls, teamsProviderAPICall{Method: method, Path: path, Body: body})
}

func (s *teamsProviderServer) ServiceURL() string {
	return s.server.URL
}

func (s *teamsProviderServer) MetadataURL() string {
	return s.server.URL + "/openid/.well-known/openidconfiguration"
}

func (s *teamsProviderServer) TokenURL() string {
	return s.server.URL + "/oauth2/v2.0/token"
}

func (s *teamsProviderServer) APICalls() []teamsProviderAPICall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]teamsProviderAPICall, len(s.apiCalls))
	copy(out, s.apiCalls)
	return out
}

func (s *teamsProviderServer) SignedToken(t *testing.T, appID string, serviceURL string) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, teamsAuthClaims{
		ServiceURL: serviceURL,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://api.botframework.com",
			Audience:  []string{appID},
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Hour)),
			NotBefore: jwt.NewNumericDate(time.Now().UTC().Add(-time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		},
	})
	token.Header["kid"] = s.keyID
	signed, err := token.SignedString(s.privateKey)
	if err != nil {
		t.Fatalf("token.SignedString() error = %v", err)
	}
	return signed
}

func decodeJSONBody(t *testing.T, body io.ReadCloser) map[string]any {
	t.Helper()
	defer func() {
		if err := body.Close(); err != nil {
			t.Errorf("Teams request body close error = %v", err)
		}
	}()
	out := map[string]any{}
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		t.Errorf("decode Teams request body: %v", err)
	}
	return out
}

func bigEndianExponent(e int) []byte {
	if e == 0 {
		return []byte{0}
	}
	buf := make([]byte, 0, 4)
	for e > 0 {
		buf = append([]byte{byte(e & 0xff)}, buf...)
		e >>= 8
	}
	return buf
}

func postTeamsWebhook(
	t *testing.T,
	server *teamsProviderServer,
	webhookURL string,
	appID string,
	payload string,
) {
	t.Helper()

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		webhookURL,
		strings.NewReader(payload),
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+server.SignedToken(t, appID, server.ServiceURL()))

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		_, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if closeErr != nil {
			t.Errorf("Teams webhook response body close error = %v", closeErr)
		}
		if readErr != nil {
			t.Fatalf("read Teams webhook response body: %v", readErr)
		}
		if resp.StatusCode == http.StatusOK {
			return
		}
		if resp.StatusCode == http.StatusNotFound ||
			resp.StatusCode == http.StatusServiceUnavailable {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		t.Fatalf("webhook status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	t.Fatalf("webhook %s did not become ready before timeout", webhookURL)
}

func teamsMessageWebhook(serviceURL string, text string) string {
	return fmt.Sprintf(
		`{"type":"message","id":"activity-1","channelId":"msteams","serviceUrl":%q,"timestamp":"2026-04-15T18:25:00Z","text":%q,"from":{"id":"29:user-1","name":"Alice Example"},"recipient":{"id":"28:bot","name":"Bridge Bot"},"conversation":{"id":"19:channel@thread.tacv2;messageid=activity-1","conversationType":"channel","tenantId":"11111111-2222-3333-4444-555555555555"}}`,
		serviceURL,
		text,
	)
}

func teamsInvokeWebhook(serviceURL string) string {
	return fmt.Sprintf(
		`{"type":"invoke","id":"activity-2","channelId":"msteams","serviceUrl":%q,"timestamp":"2026-04-15T18:25:01Z","from":{"id":"29:user-1","name":"Alice Example"},"recipient":{"id":"28:bot","name":"Bridge Bot"},"conversation":{"id":"a:direct-conversation","tenantId":"11111111-2222-3333-4444-555555555555"},"value":{"action":{"data":{"actionId":"approve","value":"yes"}}}}`,
		serviceURL,
	)
}

func teamsReactionWebhook(serviceURL string) string {
	return fmt.Sprintf(
		`{"type":"messageReaction","id":"activity-3","channelId":"msteams","serviceUrl":%q,"timestamp":"2026-04-15T18:25:02Z","from":{"id":"29:user-1","name":"Alice Example"},"conversation":{"id":"19:channel@thread.tacv2;messageid=activity-1","conversationType":"channel","tenantId":"11111111-2222-3333-4444-555555555555"},"reactionsAdded":[{"type":"like"}],"reactionsRemoved":[{"type":"sad"}]}`,
		serviceURL,
	)
}

func newRuntimePeerPair(t *testing.T) (*teamsProvider, *bridgesdk.Peer, func()) {
	t.Helper()

	hostConn, runtimeConn := net.Pipe()
	runtime, err := newTeamsProvider(io.Discard)
	if err != nil {
		t.Fatalf("newTeamsProvider() error = %v", err)
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

func mustHandleLifecycle(
	t *testing.T,
	peer *bridgesdk.Peer,
	managed ...subprocess.InitializeBridgeManagedInstance,
) {
	t.Helper()

	mustHandle(
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
	mustHandle(
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
	mustHandle(
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

func setTeamsProviderRoute(
	provider *teamsProvider,
	instanceID string,
	config resolvedInstanceConfig,
) {
	provider.routes.Replace(map[string]resolvedInstanceConfig{instanceID: config}, nil)
}

func testTeamsManagedInstance(
	t *testing.T,
	now time.Time,
	instanceID string,
	providerConfig map[string]any,
) subprocess.InitializeBridgeManagedInstance {
	encodedProviderConfig, err := json.Marshal(providerConfig)
	if err != nil {
		t.Fatalf("json.Marshal(providerConfig) error = %v", err)
	}
	return subprocess.InitializeBridgeManagedInstance{
		Instance: bridgepkg.BridgeInstance{
			ID:            instanceID,
			Scope:         bridgepkg.ScopeWorkspace,
			WorkspaceID:   "ws-teams",
			Platform:      "teams",
			ExtensionName: "teams",
			DisplayName:   "Teams",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{
				IncludePeer:   true,
				IncludeGroup:  true,
				IncludeThread: true,
			},
			ProviderConfig: encodedProviderConfig,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
			{BindingName: "app_id", Kind: "token", Value: "app-id"},
			{BindingName: "app_password", Kind: "token", Value: "app-password"},
			{
				BindingName: "app_tenant_id",
				Kind:        "token",
				Value:       "11111111-2222-3333-4444-555555555555",
			},
		},
	}
}

func testTeamsProgressRequest(
	instanceID string,
	deliveryID string,
	seq int64,
	toolCallID string,
	toolID string,
	label string,
) bridgepkg.DeliveryRequest {
	threadID := encodeTeamsThreadID(teamsThreadRef{
		ConversationID: "19:channel@thread.tacv2;messageid=activity-parent",
		ServiceURL:     "https://service.test",
	})
	return bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
		DeliveryID:       deliveryID,
		BridgeInstanceID: instanceID,
		RoutingKey: bridgepkg.RoutingKey{
			Scope:            bridgepkg.ScopeWorkspace,
			WorkspaceID:      "ws-teams",
			BridgeInstanceID: instanceID,
			GroupID:          "19:channel@thread.tacv2",
			ThreadID:         threadID,
		},
		DeliveryTarget: bridgepkg.DeliveryTarget{
			BridgeInstanceID: instanceID,
			GroupID:          "19:channel@thread.tacv2",
			ThreadID:         threadID,
			Mode:             bridgepkg.DeliveryModeReply,
		},
		Seq:       seq,
		EventType: bridgepkg.DeliveryEventTypeProgress,
		Progress: &bridgepkg.ToolProgress{
			ToolCallID: toolCallID,
			ToolID:     toolID,
			Phase:      bridgepkg.ToolProgressPhaseStarted,
			Label:      label,
			Index:      int(seq),
		},
	}}
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
			Name:       "teams",
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
				Provider:         "teams",
				Platform:         "teams",
				ManagedInstances: managed,
			},
		},
	}
}

func TestTeamsResolvedConfigHealthAndListenerHelpers(t *testing.T) {
	t.Run("Should classify each invalid provider endpoint field", func(t *testing.T) {
		t.Parallel()

		valid := resolvedInstanceConfig{
			webhookPath:       "/teams",
			serviceURL:        teamsDefaultServiceURL,
			openIDMetadataURL: teamsDefaultOpenIDMetadata,
			tokenURL:          "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		}
		cases := []struct {
			name string
			edit func(*resolvedInstanceConfig)
			want string
		}{
			{
				name: "webhook path",
				edit: func(cfg *resolvedInstanceConfig) { cfg.webhookPath = "" },
				want: "webhook path",
			},
			{name: "service URL", edit: func(cfg *resolvedInstanceConfig) { cfg.serviceURL = "" }, want: "service url"},
			{
				name: "invalid service URL",
				edit: func(cfg *resolvedInstanceConfig) { cfg.serviceURL = "ftp://invalid" },
				want: "service url",
			},
			{
				name: "metadata URL",
				edit: func(cfg *resolvedInstanceConfig) { cfg.openIDMetadataURL = "" },
				want: "openid metadata",
			},
			{
				name: "invalid metadata URL",
				edit: func(cfg *resolvedInstanceConfig) { cfg.openIDMetadataURL = "ftp://invalid" },
				want: "openid metadata",
			},
			{
				name: "token URL",
				edit: func(cfg *resolvedInstanceConfig) { cfg.tokenURL = "" },
				want: "token url",
			},
			{
				name: "invalid token URL",
				edit: func(cfg *resolvedInstanceConfig) { cfg.tokenURL = "ftp://invalid" },
				want: "token url",
			},
			{
				name: "tenant ID",
				edit: func(cfg *resolvedInstanceConfig) { cfg.appTenantID = "malformed" },
				want: "app_tenant_id",
			},
		}
		for _, test := range cases {
			t.Run("Should reject "+test.name, func(t *testing.T) {
				t.Parallel()
				cfg := valid
				test.edit(&cfg)
				validateTeamsResolvedConfig(&cfg)
				if cfg.configError == nil || !strings.Contains(cfg.configError.Error(), test.want) {
					t.Fatalf("config error = %v, want %q", cfg.configError, test.want)
				}
			})
		}
	})

	t.Run("Should expose degradation health text and release a listener", func(t *testing.T) {
		t.Parallel()

		validateTeamsResolvedConfig(nil)
		if got := bridgeHealthMessage(nil); got != "" {
			t.Fatalf("bridgeHealthMessage(nil) = %q", got)
		}
		if got, want := bridgeHealthMessage(&bridgepkg.BridgeDegradation{
			Message: "  explicit health message  ",
		}), "explicit health message"; got != want {
			t.Fatalf("bridgeHealthMessage(message) = %q, want %q", got, want)
		}
		if got, want := bridgeHealthMessage(&bridgepkg.BridgeDegradation{
			Reason: bridgepkg.BridgeDegradationReasonAuthFailed,
		}), string(bridgepkg.BridgeDegradationReasonAuthFailed); got != want {
			t.Fatalf("bridgeHealthMessage(reason) = %q, want %q", got, want)
		}
		listener, err := listenTeamsWebhook("127.0.0.1:0")
		if err != nil {
			t.Fatalf("listenTeamsWebhook() error = %v", err)
		}
		if err := listener.Close(); err != nil {
			t.Fatalf("listener.Close() error = %v", err)
		}
		fallbackListener, err := listenTeamsWebhook("")
		if err != nil {
			t.Fatalf("listenTeamsWebhook(empty) error = %v", err)
		}
		if err := fallbackListener.Close(); err != nil {
			t.Fatalf("fallback listener.Close() error = %v", err)
		}
		if _, err := refreshTeamsJWKS(context.Background(), "ftp://invalid"); err == nil {
			t.Fatal("refreshTeamsJWKS(invalid URL) error = nil")
		}
		if got, want := defaultTeamsTokenURL(""),
			"https://login.microsoftonline.com/botframework.com/oauth2/v2.0/token"; got != want {
			t.Fatalf("defaultTeamsTokenURL(empty) = %q, want %q", got, want)
		}
	})
}

func setProviderTestEnv(t *testing.T) bridgesdk.AdapterMarkerPaths {
	t.Helper()
	enableTeamsLoopbackCredentialedURLsForTesting(t)

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

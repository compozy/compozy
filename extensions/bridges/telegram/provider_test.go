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
	"sync/atomic"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/testutil"
)

func TestMapTelegramUpdateDirectAndForumRouting(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-telegram")

	direct, err := mapTelegramUpdate(telegramUpdate{
		UpdateID: 10,
		Message: &telegramMessage{
			MessageID:       111,
			MessageThreadID: 77,
			Date:            now.Unix(),
			Chat: telegramChat{
				ID:   42,
				Type: "private",
			},
			From: telegramUser{
				ID:        7,
				Username:  "alice",
				FirstName: "Alice",
				LastName:  "Example",
			},
			Text: " hello ",
		},
	}, managed, now, nil)
	if err != nil {
		t.Fatalf("mapTelegramUpdate(direct) error = %v", err)
	}
	if got, want := direct.PeerID, "42"; got != want {
		t.Fatalf("direct.PeerID = %q, want %q", got, want)
	}
	if got := direct.GroupID; got != "" {
		t.Fatalf("direct.GroupID = %q, want empty", got)
	}
	if got, want := direct.ThreadID, "77"; got != want {
		t.Fatalf("direct.ThreadID = %q, want %q", got, want)
	}

	forum, err := mapTelegramUpdate(telegramUpdate{
		UpdateID: 11,
		Message: &telegramMessage{
			MessageID: 222,
			Date:      now.Unix(),
			Chat: telegramChat{
				ID:      -100123,
				Type:    "supergroup",
				IsForum: true,
				Title:   "ops",
			},
			From: telegramUser{
				ID:       8,
				Username: "bob",
			},
			Caption: "  from forum  ",
		},
	}, managed, now, nil)
	if err != nil {
		t.Fatalf("mapTelegramUpdate(forum) error = %v", err)
	}
	if got := forum.PeerID; got != "" {
		t.Fatalf("forum.PeerID = %q, want empty", got)
	}
	if got, want := forum.GroupID, "-100123"; got != want {
		t.Fatalf("forum.GroupID = %q, want %q", got, want)
	}
	if got, want := forum.ThreadID, telegramGeneralTopicID; got != want {
		t.Fatalf("forum.ThreadID = %q, want %q", got, want)
	}
	if got, want := forum.Content.Text, "from forum"; got != want {
		t.Fatalf("forum.Content.Text = %q, want %q", got, want)
	}
}

func TestMapTelegramEditsAndReplyContext(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 12, 16, 0, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-telegram-edits")
	managed.Instance.Scope = bridgepkg.ScopeWorkspace
	managed.Instance.WorkspaceID = "ws-a"

	t.Run("Should map edited messages and channel posts to typed edits", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name   string
			update telegramUpdate
		}{
			{
				name: "Should map an edited direct message",
				update: telegramUpdate{
					UpdateID: 101,
					EditedMessage: &telegramMessage{
						MessageID: 9001,
						Date:      now.Unix(),
						EditDate:  now.Add(time.Minute).Unix(),
						Chat:      telegramChat{ID: 42, Type: "private"},
						From:      telegramUser{ID: 7, Username: "alice", FirstName: "Alice"},
						Text:      "corrected direct message",
					},
				},
			},
			{
				name: "Should map an edited channel post",
				update: telegramUpdate{
					UpdateID: 102,
					EditedChannelPost: &telegramMessage{
						MessageID: 9002,
						Date:      now.Unix(),
						EditDate:  now.Add(2 * time.Minute).Unix(),
						Chat:      telegramChat{ID: -100123, Type: "channel", Title: "AGH Updates"},
						SenderChat: &telegramChat{
							ID:       -100456,
							Type:     "channel",
							Title:    "AGH Updates",
							Username: "agh_updates",
						},
						Caption: "corrected channel post",
					},
				},
			},
		}
		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				envelope, err := mapTelegramUpdate(
					testCase.update,
					managed,
					now.Add(3*time.Minute),
					bridgesdk.NewParentMessageCache(0),
				)
				if err != nil {
					t.Fatalf("mapTelegramUpdate() error = %v", err)
				}
				message := selectTelegramMessage(testCase.update)
				if message == nil {
					t.Fatal("selectTelegramMessage() = nil, want edited message")
				}
				if got, want := envelope.EventFamily, bridgepkg.InboundEventFamilyEdit; got != want {
					t.Fatalf("EventFamily = %q, want %q", got, want)
				}
				if envelope.Edit == nil {
					t.Fatal("Edit = nil, want typed payload")
				}
				if got, want := envelope.Edit.MessageID, fmt.Sprint(message.MessageID); got != want {
					t.Fatalf("Edit.MessageID = %q, want %q", got, want)
				}
				if got, want := envelope.Edit.Operation, bridgepkg.InboundEditOperationUpdated; got != want {
					t.Fatalf("Edit.Operation = %q, want %q", got, want)
				}
				if envelope.Edit.NewText == "" {
					t.Fatal("Edit.NewText = empty, want corrected text")
				}
				if got, want := envelope.Edit.OriginalTimestamp, time.Unix(message.Date, 0).UTC(); !got.Equal(want) {
					t.Fatalf("Edit.OriginalTimestamp = %s, want %s", got, want)
				}
				if shouldBatchTelegramInbound(testCase.update) {
					t.Fatal("shouldBatchTelegramInbound(edit) = true, want false")
				}
				if testCase.update.EditedChannelPost != nil {
					if got, want := envelope.Sender.ID, "-100456"; got != want {
						t.Fatalf("channel edit sender id = %q, want %q", got, want)
					}
					if got, want := envelope.Sender.DisplayName, "AGH Updates"; got != want {
						t.Fatalf("channel edit sender display name = %q, want %q", got, want)
					}
				}
			})
		}
	})

	t.Run("Should fill reply context from the scoped parent cache and degrade on miss", func(t *testing.T) {
		t.Parallel()

		parents := bridgesdk.NewParentMessageCache(0)
		root := telegramUpdate{
			UpdateID: 201,
			Message: &telegramMessage{
				MessageID: 8001,
				Date:      now.Unix(),
				Chat:      telegramChat{ID: -100123, Type: "supergroup"},
				From: telegramUser{
					ID:        7,
					Username:  "bob",
					FirstName: "Bob",
					LastName:  "Example",
				},
				Text: "original proposal",
			},
		}
		rootEnvelope, err := mapTelegramUpdate(root, managed, now, parents)
		if err != nil {
			t.Fatalf("mapTelegramUpdate(root) error = %v", err)
		}
		parents.RememberEnvelope(rootEnvelope)

		reply := telegramUpdate{
			UpdateID: 202,
			Message: &telegramMessage{
				MessageID: 8002,
				Date:      now.Add(time.Minute).Unix(),
				Chat:      telegramChat{ID: -100123, Type: "supergroup"},
				From:      telegramUser{ID: 8, Username: "alice", FirstName: "Alice"},
				Text:      "I agree",
				ReplyToMessage: &telegramMessage{
					MessageID: 8001,
				},
			},
		}
		envelope, err := mapTelegramUpdate(reply, managed, now.Add(time.Minute), parents)
		if err != nil {
			t.Fatalf("mapTelegramUpdate(reply) error = %v", err)
		}
		if envelope.ReplyToText != "original proposal" ||
			envelope.ReplyToAuthorID != "7" ||
			envelope.ReplyToAuthorName != "Bob Example" {
			t.Fatalf(
				"reply context = (%q, %q, %q), want cached parent",
				envelope.ReplyToText,
				envelope.ReplyToAuthorID,
				envelope.ReplyToAuthorName,
			)
		}
		if shouldBatchTelegramInbound(reply) {
			t.Fatal("shouldBatchTelegramInbound(reply) = true, want false")
		}

		inlineReply := reply
		inlineMessage := *reply.Message
		inlineMessage.ReplyToMessage = &telegramMessage{
			MessageID: 8001,
			From: telegramUser{
				ID:        7,
				Username:  "bob",
				FirstName: "Bob",
				LastName:  "Example",
			},
			Text: "provider snapshot proposal",
		}
		inlineReply.Message = &inlineMessage
		inline, err := mapTelegramUpdate(
			inlineReply,
			managed,
			now.Add(time.Minute),
			bridgesdk.NewParentMessageCache(0),
		)
		if err != nil {
			t.Fatalf("mapTelegramUpdate(inline reply) error = %v", err)
		}
		if inline.ReplyToText != "provider snapshot proposal" ||
			inline.ReplyToAuthorID != "7" ||
			inline.ReplyToAuthorName != "Bob Example" {
			t.Fatalf(
				"inline reply context = (%q, %q, %q), want provider snapshot",
				inline.ReplyToText,
				inline.ReplyToAuthorID,
				inline.ReplyToAuthorName,
			)
		}

		cold, err := mapTelegramUpdate(
			reply,
			managed,
			now.Add(time.Minute),
			bridgesdk.NewParentMessageCache(0),
		)
		if err != nil {
			t.Fatalf("mapTelegramUpdate(cold reply) error = %v", err)
		}
		if cold.ReplyToText != "" || cold.ReplyToAuthorID != "" || cold.ReplyToAuthorName != "" {
			t.Fatalf("cold reply context = %#v, want empty cache-miss fields", cold)
		}
	})
}

func TestAllowDirectMessagePolicies(t *testing.T) {
	t.Parallel()

	message := telegramMessage{
		Chat: telegramChat{Type: "private"},
		From: telegramUser{ID: 42, Username: "alice"},
	}

	if !allowDirectMessage(&resolvedInstanceConfig{dmPolicy: bridgepkg.BridgeDMPolicyOpen}, message) {
		t.Fatal("allowDirectMessage(open) = false, want true")
	}

	allowlist := resolvedInstanceConfig{
		dmPolicy:       bridgepkg.BridgeDMPolicyAllowlist,
		allowUserIDs:   map[string]struct{}{"42": {}},
		allowUsernames: map[string]struct{}{"bob": {}},
	}
	if !allowDirectMessage(&allowlist, message) {
		t.Fatal("allowDirectMessage(allowlist by id) = false, want true")
	}

	pairing := resolvedInstanceConfig{
		dmPolicy:        bridgepkg.BridgeDMPolicyPairing,
		pairedUsernames: map[string]struct{}{"alice": {}},
	}
	if !allowDirectMessage(&pairing, message) {
		t.Fatal("allowDirectMessage(pairing by username) = false, want true")
	}

	rejected := resolvedInstanceConfig{
		dmPolicy: bridgepkg.BridgeDMPolicyAllowlist,
	}
	if allowDirectMessage(&rejected, message) {
		t.Fatal("allowDirectMessage(rejected) = true, want false")
	}
}

func TestExecuteDeliveryPostEditDeleteAndResume(t *testing.T) {
	t.Parallel()

	api := &fakeTelegramAPI{nextMessageID: 500}

	startReq := testDeliveryRequest(
		"brg-1",
		"delivery-1",
		1,
		bridgepkg.DeliveryEventTypeStart,
		false,
	)
	startAck, state, err := executeDelivery(
		context.Background(),
		api,
		startReq,
		deliveryState{},
	)
	if err != nil {
		t.Fatalf("executeDelivery(start) error = %v", err)
	}
	if got, want := startAck.RemoteMessageID, "peer-1:500"; got != want {
		t.Fatalf("startAck.RemoteMessageID = %q, want %q", got, want)
	}

	finalReq := testDeliveryRequest(
		"brg-1",
		"delivery-1",
		2,
		bridgepkg.DeliveryEventTypeFinal,
		true,
	)
	finalReq.Event.Content.Text = "hello world"
	finalAck, state, err := executeDelivery(context.Background(), api, finalReq, state)
	if err != nil {
		t.Fatalf("executeDelivery(final) error = %v", err)
	}
	if got, want := finalAck.ReplaceRemoteMessageID, startAck.RemoteMessageID; got != want {
		t.Fatalf("finalAck.ReplaceRemoteMessageID = %q, want %q", got, want)
	}

	finalNoOpReq := testDeliveryRequest(
		"brg-1",
		"delivery-1",
		3,
		bridgepkg.DeliveryEventTypeFinal,
		true,
	)
	finalNoOpReq.Event.Content.Text = "hello world"
	finalNoOpAck, state, err := executeDelivery(context.Background(), api, finalNoOpReq, state)
	if err != nil {
		t.Fatalf("executeDelivery(final no-op) error = %v", err)
	}
	if got, want := finalNoOpAck.RemoteMessageID, finalAck.RemoteMessageID; got != want {
		t.Fatalf("finalNoOpAck.RemoteMessageID = %q, want %q", got, want)
	}
	if got, want := finalNoOpAck.ReplaceRemoteMessageID, finalAck.RemoteMessageID; got != want {
		t.Fatalf("finalNoOpAck.ReplaceRemoteMessageID = %q, want %q", got, want)
	}

	deleteReq := testDeleteRequest("brg-1", "delivery-1", 4, finalNoOpAck.RemoteMessageID)
	deleteAck, _, err := executeDelivery(context.Background(), api, deleteReq, state)
	if err != nil {
		t.Fatalf("executeDelivery(delete) error = %v", err)
	}
	if got, want := deleteAck.RemoteMessageID, finalNoOpAck.RemoteMessageID; got != want {
		t.Fatalf("deleteAck.RemoteMessageID = %q, want %q", got, want)
	}
	if got, want := strings.Join(api.methods, ","), "sendMessage,editMessageText,deleteMessage"; got != want {
		t.Fatalf("api methods = %q, want %q", got, want)
	}

	resumeAPI := &fakeTelegramAPI{nextMessageID: 900}
	resumeReq := testDeliveryRequest(
		"brg-1",
		"delivery-2",
		1,
		bridgepkg.DeliveryEventTypeResume,
		false,
	)
	resumeReq.Event.Resume = &bridgepkg.DeliveryResumeState{
		LatestEventType: bridgepkg.DeliveryEventTypeFinal,
	}
	resumeReq.Snapshot = &bridgepkg.DeliverySnapshot{
		DeliveryID:       "delivery-2",
		SessionID:        "sess-1",
		TurnID:           "turn-1",
		BridgeInstanceID: "brg-1",
		RoutingKey:       resumeReq.Event.RoutingKey,
		DeliveryTarget:   resumeReq.Event.DeliveryTarget,
		LatestSeq:        1,
		LatestEventType:  bridgepkg.DeliveryEventTypeFinal,
		CurrentContent:   bridgepkg.MessageContent{Text: "hello"},
		Final:            true,
		UpdatedAt:        time.Date(2026, 4, 15, 12, 5, 0, 0, time.UTC),
	}
	resumeAck, _, err := executeDelivery(
		context.Background(),
		resumeAPI,
		resumeReq,
		deliveryState{},
	)
	if err != nil {
		t.Fatalf("executeDelivery(resume without remote) error = %v", err)
	}
	if got, want := resumeAck.RemoteMessageID, "peer-1:900"; got != want {
		t.Fatalf("resumeAck.RemoteMessageID = %q, want %q", got, want)
	}

	resumeEditAPI := &fakeTelegramAPI{}
	resumeEditReq := testDeliveryRequest(
		"brg-1",
		"delivery-3",
		2,
		bridgepkg.DeliveryEventTypeResume,
		true,
	)
	resumeEditReq.Event.Content.Text = "hello world"
	resumeEditReq.Event.Resume = &bridgepkg.DeliveryResumeState{
		LatestEventType: bridgepkg.DeliveryEventTypeFinal,
	}
	resumeEditReq.Snapshot = &bridgepkg.DeliverySnapshot{
		DeliveryID:       "delivery-3",
		SessionID:        "sess-1",
		TurnID:           "turn-2",
		BridgeInstanceID: "brg-1",
		RoutingKey:       resumeEditReq.Event.RoutingKey,
		DeliveryTarget:   resumeEditReq.Event.DeliveryTarget,
		LatestSeq:        2,
		LatestEventType:  bridgepkg.DeliveryEventTypeFinal,
		CurrentContent:   bridgepkg.MessageContent{Text: "hello world"},
		LastSentSeq:      2,
		LastAckedSeq:     1,
		RemoteMessageID:  "peer-1:500",
		Final:            true,
		UpdatedAt:        time.Date(2026, 4, 15, 12, 6, 0, 0, time.UTC),
	}
	resumeEditAck, resumeEditState, err := executeDelivery(
		context.Background(),
		resumeEditAPI,
		resumeEditReq,
		deliveryState{},
	)
	if err != nil {
		t.Fatalf("executeDelivery(resume stale remote) error = %v", err)
	}
	if got, want := strings.Join(resumeEditAPI.methods, ","), "editMessageText"; got != want {
		t.Fatalf("resume edit methods = %q, want %q", got, want)
	}
	if got, want := resumeEditAck.RemoteMessageID, "peer-1:500"; got != want {
		t.Fatalf("resumeEditAck.RemoteMessageID = %q, want %q", got, want)
	}
	if got, want := resumeEditState.LastContent, "hello world"; got != want {
		t.Fatalf("resumeEditState.LastContent = %q, want %q", got, want)
	}

	t.Run("Should route edits through their explicit remote reference", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			state deliveryState
		}{
			{
				name: "Should edit the referenced message without prior state",
			},
			{
				name: "Should prefer the explicit reference over stored state",
				state: deliveryState{
					LastSeq:         1,
					LastContent:     "old",
					RemoteMessageID: encodeRemoteMessageID("peer-1", 700),
				},
			},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				api := &fakeTelegramAPI{nextMessageID: 900}
				request := testDeliveryRequest(
					"brg-1",
					"delivery-reference-edit",
					testCase.state.LastSeq+1,
					bridgepkg.DeliveryEventTypeDelta,
					false,
				)
				request.Event.Operation = bridgepkg.DeliveryOperationEdit
				request.Event.Reference = &bridgepkg.DeliveryMessageReference{
					RemoteMessageID: encodeRemoteMessageID("peer-1", 777),
				}
				request.Event.Content.Text = "updated"

				ack, gotState, err := executeDelivery(context.Background(), api, request, testCase.state)
				if err != nil {
					t.Fatalf("executeDelivery() error = %v", err)
				}
				if got, want := strings.Join(api.methods, ","), "editMessageText"; got != want {
					t.Fatalf("api methods = %q, want %q", got, want)
				}
				if got, want := api.editRequests[0].MessageID, int64(777); got != want {
					t.Fatalf("edited message id = %d, want %d", got, want)
				}
				if got, want := ack.RemoteMessageID, encodeRemoteMessageID("peer-1", 777); got != want {
					t.Fatalf("ack remote id = %q, want %q", got, want)
				}
				if got, want := gotState.RemoteMessageID, ack.RemoteMessageID; got != want {
					t.Fatalf("state remote id = %q, want %q", got, want)
				}
			})
		}
	})

	t.Run("Should reject malformed successful sends without advancing state", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name                string
			content             string
			sendResults         []*telegramSentMessage
			wantCompletedChunks int
		}{
			{
				name:        "Should reject a nil send response",
				content:     "hello",
				sendResults: []*telegramSentMessage{nil},
			},
			{
				name:        "Should reject a zero message id",
				content:     "hello",
				sendResults: []*telegramSentMessage{{MessageID: 0}},
			},
			{
				name:        "Should reject a negative message id",
				content:     "hello",
				sendResults: []*telegramSentMessage{{MessageID: -1}},
			},
			{
				name:                "Should reject a non-positive continuation message id",
				content:             strings.Repeat("continuation ", telegramMaxMessageLen),
				wantCompletedChunks: 1,
				sendResults: []*telegramSentMessage{
					{MessageID: 901},
					{MessageID: 0},
				},
			},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				api := &fakeTelegramAPI{sendResults: testCase.sendResults}
				request := testDeliveryRequest(
					"brg-1",
					"delivery-malformed-send",
					1,
					bridgepkg.DeliveryEventTypeFinal,
					true,
				)
				request.Event.Content.Text = testCase.content

				_, gotState, err := executeDelivery(context.Background(), api, request, deliveryState{})
				var committedErr *bridgesdk.CommittedMutationError
				if !errors.As(err, &committedErr) {
					t.Fatalf("executeDelivery() error = %T %v, want CommittedMutationError", err, err)
				}
				if gotState.LastSeq != 0 || gotState.RemoteMessageID != "" || gotState.ReplaceRemoteMessageID != "" {
					t.Fatalf("executeDelivery() committed state = %#v, want no final acknowledgement", gotState)
				}
				if got, want := gotState.Chunks.NextChunk(), testCase.wantCompletedChunks; got != want {
					t.Fatalf("completed chunk prefix = %d, want %d", got, want)
				}
				if got, want := len(api.sendRequests), testCase.wantCompletedChunks+1; got != want {
					t.Fatalf("send attempts = %d, want %d", got, want)
				}
			})
		}
	})

	t.Run("Should honor Retry-After for edit and delete operations", func(t *testing.T) {
		t.Parallel()

		rateLimit := &bridgesdk.RateLimitError{
			Err:        errors.New("rate limited primary operation"),
			RetryAfter: time.Nanosecond,
		}
		api := &fakeTelegramAPI{
			editErrors:   []error{rateLimit, nil},
			deleteErrors: []error{rateLimit, nil},
		}
		state := deliveryState{
			LastSeq:         1,
			LastContent:     "before",
			RemoteMessageID: encodeRemoteMessageID("peer-1", 901),
		}
		update := testDeliveryRequest(
			"brg-1",
			"delivery-primary-retry",
			2,
			bridgepkg.DeliveryEventTypeDelta,
			false,
		)
		update.Event.Operation = bridgepkg.DeliveryOperationEdit
		update.Event.Reference = &bridgepkg.DeliveryMessageReference{RemoteMessageID: state.RemoteMessageID}
		update.Event.Content.Text = "updated after rate limit"

		ack, state, err := executeDelivery(context.Background(), api, update, state)
		if err != nil {
			t.Fatalf("executeDelivery(update) error = %v", err)
		}
		if got, want := len(api.editRequests), 2; got != want {
			t.Fatalf("edit attempts = %d, want %d", got, want)
		}

		deleteRequest := testDeleteRequest(
			"brg-1",
			"delivery-primary-retry",
			3,
			ack.RemoteMessageID,
		)
		if _, _, err := executeDelivery(context.Background(), api, deleteRequest, state); err != nil {
			t.Fatalf("executeDelivery(delete) error = %v", err)
		}
		if got, want := len(api.deleteRequests), 2; got != want {
			t.Fatalf("delete attempts = %d, want %d", got, want)
		}
	})
}

func TestTelegramProgressDelivery(t *testing.T) {
	t.Parallel()

	t.Run("Should post a formatted progress bubble in the inbound topic before reacting", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
		api := &fakeTelegramAPI{nextMessageID: 900}
		provider := newTelegramProgressTestProvider(t, func() time.Time { return now }, api)
		request := testProgressRequest(
			"brg-progress",
			"delivery-progress",
			1,
			bridgepkg.ToolProgressPhaseStarted,
			"**Reading**",
		)
		request.Event.Progress.Preview = "a.b!"
		request.Event.RoutingKey.ThreadID = "42"
		request.Event.DeliveryTarget.ThreadID = "42"

		ack, err := provider.handleBridgesProgress(context.Background(), &bridgesdk.Session{}, request)
		if err != nil {
			t.Fatalf("handleBridgesProgress() error = %v", err)
		}
		if got := ack.RemoteMessageID; got != "" {
			t.Fatalf("ack.RemoteMessageID = %q, want empty progress ACK", got)
		}
		if got, want := len(api.sendRequests), 1; got != want {
			t.Fatalf("send calls = %d, want %d", got, want)
		}
		if got, want := api.sendRequests[0].MessageThreadID, int64(42); got != want {
			t.Fatalf("message_thread_id = %d, want %d", got, want)
		}
		if got, want := api.sendRequests[0].Text, "\U0001f527 *Reading* a\\.b\\!"; got != want {
			t.Fatalf("send text = %q, want %q", got, want)
		}
		if got, want := api.sendRequests[0].ParseMode, telegramMarkdownV2; got != want {
			t.Fatalf("parse mode = %q, want %q", got, want)
		}
		if got, want := len(api.chatActions), 1; got != want {
			t.Fatalf("chat action calls = %d, want %d", got, want)
		}
		if got, want := api.chatActions[0].Action, "typing"; got != want {
			t.Fatalf("chat action = %q, want %q", got, want)
		}
		if got, want := len(api.reactions), 1; got != want {
			t.Fatalf("reaction calls = %d, want %d", got, want)
		}
		if got, want := api.reactions[0].MessageID, int64(900); got != want {
			t.Fatalf("reaction message id = %d, want progress bubble %d", got, want)
		}
		if got, want := api.reactions[0].Reaction[0].Emoji, "\U0001f440"; got != want {
			t.Fatalf("reaction emoji = %q, want %q", got, want)
		}
	})

	t.Run("Should deliver final text once after a committed progress edit result failure", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 13, 12, 5, 0, 0, time.UTC)
		api := &fakeTelegramAPI{
			nextMessageID: 915,
			editErrors: []error{
				bridgesdk.MarkCommittedMutation(errors.New("progress edit result unavailable")),
			},
		}
		provider := newTelegramProgressTestProvider(t, func() time.Time { return now }, api)
		cfg, ok := provider.routes.Get("brg-progress")
		if !ok {
			t.Fatal("progress route = missing")
		}
		target := testDeliveryRequest(
			"brg-progress",
			"delivery-progress-committed",
			3,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		).Event.DeliveryTarget
		accumulator := bridgesdk.NewProgressAccumulator(bridgesdk.ProgressConfig{
			ProgressConfig: telegramResolvedProgressConfig(cfg),
			Target:         target,
			MaxMessageLen:  telegramMaxMessageLen,
			Len: func(value string) int {
				return bridgesdk.UTF16Len(formatOutbound(value).Text)
			},
		}, &telegramProgressSink{api: api}, func() time.Time { return now })
		dispatcher := bridgesdk.NewProgressDispatcher(
			accumulator,
			bridgesdk.WithProgressSchedule(func(time.Duration, func()) bridgesdk.ProgressCancel {
				return func() bool { return true }
			}),
		)
		t.Cleanup(dispatcher.Close)
		if err := dispatcher.OnProgress(context.Background(), bridgepkg.ToolProgress{
			ToolCallID: "call-progress-started",
			ToolID:     "agh__terminal",
			Phase:      bridgepkg.ToolProgressPhaseStarted,
			Label:      "Inspecting",
			Emoji:      "\U0001f527",
			Index:      1,
		}); err != nil {
			t.Fatalf("ProgressDispatcher.OnProgress(started) error = %v", err)
		}
		if err := dispatcher.OnProgress(context.Background(), bridgepkg.ToolProgress{
			ToolCallID: "call-progress-completed",
			ToolID:     "agh__terminal",
			Phase:      bridgepkg.ToolProgressPhaseCompleted,
			Label:      "Inspecting",
			Emoji:      "\U0001f527",
			Index:      2,
		}); err != nil {
			t.Fatalf("ProgressDispatcher.OnProgress(completed) error = %v", err)
		}
		provider.deliveries.Store(
			deliveryStateKey("brg-progress", "delivery-progress-committed"),
			deliveryState{Progress: dispatcher},
		)

		final := testDeliveryRequest(
			"brg-progress",
			"delivery-progress-committed",
			3,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		final.Event.Content.Text = "final answer"
		ack, err := provider.handleBridgesDeliver(context.Background(), &bridgesdk.Session{}, final)
		if err != nil {
			t.Fatalf("handleBridgesDeliver(final) error = %v", err)
		}
		if got, want := ack.DeliveryID, final.Event.DeliveryID; got != want {
			t.Fatalf("ack delivery id = %q, want %q", got, want)
		}
		if got, want := len(api.editRequests), 1; got != want {
			t.Fatalf("committed progress edit attempts = %d, want %d", got, want)
		}
		if got, want := countTelegramSendRequests(api.sendRequests, "final answer"), 1; got != want {
			t.Fatalf("final text send attempts = %d, want %d", got, want)
		}
	})

	t.Run("Should discard and close progress state after a committed final text result failure", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 13, 12, 6, 0, 0, time.UTC)
		api := &fakeTelegramAPI{nextMessageID: 916}
		provider := newTelegramProgressTestProvider(t, func() time.Time { return now }, api)
		started := testProgressRequest(
			"brg-progress",
			"delivery-text-committed",
			1,
			bridgepkg.ToolProgressPhaseStarted,
			"Inspecting",
		)
		if _, err := provider.handleBridgesProgress(context.Background(), &bridgesdk.Session{}, started); err != nil {
			t.Fatalf("handleBridgesProgress(started) error = %v", err)
		}
		dispatcher := provider.deliveryState("brg-progress", "delivery-text-committed").Progress
		if dispatcher == nil {
			t.Fatal("progress dispatcher = nil, want active dispatcher")
		}
		api.sendErrorText = "final answer"
		api.sendError = bridgesdk.MarkCommittedMutation(errors.New("final text result unavailable"))
		final := testDeliveryRequest(
			"brg-progress",
			"delivery-text-committed",
			2,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		final.Event.Content.Text = "final answer"

		_, err := provider.handleBridgesDeliver(context.Background(), &bridgesdk.Session{}, final)
		if !bridgesdk.IsCommittedMutation(err) {
			t.Fatalf("handleBridgesDeliver(final) error = %T %v, want committed mutation", err, err)
		}
		if got, want := countTelegramSendRequests(api.sendRequests, "final answer"), 1; got != want {
			t.Fatalf("final text send attempts = %d, want %d", got, want)
		}
		if _, retained := provider.deliveries.Load(
			deliveryStateKey("brg-progress", "delivery-text-committed"),
		); retained {
			t.Fatal("committed final text failure retained delivery state")
		}
		if flushErr := dispatcher.Flush(context.Background()); flushErr == nil ||
			!strings.Contains(flushErr.Error(), "closed") {
			t.Fatalf("closed dispatcher Flush() error = %v, want closed error", flushErr)
		}
	})

	t.Run("Should retry a rate limited edit without dropping accumulated lines", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 12, 10, 5, 0, 0, time.UTC)
		api := &fakeTelegramAPI{
			nextMessageID: 905,
			editErrors: []error{
				&bridgesdk.RateLimitError{Err: errors.New("rate limited"), RetryAfter: time.Nanosecond},
				nil,
			},
		}
		provider := newTelegramProgressTestProvider(t, func() time.Time { return now }, api)
		for seq, label := range []string{"Inspecting", "Reading"} {
			request := testProgressRequest(
				"brg-progress",
				"delivery-retry",
				int64(seq+1),
				bridgepkg.ToolProgressPhaseStarted,
				label,
			)
			_, err := provider.handleBridgesProgress(context.Background(), &bridgesdk.Session{}, request)
			if err != nil {
				t.Fatalf("handleBridgesProgress(%q) error = %v", label, err)
			}
		}
		final := testLifecycleOnlyFinalRequest("brg-progress", "delivery-retry", 3)
		if _, err := provider.handleBridgesProgress(context.Background(), &bridgesdk.Session{}, final); err != nil {
			t.Fatalf("handleBridgesProgress(final) error = %v", err)
		}

		if got, want := len(api.editRequests), 2; got != want {
			t.Fatalf("edit attempts = %d, want %d", got, want)
		}
		wantText := "\U0001f527 Inspecting\n\U0001f527 Reading"
		for index, edit := range api.editRequests {
			if got := edit.Text; got != wantText {
				t.Fatalf("edit[%d].Text = %q, want %q", index, got, wantText)
			}
			if got, want := edit.MessageID, int64(905); got != want {
				t.Fatalf("edit[%d].MessageID = %d, want %d", index, got, want)
			}
		}
	})

	t.Run("Should deliver completed duration and failed detail through the progress bubble", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 12, 10, 7, 0, 0, time.UTC)
		api := &fakeTelegramAPI{nextMessageID: 907}
		provider := newTelegramProgressTestProvider(t, func() time.Time { return now }, api)
		started := testProgressRequest(
			"brg-progress",
			"delivery-terminal-lines",
			1,
			bridgepkg.ToolProgressPhaseStarted,
			"Inspecting",
		)
		if _, err := provider.handleBridgesProgress(context.Background(), &bridgesdk.Session{}, started); err != nil {
			t.Fatalf("handleBridgesProgress(started) error = %v", err)
		}
		now = now.Add(2 * time.Second)
		completed := testProgressRequest(
			"brg-progress",
			"delivery-terminal-lines",
			2,
			bridgepkg.ToolProgressPhaseCompleted,
			"Inspecting",
		)
		completed.Event.Progress.DurationMS = 31_000
		if _, err := provider.handleBridgesProgress(context.Background(), &bridgesdk.Session{}, completed); err != nil {
			t.Fatalf("handleBridgesProgress(completed) error = %v", err)
		}
		now = now.Add(2 * time.Second)
		failed := testProgressRequest(
			"brg-progress",
			"delivery-terminal-lines",
			3,
			bridgepkg.ToolProgressPhaseFailed,
			"Writing",
		)
		failed.Event.Progress.Error = "permission denied"
		if _, err := provider.handleBridgesProgress(context.Background(), &bridgesdk.Session{}, failed); err != nil {
			t.Fatalf("handleBridgesProgress(failed) error = %v", err)
		}

		if got, want := len(api.editRequests), 2; got != want {
			t.Fatalf("progress edits = %d, want %d", got, want)
		}
		body := api.editRequests[len(api.editRequests)-1].Text
		if !strings.Contains(body, "\u2705 Inspecting \u00b7 31s") ||
			!strings.Contains(body, "\u274c Writing \u2014 permission denied") {
			t.Fatalf("progress body = %q, want completed duration and failed detail", body)
		}
		if got, want := api.reactions[len(api.reactions)-1].Reaction[0].Emoji, "\u274c"; got != want {
			t.Fatalf("failed reaction = %q, want %q", got, want)
		}
	})

	t.Run("Should detach an existing dispatcher when progress mode changes to off", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 12, 10, 8, 0, 0, time.UTC)
		api := &fakeTelegramAPI{nextMessageID: 908}
		provider := newTelegramProgressTestProvider(t, func() time.Time { return now }, api)
		started := testProgressRequest(
			"brg-progress",
			"delivery-mode-change",
			1,
			bridgepkg.ToolProgressPhaseStarted,
			"Inspecting",
		)
		if _, err := provider.handleBridgesProgress(context.Background(), &bridgesdk.Session{}, started); err != nil {
			t.Fatalf("handleBridgesProgress(started) error = %v", err)
		}
		callsBeforeOff := len(api.methods)
		cfg, _ := provider.routes.Get("brg-progress")
		cfg.managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"off","grouping":"accumulate","typing":false,"reactions":false}}`,
		)
		provider.routes.Update("brg-progress", func(resolvedInstanceConfig) resolvedInstanceConfig { return cfg })

		off := testProgressRequest(
			"brg-progress",
			"delivery-mode-change",
			2,
			bridgepkg.ToolProgressPhaseStarted,
			"Reading",
		)
		if _, err := provider.handleBridgesProgress(context.Background(), &bridgesdk.Session{}, off); err != nil {
			t.Fatalf("handleBridgesProgress(off) error = %v", err)
		}
		if got := len(api.methods); got != callsBeforeOff {
			t.Fatalf("platform calls after mode off = %d, want unchanged %d", got, callsBeforeOff)
		}
		if state := provider.deliveryState("brg-progress", "delivery-mode-change"); state.Progress != nil {
			t.Fatal("mode-off delivery retained a progress dispatcher")
		}
	})

	t.Run("Should make zero platform calls when progress mode is off", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 12, 10, 10, 0, 0, time.UTC)
		api := &fakeTelegramAPI{nextMessageID: 910}
		provider := newTelegramProgressTestProvider(t, func() time.Time { return now }, api)
		cfg, _ := provider.routes.Get("brg-progress")
		cfg.managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"off","grouping":"accumulate","typing":false,"reactions":false}}`,
		)
		provider.routes.Update("brg-progress", func(resolvedInstanceConfig) resolvedInstanceConfig { return cfg })
		factoryCalls := 0
		provider.apiFactory = func(*resolvedInstanceConfig) telegramAPI {
			factoryCalls++
			return api
		}

		request := testProgressRequest(
			"brg-progress",
			"delivery-off",
			1,
			bridgepkg.ToolProgressPhaseStarted,
			"Inspecting",
		)
		if _, err := provider.handleBridgesProgress(context.Background(), &bridgesdk.Session{}, request); err != nil {
			t.Fatalf("handleBridgesProgress() error = %v", err)
		}
		if factoryCalls != 0 {
			t.Fatalf("api factory calls = %d, want zero", factoryCalls)
		}
		if got := len(api.methods); got != 0 {
			t.Fatalf("platform calls = %d, want zero", got)
		}
	})
}

func TestExecuteDeliveryFormatsAndChunksCumulativeText(t *testing.T) {
	t.Parallel()

	t.Run("Should keep an oversized preview in one message and materialize continuations on final", func(t *testing.T) {
		t.Parallel()

		api := &fakeTelegramAPI{nextMessageID: 500}
		content := strings.Repeat("**bold!** 🙂 ", 400)
		startReq := testDeliveryRequest(
			"brg-1",
			"delivery-long",
			1,
			bridgepkg.DeliveryEventTypeStart,
			false,
		)
		startReq.Event.Content.Text = content
		startReq.Event.RoutingKey.ThreadID = "42"
		startReq.Event.DeliveryTarget.ThreadID = "42"

		startAck, state, err := executeDelivery(context.Background(), api, startReq, deliveryState{})
		if err != nil {
			t.Fatalf("executeDelivery(start) error = %v", err)
		}
		if got, want := len(api.sendRequests), 1; got != want {
			t.Fatalf("SendMessage calls after preview = %d, want %d", got, want)
		}
		if got := bridgesdk.UTF16Len(api.sendRequests[0].Text); got > telegramMaxMessageLen {
			t.Fatalf("preview UTF-16 length = %d, want <= %d", got, telegramMaxMessageLen)
		}
		if got, want := api.sendRequests[0].ParseMode, telegramMarkdownV2; got != want {
			t.Fatalf("preview parse mode = %q, want %q", got, want)
		}
		if got, want := api.sendRequests[0].MessageThreadID, int64(42); got != want {
			t.Fatalf("preview message_thread_id = %d, want %d", got, want)
		}
		if !state.PreviewOnly {
			t.Fatal("preview state = materialized, want preview-only")
		}

		deltaReq := testDeliveryRequest(
			"brg-1",
			"delivery-long",
			2,
			bridgepkg.DeliveryEventTypeDelta,
			false,
		)
		deltaReq.Event.Operation = bridgepkg.DeliveryOperationEdit
		deltaReq.Event.Reference = &bridgepkg.DeliveryMessageReference{
			RemoteMessageID: startAck.RemoteMessageID,
		}
		deltaReq.Event.Content.Text = content
		deltaReq.Event.RoutingKey.ThreadID = "42"
		deltaReq.Event.DeliveryTarget.ThreadID = "42"
		_, state, err = executeDelivery(context.Background(), api, deltaReq, state)
		if err != nil {
			t.Fatalf("executeDelivery(delta edit) error = %v", err)
		}
		if got, want := len(api.editRequests), 1; got != want {
			t.Fatalf("EditMessageText calls after delta = %d, want %d", got, want)
		}
		if got, want := len(api.sendRequests), 1; got != want {
			t.Fatalf("SendMessage calls after delta = %d, want preview only (%d)", got, want)
		}
		if !state.PreviewOnly {
			t.Fatal("delta state = materialized, want preview-only")
		}

		finalReq := testDeliveryRequest(
			"brg-1",
			"delivery-long",
			3,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		finalReq.Event.Content.Text = content
		finalReq.Event.RoutingKey.ThreadID = "42"
		finalReq.Event.DeliveryTarget.ThreadID = "42"
		finalAck, finalState, err := executeDelivery(context.Background(), api, finalReq, state)
		if err != nil {
			t.Fatalf("executeDelivery(final) error = %v", err)
		}
		if got, want := len(api.editRequests), 2; got != want {
			t.Fatalf("EditMessageText calls = %d, want %d", got, want)
		}
		if got := len(api.sendRequests); got < 2 {
			t.Fatalf("SendMessage calls after final = %d, want preview plus continuation", got)
		}
		if finalState.PreviewOnly {
			t.Fatal("final state = preview-only, want materialized")
		}
		if got, want := finalAck.RemoteMessageID, finalState.RemoteMessageID; got != want {
			t.Fatalf("final ACK remote id = %q, want state handle %q", got, want)
		}
		if finalAck.RemoteMessageID == startAck.RemoteMessageID {
			t.Fatalf("final ACK remote id = %q, want last continuation handle", finalAck.RemoteMessageID)
		}

		wireBodies := []telegramOutboundRequest{
			{
				Text:      api.editRequests[len(api.editRequests)-1].Text,
				ParseMode: api.editRequests[len(api.editRequests)-1].ParseMode,
			},
		}
		for _, request := range api.sendRequests[1:] {
			wireBodies = append(wireBodies, telegramOutboundRequest{
				Text:      request.Text,
				ParseMode: request.ParseMode,
			})
			if got, want := request.MessageThreadID, int64(42); got != want {
				t.Fatalf("continuation message_thread_id = %d, want %d", got, want)
			}
		}
		for index, body := range wireBodies {
			if got := bridgesdk.UTF16Len(body.Text); got > telegramMaxMessageLen {
				t.Fatalf("final chunk %d UTF-16 length = %d, want <= %d", index, got, telegramMaxMessageLen)
			}
			if got, want := body.ParseMode, telegramMarkdownV2; got != want {
				t.Fatalf("final chunk %d parse mode = %q, want %q", index, got, want)
			}
		}
	})

	t.Run("Should retry one MarkdownV2 parse rejection as plain text", func(t *testing.T) {
		t.Parallel()

		api := &fakeTelegramAPI{
			nextMessageID: 700,
			sendErrors: []error{
				&telegramMarkdownParseError{err: errors.New("can't parse entities")},
				nil,
			},
		}
		request := testDeliveryRequest(
			"brg-1",
			"delivery-fallback",
			1,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		request.Event.Content.Text = "**bold** result"

		if _, _, err := executeDelivery(context.Background(), api, request, deliveryState{}); err != nil {
			t.Fatalf("executeDelivery() error = %v", err)
		}
		if got, want := len(api.sendRequests), 2; got != want {
			t.Fatalf("SendMessage calls = %d, want %d", got, want)
		}
		if got, want := api.sendRequests[0].ParseMode, telegramMarkdownV2; got != want {
			t.Fatalf("first parse mode = %q, want %q", got, want)
		}
		if got := api.sendRequests[1].ParseMode; got != "" {
			t.Fatalf("fallback parse mode = %q, want empty", got)
		}
		if got, want := api.sendRequests[1].Text, request.Event.Content.Text; got != want {
			t.Fatalf("fallback text = %q, want original %q", got, want)
		}
	})

	t.Run("Should retry one MarkdownV2 edit rejection as plain text", func(t *testing.T) {
		t.Parallel()

		api := &fakeTelegramAPI{
			editErrors: []error{
				&telegramMarkdownParseError{err: errors.New("can't parse entities")},
				nil,
			},
		}
		request := testDeliveryRequest(
			"brg-1",
			"delivery-edit-fallback",
			2,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		request.Event.Content.Text = "**bold** updated"
		state := deliveryState{
			LastSeq:         1,
			LastContent:     "old",
			RemoteMessageID: encodeRemoteMessageID("peer-1", 41),
		}

		if _, _, err := executeDelivery(context.Background(), api, request, state); err != nil {
			t.Fatalf("executeDelivery() error = %v", err)
		}
		if got, want := len(api.editRequests), 2; got != want {
			t.Fatalf("EditMessageText calls = %d, want %d", got, want)
		}
		if got, want := api.editRequests[0].ParseMode, telegramMarkdownV2; got != want {
			t.Fatalf("first parse mode = %q, want %q", got, want)
		}
		if got := api.editRequests[1].ParseMode; got != "" {
			t.Fatalf("fallback parse mode = %q, want empty", got)
		}
		if got, want := api.editRequests[1].Text, request.Event.Content.Text; got != want {
			t.Fatalf("fallback text = %q, want original %q", got, want)
		}
	})

	t.Run("Should not retry a committed MarkdownV2 send rejection as plain text", func(t *testing.T) {
		t.Parallel()

		api := &fakeTelegramAPI{
			sendErrors: []error{bridgesdk.MarkCommittedMutation(
				&telegramMarkdownParseError{err: errors.New("can't parse entities")},
			)},
		}
		request := testDeliveryRequest(
			"brg-1",
			"delivery-committed-send-parse",
			1,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		request.Event.Content.Text = "**bold** result"

		_, _, err := executeDelivery(context.Background(), api, request, deliveryState{})
		if !bridgesdk.IsCommittedMutation(err) {
			t.Fatalf("executeDelivery() error = %T %v, want committed mutation", err, err)
		}
		if got, want := len(api.sendRequests), 1; got != want {
			t.Fatalf("SendMessage calls = %d, want %d without fallback", got, want)
		}
	})

	t.Run("Should not retry a committed MarkdownV2 edit rejection as plain text", func(t *testing.T) {
		t.Parallel()

		api := &fakeTelegramAPI{
			editErrors: []error{bridgesdk.MarkCommittedMutation(
				&telegramMarkdownParseError{err: errors.New("can't parse entities")},
			)},
		}
		request := testDeliveryRequest(
			"brg-1",
			"delivery-committed-edit-parse",
			2,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		request.Event.Content.Text = "**bold** updated"
		state := deliveryState{
			LastSeq:         1,
			LastContent:     "old",
			RemoteMessageID: encodeRemoteMessageID("peer-1", 41),
		}

		_, _, err := executeDelivery(context.Background(), api, request, state)
		if !bridgesdk.IsCommittedMutation(err) {
			t.Fatalf("executeDelivery() error = %T %v, want committed mutation", err, err)
		}
		if got, want := len(api.editRequests), 1; got != want {
			t.Fatalf("EditMessageText calls = %d, want %d without fallback", got, want)
		}
	})

	t.Run("Should preserve fenced code literals in a chunk plain-text fallback", func(t *testing.T) {
		t.Parallel()

		api := &fakeTelegramAPI{
			nextMessageID: 900,
			sendErrors: []error{
				&telegramMarkdownParseError{err: errors.New("can't parse entities")},
				nil,
			},
		}
		request := testDeliveryRequest(
			"brg-1",
			"delivery-fenced-fallback",
			1,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		const protectedLine = "my_value * ~"
		request.Event.Content.Text = "**Result**\n```txt\n" +
			strings.Repeat(protectedLine+"\n", 500) + "```"

		if _, _, err := executeDelivery(context.Background(), api, request, deliveryState{}); err != nil {
			t.Fatalf("executeDelivery() error = %v", err)
		}
		if got := len(api.sendRequests); got < 3 {
			t.Fatalf("SendMessage calls = %d, want rich attempt, fallback, and continuation", got)
		}
		if got := api.sendRequests[1].ParseMode; got != "" {
			t.Fatalf("fallback parse mode = %q, want empty", got)
		}
		if !strings.Contains(api.sendRequests[1].Text, protectedLine) {
			t.Fatalf("fallback text does not preserve protected line %q: %q", protectedLine, api.sendRequests[1].Text)
		}
	})

	t.Run("Should reject a malformed successful plain-text fallback response", func(t *testing.T) {
		t.Parallel()

		api := &fakeTelegramAPI{
			sendErrors: []error{
				&telegramMarkdownParseError{err: errors.New("can't parse entities")},
				nil,
			},
			sendResults: []*telegramSentMessage{
				nil,
				{MessageID: 0},
			},
		}
		request := testDeliveryRequest(
			"brg-1",
			"delivery-malformed-fallback",
			1,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		request.Event.Content.Text = "**bold** result"

		_, gotState, err := executeDelivery(context.Background(), api, request, deliveryState{})
		var committedErr *bridgesdk.CommittedMutationError
		if !errors.As(err, &committedErr) {
			t.Fatalf("executeDelivery() error = %T %v, want CommittedMutationError", err, err)
		}
		if gotState.LastSeq != 0 || gotState.RemoteMessageID != "" || gotState.Chunks.NextChunk() != 0 {
			t.Fatalf("executeDelivery() state = %#v, want no committed chunk", gotState)
		}
		if got, want := len(api.sendRequests), 2; got != want {
			t.Fatalf("SendMessage calls = %d, want rich rejection and one plain-text mutation", got)
		}
	})

	t.Run("Should send oversized inline code as bounded plain-text chunks", func(t *testing.T) {
		t.Parallel()

		api := &fakeTelegramAPI{nextMessageID: 800}
		request := testDeliveryRequest(
			"brg-1",
			"delivery-inline-code",
			1,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		request.Event.Content.Text = "`" + strings.Repeat("value_", 800) + "`"

		if _, _, err := executeDelivery(context.Background(), api, request, deliveryState{}); err != nil {
			t.Fatalf("executeDelivery() error = %v", err)
		}
		if got := len(api.sendRequests); got < 2 {
			t.Fatalf("SendMessage calls = %d, want multiple plain-text chunks", got)
		}
		for index, outbound := range api.sendRequests {
			if got := outbound.ParseMode; got != "" {
				t.Fatalf("chunk %d parse mode = %q, want empty", index, got)
			}
			if got := bridgesdk.UTF16Len(outbound.Text); got > telegramMaxMessageLen {
				t.Fatalf("chunk %d UTF-16 length = %d, want <= %d", index, got, telegramMaxMessageLen)
			}
		}
	})

	t.Run("Should not retry a non-format Telegram rejection", func(t *testing.T) {
		t.Parallel()

		api := &fakeTelegramAPI{
			nextMessageID: 700,
			sendErrors: []error{
				&bridgesdk.PermanentError{Err: errors.New("chat not found")},
			},
		}
		request := testDeliveryRequest(
			"brg-1",
			"delivery-no-fallback",
			1,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		if _, _, err := executeDelivery(context.Background(), api, request, deliveryState{}); err == nil {
			t.Fatal("executeDelivery() error = nil, want permanent failure")
		}
		if got, want := len(api.sendRequests), 1; got != want {
			t.Fatalf("SendMessage calls = %d, want %d", got, want)
		}
	})

	t.Run("Should resume at the first unsent chunk after exhausting Retry-After attempts", func(t *testing.T) {
		t.Parallel()

		request := testDeliveryRequest(
			"brg-1",
			"delivery-partial-chunk",
			1,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		request.Event.Content.Text = strings.Repeat("partial Telegram delivery ", 400)
		chunks, _ := telegramDeliveryChunks(request.Event)
		if len(chunks) < 2 {
			t.Fatalf("telegramDeliveryChunks() returned %d chunk, want at least 2", len(chunks))
		}

		api := &fakeTelegramAPI{
			nextMessageID: 900,
			sendErrorText: chunks[1].Text,
			sendError: &bridgesdk.RateLimitError{
				Err:        errors.New("rate limited continuation"),
				RetryAfter: time.Nanosecond,
			},
		}
		_, partialState, err := executeDelivery(context.Background(), api, request, deliveryState{})
		if err == nil {
			t.Fatal("executeDelivery(partial) error = nil, want exhausted rate limit")
		}
		if got, want := countTelegramSendRequests(
			api.sendRequests,
			chunks[1].Text,
		), bridgesdk.DefaultRetryConfig().Attempts; got != want {
			t.Fatalf("failed continuation attempts = %d, want %d", got, want)
		}
		if got, want := countTelegramSendRequests(api.successfulSendRequests, chunks[0].Text), 1; got != want {
			t.Fatalf("successful first-chunk sends = %d, want %d", got, want)
		}

		api.sendError = nil
		resume := testTelegramResumeRequest(request)
		if _, _, err := executeDelivery(context.Background(), api, resume, partialState); err != nil {
			t.Fatalf("executeDelivery(resume) error = %v", err)
		}
		if got, want := countTelegramSendRequests(api.successfulSendRequests, chunks[0].Text), 1; got != want {
			t.Fatalf("successful first-chunk sends after resume = %d, want %d", got, want)
		}
		if got, want := len(api.successfulSendRequests), len(chunks); got != want {
			t.Fatalf("successful chunk sends = %d, want %d", got, want)
		}
	})
}

type telegramOutboundRequest struct {
	Text      string
	ParseMode string
}

func TestVerifyWebhookSecret(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.com/telegram/brg-1",
		strings.NewReader(`{}`),
	)
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "secret")

	if err := verifyWebhookSecret(context.Background(), req, nil, "secret"); err != nil {
		t.Fatalf("verifyWebhookSecret(valid) error = %v", err)
	}
	if err := verifyWebhookSecret(context.Background(), req, nil, "wrong"); err == nil {
		t.Fatal("verifyWebhookSecret(invalid) error = nil, want non-nil")
	}
	if err := verifyWebhookSecret(context.Background(), req, nil, ""); err == nil ||
		!strings.Contains(err.Error(), "webhook secret is required") {
		t.Fatalf("verifyWebhookSecret(missing configured secret) error = %v, want missing-secret error", err)
	}
}

func TestRuntimeInitializeStartsWebhookServerAndWritesMarkers(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mockAPI := newTelegramAPIServer(t)
	t.Setenv(telegramListenAddrEnv, listenAddr)
	t.Setenv(telegramAPIBaseEnv, mockAPI.URL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC)
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
		func(_ context.Context, params json.RawMessage) (any, error) {
			var payload bridgepkg.BridgeInstanceTargetParams
			if err := json.Unmarshal(params, &payload); err != nil {
				return nil, err
			}
			switch payload.BridgeInstanceID {
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
		func(_ context.Context, params json.RawMessage) (any, error) {
			var payload bridgepkg.BridgesInstancesReportStateParams
			if err := json.Unmarshal(params, &payload); err != nil {
				return nil, err
			}
			instance := managed[0].Instance
			if payload.BridgeInstanceID == "brg-2" {
				instance = managed[1].Instance
			}
			instance.Status = payload.Status
			instance.Degradation = payload.Degradation
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

	handshake := waitForJSONFile[bridgesdk.InitializeMarker](t, env.HandshakePath)
	if got, want := handshake.Request.Runtime.Bridge.Provider, "telegram"; got != want {
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

func TestWebhookIngressRejectsInvalidSecretAndIngestsMessage(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mockAPI := newTelegramAPIServer(t)
	t.Setenv(telegramListenAddrEnv, listenAddr)
	t.Setenv(telegramAPIBaseEnv, mockAPI.URL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 13, 5, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-1")
	managed.BoundSecrets = []subprocess.InitializeBridgeBoundSecret{
		{BindingName: "bot_token", Kind: "token", Value: "telegram-token"},
		{BindingName: "webhook_secret", Kind: "token", Value: "top-secret"},
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
	waitForCondition(t, func() bool {
		return strings.TrimSpace(runtime.http.Address()) != ""
	})

	serverAddr := runtime.http.Address()
	webhookURL := "http://" + serverAddr + "/telegram/brg-1"

	invalidReq, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		webhookURL,
		strings.NewReader(telegramWebhookPayload()),
	)
	if err != nil {
		t.Fatalf("http.NewRequest(invalid) error = %v", err)
	}
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidReq.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wrong")
	invalidResp, err := http.DefaultClient.Do(invalidReq)
	if err != nil {
		t.Fatalf("http.DefaultClient.Do(invalid) error = %v", err)
	}
	defer func() {
		if closeErr := invalidResp.Body.Close(); closeErr != nil {
			t.Fatalf("invalidResp.Body.Close() error = %v", closeErr)
		}
	}()
	if got, want := invalidResp.StatusCode, http.StatusUnauthorized; got != want {
		t.Fatalf("invalid webhook status = %d, want %d", got, want)
	}

	validReq, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		webhookURL,
		strings.NewReader(telegramWebhookPayload()),
	)
	if err != nil {
		t.Fatalf("http.NewRequest(valid) error = %v", err)
	}
	validReq.Header.Set("Content-Type", "application/json")
	validReq.Header.Set("X-Telegram-Bot-Api-Secret-Token", "top-secret")
	validResp, err := http.DefaultClient.Do(validReq)
	if err != nil {
		t.Fatalf("http.DefaultClient.Do(valid) error = %v", err)
	}
	defer func() {
		if closeErr := validResp.Body.Close(); closeErr != nil {
			t.Fatalf("validResp.Body.Close() error = %v", closeErr)
		}
	}()
	if got, want := validResp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("valid webhook status = %d, want %d", got, want)
	}

	ingests := waitForJSONLinesFile[bridgesdk.IngestMarker](
		t,
		env.IngestPath,
		func(items []bridgesdk.IngestMarker) bool {
			return len(items) == 1 && strings.TrimSpace(items[0].Result.SessionID) != ""
		},
	)
	if got, want := ingests[0].Envelope.PeerID, "12345"; got != want {
		t.Fatalf("ingest envelope peer id = %q, want %q", got, want)
	}
	mu.Lock()
	if got, want := len(ingested), 1; got != want {
		t.Fatalf("len(ingested) = %d, want %d", got, want)
	}
	mu.Unlock()
}

func TestRuntimeDeliveriesCallTelegramBotAPI(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mockAPI := newTelegramAPIServer(t)
	t.Setenv(telegramListenAddrEnv, listenAddr)
	t.Setenv(telegramAPIBaseEnv, mockAPI.URL())

	_, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 13, 10, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-1")
	managed.BoundSecrets = []subprocess.InitializeBridgeBoundSecret{
		{BindingName: "bot_token", Kind: "token", Value: "telegram-token"},
		{BindingName: "webhook_secret", Kind: "token", Value: "top-secret"},
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
	states := waitForJSONLinesFile[bridgesdk.StateMarker](
		t,
		env.StatePath,
		func(items []bridgesdk.StateMarker) bool { return len(items) >= 1 },
	)
	if got, want := states[len(states)-1].Status.Normalize(), bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("states[last].Status = %q, want %q", got, want)
	}

	var ack bridgepkg.DeliveryAck
	startReq := testDeliveryRequest(
		"brg-1",
		"delivery-1",
		1,
		bridgepkg.DeliveryEventTypeStart,
		false,
	)
	startReq.Event.RoutingKey.ThreadID = "42"
	startReq.Event.DeliveryTarget.ThreadID = "42"
	if err := hostPeer.Call(context.Background(), "bridges/deliver", startReq, &ack); err != nil {
		t.Fatalf("hostPeer.Call(start delivery) error = %v", err)
	}
	finalReq := testDeliveryRequest(
		"brg-1",
		"delivery-1",
		2,
		bridgepkg.DeliveryEventTypeFinal,
		true,
	)
	finalReq.Event.Content.Text = "**Result:** [doc](https://x.y)\n```go\n" +
		strings.Repeat("fmt.Println(\"hello\")\n", 230) + "```"
	finalReq.Event.RoutingKey.ThreadID = "42"
	finalReq.Event.DeliveryTarget.ThreadID = "42"
	if err := hostPeer.Call(context.Background(), "bridges/deliver", finalReq, &ack); err != nil {
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

	calls := mockAPI.Calls()
	if got, want := len(calls), 4; got != want {
		t.Fatalf("len(mockAPI calls) = %d, want %d (getMe + send + edit + continuation)", got, want)
	}
	if got, want := calls[0].Method, "getMe"; got != want {
		t.Fatalf("calls[0].Method = %q, want %q", got, want)
	}
	if got, want := calls[1].Method, "sendMessage"; got != want {
		t.Fatalf("calls[1].Method = %q, want %q", got, want)
	}
	if got, want := calls[2].Method, "editMessageText"; got != want {
		t.Fatalf("calls[2].Method = %q, want %q", got, want)
	}
	if got, want := calls[3].Method, "sendMessage"; got != want {
		t.Fatalf("calls[3].Method = %q, want %q", got, want)
	}
	if got, want := calls[1].Body["parse_mode"], telegramMarkdownV2; got != want {
		t.Fatalf("send parse_mode = %#v, want %#v", got, want)
	}
	if got, want := calls[2].Body["parse_mode"], telegramMarkdownV2; got != want {
		t.Fatalf("edit parse_mode = %#v, want %#v", got, want)
	}
	if got, want := calls[1].Body["message_thread_id"], float64(42); got != want {
		t.Fatalf("send message_thread_id = %#v, want %#v", got, want)
	}
	if got, want := calls[3].Body["message_thread_id"], float64(42); got != want {
		t.Fatalf("continuation message_thread_id = %#v, want %#v", got, want)
	}
	for index, call := range calls[2:] {
		text, ok := call.Body["text"].(string)
		if !ok {
			t.Fatalf("final call %d text = %#v, want string", index, call.Body["text"])
		}
		if got := bridgesdk.UTF16Len(text); got > telegramMaxMessageLen {
			t.Fatalf("final call %d UTF-16 length = %d, want <= %d", index, got, telegramMaxMessageLen)
		}
		if got, want := call.Body["parse_mode"], telegramMarkdownV2; got != want {
			t.Fatalf("final call %d parse_mode = %#v, want %#v", index, got, want)
		}
		if got := strings.Count(text, "```"); got%2 != 0 {
			t.Fatalf("final call %d fence count = %d, want balanced", index, got)
		}
	}
	firstFinalText, ok := calls[2].Body["text"].(string)
	if !ok || !strings.Contains(firstFinalText, "*Result:* [doc](https://x.y)") {
		t.Fatalf("first final text = %#v, want Telegram MarkdownV2 conversion", calls[2].Body["text"])
	}
	if records[1].Ack == nil || records[1].Ack.RemoteMessageID != encodeRemoteMessageID("peer-1", 701) {
		t.Fatalf("final ACK = %#v, want last continuation handle", records[1].Ack)
	}
}

func TestHandleShutdownWritesMarker(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mockAPI := newTelegramAPIServer(t)
	t.Setenv(telegramListenAddrEnv, listenAddr)
	t.Setenv(telegramAPIBaseEnv, mockAPI.URL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 13, 15, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-1")
	managed.BoundSecrets = []subprocess.InitializeBridgeBoundSecret{
		{BindingName: "bot_token", Kind: "token", Value: "telegram-token"},
		{BindingName: "webhook_secret", Kind: "token", Value: "top-secret"},
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
	if err := runtime.lifecycle.Shutdown(
		context.Background(),
		nil,
		subprocess.ShutdownRequest{DeadlineMS: 50},
	); err != nil {
		t.Fatalf("handleShutdown() error = %v", err)
	}
	lines := waitForNonEmptyLines(t, env.ShutdownPath)
	if len(lines) == 0 || !strings.Contains(lines[0], "pid=") {
		t.Fatalf("shutdown marker lines = %#v, want pid entry", lines)
	}
}

func TestResolveInstanceConfigAndHelperNormalization(t *testing.T) {
	env := setProviderTestEnv(t)
	_ = env
	listenAddr := reserveListenAddr(t)
	mockAPI := newTelegramAPIServer(t)
	apiBaseURL := mockAPI.URL() + "/"
	t.Setenv(telegramAPIBaseEnv, apiBaseURL)

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-1")
	managed.Instance.DMPolicy = bridgepkg.BridgeDMPolicyPairing
	managed.Instance.ProviderConfig = fmt.Appendf(nil, `{
		"webhook":{"listen_addr":%q,"path":"telegram"},
		"batching":{"delay_ms":5,"split_delay_ms":7,"split_threshold":2},
		"dm":{"allow_user_ids":[" 42 "],"allow_usernames":["@Alice"],"paired_usernames":["Bob"]}
	}`, listenAddr)
	managed.BoundSecrets = []subprocess.InitializeBridgeBoundSecret{
		{BindingName: "bot_token", Kind: "token", Value: "telegram-token"},
		{BindingName: "webhook_secret", Kind: "token", Value: "top-secret"},
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

	session := runtime.currentSession()
	if session == nil {
		t.Fatal("runtime.currentSession() = nil, want session")
	}

	cfg := runtime.resolveInstanceConfig(session, managed)
	if cfg.configError != nil {
		t.Fatalf("resolveInstanceConfig() configError = %v, want nil", cfg.configError)
	}
	defer cfg.batcher.Close()

	if got, want := cfg.apiBaseURL, strings.TrimSuffix(mockAPI.URL(), "/"); got != want {
		t.Fatalf("cfg.apiBaseURL = %q, want %q", got, want)
	}
	if got, want := cfg.listenAddr, listenAddr; got != want {
		t.Fatalf("cfg.listenAddr = %q, want %q", got, want)
	}
	if got, want := cfg.webhookPath, "/telegram"; got != want {
		t.Fatalf("cfg.webhookPath = %q, want %q", got, want)
	}
	if got, want := cfg.botToken, "telegram-token"; got != want {
		t.Fatalf("cfg.botToken = %q, want %q", got, want)
	}
	if got, want := cfg.webhookSecret, "top-secret"; got != want {
		t.Fatalf("cfg.webhookSecret = %q, want %q", got, want)
	}
	if cfg.batcher == nil {
		t.Fatal("cfg.batcher = nil, want batcher")
	}
	if _, ok := cfg.allowUserIDs["42"]; !ok {
		t.Fatalf("cfg.allowUserIDs = %#v, want normalized user id", cfg.allowUserIDs)
	}
	if _, ok := cfg.allowUsernames["alice"]; !ok {
		t.Fatalf("cfg.allowUsernames = %#v, want normalized username", cfg.allowUsernames)
	}
	if _, ok := cfg.pairedUsernames["bob"]; !ok {
		t.Fatalf("cfg.pairedUsernames = %#v, want normalized username", cfg.pairedUsernames)
	}
	if got, want := normalizeWebhookPath("telegram"), "/telegram"; got != want {
		t.Fatalf("normalizeWebhookPath() = %q, want %q", got, want)
	}
	if got, want := normalizeURL("http://example.com/path/"), "http://example.com/path"; got != want {
		t.Fatalf("normalizeURL() = %q, want %q", got, want)
	}

	bad := managed
	bad.Instance.ProviderConfig = []byte("{")
	if cfg := runtime.resolveInstanceConfig(session, bad); cfg.configError == nil {
		t.Fatal("resolveInstanceConfig(bad json) configError = nil, want non-nil")
	}
}

func TestDetermineInitialStateRetryAndHealthHelpers(t *testing.T) {
	t.Parallel()

	runtime, err := newTelegramProvider(io.Discard)
	if err != nil {
		t.Fatalf("newTelegramProvider() error = %v", err)
	}

	badConfig := errors.New("bad config")
	status, degradation, err := runtime.determineInitialState(
		context.Background(),
		&resolvedInstanceConfig{
			instanceID:  "cfg-err",
			configError: badConfig,
		},
	)
	if !errors.Is(err, badConfig) {
		t.Fatalf("determineInitialState(configError) error = %v, want %v", err, badConfig)
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
		&resolvedInstanceConfig{instanceID: "missing-token"},
	)
	if err == nil {
		t.Fatal("determineInitialState(missing token) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if degradation == nil || degradation.Reason != bridgepkg.BridgeDegradationReasonAuthFailed {
		t.Fatalf("degradation = %#v, want auth failed", degradation)
	}

	runtime.apiFactory = func(cfg *resolvedInstanceConfig) telegramAPI {
		switch cfg.instanceID {
		case "auth":
			return fakeTelegramAPIError{err: &bridgesdk.AuthError{Err: errors.New("invalid token")}}
		case "transient":
			return fakeTelegramAPIError{
				err: &bridgesdk.HTTPError{
					StatusCode: http.StatusServiceUnavailable,
					Message:    "provider unavailable",
				},
			}
		default:
			return &fakeTelegramAPI{}
		}
	}

	status, degradation, err = runtime.determineInitialState(
		context.Background(),
		&resolvedInstanceConfig{
			instanceID: "auth",
			botToken:   "telegram-token",
		},
	)
	if err == nil {
		t.Fatal("determineInitialState(auth) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if degradation == nil || degradation.Reason != bridgepkg.BridgeDegradationReasonAuthFailed {
		t.Fatalf("degradation = %#v, want auth failed", degradation)
	}

	status, degradation, err = runtime.determineInitialState(
		context.Background(),
		&resolvedInstanceConfig{
			instanceID: "transient",
			botToken:   "telegram-token",
		},
	)
	if err == nil {
		t.Fatal("determineInitialState(transient) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusDegraded; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if degradation == nil ||
		degradation.Reason != bridgepkg.BridgeDegradationReasonProviderTimeout {
		t.Fatalf("degradation = %#v, want provider timeout", degradation)
	}

	status, degradation, err = runtime.determineInitialState(
		context.Background(),
		&resolvedInstanceConfig{
			instanceID: "ready",
			botToken:   "telegram-token",
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

	runtime.setLastError(errors.New("boom"))
	if err := runtime.healthCheck(); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("healthCheck() error = %v, want boom", err)
	}
	runtime.clearLastError()
	if err := runtime.healthCheck(); err != nil {
		t.Fatalf("healthCheck() error = %v, want nil", err)
	}
	if got, want := maxInt(0, 2, 1), 2; got != want {
		t.Fatalf("maxInt() = %d, want %d", got, want)
	}
}

func TestTelegramBotClientAndClassificationHelpers(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch filepath.Base(r.URL.Path) {
		case "deleteMessage":
			writeTelegramAPIResponse(t, w, true)
		case "getMe":
			if _, err := w.Write([]byte(`not-json`)); err != nil {
				t.Errorf("write malformed getMe response: %v", err)
			}
		default:
			w.WriteHeader(http.StatusInternalServerError)
			if err := json.NewEncoder(w).Encode(map[string]any{
				"ok":          false,
				"error_code":  http.StatusInternalServerError,
				"description": "broken",
			}); err != nil {
				t.Errorf("encode Telegram error response: %v", err)
			}
		}
	}))
	defer server.Close()

	client := &telegramBotClient{
		baseURL:    server.URL,
		botToken:   "telegram-token",
		httpClient: server.Client(),
	}
	if err := client.DeleteMessage(
		context.Background(),
		telegramDeleteMessageRequest{ChatID: "42", MessageID: 99},
	); err != nil {
		t.Fatalf("DeleteMessage() error = %v", err)
	}
	_, getMeErr := client.GetMe(context.Background())
	if getMeErr == nil {
		t.Fatal("GetMe(invalid json) error = nil, want non-nil")
	}
	if committedGetMeErr, ok := errors.AsType[*bridgesdk.CommittedMutationError](
		getMeErr,
	); ok && committedGetMeErr != nil {
		t.Fatalf("GetMe(invalid json) error = %T %v, want non-committed error", getMeErr, getMeErr)
	}

	t.Run("Should not retry a send after a successful status with a truncated result", func(t *testing.T) {
		t.Parallel()

		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attempts.Add(1)
			if _, err := w.Write([]byte(`{"ok":true,"result":{"message_id":`)); err != nil {
				t.Errorf("write truncated Telegram response: %v", err)
			}
		}))
		defer server.Close()

		client := &telegramBotClient{
			baseURL:    server.URL,
			botToken:   "telegram-token",
			httpClient: server.Client(),
		}
		_, err := sendTelegramOutbound(
			t.Context(),
			client,
			telegramSendMessageRequest{ChatID: "42"},
			telegramOutboundText{Text: "hello", PlainText: "hello"},
		)
		var committedErr *bridgesdk.CommittedMutationError
		if !errors.As(err, &committedErr) {
			t.Fatalf("sendTelegramOutbound() error = %T %v, want CommittedMutationError", err, err)
		}
		if got, want := attempts.Load(), int32(1); got != want {
			t.Fatalf("send attempts = %d, want %d", got, want)
		}
	})

	t.Run("Should preserve rate-limit classification when the error body read and close fail", func(t *testing.T) {
		t.Parallel()

		readErr := errors.New("Telegram error body read failed")
		closeErr := errors.New("Telegram error body close failed")
		body := &telegramPartialErrorResponseBody{
			content:  []byte(`{"ok":false,"error_code":429,"description":"slow down","parameters":{"retry_after":2}}`),
			readErr:  readErr,
			closeErr: closeErr,
		}
		client := &telegramBotClient{
			baseURL:  "https://api.telegram.test",
			botToken: "telegram-token",
			httpClient: &http.Client{Transport: telegramErrorResponseTransport{
				statusCode: http.StatusTooManyRequests,
				headers:    http.Header{"Retry-After": []string{"2"}},
				body:       body,
			}},
		}

		var result telegramSentMessage
		err := client.callTelegram(
			t.Context(),
			"sendMessage",
			telegramSendMessageRequest{ChatID: "42", Text: "hello"},
			&result,
			bridgesdk.HTTPResponseCommitOnSuccessStatus,
		)
		if !errors.Is(err, readErr) {
			t.Fatalf("callTelegram() error = %v, want response body read failure", err)
		}
		if !errors.Is(err, closeErr) {
			t.Fatalf("callTelegram() error = %v, want response body close failure", err)
		}
		var rateLimitErr *bridgesdk.RateLimitError
		if !errors.As(err, &rateLimitErr) || rateLimitErr.RetryAfter != 2*time.Second {
			t.Fatalf("callTelegram() error = %v, want rate limit with two-second retry", err)
		}
		if got, want := bridgesdk.ClassifyError(err).Class, bridgesdk.ErrorClassRateLimit; got != want {
			t.Fatalf("callTelegram() error class = %q, want %q", got, want)
		}
		if !strings.Contains(rateLimitErr.Error(), "slow down") {
			t.Fatalf("callTelegram() rate-limit error = %q, want partial provider body", rateLimitErr.Error())
		}
		if bridgesdk.IsCommittedMutation(err) {
			t.Fatalf("callTelegram() error = %v, want pre-commit rejection", err)
		}
		if !body.closed {
			t.Fatal("response body closed = false after error classification")
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
				if _, err := io.WriteString(w, `{"ok":true,"result":{"message_id":42}}`); err != nil {
					t.Errorf("write redirect target Telegram response: %v", err)
				}
			}))
			defer target.Close()

			redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", target.URL)
				w.WriteHeader(testCase.statusCode)
				if _, err := fmt.Fprintf(
					w,
					`{"ok":false,"error_code":%d,"description":"redirect refused"}`,
					testCase.statusCode,
				); err != nil {
					t.Errorf("write redirect Telegram response: %v", err)
				}
				if got, want := r.Method, http.MethodPost; got != want {
					t.Errorf("redirect source method = %q, want %q", got, want)
				}
			}))
			defer redirect.Close()

			client := &telegramBotClient{
				baseURL:    redirect.URL,
				botToken:   "telegram-token",
				httpClient: redirect.Client(),
			}
			var result telegramSentMessage
			err := client.callTelegram(
				t.Context(),
				"sendMessage",
				telegramSendMessageRequest{ChatID: "42", Text: "hello"},
				&result,
				bridgesdk.HTTPResponseCommitOnSuccessStatus,
			)
			var httpErr *bridgesdk.HTTPError
			if !errors.As(err, &httpErr) || httpErr.StatusCode != testCase.statusCode {
				t.Fatalf(
					"callTelegram(%d) error = %T %v, want first-response HTTPError",
					testCase.statusCode,
					err,
					err,
				)
			}
			if bridgesdk.IsCommittedMutation(err) {
				t.Fatalf("callTelegram(%d) error = %v, want pre-commit rejection", testCase.statusCode, err)
			}
			if got := targetHits.Load(); got != 0 {
				t.Fatalf("redirect target hits = %d, want 0", got)
			}
		})
	}

	t.Run("Should not expose the bot token when request construction fails", func(t *testing.T) {
		t.Parallel()

		const secretToken = "123456:telegram-secret-token"
		client := &telegramBotClient{
			baseURL:  "://invalid-base-url",
			botToken: secretToken,
		}
		var result telegramBotIdentity
		err := client.callTelegram(
			t.Context(),
			"getMe",
			map[string]any{},
			&result,
			bridgesdk.HTTPResponseNoCommit,
		)
		if err == nil {
			t.Fatal("callTelegram(invalid endpoint) error = nil, want request construction failure")
		}
		if strings.Contains(err.Error(), secretToken) {
			t.Fatalf("callTelegram(invalid endpoint) error exposed bot token: %v", err)
		}
	})

	rateErr := classifyTelegramHTTPError(429, telegramAPIEnvelope[json.RawMessage]{
		Description: "slow down",
		Parameters:  telegramAPIErrorDetails{RetryAfter: 2},
	})
	var typedRateErr *bridgesdk.RateLimitError
	if !errors.As(rateErr, &typedRateErr) {
		t.Fatalf("classifyTelegramHTTPError(429) = %T, want *RateLimitError", rateErr)
	}

	authErr := classifyTelegramHTTPError(
		401,
		telegramAPIEnvelope[json.RawMessage]{Description: "unauthorized"},
	)
	var typedAuthErr *bridgesdk.AuthError
	if !errors.As(authErr, &typedAuthErr) {
		t.Fatalf("classifyTelegramHTTPError(401) = %T, want *AuthError", authErr)
	}

	markdownErr := classifyTelegramHTTPError(
		http.StatusBadRequest,
		telegramAPIEnvelope[json.RawMessage]{Description: "Bad Request: can't parse entities"},
	)
	var typedMarkdownErr *telegramMarkdownParseError
	if !errors.As(markdownErr, &typedMarkdownErr) {
		t.Fatalf(
			"classifyTelegramHTTPError(markdown) = %T, want *telegramMarkdownParseError",
			markdownErr,
		)
	}

	httpErr := classifyTelegramHTTPError(0, telegramAPIEnvelope[json.RawMessage]{ErrorCode: 502})
	var typedHTTPErr *bridgesdk.HTTPError
	if !errors.As(httpErr, &typedHTTPErr) {
		t.Fatalf("classifyTelegramHTTPError(default) = %T, want *HTTPError", httpErr)
	}
	if got, want := typedHTTPErr.Message, "telegram bot api error 502"; got != want {
		t.Fatalf("typedHTTPErr.Message = %q, want %q", got, want)
	}

	update := telegramUpdate{EditedMessage: &telegramMessage{MessageID: 1}}
	if message := selectTelegramMessage(update); message == nil || message.MessageID != 1 {
		t.Fatalf("selectTelegramMessage(edited) = %#v, want edited message", message)
	}
	if got, err := resolveTelegramThreadID("654", "-100777"); err != nil {
		t.Fatalf("resolveTelegramThreadID(valid) error = %v", err)
	} else if want := int64(654); got != want {
		t.Fatalf("resolveTelegramThreadID() = %d, want %d", got, want)
	}
	if got, err := resolveTelegramThreadID("1", "-100777"); err != nil {
		t.Fatalf("resolveTelegramThreadID(general topic) error = %v", err)
	} else if got != 0 {
		t.Fatalf("resolveTelegramThreadID(general topic) = %d, want 0", got)
	}
	chatID, messageID, err := decodeRemoteMessageID("chat:321")
	if err != nil {
		t.Fatalf("decodeRemoteMessageID() error = %v", err)
	}
	if got, want := chatID, "chat"; got != want {
		t.Fatalf("chatID = %q, want %q", got, want)
	}
	if got, want := messageID, int64(321); got != want {
		t.Fatalf("messageID = %d, want %d", got, want)
	}
	if _, _, err := decodeRemoteMessageID("bad"); err == nil {
		t.Fatal("decodeRemoteMessageID(bad) error = nil, want non-nil")
	}
	if got, want := referenceRemoteMessageID(
		&bridgepkg.DeliveryMessageReference{RemoteMessageID: " remote "},
	), "remote"; got != want {
		t.Fatalf("referenceRemoteMessageID() = %q, want %q", got, want)
	}
	if got, want := firstNonEmpty("", " value ", "other"), "value"; got != want {
		t.Fatalf("firstNonEmpty() = %q, want %q", got, want)
	}
}

type telegramErrorResponseTransport struct {
	statusCode int
	headers    http.Header
	body       io.ReadCloser
}

func (t telegramErrorResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: t.statusCode,
		Header:     t.headers,
		Body:       t.body,
		Request:    req,
	}, nil
}

type telegramPartialErrorResponseBody struct {
	content  []byte
	readErr  error
	closeErr error
	reads    int
	closed   bool
}

func (b *telegramPartialErrorResponseBody) Read(buffer []byte) (int, error) {
	b.reads++
	if b.reads > 1 {
		return 0, io.EOF
	}
	return copy(buffer, b.content), b.readErr
}

func (b *telegramPartialErrorResponseBody) Close() error {
	b.closed = true
	return b.closeErr
}

func TestWebhookShortCircuitsAndBatchDispatch(t *testing.T) {
	env := setProviderTestEnv(t)

	runtime, err := newTelegramProvider(io.Discard)
	if err != nil {
		t.Fatalf("newTelegramProvider() error = %v", err)
	}
	managed := testBridgeRuntime(time.Date(2026, 4, 15, 14, 9, 0, 0, time.UTC), "brg-1")

	dedupCfg := resolvedInstanceConfig{
		instanceID: "brg-1",
		managed:    &managed,
		dedup:      bridgesdk.NewDedupCache(time.Minute, 10),
		dmPolicy:   bridgepkg.BridgeDMPolicyOpen,
	}
	dedupCfg.dedup.Mark("telegram:brg-1:1001")
	recorder := httptest.NewRecorder()
	if err := runtime.handleWebhookRequest(recorder, nil, &dedupCfg, bridgesdk.WebhookRequest{
		Body:       []byte(telegramWebhookPayload()),
		ReceivedAt: time.Date(2026, 4, 15, 14, 10, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("handleWebhookRequest(dedup) error = %v", err)
	}
	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("recorder.Code = %d, want %d", got, want)
	}

	blockedRecorder := httptest.NewRecorder()
	if err := runtime.handleWebhookRequest(blockedRecorder, nil, &resolvedInstanceConfig{
		instanceID: "brg-1",
		managed:    &managed,
		dedup:      bridgesdk.NewDedupCache(time.Minute, 10),
		dmPolicy:   bridgepkg.BridgeDMPolicyAllowlist,
	}, bridgesdk.WebhookRequest{
		Body:       []byte(telegramWebhookPayload()),
		ReceivedAt: time.Date(2026, 4, 15, 14, 11, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("handleWebhookRequest(blocked dm) error = %v", err)
	}
	if got, want := blockedRecorder.Code, http.StatusOK; got != want {
		t.Fatalf("blockedRecorder.Code = %d, want %d", got, want)
	}

	noopRecorder := httptest.NewRecorder()
	if err := runtime.handleWebhookRequest(noopRecorder, nil, &resolvedInstanceConfig{
		instanceID: "brg-1",
		managed:    &managed,
		dedup:      bridgesdk.NewDedupCache(time.Minute, 10),
		dmPolicy:   bridgepkg.BridgeDMPolicyOpen,
	}, bridgesdk.WebhookRequest{
		Body:       []byte(`{"update_id":2001}`),
		ReceivedAt: time.Date(2026, 4, 15, 14, 12, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("handleWebhookRequest(no message) error = %v", err)
	}
	if got, want := noopRecorder.Code, http.StatusOK; got != want {
		t.Fatalf("noopRecorder.Code = %d, want %d", got, want)
	}

	textlessEditRecorder := httptest.NewRecorder()
	if err := runtime.handleWebhookRequest(textlessEditRecorder, nil, &resolvedInstanceConfig{
		instanceID: "brg-1",
		managed:    &managed,
		dedup:      bridgesdk.NewDedupCache(time.Minute, 10),
		dmPolicy:   bridgepkg.BridgeDMPolicyOpen,
	}, bridgesdk.WebhookRequest{
		Body: []byte(`{
			"update_id":2002,
			"edited_message":{
				"message_id":77,
				"date":1776262200,
				"edit_date":1776262260,
				"chat":{"id":42,"type":"private"},
				"from":{"id":7,"username":"alice"}
			}
		}`),
		ReceivedAt: time.Date(2026, 4, 15, 14, 13, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("handleWebhookRequest(textless edit) error = %v", err)
	}
	if got, want := textlessEditRecorder.Code, http.StatusOK; got != want {
		t.Fatalf("textless edit status = %d, want %d", got, want)
	}

	listenAddr := reserveListenAddr(t)
	mockAPI := newTelegramAPIServer(t)
	t.Setenv(telegramListenAddrEnv, listenAddr)
	t.Setenv(telegramAPIBaseEnv, mockAPI.URL())

	runtimeWithSession, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 14, 15, 0, 0, time.UTC)
	managedRuntime := testBridgeRuntime(now, "brg-1")
	var ingested []bridgepkg.InboundMessageEnvelope
	var mu sync.Mutex

	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesList),
		func(context.Context, json.RawMessage) (any, error) {
			return []bridgepkg.BridgeInstance{managedRuntime.Instance}, nil
		},
	)
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
		func(context.Context, json.RawMessage) (any, error) {
			return managedRuntime.Instance, nil
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
			instance := managedRuntime.Instance
			instance.Status = payload.Status
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
				SessionID: "sess-1",
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

	if err := hostPeer.Call(
		context.Background(),
		"initialize",
		testInitializeRequest(now, managedRuntime),
		nil,
	); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}
	waitForCondition(t, func() bool {
		return runtimeWithSession.currentSession() != nil
	})
	runtimeWithSession.routes.Replace(map[string]resolvedInstanceConfig{
		"brg-1": {
			instanceID: "brg-1",
			dedup:      bridgesdk.NewDedupCache(time.Minute, 10),
		},
	}, nil)

	batch := bridgesdk.InboundBatch{
		Items: []bridgepkg.InboundMessageEnvelope{
			{
				BridgeInstanceID:  "brg-1",
				Scope:             bridgepkg.ScopeWorkspace,
				WorkspaceID:       "ws-telegram",
				GroupID:           "-100777",
				ThreadID:          "654",
				PlatformMessageID: "321",
				ReceivedAt:        now,
				Content:           bridgepkg.MessageContent{Text: "hello"},
				EventFamily:       bridgepkg.InboundEventFamilyMessage,
				IdempotencyKey:    "telegram:brg-1:1",
			},
			{
				BridgeInstanceID:  "brg-1",
				Scope:             bridgepkg.ScopeWorkspace,
				WorkspaceID:       "ws-telegram",
				GroupID:           "-100777",
				ThreadID:          "654",
				PlatformMessageID: "322",
				ReceivedAt:        now,
				Content:           bridgepkg.MessageContent{Text: "world"},
				EventFamily:       bridgepkg.InboundEventFamilyMessage,
				IdempotencyKey:    "telegram:brg-1:2",
			},
		},
	}
	if err := runtimeWithSession.dispatchInboundBatch(context.Background(), "brg-1", batch); err != nil {
		t.Fatalf("dispatchInboundBatch() error = %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if got, want := len(ingested), 1; got != want {
		t.Fatalf("len(ingested) = %d, want %d", got, want)
	}
	if got, want := ingested[0].Content.Text, "hello\nworld"; got != want {
		t.Fatalf("ingested[0].Content.Text = %q, want %q", got, want)
	}
	ingests := waitForJSONLinesFile[bridgesdk.IngestMarker](
		t,
		env.IngestPath,
		func(items []bridgesdk.IngestMarker) bool { return len(items) >= 1 },
	)
	if got, want := ingests[len(ingests)-1].Envelope.Content.Text, "hello\nworld"; got != want {
		t.Fatalf("ingest marker text = %q, want %q", got, want)
	}
}

func TestExecuteDeliveryRejectsMalformedThreadID(t *testing.T) {
	t.Parallel()

	t.Run("Should fail closed when thread id is malformed", func(t *testing.T) {
		t.Parallel()

		api := &fakeTelegramAPI{nextMessageID: 500}
		request := testDeliveryRequest(
			"brg-1",
			"delivery-invalid-thread",
			1,
			bridgepkg.DeliveryEventTypeStart,
			false,
		)
		request.Event.DeliveryTarget.GroupID = "-100777"
		request.Event.DeliveryTarget.ThreadID = "abc"
		request.Event.RoutingKey.GroupID = "-100777"
		request.Event.RoutingKey.ThreadID = "abc"

		_, _, err := executeDelivery(context.Background(), api, request, deliveryState{})
		if err == nil {
			t.Fatal("executeDelivery(invalid thread id) error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "invalid thread id") {
			t.Fatalf("executeDelivery(invalid thread id) error = %v, want invalid-thread-id error", err)
		}
		if got := strings.Join(api.methods, ","); got != "" {
			t.Fatalf("api methods = %q, want no API calls", got)
		}
	})
}

func TestRunRejectsUnsupportedCommand(t *testing.T) {
	t.Parallel()

	if err := run([]string{"bad"}, strings.NewReader(""), io.Discard, io.Discard); err == nil {
		t.Fatal("run(unsupported) error = nil, want non-nil")
	}
}

func TestRunServeReturnsOnEOF(t *testing.T) {
	t.Parallel()

	done := make(chan error, 1)
	go func() {
		done <- run([]string{"serve"}, strings.NewReader(""), io.Discard, io.Discard)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run(serve) error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run(serve) did not return before timeout")
	}
}

type fakeTelegramAPI struct {
	methods                []string
	nextMessageID          int64
	sendRequests           []telegramSendMessageRequest
	successfulSendRequests []telegramSendMessageRequest
	editRequests           []telegramEditMessageTextRequest
	chatActions            []telegramSendChatActionRequest
	reactions              []telegramSetMessageReactionRequest
	deleteRequests         []telegramDeleteMessageRequest
	sendErrors             []error
	sendErrorText          string
	sendError              error
	editErrors             []error
	deleteErrors           []error
	sendResults            []*telegramSentMessage
}

func (f *fakeTelegramAPI) GetMe(context.Context) (*telegramBotIdentity, error) {
	f.methods = append(f.methods, "getMe")
	return &telegramBotIdentity{ID: 1, Username: "aghbot"}, nil
}

func (f *fakeTelegramAPI) SendMessage(
	_ context.Context,
	request telegramSendMessageRequest,
) (*telegramSentMessage, error) {
	f.methods = append(f.methods, "sendMessage")
	f.sendRequests = append(f.sendRequests, request)
	callIndex := len(f.sendRequests) - 1
	if f.sendError != nil && (f.sendErrorText == "" || request.Text == f.sendErrorText) {
		return nil, f.sendError
	}
	if callIndex < len(f.sendErrors) && f.sendErrors[callIndex] != nil {
		return nil, f.sendErrors[callIndex]
	}
	f.successfulSendRequests = append(f.successfulSendRequests, request)
	if callIndex < len(f.sendResults) {
		return f.sendResults[callIndex], nil
	}
	messageID := f.nextMessageID
	f.nextMessageID++
	return &telegramSentMessage{MessageID: messageID}, nil
}

func countTelegramSendRequests(requests []telegramSendMessageRequest, text string) int {
	count := 0
	for _, request := range requests {
		if request.Text == text {
			count++
		}
	}
	return count
}

func testTelegramResumeRequest(request bridgepkg.DeliveryRequest) bridgepkg.DeliveryRequest {
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

func (f *fakeTelegramAPI) EditMessageText(
	_ context.Context,
	request telegramEditMessageTextRequest,
) error {
	f.methods = append(f.methods, "editMessageText")
	f.editRequests = append(f.editRequests, request)
	callIndex := len(f.editRequests) - 1
	if callIndex < len(f.editErrors) {
		return f.editErrors[callIndex]
	}
	return nil
}

func (f *fakeTelegramAPI) DeleteMessage(_ context.Context, request telegramDeleteMessageRequest) error {
	f.methods = append(f.methods, "deleteMessage")
	f.deleteRequests = append(f.deleteRequests, request)
	callIndex := len(f.deleteRequests) - 1
	if callIndex < len(f.deleteErrors) {
		return f.deleteErrors[callIndex]
	}
	return nil
}

func (f *fakeTelegramAPI) SendChatAction(_ context.Context, request telegramSendChatActionRequest) error {
	f.methods = append(f.methods, "sendChatAction")
	f.chatActions = append(f.chatActions, request)
	return nil
}

func (f *fakeTelegramAPI) SetMessageReaction(
	_ context.Context,
	request telegramSetMessageReactionRequest,
) error {
	f.methods = append(f.methods, "setMessageReaction")
	f.reactions = append(f.reactions, request)
	return nil
}

type fakeTelegramAPIError struct {
	err error
}

func (f fakeTelegramAPIError) GetMe(context.Context) (*telegramBotIdentity, error) {
	return nil, f.err
}

func (f fakeTelegramAPIError) SendMessage(
	context.Context,
	telegramSendMessageRequest,
) (*telegramSentMessage, error) {
	return nil, f.err
}

func (f fakeTelegramAPIError) EditMessageText(
	context.Context,
	telegramEditMessageTextRequest,
) error {
	return f.err
}

func (f fakeTelegramAPIError) DeleteMessage(context.Context, telegramDeleteMessageRequest) error {
	return f.err
}

func (f fakeTelegramAPIError) SendChatAction(context.Context, telegramSendChatActionRequest) error {
	return f.err
}

func (f fakeTelegramAPIError) SetMessageReaction(context.Context, telegramSetMessageReactionRequest) error {
	return f.err
}

type telegramAPIServer struct {
	server        *httptest.Server
	mu            sync.Mutex
	calls         []telegramAPICall
	nextMessageID int64
}

type telegramAPICall struct {
	Method string
	Body   map[string]any
}

func newTelegramAPIServer(t *testing.T) *telegramAPIServer {
	t.Helper()

	srv := &telegramAPIServer{nextMessageID: 700}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := filepath.Base(r.URL.Path)
		body := map[string]any{}
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
				t.Errorf("decode Telegram API request: %v", err)
			}
		}
		srv.mu.Lock()
		srv.calls = append(srv.calls, telegramAPICall{Method: method, Body: body})
		srv.mu.Unlock()

		switch method {
		case "getMe":
			writeTelegramAPIResponse(t, w, map[string]any{"id": 1, "username": "aghbot"})
		case "sendMessage":
			srv.mu.Lock()
			messageID := srv.nextMessageID
			srv.nextMessageID++
			srv.mu.Unlock()
			writeTelegramAPIResponse(t, w, map[string]any{"message_id": messageID})
		case "editMessageText", "deleteMessage", "sendChatAction", "setMessageReaction":
			writeTelegramAPIResponse(t, w, true)
		default:
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(map[string]any{
				"ok":          false,
				"error_code":  http.StatusNotFound,
				"description": "unknown method",
			}); err != nil {
				t.Errorf("encode unknown Telegram method response: %v", err)
			}
		}
	}))
	srv.server = server
	t.Cleanup(server.Close)
	return srv
}

func (s *telegramAPIServer) URL() string {
	return s.server.URL
}

func (s *telegramAPIServer) Calls() []telegramAPICall {
	s.mu.Lock()
	defer s.mu.Unlock()
	cloned := make([]telegramAPICall, len(s.calls))
	copy(cloned, s.calls)
	return cloned
}

func writeTelegramAPIResponse(t *testing.T, w http.ResponseWriter, result any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"ok":     true,
		"result": result,
	}); err != nil {
		t.Fatalf("json.NewEncoder().Encode() error = %v", err)
	}
}

func newRuntimePeerPair(t *testing.T) (*telegramProvider, *bridgesdk.Peer, func()) {
	t.Helper()

	hostConn, runtimeConn := net.Pipe()
	runtime, err := newTelegramProvider(io.Discard)
	if err != nil {
		t.Fatalf("newTelegramProvider() error = %v", err)
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

func testBridgeRuntime(
	now time.Time,
	instanceID string,
) subprocess.InitializeBridgeManagedInstance {
	return subprocess.InitializeBridgeManagedInstance{
		Instance: bridgepkg.BridgeInstance{
			ID:            instanceID,
			Scope:         bridgepkg.ScopeWorkspace,
			WorkspaceID:   "ws-telegram",
			Platform:      "telegram",
			ExtensionName: "telegram",
			DisplayName:   "Telegram",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{
				IncludePeer:   true,
				IncludeThread: true,
				IncludeGroup:  true,
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
		BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
			{BindingName: "bot_token", Kind: "token", Value: "telegram-token"},
			{BindingName: "webhook_secret", Kind: "token", Value: "top-secret"},
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
			Name:       "telegram",
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
				Provider:         "telegram",
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
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: instanceID,
				PeerID:           "peer-1",
				Mode:             bridgepkg.DeliveryModeReply,
			},
			Seq:       seq,
			EventType: eventType,
			Content:   bridgepkg.MessageContent{Text: "hello"},
			Final:     final,
		},
	}
}

func testProgressRequest(
	instanceID string,
	deliveryID string,
	seq int64,
	phase bridgepkg.ToolProgressPhase,
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
		ToolCallID: fmt.Sprintf("call-%d", seq),
		ToolID:     "agh__terminal",
		Phase:      phase,
		Label:      label,
		Emoji:      "\U0001f527",
		Index:      int(seq),
	}
	return request
}

func testLifecycleOnlyFinalRequest(instanceID string, deliveryID string, seq int64) bridgepkg.DeliveryRequest {
	request := testDeliveryRequest(
		instanceID,
		deliveryID,
		seq,
		bridgepkg.DeliveryEventTypeFinal,
		true,
	)
	request.Event.Content = bridgepkg.MessageContent{}
	return request
}

func newTelegramProgressTestProvider(
	t *testing.T,
	now func() time.Time,
	api telegramAPI,
) *telegramProvider {
	t.Helper()

	provider, err := newTelegramProvider(io.Discard)
	if err != nil {
		t.Fatalf("newTelegramProvider() error = %v", err)
	}
	managed := testBridgeRuntime(now(), "brg-progress")
	provider.now = now
	provider.routes.Replace(map[string]resolvedInstanceConfig{
		"brg-progress": {
			managed:    &managed,
			instanceID: "brg-progress",
		},
	}, nil)
	provider.apiFactory = func(*resolvedInstanceConfig) telegramAPI { return api }
	t.Cleanup(provider.lifecycle.Stop)
	return provider
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

func telegramWebhookPayload() string {
	return `{"update_id":1001,"message":{"message_id":9,"date":1775866800,"chat":{"id":12345,"type":"private"},"from":{"id":7,"username":"alice","first_name":"Alice"},"text":"hello"}}`
}

func TestTelegramResolvedConfigAndListenErrors(t *testing.T) {
	t.Run("Should classify invalid webhook configuration before startup", func(t *testing.T) {
		t.Parallel()

		missingPath := resolvedInstanceConfig{}
		validateTelegramResolvedConfig(&missingPath)
		if missingPath.configError == nil || !strings.Contains(missingPath.configError.Error(), "webhook path") {
			t.Fatalf("missing path config error = %v", missingPath.configError)
		}

		missingSecret := resolvedInstanceConfig{webhookPath: "/telegram", listenAddr: "127.0.0.1:1"}
		validateTelegramResolvedConfig(&missingSecret)
		if missingSecret.configError == nil || !strings.Contains(missingSecret.configError.Error(), "webhook secret") {
			t.Fatalf("missing secret config error = %v", missingSecret.configError)
		}
	})

	t.Run("Should project missing-listener and startup failures onto valid configs", func(t *testing.T) {
		t.Parallel()

		configs := []resolvedInstanceConfig{{instanceID: "brg-1"}}
		applyTelegramListenErrors(configs, "", func(string) error { return nil })
		if configs[0].configError == nil || !strings.Contains(configs[0].configError.Error(), "listen address") {
			t.Fatalf("missing listener config error = %v", configs[0].configError)
		}

		configs = []resolvedInstanceConfig{{instanceID: "brg-1"}}
		applyTelegramListenErrors(configs, "127.0.0.1:1", func(string) error { return errors.New("listen failed") })
		if configs[0].configError == nil || !strings.Contains(configs[0].configError.Error(), "listen failed") {
			t.Fatalf("startup config error = %v", configs[0].configError)
		}
	})
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
	return fmt.Sprintf("127.0.0.1:%d", testutil.FreeTCPPort(t))
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

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

func TestVerifyDiscordSignatureRejectsInvalidPublicKeySignatures(t *testing.T) {
	t.Parallel()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	body := []byte(`{"hello":"discord"}`)
	timestamp := "1775866800"
	message := append([]byte(timestamp), body...)
	signature := ed25519.Sign(priv, message)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/discord/brg-1", http.NoBody)
	req.Header.Set("X-Signature-Timestamp", timestamp)
	req.Header.Set("X-Signature-Ed25519", hex.EncodeToString(signature))

	now := time.Unix(1775866800, 0).UTC()
	if err := verifyDiscordSignature(context.Background(), req, body, hex.EncodeToString(pub), now); err != nil {
		t.Fatalf("verifyDiscordSignature(valid) error = %v", err)
	}

	req.Header.Set("X-Signature-Ed25519", hex.EncodeToString(ed25519.Sign(priv, []byte("wrong"+string(body)))))
	if err := verifyDiscordSignature(context.Background(), req, body, hex.EncodeToString(pub), now); err == nil {
		t.Fatal("verifyDiscordSignature(invalid) error = nil, want non-nil")
	}
}

func TestMapDiscordMessageEventRoutingAndAttachments(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	managed := testDiscordManagedInstance("brg-discord")

	direct, ignored, err := mapDiscordMessageEvent(discordMessageEvent{
		ID:          "msg-dm-1",
		ChannelID:   "dm-1",
		ChannelType: discordChannelTypeDM,
		Content:     " hello ",
		Timestamp:   now.Format(time.RFC3339Nano),
		Author: discordUser{
			ID:       "user-1",
			Username: "alice",
		},
		Attachments: []discordAttachment{{
			ID:          "att-1",
			Filename:    "report.txt",
			ContentType: "text/plain",
			URL:         "https://cdn.test/report.txt",
		}},
	}, managed, now, "evt-msg-1")
	if err != nil {
		t.Fatalf("mapDiscordMessageEvent(direct) error = %v", err)
	}
	if ignored {
		t.Fatal("mapDiscordMessageEvent(direct) ignored = true, want false")
	}
	if got, want := direct.Envelope.PeerID, "dm-1"; got != want {
		t.Fatalf("PeerID = %q, want %q", got, want)
	}
	if got, want := direct.Envelope.Attachments[0].ID, "att-1"; got != want {
		t.Fatalf("attachment id = %q, want %q", got, want)
	}

	threaded, ignored, err := mapDiscordMessageEvent(discordMessageEvent{
		ID:          "msg-thread-1",
		ChannelID:   "thread-1",
		GuildID:     "guild-1",
		ParentID:    "channel-1",
		ChannelType: discordChannelTypePublicThread,
		Content:     " need summary ",
		Timestamp:   now.Format(time.RFC3339Nano),
		Author: discordUser{
			ID:       "user-2",
			Username: "bob",
		},
	}, managed, now, "evt-msg-2")
	if err != nil {
		t.Fatalf("mapDiscordMessageEvent(threaded) error = %v", err)
	}
	if ignored {
		t.Fatal("mapDiscordMessageEvent(threaded) ignored = true, want false")
	}
	if got, want := threaded.Envelope.GroupID, "channel-1"; got != want {
		t.Fatalf("GroupID = %q, want %q", got, want)
	}
	if got, want := threaded.Envelope.ThreadID, "thread-1"; got != want {
		t.Fatalf("ThreadID = %q, want %q", got, want)
	}
	if got, want := threaded.Envelope.Content.Text, "need summary"; got != want {
		t.Fatalf("Content.Text = %q, want %q", got, want)
	}
}

func TestMapDiscordInteractionPayloadsStableTargetIdentity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 12, 1, 0, 0, time.UTC)
	managed := testDiscordManagedInstance("brg-discord")

	command, err := mapDiscordInteractionCommand(discordInteraction{
		ID:        "ixn-cmd-1",
		Type:      discordInteractionTypeApplicationCommand,
		Token:     "token-cmd-1",
		GuildID:   "guild-1",
		ChannelID: "thread-1",
		Channel: &discordInteractionChannel{
			ID:       "thread-1",
			Type:     discordChannelTypePublicThread,
			ParentID: "channel-1",
		},
		Member: &discordInteractionMember{
			User: &discordUser{
				ID:         "user-1",
				Username:   "alice",
				GlobalName: "Alice",
			},
		},
		Data: &discordInteractionData{
			Name: "agh",
			Options: []discordInteractionOption{{
				Name: "summarize",
				Type: discordApplicationCommandOptionTypeSubcommand,
				Options: []discordInteractionOption{{
					Name:  "topic",
					Value: "release notes",
				}},
			}},
		},
	}, managed, now)
	if err != nil {
		t.Fatalf("mapDiscordInteractionCommand() error = %v", err)
	}
	if got, want := command.Envelope.EventFamily, bridgepkg.InboundEventFamilyCommand; got != want {
		t.Fatalf("EventFamily = %q, want %q", got, want)
	}
	if got, want := command.Envelope.GroupID, "channel-1"; got != want {
		t.Fatalf("GroupID = %q, want %q", got, want)
	}
	if got, want := command.Envelope.ThreadID, "thread-1"; got != want {
		t.Fatalf("ThreadID = %q, want %q", got, want)
	}
	if got, want := command.Envelope.Command.Command, "/agh summarize"; got != want {
		t.Fatalf("Command.Command = %q, want %q", got, want)
	}
	if got, want := command.Envelope.Command.Text, "release notes"; got != want {
		t.Fatalf("Command.Text = %q, want %q", got, want)
	}
	if got, want := command.Envelope.IdempotencyKey, "ixn-cmd-1"; got != want {
		t.Fatalf("IdempotencyKey = %q, want %q", got, want)
	}

	action, err := mapDiscordInteractionAction(discordInteraction{
		ID:        "ixn-action-1",
		Type:      discordInteractionTypeMessageComponent,
		Token:     "token-action-1",
		ChannelID: "dm-1",
		Channel: &discordInteractionChannel{
			ID:   "dm-1",
			Type: discordChannelTypeDM,
		},
		User: &discordUser{
			ID:       "user-2",
			Username: "bob",
		},
		Message: &discordInteractionMessage{ID: "msg-1"},
		Data: &discordInteractionData{
			CustomID:      "approve",
			ComponentType: 2,
			Values:        []string{"yes"},
		},
	}, managed, now)
	if err != nil {
		t.Fatalf("mapDiscordInteractionAction() error = %v", err)
	}
	if got, want := action.Envelope.EventFamily, bridgepkg.InboundEventFamilyAction; got != want {
		t.Fatalf("EventFamily = %q, want %q", got, want)
	}
	if got, want := action.Envelope.PeerID, "dm-1"; got != want {
		t.Fatalf("PeerID = %q, want %q", got, want)
	}
	if got, want := action.Envelope.Action.ActionID, "approve"; got != want {
		t.Fatalf("ActionID = %q, want %q", got, want)
	}
	if got, want := action.Envelope.Action.MessageID, "msg-1"; got != want {
		t.Fatalf("MessageID = %q, want %q", got, want)
	}
	if got, want := action.Envelope.Action.Value, "yes"; got != want {
		t.Fatalf("Value = %q, want %q", got, want)
	}
}

func TestMapDiscordReactionPayloads(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 12, 3, 0, 0, time.UTC)
	managed := testDiscordManagedInstance("brg-discord")

	mapped, err := mapDiscordReactionEvent(discordReactionEvent{
		ChannelID:   "thread-1",
		GuildID:     "guild-1",
		ParentID:    "channel-1",
		ChannelType: discordChannelTypePublicThread,
		MessageID:   "msg-1",
		UserID:      "user-1",
		Emoji:       discordEmoji{Name: "thumbsup"},
		Timestamp:   now.Format(time.RFC3339Nano),
	}, managed, now, "evt-reaction-1", "MESSAGE_REACTION_ADD")
	if err != nil {
		t.Fatalf("mapDiscordReactionEvent(valid) error = %v", err)
	}
	if got, want := mapped.Envelope.EventFamily, bridgepkg.InboundEventFamilyReaction; got != want {
		t.Fatalf("EventFamily = %q, want %q", got, want)
	}
	if got, want := mapped.Envelope.GroupID, "channel-1"; got != want {
		t.Fatalf("GroupID = %q, want %q", got, want)
	}
	if got, want := mapped.Envelope.ThreadID, "thread-1"; got != want {
		t.Fatalf("ThreadID = %q, want %q", got, want)
	}
	if got, want := mapped.Envelope.Reaction.Emoji, ":thumbsup:"; got != want {
		t.Fatalf("Reaction.Emoji = %q, want %q", got, want)
	}
	if !mapped.Envelope.Reaction.Added {
		t.Fatal("Reaction.Added = false, want true")
	}

	if _, err := mapDiscordReactionEvent(discordReactionEvent{
		ChannelID: "channel-1",
		UserID:    "user-1",
	}, managed, now, "evt-bad", "MESSAGE_REACTION_ADD"); err == nil {
		t.Fatal("mapDiscordReactionEvent(malformed) error = nil, want non-nil")
	}
}

func TestExecuteDiscordDeliveryValidatesEditAndDeleteOperations(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	target := bridgepkg.DeliveryTarget{
		BridgeInstanceID: "brg-discord",
		GroupID:          "channel-1",
		ThreadID:         "thread-1",
		Mode:             bridgepkg.DeliveryModeReply,
	}
	routing := bridgepkg.RoutingKey{
		Scope:            bridgepkg.ScopeWorkspace,
		WorkspaceID:      "ws-1",
		BridgeInstanceID: "brg-discord",
		GroupID:          "channel-1",
		ThreadID:         "thread-1",
	}

	postReq := bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "del-1",
			BridgeInstanceID: "brg-discord",
			RoutingKey:       routing,
			DeliveryTarget:   target,
			Seq:              1,
			EventType:        bridgepkg.DeliveryEventTypeStart,
			Final:            false,
			Content:          bridgepkg.MessageContent{Text: "hello"},
		},
	}
	editReq := bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "del-1",
			BridgeInstanceID: "brg-discord",
			RoutingKey:       routing,
			DeliveryTarget:   target,
			Seq:              2,
			EventType:        bridgepkg.DeliveryEventTypeDelta,
			Content:          bridgepkg.MessageContent{Text: "hello world"},
			Operation:        bridgepkg.DeliveryOperationEdit,
			Reference:        &bridgepkg.DeliveryMessageReference{RemoteMessageID: "thread-1:msg-1"},
		},
	}
	invalidEditReq := bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "del-1",
			BridgeInstanceID: "brg-discord",
			RoutingKey:       routing,
			DeliveryTarget:   target,
			Seq:              2,
			EventType:        bridgepkg.DeliveryEventTypeDelta,
			Content:          bridgepkg.MessageContent{Text: "hello world"},
			Operation:        bridgepkg.DeliveryOperationEdit,
		},
	}
	deleteReq := bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "del-1",
			BridgeInstanceID: "brg-discord",
			RoutingKey:       routing,
			DeliveryTarget:   target,
			Seq:              3,
			EventType:        bridgepkg.DeliveryEventTypeDelete,
			Final:            true,
			Operation:        bridgepkg.DeliveryOperationDelete,
			Reference:        &bridgepkg.DeliveryMessageReference{RemoteMessageID: "thread-1:msg-1"},
		},
	}

	api := &discordAPIFake{postedMessageID: "msg-1"}
	ack, state, err := executeDiscordDelivery(ctx, api, postReq, deliveryState{})
	if err != nil {
		t.Fatalf("executeDiscordDelivery(post) error = %v", err)
	}
	if got, want := ack.RemoteMessageID, "thread-1:msg-1"; got != want {
		t.Fatalf("RemoteMessageID = %q, want %q", got, want)
	}
	if got, want := api.postRequests[0].ChannelID, "thread-1"; got != want {
		t.Fatalf("post channel = %q, want %q", got, want)
	}

	if _, _, err := executeDiscordDelivery(ctx, api, invalidEditReq, deliveryState{}); err == nil {
		t.Fatal("executeDiscordDelivery(edit without state) error = nil, want non-nil")
	}

	if _, state, err = executeDiscordDelivery(ctx, api, editReq, state); err != nil {
		t.Fatalf("executeDiscordDelivery(edit with state) error = %v", err)
	}
	if got, want := api.updateRequests[0].MessageID, "msg-1"; got != want {
		t.Fatalf("update message id = %q, want %q", got, want)
	}

	if _, _, err := executeDiscordDelivery(ctx, api, deleteReq, state); err != nil {
		t.Fatalf("executeDiscordDelivery(delete) error = %v", err)
	}
	if got, want := api.deleteRequests[0].MessageID, "msg-1"; got != want {
		t.Fatalf("delete message id = %q, want %q", got, want)
	}

	t.Run("Should edit an explicit reference without delivery state", func(t *testing.T) {
		t.Parallel()

		api := &discordAPIFake{}
		remoteID := "thread-reference:message-reference"
		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "del-explicit-reference",
			BridgeInstanceID: "brg-discord",
			RoutingKey:       routing,
			DeliveryTarget:   target,
			Seq:              1,
			EventType:        bridgepkg.DeliveryEventTypeDelta,
			Operation:        bridgepkg.DeliveryOperationEdit,
			Reference:        &bridgepkg.DeliveryMessageReference{RemoteMessageID: remoteID},
			Content:          bridgepkg.MessageContent{Text: "referenced update"},
		}}

		ack, state, err := executeDiscordDelivery(ctx, api, request, deliveryState{})
		if err != nil {
			t.Fatalf("executeDiscordDelivery() error = %v", err)
		}
		if got := len(api.postRequests); got != 0 {
			t.Fatalf("post requests = %d, want 0", got)
		}
		if got, want := len(api.updateRequests), 1; got != want {
			t.Fatalf("update requests = %d, want %d", got, want)
		}
		if got, want := api.updateRequests[0].ChannelID, "thread-reference"; got != want {
			t.Fatalf("update channel = %q, want %q", got, want)
		}
		if got, want := api.updateRequests[0].MessageID, "message-reference"; got != want {
			t.Fatalf("update message id = %q, want %q", got, want)
		}
		if got := ack.RemoteMessageID; got != remoteID {
			t.Fatalf("ack remote message id = %q, want %q", got, remoteID)
		}
		if got := state.RemoteMessageID; got != remoteID {
			t.Fatalf("state remote message id = %q, want %q", got, remoteID)
		}
	})

	t.Run("Should honor Retry-After for update and delete operations", func(t *testing.T) {
		t.Parallel()

		rateLimit := &bridgesdk.RateLimitError{
			Err:        errors.New("rate limited primary operation"),
			RetryAfter: time.Nanosecond,
		}
		api := &discordAPIFake{
			updateErrors: []error{rateLimit, nil},
			deleteErrors: []error{rateLimit, nil},
		}
		state := deliveryState{LastSeq: 1, RemoteMessageID: "thread-1:message-1", LastContent: "before"}
		update := editReq
		update.Event.DeliveryID = "del-primary-retry"
		update.Event.Content.Text = "updated after rate limit"
		update.Event.Reference = &bridgepkg.DeliveryMessageReference{RemoteMessageID: state.RemoteMessageID}

		ack, state, err := executeDiscordDelivery(context.Background(), api, update, state)
		if err != nil {
			t.Fatalf("executeDiscordDelivery(update) error = %v", err)
		}
		if got, want := len(api.updateAttempts), 2; got != want {
			t.Fatalf("update attempts = %d, want %d", got, want)
		}

		deleteRequest := deleteReq
		deleteRequest.Event.DeliveryID = "del-primary-retry"
		deleteRequest.Event.Reference = &bridgepkg.DeliveryMessageReference{RemoteMessageID: ack.RemoteMessageID}
		if _, _, err := executeDiscordDelivery(context.Background(), api, deleteRequest, state); err != nil {
			t.Fatalf("executeDiscordDelivery(delete) error = %v", err)
		}
		if got, want := len(api.deleteAttempts), 2; got != want {
			t.Fatalf("delete attempts = %d, want %d", got, want)
		}
	})
}

func TestExecuteDiscordDeliveryChunksTerminalContent(t *testing.T) {
	t.Parallel()

	target := bridgepkg.DeliveryTarget{
		BridgeInstanceID: "brg-discord",
		GroupID:          "channel-1",
		ThreadID:         "thread-1",
		Mode:             bridgepkg.DeliveryModeReply,
	}
	routing := bridgepkg.RoutingKey{
		Scope:            bridgepkg.ScopeWorkspace,
		WorkspaceID:      "ws-1",
		BridgeInstanceID: "brg-discord",
		GroupID:          "channel-1",
		ThreadID:         "thread-1",
	}
	longText := strings.Repeat("discord chunk ", 500)

	t.Run("Should post terminal chunks in order and acknowledge the last message", func(t *testing.T) {
		t.Parallel()

		api := &discordAPIFake{postedMessageIDs: []string{"msg-1", "msg-2", "msg-3", "msg-4"}}
		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "del-create",
			BridgeInstanceID: "brg-discord",
			RoutingKey:       routing,
			DeliveryTarget:   target,
			Seq:              1,
			EventType:        bridgepkg.DeliveryEventTypeFinal,
			Final:            true,
			Content:          bridgepkg.MessageContent{Text: longText},
		}}

		ack, _, err := executeDiscordDelivery(context.Background(), api, request, deliveryState{})
		if err != nil {
			t.Fatalf("executeDiscordDelivery() error = %v", err)
		}
		wantChunks := bridgesdk.ChunkMessage(longText, discordMaxMessageLen, nil)
		if got, want := len(api.postRequests), len(wantChunks); got != want {
			t.Fatalf("post requests = %d, want %d chunks", got, want)
		}
		for index, posted := range api.postRequests {
			if got, want := posted.Content, wantChunks[index]; got != want {
				t.Fatalf("chunk %d content = %q, want %q", index, got, want)
			}
			if got := len([]rune(posted.Content)); got > discordMaxMessageLen {
				t.Fatalf("chunk %d length = %d, want <= %d", index, got, discordMaxMessageLen)
			}
			if got, want := posted.ChannelID, "thread-1"; got != want {
				t.Fatalf("chunk %d channel = %q, want %q", index, got, want)
			}
			indicator := fmt.Sprintf("\n\n(%d/%d)", index+1, len(api.postRequests))
			if !strings.HasSuffix(posted.Content, indicator) {
				t.Fatalf("chunk %d content = %q, want suffix %q", index, posted.Content, indicator)
			}
		}
		wantRemoteID := fmt.Sprintf("thread-1:msg-%d", len(api.postRequests))
		if got := ack.RemoteMessageID; got != wantRemoteID {
			t.Fatalf("RemoteMessageID = %q, want %q", got, wantRemoteID)
		}
	})

	t.Run("Should edit the active message then post terminal continuations", func(t *testing.T) {
		t.Parallel()

		api := &discordAPIFake{postedMessageIDs: []string{"msg-1", "msg-2", "msg-3", "msg-4"}}
		state := deliveryState{LastSeq: 1, RemoteMessageID: "thread-1:active"}
		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "del-edit",
			BridgeInstanceID: "brg-discord",
			RoutingKey:       routing,
			DeliveryTarget:   target,
			Seq:              2,
			EventType:        bridgepkg.DeliveryEventTypeFinal,
			Final:            true,
			Operation:        bridgepkg.DeliveryOperationEdit,
			Reference:        &bridgepkg.DeliveryMessageReference{RemoteMessageID: "thread-1:active"},
			Content:          bridgepkg.MessageContent{Text: longText},
		}}

		ack, _, err := executeDiscordDelivery(context.Background(), api, request, state)
		if err != nil {
			t.Fatalf("executeDiscordDelivery() error = %v", err)
		}
		if got, want := len(api.updateRequests), 1; got != want {
			t.Fatalf("update requests = %d, want %d", got, want)
		}
		if len(api.postRequests) == 0 {
			t.Fatal("post requests = 0, want terminal continuations")
		}
		if got, want := api.updateRequests[0].MessageID, "active"; got != want {
			t.Fatalf("updated message id = %q, want %q", got, want)
		}
		wantChunks := bridgesdk.ChunkMessage(longText, discordMaxMessageLen, nil)
		if got, want := len(api.postRequests)+1, len(wantChunks); got != want {
			t.Fatalf("delivered chunks = %d, want %d", got, want)
		}
		if got, want := api.updateRequests[0].Content, wantChunks[0]; got != want {
			t.Fatalf("updated chunk = %q, want %q", got, want)
		}
		totalChunks := len(wantChunks)
		if got := len([]rune(api.updateRequests[0].Content)); got > discordMaxMessageLen {
			t.Fatalf("updated chunk length = %d, want <= %d", got, discordMaxMessageLen)
		}
		indicator := fmt.Sprintf("\n\n(1/%d)", totalChunks)
		if !strings.HasSuffix(api.updateRequests[0].Content, indicator) {
			t.Fatalf("updated chunk = %q, want suffix %q", api.updateRequests[0].Content, indicator)
		}
		for index, posted := range api.postRequests {
			if got, want := posted.Content, wantChunks[index+1]; got != want {
				t.Fatalf("continuation %d = %q, want %q", index, got, want)
			}
			if got := len([]rune(posted.Content)); got > discordMaxMessageLen {
				t.Fatalf("continuation %d length = %d, want <= %d", index, got, discordMaxMessageLen)
			}
			indicator := fmt.Sprintf("\n\n(%d/%d)", index+2, totalChunks)
			if !strings.HasSuffix(posted.Content, indicator) {
				t.Fatalf("continuation %d = %q, want suffix %q", index, posted.Content, indicator)
			}
		}
		wantRemoteID := fmt.Sprintf("thread-1:msg-%d", len(api.postRequests))
		if got := ack.RemoteMessageID; got != wantRemoteID {
			t.Fatalf("RemoteMessageID = %q, want %q", got, wantRemoteID)
		}
		if got, want := ack.ReplaceRemoteMessageID, "thread-1:active"; got != want {
			t.Fatalf("ReplaceRemoteMessageID = %q, want %q", got, want)
		}
	})

	t.Run("Should keep an overflowing non-terminal preview in one message", func(t *testing.T) {
		t.Parallel()

		api := &discordAPIFake{postedMessageIDs: []string{"msg-1", "msg-2"}}
		state := deliveryState{LastSeq: 1, RemoteMessageID: "thread-1:active"}
		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "del-preview",
			BridgeInstanceID: "brg-discord",
			RoutingKey:       routing,
			DeliveryTarget:   target,
			Seq:              2,
			EventType:        bridgepkg.DeliveryEventTypeDelta,
			Operation:        bridgepkg.DeliveryOperationEdit,
			Reference:        &bridgepkg.DeliveryMessageReference{RemoteMessageID: "thread-1:active"},
			Content:          bridgepkg.MessageContent{Text: longText},
		}}

		_, _, err := executeDiscordDelivery(context.Background(), api, request, state)
		if err != nil {
			t.Fatalf("executeDiscordDelivery() error = %v", err)
		}
		if got, want := len(api.updateRequests), 1; got != want {
			t.Fatalf("update requests = %d, want %d", got, want)
		}
		wantPreview := bridgesdk.ChunkMessage(longText, discordMaxMessageLen, nil)[0]
		if got := api.updateRequests[0].Content; got != wantPreview {
			t.Fatalf("preview content = %q, want %q", got, wantPreview)
		}
		if got := len([]rune(api.updateRequests[0].Content)); got > discordMaxMessageLen {
			t.Fatalf("preview length = %d, want <= %d", got, discordMaxMessageLen)
		}
		if !strings.Contains(api.updateRequests[0].Content, "\n\n(1/") {
			t.Fatalf("preview content = %q, want first-chunk indicator", api.updateRequests[0].Content)
		}
		if got := len(api.postRequests); got != 0 {
			t.Fatalf("post requests = %d, want no preview continuations", got)
		}
	})

	t.Run("Should update an explicit reference even when current content matches", func(t *testing.T) {
		t.Parallel()

		api := &discordAPIFake{}
		state := deliveryState{
			LastSeq:         1,
			RemoteMessageID: "thread-1:current",
			LastContent:     "same content",
		}
		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "del-reference",
			BridgeInstanceID: "brg-discord",
			RoutingKey:       routing,
			DeliveryTarget:   target,
			Seq:              2,
			EventType:        bridgepkg.DeliveryEventTypeDelta,
			Operation:        bridgepkg.DeliveryOperationEdit,
			Reference:        &bridgepkg.DeliveryMessageReference{RemoteMessageID: "thread-1:target"},
			Content:          bridgepkg.MessageContent{Text: "same content"},
		}}

		_, _, err := executeDiscordDelivery(context.Background(), api, request, state)
		if err != nil {
			t.Fatalf("executeDiscordDelivery() error = %v", err)
		}
		if got, want := len(api.updateRequests), 1; got != want {
			t.Fatalf("update requests = %d, want %d", got, want)
		}
		if got, want := api.updateRequests[0].MessageID, "target"; got != want {
			t.Fatalf("updated message id = %q, want %q", got, want)
		}
	})

	t.Run("Should reject a post response without a message id", func(t *testing.T) {
		t.Parallel()

		api := &discordAPIFake{}
		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "del-empty-response",
			BridgeInstanceID: "brg-discord",
			RoutingKey:       routing,
			DeliveryTarget:   target,
			Seq:              1,
			EventType:        bridgepkg.DeliveryEventTypeFinal,
			Final:            true,
			Content:          bridgepkg.MessageContent{Text: "hello"},
		}}

		_, _, err := executeDiscordDelivery(context.Background(), api, request, deliveryState{})
		if got, want := bridgesdk.ClassifyError(err).Class, bridgesdk.ErrorClassPermanent; got != want {
			t.Fatalf("executeDiscordDelivery() error class = %q, want %q: %v", got, want, err)
		}
		var committedErr *bridgesdk.CommittedMutationError
		if !errors.As(err, &committedErr) {
			t.Fatalf("executeDiscordDelivery() error = %T, want CommittedMutationError", err)
		}
		if got, want := len(api.postAttempts), 1; got != want {
			t.Fatalf("post attempts = %d, want %d", got, want)
		}
	})

	t.Run("Should resume at the first unsent chunk after exhausting Retry-After attempts", func(t *testing.T) {
		t.Parallel()

		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "del-partial-chunk",
			BridgeInstanceID: "brg-discord",
			RoutingKey:       routing,
			DeliveryTarget:   target,
			Seq:              1,
			EventType:        bridgepkg.DeliveryEventTypeFinal,
			Final:            true,
			Content:          bridgepkg.MessageContent{Text: longText},
		}}
		chunks := discordDeliveryChunks(request.Event)
		if len(chunks) < 2 {
			t.Fatalf("discordDeliveryChunks() returned %d chunk, want at least 2", len(chunks))
		}

		api := &discordAPIFake{
			postedMessageIDs: []string{"msg-1", "msg-2", "msg-3", "msg-4"},
			postErrorContent: chunks[1],
			postErr: &bridgesdk.RateLimitError{
				Err:        errors.New("rate limited continuation"),
				RetryAfter: time.Nanosecond,
			},
		}
		_, partialState, err := executeDiscordDelivery(context.Background(), api, request, deliveryState{})
		if err == nil {
			t.Fatal("executeDiscordDelivery(partial) error = nil, want exhausted rate limit")
		}
		if got, want := countDiscordPostRequests(
			api.postAttempts,
			chunks[1],
		), bridgesdk.DefaultRetryConfig().Attempts; got != want {
			t.Fatalf("failed continuation attempts = %d, want %d", got, want)
		}
		if got, want := countDiscordPostRequests(api.postRequests, chunks[0]), 1; got != want {
			t.Fatalf("successful first-chunk posts = %d, want %d", got, want)
		}

		api.postErr = nil
		resume := testDiscordResumeRequest(request)
		if _, _, err := executeDiscordDelivery(context.Background(), api, resume, partialState); err != nil {
			t.Fatalf("executeDiscordDelivery(resume) error = %v", err)
		}
		if got, want := countDiscordPostRequests(api.postRequests, chunks[0]), 1; got != want {
			t.Fatalf("successful first-chunk posts after resume = %d, want %d", got, want)
		}
		if got, want := len(api.postRequests), len(chunks); got != want {
			t.Fatalf("successful chunk posts = %d, want %d", got, want)
		}
	})
}

func TestDiscordProgressDeliveryRendersAndEditsOneBubble(t *testing.T) {
	t.Run("Should render exact lifecycle lines by editing the created progress bubble", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 15, 12, 30, 0, 0, time.UTC)
		timer := &discordManualProgressTimer{}
		api := &discordAPIFake{postedMessageID: "progress-1"}
		provider, err := newDiscordProvider(io.Discard)
		if err != nil {
			t.Fatalf("newDiscordProvider() error = %v", err)
		}
		provider.now = func() time.Time { return now }
		provider.progressDispatcherOptions = []bridgesdk.ProgressDispatcherOption{
			bridgesdk.WithProgressSchedule(timer.Schedule),
		}
		provider.apiFactory = func(resolvedInstanceConfig) discordAPI { return api }
		setDiscordProviderRoute(provider, "brg-discord", &resolvedInstanceConfig{
			instanceID: "brg-discord",
			managed:    testDiscordManagedInstance("brg-discord"),
		})
		session := &bridgesdk.Session{}

		started := testDiscordProgressRequest(
			"delivery-progress",
			1,
			bridgepkg.ToolProgress{
				ToolCallID: "tool-call-search",
				ToolID:     "agh__search",
				Phase:      bridgepkg.ToolProgressPhaseStarted,
				Label:      "Search",
				Preview:    "project docs",
				Emoji:      "🔎",
				Index:      1,
			},
		)
		ack, err := provider.handleBridgesProgress(context.Background(), session, started)
		if err != nil {
			t.Fatalf("handleBridgesProgress(started) error = %v", err)
		}
		if ack.RemoteMessageID != "" || ack.ReplaceRemoteMessageID != "" {
			t.Fatalf("progress ack = %#v, want no textual remote handles", ack)
		}
		if got, want := api.progressActions, []string{
			"typing:true:thread-1",
			"post:thread-1:🔎 Search project docs",
			"react:thread-1:progress-1:👀",
		}; !slices.Equal(got, want) {
			t.Fatalf("progress actions = %#v, want %#v", got, want)
		}

		now = now.Add(500 * time.Millisecond)
		completed := testDiscordProgressRequest(
			"delivery-progress",
			2,
			bridgepkg.ToolProgress{
				ToolCallID: "tool-call-build",
				ToolID:     "agh__build",
				Phase:      bridgepkg.ToolProgressPhaseCompleted,
				Label:      "Build",
				DurationMS: 31_200,
				Index:      2,
			},
		)
		if _, err := provider.handleBridgesProgress(context.Background(), session, completed); err != nil {
			t.Fatalf("handleBridgesProgress(completed) error = %v", err)
		}
		if got, want := timer.Delay(), time.Second; got != want {
			t.Fatalf("scheduled edit delay = %s, want %s", got, want)
		}
		now = now.Add(time.Second)
		timer.Run(t)
		if got, want := len(api.updateRequests), 1; got != want {
			t.Fatalf("update requests = %d, want %d", got, want)
		}
		if got, want := api.updateRequests[0], (discordUpdateMessageRequest{
			ChannelID: "thread-1",
			MessageID: "progress-1",
			Content:   "🔎 Search project docs\n✅ Build · 31.2s",
		}); got != want {
			t.Fatalf("update request = %#v, want %#v", got, want)
		}
		gotReaction := api.reactionRequests[len(api.reactionRequests)-1]
		if gotReaction.MessageID != "progress-1" || gotReaction.Reaction != "✅" {
			t.Fatalf("completion reaction = %#v, want progress-1 + ✅", gotReaction)
		}

		now = now.Add(1600 * time.Millisecond)
		failed := testDiscordProgressRequest(
			"delivery-progress",
			3,
			bridgepkg.ToolProgress{
				ToolCallID: "tool-call-deploy",
				ToolID:     "agh__deploy",
				Phase:      bridgepkg.ToolProgressPhaseFailed,
				Label:      "Deploy",
				Error:      "permission denied",
				Index:      3,
			},
		)
		if _, err := provider.handleBridgesProgress(context.Background(), session, failed); err != nil {
			t.Fatalf("handleBridgesProgress(failed) error = %v", err)
		}
		if got, want := api.updateRequests[len(api.updateRequests)-1].Content,
			"🔎 Search project docs\n✅ Build · 31.2s\n❌ Deploy — permission denied"; got != want {
			t.Fatalf("failed progress content = %q, want %q", got, want)
		}
		gotReaction = api.reactionRequests[len(api.reactionRequests)-1]
		if gotReaction.MessageID != "progress-1" || gotReaction.Reaction != "❌" {
			t.Fatalf("failure reaction = %#v, want progress-1 + ❌", gotReaction)
		}

		final := testDiscordLifecycleFinalRequest("delivery-progress", 4)
		if _, err := provider.handleBridgesProgress(context.Background(), session, final); err != nil {
			t.Fatalf("handleBridgesProgress(lifecycle final) error = %v", err)
		}
		if got := provider.deliveryState("brg-discord", "delivery-progress").Progress; got != nil {
			t.Fatal("delivery progress dispatcher retained after lifecycle final")
		}
		_, retained := provider.deliveries.Load(deliveryStateKey("brg-discord", "delivery-progress"))
		if retained {
			t.Fatal("lifecycle-only final retained an orphaned delivery state")
		}
	})

	t.Run("Should deliver final text once after a committed progress edit result failure", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 13, 12, 5, 0, 0, time.UTC)
		timer := &discordManualProgressTimer{}
		api := &discordAPIFake{
			postedMessageID: "progress-committed",
			updateErrors: []error{
				bridgesdk.MarkCommittedMutation(errors.New("progress edit result unavailable")),
			},
		}
		provider, err := newDiscordProvider(io.Discard)
		if err != nil {
			t.Fatalf("newDiscordProvider() error = %v", err)
		}
		t.Cleanup(provider.stopResources)
		provider.now = func() time.Time { return now }
		provider.progressDispatcherOptions = []bridgesdk.ProgressDispatcherOption{
			bridgesdk.WithProgressSchedule(timer.Schedule),
		}
		provider.apiFactory = func(resolvedInstanceConfig) discordAPI { return api }
		setDiscordProviderRoute(provider, "brg-discord", &resolvedInstanceConfig{
			instanceID: "brg-discord",
			managed:    testDiscordManagedInstance("brg-discord"),
		})
		session := &bridgesdk.Session{}
		started := testDiscordProgressRequest(
			"delivery-progress-committed",
			1,
			bridgepkg.ToolProgress{
				ToolCallID: "call-progress-started",
				ToolID:     "agh__terminal",
				Phase:      bridgepkg.ToolProgressPhaseStarted,
				Label:      "Inspecting",
				Emoji:      "\U0001f527",
				Index:      1,
			},
		)
		if _, err := provider.handleBridgesProgress(context.Background(), session, started); err != nil {
			t.Fatalf("handleBridgesProgress(started) error = %v", err)
		}
		completed := testDiscordProgressRequest(
			"delivery-progress-committed",
			2,
			bridgepkg.ToolProgress{
				ToolCallID: "call-progress-completed",
				ToolID:     "agh__terminal",
				Phase:      bridgepkg.ToolProgressPhaseCompleted,
				Label:      "Inspecting",
				Emoji:      "\U0001f527",
				Index:      2,
			},
		)
		if _, err := provider.handleBridgesProgress(context.Background(), session, completed); err != nil {
			t.Fatalf("handleBridgesProgress(completed) error = %v", err)
		}
		final := testDiscordLifecycleFinalRequest("delivery-progress-committed", 3)
		final.Event.Content.Text = "final answer"

		ack, err := provider.handleBridgesDeliver(context.Background(), session, final)
		if err != nil {
			t.Fatalf("handleBridgesDeliver(final) error = %v", err)
		}
		if got, want := ack.DeliveryID, final.Event.DeliveryID; got != want {
			t.Fatalf("ack delivery id = %q, want %q", got, want)
		}
		if got, want := len(api.updateAttempts), 1; got != want {
			t.Fatalf("committed progress edit attempts = %d, want %d", got, want)
		}
		if got, want := countDiscordPostRequests(api.postAttempts, "final answer"), 1; got != want {
			t.Fatalf("final text post attempts = %d, want %d", got, want)
		}
	})

	t.Run("Should discard and close progress state after a committed final text result failure", func(t *testing.T) {
		t.Parallel()

		api := &discordAPIFake{postedMessageID: "progress-text-committed"}
		provider, err := newDiscordProvider(io.Discard)
		if err != nil {
			t.Fatalf("newDiscordProvider() error = %v", err)
		}
		t.Cleanup(provider.stopResources)
		provider.apiFactory = func(resolvedInstanceConfig) discordAPI { return api }
		setDiscordProviderRoute(provider, "brg-discord", &resolvedInstanceConfig{
			instanceID: "brg-discord",
			managed:    testDiscordManagedInstance("brg-discord"),
		})
		session := &bridgesdk.Session{}
		started := testDiscordProgressRequest(
			"delivery-text-committed",
			1,
			bridgepkg.ToolProgress{
				ToolCallID: "call-progress-started",
				ToolID:     "agh__terminal",
				Phase:      bridgepkg.ToolProgressPhaseStarted,
				Label:      "Inspecting",
				Emoji:      "\U0001f527",
				Index:      1,
			},
		)
		if _, err := provider.handleBridgesProgress(context.Background(), session, started); err != nil {
			t.Fatalf("handleBridgesProgress(started) error = %v", err)
		}
		dispatcher := provider.deliveryState("brg-discord", "delivery-text-committed").Progress
		if dispatcher == nil {
			t.Fatal("progress dispatcher = nil, want active dispatcher")
		}
		api.postErrorContent = "final answer"
		api.postErr = bridgesdk.MarkCommittedMutation(errors.New("final text result unavailable"))
		final := testDiscordLifecycleFinalRequest("delivery-text-committed", 2)
		final.Event.Content.Text = "final answer"

		_, err = provider.handleBridgesDeliver(context.Background(), session, final)
		if !bridgesdk.IsCommittedMutation(err) {
			t.Fatalf("handleBridgesDeliver(final) error = %T %v, want committed mutation", err, err)
		}
		if got, want := countDiscordPostRequests(api.postAttempts, "final answer"), 1; got != want {
			t.Fatalf("final text post attempts = %d, want %d", got, want)
		}
		if _, retained := provider.deliveries.Load(
			deliveryStateKey("brg-discord", "delivery-text-committed"),
		); retained {
			t.Fatal("committed final text failure retained delivery state")
		}
		if flushErr := dispatcher.Flush(context.Background()); flushErr == nil ||
			!strings.Contains(flushErr.Error(), "closed") {
			t.Fatalf("closed dispatcher Flush() error = %v, want closed error", flushErr)
		}
	})
}

func TestDiscordProgressDeliveryHonorsModeOffBeforeCreatingAPI(t *testing.T) {
	t.Run("Should ack mode-off progress before creating a Discord API client", func(t *testing.T) {
		t.Parallel()

		provider, err := newDiscordProvider(io.Discard)
		if err != nil {
			t.Fatalf("newDiscordProvider() error = %v", err)
		}
		managed := testDiscordManagedInstance("brg-discord-off")
		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"off","grouping":"accumulate","typing":false,"reactions":false}}`,
		)
		setDiscordProviderRoute(provider, managed.Instance.ID, &resolvedInstanceConfig{
			instanceID: managed.Instance.ID,
			managed:    managed,
		})
		apiFactoryCalls := 0
		provider.apiFactory = func(resolvedInstanceConfig) discordAPI {
			apiFactoryCalls++
			return &discordAPIFake{}
		}

		request := testDiscordProgressRequest(
			"delivery-off",
			1,
			bridgepkg.ToolProgress{
				ToolCallID: "tool-call-off",
				ToolID:     "agh__search",
				Phase:      bridgepkg.ToolProgressPhaseStarted,
				Label:      "Search",
				Index:      1,
			},
		)
		request.Event.BridgeInstanceID = managed.Instance.ID
		request.Event.RoutingKey.BridgeInstanceID = managed.Instance.ID
		request.Event.DeliveryTarget.BridgeInstanceID = managed.Instance.ID

		ack, err := provider.handleBridgesProgress(context.Background(), &bridgesdk.Session{}, request)
		if err != nil {
			t.Fatalf("handleBridgesProgress(mode off) error = %v", err)
		}
		if got, want := ack.DeliveryID, request.Event.DeliveryID; got != want {
			t.Fatalf("ack delivery id = %q, want %q", got, want)
		}
		if got := apiFactoryCalls; got != 0 {
			t.Fatalf("apiFactory calls = %d, want 0", got)
		}
		if got := provider.deliveryState(managed.Instance.ID, request.Event.DeliveryID).Progress; got != nil {
			t.Fatal("mode-off progress created a dispatcher")
		}
	})

	t.Run("Should remove delivery state after a progress mode flip and lifecycle final", func(t *testing.T) {
		t.Parallel()

		provider, err := newDiscordProvider(io.Discard)
		if err != nil {
			t.Fatalf("newDiscordProvider() error = %v", err)
		}
		timer := &discordManualProgressTimer{}
		provider.progressDispatcherOptions = []bridgesdk.ProgressDispatcherOption{
			bridgesdk.WithProgressSchedule(timer.Schedule),
		}
		managed := testDiscordManagedInstance("brg-discord-flip")
		setDiscordProviderRoute(provider, managed.Instance.ID, &resolvedInstanceConfig{
			instanceID: managed.Instance.ID,
			managed:    managed,
		})
		apiFactoryCalls := 0
		api := &discordAPIFake{postedMessageID: "progress-flip"}
		provider.apiFactory = func(resolvedInstanceConfig) discordAPI {
			apiFactoryCalls++
			return api
		}
		request := testDiscordProgressRequest("delivery-flip", 1, bridgepkg.ToolProgress{
			ToolCallID: "tool-call-flip",
			ToolID:     "agh__search",
			Phase:      bridgepkg.ToolProgressPhaseStarted,
			Label:      "Search",
			Index:      1,
		})
		request.Event.BridgeInstanceID = managed.Instance.ID
		request.Event.RoutingKey.BridgeInstanceID = managed.Instance.ID
		request.Event.DeliveryTarget.BridgeInstanceID = managed.Instance.ID
		session := &bridgesdk.Session{}
		if _, err := provider.handleBridgesProgress(context.Background(), session, request); err != nil {
			t.Fatalf("handleBridgesProgress(default on) error = %v", err)
		}
		request.Event.Seq = 2
		request.Event.Progress = &bridgepkg.ToolProgress{
			ToolCallID: "tool-call-flip-dirty",
			ToolID:     "agh__read",
			Phase:      bridgepkg.ToolProgressPhaseCompleted,
			Label:      "Read",
			Index:      2,
		}
		if _, err := provider.handleBridgesProgress(context.Background(), session, request); err != nil {
			t.Fatalf("handleBridgesProgress(dirty update) error = %v", err)
		}

		managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"off","grouping":"accumulate","typing":false,"reactions":false}}`,
		)
		provider.routes.Update(managed.Instance.ID, func(cfg resolvedInstanceConfig) resolvedInstanceConfig {
			cfg.managed = managed
			return cfg
		})
		final := testDiscordLifecycleFinalRequest("delivery-flip", 3)
		final.Event.BridgeInstanceID = managed.Instance.ID
		final.Event.RoutingKey.BridgeInstanceID = managed.Instance.ID
		final.Event.DeliveryTarget.BridgeInstanceID = managed.Instance.ID
		if _, err := provider.handleBridgesProgress(context.Background(), session, final); err != nil {
			t.Fatalf("handleBridgesProgress(lifecycle final) error = %v", err)
		}
		if got, want := apiFactoryCalls, 1; got != want {
			t.Fatalf("apiFactory calls = %d, want %d", got, want)
		}
		if got := len(api.updateAttempts); got != 0 {
			t.Fatalf("Discord update attempts after mode flip = %d, want 0", got)
		}
		_, retained := provider.deliveries.Load(deliveryStateKey(managed.Instance.ID, request.Event.DeliveryID))
		if retained {
			t.Fatal("mode-flipped lifecycle final retained delivery state")
		}
	})
}

func TestDiscordProgressSinkRetriesRateLimitedEdits(t *testing.T) {
	t.Run("Should honor Retry-After and keep the throttled edit", func(t *testing.T) {
		t.Parallel()

		api := &discordAPIFake{
			postedMessageID: "progress-1",
			updateErrors: []error{
				&bridgesdk.HTTPError{
					StatusCode: http.StatusTooManyRequests,
					Message:    "slow down",
					RetryAfter: 2 * time.Millisecond,
				},
				nil,
			},
		}
		sink := newDiscordProgressSink(api)
		target := testDiscordProgressRequest(
			"delivery-retry",
			1,
			bridgepkg.ToolProgress{
				ToolCallID: "tool-call-retry",
				ToolID:     "agh__search",
				Phase:      bridgepkg.ToolProgressPhaseStarted,
				Label:      "Search",
				Index:      1,
			},
		).Event.DeliveryTarget
		remoteID, err := sink.Post(context.Background(), target, "👀 Search")
		if err != nil {
			t.Fatalf("Post() error = %v", err)
		}

		startedAt := time.Now()
		if err := sink.Edit(context.Background(), target, remoteID, "👀 Search\n✅ Search"); err != nil {
			t.Fatalf("Edit() error = %v", err)
		}
		if got, want := len(api.updateAttempts), 2; got != want {
			t.Fatalf("update attempts = %d, want %d", got, want)
		}
		if elapsed := time.Since(startedAt); elapsed < 2*time.Millisecond {
			t.Fatalf("retry elapsed = %s, want at least Retry-After 2ms", elapsed)
		}
		if got, want := api.updateRequests[len(api.updateRequests)-1].Content,
			"👀 Search\n✅ Search"; got != want {
			t.Fatalf("successful update content = %q, want %q", got, want)
		}
	})
}

func TestHandleInteractionWebhookAcknowledgesImmediately(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 12, 4, 0, 0, time.UTC)
	provider, err := newDiscordProvider(nil)
	if err != nil {
		t.Fatalf("newDiscordProvider() error = %v", err)
	}
	provider.now = func() time.Time { return now }
	setDiscordProviderRoute(provider, "brg-discord", &resolvedInstanceConfig{
		instanceID: "brg-discord",
		managed:    testDiscordManagedInstance("brg-discord"),
		dedup:      bridgesdk.NewDedupCache(time.Minute, 16),
		dmPolicy:   bridgepkg.BridgeDMPolicyOpen,
	})

	recorder := httptest.NewRecorder()
	cfg := &resolvedInstanceConfig{
		instanceID: "brg-discord",
		managed:    testDiscordManagedInstance("brg-discord"),
		dedup:      bridgesdk.NewDedupCache(time.Minute, 16),
		dmPolicy:   bridgepkg.BridgeDMPolicyOpen,
	}
	err = provider.handleInteractionWebhook(recorder, cfg, bridgepkgToWebhookRequest(t, discordInteraction{
		ID:        "ixn-cmd-1",
		Type:      discordInteractionTypeApplicationCommand,
		Token:     "token-cmd-1",
		ChannelID: "dm-1",
		Channel:   &discordInteractionChannel{ID: "dm-1", Type: discordChannelTypeDM},
		User:      &discordUser{ID: "user-1", Username: "alice"},
		Data:      &discordInteractionData{Name: "agh"},
	}, now))
	if err != nil {
		t.Fatalf("handleInteractionWebhook() error = %v", err)
	}
	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if body := strings.TrimSpace(recorder.Body.String()); body != `{"type":5}` {
		t.Fatalf("body = %s, want {\"type\":5}", body)
	}
}

func TestDiscordBotClientRoutesRequestsAndClassifiesFailures(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve rate-limit classification when the error body read fails", func(t *testing.T) {
		t.Parallel()

		readErr := errors.New("Discord error body read failed")
		body := &discordPartialErrorResponseBody{
			content: []byte("too many requests"),
			readErr: readErr,
		}
		client := &discordBotClient{
			baseURL:  "https://discord.test",
			botToken: "discord-bot-token",
			httpClient: &http.Client{
				Transport: discordRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusTooManyRequests,
						Header:     http.Header{"Retry-After": []string{"2"}},
						Body:       body,
						Request:    req,
					}, nil
				}),
			},
		}

		_, err := client.PostMessage(t.Context(), discordPostMessageRequest{
			ChannelID: "thread-rate-limited",
			Content:   "hello",
		})
		if !errors.Is(err, readErr) {
			t.Fatalf("PostMessage() error = %v, want response body read failure", err)
		}
		var httpErr *bridgesdk.HTTPError
		if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("PostMessage() error = %v, want HTTP 429 classification", err)
		}
		classified := bridgesdk.ClassifyError(err)
		if classified.Class != bridgesdk.ErrorClassRateLimit || classified.RetryAfter != 2*time.Second {
			t.Fatalf("PostMessage() classification = %#v, want rate-limit with two-second retry", classified)
		}
		if !strings.Contains(httpErr.Message, "too many requests") {
			t.Fatalf("PostMessage() HTTP message = %q, want partial provider body", httpErr.Message)
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
		client := &discordBotClient{
			baseURL:  "https://discord.test",
			botToken: "discord-bot-token",
			httpClient: &http.Client{
				Transport: discordRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
					attempts++
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"id":`)),
						Request:    req,
					}, nil
				}),
			},
		}

		_, err := postDiscordDeliveryMessage(t.Context(), client, discordPostMessageRequest{
			ChannelID: "thread-committed",
			Content:   "hello",
		})
		if err == nil {
			t.Fatal("postDiscordDeliveryMessage(truncated response) error = nil, want non-nil")
		}
		var committedErr *bridgesdk.CommittedMutationError
		if !errors.As(err, &committedErr) {
			t.Fatalf("postDiscordDeliveryMessage() error = %T, want CommittedMutationError", err)
		}
		if got, want := attempts, 1; got != want {
			t.Fatalf("transport attempts = %d, want %d", got, want)
		}
	})

	t.Run("Should preserve a materialized message when response cleanup fails", func(t *testing.T) {
		t.Parallel()

		closeErr := &net.OpError{
			Op:  "close",
			Net: "tcp",
			Err: errors.New("connection reset by peer"),
		}
		attempts := 0
		reported := make([]error, 0, 1)
		client := &discordBotClient{
			baseURL:  "https://discord.test",
			botToken: "discord-bot-token",
			httpClient: &http.Client{
				Transport: discordRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
					attempts++
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: &discordResponseBody{
							Reader:   strings.NewReader(`{"id":"msg-committed"}`),
							closeErr: closeErr,
						},
						Request: req,
					}, nil
				}),
			},
			reportResponseCleanup: func(err error) {
				reported = append(reported, err)
			},
		}

		message, err := postDiscordDeliveryMessage(t.Context(), client, discordPostMessageRequest{
			ChannelID: "thread-committed",
			Content:   "hello",
		})
		if err != nil {
			t.Fatalf("postDiscordDeliveryMessage() error = %v", err)
		}
		if message == nil || message.ID != "msg-committed" {
			t.Fatalf("postDiscordDeliveryMessage() message = %#v, want materialized message", message)
		}
		if attempts != 1 {
			t.Fatalf("transport attempts = %d, want 1", attempts)
		}
		if len(reported) != 1 || !errors.Is(reported[0], closeErr) {
			t.Fatalf("reported cleanup errors = %#v, want close failure", reported)
		}
	})

	t.Run("Should preserve a status-confirmed deletion when response cleanup fails", func(t *testing.T) {
		t.Parallel()

		closeErr := &net.OpError{
			Op:  "close",
			Net: "tcp",
			Err: errors.New("connection reset by peer"),
		}
		attempts := 0
		reported := make([]error, 0, 1)
		client := &discordBotClient{
			baseURL:  "https://discord.test",
			botToken: "discord-bot-token",
			httpClient: &http.Client{
				Transport: discordRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
					attempts++
					if attempts > 1 {
						return &http.Response{
							StatusCode: http.StatusNotFound,
							Header:     make(http.Header),
							Body:       io.NopCloser(strings.NewReader("already deleted")),
							Request:    req,
						}, nil
					}
					return &http.Response{
						StatusCode: http.StatusNoContent,
						Header:     make(http.Header),
						Body: &discordResponseBody{
							Reader:   strings.NewReader(""),
							closeErr: closeErr,
						},
						Request: req,
					}, nil
				}),
			},
			reportResponseCleanup: func(err error) {
				reported = append(reported, err)
			},
		}

		err := deleteDiscordDeliveryMessage(t.Context(), client, discordDeleteMessageRequest{
			ChannelID: "thread-committed",
			MessageID: "msg-deleted",
		})
		if err != nil {
			t.Fatalf("deleteDiscordDeliveryMessage() error = %v", err)
		}
		if attempts != 1 {
			t.Fatalf("transport attempts = %d, want 1", attempts)
		}
		if len(reported) != 1 || !errors.Is(reported[0], closeErr) {
			t.Fatalf("reported cleanup errors = %#v, want close failure", reported)
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
				if err := json.NewEncoder(w).Encode(discordPostedMessage{ID: "msg-redirect"}); err != nil {
					t.Errorf("encode redirect target Discord response: %v", err)
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

			client := &discordBotClient{
				baseURL:    redirect.URL,
				botToken:   "discord-bot-token",
				httpClient: redirect.Client(),
			}
			_, err := client.PostMessage(t.Context(), discordPostMessageRequest{
				ChannelID: "thread-redirect",
				Content:   "hello",
			})
			var httpErr *bridgesdk.HTTPError
			if !errors.As(err, &httpErr) || httpErr.StatusCode != testCase.statusCode {
				t.Fatalf(
					"PostMessage(%d) error = %T %v, want first-response HTTPError",
					testCase.statusCode,
					err,
					err,
				)
			}
			if bridgesdk.IsCommittedMutation(err) {
				t.Fatalf("PostMessage(%d) error = %v, want pre-commit rejection", testCase.statusCode, err)
			}
			if got := targetHits.Load(); got != 0 {
				t.Fatalf("redirect target hits = %d, want 0", got)
			}
		})
	}

	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.Method+" "+r.URL.EscapedPath())
		mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/users/@me":
			if err := json.NewEncoder(w).Encode(discordBotIdentity{ID: "bot-1", Username: "agh"}); err != nil {
				t.Errorf("encode Discord bot identity: %v", err)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/channels/thread-1/messages":
			if err := json.NewEncoder(w).Encode(discordPostedMessage{ID: "msg-1"}); err != nil {
				t.Errorf("encode Discord posted message: %v", err)
			}
		case r.Method == http.MethodPatch && r.URL.Path == "/channels/thread-1/messages/msg-1":
			if err := json.NewEncoder(w).Encode(discordPostedMessage{ID: "msg-1"}); err != nil {
				t.Errorf("encode Discord updated message: %v", err)
			}
		case r.Method == http.MethodDelete && r.URL.Path == "/channels/thread-1/messages/msg-1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/channels/thread-1/typing":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPut && r.URL.Path == "/channels/thread-1/messages/msg-1/reactions/👀/@me":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/channels/thread-2/messages":
			w.Header().Set("Retry-After", "2")
			http.Error(w, "too many requests", http.StatusTooManyRequests)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &discordBotClient{
		baseURL:    server.URL,
		botToken:   "discord-bot-token",
		httpClient: server.Client(),
	}

	if _, err := client.GetBotUser(context.Background()); err != nil {
		t.Fatalf("GetBotUser() error = %v", err)
	}
	if _, err := client.PostMessage(context.Background(), discordPostMessageRequest{
		ChannelID: "thread-1",
		Content:   "hello",
	}); err != nil {
		t.Fatalf("PostMessage() error = %v", err)
	}
	if err := client.UpdateMessage(context.Background(), discordUpdateMessageRequest{
		ChannelID: "thread-1",
		MessageID: "msg-1",
		Content:   "hello world",
	}); err != nil {
		t.Fatalf("UpdateMessage() error = %v", err)
	}
	if err := client.DeleteMessage(context.Background(), discordDeleteMessageRequest{
		ChannelID: "thread-1",
		MessageID: "msg-1",
	}); err != nil {
		t.Fatalf("DeleteMessage() error = %v", err)
	}
	if err := client.SetTyping(context.Background(), discordTypingRequest{
		ChannelID: "thread-1",
		Active:    true,
	}); err != nil {
		t.Fatalf("SetTyping(active) error = %v", err)
	}
	if err := client.SetTyping(context.Background(), discordTypingRequest{
		ChannelID: "thread-1",
		Active:    false,
	}); err != nil {
		t.Fatalf("SetTyping(inactive) error = %v", err)
	}
	if err := client.AddReaction(context.Background(), discordReactionRequest{
		ChannelID: "thread-1",
		MessageID: "msg-1",
		Reaction:  "👀",
	}); err != nil {
		t.Fatalf("AddReaction() error = %v", err)
	}
	if _, err := client.PostMessage(context.Background(), discordPostMessageRequest{
		ChannelID: "thread-2",
		Content:   "slow",
	}); err == nil {
		t.Fatal("PostMessage(rate limited) error = nil, want non-nil")
	}

	mu.Lock()
	defer mu.Unlock()
	if got, want := len(paths), 7; got != want {
		t.Fatalf("len(paths) = %d, want %d", got, want)
	}
	for _, want := range []string{
		"POST /channels/thread-1/typing",
		"PUT /channels/thread-1/messages/msg-1/reactions/%F0%9F%91%80/@me",
	} {
		if !slices.Contains(paths, want) {
			t.Fatalf("paths = %#v, want %q", paths, want)
		}
	}
}

type discordRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f discordRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type discordResponseBody struct {
	io.Reader
	closeErr error
}

type discordPartialErrorResponseBody struct {
	content []byte
	readErr error
	reads   int
	closed  bool
}

func (b *discordPartialErrorResponseBody) Read(buffer []byte) (int, error) {
	b.reads++
	if b.reads > 1 {
		return 0, io.EOF
	}
	return copy(buffer, b.content), b.readErr
}

func (b *discordPartialErrorResponseBody) Close() error {
	b.closed = true
	return nil
}

func (b *discordResponseBody) Close() error {
	return b.closeErr
}

func TestServeWebhookHTTPHandlesSignedMessageWebhookWithBatching(t *testing.T) {
	t.Parallel()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	now := time.Now().UTC()
	deliveries := make(chan bridgepkg.InboundMessageEnvelope, 1)
	batcher, err := bridgesdk.NewInboundBatcher(bridgesdk.InboundBatcherConfig{
		Context: context.Background(),
		Delay:   time.Millisecond,
		Dispatch: func(_ context.Context, batch bridgesdk.InboundBatch) error {
			deliveries <- batch.Items[0]
			return nil
		},
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewInboundBatcher() error = %v", err)
	}
	defer batcher.Close()

	provider, err := newDiscordProvider(io.Discard)
	if err != nil {
		t.Fatalf("newDiscordProvider() error = %v", err)
	}
	provider.now = func() time.Time { return now }
	setDiscordProviderRoute(provider, "brg-discord", &resolvedInstanceConfig{
		instanceID:      "brg-discord",
		managed:         testDiscordManagedInstance("brg-discord"),
		webhookPath:     "/discord/brg-discord",
		publicKey:       hex.EncodeToString(pub),
		dedup:           bridgesdk.NewDedupCache(time.Minute, 16),
		rateLimiter:     bridgesdk.NewFixedWindowRateLimiter(10, time.Minute),
		inFlightLimiter: bridgesdk.NewInFlightLimiter(4),
		batcher:         batcher,
		dmPolicy:        bridgepkg.BridgeDMPolicyOpen,
	})

	payload := map[string]any{
		"type": 1,
		"event": map[string]any{
			"id":        "evt-msg-1",
			"type":      "MESSAGE_CREATE",
			"timestamp": now.Format(time.RFC3339Nano),
			"data": map[string]any{
				"id":           "msg-1",
				"channel_id":   "thread-1",
				"guild_id":     "guild-1",
				"parent_id":    "channel-1",
				"channel_type": 11,
				"content":      "Need a summary",
				"timestamp":    now.Format(time.RFC3339Nano),
				"author": map[string]any{
					"id":       "user-1",
					"username": "alice",
				},
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	timestamp := strconv.FormatInt(now.Unix(), 10)
	signature := ed25519.Sign(priv, append([]byte(timestamp), body...))

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://discord.test/discord/brg-discord",
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature-Timestamp", timestamp)
	req.Header.Set("X-Signature-Ed25519", hex.EncodeToString(signature))
	recorder := httptest.NewRecorder()

	provider.serveWebhookHTTP(recorder, req)

	if got, want := recorder.Code, http.StatusNoContent; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	select {
	case envelope := <-deliveries:
		if got, want := envelope.GroupID, "channel-1"; got != want {
			t.Fatalf("GroupID = %q, want %q", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for batched envelope")
	}
}

func TestServeWebhookHTTPRejectsInvalidSignatureBeforeDispatch(t *testing.T) {
	t.Run("Should reject invalid signature without dispatching inbound messages", func(t *testing.T) {
		t.Parallel()

		pub, _, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatalf("GenerateKey(valid) error = %v", err)
		}
		_, wrongPriv, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatalf("GenerateKey(wrong) error = %v", err)
		}
		now := time.Date(2026, 4, 15, 17, 10, 0, 0, time.UTC)
		deliveries := make(chan bridgepkg.InboundMessageEnvelope, 1)
		batcher, err := bridgesdk.NewInboundBatcher(bridgesdk.InboundBatcherConfig{
			Context: context.Background(),
			Delay:   time.Millisecond,
			Dispatch: func(_ context.Context, batch bridgesdk.InboundBatch) error {
				deliveries <- batch.Items[0]
				return nil
			},
			Now: func() time.Time { return now },
		})
		if err != nil {
			t.Fatalf("NewInboundBatcher() error = %v", err)
		}
		defer batcher.Close()

		provider, err := newDiscordProvider(io.Discard)
		if err != nil {
			t.Fatalf("newDiscordProvider() error = %v", err)
		}
		provider.now = func() time.Time { return now }
		setDiscordProviderRoute(provider, "brg-discord", &resolvedInstanceConfig{
			instanceID:      "brg-discord",
			managed:         testDiscordManagedInstance("brg-discord"),
			webhookPath:     "/discord/brg-discord",
			publicKey:       hex.EncodeToString(pub),
			dedup:           bridgesdk.NewDedupCache(time.Minute, 16),
			rateLimiter:     bridgesdk.NewFixedWindowRateLimiter(10, time.Minute),
			inFlightLimiter: bridgesdk.NewInFlightLimiter(4),
			batcher:         batcher,
			dmPolicy:        bridgepkg.BridgeDMPolicyOpen,
		})

		payload := map[string]any{
			"type": 1,
			"event": map[string]any{
				"id":        "evt-msg-1",
				"type":      "MESSAGE_CREATE",
				"timestamp": now.Format(time.RFC3339Nano),
				"data": map[string]any{
					"id":           "msg-1",
					"channel_id":   "thread-1",
					"guild_id":     "guild-1",
					"parent_id":    "channel-1",
					"channel_type": 11,
					"content":      "Need a summary",
					"timestamp":    now.Format(time.RFC3339Nano),
					"author": map[string]any{
						"id":       "user-1",
						"username": "alice",
					},
				},
			},
		}
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		timestamp := strconv.FormatInt(now.Unix(), 10)
		signature := ed25519.Sign(wrongPriv, append([]byte(timestamp), body...))
		req := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"http://discord.test/discord/brg-discord",
			bytes.NewReader(body),
		)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Signature-Timestamp", timestamp)
		req.Header.Set("X-Signature-Ed25519", hex.EncodeToString(signature))
		recorder := httptest.NewRecorder()

		provider.serveWebhookHTTP(recorder, req)

		if got, want := recorder.Code, http.StatusUnauthorized; got != want {
			t.Fatalf("status = %d, want %d; body=%q", got, want, strings.TrimSpace(recorder.Body.String()))
		}
		select {
		case envelope := <-deliveries:
			t.Fatalf("unexpected dispatch for invalid signature: %#v", envelope)
		default:
		}
	})
}

func TestDetermineInitialStateAndLifecycleHelpers(t *testing.T) {
	t.Parallel()

	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	provider, err := newDiscordProvider(io.Discard)
	if err != nil {
		t.Fatalf("newDiscordProvider() error = %v", err)
	}
	provider.apiFactory = func(resolvedInstanceConfig) discordAPI {
		return &discordAPIFake{postedMessageID: "msg-1"}
	}

	status, degradation, err := provider.determineInitialState(context.Background(), &resolvedInstanceConfig{})
	if err == nil || status != bridgepkg.BridgeStatusAuthRequired || degradation == nil {
		t.Fatalf(
			"determineInitialState(missing token) = (%q, %#v, %v), want auth_required with error",
			status,
			degradation,
			err,
		)
	}

	status, degradation, err = provider.determineInitialState(context.Background(), &resolvedInstanceConfig{
		botToken:  "discord-bot-token",
		publicKey: "bad",
	})
	if err == nil || status != bridgepkg.BridgeStatusAuthRequired || degradation == nil {
		t.Fatalf(
			"determineInitialState(invalid key) = (%q, %#v, %v), want auth_required with error",
			status,
			degradation,
			err,
		)
	}

	status, degradation, err = provider.determineInitialState(context.Background(), &resolvedInstanceConfig{
		botToken:      "discord-bot-token",
		publicKey:     hex.EncodeToString(pub),
		applicationID: "bot-1",
	})
	if err != nil {
		t.Fatalf("determineInitialState(valid) error = %v", err)
	}
	if got, want := status, bridgepkg.BridgeStatusReady; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if degradation != nil {
		t.Fatalf("degradation = %#v, want nil", degradation)
	}

	status, degradation, err = provider.determineInitialState(context.Background(), &resolvedInstanceConfig{
		botToken:      "discord-bot-token",
		publicKey:     hex.EncodeToString(pub),
		applicationID: "other-bot",
	})
	if err == nil || status != bridgepkg.BridgeStatusDegraded || degradation == nil {
		t.Fatalf("determineInitialState(mismatch) = (%q, %#v, %v), want degraded with error", status, degradation, err)
	}

	provider.setLastError(errors.New("boom"))
	if err := provider.healthCheck(); err == nil {
		t.Fatal("healthCheck() error = nil, want non-nil")
	}
	provider.clearLastError()
	if err := provider.healthCheck(); err != nil {
		t.Fatalf("healthCheck() error = %v, want nil", err)
	}
}

func TestHandleEventAndInteractionWebhookBranches(t *testing.T) {
	t.Parallel()

	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	provider, err := newDiscordProvider(io.Discard)
	if err != nil {
		t.Fatalf("newDiscordProvider() error = %v", err)
	}
	defer func() {
		provider.lifecycle.Stop()
		if err := provider.lifecycle.Wait(context.Background()); err != nil {
			t.Fatalf("provider lifecycle wait error = %v", err)
		}
	}()
	provider.apiFactory = func(resolvedInstanceConfig) discordAPI {
		return &discordAPIFake{postedMessageID: "msg-1"}
	}
	var mu sync.Mutex
	ingests := make([]bridgepkg.InboundMessageEnvelope, 0)
	cfg := resolvedInstanceConfig{
		instanceID: "brg-discord",
		managed:    testDiscordManagedInstance("brg-discord"),
		publicKey:  hex.EncodeToString(pub),
		dedup:      bridgesdk.NewDedupCache(time.Minute, 16),
		dmPolicy:   bridgepkg.BridgeDMPolicyOpen,
	}
	session := injectedDiscordSession(t,
		bridgesdk.NewHostAPIClientFromCall(func(_ context.Context, method string, params any, result any) error {
			if method == "bridges/messages/ingest" {
				mu.Lock()
				ingests = append(ingests, params.(bridgepkg.InboundMessageEnvelope))
				mu.Unlock()
				target := result.(*bridgepkg.BridgesMessagesIngestResult)
				*target = bridgepkg.BridgesMessagesIngestResult{SessionID: "sess-1"}
				return nil
			}
			if method == "bridges/instances/report_state" {
				target := result.(*bridgepkg.BridgeInstance)
				*target = testDiscordManagedInstance("brg-discord").Instance
				return nil
			}
			return nil
		}),
		bridgesdk.NewInstanceCache(&subprocess.InitializeBridgeRuntime{}),
	)
	if err := provider.handleInitialize(context.Background(), session); err != nil {
		t.Fatalf("handleInitialize() error = %v", err)
	}
	select {
	case <-provider.lifecycle.Initialized():
	case <-t.Context().Done():
		t.Fatal("provider initialization did not finish")
	}
	setDiscordProviderRoute(provider, "brg-discord", &cfg)

	recorder := httptest.NewRecorder()
	if err := provider.handleEventWebhook(
		recorder,
		nil,
		&cfg,
		bridgepkgToWebhookRequest(t, map[string]any{"type": 0}, time.Now().UTC()),
	); err != nil {
		t.Fatalf("handleEventWebhook(ping) error = %v", err)
	}
	if got, want := recorder.Code, http.StatusNoContent; got != want {
		t.Fatalf("ping status = %d, want %d", got, want)
	}

	recorder = httptest.NewRecorder()
	if err := provider.handleEventWebhook(recorder, nil, &cfg, bridgepkgToWebhookRequest(t, map[string]any{
		"type": 1,
		"event": map[string]any{
			"id":        "evt-reaction-1",
			"type":      "MESSAGE_REACTION_ADD",
			"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
			"data": map[string]any{
				"channel_id": "dm-1",
				"message_id": "msg-1",
				"user_id":    "user-1",
				"emoji": map[string]any{
					"name": "thumbsup",
				},
			},
		},
	}, time.Now().UTC())); err != nil {
		t.Fatalf("handleEventWebhook(reaction) error = %v", err)
	}
	if got, want := recorder.Code, http.StatusNoContent; got != want {
		t.Fatalf("reaction status = %d, want %d", got, want)
	}
	mu.Lock()
	reactionIngests := len(ingests)
	mu.Unlock()
	if got, want := reactionIngests, 1; got != want {
		t.Fatalf("reaction ingests = %d, want %d", got, want)
	}

	recorder = httptest.NewRecorder()
	blockedCfg := cfg
	blockedCfg.dedup = bridgesdk.NewDedupCache(time.Minute, 16)
	blockedCfg.dmPolicy = bridgepkg.BridgeDMPolicyAllowlist
	if err := provider.handleEventWebhook(recorder, nil, &blockedCfg, bridgepkgToWebhookRequest(t, map[string]any{
		"type": 1,
		"event": map[string]any{
			"id":        "evt-reaction-blocked",
			"type":      "MESSAGE_REACTION_ADD",
			"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
			"data": map[string]any{
				"channel_id": "dm-2",
				"message_id": "msg-2",
				"user_id":    "user-blocked",
				"emoji": map[string]any{
					"name": "thumbsup",
				},
			},
		},
	}, time.Now().UTC())); err != nil {
		t.Fatalf("handleEventWebhook(blocked reaction) error = %v", err)
	}
	if got, want := recorder.Code, http.StatusNoContent; got != want {
		t.Fatalf("blocked reaction status = %d, want %d", got, want)
	}
	mu.Lock()
	blockedReactionIngests := len(ingests)
	mu.Unlock()
	if got, want := blockedReactionIngests, 1; got != want {
		t.Fatalf("blocked reaction ingests = %d, want %d", got, want)
	}

	recorder = httptest.NewRecorder()
	if err := provider.handleInteractionWebhook(
		recorder,
		&cfg,
		bridgepkgToWebhookRequest(
			t,
			discordInteraction{ID: "ixn-ping", Type: discordInteractionTypePing},
			time.Now().UTC(),
		),
	); err != nil {
		t.Fatalf("handleInteractionWebhook(ping) error = %v", err)
	}
	if got, want := strings.TrimSpace(recorder.Body.String()), `{"type":1}`; got != want {
		t.Fatalf("ping body = %s, want %s", got, want)
	}

	recorder = httptest.NewRecorder()
	if err := provider.handleInteractionWebhook(recorder, &cfg, bridgepkgToWebhookRequest(t, discordInteraction{
		ID:        "ixn-action-1",
		Type:      discordInteractionTypeMessageComponent,
		Token:     "ixn-token-1",
		ChannelID: "dm-1",
		Channel:   &discordInteractionChannel{ID: "dm-1", Type: discordChannelTypeDM},
		User:      &discordUser{ID: "user-1", Username: "alice"},
		Message:   &discordInteractionMessage{ID: "msg-1"},
		Data:      &discordInteractionData{CustomID: "approve"},
	}, time.Now().UTC())); err != nil {
		t.Fatalf("handleInteractionWebhook(action) error = %v", err)
	}
	if got, want := strings.TrimSpace(recorder.Body.String()), `{"type":6}`; got != want {
		t.Fatalf("action body = %s, want %s", got, want)
	}
}

func TestAllowDiscordDirectMessagePoliciesAndUtilityHelpers(t *testing.T) {
	t.Parallel()

	user := discordUserIdentity{ID: "user-1", Username: "alice"}
	if !allowDiscordDirectMessage(&resolvedInstanceConfig{dmPolicy: bridgepkg.BridgeDMPolicyOpen}, user, true) {
		t.Fatal("allowDiscordDirectMessage(open) = false, want true")
	}
	if allowDiscordDirectMessage(&resolvedInstanceConfig{
		dmPolicy:     bridgepkg.BridgeDMPolicyAllowlist,
		allowUserIDs: buildDiscordIDSet([]string{"other"}),
	}, user, true) {
		t.Fatal("allowDiscordDirectMessage(allowlist mismatch) = true, want false")
	}
	if !allowDiscordDirectMessage(&resolvedInstanceConfig{
		dmPolicy:       bridgepkg.BridgeDMPolicyPairing,
		pairedUserIDs:  buildDiscordIDSet([]string{"user-1"}),
		allowUsernames: buildDiscordUsernameSet([]string{"alice"}),
	}, user, true) {
		t.Fatal("allowDiscordDirectMessage(pairing) = false, want true")
	}
	if got := rawDiscordEmoji(discordEmoji{Name: "thumbsup", ID: "123"}); got != "thumbsup:123" {
		t.Fatalf("rawDiscordEmoji() = %q, want thumbsup:123", got)
	}
	if got := normalizeDiscordEmoji(discordEmoji{Name: "wave", ID: "123"}); got != "<:wave:123>" {
		t.Fatalf("normalizeDiscordEmoji(custom) = %q, want <:wave:123>", got)
	}
	if got := parseRetryAfter("2"); got != 2*time.Second {
		t.Fatalf("parseRetryAfter() = %s, want 2s", got)
	}
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve fractional seconds without under-waiting", func(t *testing.T) {
		t.Parallel()

		if got, want := parseRetryAfter("0.75"), 750*time.Millisecond; got != want {
			t.Fatalf("parseRetryAfter(0.75) = %s, want %s", got, want)
		}
	})

	t.Run("Should reject non-positive and invalid delays", func(t *testing.T) {
		t.Parallel()

		for _, value := range []string{"0", "-0.5", "bad"} {
			if got := parseRetryAfter(value); got != 0 {
				t.Fatalf("parseRetryAfter(%q) = %s, want 0", value, got)
			}
		}
	})
}

func TestHandleBridgesDeliverFailureAndMainHelpers(t *testing.T) {
	t.Parallel()

	provider, err := newDiscordProvider(io.Discard)
	if err != nil {
		t.Fatalf("newDiscordProvider() error = %v", err)
	}
	provider.apiFactory = func(resolvedInstanceConfig) discordAPI {
		return &discordAPIFake{
			postErr: &bridgesdk.HTTPError{StatusCode: http.StatusTooManyRequests, Message: "slow down"},
		}
	}
	setDiscordProviderRoute(provider, "brg-discord", &resolvedInstanceConfig{
		instanceID: "brg-discord",
		managed:    testDiscordManagedInstance("brg-discord"),
		dedup:      bridgesdk.NewDedupCache(time.Minute, 16),
	})

	session := injectedDiscordSession(t,
		bridgesdk.NewHostAPIClientFromCall(func(_ context.Context, method string, params any, result any) error {
			if method == "bridges/instances/report_state" {
				target := result.(*bridgepkg.BridgeInstance)
				updated := testDiscordManagedInstance("brg-discord").Instance
				updated.Status = params.(bridgepkg.BridgesInstancesReportStateParams).Status
				*target = updated
			}
			return nil
		}),
		bridgesdk.NewInstanceCache(&subprocess.InitializeBridgeRuntime{}),
	)

	req := bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "del-err-1",
			BridgeInstanceID: "brg-discord",
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-1",
				BridgeInstanceID: "brg-discord",
				GroupID:          "channel-1",
				ThreadID:         "thread-1",
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: "brg-discord",
				GroupID:          "channel-1",
				ThreadID:         "thread-1",
				Mode:             bridgepkg.DeliveryModeReply,
			},
			Seq:       1,
			EventType: bridgepkg.DeliveryEventTypeStart,
			Content:   bridgepkg.MessageContent{Text: "hello"},
		},
	}
	if _, err := provider.handleBridgesDeliver(context.Background(), session, req); err == nil {
		t.Fatal("handleBridgesDeliver(rate limited) error = nil, want non-nil")
	}

	if err := run([]string{"bad"}, bytes.NewBuffer(nil), &bytes.Buffer{}, io.Discard); err == nil {
		t.Fatal("run(bad) error = nil, want non-nil")
	}

	done := make(chan error, 1)
	go func() {
		done <- run(nil, bytes.NewBuffer(nil), &bytes.Buffer{}, io.Discard)
	}()
	select {
	case <-time.After(time.Second):
		t.Fatal("run(nil) timed out, want serve path to exit")
	case <-done:
	}
}

func TestAfterInitializeSuccessAndParsingBranches(t *testing.T) {
	tmpDir := t.TempDir()
	ownershipPath := filepath.Join(tmpDir, "ownership.json")
	statePath := filepath.Join(tmpDir, "state.jsonl")
	t.Setenv(bridgesdk.AdapterHandshakePathEnv, filepath.Join(tmpDir, "handshake.json"))
	t.Setenv(bridgesdk.AdapterOwnershipPathEnv, ownershipPath)
	t.Setenv(bridgesdk.AdapterStatePathEnv, statePath)
	t.Setenv(bridgesdk.AdapterStartsPathEnv, filepath.Join(tmpDir, "starts.log"))

	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	cfg := discordProviderConfig{}
	cfg.Webhook.ListenAddr = "127.0.0.1:0"
	cfg.Webhook.Path = "/discord/brg-discord"
	rawConfig, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	instance := bridgepkg.BridgeInstance{
		ID:             "brg-discord",
		Scope:          bridgepkg.ScopeWorkspace,
		WorkspaceID:    "ws-1",
		ProviderConfig: rawConfig,
	}
	managed := subprocess.InitializeBridgeManagedInstance{
		Instance: instance,
		BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
			{BindingName: "bot_token", Value: "discord-bot-token"},
			{BindingName: "public_key", Value: hex.EncodeToString(pub)},
		},
	}
	session := injectedDiscordSession(t,
		bridgesdk.NewHostAPIClientFromCall(func(_ context.Context, method string, params any, result any) error {
			switch method {
			case "bridges/instances/list":
				target := result.(*[]bridgepkg.BridgeInstance)
				*target = []bridgepkg.BridgeInstance{instance}
			case "bridges/instances/get":
				target := result.(*bridgepkg.BridgeInstance)
				*target = instance
			case "bridges/instances/report_state":
				target := result.(*bridgepkg.BridgeInstance)
				updated := instance
				updated.Status = params.(bridgepkg.BridgesInstancesReportStateParams).Status
				*target = updated
			default:
				return nil
			}
			return nil
		}),
		bridgesdk.NewInstanceCache(&subprocess.InitializeBridgeRuntime{
			RuntimeVersion: subprocess.InitializeBridgeRuntimeVersion2,
			Purpose:        subprocess.BridgeRuntimePurposeService,
			Provider:       "discord",
			Platform:       "discord",
			ManagedInstances: []subprocess.InitializeBridgeManagedInstance{
				managed,
			},
		}),
	)
	provider, err := newDiscordProvider(io.Discard)
	if err != nil {
		t.Fatalf("newDiscordProvider() error = %v", err)
	}
	provider.apiFactory = func(resolvedInstanceConfig) discordAPI {
		return &discordAPIFake{postedMessageID: "msg-1"}
	}
	if err := provider.handleInitialize(context.Background(), session); err != nil {
		t.Fatalf("handleInitialize() error = %v", err)
	}
	select {
	case <-provider.lifecycle.Initialized():
	case <-t.Context().Done():
		t.Fatal("provider initialization did not finish")
	}
	defer func() {
		if err := provider.http.Shutdown(context.Background()); err != nil {
			t.Fatalf("provider HTTP shutdown error = %v", err)
		}
	}()

	if _, err := os.Stat(ownershipPath); err != nil {
		t.Fatalf("ownership marker missing: %v", err)
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("state marker missing: %v", err)
	}

	if got := parseDiscordReceivedAt("", time.Unix(1, 0).UTC()); !got.Equal(time.Unix(1, 0).UTC()) {
		t.Fatalf("parseDiscordReceivedAt(empty) = %s, want fallback", got)
	}
	if got := parseDiscordReceivedAt("1775866800", time.Time{}); got.Unix() != 1775866800 {
		t.Fatalf("parseDiscordReceivedAt(unix) = %d, want 1775866800", got.Unix())
	}
}

func TestResolveInstanceConfigErrorBranchesAndServerGuards(t *testing.T) {
	t.Parallel()

	provider, err := newDiscordProvider(io.Discard)
	if err != nil {
		t.Fatalf("newDiscordProvider() error = %v", err)
	}

	badManaged := subprocess.InitializeBridgeManagedInstance{
		Instance: bridgepkg.BridgeInstance{
			ID:             "brg-bad",
			Scope:          bridgepkg.ScopeWorkspace,
			WorkspaceID:    "ws-1",
			ProviderConfig: []byte(`{`),
		},
	}
	if cfg := provider.resolveInstanceConfig(&bridgesdk.Session{}, badManaged); cfg.configError == nil {
		t.Fatal("resolveInstanceConfig(invalid json) configError = nil, want non-nil")
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/discord/missing", http.NoBody)
	provider.serveWebhookHTTP(recorder, req)
	if got, want := recorder.Code, http.StatusNotFound; got != want {
		t.Fatalf("serveWebhookHTTP(not found) status = %d, want %d", got, want)
	}
}

func TestAdditionalDiscordProviderBranches(t *testing.T) {
	t.Parallel()

	provider, err := newDiscordProvider(io.Discard)
	if err != nil {
		t.Fatalf("newDiscordProvider() error = %v", err)
	}
	provider.apiFactory = func(resolvedInstanceConfig) discordAPI {
		return &discordAPIGetBotUserErrorFake{err: context.DeadlineExceeded}
	}

	status, degradation, err := provider.determineInitialState(context.Background(), &resolvedInstanceConfig{
		configError: errors.New("bad config"),
	})
	if err == nil || status != bridgepkg.BridgeStatusDegraded || degradation == nil {
		t.Fatalf(
			"determineInitialState(configError) = (%q, %#v, %v), want degraded with error",
			status,
			degradation,
			err,
		)
	}

	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	status, degradation, err = provider.determineInitialState(context.Background(), &resolvedInstanceConfig{
		botToken:  "discord-bot-token",
		publicKey: hex.EncodeToString(pub),
	})
	if err == nil || status != bridgepkg.BridgeStatusDegraded || degradation == nil {
		t.Fatalf("determineInitialState(timeout) = (%q, %#v, %v), want degraded with error", status, degradation, err)
	}

	if _, err := provider.waitForInstanceConfig("missing", 20*time.Millisecond); err == nil {
		t.Fatal("waitForInstanceConfig(missing) error = nil, want non-nil")
	}
	if got := parseDiscordReceivedAt(
		time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
		time.Time{},
	); got.IsZero() {
		t.Fatal("parseDiscordReceivedAt(rfc3339) = zero, want parsed time")
	}

	if !shouldPostDiscordMessage(
		bridgepkg.DeliveryEvent{EventType: bridgepkg.DeliveryEventTypeResume},
		deliveryState{},
		bridgepkg.DeliveryRequest{},
	) {
		t.Fatal("shouldPostDiscordMessage(resume without snapshot) = false, want true")
	}

	recorder := httptest.NewRecorder()
	if err := provider.handleWebhookRequest(recorder, nil, &resolvedInstanceConfig{
		instanceID: "brg-discord",
		managed:    testDiscordManagedInstance("brg-discord"),
		dedup:      bridgesdk.NewDedupCache(time.Minute, 16),
		dmPolicy:   bridgepkg.BridgeDMPolicyOpen,
	}, bridgepkgToWebhookRequest(t, map[string]any{
		"type":  2,
		"token": "ixn-token-1",
	}, time.Now().UTC())); err == nil {
		t.Fatal("handleWebhookRequest(invalid interaction) error = nil, want non-nil")
	}
}

func TestDiscordWebhookAndHelperErrorBranches(t *testing.T) {
	t.Parallel()

	provider, err := newDiscordProvider(io.Discard)
	if err != nil {
		t.Fatalf("newDiscordProvider() error = %v", err)
	}
	cfg := resolvedInstanceConfig{
		instanceID: "brg-discord",
		managed:    testDiscordManagedInstance("brg-discord"),
		dedup:      bridgesdk.NewDedupCache(time.Minute, 16),
		dmPolicy:   bridgepkg.BridgeDMPolicyOpen,
	}

	recorder := httptest.NewRecorder()
	err = provider.handleWebhookRequest(recorder, nil, &cfg, bridgesdk.WebhookRequest{
		Body:       []byte("{"),
		ReceivedAt: time.Now().UTC(),
	})
	var httpErr *bridgesdk.HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("handleWebhookRequest(invalid json) error = %v, want bad request http error", err)
	}

	recorder = httptest.NewRecorder()
	err = provider.handleInteractionWebhook(recorder, &cfg, bridgepkgToWebhookRequest(t, discordInteraction{
		ID:    "ixn-unsupported-1",
		Type:  999,
		Token: "ixn-token-1",
	}, time.Now().UTC()))
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("handleInteractionWebhook(unsupported) error = %v, want bad request http error", err)
	}

	recorder = httptest.NewRecorder()
	if err := provider.handleEventWebhook(
		recorder,
		nil,
		&cfg,
		bridgepkgToWebhookRequest(t, map[string]any{"type": 1}, time.Now().UTC()),
	); err != nil {
		t.Fatalf("handleEventWebhook(missing event) error = %v", err)
	}
	if got, want := recorder.Code, http.StatusNoContent; got != want {
		t.Fatalf("handleEventWebhook(missing event) status = %d, want %d", got, want)
	}

	if got := normalizeWebhookPath("discord/brg-discord"); got != "/discord/brg-discord" {
		t.Fatalf("normalizeWebhookPath() = %q, want /discord/brg-discord", got)
	}
	if got := normalizeURL(" https://discord.test/api/ "); got != "https://discord.test/api" {
		t.Fatalf("normalizeURL() = %q, want https://discord.test/api", got)
	}
	if got := referenceRemoteMessageID(nil); got != "" {
		t.Fatalf("referenceRemoteMessageID(nil) = %q, want empty", got)
	}
	if got := referenceRemoteMessageID(
		&bridgepkg.DeliveryMessageReference{RemoteMessageID: " channel-1:msg-1 "},
	); got != "channel-1:msg-1" {
		t.Fatalf("referenceRemoteMessageID(value) = %q, want channel-1:msg-1", got)
	}
	if _, _, err := decodeRemoteMessageID("broken"); err == nil {
		t.Fatal("decodeRemoteMessageID(invalid) error = nil, want non-nil")
	}
	channelID, messageID, err := decodeRemoteMessageID(" channel-1 : msg-1 ")
	if err != nil {
		t.Fatalf("decodeRemoteMessageID(valid) error = %v", err)
	}
	if channelID != "channel-1" || messageID != "msg-1" {
		t.Fatalf("decodeRemoteMessageID(valid) = (%q, %q), want (channel-1, msg-1)", channelID, messageID)
	}
	if got := parseRetryAfter("0"); got != 0 {
		t.Fatalf("parseRetryAfter(0) = %s, want 0", got)
	}
	if got := parseRetryAfter("bad"); got != 0 {
		t.Fatalf("parseRetryAfter(bad) = %s, want 0", got)
	}
}

func TestHandleDiscordEventWebhookUsesRequestContext(t *testing.T) {
	t.Parallel()

	provider, err := newDiscordProvider(io.Discard)
	if err != nil {
		t.Fatalf("newDiscordProvider() error = %v", err)
	}

	managed := testDiscordManagedInstance("brg-discord")
	runtime := &subprocess.InitializeBridgeRuntime{
		RuntimeVersion: subprocess.InitializeBridgeRuntimeVersion2,
		Purpose:        subprocess.BridgeRuntimePurposeService,
		Provider:       "discord",
		Platform:       "discord",
		ManagedInstances: []subprocess.InitializeBridgeManagedInstance{
			managed,
		},
	}
	session := injectedDiscordSession(t,
		bridgesdk.NewHostAPIClientFromCall(func(ctx context.Context, method string, _ any, _ any) error {
			if method != "bridges/messages/ingest" {
				return fmt.Errorf("unexpected host api method %q", method)
			}
			if !errors.Is(ctx.Err(), context.Canceled) {
				t.Fatalf("host call ctx.Err() = %v, want context.Canceled", ctx.Err())
			}
			return context.Canceled
		}),
		bridgesdk.NewInstanceCache(runtime),
	)
	if err := provider.handleInitialize(context.Background(), session); err != nil {
		t.Fatalf("handleInitialize() error = %v", err)
	}
	t.Cleanup(func() {
		provider.lifecycle.Stop()
		if err := provider.lifecycle.Wait(context.Background()); err != nil {
			t.Fatalf("provider lifecycle wait error = %v", err)
		}
	})

	cfg := resolvedInstanceConfig{
		instanceID: "brg-discord",
		managed:    managed,
		dedup:      bridgesdk.NewDedupCache(time.Minute, 16),
		dmPolicy:   bridgepkg.BridgeDMPolicyOpen,
	}
	now := time.Date(2026, 4, 15, 22, 0, 0, 0, time.UTC)
	payload := map[string]any{
		"type": 1,
		"event": map[string]any{
			"id":        "evt-msg-ctx",
			"type":      "MESSAGE_CREATE",
			"timestamp": now.Format(time.RFC3339Nano),
			"data": map[string]any{
				"id":           "msg-ctx",
				"channel_id":   "dm-1",
				"channel_type": 1,
				"content":      "Need context",
				"timestamp":    now.Format(time.RFC3339Nano),
				"author": map[string]any{
					"id":       "user-ctx",
					"username": "alice",
				},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	recorder := httptest.NewRecorder()
	err = provider.handleEventWebhook(
		recorder,
		httptest.NewRequestWithContext(ctx, http.MethodPost, "http://discord.test/discord/brg-discord", http.NoBody),
		&cfg,
		bridgepkgToWebhookRequest(t, payload, now),
	)
	var httpErr *bridgesdk.HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("handleEventWebhook(canceled context) error = %v, want HTTP 500", err)
	}
}

func TestReconcileConfigHelpers(t *testing.T) {
	t.Parallel()

	provider, err := newDiscordProvider(io.Discard)
	if err != nil {
		t.Fatalf("newDiscordProvider() error = %v", err)
	}
	provider.apiFactory = func(resolvedInstanceConfig) discordAPI {
		return &discordAPIFake{postedMessageID: "msg-1"}
	}

	cfg := discordProviderConfig{ApplicationID: "bot-1"}
	cfg.Webhook.ListenAddr = "127.0.0.1:0"
	cfg.Webhook.Path = "/discord/brg-discord"
	rawConfig, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	managed := subprocess.InitializeBridgeManagedInstance{
		Instance: bridgepkg.BridgeInstance{
			ID:             "brg-discord",
			Scope:          bridgepkg.ScopeWorkspace,
			WorkspaceID:    "ws-1",
			DMPolicy:       bridgepkg.BridgeDMPolicyOpen,
			ProviderConfig: rawConfig,
		},
	}

	configs := provider.reconcileInstanceConfigs(
		context.Background(),
		&bridgesdk.Session{},
		[]subprocess.InitializeBridgeManagedInstance{managed},
	)
	if len(configs) != 1 {
		t.Fatalf("len(configs) = %d, want 1", len(configs))
	}
	if got, want := configs[0].apiBaseURL, discordDefaultAPIBaseURL; got != want {
		t.Fatalf("apiBaseURL = %q, want %q", got, want)
	}
	if provider.http.Address() == "" {
		t.Fatal("provider HTTP address is empty after reconciliation")
	}
	if _, err := provider.waitForInstanceConfig("brg-discord", time.Second); err != nil {
		t.Fatalf("waitForInstanceConfig() error = %v", err)
	}
	if _, ok := provider.configForPath("/discord/brg-discord"); !ok {
		t.Fatal("configForPath() ok = false, want true")
	}
}

func TestProviderHostAPIFlowWithInjectedSession(t *testing.T) {
	t.Parallel()

	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	cfg := discordProviderConfig{ApplicationID: "bot-1"}
	cfg.Webhook.ListenAddr = "127.0.0.1:0"
	cfg.Webhook.Path = "/discord/brg-discord"
	cfg.Batching.DelayMS = 1
	cfg.DM.AllowUserIDs = []string{"user-allow"}
	rawConfig, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	instance := bridgepkg.BridgeInstance{
		ID:             "brg-discord",
		Scope:          bridgepkg.ScopeWorkspace,
		WorkspaceID:    "ws-1",
		DMPolicy:       bridgepkg.BridgeDMPolicyAllowlist,
		ProviderConfig: rawConfig,
	}
	managed := subprocess.InitializeBridgeManagedInstance{
		Instance: instance,
		BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
			{BindingName: "bot_token", Value: "discord-bot-token"},
			{BindingName: "public_key", Value: hex.EncodeToString(pub)},
		},
	}
	runtime := &subprocess.InitializeBridgeRuntime{
		RuntimeVersion: subprocess.InitializeBridgeRuntimeVersion2,
		Purpose:        subprocess.BridgeRuntimePurposeService,
		Provider:       "discord",
		Platform:       "discord",
		ManagedInstances: []subprocess.InitializeBridgeManagedInstance{
			managed,
		},
	}

	var mu sync.Mutex
	ingests := make([]bridgepkg.InboundMessageEnvelope, 0)
	reportedStates := make([]bridgepkg.BridgesInstancesReportStateParams, 0)
	session := injectedDiscordSession(t,
		bridgesdk.NewHostAPIClientFromCall(func(_ context.Context, method string, params any, result any) error {
			mu.Lock()
			defer mu.Unlock()
			switch method {
			case "bridges/instances/list":
				target := result.(*[]bridgepkg.BridgeInstance)
				*target = []bridgepkg.BridgeInstance{instance}
				return nil
			case "bridges/instances/get":
				target := result.(*bridgepkg.BridgeInstance)
				*target = instance
				return nil
			case "bridges/instances/report_state":
				reportedStates = append(reportedStates, params.(bridgepkg.BridgesInstancesReportStateParams))
				target := result.(*bridgepkg.BridgeInstance)
				updated := instance
				updated.Status = params.(bridgepkg.BridgesInstancesReportStateParams).Status
				updated.Degradation = params.(bridgepkg.BridgesInstancesReportStateParams).Degradation
				*target = updated
				return nil
			case "bridges/messages/ingest":
				ingests = append(ingests, params.(bridgepkg.InboundMessageEnvelope))
				target := result.(*bridgepkg.BridgesMessagesIngestResult)
				*target = bridgepkg.BridgesMessagesIngestResult{SessionID: "sess-1"}
				return nil
			default:
				return fmt.Errorf("unexpected host api method %q", method)
			}
		}),
		bridgesdk.NewInstanceCache(runtime),
	)

	provider, err := newDiscordProvider(io.Discard)
	if err != nil {
		t.Fatalf("newDiscordProvider() error = %v", err)
	}
	provider.apiFactory = func(resolvedInstanceConfig) discordAPI {
		return &discordAPIFake{postedMessageID: "msg-1"}
	}

	resolved := provider.resolveInstanceConfig(session, managed)
	if got, want := resolved.botToken, "discord-bot-token"; got != want {
		t.Fatalf("botToken = %q, want %q", got, want)
	}
	if got, want := resolved.publicKey, hex.EncodeToString(pub); got != want {
		t.Fatalf("publicKey = %q, want %q", got, want)
	}
	if got, want := resolved.applicationID, "bot-1"; got != want {
		t.Fatalf("applicationID = %q, want %q", got, want)
	}

	configs := provider.reconcileInstanceConfigs(
		context.Background(),
		session,
		[]subprocess.InitializeBridgeManagedInstance{managed},
	)
	defer func() {
		if err := provider.http.Shutdown(context.Background()); err != nil {
			t.Fatalf("provider HTTP shutdown error = %v", err)
		}
		provider.lifecycle.Stop()
	}()
	if len(configs) != 1 {
		t.Fatalf("len(configs) = %d, want 1", len(configs))
	}

	if err := provider.handleInitialize(context.Background(), session); err != nil {
		t.Fatalf("handleInitialize() error = %v", err)
	}

	message := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID:  instance.ID,
		Scope:             instance.Scope,
		WorkspaceID:       instance.WorkspaceID,
		GroupID:           "channel-1",
		ThreadID:          "thread-1",
		PlatformMessageID: "msg-in-1",
		ReceivedAt:        time.Now().UTC(),
		Sender:            bridgepkg.MessageSender{ID: "user-allow"},
		Content:           bridgepkg.MessageContent{Text: "hello"},
		EventFamily:       bridgepkg.InboundEventFamilyMessage,
		IdempotencyKey:    "idem-1",
	}
	if err := provider.dispatchInboundEnvelope(context.Background(), instance.ID, message); err != nil {
		t.Fatalf("dispatchInboundEnvelope() error = %v", err)
	}
	if len(ingests) != 1 {
		t.Fatalf("len(ingests) = %d, want 1", len(ingests))
	}

	if err := provider.dispatchInboundBatch(context.Background(), instance.ID, bridgesdk.InboundBatch{
		Items: []bridgepkg.InboundMessageEnvelope{message, {
			BridgeInstanceID:  instance.ID,
			Scope:             instance.Scope,
			WorkspaceID:       instance.WorkspaceID,
			GroupID:           "channel-1",
			ThreadID:          "thread-1",
			PlatformMessageID: "msg-in-2",
			ReceivedAt:        time.Now().UTC(),
			Sender:            bridgepkg.MessageSender{ID: "user-allow"},
			Content:           bridgepkg.MessageContent{Text: "world"},
			EventFamily:       bridgepkg.InboundEventFamilyMessage,
			IdempotencyKey:    "idem-2",
		}},
	}); err != nil {
		t.Fatalf("dispatchInboundBatch() error = %v", err)
	}
	if len(ingests) != 2 {
		t.Fatalf("len(ingests) = %d, want 2", len(ingests))
	}
	if got, want := ingests[1].Content.Text, "hello\nworld"; got != want {
		t.Fatalf("batched text = %q, want %q", got, want)
	}

	req := bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "del-1",
			BridgeInstanceID: instance.ID,
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-1",
				BridgeInstanceID: instance.ID,
				GroupID:          "channel-1",
				ThreadID:         "thread-1",
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: instance.ID,
				GroupID:          "channel-1",
				ThreadID:         "thread-1",
				Mode:             bridgepkg.DeliveryModeReply,
			},
			Seq:       1,
			EventType: bridgepkg.DeliveryEventTypeStart,
			Content:   bridgepkg.MessageContent{Text: "hello"},
		},
	}
	ack, err := provider.handleBridgesDeliver(context.Background(), session, req)
	if err != nil {
		t.Fatalf("handleBridgesDeliver() error = %v", err)
	}
	if got, want := ack.RemoteMessageID, "thread-1:msg-1"; got != want {
		t.Fatalf("ack.RemoteMessageID = %q, want %q", got, want)
	}
	if got := provider.deliveryState(instance.ID, "del-1").RemoteMessageID; got == "" {
		t.Fatal("deliveryState().RemoteMessageID = empty, want stored value")
	}

	if len(reportedStates) == 0 {
		t.Fatal("reportedStates = 0, want at least one state report")
	}
}

type discordAPIFake struct {
	postedMessageID  string
	postedMessageIDs []string
	postErr          error
	postErrorContent string
	updateErr        error
	updateErrors     []error
	updateStarted    chan struct{}
	updateBlock      <-chan struct{}
	updateStartOnce  sync.Once
	deleteErr        error
	deleteErrors     []error
	typingStopErr    error

	postRequests     []discordPostMessageRequest
	postAttempts     []discordPostMessageRequest
	updateAttempts   []discordUpdateMessageRequest
	updateRequests   []discordUpdateMessageRequest
	deleteRequests   []discordDeleteMessageRequest
	deleteAttempts   []discordDeleteMessageRequest
	typingRequests   []discordTypingRequest
	reactionRequests []discordReactionRequest
	progressActions  []string
}

func (f *discordAPIFake) GetBotUser(context.Context) (*discordBotIdentity, error) {
	return &discordBotIdentity{ID: "bot-1", Username: "agh"}, nil
}

func (f *discordAPIFake) PostMessage(_ context.Context, req discordPostMessageRequest) (*discordPostedMessage, error) {
	f.postAttempts = append(f.postAttempts, req)
	if f.postErr != nil && (f.postErrorContent == "" || req.Content == f.postErrorContent) {
		return nil, f.postErr
	}
	f.postRequests = append(f.postRequests, req)
	f.progressActions = append(f.progressActions, "post:"+req.ChannelID+":"+req.Content)
	messageID := f.postedMessageID
	if index := len(f.postRequests) - 1; index < len(f.postedMessageIDs) {
		messageID = f.postedMessageIDs[index]
	}
	return &discordPostedMessage{ID: messageID}, nil
}

func countDiscordPostRequests(requests []discordPostMessageRequest, content string) int {
	count := 0
	for _, request := range requests {
		if request.Content == content {
			count++
		}
	}
	return count
}

func testDiscordResumeRequest(request bridgepkg.DeliveryRequest) bridgepkg.DeliveryRequest {
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

func (f *discordAPIFake) UpdateMessage(_ context.Context, req discordUpdateMessageRequest) error {
	f.updateAttempts = append(f.updateAttempts, req)
	if f.updateStarted != nil {
		f.updateStartOnce.Do(func() { close(f.updateStarted) })
	}
	if f.updateBlock != nil {
		<-f.updateBlock
	}
	if index := len(f.updateAttempts) - 1; index < len(f.updateErrors) && f.updateErrors[index] != nil {
		return f.updateErrors[index]
	}
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updateRequests = append(f.updateRequests, req)
	f.progressActions = append(f.progressActions, "update:"+req.ChannelID+":"+req.MessageID+":"+req.Content)
	return nil
}

func (f *discordAPIFake) DeleteMessage(_ context.Context, req discordDeleteMessageRequest) error {
	f.deleteAttempts = append(f.deleteAttempts, req)
	callIndex := len(f.deleteAttempts) - 1
	if callIndex < len(f.deleteErrors) && f.deleteErrors[callIndex] != nil {
		return f.deleteErrors[callIndex]
	}
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleteRequests = append(f.deleteRequests, req)
	return nil
}

func (f *discordAPIFake) SetTyping(_ context.Context, req discordTypingRequest) error {
	f.typingRequests = append(f.typingRequests, req)
	f.progressActions = append(
		f.progressActions,
		fmt.Sprintf("typing:%t:%s", req.Active, req.ChannelID),
	)
	if !req.Active && f.typingStopErr != nil {
		return f.typingStopErr
	}
	return nil
}

func (f *discordAPIFake) AddReaction(_ context.Context, req discordReactionRequest) error {
	f.reactionRequests = append(f.reactionRequests, req)
	f.progressActions = append(
		f.progressActions,
		"react:"+req.ChannelID+":"+req.MessageID+":"+req.Reaction,
	)
	return nil
}

type discordManualProgressTimer struct {
	mu       sync.Mutex
	delay    time.Duration
	callback func()
}

func (t *discordManualProgressTimer) Schedule(
	delay time.Duration,
	callback func(),
) bridgesdk.ProgressCancel {
	t.mu.Lock()
	t.delay = delay
	t.callback = callback
	t.mu.Unlock()
	return func() bool {
		t.mu.Lock()
		defer t.mu.Unlock()
		if t.callback == nil {
			return false
		}
		t.callback = nil
		return true
	}
}

func (t *discordManualProgressTimer) Delay() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.delay
}

func (t *discordManualProgressTimer) Run(test *testing.T) {
	test.Helper()
	t.mu.Lock()
	callback := t.callback
	t.callback = nil
	t.mu.Unlock()
	if callback == nil {
		test.Fatal("scheduled progress callback = nil")
	}
	callback()
}

func testDiscordProgressRequest(
	deliveryID string,
	seq int64,
	progress bridgepkg.ToolProgress,
) bridgepkg.DeliveryRequest {
	return bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
		DeliveryID:       deliveryID,
		BridgeInstanceID: "brg-discord",
		RoutingKey: bridgepkg.RoutingKey{
			Scope:            bridgepkg.ScopeWorkspace,
			WorkspaceID:      "ws-1",
			BridgeInstanceID: "brg-discord",
			GroupID:          "channel-1",
			ThreadID:         "thread-1",
		},
		DeliveryTarget: bridgepkg.DeliveryTarget{
			BridgeInstanceID: "brg-discord",
			GroupID:          "channel-1",
			ThreadID:         "thread-1",
			Mode:             bridgepkg.DeliveryModeReply,
		},
		Seq:       seq,
		EventType: bridgepkg.DeliveryEventTypeProgress,
		Operation: bridgepkg.DeliveryOperationPost,
		Progress:  &progress,
	}}
}

func testDiscordLifecycleFinalRequest(deliveryID string, seq int64) bridgepkg.DeliveryRequest {
	request := testDiscordProgressRequest(deliveryID, seq, bridgepkg.ToolProgress{})
	request.Event.EventType = bridgepkg.DeliveryEventTypeFinal
	request.Event.Final = true
	request.Event.Progress = nil
	return request
}

func setDiscordProviderRoute(
	provider *discordProvider,
	instanceID string,
	config *resolvedInstanceConfig,
) {
	provider.routes.Replace(map[string]resolvedInstanceConfig{instanceID: *config}, nil)
}

func testDiscordManagedInstance(id string) subprocess.InitializeBridgeManagedInstance {
	return subprocess.InitializeBridgeManagedInstance{
		Instance: bridgepkg.BridgeInstance{
			ID:          id,
			Scope:       bridgepkg.ScopeWorkspace,
			WorkspaceID: "ws-1",
			DMPolicy:    bridgepkg.BridgeDMPolicyOpen,
		},
	}
}

func bridgepkgToWebhookRequest(t *testing.T, payload any, receivedAt time.Time) bridgesdk.WebhookRequest {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return bridgesdk.WebhookRequest{
		Body:       body,
		ReceivedAt: receivedAt,
	}
}

type discordAPIGetBotUserErrorFake struct {
	err error
}

func (f *discordAPIGetBotUserErrorFake) GetBotUser(context.Context) (*discordBotIdentity, error) {
	return nil, f.err
}

func (f *discordAPIGetBotUserErrorFake) PostMessage(
	context.Context,
	discordPostMessageRequest,
) (*discordPostedMessage, error) {
	return nil, f.err
}

func (f *discordAPIGetBotUserErrorFake) UpdateMessage(context.Context, discordUpdateMessageRequest) error {
	return f.err
}

func (f *discordAPIGetBotUserErrorFake) DeleteMessage(context.Context, discordDeleteMessageRequest) error {
	return f.err
}

func injectedDiscordSession(
	t *testing.T,
	host *bridgesdk.HostAPIClient,
	cache *bridgesdk.InstanceCache,
) *bridgesdk.Session {
	t.Helper()

	session := &bridgesdk.Session{}
	sessionValue := reflect.ValueOf(session).Elem()
	setUnexportedField(t, sessionValue.FieldByName("host"), host)
	setUnexportedField(t, sessionValue.FieldByName("cache"), cache)
	setUnexportedField(t, sessionValue.FieldByName("now"), func() time.Time { return time.Now().UTC() })
	return session
}

func setUnexportedField(t *testing.T, field reflect.Value, value any) {
	t.Helper()

	if !field.IsValid() {
		t.Fatal("setUnexportedField() received invalid field")
	}
	replacement := reflect.ValueOf(value)
	if !replacement.Type().AssignableTo(field.Type()) {
		t.Fatalf("setUnexportedField() type mismatch: got %s want %s", replacement.Type(), field.Type())
	}
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(replacement)
}

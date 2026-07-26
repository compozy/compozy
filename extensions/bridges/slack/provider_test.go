package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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

func TestMapSlackMessageEventRoutingAndAttachments(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-slack")

	direct, ignored, err := mapSlackMessageEvent(slackMessageEvent{
		Type:        "message",
		Channel:     "D123",
		ChannelType: "im",
		User:        "U123",
		Username:    "Alice",
		Text:        " hello ",
		ThreadTS:    "1775866800.500000",
		TS:          "1775866800.100000",
		Files: []slackFile{{
			ID:         "F1",
			Name:       "report.txt",
			MIMEType:   "text/plain",
			URLPrivate: "https://files.example/F1",
		}},
	}, managed, now, "Ev1", "T1", now.Unix(), nil)
	if err != nil {
		t.Fatalf("mapSlackMessageEvent(direct) error = %v", err)
	}
	if ignored {
		t.Fatal("mapSlackMessageEvent(direct) ignored = true, want false")
	}
	if got, want := direct.Envelope.PeerID, "D123"; got != want {
		t.Fatalf("direct.Envelope.PeerID = %q, want %q", got, want)
	}
	if got, want := direct.Envelope.ThreadID, "1775866800.500000"; got != want {
		t.Fatalf("direct.Envelope.ThreadID = %q, want %q", got, want)
	}
	if got, want := direct.Envelope.Attachments[0].ID, "F1"; got != want {
		t.Fatalf("direct attachment id = %q, want %q", got, want)
	}

	channel, ignored, err := mapSlackMessageEvent(slackMessageEvent{
		Type:        "message",
		Channel:     "C777",
		ChannelType: "channel",
		User:        "U777",
		Username:    "bob",
		Text:        " need summary ",
		TS:          "1775866801.100000",
	}, managed, now, "Ev2", "T1", now.Unix(), nil)
	if err != nil {
		t.Fatalf("mapSlackMessageEvent(channel) error = %v", err)
	}
	if ignored {
		t.Fatal("mapSlackMessageEvent(channel) ignored = true, want false")
	}
	if got, want := channel.Envelope.GroupID, "C777"; got != want {
		t.Fatalf("channel.Envelope.GroupID = %q, want %q", got, want)
	}
	if got, want := channel.Envelope.ThreadID, "1775866801.100000"; got != want {
		t.Fatalf("channel.Envelope.ThreadID = %q, want %q", got, want)
	}
	if got, want := channel.Envelope.Content.Text, "need summary"; got != want {
		t.Fatalf("channel.Envelope.Content.Text = %q, want %q", got, want)
	}
}

func TestMapSlackEditAndReplyContext(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 12, 15, 0, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-slack-edits")
	managed.Instance.Scope = bridgepkg.ScopeWorkspace
	managed.Instance.WorkspaceID = "ws-a"

	t.Run("Should map message changed and deleted events to typed edits", func(t *testing.T) {
		t.Parallel()

		parents := bridgesdk.NewParentMessageCache(0)
		var changedEvent slackMessageEvent
		if err := json.Unmarshal([]byte(`{
			"type":"message",
			"subtype":"message_changed",
			"channel":"C123",
			"channel_type":"channel",
			"message":{
				"user":"U123",
				"username":"Alice",
				"text":"corrected answer",
				"ts":"1783864800.123456"
			},
			"previous_message":{
				"user":"U123",
				"username":"Alice",
				"text":"original answer",
				"ts":"1783864800.123456"
			}
		}`), &changedEvent); err != nil {
			t.Fatalf("json.Unmarshal(message_changed) error = %v", err)
		}
		changed, ignored, err := mapSlackMessageEvent(
			changedEvent,
			managed,
			now,
			"Ev-edit",
			"T123",
			now.Unix(),
			parents,
		)
		if err != nil {
			t.Fatalf("mapSlackMessageEvent(changed) error = %v", err)
		}
		if ignored {
			t.Fatal("mapSlackMessageEvent(changed) ignored = true, want false")
		}
		if got, want := changed.Envelope.EventFamily, bridgepkg.InboundEventFamilyEdit; got != want {
			t.Fatalf("changed EventFamily = %q, want %q", got, want)
		}
		if changed.Envelope.Edit == nil {
			t.Fatal("changed Edit = nil, want typed payload")
		}
		if got, want := changed.Envelope.Edit.MessageID, "1783864800.123456"; got != want {
			t.Fatalf("changed Edit.MessageID = %q, want %q", got, want)
		}
		if got, want := changed.Envelope.Edit.NewText, "corrected answer"; got != want {
			t.Fatalf("changed Edit.NewText = %q, want %q", got, want)
		}
		if got, want := changed.Envelope.Edit.Operation, bridgepkg.InboundEditOperationUpdated; got != want {
			t.Fatalf("changed Edit.Operation = %q, want %q", got, want)
		}
		wantOriginal, err := parseSlackTimestamp("1783864800.123456")
		if err != nil {
			t.Fatalf("parseSlackTimestamp() error = %v", err)
		}
		if !changed.Envelope.Edit.OriginalTimestamp.Equal(wantOriginal) {
			t.Fatalf(
				"changed Edit.OriginalTimestamp = %s, want %s",
				changed.Envelope.Edit.OriginalTimestamp,
				wantOriginal,
			)
		}
		if shouldBatchSlackInbound(slackMessageEvent{Subtype: "message_changed"}) {
			t.Fatal("shouldBatchSlackInbound(message_changed) = true, want false")
		}

		deleted, ignored, err := mapSlackMessageEvent(slackMessageEvent{
			Type:        "message",
			Subtype:     "message_deleted",
			Channel:     "C123",
			ChannelType: "channel",
			DeletedTS:   "1783864800.123456",
			PreviousMessage: &slackMessageSnapshot{
				User:     "U123",
				Username: "Alice",
				Text:     "corrected answer",
				TS:       "1783864800.123456",
			},
		}, managed, now.Add(time.Minute), "Ev-delete", "T123", now.Add(time.Minute).Unix(), parents)
		if err != nil {
			t.Fatalf("mapSlackMessageEvent(deleted) error = %v", err)
		}
		if ignored {
			t.Fatal("mapSlackMessageEvent(deleted) ignored = true, want false")
		}
		if deleted.Envelope.Edit == nil ||
			deleted.Envelope.Edit.Operation != bridgepkg.InboundEditOperationDeleted ||
			deleted.Envelope.Edit.NewText != "" {
			t.Fatalf("deleted Edit = %#v, want deleted operation without replacement text", deleted.Envelope.Edit)
		}

		mapped, ignored, err := mapSlackMessageEvent(slackMessageEvent{
			Type:        "message",
			Subtype:     slackMessageSubtypeChanged,
			Channel:     "C123",
			ChannelType: "channel",
			Message: &slackMessageSnapshot{
				TS:   "1783864801.000000",
				User: "U123",
			},
		}, managed, now.Add(2*time.Minute), "Ev-no-text", "T123", now.Add(2*time.Minute).Unix(), parents)
		if err != nil {
			t.Fatalf("mapSlackMessageEvent(textless edit) error = %v", err)
		}
		if !ignored {
			t.Fatalf("mapSlackMessageEvent(textless edit) ignored = false, mapped %#v", mapped)
		}
	})

	t.Run("Should fill threaded reply context from the bounded cache and miss without fetching", func(t *testing.T) {
		t.Parallel()

		parents := bridgesdk.NewParentMessageCache(0)
		rootEvent := slackMessageEvent{
			Type:        "message",
			Channel:     "C123",
			ChannelType: "channel",
			User:        "U-parent",
			Username:    "Bob",
			Text:        "original proposal",
			TS:          "1783864800.100000",
		}
		root, ignored, err := mapSlackMessageEvent(
			rootEvent,
			managed,
			now,
			"Ev-root",
			"T123",
			now.Unix(),
			parents,
		)
		if err != nil || ignored {
			t.Fatalf("mapSlackMessageEvent(root) = (ignored=%v, err=%v), want mapped", ignored, err)
		}
		parents.RememberEnvelope(root.Envelope)

		replyEvent := slackMessageEvent{
			Type:        "message",
			Channel:     "C123",
			ChannelType: "channel",
			User:        "U-reply",
			Username:    "Alice",
			Text:        "I agree",
			ThreadTS:    rootEvent.TS,
			TS:          "1783864860.200000",
		}
		reply, ignored, err := mapSlackMessageEvent(
			replyEvent,
			managed,
			now.Add(time.Minute),
			"Ev-reply",
			"T123",
			now.Add(time.Minute).Unix(),
			parents,
		)
		if err != nil || ignored {
			t.Fatalf("mapSlackMessageEvent(reply) = (ignored=%v, err=%v), want mapped", ignored, err)
		}
		if reply.Envelope.ReplyToText != "original proposal" ||
			reply.Envelope.ReplyToAuthorID != "U-PARENT" ||
			reply.Envelope.ReplyToAuthorName != "Bob" {
			t.Fatalf(
				"reply context = (%q, %q, %q), want cached parent",
				reply.Envelope.ReplyToText,
				reply.Envelope.ReplyToAuthorID,
				reply.Envelope.ReplyToAuthorName,
			)
		}
		if shouldBatchSlackInbound(replyEvent) {
			t.Fatal("shouldBatchSlackInbound(threaded reply) = true, want false")
		}

		cold, ignored, err := mapSlackMessageEvent(
			replyEvent,
			managed,
			now.Add(time.Minute),
			"Ev-cold",
			"T123",
			now.Add(time.Minute).Unix(),
			bridgesdk.NewParentMessageCache(0),
		)
		if err != nil || ignored {
			t.Fatalf("mapSlackMessageEvent(cold reply) = (ignored=%v, err=%v), want mapped", ignored, err)
		}
		if cold.Envelope.ReplyToText != "" ||
			cold.Envelope.ReplyToAuthorID != "" ||
			cold.Envelope.ReplyToAuthorName != "" {
			t.Fatalf("cold reply context = %#v, want empty cache-miss fields", cold.Envelope)
		}
	})
}

func TestMapSlackSlashCommandStableTargetIdentity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 12, 1, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-slack")

	mapped, err := mapSlackSlashCommand(url.Values{
		"command":      []string{"/agh"},
		"text":         []string{"summarize this"},
		"user_id":      []string{"u123"},
		"user_name":    []string{"Alice"},
		"channel_id":   []string{"C123"},
		"channel_name": []string{"general"},
		"team_id":      []string{"T123"},
		"trigger_id":   []string{"1337.42"},
		"response_url": []string{"https://hooks.slack.test/cmd"},
	}, managed, now)
	if err != nil {
		t.Fatalf("mapSlackSlashCommand() error = %v", err)
	}
	if got, want := mapped.Envelope.EventFamily, bridgepkg.InboundEventFamilyCommand; got != want {
		t.Fatalf("EventFamily = %q, want %q", got, want)
	}
	if got, want := mapped.Envelope.GroupID, "C123"; got != want {
		t.Fatalf("GroupID = %q, want %q", got, want)
	}
	if got, want := mapped.Envelope.Command.Command, "/agh"; got != want {
		t.Fatalf("Command.Command = %q, want %q", got, want)
	}
	if got, want := mapped.Envelope.Command.TriggerID, "1337.42"; got != want {
		t.Fatalf("Command.TriggerID = %q, want %q", got, want)
	}
	if got, want := mapped.Envelope.IdempotencyKey, "1337.42"; got != want {
		t.Fatalf("IdempotencyKey = %q, want %q", got, want)
	}

	direct, err := mapSlackSlashCommand(url.Values{
		"command":      []string{"/agh"},
		"user_id":      []string{"U999"},
		"user_name":    []string{"Bob"},
		"channel_id":   []string{"D999"},
		"channel_name": []string{"directmessage"},
	}, managed, now)
	if err != nil {
		t.Fatalf("mapSlackSlashCommand(direct) error = %v", err)
	}
	if got, want := direct.Envelope.PeerID, "D999"; got != want {
		t.Fatalf("direct.PeerID = %q, want %q", got, want)
	}
	if got := direct.Envelope.GroupID; got != "" {
		t.Fatalf("direct.GroupID = %q, want empty", got)
	}
}

func TestMapSlackBlockActionsPreserveIdentifiers(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 12, 2, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-slack")
	payload := slackBlockActionsPayload{
		Type:        "block_actions",
		ResponseURL: "https://hooks.slack.test/action",
		TriggerID:   "trigger-1",
	}
	payload.Channel.ID = "C123"
	payload.Container.ChannelID = "C123"
	payload.Container.MessageTS = "1775866802.100000"
	payload.Container.ThreadTS = "1775866802.000000"
	payload.User.ID = "U123"
	payload.User.Username = "alice"
	payload.Actions = []slackBlockAction{{
		Type:     "button",
		ActionID: "approve",
		BlockID:  "primary",
		Value:    "yes",
		ActionTS: "1775866802.200000",
	}}

	mapped, err := mapSlackBlockActions(payload, managed, now)
	if err != nil {
		t.Fatalf("mapSlackBlockActions() error = %v", err)
	}
	if got, want := len(mapped), 1; got != want {
		t.Fatalf("len(mapped) = %d, want %d", got, want)
	}
	envelope := mapped[0].Envelope
	if got, want := envelope.EventFamily, bridgepkg.InboundEventFamilyAction; got != want {
		t.Fatalf("EventFamily = %q, want %q", got, want)
	}
	if got, want := envelope.GroupID, "C123"; got != want {
		t.Fatalf("GroupID = %q, want %q", got, want)
	}
	if got, want := envelope.ThreadID, "1775866802.000000"; got != want {
		t.Fatalf("ThreadID = %q, want %q", got, want)
	}
	if got, want := envelope.Action.ActionID, "approve"; got != want {
		t.Fatalf("ActionID = %q, want %q", got, want)
	}
	if got, want := envelope.Action.MessageID, "1775866802.100000"; got != want {
		t.Fatalf("MessageID = %q, want %q", got, want)
	}
	if got, want := envelope.Action.Value, "yes"; got != want {
		t.Fatalf("Value = %q, want %q", got, want)
	}
	if got, want := envelope.IdempotencyKey, "1775866802.200000"; got != want {
		t.Fatalf("IdempotencyKey = %q, want %q", got, want)
	}
	if !strings.Contains(string(envelope.ProviderMetadata), `"response_url":"https://hooks.slack.test/action"`) {
		t.Fatalf("ProviderMetadata = %s, want response_url", envelope.ProviderMetadata)
	}
}

func TestMapSlackReactionEventAndRejectMalformed(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 12, 3, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-slack")

	mapped, err := mapSlackReactionEvent(slackReactionEvent{
		Type:     "reaction_added",
		User:     "U555",
		Reaction: "thumbsup",
		EventTS:  "1775866803.100000",
		Item: slackReactionItem{
			Type:    "message",
			Channel: "C555",
			TS:      "1775866803.000000",
		},
	}, managed, now, "Ev555", "T555")
	if err != nil {
		t.Fatalf("mapSlackReactionEvent(valid) error = %v", err)
	}
	if got, want := mapped.Envelope.EventFamily, bridgepkg.InboundEventFamilyReaction; got != want {
		t.Fatalf("EventFamily = %q, want %q", got, want)
	}
	if got, want := mapped.Envelope.Reaction.Emoji, ":thumbsup:"; got != want {
		t.Fatalf("Reaction.Emoji = %q, want %q", got, want)
	}
	if !mapped.Envelope.Reaction.Added {
		t.Fatal("Reaction.Added = false, want true")
	}
	if got, want := mapped.Envelope.GroupID, "C555"; got != want {
		t.Fatalf("GroupID = %q, want %q", got, want)
	}

	if _, err := mapSlackReactionEvent(slackReactionEvent{
		Type:     "reaction_added",
		User:     "U1",
		Reaction: "eyes",
		Item: slackReactionItem{
			Type: "file",
			TS:   "1775866803.000000",
		},
	}, managed, now, "", ""); err == nil {
		t.Fatal("mapSlackReactionEvent(malformed) error = nil, want non-nil")
	}
}

func TestAllowSlackDirectMessagePolicies(t *testing.T) {
	t.Parallel()

	user := slackUserIdentity{ID: "U123", Username: "alice"}

	if !allowSlackDirectMessage(&resolvedInstanceConfig{dmPolicy: bridgepkg.BridgeDMPolicyOpen}, user, true) {
		t.Fatal("allowSlackDirectMessage(open) = false, want true")
	}
	if !allowSlackDirectMessage(&resolvedInstanceConfig{
		dmPolicy:       bridgepkg.BridgeDMPolicyAllowlist,
		allowUserIDs:   map[string]struct{}{"U123": {}},
		allowUsernames: map[string]struct{}{"bob": {}},
	}, user, true) {
		t.Fatal("allowSlackDirectMessage(allowlist by id) = false, want true")
	}
	if !allowSlackDirectMessage(&resolvedInstanceConfig{
		dmPolicy:        bridgepkg.BridgeDMPolicyPairing,
		pairedUsernames: map[string]struct{}{"alice": {}},
	}, user, true) {
		t.Fatal("allowSlackDirectMessage(pairing by username) = false, want true")
	}
	if allowSlackDirectMessage(&resolvedInstanceConfig{dmPolicy: bridgepkg.BridgeDMPolicyAllowlist}, user, true) {
		t.Fatal("allowSlackDirectMessage(rejected direct) = true, want false")
	}
	if !allowSlackDirectMessage(&resolvedInstanceConfig{dmPolicy: bridgepkg.BridgeDMPolicyAllowlist}, user, false) {
		t.Fatal("allowSlackDirectMessage(non-direct) = false, want true")
	}
}

func TestVerifySlackSignature(t *testing.T) {
	t.Parallel()

	body := []byte(`{"type":"event_callback"}`)
	now := time.Date(2026, 4, 15, 12, 4, 0, 0, time.UTC)
	timestamp := strconv.FormatInt(now.Unix(), 10)
	signature := slackSignature("top-secret", timestamp, body)

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.com/slack/brg-1",
		bytes.NewReader(body),
	)
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	req.Header.Set("X-Slack-Signature", signature)

	if err := verifySlackSignature(context.Background(), req, body, "top-secret", now); err != nil {
		t.Fatalf("verifySlackSignature(valid) error = %v", err)
	}
	if err := verifySlackSignature(context.Background(), req, body, "wrong", now); err == nil {
		t.Fatal("verifySlackSignature(invalid secret) error = nil, want non-nil")
	}
	if err := verifySlackSignature(context.Background(), req, body, "top-secret", now.Add(10*time.Minute)); err == nil {
		t.Fatal("verifySlackSignature(stale) error = nil, want non-nil")
	}
}

func TestVerifySlackSignatureValidationErrors(t *testing.T) {
	t.Parallel()

	body := []byte(`{}`)
	now := time.Date(2026, 4, 15, 12, 4, 30, 0, time.UTC)

	if err := verifySlackSignature(context.Background(), nil, body, "top-secret", now); err == nil {
		t.Fatal("verifySlackSignature(nil request) error = nil, want non-nil")
	}
	if err := verifySlackSignature(
		context.Background(),
		httptest.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"http://example.com",
			bytes.NewReader(body),
		),
		body,
		"",
		now,
	); err == nil {
		t.Fatal("verifySlackSignature(empty secret) error = nil, want non-nil")
	}

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.com",
		bytes.NewReader(body),
	)
	if err := verifySlackSignature(context.Background(), req, body, "top-secret", now); err == nil {
		t.Fatal("verifySlackSignature(missing headers) error = nil, want non-nil")
	}

	req.Header.Set("X-Slack-Request-Timestamp", "not-a-number")
	req.Header.Set("X-Slack-Signature", "v0=signature")
	if err := verifySlackSignature(context.Background(), req, body, "top-secret", now); err == nil {
		t.Fatal("verifySlackSignature(invalid timestamp) error = nil, want non-nil")
	}
}

func TestExecuteDeliveryPostEditDeleteAndResume(t *testing.T) {
	t.Parallel()

	api := &fakeSlackAPI{nextTS: "1775866805.100000"}

	startReq := testDeliveryRequest("brg-slack", "delivery-1", 1, bridgepkg.DeliveryEventTypeStart, false)
	startAck, state, err := executeDelivery(context.Background(), api, startReq, deliveryState{})
	if err != nil {
		t.Fatalf("executeDelivery(start) error = %v", err)
	}
	if got, want := startAck.RemoteMessageID, "C123:1775866805.100000"; got != want {
		t.Fatalf("startAck.RemoteMessageID = %q, want %q", got, want)
	}

	finalReq := testDeliveryRequest("brg-slack", "delivery-1", 2, bridgepkg.DeliveryEventTypeFinal, true)
	finalAck, state, err := executeDelivery(context.Background(), api, finalReq, state)
	if err != nil {
		t.Fatalf("executeDelivery(final) error = %v", err)
	}
	if got, want := finalAck.ReplaceRemoteMessageID, startAck.RemoteMessageID; got != want {
		t.Fatalf("finalAck.ReplaceRemoteMessageID = %q, want %q", got, want)
	}

	deleteReq := testDeleteRequest("brg-slack", "delivery-1", 3, finalAck.RemoteMessageID)
	deleteAck, _, err := executeDelivery(context.Background(), api, deleteReq, state)
	if err != nil {
		t.Fatalf("executeDelivery(delete) error = %v", err)
	}
	if got, want := deleteAck.RemoteMessageID, finalAck.RemoteMessageID; got != want {
		t.Fatalf("deleteAck.RemoteMessageID = %q, want %q", got, want)
	}
	if got, want := strings.Join(api.methods, ","), "chat.postMessage,chat.update,chat.delete"; got != want {
		t.Fatalf("api methods = %q, want %q", got, want)
	}

	resumeAPI := &fakeSlackAPI{nextTS: "1775866806.100000"}
	resumeReq := testDeliveryRequest("brg-slack", "delivery-2", 1, bridgepkg.DeliveryEventTypeResume, false)
	resumeReq.Event.Resume = &bridgepkg.DeliveryResumeState{LatestEventType: bridgepkg.DeliveryEventTypeFinal}
	resumeReq.Snapshot = &bridgepkg.DeliverySnapshot{
		DeliveryID:       "delivery-2",
		SessionID:        "sess-1",
		TurnID:           "turn-1",
		BridgeInstanceID: "brg-slack",
		RoutingKey:       resumeReq.Event.RoutingKey,
		DeliveryTarget:   resumeReq.Event.DeliveryTarget,
		LatestSeq:        1,
		LatestEventType:  bridgepkg.DeliveryEventTypeFinal,
		CurrentContent:   bridgepkg.MessageContent{Text: "hello"},
		Final:            true,
		UpdatedAt:        time.Date(2026, 4, 15, 12, 5, 0, 0, time.UTC),
	}
	resumeAck, _, err := executeDelivery(context.Background(), resumeAPI, resumeReq, deliveryState{})
	if err != nil {
		t.Fatalf("executeDelivery(resume without remote) error = %v", err)
	}
	if got, want := resumeAck.RemoteMessageID, "C123:1775866806.100000"; got != want {
		t.Fatalf("resumeAck.RemoteMessageID = %q, want %q", got, want)
	}

	t.Run("Should reject malformed successful post responses without advancing state", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name                string
			content             string
			postResults         []*slackPostedMessage
			wantCompletedChunks int
		}{
			{
				name:        "Should reject a nil post response",
				content:     "hello",
				postResults: []*slackPostedMessage{nil},
			},
			{
				name:        "Should reject a blank post timestamp",
				content:     "hello",
				postResults: []*slackPostedMessage{{TS: " "}},
			},
			{
				name:                "Should reject a blank continuation timestamp",
				content:             strings.Repeat("continuation ", slackMaxMessageLen),
				wantCompletedChunks: 1,
				postResults: []*slackPostedMessage{
					{TS: "1775866807.100000"},
					{TS: " "},
				},
			},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				api := &fakeSlackAPI{postResults: testCase.postResults}
				request := testDeliveryRequest(
					"brg-slack",
					"delivery-malformed-post",
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
				if got, want := len(api.postAttempts), testCase.wantCompletedChunks+1; got != want {
					t.Fatalf("post attempts = %d, want %d", got, want)
				}
			})
		}
	})

	t.Run("Should edit the explicitly referenced message without prior state", func(t *testing.T) {
		t.Parallel()

		api := &fakeSlackAPI{nextTS: "1775866808.100000"}
		request := testDeliveryRequest(
			"brg-slack",
			"delivery-reference-edit",
			1,
			bridgepkg.DeliveryEventTypeDelta,
			false,
		)
		request.Event.Operation = bridgepkg.DeliveryOperationEdit
		request.Event.Reference = &bridgepkg.DeliveryMessageReference{
			RemoteMessageID: "C123:1775866807.100000",
		}
		request.Event.Content.Text = "updated"

		ack, gotState, err := executeDelivery(context.Background(), api, request, deliveryState{})
		if err != nil {
			t.Fatalf("executeDelivery() error = %v", err)
		}
		if got, want := strings.Join(api.methods, ","), "chat.update"; got != want {
			t.Fatalf("api methods = %q, want %q", got, want)
		}
		if got, want := api.updates[0].TS, "1775866807.100000"; got != want {
			t.Fatalf("updated timestamp = %q, want %q", got, want)
		}
		if got, want := ack.RemoteMessageID, request.Event.Reference.RemoteMessageID; got != want {
			t.Fatalf("ack remote id = %q, want %q", got, want)
		}
		if got, want := gotState.RemoteMessageID, ack.RemoteMessageID; got != want {
			t.Fatalf("state remote id = %q, want %q", got, want)
		}
	})

	t.Run("Should honor Retry-After for update and delete operations", func(t *testing.T) {
		t.Parallel()

		rateLimit := &bridgesdk.RateLimitError{
			Err:        errors.New("rate limited primary operation"),
			RetryAfter: time.Nanosecond,
		}
		api := &fakeSlackAPI{
			updateErrors: []error{rateLimit, nil},
			deleteErrors: []error{rateLimit, nil},
		}
		state := deliveryState{
			LastSeq:         1,
			RemoteMessageID: "C123:1775866811.100000",
		}
		update := testDeliveryRequest(
			"brg-slack",
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
		if got, want := len(api.updates), 2; got != want {
			t.Fatalf("update attempts = %d, want %d", got, want)
		}

		deleteRequest := testDeleteRequest(
			"brg-slack",
			"delivery-primary-retry",
			3,
			ack.RemoteMessageID,
		)
		if _, _, err := executeDelivery(context.Background(), api, deleteRequest, state); err != nil {
			t.Fatalf("executeDelivery(delete) error = %v", err)
		}
		if got, want := len(api.deletes), 2; got != want {
			t.Fatalf("delete attempts = %d, want %d", got, want)
		}
	})
}

func TestSlackProgressDelivery(t *testing.T) {
	t.Parallel()

	t.Run("Should post a formatted progress bubble in the inbound thread before reacting", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
		api := &fakeSlackAPI{nextTS: "1775866900.100000"}
		provider := newSlackProgressTestProvider(t, func() time.Time { return now }, api)
		request := testProgressRequest(
			"brg-progress",
			"delivery-progress",
			1,
			bridgepkg.ToolProgressPhaseStarted,
			"**Reading**",
		)
		request.Event.Progress.Preview = "A & <B>"

		ack, err := provider.handleBridgesProgress(context.Background(), &bridgesdk.Session{}, request)
		if err != nil {
			t.Fatalf("handleBridgesProgress() error = %v", err)
		}
		if got := ack.RemoteMessageID; got != "" {
			t.Fatalf("ack.RemoteMessageID = %q, want empty progress ACK", got)
		}
		if got, want := len(api.posts), 1; got != want {
			t.Fatalf("post calls = %d, want %d", got, want)
		}
		if got, want := api.posts[0].ThreadTS, request.Event.DeliveryTarget.ThreadID; got != want {
			t.Fatalf("post thread_ts = %q, want %q", got, want)
		}
		if got, want := api.posts[0].Text, "\U0001f527 *Reading* A &amp; &lt;B&gt;"; got != want {
			t.Fatalf("post text = %q, want %q", got, want)
		}
		if got, want := len(api.statuses), 1; got != want {
			t.Fatalf("status calls = %d, want %d", got, want)
		}
		if got, want := api.statuses[0].Status, "is thinking..."; got != want {
			t.Fatalf("status = %q, want %q", got, want)
		}
		if got, want := len(api.reactions), 1; got != want {
			t.Fatalf("reaction calls = %d, want %d", got, want)
		}
		if got, want := api.reactions[0].Timestamp, "1775866900.100000"; got != want {
			t.Fatalf("reaction timestamp = %q, want progress bubble %q", got, want)
		}
		if got, want := api.reactions[0].Name, "eyes"; got != want {
			t.Fatalf("reaction name = %q, want %q", got, want)
		}
	})

	t.Run("Should keep progress bubbles within the Slack UTF-16 limit", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
		api := &fakeSlackAPI{nextTS: "1775866910.100000"}
		provider := newSlackProgressTestProvider(t, func() time.Time { return now }, api)
		request := testProgressRequest(
			"brg-progress",
			"delivery-progress-utf16",
			1,
			bridgepkg.ToolProgressPhaseStarted,
			"Inspecting",
		)
		request.Event.Progress.Preview = strings.Repeat("😀", slackMaxMessageLen/2+1)

		if _, err := provider.handleBridgesProgress(context.Background(), &bridgesdk.Session{}, request); err != nil {
			t.Fatalf("handleBridgesProgress() error = %v", err)
		}
		if got := len(api.posts); got < 2 {
			t.Fatalf("progress posts = %d, want multiple UTF-16-bounded chunks", got)
		}
		for index, post := range api.posts {
			if got := bridgesdk.UTF16Len(post.Text); got > slackMaxMessageLen {
				t.Fatalf("progress post %d wire length = %d, want <= %d", index, got, slackMaxMessageLen)
			}
		}
	})

	t.Run("Should deliver final text once after a committed progress edit result failure", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 13, 12, 5, 0, 0, time.UTC)
		api := &fakeSlackAPI{
			nextTS: "1775866915.100000",
			updateErrors: []error{
				bridgesdk.MarkCommittedMutation(errors.New("progress edit result unavailable")),
			},
		}
		provider := newSlackProgressTestProvider(t, func() time.Time { return now }, api)
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
			ProgressConfig: slackResolvedProgressConfig(cfg),
			Target:         target,
			MaxMessageLen:  slackMaxMessageLen,
			Len: func(value string) int {
				return bridgesdk.UTF16Len(formatOutbound(value))
			},
		}, &slackProgressSink{api: api}, func() time.Time { return now })
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
		if got, want := len(api.updates), 1; got != want {
			t.Fatalf("committed progress edit attempts = %d, want %d", got, want)
		}
		if got, want := countSlackPostAttempts(api.postAttempts, "final answer"), 1; got != want {
			t.Fatalf("final text post attempts = %d, want %d", got, want)
		}
	})

	t.Run("Should discard and close progress state after a committed final text result failure", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 13, 12, 6, 0, 0, time.UTC)
		api := &fakeSlackAPI{nextTS: "1775866916.100000"}
		provider := newSlackProgressTestProvider(t, func() time.Time { return now }, api)
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
		api.postErrorText = "final answer"
		api.postError = bridgesdk.MarkCommittedMutation(errors.New("final text result unavailable"))
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
		if got, want := countSlackPostAttempts(api.postAttempts, "final answer"), 1; got != want {
			t.Fatalf("final text post attempts = %d, want %d", got, want)
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
		api := &fakeSlackAPI{
			nextTS: "1775866905.100000",
			updateErrors: []error{
				&bridgesdk.RateLimitError{Err: errors.New("rate limited"), RetryAfter: time.Nanosecond},
				nil,
			},
		}
		provider := newSlackProgressTestProvider(t, func() time.Time { return now }, api)
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

		if got, want := len(api.updates), 2; got != want {
			t.Fatalf("update attempts = %d, want %d", got, want)
		}
		wantText := "\U0001f527 Inspecting\n\U0001f527 Reading"
		for index, update := range api.updates {
			if got := update.Text; got != wantText {
				t.Fatalf("update[%d].Text = %q, want %q", index, got, wantText)
			}
			if got, want := update.TS, "1775866905.100000"; got != want {
				t.Fatalf("update[%d].TS = %q, want %q", index, got, want)
			}
		}
	})

	t.Run("Should deliver completed duration and failed detail through the progress bubble", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 12, 10, 7, 0, 0, time.UTC)
		api := &fakeSlackAPI{nextTS: "1775866907.100000"}
		provider := newSlackProgressTestProvider(t, func() time.Time { return now }, api)
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

		if got, want := len(api.updates), 2; got != want {
			t.Fatalf("progress edits = %d, want %d", got, want)
		}
		body := api.updates[len(api.updates)-1].Text
		if !strings.Contains(body, "\u2705 Inspecting \u00b7 31s") ||
			!strings.Contains(body, "\u274c Writing \u2014 permission denied") {
			t.Fatalf("progress body = %q, want completed duration and failed detail", body)
		}
		if got, want := api.reactions[len(api.reactions)-1].Name, "x"; got != want {
			t.Fatalf("failed reaction = %q, want %q", got, want)
		}
	})

	t.Run("Should detach an existing dispatcher when progress mode changes to off", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 12, 10, 8, 0, 0, time.UTC)
		api := &fakeSlackAPI{nextTS: "1775866908.100000"}
		provider := newSlackProgressTestProvider(t, func() time.Time { return now }, api)
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
		api := &fakeSlackAPI{nextTS: "unused"}
		provider := newSlackProgressTestProvider(t, func() time.Time { return now }, api)
		cfg, _ := provider.routes.Get("brg-progress")
		cfg.managed.Instance.DeliveryDefaults = json.RawMessage(
			`{"progress":{"tool_progress":"off","grouping":"accumulate","typing":false,"reactions":false}}`,
		)
		provider.routes.Update("brg-progress", func(resolvedInstanceConfig) resolvedInstanceConfig { return cfg })
		factoryCalls := 0
		provider.apiFactory = func(*resolvedInstanceConfig) slackAPI {
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

func TestSlackDeleteDeliveryPayload(t *testing.T) {
	t.Parallel()

	t.Run("Should delete the exact referenced Slack message", func(t *testing.T) {
		t.Parallel()

		api := &fakeSlackAPI{}
		request := testDeleteRequest("brg-slack", "delivery-delete", 1, "C456:1775866999.100000")
		if _, _, err := executeDelivery(context.Background(), api, request, deliveryState{}); err != nil {
			t.Fatalf("executeDelivery(delete) error = %v", err)
		}
		if got, want := len(api.deletes), 1; got != want {
			t.Fatalf("delete calls = %d, want %d", got, want)
		}
		if got, want := api.deletes[0].Channel, "C456"; got != want {
			t.Fatalf("delete channel = %q, want %q", got, want)
		}
		if got, want := api.deletes[0].TS, "1775866999.100000"; got != want {
			t.Fatalf("delete ts = %q, want %q", got, want)
		}
	})
}

func TestExecuteDeliveryFormatsAndChunksCumulativeText(t *testing.T) {
	t.Parallel()

	t.Run("Should keep an oversized preview in one message and materialize continuations on final", func(t *testing.T) {
		t.Parallel()

		api := &fakeSlackAPI{nextTS: "1775866805.100000"}
		content := strings.Repeat("**bold!** & ", 4_000)
		startReq := testDeliveryRequest(
			"brg-slack",
			"delivery-long",
			1,
			bridgepkg.DeliveryEventTypeStart,
			false,
		)
		startReq.Event.Content.Text = content

		startAck, state, err := executeDelivery(context.Background(), api, startReq, deliveryState{})
		if err != nil {
			t.Fatalf("executeDelivery(start) error = %v", err)
		}
		if got, want := len(api.posts), 1; got != want {
			t.Fatalf("PostMessage calls after preview = %d, want %d", got, want)
		}
		if got := bridgesdk.UTF16Len(api.posts[0].Text); got > slackMaxMessageLen {
			t.Fatalf("preview wire length = %d, want <= %d", got, slackMaxMessageLen)
		}
		if strings.Contains(api.posts[0].Text, "**bold!**") || !strings.Contains(api.posts[0].Text, "&amp;") {
			t.Fatalf("preview text = %q, want Slack mrkdwn", api.posts[0].Text[:min(len(api.posts[0].Text), 80)])
		}
		if !api.posts[0].Mrkdwn {
			t.Fatal("preview Mrkdwn = false, want true")
		}

		deltaReq := testDeliveryRequest(
			"brg-slack",
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
		_, state, err = executeDelivery(context.Background(), api, deltaReq, state)
		if err != nil {
			t.Fatalf("executeDelivery(delta edit) error = %v", err)
		}
		if got, want := len(api.updates), 1; got != want {
			t.Fatalf("UpdateMessage calls after delta = %d, want %d", got, want)
		}
		if got, want := len(api.posts), 1; got != want {
			t.Fatalf("PostMessage calls after delta = %d, want preview only (%d)", got, want)
		}

		finalReq := testDeliveryRequest(
			"brg-slack",
			"delivery-long",
			3,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		finalReq.Event.Content.Text = content
		finalAck, finalState, err := executeDelivery(context.Background(), api, finalReq, state)
		if err != nil {
			t.Fatalf("executeDelivery(final) error = %v", err)
		}
		if got, want := len(api.updates), 2; got != want {
			t.Fatalf("UpdateMessage calls = %d, want %d", got, want)
		}
		if got := len(api.posts); got < 2 {
			t.Fatalf("PostMessage calls after final = %d, want preview plus continuation", got)
		}
		if got, want := finalAck.RemoteMessageID, finalState.RemoteMessageID; got != want {
			t.Fatalf("final ACK remote id = %q, want state handle %q", got, want)
		}
		if finalAck.RemoteMessageID == startAck.RemoteMessageID {
			t.Fatalf("final ACK remote id = %q, want last continuation handle", finalAck.RemoteMessageID)
		}
		if got, want := api.posts[len(api.posts)-1].ThreadTS, startReq.Event.DeliveryTarget.ThreadID; got != want {
			t.Fatalf("continuation thread_ts = %q, want %q", got, want)
		}

		wireBodies := []string{api.updates[len(api.updates)-1].Text}
		for _, request := range api.posts[1:] {
			wireBodies = append(wireBodies, request.Text)
		}
		for index, body := range wireBodies {
			if got := bridgesdk.UTF16Len(body); got > slackMaxMessageLen {
				t.Fatalf("final chunk %d wire length = %d, want <= %d", index, got, slackMaxMessageLen)
			}
		}
	})

	t.Run("Should keep astral characters within the Slack UTF-16 limit", func(t *testing.T) {
		t.Parallel()

		request := testDeliveryRequest(
			"brg-slack",
			"delivery-utf16",
			1,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		request.Event.Content.Text = strings.Repeat("😀", slackMaxMessageLen/2+1)

		chunks := slackDeliveryChunks(request.Event)
		if len(chunks) < 2 {
			t.Fatalf("slackDeliveryChunks() returned %d chunk, want multiple UTF-16-bounded chunks", len(chunks))
		}
		for index, chunk := range chunks {
			if got := bridgesdk.UTF16Len(chunk); got > slackMaxMessageLen {
				t.Fatalf("chunk %d UTF-16 length = %d, want <= %d", index, got, slackMaxMessageLen)
			}
		}
	})

	t.Run("Should resume at the first unsent chunk after exhausting Retry-After attempts", func(t *testing.T) {
		t.Parallel()

		request := testDeliveryRequest(
			"brg-slack",
			"delivery-partial-chunk",
			1,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		request.Event.Content.Text = strings.Repeat("partial Slack delivery ", 3_000)
		chunks := slackDeliveryChunks(request.Event)
		if len(chunks) < 2 {
			t.Fatalf("slackDeliveryChunks() returned %d chunk, want at least 2", len(chunks))
		}

		api := &fakeSlackAPI{
			nextTS:        "1775866810.100000",
			postErrorText: chunks[1],
			postError: &bridgesdk.RateLimitError{
				Err:        errors.New("rate limited continuation"),
				RetryAfter: time.Nanosecond,
			},
		}
		_, partialState, err := executeDelivery(context.Background(), api, request, deliveryState{})
		if err == nil {
			t.Fatal("executeDelivery(partial) error = nil, want exhausted rate limit")
		}
		if got, want := countSlackPostAttempts(
			api.postAttempts,
			chunks[1],
		), bridgesdk.DefaultRetryConfig().Attempts; got != want {
			t.Fatalf("failed continuation attempts = %d, want %d", got, want)
		}
		if got, want := countSlackPostAttempts(api.posts, chunks[0]), 1; got != want {
			t.Fatalf("successful first-chunk posts = %d, want %d", got, want)
		}

		api.postError = nil
		resume := testSlackResumeRequest(request)
		if _, _, err := executeDelivery(context.Background(), api, resume, partialState); err != nil {
			t.Fatalf("executeDelivery(resume) error = %v", err)
		}
		if got, want := countSlackPostAttempts(api.posts, chunks[0]), 1; got != want {
			t.Fatalf("successful first-chunk posts after resume = %d, want %d", got, want)
		}
		if got, want := len(api.posts), len(chunks); got != want {
			t.Fatalf("successful chunk posts = %d, want %d", got, want)
		}
	})
}

func TestRuntimeInitializeStartsWebhookServerAndWritesMarkers(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mockAPI := newSlackAPIServer(t)
	t.Setenv(slackListenAddrEnv, listenAddr)
	t.Setenv(slackAPIBaseEnv, mockAPI.URL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC)
	runtime.now = func() time.Time { return now }
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
	if got, want := handshake.Request.Runtime.Bridge.Provider, "slack"; got != want {
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

func TestHandleShutdownWritesMarker(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mockAPI := newSlackAPIServer(t)
	t.Setenv(slackListenAddrEnv, listenAddr)
	t.Setenv(slackAPIBaseEnv, mockAPI.URL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 13, 2, 0, 0, time.UTC)
	runtime.now = func() time.Time { return now }
	managed := testBridgeRuntime(now, "brg-1")

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

func TestHandleJSONWebhookChallengeAndReaction(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mockAPI := newSlackAPIServer(t)
	t.Setenv(slackListenAddrEnv, listenAddr)
	t.Setenv(slackAPIBaseEnv, mockAPI.URL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 13, 3, 0, 0, time.UTC)
	runtime.now = func() time.Time { return now }
	managed := testBridgeRuntime(now, "brg-1")

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
			return bridgepkg.BridgesMessagesIngestResult{SessionID: "sess-1"}, nil
		},
	)

	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}
	cfg, err := runtime.waitForInstanceConfig("brg-1", time.Second)
	if err != nil {
		t.Fatalf("waitForInstanceConfig() error = %v", err)
	}

	challengeRecorder := httptest.NewRecorder()
	if err := runtime.handleJSONWebhook(context.Background(), challengeRecorder, &cfg, bridgesdk.WebhookRequest{
		Body:       []byte(`{"type":"url_verification","challenge":"abc123"}`),
		ReceivedAt: now,
	}); err != nil {
		t.Fatalf("handleJSONWebhook(challenge) error = %v", err)
	}
	if got, want := challengeRecorder.Code, http.StatusOK; got != want {
		t.Fatalf("challenge status = %d, want %d", got, want)
	}
	if !strings.Contains(strings.TrimSpace(challengeRecorder.Body.String()), `"challenge":"abc123"`) {
		t.Fatalf("challenge body = %q, want challenge json", challengeRecorder.Body.String())
	}

	reactionRecorder := httptest.NewRecorder()
	if err := runtime.handleJSONWebhook(context.Background(), reactionRecorder, &cfg, bridgesdk.WebhookRequest{
		Body: []byte(
			`{"type":"event_callback","team_id":"T1","event_id":"EvReaction","event":{"type":"reaction_added","user":"U1","reaction":"eyes","event_ts":"1775866803.100000","item":{"type":"message","channel":"C123","ts":"1775866803.000000"}}}`,
		),
		ReceivedAt: now,
	}); err != nil {
		t.Fatalf("handleJSONWebhook(reaction) error = %v", err)
	}
	if got, want := reactionRecorder.Code, http.StatusOK; got != want {
		t.Fatalf("reaction status = %d, want %d", got, want)
	}
	ingests := waitForJSONLinesFile[bridgesdk.IngestMarker](
		t,
		env.IngestPath,
		func(items []bridgesdk.IngestMarker) bool { return len(items) >= 1 },
	)
	if got, want := ingests[len(ingests)-1].Envelope.EventFamily, bridgepkg.InboundEventFamilyReaction; got != want {
		t.Fatalf("reaction ingest family = %q, want %q", got, want)
	}
}

func TestWebhookIngressRejectsInvalidSignatureAndIngestsMessage(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mockAPI := newSlackAPIServer(t)
	t.Setenv(slackListenAddrEnv, listenAddr)
	t.Setenv(slackAPIBaseEnv, mockAPI.URL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 13, 5, 0, 0, time.UTC)
	runtime.now = func() time.Time { return now }
	managed := testBridgeRuntime(now, "brg-1")
	managed.BoundSecrets = []subprocess.InitializeBridgeBoundSecret{
		{BindingName: "bot_token", Kind: "token", Value: "xoxb-slack-token"},
		{BindingName: "signing_secret", Kind: "token", Value: "top-secret"},
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
	webhookURL := "http://" + serverAddr + "/slack/brg-1"
	body := []byte(slackMessageWebhookPayload())
	timestamp := strconv.FormatInt(now.Unix(), 10)

	invalidReq, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		webhookURL,
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("http.NewRequest(invalid) error = %v", err)
	}
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidReq.Header.Set("X-Slack-Request-Timestamp", timestamp)
	invalidReq.Header.Set("X-Slack-Signature", "v0=invalid")
	invalidResp, err := http.DefaultClient.Do(invalidReq)
	if err != nil {
		t.Fatalf("http.DefaultClient.Do(invalid) error = %v", err)
	}
	defer func() {
		if err := invalidResp.Body.Close(); err != nil {
			t.Errorf("invalid response body close error = %v", err)
		}
	}()
	if got, want := invalidResp.StatusCode, http.StatusUnauthorized; got != want {
		t.Fatalf("invalid webhook status = %d, want %d", got, want)
	}

	validReq, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		webhookURL,
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("http.NewRequest(valid) error = %v", err)
	}
	validReq.Header.Set("Content-Type", "application/json")
	validReq.Header.Set("X-Slack-Request-Timestamp", timestamp)
	validReq.Header.Set("X-Slack-Signature", slackSignature("top-secret", timestamp, body))
	validResp, err := http.DefaultClient.Do(validReq)
	if err != nil {
		t.Fatalf("http.DefaultClient.Do(valid) error = %v", err)
	}
	defer func() {
		if err := validResp.Body.Close(); err != nil {
			t.Errorf("valid response body close error = %v", err)
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
	if got, want := ingests[0].Envelope.GroupID, "C123"; got != want {
		t.Fatalf("ingest envelope group id = %q, want %q", got, want)
	}
	mu.Lock()
	if got, want := len(ingested), 1; got != want {
		t.Fatalf("len(ingested) = %d, want %d", got, want)
	}
	mu.Unlock()
}

func TestWebhookIngressHandlesSlashCommandAndBlockActions(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mockAPI := newSlackAPIServer(t)
	t.Setenv(slackListenAddrEnv, listenAddr)
	t.Setenv(slackAPIBaseEnv, mockAPI.URL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 13, 10, 0, 0, time.UTC)
	runtime.now = func() time.Time { return now }
	managed := testBridgeRuntime(now, "brg-1")
	managed.BoundSecrets = []subprocess.InitializeBridgeBoundSecret{
		{BindingName: "bot_token", Kind: "token", Value: "xoxb-slack-token"},
		{BindingName: "signing_secret", Kind: "token", Value: "top-secret"},
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
	webhookURL := "http://" + serverAddr + "/slack/brg-1"

	commandBody := []byte(
		"token=t&team_id=T1&channel_id=C123&channel_name=general&user_id=U123&user_name=alice&command=%2Fagh&text=hello&trigger_id=1337.42",
	)
	postSignedSlackForm(t, webhookURL, "top-secret", now, commandBody)

	actionBody := []byte("payload=" + url.QueryEscape(slackBlockActionsPayloadJSON()))
	postSignedSlackForm(t, webhookURL, "top-secret", now, actionBody)

	ingests := waitForJSONLinesFile[bridgesdk.IngestMarker](
		t,
		env.IngestPath,
		func(items []bridgesdk.IngestMarker) bool { return len(items) >= 2 },
	)
	if got, want := ingests[0].Envelope.EventFamily, bridgepkg.InboundEventFamilyCommand; got != want {
		t.Fatalf("ingests[0].Envelope.EventFamily = %q, want %q", got, want)
	}
	if got, want := ingests[1].Envelope.EventFamily, bridgepkg.InboundEventFamilyAction; got != want {
		t.Fatalf("ingests[1].Envelope.EventFamily = %q, want %q", got, want)
	}
}

func TestWebhookRetriesAfterTransientIngestFailure(t *testing.T) {
	testCases := []struct {
		name    string
		wantKey string
		invoke  func(context.Context, *slackProvider, resolvedInstanceConfig, time.Time) (*httptest.ResponseRecorder, error)
	}{
		{
			name:    "ShouldRetrySlashCommandAfterTransientIngestFailure",
			wantKey: "1337.42",
			invoke: func(
				ctx context.Context,
				runtime *slackProvider,
				cfg resolvedInstanceConfig,
				now time.Time,
			) (*httptest.ResponseRecorder, error) {
				recorder := httptest.NewRecorder()
				commandBody := []byte(
					"token=t&team_id=T1&channel_id=C123&channel_name=general&user_id=U123&user_name=alice&command=%2Fagh&text=hello&trigger_id=1337.42",
				)
				return recorder, runtime.handleFormWebhook(ctx, recorder, &cfg, bridgesdk.WebhookRequest{
					Body:       commandBody,
					ReceivedAt: now,
				})
			},
		},
		{
			name:    "ShouldRetryBlockActionsAfterTransientIngestFailure",
			wantKey: "1775866802.200000",
			invoke: func(
				ctx context.Context,
				runtime *slackProvider,
				cfg resolvedInstanceConfig,
				now time.Time,
			) (*httptest.ResponseRecorder, error) {
				recorder := httptest.NewRecorder()
				body := []byte("payload=" + url.QueryEscape(slackBlockActionsPayloadJSON()))
				return recorder, runtime.handleFormWebhook(ctx, recorder, &cfg, bridgesdk.WebhookRequest{
					Body:       body,
					ReceivedAt: now,
				})
			},
		},
		{
			name:    "ShouldRetryJSONMessageAfterTransientIngestFailure",
			wantKey: "EvMessage",
			invoke: func(
				ctx context.Context,
				runtime *slackProvider,
				cfg resolvedInstanceConfig,
				now time.Time,
			) (*httptest.ResponseRecorder, error) {
				recorder := httptest.NewRecorder()
				return recorder, runtime.handleJSONWebhook(ctx, recorder, &cfg, bridgesdk.WebhookRequest{
					Body:       []byte(slackMessageWebhookPayload()),
					ReceivedAt: now,
				})
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			env := setProviderTestEnv(t)

			runtime, hostPeer, cleanup := newRuntimePeerPair(t)
			defer cleanup()

			now := time.Date(2026, 4, 15, 13, 12, 0, 0, time.UTC)
			runtime.now = func() time.Time { return now }
			managed := testBridgeRuntime(now, "brg-1")

			var mu sync.Mutex
			attempts := make(map[string]int)

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
					attempts[envelope.IdempotencyKey]++
					attempt := attempts[envelope.IdempotencyKey]
					mu.Unlock()
					if attempt == 1 {
						return nil, errors.New("transient ingest failure")
					}
					return bridgepkg.BridgesMessagesIngestResult{SessionID: "sess-1"}, nil
				},
			)

			if err := hostPeer.Call(
				context.Background(),
				"initialize",
				testInitializeRequest(now, managed),
				nil,
			); err != nil {
				t.Fatalf("hostPeer.Call(initialize) error = %v", err)
			}
			cfg, err := runtime.waitForInstanceConfig("brg-1", time.Second)
			if err != nil {
				t.Fatalf("waitForInstanceConfig() error = %v", err)
			}

			_, firstErr := tt.invoke(context.Background(), runtime, cfg, now)
			var httpErr *bridgesdk.HTTPError
			if !errors.As(firstErr, &httpErr) {
				t.Fatalf("first invoke error type = %T, want *bridgesdk.HTTPError", firstErr)
			}
			if got, want := httpErr.StatusCode, http.StatusInternalServerError; got != want {
				t.Fatalf("first invoke status = %d, want %d", got, want)
			}

			recorder, err := tt.invoke(context.Background(), runtime, cfg, now)
			if err != nil {
				t.Fatalf("second invoke error = %v", err)
			}
			if got, want := recorder.Code, http.StatusOK; got != want {
				t.Fatalf("second invoke status = %d, want %d", got, want)
			}

			mu.Lock()
			attemptCount := attempts[tt.wantKey]
			mu.Unlock()
			if got, want := attemptCount, 2; got != want {
				t.Fatalf("attempts[%q] = %d, want %d", tt.wantKey, got, want)
			}
			if !cfg.dedup.Seen(tt.wantKey) {
				t.Fatalf("cfg.dedup.Seen(%q) = false, want true", tt.wantKey)
			}

			ingests := waitForJSONLinesFile[bridgesdk.IngestMarker](
				t,
				env.IngestPath,
				func(items []bridgesdk.IngestMarker) bool { return len(items) >= 2 },
			)
			if got, want := ingests[0].Envelope.IdempotencyKey, tt.wantKey; got != want {
				t.Fatalf("ingests[0].Envelope.IdempotencyKey = %q, want %q", got, want)
			}
			if got := strings.TrimSpace(ingests[0].Error); got == "" {
				t.Fatal("ingests[0].Error = empty, want transient failure")
			}
			if got, want := ingests[1].Envelope.IdempotencyKey, tt.wantKey; got != want {
				t.Fatalf("ingests[1].Envelope.IdempotencyKey = %q, want %q", got, want)
			}
			if got := strings.TrimSpace(ingests[1].Error); got != "" {
				t.Fatalf("ingests[1].Error = %q, want empty", got)
			}
			if got, want := ingests[1].Result.SessionID, "sess-1"; got != want {
				t.Fatalf("ingests[1].Result.SessionID = %q, want %q", got, want)
			}
		})
	}
}

func TestRuntimeDeliveriesCallSlackAPI(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mockAPI := newSlackAPIServer(t)
	t.Setenv(slackListenAddrEnv, listenAddr)
	t.Setenv(slackAPIBaseEnv, mockAPI.URL())

	_, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 13, 15, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-1")
	managed.BoundSecrets = []subprocess.InitializeBridgeBoundSecret{
		{BindingName: "bot_token", Kind: "token", Value: "xoxb-slack-token"},
		{BindingName: "signing_secret", Kind: "token", Value: "top-secret"},
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
	if err := hostPeer.Call(
		context.Background(),
		"bridges/deliver",
		testDeliveryRequest("brg-1", "delivery-1", 1, bridgepkg.DeliveryEventTypeStart, false),
		&ack,
	); err != nil {
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
		strings.Repeat("fmt.Println(\"hello\")\n", 2_100) + "```"
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
		t.Fatalf("len(mockAPI calls) = %d, want %d (auth + post + update + continuation)", got, want)
	}
	if got, want := calls[0].Method, "auth.test"; got != want {
		t.Fatalf("calls[0].Method = %q, want %q", got, want)
	}
	if got, want := calls[1].Method, "chat.postMessage"; got != want {
		t.Fatalf("calls[1].Method = %q, want %q", got, want)
	}
	if got, want := calls[2].Method, "chat.update"; got != want {
		t.Fatalf("calls[2].Method = %q, want %q", got, want)
	}
	if got, want := calls[3].Method, "chat.postMessage"; got != want {
		t.Fatalf("calls[3].Method = %q, want %q", got, want)
	}
	if got, want := calls[1].Body["mrkdwn"], true; got != want {
		t.Fatalf("post mrkdwn = %#v, want %#v", got, want)
	}
	if got, want := calls[1].Body["thread_ts"], "1775866805.000000"; got != want {
		t.Fatalf("post thread_ts = %#v, want %#v", got, want)
	}
	if got, want := calls[3].Body["thread_ts"], "1775866805.000000"; got != want {
		t.Fatalf("continuation thread_ts = %#v, want %#v", got, want)
	}
	if got, want := calls[3].Body["mrkdwn"], true; got != want {
		t.Fatalf("continuation mrkdwn = %#v, want %#v", got, want)
	}
	for index, call := range calls[2:] {
		text, ok := call.Body["text"].(string)
		if !ok {
			t.Fatalf("final call %d text = %#v, want string", index, call.Body["text"])
		}
		if got := len([]rune(text)); got > slackMaxMessageLen {
			t.Fatalf("final call %d text length = %d, want <= %d", index, got, slackMaxMessageLen)
		}
		if got := strings.Count(text, "```"); got%2 != 0 {
			t.Fatalf("final call %d fence count = %d, want balanced", index, got)
		}
	}
	firstFinalText, ok := calls[2].Body["text"].(string)
	if !ok || !strings.Contains(firstFinalText, "*Result:* <https://x.y|doc>") {
		t.Fatalf("first final text = %#v, want Slack mrkdwn conversion", calls[2].Body["text"])
	}
}

func TestHandleFormWebhookRejectsMissingPayload(t *testing.T) {
	t.Parallel()

	runtime, err := newSlackProvider(io.Discard)
	if err != nil {
		t.Fatalf("newSlackProvider() error = %v", err)
	}
	recorder := httptest.NewRecorder()
	err = runtime.handleFormWebhook(
		context.Background(),
		recorder,
		&resolvedInstanceConfig{},
		bridgesdk.WebhookRequest{
			Body:       []byte("payload="),
			ReceivedAt: time.Now().UTC(),
		},
	)
	var httpErr *bridgesdk.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("handleFormWebhook() error type = %T, want *bridgesdk.HTTPError", err)
	}
	if got, want := httpErr.StatusCode, http.StatusBadRequest; got != want {
		t.Fatalf("httpErr.StatusCode = %d, want %d", got, want)
	}
}

func TestHandleBridgesDeliverErrorPaths(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mockAPI := newSlackAPIServer(t)
	t.Setenv(slackListenAddrEnv, listenAddr)
	t.Setenv(slackAPIBaseEnv, mockAPI.URL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 13, 18, 0, 0, time.UTC)
	runtime.now = func() time.Time { return now }
	managed := testBridgeRuntime(now, "brg-1")

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

	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}
	session := runtime.currentSession()
	if session == nil {
		t.Fatal("runtime.currentSession() = nil, want session")
	}

	if _, err := runtime.handleBridgesDeliver(
		context.Background(),
		session,
		testDeliveryRequest("missing", "delivery-x", 1, bridgepkg.DeliveryEventTypeStart, false),
	); err == nil {
		t.Fatal("handleBridgesDeliver(missing instance) error = nil, want non-nil")
	}

	runtime.apiFactory = func(*resolvedInstanceConfig) slackAPI {
		return fakeSlackAPIError{err: &bridgesdk.AuthError{Err: errors.New("invalid_auth")}}
	}
	if _, err := runtime.handleBridgesDeliver(
		context.Background(),
		session,
		testDeliveryRequest("brg-1", "delivery-y", 1, bridgepkg.DeliveryEventTypeStart, false),
	); err == nil {
		t.Fatal("handleBridgesDeliver(auth failure) error = nil, want non-nil")
	}
	lines := waitForJSONLinesFile[bridgesdk.DeliveryMarker](
		t,
		env.DeliveryPath,
		func(items []bridgesdk.DeliveryMarker) bool { return len(items) >= 2 },
	)
	if lines[0].Error == "" || lines[1].Error == "" {
		t.Fatalf("delivery errors = %#v, want recorded marker failures", lines)
	}
}

func TestDispatchInboundBatchMergesContent(t *testing.T) {
	env := setProviderTestEnv(t)
	listenAddr := reserveListenAddr(t)
	mockAPI := newSlackAPIServer(t)
	t.Setenv(slackListenAddrEnv, listenAddr)
	t.Setenv(slackAPIBaseEnv, mockAPI.URL())

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 13, 20, 0, 0, time.UTC)
	runtime.now = func() time.Time { return now }
	managed := testBridgeRuntime(now, "brg-1")

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
			return bridgepkg.BridgesMessagesIngestResult{SessionID: "sess-1"}, nil
		},
	)

	managed.Instance.ProviderConfig = []byte(
		`{"webhook":{"listen_addr":"127.0.0.1:9999","path":"/slack/brg-1"},"batching":{"delay_ms":5,"split_delay_ms":5,"split_threshold":2}}`,
	)
	if err := hostPeer.Call(context.Background(), "initialize", testInitializeRequest(now, managed), nil); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	cfg, err := runtime.waitForInstanceConfig("brg-1", time.Second)
	if err != nil {
		t.Fatalf("waitForInstanceConfig() error = %v", err)
	}
	if cfg.batcher == nil {
		t.Fatal("cfg.batcher = nil, want batcher")
	}
	if err := runtime.dispatchInboundBatch(context.Background(), "brg-1", bridgesdk.InboundBatch{
		Items: []bridgepkg.InboundMessageEnvelope{
			{
				BridgeInstanceID:  "brg-1",
				Scope:             bridgepkg.ScopeWorkspace,
				WorkspaceID:       "ws-slack",
				GroupID:           "C123",
				ThreadID:          "thread-1",
				PlatformMessageID: "m1",
				ReceivedAt:        now,
				Sender:            bridgepkg.MessageSender{ID: "U1"},
				Content:           bridgepkg.MessageContent{Text: "hello"},
				EventFamily:       bridgepkg.InboundEventFamilyMessage,
				IdempotencyKey:    "k1",
			},
			{
				BridgeInstanceID:  "brg-1",
				Scope:             bridgepkg.ScopeWorkspace,
				WorkspaceID:       "ws-slack",
				GroupID:           "C123",
				ThreadID:          "thread-1",
				PlatformMessageID: "m2",
				ReceivedAt:        now,
				Sender:            bridgepkg.MessageSender{ID: "U1"},
				Content:           bridgepkg.MessageContent{Text: "world"},
				EventFamily:       bridgepkg.InboundEventFamilyMessage,
				IdempotencyKey:    "k2",
			},
		},
	}); err != nil {
		t.Fatalf("dispatchInboundBatch() error = %v", err)
	}
	waitForCondition(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ingested) == 1
	})
	mu.Lock()
	defer mu.Unlock()
	if got, want := ingested[0].Content.Text, "hello\nworld"; got != want {
		t.Fatalf("merged content = %q, want %q", got, want)
	}
	if got, want := ingested[0].IdempotencyKey, "k1:batch:2"; got != want {
		t.Fatalf("merged idempotency key = %q, want %q", got, want)
	}
	if got, want := len(
		waitForJSONLinesFile[bridgesdk.IngestMarker](
			t,
			env.IngestPath,
			func(items []bridgesdk.IngestMarker) bool { return len(items) == 1 },
		),
	), 1; got != want {
		t.Fatalf("ingest markers = %d, want %d", got, want)
	}
}

func TestStopClosesBatchersWithoutProviderLockDeadlock(t *testing.T) {
	runtime, err := newSlackProvider(io.Discard)
	if err != nil {
		t.Fatalf("newSlackProvider() error = %v", err)
	}

	dispatchStarted := make(chan struct{})
	allowLookup := make(chan struct{})
	dispatchDone := make(chan struct{})

	batcher, err := bridgesdk.NewInboundBatcher(bridgesdk.InboundBatcherConfig{
		Context: context.Background(),
		Delay:   5 * time.Millisecond,
		Dispatch: func(context.Context, bridgesdk.InboundBatch) error {
			close(dispatchStarted)
			<-allowLookup
			_, lookupErr := runtime.configForInstance("brg-1")
			close(dispatchDone)
			return lookupErr
		},
		Now: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatalf("NewInboundBatcher() error = %v", err)
	}

	runtime.routes.Replace(map[string]resolvedInstanceConfig{
		"brg-1": {
			instanceID:  "brg-1",
			webhookPath: "/slack/brg-1",
			batcher:     batcher,
		},
	}, nil)

	if err := batcher.Enqueue(bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID:  "brg-1",
		Scope:             bridgepkg.ScopeWorkspace,
		WorkspaceID:       "ws-slack",
		GroupID:           "C123",
		ThreadID:          "thread-1",
		PlatformMessageID: "m-1",
		ReceivedAt:        time.Now().UTC(),
		Sender:            bridgepkg.MessageSender{ID: "U1"},
		Content:           bridgepkg.MessageContent{Text: "hello"},
		EventFamily:       bridgepkg.InboundEventFamilyMessage,
		IdempotencyKey:    "slack-stop-deadlock",
	}); err != nil {
		t.Fatalf("batcher.Enqueue() error = %v", err)
	}

	select {
	case <-dispatchStarted:
	case <-time.After(time.Second):
		t.Fatal("dispatch did not start before timeout")
	}

	stopDone := make(chan struct{})
	go func() {
		runtime.lifecycle.Stop()
		close(stopDone)
	}()

	close(allowLookup)

	select {
	case <-dispatchDone:
	case <-time.After(time.Second):
		t.Fatal("dispatch remained blocked during stop")
	}

	select {
	case <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("stop() remained blocked while closing batchers")
	}
}

func TestResolveInstanceConfigAndHelperNormalization(t *testing.T) {
	env := setProviderTestEnv(t)
	_ = env
	t.Setenv(slackAPIBaseEnv, "https://slack-gov.example/api/")

	runtime, hostPeer, cleanup := newRuntimePeerPair(t)
	defer cleanup()

	now := time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-1")
	managed.Instance.DMPolicy = bridgepkg.BridgeDMPolicyPairing
	configured := managed
	configured.Instance.ProviderConfig = []byte(`{
		"webhook":{"listen_addr":"127.0.0.1:9999","path":"slack"},
		"batching":{"delay_ms":5,"split_delay_ms":7,"split_threshold":2},
		"dm":{"allow_user_ids":[" u123 "],"allow_usernames":["@Alice"],"paired_usernames":["Bob"]}
	}`)
	configured.BoundSecrets = []subprocess.InitializeBridgeBoundSecret{
		{BindingName: "bot_token", Kind: "token", Value: "xoxb-slack-token"},
		{BindingName: "signing_secret", Kind: "token", Value: "top-secret"},
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

	cfg := runtime.resolveInstanceConfig(session, configured)
	if cfg.configError != nil {
		t.Fatalf("resolveInstanceConfig() configError = %v, want nil", cfg.configError)
	}
	defer cfg.batcher.Close()

	if got, want := cfg.apiBaseURL, "https://slack-gov.example/api"; got != want {
		t.Fatalf("cfg.apiBaseURL = %q, want %q", got, want)
	}
	if got, want := cfg.listenAddr, "127.0.0.1:9999"; got != want {
		t.Fatalf("cfg.listenAddr = %q, want %q", got, want)
	}
	if got, want := cfg.webhookPath, "/slack"; got != want {
		t.Fatalf("cfg.webhookPath = %q, want %q", got, want)
	}
	if got, want := cfg.botToken, "xoxb-slack-token"; got != want {
		t.Fatalf("cfg.botToken = %q, want %q", got, want)
	}
	if got, want := cfg.signingSecret, "top-secret"; got != want {
		t.Fatalf("cfg.signingSecret = %q, want %q", got, want)
	}
	if cfg.batcher == nil {
		t.Fatal("cfg.batcher = nil, want batcher")
	}
	if _, ok := cfg.allowUserIDs["U123"]; !ok {
		t.Fatalf("cfg.allowUserIDs = %#v, want normalized user id", cfg.allowUserIDs)
	}
	if _, ok := cfg.allowUsernames["alice"]; !ok {
		t.Fatalf("cfg.allowUsernames = %#v, want normalized username", cfg.allowUsernames)
	}
	if _, ok := cfg.pairedUsernames["bob"]; !ok {
		t.Fatalf("cfg.pairedUsernames = %#v, want normalized username", cfg.pairedUsernames)
	}
	if got, want := normalizeWebhookPath("slack"), "/slack"; got != want {
		t.Fatalf("normalizeWebhookPath() = %q, want %q", got, want)
	}
	if got, want := normalizeURL("https://example.com/api/"), "https://example.com/api"; got != want {
		t.Fatalf("normalizeURL() = %q, want %q", got, want)
	}

	bad := configured
	bad.Instance.ProviderConfig = []byte("{")
	if cfg := runtime.resolveInstanceConfig(session, bad); cfg.configError == nil {
		t.Fatal("resolveInstanceConfig(bad json) configError = nil, want non-nil")
	}
}

func TestSlackWebhookPathConflictsDegradeAllRoutes(t *testing.T) {
	t.Parallel()

	configs := make([]resolvedInstanceConfig, 0, 2)
	usedPaths := make(map[string]int)

	first := resolvedInstanceConfig{instanceID: "brg-1", webhookPath: "/slack"}
	applySlackWebhookPathConflict(&first, usedPaths, configs)
	configs = append(configs, first)

	second := resolvedInstanceConfig{instanceID: "brg-2", webhookPath: "/slack"}
	applySlackWebhookPathConflict(&second, usedPaths, configs)
	configs = append(configs, second)

	if configs[0].configError == nil {
		t.Fatal("first duplicate configError = nil, want conflict degradation")
	}
	if configs[1].configError == nil {
		t.Fatal("second duplicate configError = nil, want conflict degradation")
	}

	runtime, err := newSlackProvider(io.Discard)
	if err != nil {
		t.Fatalf("newSlackProvider() error = %v", err)
	}
	runtime.routes.Replace(map[string]resolvedInstanceConfig{
		"brg-1": configs[0],
		"brg-2": configs[1],
	}, nil)

	if _, ok := runtime.configForPath("/slack"); ok {
		t.Fatal("configForPath(/slack) returned conflicted config, want deterministic rejection")
	}
}

func TestDetermineInitialStateRetryAndHealthHelpers(t *testing.T) {
	t.Parallel()

	runtime, err := newSlackProvider(io.Discard)
	if err != nil {
		t.Fatalf("newSlackProvider() error = %v", err)
	}

	badConfig := errors.New("bad config")
	status, degradation, err := runtime.determineInitialState(context.Background(), &resolvedInstanceConfig{
		instanceID:  "cfg-err",
		configError: badConfig,
	})
	if !errors.Is(err, badConfig) {
		t.Fatalf("determineInitialState(configError) error = %v, want %v", err, badConfig)
	}
	if got, want := status, bridgepkg.BridgeStatusDegraded; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if got, want := degradation.Reason, bridgepkg.BridgeDegradationReasonTenantConfigInvalid; got != want {
		t.Fatalf("degradation reason = %q, want %q", got, want)
	}

	runtime.apiFactory = func(cfg *resolvedInstanceConfig) slackAPI {
		switch cfg.instanceID {
		case "auth":
			return fakeSlackAPIError{err: &bridgesdk.AuthError{Err: errors.New("invalid_auth")}}
		case "transient":
			return fakeSlackAPIError{err: &bridgesdk.TransientError{Err: errors.New("unavailable")}}
		default:
			return &fakeSlackAPI{}
		}
	}

	status, degradation, err = runtime.determineInitialState(context.Background(), &resolvedInstanceConfig{
		instanceID:    "auth",
		botToken:      "xoxb",
		signingSecret: "secret",
	})
	if err == nil {
		t.Fatal("determineInitialState(auth) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusAuthRequired; got != want {
		t.Fatalf("auth status = %q, want %q", got, want)
	}
	if got, want := degradation.Reason, bridgepkg.BridgeDegradationReasonAuthFailed; got != want {
		t.Fatalf("auth degradation = %q, want %q", got, want)
	}

	status, degradation, err = runtime.determineInitialState(context.Background(), &resolvedInstanceConfig{
		instanceID:    "transient",
		botToken:      "xoxb",
		signingSecret: "secret",
	})
	if err == nil {
		t.Fatal("determineInitialState(transient) error = nil, want non-nil")
	}
	if got, want := status, bridgepkg.BridgeStatusDegraded; got != want {
		t.Fatalf("transient status = %q, want %q", got, want)
	}
	if got, want := degradation.Reason, bridgepkg.BridgeDegradationReasonProviderTimeout; got != want {
		t.Fatalf("transient degradation reason = %q, want %q", got, want)
	}

	runtime.setLastError(errors.New("boom"))
	if err := runtime.healthCheck(); err == nil {
		t.Fatal("healthCheck() error = nil, want non-nil")
	}
	runtime.clearLastError()
	if err := runtime.healthCheck(); err != nil {
		t.Fatalf("healthCheck(clear) error = %v", err)
	}
}

func TestClassifySlackAPIErrorAndDeleteMessage(t *testing.T) {
	t.Parallel()

	rateErr := classifySlackAPIError(http.StatusTooManyRequests, "ratelimited", 3*time.Second)
	var typedRateErr *bridgesdk.RateLimitError
	if !errors.As(rateErr, &typedRateErr) {
		t.Fatalf("rateErr type = %T, want *bridgesdk.RateLimitError", rateErr)
	}

	authErr := classifySlackAPIError(http.StatusUnauthorized, "invalid_auth", 0)
	var typedAuthErr *bridgesdk.AuthError
	if !errors.As(authErr, &typedAuthErr) {
		t.Fatalf("authErr type = %T, want *bridgesdk.AuthError", authErr)
	}

	transientErr := classifySlackAPIError(http.StatusServiceUnavailable, "service_unavailable", 0)
	var typedServerErr *bridgesdk.HTTPError
	if !errors.As(transientErr, &typedServerErr) || typedServerErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("transientErr = %#v, want HTTP 503", transientErr)
	}

	permanentErr := classifySlackAPIError(0, "unknown_problem", 0)
	var typedPermanentErr *bridgesdk.PermanentError
	if !errors.As(permanentErr, &typedPermanentErr) {
		t.Fatalf("permanentErr type = %T, want *bridgesdk.PermanentError", permanentErr)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := strings.TrimPrefix(r.URL.Path, "/"), "chat.delete"; got != want {
			t.Fatalf("method path = %q, want %q", got, want)
		}
		writeSlackAPIResponse(t, w, map[string]any{})
	}))
	defer server.Close()

	client := &slackBotClient{
		baseURL:    server.URL,
		botToken:   "xoxb-token",
		httpClient: &http.Client{Timeout: time.Second},
	}
	if err := client.DeleteMessage(
		context.Background(),
		slackDeleteMessageRequest{Channel: "C123", TS: "1775866808.100000"},
	); err != nil {
		t.Fatalf("DeleteMessage() error = %v", err)
	}
	if got, want := parseRetryAfter("3"), 3*time.Second; got != want {
		t.Fatalf("parseRetryAfter() = %v, want %v", got, want)
	}
	if got, want := maxInt(0, 500), 500; got != want {
		t.Fatalf("maxInt() = %d, want %d", got, want)
	}
}

func TestSlackBotClientCallBranches(t *testing.T) {
	t.Parallel()

	if err := ((*slackBotClient)(nil)).call(
		context.Background(),
		"chat.postMessage",
		map[string]any{},
		nil,
	); err == nil {
		t.Fatal("nil client call error = nil, want non-nil")
	}

	client := &slackBotClient{baseURL: "http://example.com", botToken: "xoxb"}
	if err := client.call(context.Background(), "chat.postMessage", func() {}, nil); err == nil {
		t.Fatal("marshal failure error = nil, want non-nil")
	}

	badURLClient := &slackBotClient{baseURL: "://bad-url", botToken: "xoxb"}
	if err := badURLClient.call(context.Background(), "chat.postMessage", map[string]any{}, nil); err == nil {
		t.Fatal("bad URL error = nil, want non-nil")
	}

	t.Run("Should rate limited", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
			writeSlackAPIResponse(t, w, map[string]any{"ok": false, "error": "ratelimited"})
		}))
		defer server.Close()

		client := &slackBotClient{baseURL: server.URL, botToken: "xoxb", httpClient: &http.Client{Timeout: time.Second}}
		err := client.call(context.Background(), "chat.postMessage", map[string]any{"channel": "C1"}, nil)
		var rateErr *bridgesdk.RateLimitError
		if !errors.As(err, &rateErr) {
			t.Fatalf("rate limited error type = %T, want *bridgesdk.RateLimitError", err)
		}
		if got, want := rateErr.RetryAfter, 7*time.Second; got != want {
			t.Fatalf("RetryAfter = %v, want %v", got, want)
		}
	})

	t.Run("Should retry one overloaded response through the shared overload profile", func(t *testing.T) {
		t.Parallel()

		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if attempts.Add(1) == 1 {
				w.WriteHeader(bridgesdk.HTTPStatusOverloaded)
				writeSlackAPIResponse(t, w, map[string]any{"ok": false, "error": "provider_overloaded"})
				return
			}
			writeSlackAPIResponse(t, w, map[string]any{"channel": "C1", "ts": "1775866808.200000"})
		}))
		defer server.Close()

		client := &slackBotClient{
			baseURL:    server.URL,
			botToken:   "xoxb",
			httpClient: &http.Client{Timeout: time.Second},
		}
		var (
			delays   []time.Duration
			observed []bridgesdk.RetryAttempt
		)
		result, err := bridgesdk.RetryDo(t.Context(), bridgesdk.RetryConfig{
			Attempts: 2,
			StandardBackoff: bridgesdk.RetryBackoff{
				BaseDelay: 10 * time.Millisecond,
				MaxDelay:  time.Second,
			},
			OverloadedBackoff: bridgesdk.RetryBackoff{
				BaseDelay: 40 * time.Millisecond,
				MaxDelay:  time.Second,
			},
			RandFloat: func() float64 { return 0 },
			Sleep: func(_ context.Context, delay time.Duration) error {
				delays = append(delays, delay)
				return nil
			},
			OnRetry: func(_ context.Context, attempt bridgesdk.RetryAttempt) error {
				observed = append(observed, attempt)
				return nil
			},
		}, func(ctx context.Context) (*slackPostedMessage, error) {
			return client.PostMessage(ctx, slackPostMessageRequest{Channel: "C1", Text: "hello"})
		})
		if err != nil {
			t.Fatalf("RetryDo(overloaded) error = %v", err)
		}
		if result == nil || result.TS != "1775866808.200000" {
			t.Fatalf("RetryDo(overloaded) result = %#v, want successful Slack message", result)
		}
		if got, want := attempts.Load(), int32(2); got != want {
			t.Fatalf("HTTP attempts = %d, want %d", got, want)
		}
		if len(delays) != 1 || delays[0] != 40*time.Millisecond {
			t.Fatalf("retry delays = %v, want [40ms]", delays)
		}
		if len(observed) != 1 || observed[0].Classified.Class != bridgesdk.ErrorClassOverloaded {
			t.Fatalf("retry observations = %#v, want one overloaded classification", observed)
		}
	})

	t.Run("Should preserve rate limit classification for a non-JSON error response", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
			if _, err := w.Write([]byte("<html>slow down</html>")); err != nil {
				t.Errorf("write malformed rate limit response: %v", err)
			}
		}))
		defer server.Close()

		client := &slackBotClient{
			baseURL:    server.URL,
			botToken:   "xoxb",
			httpClient: &http.Client{Timeout: time.Second},
		}
		err := client.call(t.Context(), "chat.postMessage", map[string]any{"channel": "C1"}, nil)
		var rateErr *bridgesdk.RateLimitError
		if !errors.As(err, &rateErr) {
			t.Fatalf("malformed 429 error type = %T %v, want RateLimitError", err, err)
		}
		if got, want := rateErr.RetryAfter, 7*time.Second; got != want {
			t.Fatalf("RetryAfter = %v, want %v", got, want)
		}
		var syntaxErr *json.SyntaxError
		if !errors.As(err, &syntaxErr) {
			t.Fatalf("malformed 429 error = %T %v, want preserved JSON syntax cause", err, err)
		}
	})

	t.Run("Should bound retries for a server response with a partial body", func(t *testing.T) {
		t.Parallel()

		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attempts.Add(1)
			partialBody := []byte(`{"ok":false,"error":"internal_`)
			w.Header().Set("Content-Length", strconv.Itoa(len(partialBody)+1))
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write(partialBody); err != nil {
				t.Errorf("write partial server response: %v", err)
			}
		}))
		defer server.Close()

		client := &slackBotClient{
			baseURL:    server.URL,
			botToken:   "xoxb",
			httpClient: &http.Client{Timeout: time.Second},
		}
		retryConfig := bridgesdk.DefaultRetryConfig()
		retryConfig.StandardBackoff = bridgesdk.RetryBackoff{
			BaseDelay: time.Nanosecond,
			MaxDelay:  time.Nanosecond,
		}
		retryConfig.RandFloat = func() float64 { return 0.5 }
		_, err := bridgesdk.RetryDo(t.Context(), retryConfig, func(ctx context.Context) (*slackPostedMessage, error) {
			return client.PostMessage(ctx, slackPostMessageRequest{Channel: "C1", Text: "hello"})
		})
		var httpErr *bridgesdk.HTTPError
		if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusInternalServerError {
			t.Fatalf("partial 500 error = %T %v, want HTTPError status 500", err, err)
		}
		if got, want := bridgesdk.ClassifyError(err).Class, bridgesdk.ErrorClassServerError; got != want {
			t.Fatalf("partial 500 class = %q, want %q", got, want)
		}
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("partial 500 error = %T %v, want preserved unexpected EOF", err, err)
		}
		if got, want := attempts.Load(), int32(retryConfig.Attempts); got != want {
			t.Fatalf("server-error attempts = %d, want %d", got, want)
		}
	})

	t.Run("Should decode response failure", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(`not-json`)); err != nil {
				t.Errorf("write malformed response: %v", err)
			}
		}))
		defer server.Close()

		client := &slackBotClient{baseURL: server.URL, botToken: "xoxb", httpClient: &http.Client{Timeout: time.Second}}
		err := client.call(context.Background(), "auth.test", map[string]any{}, nil)
		if err == nil {
			t.Fatal("decode response error = nil, want non-nil")
		}
		if committedErr, ok := errors.AsType[*bridgesdk.CommittedMutationError](err); ok && committedErr != nil {
			t.Fatalf("auth read error = %T %v, want non-committed error", err, err)
		}
	})

	t.Run("Should not retry a post after a successful status with a truncated result", func(t *testing.T) {
		t.Parallel()

		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attempts.Add(1)
			if _, err := w.Write([]byte(`{"ok":true,"ts":`)); err != nil {
				t.Errorf("write truncated response: %v", err)
			}
		}))
		defer server.Close()

		client := &slackBotClient{
			baseURL:    server.URL,
			botToken:   "xoxb",
			httpClient: &http.Client{Timeout: time.Second},
		}
		_, err := postSlackDeliveryMessage(t.Context(), client, slackPostMessageRequest{Channel: "C1", Text: "hello"})
		var committedErr *bridgesdk.CommittedMutationError
		if !errors.As(err, &committedErr) {
			t.Fatalf("postSlackDeliveryMessage() error = %T %v, want CommittedMutationError", err, err)
		}
		if got, want := attempts.Load(), int32(1); got != want {
			t.Fatalf("post attempts = %d, want %d", got, want)
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
				writeSlackAPIResponse(t, w, map[string]any{"ok": true, "ts": "123.456"})
			}))
			defer target.Close()

			redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", target.URL)
				w.WriteHeader(testCase.statusCode)
				writeSlackAPIResponse(t, w, map[string]any{"ok": false, "error": "redirect_refused"})
				if got, want := r.Method, http.MethodPost; got != want {
					t.Errorf("redirect source method = %q, want %q", got, want)
				}
			}))
			defer redirect.Close()

			client := &slackBotClient{
				baseURL:    redirect.URL,
				botToken:   "xoxb",
				httpClient: redirect.Client(),
			}
			var result slackPostedMessage
			err := client.callMutation(t.Context(), "chat.postMessage", map[string]any{"channel": "C1"}, &result)
			var permanentErr *bridgesdk.PermanentError
			if !errors.As(err, &permanentErr) {
				t.Fatalf(
					"callMutation(%d) error = %T %v, want first-response PermanentError",
					testCase.statusCode,
					err,
					err,
				)
			}
			if bridgesdk.IsCommittedMutation(err) {
				t.Fatalf("callMutation(%d) error = %v, want pre-commit rejection", testCase.statusCode, err)
			}
			if got := targetHits.Load(); got != 0 {
				t.Fatalf("redirect target hits = %d, want 0", got)
			}
		})
	}

	t.Run("Should api error classification", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeSlackAPIResponse(t, w, map[string]any{"ok": false, "error": "missing_scope"})
		}))
		defer server.Close()

		client := &slackBotClient{baseURL: server.URL, botToken: "xoxb", httpClient: &http.Client{Timeout: time.Second}}
		err := client.call(context.Background(), "chat.postMessage", map[string]any{"channel": "C1"}, nil)
		var authErr *bridgesdk.AuthError
		if !errors.As(err, &authErr) {
			t.Fatalf("api error type = %T, want *bridgesdk.AuthError", err)
		}
	})

	t.Run("Should decode result failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeSlackAPIResponse(t, w, map[string]any{"ok": true, "ts": "1775866808.100000"})
		}))
		defer server.Close()

		client := &slackBotClient{baseURL: server.URL, botToken: "xoxb", httpClient: &http.Client{Timeout: time.Second}}
		if err := client.call(
			context.Background(),
			"chat.postMessage",
			map[string]any{"channel": "C1"},
			make(chan int),
		); err == nil {
			t.Fatal("decode result error = nil, want non-nil")
		}
	})

	t.Run("Should wrapper success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch strings.TrimPrefix(r.URL.Path, "/") {
			case "auth.test":
				writeSlackAPIResponse(t, w, map[string]any{"ok": true, "bot_id": "B1", "user_id": "U1"})
			case "chat.postMessage":
				writeSlackAPIResponse(t, w, map[string]any{"ok": true, "ts": "1775866808.200000"})
			default:
				t.Fatalf("unexpected method path %q", r.URL.Path)
			}
		}))
		defer server.Close()

		client := &slackBotClient{baseURL: server.URL, botToken: "xoxb", httpClient: &http.Client{Timeout: time.Second}}
		auth, err := client.AuthTest(context.Background())
		if err != nil {
			t.Fatalf("AuthTest() error = %v", err)
		}
		if got, want := auth.BotID, "B1"; got != want {
			t.Fatalf("auth.BotID = %q, want %q", got, want)
		}
		posted, err := client.PostMessage(context.Background(), slackPostMessageRequest{Channel: "C1", Text: "hello"})
		if err != nil {
			t.Fatalf("PostMessage() error = %v", err)
		}
		if got, want := posted.TS, "1775866808.200000"; got != want {
			t.Fatalf("posted.TS = %q, want %q", got, want)
		}
	})

	t.Run("Should paginate delivery history until a matching message is found", func(t *testing.T) {
		t.Parallel()

		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got, want := strings.TrimPrefix(r.URL.Path, "/"), "conversations.replies"; got != want {
				t.Fatalf("method path = %q, want %q", got, want)
			}
			var req slackConversationMessagesRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("Decode(request) error = %v", err)
			}
			requestCount++
			switch requestCount {
			case 1:
				if req.Cursor != "" || req.TS != "1775866808.100000" || !req.Inclusive {
					t.Fatalf("first request = %#v, want initial replies page", req)
				}
				writeSlackAPIResponse(t, w, map[string]any{
					"ok":       true,
					"has_more": true,
					"messages": []map[string]any{{
						"ts": "1775866808.100000",
						"metadata": map[string]any{
							"event_type": "agh_bridge_delivery",
							"event_payload": map[string]any{
								"bridge_instance_id": "brg-other",
								"delivery_id":        "delivery-1",
							},
						},
					}},
					"response_metadata": map[string]any{"next_cursor": "cursor-2"},
				})
			case 2:
				if req.Cursor != "cursor-2" || req.TS != "1775866808.100000" || !req.Inclusive {
					t.Fatalf("second request = %#v, want paginated replies page", req)
				}
				writeSlackAPIResponse(t, w, map[string]any{
					"ok":       true,
					"has_more": false,
					"messages": []map[string]any{{
						"ts": "1775866808.200000",
						"metadata": map[string]any{
							"event_type": "agh_bridge_delivery",
							"event_payload": map[string]any{
								"bridge_instance_id": "brg-slack",
								"delivery_id":        "delivery-1",
							},
						},
					}},
				})
			default:
				t.Fatalf("unexpected request count %d", requestCount)
			}
		}))
		defer server.Close()

		client := &slackBotClient{baseURL: server.URL, botToken: "xoxb", httpClient: &http.Client{Timeout: time.Second}}
		message, err := client.FindDeliveryMessage(context.Background(), slackFindDeliveryMessageRequest{
			Channel:          "C123",
			ThreadTS:         "1775866808.100000",
			DeliveryID:       "delivery-1",
			BridgeInstanceID: "brg-slack",
		})
		if err != nil {
			t.Fatalf("FindDeliveryMessage() error = %v", err)
		}
		if message == nil || message.TS != "1775866808.200000" {
			t.Fatalf("FindDeliveryMessage() = %#v, want paginated match", message)
		}
		if got, want := requestCount, 2; got != want {
			t.Fatalf("requestCount = %d, want %d", got, want)
		}
	})
}

func TestProviderHelperEdges(t *testing.T) {
	t.Parallel()

	if !isIgnoredSlackMessageEvent(slackMessageEvent{Type: "message"}) {
		t.Fatal("isIgnoredSlackMessageEvent(no user) = false, want true")
	}
	if !isIgnoredSlackMessageEvent(slackMessageEvent{Type: "message", User: "U1", BotID: "B1"}) {
		t.Fatal("isIgnoredSlackMessageEvent(bot) = false, want true")
	}
	if !isIgnoredSlackMessageEvent(slackMessageEvent{Type: "message", User: "U1", Subtype: "channel_join"}) {
		t.Fatal("isIgnoredSlackMessageEvent(channel_join) = false, want true")
	}
	if isIgnoredSlackMessageEvent(slackMessageEvent{Type: "message", User: "U1", Text: "hello"}) {
		t.Fatal("isIgnoredSlackMessageEvent(normal message) = true, want false")
	}

	if _, err := parseSlackTimestamp(""); err == nil {
		t.Fatal("parseSlackTimestamp(empty) error = nil, want non-nil")
	}
	if _, err := parseSlackTimestamp("nope"); err == nil {
		t.Fatal("parseSlackTimestamp(invalid) error = nil, want non-nil")
	}
	if got, want := normalizeSlackEmoji(""), ""; got != want {
		t.Fatalf("normalizeSlackEmoji(empty) = %q, want %q", got, want)
	}
	if got, want := normalizeURL(""), ""; got != want {
		t.Fatalf("normalizeURL(empty) = %q, want %q", got, want)
	}
	if got, want := referenceRemoteMessageID(nil), ""; got != want {
		t.Fatalf("referenceRemoteMessageID(nil) = %q, want %q", got, want)
	}
}

func TestRunRejectsUnsupportedCommand(t *testing.T) {
	t.Parallel()

	if err := run([]string{"unknown"}, nil, io.Discard, io.Discard); err == nil {
		t.Fatal("run(unknown) error = nil, want non-nil")
	}
}

type fakeSlackAPI struct {
	methods       []string
	nextTS        string
	postResults   []*slackPostedMessage
	postErrorText string
	postError     error
	postAttempts  []slackPostMessageRequest
	updateErrors  []error
	deleteErrors  []error
	posts         []slackPostMessageRequest
	updates       []slackUpdateMessageRequest
	deletes       []slackDeleteMessageRequest
	statuses      []slackSetThreadStatusRequest
	reactions     []slackAddReactionRequest
}

func (f fakeSlackAPI) AuthTest(context.Context) (*slackAuthIdentity, error) {
	return &slackAuthIdentity{UserID: "U_BOT", BotID: "B_BOT"}, nil
}

func (f *fakeSlackAPI) PostMessage(
	_ context.Context,
	request slackPostMessageRequest,
) (*slackPostedMessage, error) {
	f.methods = append(f.methods, "chat.postMessage")
	f.postAttempts = append(f.postAttempts, request)
	if f.postError != nil && (f.postErrorText == "" || request.Text == f.postErrorText) {
		return nil, f.postError
	}
	f.posts = append(f.posts, request)
	callIndex := len(f.posts) - 1
	if callIndex < len(f.postResults) {
		return f.postResults[callIndex], nil
	}
	if strings.TrimSpace(f.nextTS) == "" {
		f.nextTS = "1775866805.100000"
	}
	timestamp := f.nextTS
	if len(f.posts) > 1 {
		timestamp += "-" + strconv.Itoa(len(f.posts))
	}
	return &slackPostedMessage{TS: timestamp}, nil
}

func countSlackPostAttempts(requests []slackPostMessageRequest, text string) int {
	count := 0
	for _, request := range requests {
		if request.Text == text {
			count++
		}
	}
	return count
}

func testSlackResumeRequest(request bridgepkg.DeliveryRequest) bridgepkg.DeliveryRequest {
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

func (f *fakeSlackAPI) UpdateMessage(_ context.Context, request slackUpdateMessageRequest) error {
	f.methods = append(f.methods, "chat.update")
	f.updates = append(f.updates, request)
	callIndex := len(f.updates) - 1
	if callIndex < len(f.updateErrors) {
		return f.updateErrors[callIndex]
	}
	return nil
}

func (f *fakeSlackAPI) DeleteMessage(_ context.Context, request slackDeleteMessageRequest) error {
	f.methods = append(f.methods, "chat.delete")
	f.deletes = append(f.deletes, request)
	callIndex := len(f.deletes) - 1
	if callIndex < len(f.deleteErrors) {
		return f.deleteErrors[callIndex]
	}
	return nil
}

func (f *fakeSlackAPI) SetThreadStatus(_ context.Context, request slackSetThreadStatusRequest) error {
	f.methods = append(f.methods, "assistant.threads.setStatus")
	f.statuses = append(f.statuses, request)
	return nil
}

func (f *fakeSlackAPI) AddReaction(_ context.Context, request slackAddReactionRequest) error {
	f.methods = append(f.methods, "reactions.add")
	f.reactions = append(f.reactions, request)
	return nil
}

type fakeSlackAPIError struct {
	err error
}

func (f fakeSlackAPIError) AuthTest(context.Context) (*slackAuthIdentity, error) {
	return nil, f.err
}

func (f fakeSlackAPIError) PostMessage(context.Context, slackPostMessageRequest) (*slackPostedMessage, error) {
	return nil, f.err
}

func (f fakeSlackAPIError) UpdateMessage(context.Context, slackUpdateMessageRequest) error {
	return f.err
}

func (f fakeSlackAPIError) DeleteMessage(context.Context, slackDeleteMessageRequest) error {
	return f.err
}

func (f fakeSlackAPIError) SetThreadStatus(context.Context, slackSetThreadStatusRequest) error {
	return f.err
}

func (f fakeSlackAPIError) AddReaction(context.Context, slackAddReactionRequest) error {
	return f.err
}

type slackAPIServer struct {
	mu     sync.Mutex
	server *httptest.Server
	calls  []slackAPICall
	nextTS string
}

type slackAPICall struct {
	Method string
	Body   map[string]any
}

func newSlackAPIServer(t *testing.T) *slackAPIServer {
	t.Helper()

	srv := &slackAPIServer{nextTS: "1775866808.100000"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := strings.TrimPrefix(r.URL.Path, "/")
		body := make(map[string]any)
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			t.Fatalf("json.NewDecoder().Decode() error = %v", err)
		}
		srv.mu.Lock()
		srv.calls = append(srv.calls, slackAPICall{Method: method, Body: body})
		srv.mu.Unlock()

		switch method {
		case "auth.test":
			writeSlackAPIResponse(t, w, map[string]any{"user_id": "U_BOT", "bot_id": "B_BOT"})
		case "chat.postMessage":
			writeSlackAPIResponse(t, w, map[string]any{"ts": srv.nextTS})
		case "chat.update":
			writeSlackAPIResponse(t, w, map[string]any{"ts": body["ts"]})
		case "chat.delete":
			writeSlackAPIResponse(t, w, map[string]any{})
		case "assistant.threads.setStatus", "reactions.add":
			writeSlackAPIResponse(t, w, map[string]any{})
		default:
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "unknown_method"}); err != nil {
				t.Errorf("encode unknown Slack method response: %v", err)
			}
		}
	}))
	srv.server = server
	t.Cleanup(server.Close)
	return srv
}

func (s *slackAPIServer) URL() string {
	return s.server.URL
}

func (s *slackAPIServer) Calls() []slackAPICall {
	s.mu.Lock()
	defer s.mu.Unlock()
	cloned := make([]slackAPICall, len(s.calls))
	copy(cloned, s.calls)
	return cloned
}

func writeSlackAPIResponse(t *testing.T, w http.ResponseWriter, result any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	payload := map[string]any{"ok": true}
	switch typed := result.(type) {
	case map[string]any:
		maps.Copy(payload, typed)
	default:
		raw, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		payload["ok"] = true
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("json.NewEncoder().Encode() error = %v", err)
	}
}

func newRuntimePeerPair(t *testing.T) (*slackProvider, *bridgesdk.Peer, func()) {
	t.Helper()

	hostConn, runtimeConn := net.Pipe()
	runtime, err := newSlackProvider(io.Discard)
	if err != nil {
		t.Fatalf("newSlackProvider() error = %v", err)
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

func testBridgeRuntime(now time.Time, instanceID string) subprocess.InitializeBridgeManagedInstance {
	return subprocess.InitializeBridgeManagedInstance{
		Instance: bridgepkg.BridgeInstance{
			ID:            instanceID,
			Scope:         bridgepkg.ScopeWorkspace,
			WorkspaceID:   "ws-slack",
			Platform:      "slack",
			ExtensionName: "slack",
			DisplayName:   "Slack",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true, IncludeThread: true, IncludeGroup: true},
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		BoundSecrets: []subprocess.InitializeBridgeBoundSecret{
			{BindingName: "bot_token", Kind: "token", Value: "xoxb-slack-token"},
			{BindingName: "signing_secret", Kind: "token", Value: "top-secret"},
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
			Name:       "slack",
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
				Provider:         "slack",
				Platform:         "slack",
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
				WorkspaceID:      "ws-slack",
				BridgeInstanceID: instanceID,
				GroupID:          "C123",
				ThreadID:         "1775866805.000000",
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: instanceID,
				GroupID:          "C123",
				ThreadID:         "1775866805.000000",
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
		ToolCallID: "call-" + strconv.FormatInt(seq, 10),
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

func newSlackProgressTestProvider(
	t *testing.T,
	now func() time.Time,
	api slackAPI,
) *slackProvider {
	t.Helper()

	provider, err := newSlackProvider(io.Discard)
	if err != nil {
		t.Fatalf("newSlackProvider() error = %v", err)
	}
	managed := testBridgeRuntime(now(), "brg-progress")
	provider.now = now
	provider.routes.Replace(map[string]resolvedInstanceConfig{
		"brg-progress": {
			managed:    &managed,
			instanceID: "brg-progress",
		},
	}, nil)
	provider.apiFactory = func(*resolvedInstanceConfig) slackAPI { return api }
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

func slackMessageWebhookPayload() string {
	return `{"type":"event_callback","team_id":"T123","event_id":"EvMessage","event_time":1775866800,"event":{"type":"message","channel":"C123","channel_type":"channel","user":"U123","username":"alice","text":"hello","ts":"1775866800.100000"}}`
}

func slackBlockActionsPayloadJSON() string {
	return `{"type":"block_actions","trigger_id":"trigger-1","response_url":"https://hooks.slack.test/action","channel":{"id":"C123"},"container":{"type":"message","channel_id":"C123","message_ts":"1775866802.100000","thread_ts":"1775866802.000000"},"message":{"ts":"1775866802.100000","thread_ts":"1775866802.000000"},"user":{"id":"U123","username":"alice"},"actions":[{"type":"button","action_id":"approve","value":"yes","action_ts":"1775866802.200000"}]}`
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

func postSignedSlackForm(t *testing.T, webhookURL string, secret string, now time.Time, body []byte) {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	timestamp := strconv.FormatInt(now.Unix(), 10)
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	req.Header.Set("X-Slack-Signature", slackSignature(secret, timestamp, body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http.DefaultClient.Do() error = %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("form webhook response body close error = %v", err)
		}
	}()
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("form webhook status = %d, want %d", got, want)
	}
}

func slackSignature(secret string, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(slackSignatureVersion + ":" + strings.TrimSpace(timestamp) + ":")); err != nil {
		panic(fmt.Sprintf("write Slack signature prefix: %v", err))
	}
	if _, err := mac.Write(body); err != nil {
		panic(fmt.Sprintf("write Slack signature body: %v", err))
	}
	return slackSignatureVersion + "=" + hex.EncodeToString(mac.Sum(nil))
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

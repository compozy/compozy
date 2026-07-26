package main

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func TestSlackContractIngressRouting(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 13, 0, 0, 0, time.UTC)
	managed := testBridgeRuntime(now, "brg-slack")
	managed.Instance.RoutingPolicy = bridgepkg.RoutingPolicy{
		IncludePeer:   true,
		IncludeThread: true,
	}

	t.Run("Should map root direct messages with a thread dimension", func(t *testing.T) {
		t.Parallel()

		mapped, ignored, err := mapSlackMessageEvent(slackMessageEvent{
			Channel:     "D123",
			ChannelType: "im",
			Text:        "hello",
			TS:          "1775866805.100000",
			Type:        "message",
			User:        "U123",
		}, managed, now, "EvMessage", "T123", now.Unix(), nil)
		if err != nil {
			t.Fatalf("mapSlackMessageEvent() error = %v", err)
		}
		if ignored {
			t.Fatal("mapSlackMessageEvent() ignored = true, want false")
		}
		assertSlackRoutableDirectEnvelope(t, managed.Instance, mapped.Envelope, "1775866805.100000")
	})

	t.Run("Should map direct slash commands with a stable root thread dimension", func(t *testing.T) {
		t.Parallel()

		mapped, err := mapSlackSlashCommand(url.Values{
			"channel_id":   {"D123"},
			"channel_name": {"directmessage"},
			"command":      {"/agh"},
			"text":         {"status"},
			"user_id":      {"U123"},
		}, managed, now)
		if err != nil {
			t.Fatalf("mapSlackSlashCommand() error = %v", err)
		}
		assertSlackRoutableDirectEnvelope(t, managed.Instance, mapped.Envelope, "D123")
	})

	t.Run("Should map direct reactions with the reacted message as thread dimension", func(t *testing.T) {
		t.Parallel()

		mapped, err := mapSlackReactionEvent(slackReactionEvent{
			EventTS:  "1775866806.100000",
			Item:     slackReactionItem{Channel: "D123", TS: "1775866805.100000", Type: "message"},
			Reaction: "thumbsup",
			Type:     "reaction_added",
			User:     "U123",
		}, managed, now, "EvReaction", "T123")
		if err != nil {
			t.Fatalf("mapSlackReactionEvent() error = %v", err)
		}
		assertSlackRoutableDirectEnvelope(t, managed.Instance, mapped.Envelope, "1775866805.100000")
	})
}

func TestSlackContractDeliveryRecovery(t *testing.T) {
	t.Parallel()

	t.Run("Should reconcile a lost create ACK without posting a duplicate message", func(t *testing.T) {
		t.Parallel()

		api := &recordingSlackAPI{}
		startReq := testDeliveryRequest(
			"brg-slack",
			"delivery-lost-ack",
			1,
			bridgepkg.DeliveryEventTypeStart,
			false,
		)
		startAck, _, err := executeDelivery(context.Background(), api, startReq, deliveryState{})
		if err != nil {
			t.Fatalf("executeDelivery(start) error = %v", err)
		}

		resumeReq := testDeliveryRequest(
			"brg-slack",
			"delivery-lost-ack",
			1,
			bridgepkg.DeliveryEventTypeResume,
			false,
		)
		resumeReq.Event.Resume = &bridgepkg.DeliveryResumeState{LatestEventType: bridgepkg.DeliveryEventTypeStart}
		resumeReq.Snapshot = &bridgepkg.DeliverySnapshot{
			DeliveryID:       resumeReq.Event.DeliveryID,
			SessionID:        "sess-slack",
			TurnID:           "turn-slack",
			BridgeInstanceID: resumeReq.Event.BridgeInstanceID,
			RoutingKey:       resumeReq.Event.RoutingKey,
			DeliveryTarget:   resumeReq.Event.DeliveryTarget,
			LatestSeq:        resumeReq.Event.Seq,
			LatestEventType:  bridgepkg.DeliveryEventTypeStart,
			CurrentContent:   resumeReq.Event.Content,
			LastSentSeq:      resumeReq.Event.Seq,
			LastAckedSeq:     0,
			UpdatedAt:        time.Date(2026, 5, 16, 13, 1, 0, 0, time.UTC),
		}
		resumeAck, _, err := executeDelivery(context.Background(), api, resumeReq, deliveryState{})
		if err != nil {
			t.Fatalf("executeDelivery(resume) error = %v", err)
		}
		if got, want := resumeAck.RemoteMessageID, startAck.RemoteMessageID; got != want {
			t.Fatalf("resumeAck.RemoteMessageID = %q, want %q", got, want)
		}
		if got, want := len(api.posts), 1; got != want {
			t.Fatalf("PostMessage calls = %d, want %d", got, want)
		}
	})

	t.Run("Should reject a reconciled message without a timestamp", func(t *testing.T) {
		t.Parallel()

		api := &recordingSlackAPI{
			forcedFindResult:    &slackPostedMessage{TS: " "},
			hasForcedFindResult: true,
		}
		resumeReq := testDeliveryRequest(
			"brg-slack",
			"delivery-malformed-reconcile",
			1,
			bridgepkg.DeliveryEventTypeResume,
			false,
		)
		resumeReq.Event.Resume = &bridgepkg.DeliveryResumeState{
			LatestEventType: bridgepkg.DeliveryEventTypeStart,
		}
		resumeReq.Snapshot = &bridgepkg.DeliverySnapshot{
			DeliveryID:       resumeReq.Event.DeliveryID,
			SessionID:        "sess-slack",
			TurnID:           "turn-slack",
			BridgeInstanceID: resumeReq.Event.BridgeInstanceID,
			RoutingKey:       resumeReq.Event.RoutingKey,
			DeliveryTarget:   resumeReq.Event.DeliveryTarget,
			LatestSeq:        resumeReq.Event.Seq,
			LatestEventType:  bridgepkg.DeliveryEventTypeStart,
			CurrentContent:   resumeReq.Event.Content,
			LastSentSeq:      resumeReq.Event.Seq,
			UpdatedAt:        time.Date(2026, 5, 16, 13, 2, 0, 0, time.UTC),
		}

		_, gotState, err := executeDelivery(context.Background(), api, resumeReq, deliveryState{})
		var transientErr *bridgesdk.TransientError
		if !errors.As(err, &transientErr) {
			t.Fatalf("executeDelivery() error = %T %v, want TransientError", err, err)
		}
		if gotState != (deliveryState{}) {
			t.Fatalf("executeDelivery() state = %#v, want unchanged", gotState)
		}
		if got := len(api.posts); got != 0 {
			t.Fatalf("PostMessage calls = %d, want zero after reconciliation", got)
		}
	})
}

func TestSlackContractProgressDelivery(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should render a full progress turn and post the final answer separately in the same thread",
		func(t *testing.T) {
			t.Parallel()

			now := time.Date(2026, 7, 12, 11, 0, 0, 0, time.UTC)
			api := &recordingSlackAPI{}
			provider := newSlackProgressTestProvider(t, func() time.Time { return now }, api)
			session := &bridgesdk.Session{}
			phases := []struct {
				phase bridgepkg.ToolProgressPhase
				label string
			}{
				{phase: bridgepkg.ToolProgressPhaseStarted, label: "Inspecting"},
				{phase: bridgepkg.ToolProgressPhaseCompleted, label: "Inspecting"},
				{phase: bridgepkg.ToolProgressPhaseStarted, label: "Summarizing"},
			}
			for index, phase := range phases {
				request := testProgressRequest(
					"brg-progress",
					"delivery-full-turn",
					int64(index+1),
					phase.phase,
					phase.label,
				)
				if phase.phase == bridgepkg.ToolProgressPhaseCompleted {
					request.Event.Progress.DurationMS = 31_000
				}
				if _, err := provider.handleBridgesProgress(context.Background(), session, request); err != nil {
					t.Fatalf("handleBridgesProgress(%d) error = %v", index, err)
				}
				now = now.Add(2 * time.Second)
			}

			final := testDeliveryRequest(
				"brg-progress",
				"delivery-full-turn",
				4,
				bridgepkg.DeliveryEventTypeFinal,
				true,
			)
			final.Event.Content.Text = "Final **answer**"
			if _, err := provider.handleBridgesDeliver(context.Background(), session, final); err != nil {
				t.Fatalf("handleBridgesDeliver(final) error = %v", err)
			}

			if got, want := len(api.posts), 2; got != want {
				t.Fatalf("post calls = %d, want progress bubble plus final answer (%d)", got, want)
			}
			if got, want := len(api.updates), 2; got != want {
				t.Fatalf("progress edits = %d, want %d", got, want)
			}
			if got, want := api.posts[0].ThreadTS, final.Event.DeliveryTarget.ThreadID; got != want {
				t.Fatalf("progress thread_ts = %q, want %q", got, want)
			}
			if got, want := api.posts[1].ThreadTS, final.Event.DeliveryTarget.ThreadID; got != want {
				t.Fatalf("final thread_ts = %q, want %q", got, want)
			}
			if got, want := api.posts[1].Text, "Final *answer*"; got != want {
				t.Fatalf("final text = %q, want %q", got, want)
			}
			if got := api.updates[1].Text; !strings.Contains(got, "\u2705 Inspecting \u00b7 31s") ||
				!strings.Contains(got, "\U0001f527 Summarizing") {
				t.Fatalf("final progress bubble = %q, want completed duration and next tool", got)
			}
			if got, want := api.statuses[len(api.statuses)-1].Status, ""; got != want {
				t.Fatalf("final status = %q, want cleared status", got)
			}
			if state := provider.deliveryState("brg-progress", "delivery-full-turn"); state.Progress != nil {
				t.Fatal("terminal delivery retained a progress dispatcher")
			}
		},
	)
}

func assertSlackRoutableDirectEnvelope(
	t *testing.T,
	instance bridgepkg.BridgeInstance,
	envelope bridgepkg.InboundMessageEnvelope,
	wantThreadID string,
) {
	t.Helper()

	if got := envelope.ThreadID; got != wantThreadID {
		t.Fatalf("Envelope.ThreadID = %q, want %q", got, wantThreadID)
	}
	if _, err := bridgepkg.BuildRoutingKey(instance, bridgepkg.RoutingDimensions{
		PeerID:   envelope.PeerID,
		ThreadID: envelope.ThreadID,
		GroupID:  envelope.GroupID,
	}); err != nil {
		t.Fatalf("BuildRoutingKey() error = %v", err)
	}
}

type recordingSlackAPI struct {
	posts               []slackPostMessageRequest
	updates             []slackUpdateMessageRequest
	statuses            []slackSetThreadStatusRequest
	reactions           []slackAddReactionRequest
	messages            []slackConversationMessage
	forcedFindResult    *slackPostedMessage
	hasForcedFindResult bool
}

func (a *recordingSlackAPI) AuthTest(context.Context) (*slackAuthIdentity, error) {
	return &slackAuthIdentity{UserID: "U_BOT", BotID: "B_BOT"}, nil
}

func (a *recordingSlackAPI) PostMessage(
	_ context.Context,
	req slackPostMessageRequest,
) (*slackPostedMessage, error) {
	ts := "1775866805.100000"
	if len(a.posts) > 0 {
		ts = "1775866805.200000"
	}
	a.posts = append(a.posts, req)
	a.messages = append(a.messages, slackConversationMessage{
		TS:       ts,
		Metadata: req.Metadata,
	})
	return &slackPostedMessage{TS: ts}, nil
}

func (a *recordingSlackAPI) FindDeliveryMessage(
	_ context.Context,
	req slackFindDeliveryMessageRequest,
) (*slackPostedMessage, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if a.hasForcedFindResult {
		return a.forcedFindResult, nil
	}
	for idx := range a.messages {
		message := a.messages[idx]
		if slackMetadataMatchesDelivery(message.Metadata, req) {
			return &slackPostedMessage{TS: message.TS}, nil
		}
	}
	return nil, nil
}

func (a *recordingSlackAPI) UpdateMessage(_ context.Context, request slackUpdateMessageRequest) error {
	a.updates = append(a.updates, request)
	return nil
}

func (a *recordingSlackAPI) DeleteMessage(context.Context, slackDeleteMessageRequest) error {
	return nil
}

func (a *recordingSlackAPI) SetThreadStatus(
	_ context.Context,
	request slackSetThreadStatusRequest,
) error {
	a.statuses = append(a.statuses, request)
	return nil
}

func (a *recordingSlackAPI) AddReaction(_ context.Context, request slackAddReactionRequest) error {
	a.reactions = append(a.reactions, request)
	return nil
}

package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

const teamsMaxMessageLen = 28_000

func isTerminalTeamsDeliveryEvent(event bridgepkg.DeliveryEvent) bool {
	if event.Operation.Normalize() == bridgepkg.DeliveryOperationDelete {
		return true
	}
	switch normalizeDeliveryEventType(event.EventType) {
	case bridgepkg.DeliveryEventTypeFinal,
		bridgepkg.DeliveryEventTypeError,
		bridgepkg.DeliveryEventTypeDelete:
		return true
	default:
		return false
	}
}

func executeTeamsDelivery(
	ctx context.Context,
	api teamsAPI,
	cfg resolvedInstanceConfig,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
	referenceStateLookup teamsDeliveryStateLookup,
	userContextLookup func(string, string) (teamsUserContext, bool),
) (bridgepkg.DeliveryAck, deliveryState, error) {
	if err := request.Validate(); err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}

	event := request.Event
	if event.EventType != bridgepkg.DeliveryEventTypeResume && event.Seq <= state.LastSeq {
		return bridgepkg.DeliveryAck{}, state, fmt.Errorf(
			"teams: out-of-order delivery seq %d after %d",
			event.Seq,
			state.LastSeq,
		)
	}
	if event.EventType == bridgepkg.DeliveryEventTypeResume && request.Snapshot != nil {
		state.LastSeq = request.Snapshot.LastAckedSeq
		state.RemoteMessageID = strings.TrimSpace(request.Snapshot.RemoteMessageID)
		state.ReplaceRemoteMessageID = strings.TrimSpace(request.Snapshot.ReplaceRemoteMessageID)
	}

	switch {
	case isTeamsDeleteDelivery(event):
		return executeTeamsDeleteDelivery(ctx, api, event, request.Snapshot, state, referenceStateLookup)
	case shouldPostTeamsMessage(event, state, request):
		return executeTeamsPostDelivery(ctx, api, cfg, event, state, userContextLookup)
	default:
		return executeTeamsEditDelivery(
			ctx,
			api,
			cfg,
			event,
			request.Snapshot,
			state,
			referenceStateLookup,
			userContextLookup,
		)
	}
}

func isTeamsDeleteDelivery(event bridgepkg.DeliveryEvent) bool {
	return event.Operation.Normalize() == bridgepkg.DeliveryOperationDelete ||
		normalizeDeliveryEventType(event.EventType) == bridgepkg.DeliveryEventTypeDelete
}

func executeTeamsDeleteDelivery(
	ctx context.Context,
	api teamsAPI,
	event bridgepkg.DeliveryEvent,
	snapshot *bridgepkg.DeliverySnapshot,
	state deliveryState,
	referenceStateLookup teamsDeliveryStateLookup,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	remoteID := resolveTeamsReferencedRemoteMessageID(event.Reference, snapshot, state, referenceStateLookup)
	if remoteID == "" {
		return bridgepkg.DeliveryAck{}, state, errors.New(
			"teams: delete delivery requires a remote message id",
		)
	}
	ref, err := decodeRemoteMessageID(remoteID)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}
	if err := deleteTeamsDeliveryActivity(ctx, api, ref.ServiceURL, ref.ConversationID, ref.ActivityID); err != nil {
		return bridgepkg.DeliveryAck{}, state, fmt.Errorf("teams: delete activity: %w", err)
	}

	ack := newTeamsDeliveryAck(event, remoteID, firstNonEmpty(state.RemoteMessageID, remoteID))
	state.LastSeq = event.Seq
	state.RemoteMessageID = remoteID
	state.ReplaceRemoteMessageID = ack.ReplaceRemoteMessageID
	return ack, state, ack.ValidateFor(event)
}

func executeTeamsPostDelivery(
	ctx context.Context,
	api teamsAPI,
	cfg resolvedInstanceConfig,
	event bridgepkg.DeliveryEvent,
	state deliveryState,
	userContextLookup func(string, string) (teamsUserContext, bool),
) (bridgepkg.DeliveryAck, deliveryState, error) {
	target := state.ResolvedTarget
	if strings.TrimSpace(target.ConversationID) == "" {
		var err error
		target, err = resolveTeamsDeliveryTarget(cfg, event, userContextLookup)
		if err != nil {
			return bridgepkg.DeliveryAck{}, state, err
		}
		target, err = ensureTeamsConversation(ctx, api, cfg, target)
		if err != nil {
			return bridgepkg.DeliveryAck{}, state, err
		}
		state.ResolvedTarget = target
	}

	baseConversationID, replyToID := splitTeamsConversationTarget(
		target.ConversationID,
	)
	if target.ReplyToID != "" {
		replyToID = target.ReplyToID
	}

	chunks := teamsDeliveryChunks(event)
	state.Chunks = bridgesdk.BeginDeliveryChunks(
		state.Chunks,
		event.Seq,
		bridgesdk.DeliveryChunkModeCreate,
		len(chunks),
		event.Content.Text,
		"",
		state.RemoteMessageID,
	)
	for index := state.Chunks.NextChunk(); index < len(chunks); index++ {
		chunk := chunks[index]
		sent, sendErr := sendTeamsDeliveryActivity(
			ctx,
			api,
			target.ServiceURL,
			baseConversationID,
			replyToID,
			teamsOutboundActivity{
				Type:       providerMessageKey,
				Text:       chunk,
				TextFormat: teamsTextFormatMarkdown,
			},
		)
		if sendErr != nil {
			return bridgepkg.DeliveryAck{}, state, fmt.Errorf("teams: send activity chunk %d: %w", index+1, sendErr)
		}
		if sent == nil || strings.TrimSpace(sent.ID) == "" {
			return bridgepkg.DeliveryAck{}, state, &bridgesdk.CommittedMutationError{
				Err: errors.New("teams: send activity response omitted id"),
			}
		}
		remoteID := encodeRemoteMessageID(teamsRemoteMessageRef{
			ConversationID: baseConversationID,
			ServiceURL:     target.ServiceURL,
			ActivityID:     strings.TrimSpace(sent.ID),
		})
		state.Chunks = state.Chunks.Advance(remoteID)
	}

	remoteID := state.Chunks.LastRemoteMessageID()
	replaceRemoteID := state.Chunks.ReplaceRemoteMessageID()
	state.Chunks = bridgesdk.DeliveryChunkCursor{}
	ack := newTeamsDeliveryAck(event, remoteID, replaceRemoteID)
	state.LastSeq = event.Seq
	state.ReplaceRemoteMessageID = replaceRemoteID
	state.RemoteMessageID = remoteID
	state.LastContent = chunks[len(chunks)-1]
	return ack, state, ack.ValidateFor(event)
}

func teamsDeliveryChunks(event bridgepkg.DeliveryEvent) []string {
	chunks := bridgesdk.ChunkMessage(event.Content.Text, teamsMaxMessageLen, nil)
	if !event.Final && len(chunks) > 1 {
		return chunks[:1]
	}
	return chunks
}

func resolveTeamsReferencedRemoteMessageID(
	reference *bridgepkg.DeliveryMessageReference,
	snapshot *bridgepkg.DeliverySnapshot,
	state deliveryState,
	referenceStateLookup teamsDeliveryStateLookup,
) string {
	if remoteID := referenceRemoteMessageID(reference); remoteID != "" {
		return remoteID
	}
	if deliveryID := referenceDeliveryID(reference); deliveryID != "" {
		if referenceStateLookup == nil {
			return ""
		}
		referencedState, ok := referenceStateLookup(deliveryID)
		if !ok {
			return ""
		}
		return strings.TrimSpace(referencedState.RemoteMessageID)
	}
	if remoteID := strings.TrimSpace(state.RemoteMessageID); remoteID != "" {
		return remoteID
	}
	if snapshot != nil {
		return strings.TrimSpace(snapshot.RemoteMessageID)
	}
	return ""
}

func newTeamsDeliveryAck(
	event bridgepkg.DeliveryEvent,
	remoteMessageID string,
	replaceRemoteMessageID string,
) bridgepkg.DeliveryAck {
	ack := bridgepkg.DeliveryAck{
		DeliveryID:      event.DeliveryID,
		Seq:             event.Seq,
		RemoteMessageID: remoteMessageID,
	}
	if strings.TrimSpace(replaceRemoteMessageID) != "" {
		ack.ReplaceRemoteMessageID = strings.TrimSpace(replaceRemoteMessageID)
	}
	return ack
}

func shouldPostTeamsMessage(
	event bridgepkg.DeliveryEvent,
	state deliveryState,
	request bridgepkg.DeliveryRequest,
) bool {
	if event.Operation.Normalize() == bridgepkg.DeliveryOperationEdit {
		return false
	}
	if normalizeDeliveryEventType(event.EventType) == bridgepkg.DeliveryEventTypeStart {
		return true
	}
	if normalizeDeliveryEventType(event.EventType) == bridgepkg.DeliveryEventTypeResume {
		if request.Snapshot == nil {
			return state.RemoteMessageID == ""
		}
		return strings.TrimSpace(request.Snapshot.RemoteMessageID) == ""
	}
	return strings.TrimSpace(state.RemoteMessageID) == ""
}

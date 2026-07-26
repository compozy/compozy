package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

const discordMaxMessageLen = 2_000

type deliveryState struct {
	LastSeq                int64
	RemoteMessageID        string
	ReplaceRemoteMessageID string
	LastContent            string
	Chunks                 bridgesdk.DeliveryChunkCursor
	Progress               *bridgesdk.ProgressDispatcher
}

func (p *discordProvider) deliveryState(instanceID string, deliveryID string) deliveryState {
	state, _ := p.deliveries.Load(deliveryStateKey(instanceID, deliveryID))
	return state
}

func (p *discordProvider) storeDeliveryState(instanceID string, deliveryID string, state deliveryState) {
	p.deliveries.Store(deliveryStateKey(instanceID, deliveryID), state)
}

func executeDiscordDelivery(
	ctx context.Context,
	api discordAPI,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	if err := request.Validate(); err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}

	event := request.Event
	if event.EventType != bridgepkg.DeliveryEventTypeResume && event.Seq <= state.LastSeq {
		return bridgepkg.DeliveryAck{}, state, fmt.Errorf(
			"discord: out-of-order delivery seq %d after %d",
			event.Seq,
			state.LastSeq,
		)
	}
	if event.EventType == bridgepkg.DeliveryEventTypeResume && request.Snapshot != nil {
		state.LastSeq = request.Snapshot.LastAckedSeq
		state.RemoteMessageID = strings.TrimSpace(request.Snapshot.RemoteMessageID)
		state.ReplaceRemoteMessageID = strings.TrimSpace(request.Snapshot.ReplaceRemoteMessageID)
	}

	channelID, err := resolveDiscordDeliveryChannelID(event)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}

	switch {
	case isDiscordDeleteEvent(event):
		return executeDiscordDelete(ctx, api, request, state)
	case shouldPostDiscordMessage(event, state, request):
		return executeDiscordCreate(ctx, api, event, state, channelID)
	default:
		return executeDiscordUpdate(ctx, api, request, state)
	}
}

func shouldPostDiscordMessage(
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

func isDiscordDeleteEvent(event bridgepkg.DeliveryEvent) bool {
	return event.Operation.Normalize() == bridgepkg.DeliveryOperationDelete ||
		normalizeDeliveryEventType(event.EventType) == bridgepkg.DeliveryEventTypeDelete
}

func executeDiscordDelete(
	ctx context.Context,
	api discordAPI,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	event := request.Event
	remoteID := discordRemoteMessageIDFromRequest(request, state)
	if remoteID == "" {
		return bridgepkg.DeliveryAck{}, state, errors.New("discord: delete delivery requires a remote message id")
	}
	channelID, messageID, err := decodeRemoteMessageID(remoteID)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}
	if err := deleteDiscordDeliveryMessage(ctx, api, discordDeleteMessageRequest{
		ChannelID: channelID,
		MessageID: messageID,
	}); err != nil {
		return bridgepkg.DeliveryAck{}, state, fmt.Errorf("discord: delete message: %w", err)
	}
	ack := bridgepkg.DeliveryAck{
		DeliveryID:             event.DeliveryID,
		Seq:                    event.Seq,
		RemoteMessageID:        remoteID,
		ReplaceRemoteMessageID: firstNonEmpty(state.RemoteMessageID, remoteID),
	}
	state.LastSeq = event.Seq
	state.RemoteMessageID = remoteID
	state.ReplaceRemoteMessageID = ack.ReplaceRemoteMessageID
	return ack, state, ack.ValidateFor(event)
}

func executeDiscordCreate(
	ctx context.Context,
	api discordAPI,
	event bridgepkg.DeliveryEvent,
	state deliveryState,
	channelID string,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	chunks := discordDeliveryChunks(event)
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
		sent, err := postDiscordDeliveryMessage(ctx, api, discordPostMessageRequest{
			ChannelID: channelID,
			Content:   chunk,
		})
		if err != nil {
			return bridgepkg.DeliveryAck{}, state, fmt.Errorf("discord: post message chunk %d: %w", index+1, err)
		}
		remoteID, err := discordPostedRemoteID(channelID, sent)
		if err != nil {
			return bridgepkg.DeliveryAck{}, state, err
		}
		state.Chunks = state.Chunks.Advance(remoteID)
	}

	remoteID := state.Chunks.LastRemoteMessageID()
	replaceRemoteID := state.Chunks.ReplaceRemoteMessageID()
	state.Chunks = bridgesdk.DeliveryChunkCursor{}
	ack := bridgepkg.DeliveryAck{
		DeliveryID:      event.DeliveryID,
		Seq:             event.Seq,
		RemoteMessageID: remoteID,
	}
	state.LastSeq = event.Seq
	state.ReplaceRemoteMessageID = replaceRemoteID
	state.RemoteMessageID = remoteID
	state.LastContent = chunks[len(chunks)-1]
	if state.ReplaceRemoteMessageID != "" {
		ack.ReplaceRemoteMessageID = state.ReplaceRemoteMessageID
	}
	return ack, state, ack.ValidateFor(event)
}

func executeDiscordUpdate(
	ctx context.Context,
	api discordAPI,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	event := request.Event
	remoteID := discordRemoteMessageIDFromRequest(request, state)
	if remoteID == "" {
		return bridgepkg.DeliveryAck{}, state, errors.New("discord: edit delivery requires a remote message id")
	}
	channelID, messageID, err := decodeRemoteMessageID(remoteID)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}

	chunks := discordDeliveryChunks(event)
	replaceRemoteID := firstNonEmpty(state.RemoteMessageID, remoteID)
	state.Chunks = bridgesdk.BeginDeliveryChunks(
		state.Chunks,
		event.Seq,
		bridgesdk.DeliveryChunkModeUpdate,
		len(chunks),
		event.Content.Text,
		remoteID,
		replaceRemoteID,
	)
	for index := state.Chunks.NextChunk(); index < len(chunks); index++ {
		chunk := chunks[index]
		if index == 0 {
			if remoteID != state.RemoteMessageID || chunk != state.LastContent {
				if err := updateDiscordDeliveryMessage(ctx, api, discordUpdateMessageRequest{
					ChannelID: channelID,
					MessageID: messageID,
					Content:   chunk,
				}); err != nil {
					return bridgepkg.DeliveryAck{}, state, fmt.Errorf("discord: update message: %w", err)
				}
			}
			state.Chunks = state.Chunks.Advance(remoteID)
			continue
		}

		sent, err := postDiscordDeliveryMessage(ctx, api, discordPostMessageRequest{
			ChannelID: channelID,
			Content:   chunk,
		})
		if err != nil {
			return bridgepkg.DeliveryAck{}, state, fmt.Errorf("discord: post continuation %d: %w", index, err)
		}
		lastRemoteID, err := discordPostedRemoteID(channelID, sent)
		if err != nil {
			return bridgepkg.DeliveryAck{}, state, err
		}
		state.Chunks = state.Chunks.Advance(lastRemoteID)
	}

	lastRemoteID := state.Chunks.LastRemoteMessageID()
	replaceRemoteID = state.Chunks.ReplaceRemoteMessageID()
	state.Chunks = bridgesdk.DeliveryChunkCursor{}
	ack := bridgepkg.DeliveryAck{
		DeliveryID:             event.DeliveryID,
		Seq:                    event.Seq,
		RemoteMessageID:        lastRemoteID,
		ReplaceRemoteMessageID: replaceRemoteID,
	}
	state.LastSeq = event.Seq
	state.RemoteMessageID = lastRemoteID
	state.ReplaceRemoteMessageID = replaceRemoteID
	state.LastContent = chunks[len(chunks)-1]
	return ack, state, ack.ValidateFor(event)
}

func discordPostedRemoteID(channelID string, sent *discordPostedMessage) (string, error) {
	if sent == nil || strings.TrimSpace(sent.ID) == "" {
		return "", &bridgesdk.CommittedMutationError{
			Err: errors.New("discord: post message response omitted id"),
		}
	}
	return encodeRemoteMessageID(channelID, strings.TrimSpace(sent.ID)), nil
}

func discordDeliveryChunks(event bridgepkg.DeliveryEvent) []string {
	chunks := bridgesdk.ChunkMessage(event.Content.Text, discordMaxMessageLen, nil)
	if !event.Final && len(chunks) > 1 {
		return chunks[:1]
	}
	return chunks
}

func discordRemoteMessageIDFromRequest(request bridgepkg.DeliveryRequest, state deliveryState) string {
	remoteID := firstNonEmpty(referenceRemoteMessageID(request.Event.Reference), state.RemoteMessageID)
	if remoteID == "" && request.Snapshot != nil {
		return strings.TrimSpace(request.Snapshot.RemoteMessageID)
	}
	return remoteID
}

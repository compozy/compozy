package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

const slackMaxMessageLen = 40_000

type deliveryState struct {
	LastSeq                int64
	RemoteMessageID        string
	ReplaceRemoteMessageID string
	Chunks                 bridgesdk.DeliveryChunkCursor
	Progress               *bridgesdk.ProgressDispatcher
}

func (p *slackProvider) deliveryState(instanceID string, deliveryID string) deliveryState {
	state, _ := p.deliveries.Load(deliveryStateKey(instanceID, deliveryID))
	return state
}

func (p *slackProvider) storeDeliveryState(
	instanceID string,
	deliveryID string,
	event bridgepkg.DeliveryEvent,
	state deliveryState,
) *bridgesdk.ProgressDispatcher {
	key := deliveryStateKey(instanceID, deliveryID)
	if isTerminalSlackDeliveryEvent(event) {
		p.deliveries.Delete(key)
		return state.Progress
	}
	p.deliveries.Store(key, state)
	return nil
}

func (p *slackProvider) storeDeliveryRetryState(
	instanceID string,
	deliveryID string,
	state deliveryState,
) {
	if !state.Chunks.Active() {
		return
	}
	p.deliveries.Store(deliveryStateKey(instanceID, deliveryID), state)
}

func (p *slackProvider) detachProgressDispatcher(
	instanceID string,
	deliveryID string,
	removeState bool,
) *bridgesdk.ProgressDispatcher {
	key := deliveryStateKey(instanceID, deliveryID)
	state, ok := p.deliveries.Load(key)
	if !ok {
		return nil
	}
	dispatcher := state.Progress
	if removeState {
		p.deliveries.Delete(key)
	} else {
		p.deliveries.Update(key, func(current deliveryState) deliveryState {
			current.Progress = nil
			return current
		})
	}
	return dispatcher
}

func (p *slackProvider) takeAllProgressDispatchers() []*bridgesdk.ProgressDispatcher {
	dispatchers := make([]*bridgesdk.ProgressDispatcher, 0)
	p.deliveries.TransformAll(func(_ string, state deliveryState) deliveryState {
		if state.Progress == nil {
			return state
		}
		dispatchers = append(dispatchers, state.Progress)
		state.Progress = nil
		return state
	})
	return dispatchers
}

func executeDelivery(
	ctx context.Context,
	api slackAPI,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	if err := request.Validate(); err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}

	event := request.Event
	if event.EventType != bridgepkg.DeliveryEventTypeResume && event.Seq <= state.LastSeq {
		return bridgepkg.DeliveryAck{}, state, fmt.Errorf(
			"slack: out-of-order delivery seq %d after %d",
			event.Seq,
			state.LastSeq,
		)
	}
	if event.EventType == bridgepkg.DeliveryEventTypeResume && request.Snapshot != nil {
		state.LastSeq = request.Snapshot.LastAckedSeq
		state.RemoteMessageID = strings.TrimSpace(request.Snapshot.RemoteMessageID)
		state.ReplaceRemoteMessageID = strings.TrimSpace(request.Snapshot.ReplaceRemoteMessageID)
	}

	channelID, threadTS, err := resolveDeliveryTarget(event)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}

	switch {
	case isSlackDeleteEvent(event):
		return executeSlackDelete(ctx, api, request, state)
	case shouldPostNewMessage(event, state, request):
		return executeSlackCreate(ctx, api, request, state, channelID, threadTS)
	default:
		return executeSlackUpdate(ctx, api, request, state, threadTS)
	}
}

func isSlackDeleteEvent(event bridgepkg.DeliveryEvent) bool {
	return event.Operation.Normalize() == bridgepkg.DeliveryOperationDelete ||
		normalizeDeliveryEventType(event.EventType) == bridgepkg.DeliveryEventTypeDelete
}

func isTerminalSlackDeliveryEvent(event bridgepkg.DeliveryEvent) bool {
	if event.Operation.Normalize() == bridgepkg.DeliveryOperationDelete {
		return true
	}
	switch normalizeDeliveryEventType(event.EventType) {
	case bridgepkg.DeliveryEventTypeFinal, bridgepkg.DeliveryEventTypeError, bridgepkg.DeliveryEventTypeDelete:
		return true
	default:
		return false
	}
}

func executeSlackDelete(
	ctx context.Context,
	api slackAPI,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	event := request.Event
	remoteID := slackRemoteMessageIDFromRequest(request, state)
	if remoteID == "" {
		return bridgepkg.DeliveryAck{}, state, errors.New("slack: delete delivery requires a remote message id")
	}
	channel, timestamp, err := decodeRemoteMessageID(remoteID)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}
	if err := deleteSlackDeliveryMessage(
		ctx,
		api,
		slackDeleteMessageRequest{Channel: channel, TS: timestamp},
	); err != nil {
		return bridgepkg.DeliveryAck{}, state, err
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

func executeSlackCreate(
	ctx context.Context,
	api slackAPI,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
	channelID string,
	threadTS string,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	event := request.Event
	if normalizeDeliveryEventType(event.EventType) == bridgepkg.DeliveryEventTypeResume && !state.Chunks.Active() {
		matched, err := reconcileSlackDelivery(ctx, api, event, channelID, threadTS)
		if err != nil {
			return bridgepkg.DeliveryAck{}, state, err
		}
		if matched != nil {
			matchedTimestamp, err := slackPostedTimestamp(matched)
			if err != nil {
				return bridgepkg.DeliveryAck{}, state, err
			}
			return slackCreateAck(event, state, channelID, matchedTimestamp, state.RemoteMessageID)
		}
	}

	chunks := slackDeliveryChunks(event)
	state.Chunks = bridgesdk.BeginDeliveryChunks(
		state.Chunks,
		event.Seq,
		bridgesdk.DeliveryChunkModeCreate,
		len(chunks),
		event.Content.Text,
		"",
		state.RemoteMessageID,
	)
	lastTimestamp := ""
	if remoteID := state.Chunks.LastRemoteMessageID(); remoteID != "" {
		_, decodedTimestamp, err := decodeRemoteMessageID(remoteID)
		if err != nil {
			return bridgepkg.DeliveryAck{}, state, err
		}
		lastTimestamp = decodedTimestamp
	}
	for index := state.Chunks.NextChunk(); index < len(chunks); index++ {
		chunk := chunks[index]
		request := slackPostMessageRequest{
			Channel:  channelID,
			ThreadTS: threadTS,
			Text:     chunk,
			Mrkdwn:   true,
		}
		if index == len(chunks)-1 {
			request.Metadata = slackDeliveryMetadata(event)
		}
		sent, err := postSlackDeliveryMessage(ctx, api, request)
		if err != nil {
			return bridgepkg.DeliveryAck{}, state, err
		}
		lastTimestamp, err = slackPostedTimestamp(sent)
		if err != nil {
			return bridgepkg.DeliveryAck{}, state, err
		}
		state.Chunks = state.Chunks.Advance(encodeRemoteMessageID(channelID, lastTimestamp))
	}
	replaceRemoteID := state.Chunks.ReplaceRemoteMessageID()
	state.Chunks = bridgesdk.DeliveryChunkCursor{}
	return slackCreateAck(event, state, channelID, lastTimestamp, replaceRemoteID)
}

func reconcileSlackDelivery(
	ctx context.Context,
	api slackAPI,
	event bridgepkg.DeliveryEvent,
	channelID string,
	threadTS string,
) (*slackPostedMessage, error) {
	reconciler, ok := api.(slackDeliveryReconciler)
	if !ok {
		return nil, nil
	}
	return findSlackDeliveryMessage(ctx, reconciler, slackFindDeliveryMessageRequest{
		Channel:          channelID,
		ThreadTS:         threadTS,
		DeliveryID:       event.DeliveryID,
		BridgeInstanceID: event.BridgeInstanceID,
	})
}

func slackCreateAck(
	event bridgepkg.DeliveryEvent,
	state deliveryState,
	channelID string,
	timestamp string,
	replaceRemoteMessageID string,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	remoteID := encodeRemoteMessageID(channelID, timestamp)
	ack := bridgepkg.DeliveryAck{
		DeliveryID:      event.DeliveryID,
		Seq:             event.Seq,
		RemoteMessageID: remoteID,
	}
	state.LastSeq = event.Seq
	state.ReplaceRemoteMessageID = replaceRemoteMessageID
	state.RemoteMessageID = remoteID
	if state.ReplaceRemoteMessageID != "" {
		ack.ReplaceRemoteMessageID = state.ReplaceRemoteMessageID
	}
	return ack, state, ack.ValidateFor(event)
}

func slackDeliveryMetadata(event bridgepkg.DeliveryEvent) *slackMessageMetadata {
	return &slackMessageMetadata{
		EventType: providerAghBridgeDeliveryKey,
		EventPayload: slackMessageMetadataPayload{
			BridgeInstanceID: strings.TrimSpace(event.BridgeInstanceID),
			DeliveryID:       strings.TrimSpace(event.DeliveryID),
		},
	}
}

func executeSlackUpdate(
	ctx context.Context,
	api slackAPI,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
	threadTS string,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	event := request.Event
	remoteID := slackRemoteMessageIDFromRequest(request, state)
	if remoteID == "" {
		return bridgepkg.DeliveryAck{}, state, errors.New("slack: edit delivery requires a remote message id")
	}
	channel, timestamp, err := decodeRemoteMessageID(remoteID)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}

	chunks := slackDeliveryChunks(event)
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
			if err := updateSlackDeliveryMessage(ctx, api, slackUpdateMessageRequest{
				Channel: channel,
				TS:      timestamp,
				Text:    chunk,
			}); err != nil {
				return bridgepkg.DeliveryAck{}, state, err
			}
			state.Chunks = state.Chunks.Advance(remoteID)
			continue
		}

		post := slackPostMessageRequest{
			Channel:  channel,
			ThreadTS: threadTS,
			Text:     chunk,
			Mrkdwn:   true,
		}
		if index == len(chunks)-1 {
			post.Metadata = slackDeliveryMetadata(event)
		}
		sent, err := postSlackDeliveryMessage(ctx, api, post)
		if err != nil {
			return bridgepkg.DeliveryAck{}, state, err
		}
		postedTimestamp, err := slackPostedTimestamp(sent)
		if err != nil {
			return bridgepkg.DeliveryAck{}, state, err
		}
		state.Chunks = state.Chunks.Advance(encodeRemoteMessageID(channel, postedTimestamp))
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
	state.ReplaceRemoteMessageID = ack.ReplaceRemoteMessageID
	return ack, state, ack.ValidateFor(event)
}

func slackPostedTimestamp(message *slackPostedMessage) (string, error) {
	if message == nil {
		return "", &bridgesdk.TransientError{
			Err: errors.New("slack: post message returned no response"),
		}
	}
	timestamp := strings.TrimSpace(message.TS)
	if timestamp == "" {
		return "", &bridgesdk.TransientError{
			Err: errors.New("slack: post message returned an empty timestamp"),
		}
	}
	return timestamp, nil
}

func slackDeliveryChunks(event bridgepkg.DeliveryEvent) []string {
	chunks := bridgesdk.ChunkMessage(
		formatOutbound(event.Content.Text),
		slackMaxMessageLen,
		bridgesdk.UTF16Len,
	)
	if !materializeAllSlackChunks(event) && len(chunks) > 1 {
		return chunks[:1]
	}
	return chunks
}

func materializeAllSlackChunks(event bridgepkg.DeliveryEvent) bool {
	return event.Final
}

func slackRemoteMessageIDFromRequest(request bridgepkg.DeliveryRequest, state deliveryState) string {
	remoteID := firstNonEmpty(referenceRemoteMessageID(request.Event.Reference), state.RemoteMessageID)
	if remoteID == "" && request.Snapshot != nil {
		return strings.TrimSpace(request.Snapshot.RemoteMessageID)
	}
	return remoteID
}

func shouldPostNewMessage(
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

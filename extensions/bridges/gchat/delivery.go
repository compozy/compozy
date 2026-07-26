package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

const gchatMaxMessageBytes = 32_000

type gchatResolvedTarget struct {
	SpaceName  string
	ThreadName string
}

type deliveryState struct {
	LastSeq                int64
	RemoteMessageID        string
	ReplaceRemoteMessageID string
	Chunks                 bridgesdk.DeliveryChunkCursor
	Progress               *bridgesdk.ProgressDispatcher
}

func (p *gchatProvider) deliveryState(instanceID string, deliveryID string) deliveryState {
	state, _ := p.deliveries.Load(deliveryStateKey(instanceID, deliveryID))
	return state
}

func (p *gchatProvider) storeDeliveryState(
	instanceID string,
	deliveryID string,
	event bridgepkg.DeliveryEvent,
	state deliveryState,
) {
	key := deliveryStateKey(instanceID, deliveryID)
	if isTerminalGChatDeliveryEvent(event) {
		p.deliveries.Delete(key)
		return
	}
	p.deliveries.Store(key, state)
}

func (p *gchatProvider) storeDeliveryRetryState(
	instanceID string,
	deliveryID string,
	state deliveryState,
) {
	if !state.Chunks.Active() {
		return
	}
	p.deliveries.Store(deliveryStateKey(instanceID, deliveryID), state)
}

func isTerminalGChatDeliveryEvent(event bridgepkg.DeliveryEvent) bool {
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

func executeGChatDelivery(
	ctx context.Context,
	api gchatAPI,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	if err := request.Validate(); err != nil {
		return bridgepkg.DeliveryAck{}, state, fmt.Errorf("gchat: validate delivery request: %w", err)
	}

	event := request.Event
	if event.EventType != bridgepkg.DeliveryEventTypeResume && event.Seq <= state.LastSeq {
		return bridgepkg.DeliveryAck{}, state, fmt.Errorf(
			"gchat: out-of-order delivery seq %d after %d",
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
	case isGChatDeleteEvent(event):
		return executeGChatDelete(ctx, api, request, state)
	case event.Operation.Normalize() == bridgepkg.DeliveryOperationEdit:
		return executeGChatUpdate(ctx, api, request, state)
	case shouldPostGChatMessage(event, state, request):
		return executeGChatCreate(ctx, api, event, state)
	default:
		return executeGChatUpdate(ctx, api, request, state)
	}
}

func isGChatDeleteEvent(event bridgepkg.DeliveryEvent) bool {
	return event.Operation.Normalize() == bridgepkg.DeliveryOperationDelete ||
		normalizeDeliveryEventType(event.EventType) == bridgepkg.DeliveryEventTypeDelete
}

func executeGChatDelete(
	ctx context.Context,
	api gchatAPI,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	event := request.Event
	messageName := gchatRemoteMessageIDFromRequest(request, state)
	if messageName == "" {
		return bridgepkg.DeliveryAck{}, state, errors.New("gchat: delete delivery requires a remote message id")
	}
	if err := deleteGChatDeliveryMessage(ctx, api, messageName); err != nil {
		return bridgepkg.DeliveryAck{}, state, fmt.Errorf("gchat: delete message: %w", err)
	}
	state.LastSeq = event.Seq
	state.RemoteMessageID = messageName
	state.ReplaceRemoteMessageID = messageName
	return validateGChatDeliveryAck(event, state)
}

func executeGChatCreate(
	ctx context.Context,
	api gchatAPI,
	event bridgepkg.DeliveryEvent,
	state deliveryState,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	text, err := gchatDeliveryText(event)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}
	target, err := resolveGChatDeliveryTarget(event)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, fmt.Errorf("gchat: resolve delivery target: %w", err)
	}

	chunks := gchatDeliveryChunks(text)
	if !event.Final {
		chunks = chunks[:1]
	}
	state.Chunks = bridgesdk.BeginDeliveryChunks(
		state.Chunks,
		event.Seq,
		bridgesdk.DeliveryChunkModeCreate,
		len(chunks),
		text,
		"",
		state.RemoteMessageID,
	)
	state.Chunks, err = postGChatChunksWithCursor(ctx, api, target, chunks, state.Chunks)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}
	remoteID := state.Chunks.LastRemoteMessageID()
	previousRemoteID := state.Chunks.ReplaceRemoteMessageID()
	state.Chunks = bridgesdk.DeliveryChunkCursor{}
	state.LastSeq = event.Seq
	state.RemoteMessageID = remoteID
	state.ReplaceRemoteMessageID = previousRemoteID
	return validateGChatDeliveryAck(event, state)
}

func executeGChatUpdate(
	ctx context.Context,
	api gchatAPI,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	event := request.Event
	text, err := gchatDeliveryText(event)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}
	messageName := gchatRemoteMessageIDFromRequest(request, state)
	if messageName == "" {
		return bridgepkg.DeliveryAck{}, state, errors.New("gchat: edit delivery requires a remote message id")
	}

	chunks := gchatDeliveryChunks(text)
	if !event.Final {
		chunks = chunks[:1]
	}
	state.Chunks = bridgesdk.BeginDeliveryChunks(
		state.Chunks,
		event.Seq,
		bridgesdk.DeliveryChunkModeUpdate,
		len(chunks),
		text,
		messageName,
		messageName,
	)
	if state.Chunks.NextChunk() == 0 {
		updated, updateErr := updateGChatDeliveryMessage(ctx, api, gchatUpdateMessageRequest{
			MessageName: messageName,
			Text:        chunks[0],
		})
		if updateErr != nil {
			return bridgepkg.DeliveryAck{}, state, fmt.Errorf("gchat: update message: %w", updateErr)
		}
		remoteID := messageName
		if updated != nil && strings.TrimSpace(updated.Name) != "" {
			remoteID = strings.TrimSpace(updated.Name)
		}
		state.Chunks = state.Chunks.Advance(remoteID)
	}
	if len(chunks) > 1 {
		target, targetErr := resolveGChatDeliveryTarget(event)
		if targetErr != nil {
			return bridgepkg.DeliveryAck{}, state, fmt.Errorf("gchat: resolve continuation target: %w", targetErr)
		}
		state.Chunks, err = postGChatChunksWithCursor(ctx, api, target, chunks, state.Chunks)
		if err != nil {
			return bridgepkg.DeliveryAck{}, state, err
		}
	}

	remoteID := state.Chunks.LastRemoteMessageID()
	replaceRemoteID := state.Chunks.ReplaceRemoteMessageID()
	state.Chunks = bridgesdk.DeliveryChunkCursor{}
	state.LastSeq = event.Seq
	state.RemoteMessageID = remoteID
	state.ReplaceRemoteMessageID = replaceRemoteID
	return validateGChatDeliveryAck(event, state)
}

func postGChatChunks(
	ctx context.Context,
	api gchatAPI,
	target gchatResolvedTarget,
	chunks []string,
) (string, error) {
	cursor := bridgesdk.BeginDeliveryChunks(
		bridgesdk.DeliveryChunkCursor{},
		1,
		bridgesdk.DeliveryChunkModeCreate,
		len(chunks),
		strings.Join(chunks, "\x00"),
		"",
		"",
	)
	cursor, err := postGChatChunksWithCursor(ctx, api, target, chunks, cursor)
	if err != nil {
		return "", err
	}
	return cursor.LastRemoteMessageID(), nil
}

func postGChatChunksWithCursor(
	ctx context.Context,
	api gchatAPI,
	target gchatResolvedTarget,
	chunks []string,
	cursor bridgesdk.DeliveryChunkCursor,
) (bridgesdk.DeliveryChunkCursor, error) {
	for index := cursor.NextChunk(); index < len(chunks); index++ {
		chunk := chunks[index]
		message, err := createGChatDeliveryMessage(ctx, api, gchatCreateMessageRequest{
			SpaceName:  target.SpaceName,
			ThreadName: target.ThreadName,
			Text:       chunk,
		})
		if err != nil {
			return cursor, fmt.Errorf("gchat: create message chunk %d: %w", index+1, err)
		}
		if message == nil || strings.TrimSpace(message.Name) == "" {
			return cursor, &bridgesdk.CommittedMutationError{
				Err: errors.New("gchat: create message response omitted name"),
			}
		}
		cursor = cursor.Advance(strings.TrimSpace(message.Name))
	}
	return cursor, nil
}

func validateGChatDeliveryAck(
	event bridgepkg.DeliveryEvent,
	state deliveryState,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	ack := bridgepkg.DeliveryAck{
		DeliveryID:             event.DeliveryID,
		Seq:                    event.Seq,
		RemoteMessageID:        state.RemoteMessageID,
		ReplaceRemoteMessageID: state.ReplaceRemoteMessageID,
	}
	if err := ack.ValidateFor(event); err != nil {
		return bridgepkg.DeliveryAck{}, state, fmt.Errorf("gchat: validate delivery ack: %w", err)
	}
	return ack, state, nil
}

func gchatDeliveryChunks(text string) []string {
	return bridgesdk.ChunkMessage(text, gchatMaxMessageBytes, func(value string) int {
		return len(value)
	})
}

func gchatDeliveryText(event bridgepkg.DeliveryEvent) (string, error) {
	text := event.Content.Text
	if strings.TrimSpace(text) == "" {
		return "", &bridgesdk.PermanentError{Err: errors.New("gchat: text delivery content is required")}
	}
	return text, nil
}

func gchatRemoteMessageIDFromRequest(request bridgepkg.DeliveryRequest, state deliveryState) string {
	messageName := firstNonEmpty(referenceRemoteMessageID(request.Event.Reference), state.RemoteMessageID)
	if messageName == "" && request.Snapshot != nil {
		return strings.TrimSpace(request.Snapshot.RemoteMessageID)
	}
	return messageName
}

func shouldPostGChatMessage(
	event bridgepkg.DeliveryEvent,
	state deliveryState,
	request bridgepkg.DeliveryRequest,
) bool {
	if normalizeDeliveryEventType(event.EventType) == bridgepkg.DeliveryEventTypeStart {
		return true
	}
	if normalizeDeliveryEventType(event.EventType) == bridgepkg.DeliveryEventTypeResume {
		if request.Snapshot == nil {
			return strings.TrimSpace(state.RemoteMessageID) == ""
		}
		return strings.TrimSpace(request.Snapshot.RemoteMessageID) == ""
	}
	return strings.TrimSpace(state.RemoteMessageID) == ""
}

package main

import (
	"context"

	"errors"
	"fmt"

	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func executeLinearDelivery(
	ctx context.Context,
	api linearAPI,
	cfg resolvedInstanceConfig,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	event := request.Event
	if event.Seq <= state.LastSeq {
		return bridgepkg.DeliveryAck{}, state, fmt.Errorf(
			"linear: out-of-order delivery seq %d after %d",
			event.Seq,
			state.LastSeq,
		)
	}

	switch cfg.mode {
	case linearModeComments:
		return executeLinearCommentDelivery(ctx, api, request, state)
	case linearModeAgentSessions:
		return executeLinearAgentSessionDelivery(ctx, api, request, state)
	default:
		return bridgepkg.DeliveryAck{}, state, &bridgesdk.PermanentError{
			Err: fmt.Errorf("linear: unsupported runtime mode %q", cfg.mode),
		}
	}
}

func executeLinearCommentDelivery(
	ctx context.Context,
	api linearAPI,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	event := request.Event

	if event.Operation.Normalize() == bridgepkg.DeliveryOperationDelete ||
		normalizeDeliveryEventType(event.EventType) == bridgepkg.DeliveryEventTypeDelete {
		remoteID := resolveLinearRemoteMessageID(event.Reference, state, request.Snapshot)
		if strings.TrimSpace(remoteID) == "" {
			return bridgepkg.DeliveryAck{}, state, &bridgesdk.PermanentError{
				Err: errors.New("linear: delete requires a remote message id"),
			}
		}
		if err := api.DeleteComment(ctx, remoteID); err != nil {
			return bridgepkg.DeliveryAck{}, state, err
		}
		next := state
		next.LastSeq = event.Seq
		next.LastContent = ""
		ack := bridgepkg.DeliveryAck{
			DeliveryID:      event.DeliveryID,
			Seq:             event.Seq,
			RemoteMessageID: remoteID,
		}
		return ack, next, nil
	}

	thread, err := decodeLinearThreadID(
		firstNonEmpty(
			event.DeliveryTarget.ThreadID,
			event.RoutingKey.ThreadID,
			issueThreadIDFromGroup(event.DeliveryTarget.GroupID, event.RoutingKey.GroupID),
		),
	)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, &bridgesdk.PermanentError{Err: err}
	}
	body := event.Content.Text
	remoteID := resolveLinearRemoteMessageID(event.Reference, state, request.Snapshot)

	var comment *linearComment
	switch {
	case shouldSkipLinearCommentDelivery(event, remoteID, body):
		comment = nil
	case strings.TrimSpace(remoteID) == "":
		comment, err = api.CreateComment(ctx, thread.IssueID, body, thread.RootCommentID)
	default:
		comment, err = api.UpdateComment(ctx, remoteID, body)
	}
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}
	if comment != nil {
		remoteID = comment.ID
	}

	next := state
	next.LastSeq = event.Seq
	next.LastContent = body
	next.ReplaceRemoteMessageID = state.RemoteMessageID
	next.RemoteMessageID = strings.TrimSpace(remoteID)

	ack := bridgepkg.DeliveryAck{
		DeliveryID:             event.DeliveryID,
		Seq:                    event.Seq,
		RemoteMessageID:        strings.TrimSpace(remoteID),
		ReplaceRemoteMessageID: strings.TrimSpace(state.RemoteMessageID),
	}
	return ack, next, nil
}

func executeLinearAgentSessionDelivery(
	ctx context.Context,
	api linearAPI,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	event := request.Event
	if err := validateLinearAgentSessionDelivery(event); err != nil {
		return bridgepkg.DeliveryAck{}, state, &bridgesdk.PermanentError{Err: err}
	}

	thread, err := decodeLinearAgentSessionThread(event)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, &bridgesdk.PermanentError{Err: err}
	}

	remoteID := resolveLinearRemoteMessageID(event.Reference, state, request.Snapshot)
	if ack, next, ok := resumeLinearAgentSessionDelivery(event, state, remoteID); ok {
		return ack, next, nil
	}

	content := event.Content.Text
	delta := computeLinearAppendDelta(state.LastContent, content)
	if normalizeDeliveryEventType(event.EventType) == bridgepkg.DeliveryEventTypeResume &&
		strings.TrimSpace(remoteID) == "" {
		delta = content
	}

	next := state
	next.LastSeq = event.Seq
	next.LastContent = content
	next.ReplaceRemoteMessageID = state.RemoteMessageID
	next.RemoteMessageID = remoteID

	if delta == "" {
		ack := bridgepkg.DeliveryAck{
			DeliveryID:             event.DeliveryID,
			Seq:                    event.Seq,
			RemoteMessageID:        remoteID,
			ReplaceRemoteMessageID: firstNonEmpty(state.RemoteMessageID, remoteID),
		}
		return ack, next, nil
	}

	activity, err := api.CreateAgentActivity(ctx, thread.AgentSessionID, delta)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}
	remoteID = strings.TrimSpace(activity.ID)
	if activity.SourceComment != nil && strings.TrimSpace(activity.SourceComment.ID) != "" {
		remoteID = strings.TrimSpace(activity.SourceComment.ID)
	}
	next.RemoteMessageID = remoteID

	ack := bridgepkg.DeliveryAck{
		DeliveryID:             event.DeliveryID,
		Seq:                    event.Seq,
		RemoteMessageID:        remoteID,
		ReplaceRemoteMessageID: strings.TrimSpace(state.RemoteMessageID),
	}
	return ack, next, nil
}

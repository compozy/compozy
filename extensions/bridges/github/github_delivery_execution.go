package main

import (
	"context"

	"errors"
	"fmt"

	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

func executeGitHubDelivery(
	ctx context.Context,
	api githubAPI,
	cfg *resolvedInstanceConfig,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
	installationID int64,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	event := request.Event
	if normalizeDeliveryEventType(event.EventType) == bridgepkg.DeliveryEventTypeResume &&
		event.Seq == state.LastSeq {
		return ackGitHubIdempotentResume(request, state)
	}
	if event.Seq <= state.LastSeq {
		return bridgepkg.DeliveryAck{}, state, fmt.Errorf(
			"github: out-of-order delivery seq %d after %d",
			event.Seq,
			state.LastSeq,
		)
	}

	if isGitHubDeleteEvent(event) {
		return executeGitHubDelete(ctx, api, request, state, installationID)
	}

	target, err := resolveGitHubDeliveryTarget(cfg, event)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}
	if shouldPostGitHubMessage(event, state, request) {
		return executeGitHubCreate(ctx, api, event, target, state, installationID)
	}

	return executeGitHubUpdate(ctx, api, request, state, installationID)
}

func ackGitHubIdempotentResume(
	request bridgepkg.DeliveryRequest,
	state deliveryState,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	event := request.Event
	remoteID := gitHubRemoteIDFromRequest(request, state)
	if remoteID == "" {
		return bridgepkg.DeliveryAck{}, state, errors.New("github: resume delivery requires remote message id")
	}
	replaceRemoteID := strings.TrimSpace(state.ReplaceRemoteMessageID)
	if request.Snapshot != nil {
		replaceRemoteID = firstNonEmpty(
			replaceRemoteID,
			strings.TrimSpace(request.Snapshot.ReplaceRemoteMessageID),
		)
	}
	replaceRemoteID = firstNonEmpty(replaceRemoteID, remoteID)
	ack := bridgepkg.DeliveryAck{
		DeliveryID:             event.DeliveryID,
		Seq:                    event.Seq,
		RemoteMessageID:        remoteID,
		ReplaceRemoteMessageID: replaceRemoteID,
	}
	return ack, state, ack.ValidateFor(event)
}

func isGitHubDeleteEvent(event bridgepkg.DeliveryEvent) bool {
	return event.Operation.Normalize() == bridgepkg.DeliveryOperationDelete ||
		normalizeDeliveryEventType(event.EventType) == bridgepkg.DeliveryEventTypeDelete
}

func executeGitHubDelete(
	ctx context.Context,
	api githubAPI,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
	installationID int64,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	event := request.Event
	remoteID := gitHubRemoteIDFromRequest(request, state)
	ref, err := parseGitHubRemoteCommentRef(remoteID)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}
	if err := deleteGitHubComment(ctx, api, ref, installationID); err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}
	state.LastSeq = event.Seq
	state.ReplaceRemoteMessageID = remoteID
	ack := bridgepkg.DeliveryAck{
		DeliveryID:             event.DeliveryID,
		Seq:                    event.Seq,
		RemoteMessageID:        remoteID,
		ReplaceRemoteMessageID: remoteID,
	}
	return ack, state, ack.ValidateFor(event)
}

func deleteGitHubComment(ctx context.Context, api githubAPI, ref githubRemoteCommentRef, installationID int64) error {
	switch ref.Kind {
	case githubRemoteCommentKindReview:
		return api.DeleteReviewComment(ctx, ref.CommentID, installationID)
	default:
		return api.DeleteIssueComment(ctx, ref.CommentID, installationID)
	}
}

func executeGitHubCreate(
	ctx context.Context,
	api githubAPI,
	event bridgepkg.DeliveryEvent,
	target githubThreadRef,
	state deliveryState,
	installationID int64,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	remoteID, err := createGitHubComment(ctx, api, target, event.Content.Text, installationID)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}
	state.LastSeq = event.Seq
	state.RemoteMessageID = remoteID
	if event.Seq > 1 {
		state.ReplaceRemoteMessageID = remoteID
	}
	ack := bridgepkg.DeliveryAck{
		DeliveryID:             event.DeliveryID,
		Seq:                    event.Seq,
		RemoteMessageID:        state.RemoteMessageID,
		ReplaceRemoteMessageID: state.ReplaceRemoteMessageID,
	}
	return ack, state, ack.ValidateFor(event)
}

func createGitHubComment(
	ctx context.Context,
	api githubAPI,
	target githubThreadRef,
	body string,
	installationID int64,
) (string, error) {
	if target.ReviewCommentID > 0 {
		comment, err := api.CreateReviewCommentReply(
			ctx,
			target.Number,
			target.ReviewCommentID,
			body,
			installationID,
		)
		if err != nil {
			return "", err
		}
		return encodeGitHubRemoteCommentRef(
			githubRemoteCommentRef{
				Kind:      githubRemoteCommentKindReview,
				CommentID: comment.ID,
			},
		), nil
	}
	comment, err := api.CreateIssueComment(ctx, target.Number, body, installationID)
	if err != nil {
		return "", err
	}
	return encodeGitHubRemoteCommentRef(
		githubRemoteCommentRef{
			Kind:      githubRemoteCommentKindIssue,
			CommentID: comment.ID,
		},
	), nil
}

func executeGitHubUpdate(
	ctx context.Context,
	api githubAPI,
	request bridgepkg.DeliveryRequest,
	state deliveryState,
	installationID int64,
) (bridgepkg.DeliveryAck, deliveryState, error) {
	event := request.Event
	remoteID := gitHubRemoteIDFromRequest(request, state)
	ref, err := parseGitHubRemoteCommentRef(remoteID)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}
	updatedRemoteID, err := updateGitHubComment(ctx, api, ref, event.Content.Text, installationID)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}
	state.LastSeq = event.Seq
	state.RemoteMessageID = updatedRemoteID
	state.ReplaceRemoteMessageID = updatedRemoteID
	ack := bridgepkg.DeliveryAck{
		DeliveryID:             event.DeliveryID,
		Seq:                    event.Seq,
		RemoteMessageID:        updatedRemoteID,
		ReplaceRemoteMessageID: state.ReplaceRemoteMessageID,
	}
	return ack, state, ack.ValidateFor(event)
}

func updateGitHubComment(
	ctx context.Context,
	api githubAPI,
	ref githubRemoteCommentRef,
	body string,
	installationID int64,
) (string, error) {
	switch ref.Kind {
	case githubRemoteCommentKindReview:
		comment, err := api.UpdateReviewComment(ctx, ref.CommentID, body, installationID)
		if err != nil {
			return "", err
		}
		return encodeGitHubRemoteCommentRef(
			githubRemoteCommentRef{
				Kind:      githubRemoteCommentKindReview,
				CommentID: comment.ID,
			},
		), nil
	default:
		comment, err := api.UpdateIssueComment(ctx, ref.CommentID, body, installationID)
		if err != nil {
			return "", err
		}
		return encodeGitHubRemoteCommentRef(
			githubRemoteCommentRef{
				Kind:      githubRemoteCommentKindIssue,
				CommentID: comment.ID,
			},
		), nil
	}
}

func gitHubRemoteIDFromRequest(request bridgepkg.DeliveryRequest, state deliveryState) string {
	remoteID := firstNonEmpty(referenceRemoteMessageID(request.Event.Reference), state.RemoteMessageID)
	if remoteID == "" && request.Snapshot != nil {
		return strings.TrimSpace(request.Snapshot.RemoteMessageID)
	}
	return remoteID
}

func shouldPostGitHubMessage(
	event bridgepkg.DeliveryEvent,
	state deliveryState,
	request bridgepkg.DeliveryRequest,
) bool {
	switch normalizeDeliveryEventType(event.EventType) {
	case bridgepkg.DeliveryEventTypeStart:
		return true
	case bridgepkg.DeliveryEventTypeResume:
		if request.Snapshot == nil {
			return strings.TrimSpace(state.RemoteMessageID) == ""
		}
		return strings.TrimSpace(request.Snapshot.RemoteMessageID) == ""
	default:
		return strings.TrimSpace(state.RemoteMessageID) == ""
	}
}

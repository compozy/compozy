package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

type teamsEditDeliveryPlan struct {
	remoteID              string
	remoteRef             teamsRemoteMessageRef
	chunks                []string
	continuationReplyToID string
}

func executeTeamsEditDelivery(
	ctx context.Context,
	api teamsAPI,
	cfg resolvedInstanceConfig,
	event bridgepkg.DeliveryEvent,
	snapshot *bridgepkg.DeliverySnapshot,
	state deliveryState,
	referenceStateLookup teamsDeliveryStateLookup,
	userContextLookup func(string, string) (teamsUserContext, bool),
) (bridgepkg.DeliveryAck, deliveryState, error) {
	plan, err := prepareTeamsEditDelivery(
		cfg,
		event,
		snapshot,
		state,
		referenceStateLookup,
		userContextLookup,
	)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}

	replaceRemoteID := firstNonEmpty(state.RemoteMessageID, plan.remoteID)
	state.Chunks = bridgesdk.BeginDeliveryChunks(
		state.Chunks,
		event.Seq,
		bridgesdk.DeliveryChunkModeUpdate,
		len(plan.chunks),
		event.Content.Text,
		plan.remoteID,
		replaceRemoteID,
	)
	state, err = executeTeamsEditChunks(ctx, api, plan, state)
	if err != nil {
		return bridgepkg.DeliveryAck{}, state, err
	}

	lastRemoteID := state.Chunks.LastRemoteMessageID()
	replaceRemoteID = state.Chunks.ReplaceRemoteMessageID()
	state.Chunks = bridgesdk.DeliveryChunkCursor{}
	ack := newTeamsDeliveryAck(event, lastRemoteID, replaceRemoteID)
	state.LastSeq = event.Seq
	state.RemoteMessageID = lastRemoteID
	state.ReplaceRemoteMessageID = replaceRemoteID
	state.LastContent = plan.chunks[len(plan.chunks)-1]
	return ack, state, ack.ValidateFor(event)
}

func prepareTeamsEditDelivery(
	cfg resolvedInstanceConfig,
	event bridgepkg.DeliveryEvent,
	snapshot *bridgepkg.DeliverySnapshot,
	state deliveryState,
	referenceStateLookup teamsDeliveryStateLookup,
	userContextLookup func(string, string) (teamsUserContext, bool),
) (teamsEditDeliveryPlan, error) {
	remoteID := resolveTeamsReferencedRemoteMessageID(event.Reference, snapshot, state, referenceStateLookup)
	if remoteID == "" {
		return teamsEditDeliveryPlan{}, errors.New("teams: edit delivery requires a remote message id")
	}
	ref, err := decodeRemoteMessageID(remoteID)
	if err != nil {
		return teamsEditDeliveryPlan{}, err
	}
	chunks := teamsDeliveryChunks(event)
	replyToID, err := resolveTeamsEditContinuationReplyToID(cfg, event, ref, chunks, userContextLookup)
	if err != nil {
		return teamsEditDeliveryPlan{}, err
	}
	return teamsEditDeliveryPlan{
		remoteID:              remoteID,
		remoteRef:             ref,
		chunks:                chunks,
		continuationReplyToID: replyToID,
	}, nil
}

func resolveTeamsEditContinuationReplyToID(
	cfg resolvedInstanceConfig,
	event bridgepkg.DeliveryEvent,
	ref teamsRemoteMessageRef,
	chunks []string,
	userContextLookup func(string, string) (teamsUserContext, bool),
) (string, error) {
	if len(chunks) <= 1 {
		return "", nil
	}
	target, err := resolveTeamsDeliveryTarget(cfg, event, userContextLookup)
	if err != nil {
		return "", fmt.Errorf("teams: resolve continuation target: %w", err)
	}
	if target.ConversationID != ref.ConversationID || target.ServiceURL != ref.ServiceURL {
		return "", nil
	}
	return target.ReplyToID, nil
}

func executeTeamsEditChunks(
	ctx context.Context,
	api teamsAPI,
	plan teamsEditDeliveryPlan,
	state deliveryState,
) (deliveryState, error) {
	for index := state.Chunks.NextChunk(); index < len(plan.chunks); index++ {
		chunk := plan.chunks[index]
		if index == 0 {
			if err := updateTeamsEditPrimaryChunk(ctx, api, plan, state, chunk); err != nil {
				return state, err
			}
			state.Chunks = state.Chunks.Advance(plan.remoteID)
			continue
		}
		remoteID, err := sendTeamsEditContinuationChunk(ctx, api, plan, index, chunk)
		if err != nil {
			return state, err
		}
		state.Chunks = state.Chunks.Advance(remoteID)
	}
	return state, nil
}

func updateTeamsEditPrimaryChunk(
	ctx context.Context,
	api teamsAPI,
	plan teamsEditDeliveryPlan,
	state deliveryState,
	chunk string,
) error {
	if plan.remoteID == state.RemoteMessageID && chunk == state.LastContent {
		return nil
	}
	err := updateTeamsDeliveryActivity(
		ctx,
		api,
		plan.remoteRef.ServiceURL,
		plan.remoteRef.ConversationID,
		plan.remoteRef.ActivityID,
		teamsOutboundActivity{
			Type:       providerMessageKey,
			Text:       chunk,
			TextFormat: teamsTextFormatMarkdown,
		},
	)
	if err != nil {
		return fmt.Errorf("teams: update activity: %w", err)
	}
	return nil
}

func sendTeamsEditContinuationChunk(
	ctx context.Context,
	api teamsAPI,
	plan teamsEditDeliveryPlan,
	index int,
	chunk string,
) (string, error) {
	sent, err := sendTeamsDeliveryActivity(
		ctx,
		api,
		plan.remoteRef.ServiceURL,
		plan.remoteRef.ConversationID,
		plan.continuationReplyToID,
		teamsOutboundActivity{
			Type:       providerMessageKey,
			Text:       chunk,
			TextFormat: teamsTextFormatMarkdown,
		},
	)
	if err != nil {
		return "", fmt.Errorf("teams: send continuation %d: %w", index, err)
	}
	if sent == nil || strings.TrimSpace(sent.ID) == "" {
		return "", &bridgesdk.CommittedMutationError{
			Err: errors.New("teams: send activity response omitted id"),
		}
	}
	return encodeRemoteMessageID(teamsRemoteMessageRef{
		ConversationID: plan.remoteRef.ConversationID,
		ServiceURL:     plan.remoteRef.ServiceURL,
		ActivityID:     strings.TrimSpace(sent.ID),
	}), nil
}

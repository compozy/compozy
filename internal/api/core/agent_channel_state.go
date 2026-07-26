package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"

	"github.com/compozy/agh/internal/store"
)

func (h *BaseHandlers) agentChannelPayloads(
	ctx context.Context,
	caller agentidentity.Caller,
	service NetworkService,
) ([]contract.CoordinationChannelPayload, error) {
	infos, err := service.ListChannels(ctx, strings.TrimSpace(caller.Session.WorkspaceID))
	if err != nil {
		return nil, err
	}

	metadata := h.agentChannelMetadata(ctx, caller.Session.WorkspaceID)
	payloadByID := make(map[string]contract.CoordinationChannelPayload, len(infos)+len(metadata))
	for _, info := range infos {
		channel := strings.TrimSpace(info.Channel)
		if channel == "" {
			continue
		}
		entry, hasEntry := metadata[channel]
		if len(metadata) > 0 && !hasEntry &&
			channel != strings.TrimSpace(caller.Session.NetworkSpecSnapshot().ChannelID) {
			continue
		}
		payloadByID[channel] = coordinationChannelFromNetwork(channel, caller.Session.WorkspaceID, entry)
	}
	for channel, entry := range metadata {
		if _, ok := payloadByID[channel]; ok {
			continue
		}
		payloadByID[channel] = coordinationChannelFromNetwork(channel, caller.Session.WorkspaceID, entry)
	}
	if callerChannel := strings.TrimSpace(caller.Session.NetworkSpecSnapshot().ChannelID); callerChannel != "" {
		if _, ok := payloadByID[callerChannel]; !ok {
			payloadByID[callerChannel] = coordinationChannelFromNetwork(
				callerChannel,
				caller.Session.WorkspaceID,
				store.NetworkChannelEntry{},
			)
		}
	}

	channels := make([]contract.CoordinationChannelPayload, 0, len(payloadByID))
	for _, payload := range payloadByID {
		channels = append(channels, contract.NormalizeCoordinationChannelPayload(payload))
	}
	sortCoordinationChannels(channels)
	return channels, nil
}

func mergeCoordinationChannels(
	left []contract.CoordinationChannelPayload,
	right []contract.CoordinationChannelPayload,
) []contract.CoordinationChannelPayload {
	mergedByID := make(map[string]contract.CoordinationChannelPayload, len(left)+len(right))
	for _, channel := range left {
		normalized := contract.NormalizeCoordinationChannelPayload(channel)
		id := strings.TrimSpace(normalized.ID)
		if id != "" {
			mergedByID[id] = normalized
		}
	}
	for _, channel := range right {
		normalized := contract.NormalizeCoordinationChannelPayload(channel)
		id := strings.TrimSpace(normalized.ID)
		if id != "" {
			mergedByID[id] = normalized
		}
	}
	merged := make([]contract.CoordinationChannelPayload, 0, len(mergedByID))
	for _, channel := range mergedByID {
		merged = append(merged, channel)
	}
	sortCoordinationChannels(merged)
	return merged
}

func (h *BaseHandlers) agentChannelMetadata(
	ctx context.Context,
	workspaceID string,
) map[string]store.NetworkChannelEntry {
	if h == nil || h.NetworkStore == nil || strings.TrimSpace(workspaceID) == "" {
		return nil
	}
	entries, err := h.NetworkStore.ListNetworkChannels(ctx, store.NetworkChannelQuery{
		WorkspaceID: strings.TrimSpace(workspaceID),
	})
	if err != nil {
		if h.Logger != nil {
			h.Logger.Warn("api: skip agent channel metadata", "error", err)
		}
		return nil
	}
	metadata := make(map[string]store.NetworkChannelEntry, len(entries))
	for _, entry := range entries {
		channel := strings.TrimSpace(entry.Channel)
		if channel != "" {
			metadata[channel] = entry
		}
	}
	return metadata
}

func coordinationChannelFromNetwork(
	channel string,
	workspaceID string,
	entry store.NetworkChannelEntry,
) contract.CoordinationChannelPayload {
	channel = strings.TrimSpace(channel)
	workspaceID = firstNonEmpty(strings.TrimSpace(entry.WorkspaceID), strings.TrimSpace(workspaceID))
	payload := contract.CoordinationChannelPayload{
		ID:          channel,
		DisplayName: channel,
		Purpose:     firstNonEmpty(strings.TrimSpace(entry.Purpose), "network_channel"),
		WorkspaceID: workspaceID,
	}
	if !entry.UpdatedAt.IsZero() {
		updatedAt := entry.UpdatedAt.UTC()
		payload.LastActivityAt = &updatedAt
	}
	return payload
}

func agentChannelInbox(
	ctx context.Context,
	service NetworkService,
	sessionID string,
	channel string,
	wait bool,
) ([]network.Envelope, error) {
	if !wait {
		envelopes, err := service.Inbox(ctx, strings.TrimSpace(sessionID))
		if err != nil {
			return nil, err
		}
		return envelopes, nil
	}
	envelopes, err := service.WaitInbox(
		ctx,
		strings.TrimSpace(sessionID),
		strings.TrimSpace(channel),
	)
	if err != nil {
		return nil, err
	}
	return envelopes, nil
}

func validateAgentChannelRequest(
	body json.RawMessage,
	metadata contract.CoordinationMessageMetadataPayload,
	fullPayload any,
) error {
	if len(bytes.TrimSpace(body)) == 0 {
		return NewNetworkValidationError(errors.New("body is required"))
	}
	if !json.Valid(body) {
		return NewNetworkValidationError(errors.New("body must be valid JSON"))
	}
	if err := metadata.Validate(); err != nil {
		return NewNetworkValidationError(err)
	}
	if err := contract.ValidateNoRawClaimTokenField(fullPayload); err != nil {
		return NewNetworkValidationError(err)
	}
	return nil
}

func coordinationExt(
	metadata contract.CoordinationMessageMetadataPayload,
) (map[string]json.RawMessage, error) {
	raw, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("api: marshal coordination metadata: %w", err)
	}
	return map[string]json.RawMessage{agentCoordinationExtKey: raw}, nil
}

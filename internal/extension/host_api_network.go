package extensionpkg

import (
	"context"

	"encoding/json"
	"errors"

	"strings"

	apicontract "github.com/compozy/agh/internal/api/contract"
	extensioncontract "github.com/compozy/agh/internal/extension/contract"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/store"
)

func registerHostAPINetworkMethodHandlers(
	handler *HostAPIHandler,
	handlers map[string]hostAPIMethodFunc,
) {
	handlers[string(extensioncontract.HostAPIMethodNetworkStatus)] = handler.handleNetworkStatus
	handlers[string(extensioncontract.HostAPIMethodNetworkUsage)] = handler.handleNetworkUsage
	handlers[string(extensioncontract.HostAPIMethodNetworkChannels)] = handler.handleNetworkChannels
	handlers[string(extensioncontract.HostAPIMethodNetworkPeers)] = handler.handleNetworkPeers
	handlers[string(extensioncontract.HostAPIMethodNetworkThreads)] = handler.handleNetworkThreads
	handlers[string(extensioncontract.HostAPIMethodNetworkThreadGet)] = handler.handleNetworkThreadGet
	handlers[string(extensioncontract.HostAPIMethodNetworkThreadMessages)] = handler.handleNetworkThreadMessages
	handlers[string(extensioncontract.HostAPIMethodNetworkDirects)] = handler.handleNetworkDirects
	handlers[string(extensioncontract.HostAPIMethodNetworkDirectResolve)] = handler.handleNetworkDirectResolve
	handlers[string(extensioncontract.HostAPIMethodNetworkDirectMessages)] = handler.handleNetworkDirectMessages
	handlers[string(extensioncontract.HostAPIMethodNetworkWorkGet)] = handler.handleNetworkWorkGet
	handlers[string(extensioncontract.HostAPIMethodNetworkSend)] = handler.handleNetworkSend
}

func (h *HostAPIHandler) handleNetworkChannels(ctx context.Context, raw json.RawMessage) (any, error) {
	var params extensioncontract.NetworkChannelsParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	service, err := h.requireHostAPINetworkService()
	if err != nil {
		return nil, err
	}
	workspaceID, err := h.hostAPINetworkWorkspaceID(ctx, params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	channels, err := service.ListChannels(ctx, workspaceID)
	if err != nil {
		return nil, mapHostAPINetworkRPCError(err)
	}
	return hostAPINetworkChannelPayloads(channels), nil
}

func (h *HostAPIHandler) handleNetworkPeers(ctx context.Context, raw json.RawMessage) (any, error) {
	var params extensioncontract.NetworkPeersParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	service, err := h.requireHostAPINetworkService()
	if err != nil {
		return nil, err
	}
	channel := strings.TrimSpace(params.Channel)
	if channel != "" {
		if err := network.ValidateChannel(channel); err != nil {
			return nil, invalidParamsRPCError(err)
		}
	}
	workspaceID, err := h.hostAPINetworkWorkspaceID(ctx, params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	peers, err := service.ListPeers(ctx, workspaceID, channel)
	if err != nil {
		return nil, mapHostAPINetworkRPCError(err)
	}
	return hostAPINetworkPeerPayloads(peers), nil
}

func (h *HostAPIHandler) handleNetworkThreadGet(ctx context.Context, raw json.RawMessage) (any, error) {
	var params extensioncontract.NetworkThreadTargetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	networkStore, err := h.requireHostAPINetworkStore()
	if err != nil {
		return nil, err
	}
	channel, err := hostAPINetworkChannel(params.Channel)
	if err != nil {
		return nil, err
	}
	threadID := strings.TrimSpace(params.ThreadID)
	if err := network.ValidateConversationID(threadID, "thread_id"); err != nil {
		return nil, invalidParamsRPCError(err)
	}
	workspaceID, err := h.hostAPINetworkWorkspaceID(ctx, params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	thread, err := networkStore.GetThread(
		ctx,
		store.NetworkChannelRef{WorkspaceID: workspaceID, Channel: channel},
		threadID,
	)
	if err != nil {
		return nil, mapHostAPINetworkRPCError(err)
	}
	return hostAPINetworkThreadSummaryPayload(thread), nil
}

func (h *HostAPIHandler) handleNetworkThreadMessages(ctx context.Context, raw json.RawMessage) (any, error) {
	var params extensioncontract.NetworkThreadMessagesParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	ref := store.NetworkConversationRef{
		WorkspaceID: strings.TrimSpace(params.WorkspaceID),
		Channel:     strings.TrimSpace(params.Channel),
		Surface:     store.NetworkSurfaceThread,
		ThreadID:    strings.TrimSpace(params.ThreadID),
	}
	workspaceID, err := h.hostAPINetworkWorkspaceID(ctx, params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	ref.WorkspaceID = workspaceID
	query, err := hostAPINetworkConversationMessageQuery(
		params.Limit,
		params.Before,
		params.After,
		params.Kind,
		params.WorkID,
	)
	if err != nil {
		return nil, err
	}
	payload, page, err := h.hostAPINetworkConversationMessages(ctx, ref, query)
	if err != nil {
		return nil, err
	}
	return apicontract.NetworkThreadMessagesResponse{Messages: payload, Page: page}, nil
}

func (h *HostAPIHandler) handleNetworkDirectResolve(ctx context.Context, raw json.RawMessage) (any, error) {
	var params extensioncontract.NetworkDirectResolveParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	service, err := h.requireHostAPINetworkService()
	if err != nil {
		return nil, err
	}
	networkStore, err := h.requireHostAPINetworkStore()
	if err != nil {
		return nil, err
	}
	channel, err := hostAPINetworkChannel(params.Channel)
	if err != nil {
		return nil, err
	}
	sessionID := strings.TrimSpace(params.SessionID)
	if sessionID == "" {
		return nil, invalidParamsRPCError(errors.New("session_id is required"))
	}
	peerID := strings.TrimSpace(params.PeerID)
	if err := network.ValidatePeerID(peerID); err != nil {
		return nil, invalidParamsRPCError(err)
	}
	workspaceID, err := h.hostAPINetworkWorkspaceID(ctx, params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if err := h.requireHostAPINetworkParticipant(ctx, workspaceID, sessionID, channel); err != nil {
		return nil, err
	}
	local, remote, err := h.resolveHostAPIDirectPeers(ctx, service, workspaceID, channel, sessionID, peerID)
	if err != nil {
		return nil, mapHostAPINetworkRPCError(err)
	}
	directID, sessionA, sessionB, err := store.NetworkDirectRoomIdentity(
		workspaceID,
		channel,
		*local.SessionID,
		*remote.SessionID,
	)
	if err != nil {
		return nil, mapHostAPINetworkRPCError(err)
	}
	now := h.now()
	direct, err := networkStore.ResolveDirectRoom(ctx, store.NetworkDirectRoomEntry{
		WorkspaceID:    workspaceID,
		Channel:        channel,
		DirectID:       directID,
		SessionA:       sessionA,
		SessionB:       sessionB,
		OpenedAt:       now,
		LastActivityAt: now,
	})
	if err != nil {
		return nil, mapHostAPINetworkRPCError(err)
	}
	return hostAPINetworkDirectRoomPayload(direct), nil
}

func (h *HostAPIHandler) handleNetworkDirectMessages(ctx context.Context, raw json.RawMessage) (any, error) {
	var params extensioncontract.NetworkDirectMessagesParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	ref := store.NetworkConversationRef{
		WorkspaceID: strings.TrimSpace(params.WorkspaceID),
		Channel:     strings.TrimSpace(params.Channel),
		Surface:     store.NetworkSurfaceDirect,
		DirectID:    strings.TrimSpace(params.DirectID),
	}
	workspaceID, err := h.hostAPINetworkWorkspaceID(ctx, params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	ref.WorkspaceID = workspaceID
	query, err := hostAPINetworkConversationMessageQuery(
		params.Limit,
		params.Before,
		params.After,
		params.Kind,
		params.WorkID,
	)
	if err != nil {
		return nil, err
	}
	payload, page, err := h.hostAPINetworkConversationMessages(ctx, ref, query)
	if err != nil {
		return nil, err
	}
	return apicontract.NetworkDirectRoomMessagesResponse{Messages: payload, Page: page}, nil
}

func (h *HostAPIHandler) handleNetworkWorkGet(ctx context.Context, raw json.RawMessage) (any, error) {
	var params extensioncontract.NetworkWorkGetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	networkStore, err := h.requireHostAPINetworkStore()
	if err != nil {
		return nil, err
	}
	workID := strings.TrimSpace(params.WorkID)
	if err := network.ValidateWorkID(workID); err != nil {
		return nil, invalidParamsRPCError(err)
	}
	workspaceID, err := h.hostAPINetworkWorkspaceID(ctx, params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	work, err := networkStore.GetWork(ctx, workspaceID, workID)
	if err != nil {
		return nil, mapHostAPINetworkRPCError(err)
	}
	return hostAPINetworkWorkPayload(work), nil
}

func (h *HostAPIHandler) handleNetworkSend(ctx context.Context, raw json.RawMessage) (any, error) {
	var params extensioncontract.NetworkSendParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	service, err := h.requireHostAPINetworkService()
	if err != nil {
		return nil, err
	}
	workspaceID, err := h.hostAPINetworkWorkspaceID(ctx, params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	params.WorkspaceID = workspaceID
	sendReq, err := hostAPINetworkSendRequestFromPayload(params)
	if err != nil {
		return nil, mapHostAPINetworkRPCError(err)
	}
	if err := h.requireHostAPINetworkParticipant(
		ctx,
		workspaceID,
		params.SessionID,
		params.Channel,
	); err != nil {
		return nil, err
	}
	id, err := service.Send(ctx, sendReq)
	if err != nil {
		return nil, mapHostAPINetworkRPCError(err)
	}
	return hostAPINetworkSendPayloadFromRequest(id, params), nil
}

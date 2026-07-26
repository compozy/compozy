package extensionpkg

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	apicontract "github.com/compozy/agh/internal/api/contract"
	extensioncontract "github.com/compozy/agh/internal/extension/contract"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/store"
)

func hostAPINetworkThreadQuery(
	params extensioncontract.NetworkThreadsParams,
	sessionID string,
) (store.NetworkThreadQuery, error) {
	query := store.NetworkThreadQuery{
		Search:    strings.TrimSpace(params.Query),
		SessionID: strings.TrimSpace(sessionID),
		Sort:      strings.TrimSpace(params.Sort),
		HasWork:   params.HasWork,
		Limit:     params.Limit,
		After:     strings.TrimSpace(params.After),
	}
	if err := query.Validate(); err != nil {
		return store.NetworkThreadQuery{}, invalidParamsRPCError(err)
	}
	return query, nil
}

func hostAPINetworkDirectRoomQuery(
	params extensioncontract.NetworkDirectsParams,
	sessionID string,
) (store.NetworkDirectRoomQuery, error) {
	query := store.NetworkDirectRoomQuery{
		Search:    strings.TrimSpace(params.Query),
		SessionID: strings.TrimSpace(sessionID),
		Sort:      strings.TrimSpace(params.Sort),
		HasWork:   params.HasWork,
		Limit:     params.Limit,
		After:     strings.TrimSpace(params.After),
	}
	if err := query.Validate(); err != nil {
		return store.NetworkDirectRoomQuery{}, invalidParamsRPCError(err)
	}
	return query, nil
}

func (h *HostAPIHandler) handleNetworkThreads(ctx context.Context, raw json.RawMessage) (any, error) {
	var params extensioncontract.NetworkThreadsParams
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
	workspaceID, err := h.hostAPINetworkWorkspaceID(ctx, params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	sessionID, err := h.resolveHostAPINetworkPeerSessionID(
		ctx,
		workspaceID,
		channel,
		params.PeerID,
	)
	if err != nil {
		return nil, err
	}
	query, err := hostAPINetworkThreadQuery(params, sessionID)
	if err != nil {
		return nil, err
	}
	page, err := networkStore.ListThreads(
		ctx,
		store.NetworkChannelRef{WorkspaceID: workspaceID, Channel: channel},
		query,
	)
	if err != nil {
		return nil, mapHostAPINetworkRPCError(err)
	}
	return apicontract.NetworkThreadsResponse{
		Threads: hostAPINetworkThreadSummaryPayloads(page.Threads),
		Page: apicontract.CountedCursorPagePayload{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
			Total:      page.Total,
			Limit:      page.Limit,
		},
	}, nil
}

func (h *HostAPIHandler) handleNetworkDirects(ctx context.Context, raw json.RawMessage) (any, error) {
	var params extensioncontract.NetworkDirectsParams
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
	workspaceID, err := h.hostAPINetworkWorkspaceID(ctx, params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	sessionID, err := h.resolveHostAPINetworkPeerSessionID(
		ctx,
		workspaceID,
		channel,
		params.PeerID,
	)
	if err != nil {
		return nil, err
	}
	query, err := hostAPINetworkDirectRoomQuery(params, sessionID)
	if err != nil {
		return nil, err
	}
	page, err := networkStore.ListDirectRooms(
		ctx,
		store.NetworkChannelRef{WorkspaceID: workspaceID, Channel: channel},
		query,
	)
	if err != nil {
		return nil, mapHostAPINetworkRPCError(err)
	}
	return apicontract.NetworkDirectRoomsResponse{
		Directs: hostAPINetworkDirectRoomPayloads(page.Directs),
		Page: apicontract.CountedCursorPagePayload{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
			Total:      page.Total,
			Limit:      page.Limit,
		},
	}, nil
}

func (h *HostAPIHandler) resolveHostAPINetworkPeerSessionID(
	ctx context.Context,
	workspaceID string,
	channel string,
	peerID string,
) (string, error) {
	wanted := strings.TrimSpace(peerID)
	if wanted == "" {
		return "", nil
	}
	if err := network.ValidatePeerID(wanted); err != nil {
		return "", invalidParamsRPCError(err)
	}
	service, err := h.requireHostAPINetworkService()
	if err != nil {
		return "", err
	}
	peers, err := service.ListPeers(ctx, workspaceID, channel)
	if err != nil {
		return "", mapHostAPINetworkRPCError(err)
	}
	peer, found := hostAPINetworkFindPeer(peers, wanted)
	if !found || peer.SessionID == nil || strings.TrimSpace(*peer.SessionID) == "" {
		return "", mapHostAPINetworkRPCError(fmt.Errorf(
			"%w: peer_id=%q channel=%q has no participating session",
			network.ErrTargetPeerNotFound,
			wanted,
			channel,
		))
	}
	sessionID := strings.TrimSpace(*peer.SessionID)
	if err := h.requireHostAPINetworkParticipant(ctx, workspaceID, sessionID, channel); err != nil {
		return "", err
	}
	return sessionID, nil
}

func hostAPINetworkConversationMessageQuery(
	limit int,
	before string,
	after string,
	kind string,
	workID string,
) (store.NetworkConversationMessageQuery, error) {
	query := store.NetworkConversationMessageQuery{
		BeforeMessageID: strings.TrimSpace(before),
		AfterMessageID:  strings.TrimSpace(after),
		Kind:            strings.TrimSpace(kind),
		WorkID:          strings.TrimSpace(workID),
		Limit:           limit,
	}
	if err := query.Validate(); err != nil {
		return store.NetworkConversationMessageQuery{}, invalidParamsRPCError(err)
	}
	query.Limit = store.NormalizeNetworkMessageLimit(query.Limit) + 1
	return query, nil
}

func (h *HostAPIHandler) hostAPINetworkConversationMessages(
	ctx context.Context,
	ref store.NetworkConversationRef,
	query store.NetworkConversationMessageQuery,
) ([]apicontract.NetworkConversationMessagePayload, apicontract.CursorPagePayload, error) {
	networkStore, err := h.requireHostAPINetworkStore()
	if err != nil {
		return nil, apicontract.CursorPagePayload{}, err
	}
	ref.WorkspaceID = strings.TrimSpace(ref.WorkspaceID)
	ref.Channel = strings.TrimSpace(ref.Channel)
	ref.ThreadID = strings.TrimSpace(ref.ThreadID)
	ref.DirectID = strings.TrimSpace(ref.DirectID)
	if err := ref.Validate(); err != nil {
		return nil, apicontract.CursorPagePayload{}, invalidParamsRPCError(err)
	}
	messages, err := networkStore.ListConversationMessages(ctx, ref, query)
	if err != nil {
		return nil, apicontract.CursorPagePayload{}, mapHostAPINetworkRPCError(err)
	}
	payload := hostAPINetworkConversationMessagePayloads(messages)
	limit := query.Limit - 1
	payload, page := trimHostAPINetworkMessagePage(
		payload,
		query.AfterMessageID,
		limit,
	)
	return payload, page, nil
}

func trimHostAPINetworkMessagePage(
	messages []apicontract.NetworkConversationMessagePayload,
	after string,
	limit int,
) ([]apicontract.NetworkConversationMessagePayload, apicontract.CursorPagePayload) {
	page := apicontract.CursorPagePayload{Limit: limit}
	if len(messages) <= limit {
		return messages, page
	}
	page.HasMore = true
	if strings.TrimSpace(after) != "" {
		messages = messages[:limit]
		page.NextCursor = strings.TrimSpace(messages[len(messages)-1].MessageID)
		return messages, page
	}
	messages = messages[len(messages)-limit:]
	page.NextCursor = strings.TrimSpace(messages[0].MessageID)
	return messages, page
}

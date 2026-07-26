package e2e

import (
	"context"

	"errors"
	"fmt"

	"net/http"
	"net/url"
	"os"

	"strings"

	aghcontract "github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/store"
)

// CreateNetworkChannel creates one network channel through the public operator surface.
func (h *RuntimeHarness) CreateNetworkChannel(
	ctx context.Context,
	request aghcontract.CreateNetworkChannelRequest,
) (aghcontract.NetworkChannelDetailPayload, error) {
	var response aghcontract.CreateNetworkChannelResponse
	path, err := h.networkScopedAPIPath(request.WorkspaceID, "/channels")
	if err != nil {
		return aghcontract.NetworkChannelDetailPayload{}, err
	}
	if err := h.UDSJSON(ctx, http.MethodPost, path, request, &response); err != nil {
		return aghcontract.NetworkChannelDetailPayload{}, err
	}
	return response.Channel, nil
}

// NetworkStatus fetches the current network runtime projection.
func (h *RuntimeHarness) NetworkStatus(
	ctx context.Context,
) (aghcontract.NetworkStatusPayload, error) {
	var response aghcontract.NetworkStatusResponse
	if err := h.UDSJSON(ctx, http.MethodGet, "/api/network/status", nil, &response); err != nil {
		return aghcontract.NetworkStatusPayload{}, err
	}
	return response.Network, nil
}

// NetworkPeers fetches the current visible peers, optionally filtered by channel.
func (h *RuntimeHarness) NetworkPeers(
	ctx context.Context,
	channel string,
) ([]aghcontract.NetworkPeerPayload, error) {
	values := url.Values{}
	if trimmed := strings.TrimSpace(channel); trimmed != "" {
		values.Set("channel", trimmed)
	}
	path, err := h.networkScopedAPIPath("", "/peers"+encodeQuery(values))
	if err != nil {
		return nil, err
	}

	var response aghcontract.NetworkPeersResponse
	if err := h.UDSJSON(
		ctx,
		http.MethodGet,
		path,
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	return response.Peers, nil
}

// NetworkChannels fetches the current network channel projection.
func (h *RuntimeHarness) NetworkChannels(
	ctx context.Context,
) ([]aghcontract.NetworkChannelPayload, error) {
	var response aghcontract.NetworkChannelsResponse
	path, err := h.networkScopedAPIPath("", "/channels")
	if err != nil {
		return nil, err
	}
	if err := h.UDSJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	return response.Channels, nil
}

// NetworkChannel fetches one selected network channel detail payload.
func (h *RuntimeHarness) NetworkChannel(
	ctx context.Context,
	channel string,
) (aghcontract.NetworkChannelDetailPayload, error) {
	var response aghcontract.NetworkChannelResponse
	path, err := h.networkScopedAPIPath("", "/channels/"+url.PathEscape(channel))
	if err != nil {
		return aghcontract.NetworkChannelDetailPayload{}, err
	}
	if err := h.UDSJSON(
		ctx,
		http.MethodGet,
		path,
		nil,
		&response,
	); err != nil {
		return aghcontract.NetworkChannelDetailPayload{}, err
	}
	return response.Channel, nil
}

// NetworkThreads fetches public-thread summaries for one channel.
func (h *RuntimeHarness) NetworkThreads(
	ctx context.Context,
	channel string,
) ([]aghcontract.NetworkThreadSummaryPayload, error) {
	var response aghcontract.NetworkThreadsResponse
	path, err := h.networkScopedAPIPath("", "/channels/"+url.PathEscape(channel)+"/threads")
	if err != nil {
		return nil, err
	}
	if err := h.UDSJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	return response.Threads, nil
}

// NetworkThread fetches one public-thread summary.
func (h *RuntimeHarness) NetworkThread(
	ctx context.Context,
	channel string,
	threadID string,
) (aghcontract.NetworkThreadSummaryPayload, error) {
	var response aghcontract.NetworkThreadResponse
	path, err := h.networkScopedAPIPath("", "/channels/"+url.PathEscape(channel)+
		"/threads/"+url.PathEscape(threadID))
	if err != nil {
		return aghcontract.NetworkThreadSummaryPayload{}, err
	}
	if err := h.UDSJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return aghcontract.NetworkThreadSummaryPayload{}, err
	}
	return response.Thread, nil
}

// NetworkThreadMessages fetches messages isolated to one public thread.
func (h *RuntimeHarness) NetworkThreadMessages(
	ctx context.Context,
	channel string,
	threadID string,
) ([]aghcontract.NetworkConversationMessagePayload, error) {
	var response aghcontract.NetworkThreadMessagesResponse
	path, err := h.networkScopedAPIPath("", "/channels/"+url.PathEscape(channel)+
		"/threads/"+url.PathEscape(threadID)+"/messages")
	if err != nil {
		return nil, err
	}
	if err := h.UDSJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	return response.Messages, nil
}

// NetworkDirectRooms fetches direct-room summaries for one channel.
func (h *RuntimeHarness) NetworkDirectRooms(
	ctx context.Context,
	channel string,
) ([]aghcontract.NetworkDirectRoomPayload, error) {
	var response aghcontract.NetworkDirectRoomsResponse
	path, err := h.networkScopedAPIPath("", "/channels/"+url.PathEscape(channel)+"/directs")
	if err != nil {
		return nil, err
	}
	if err := h.UDSJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	return response.Directs, nil
}

// NetworkDirectRoom fetches one direct-room summary.
func (h *RuntimeHarness) NetworkDirectRoom(
	ctx context.Context,
	channel string,
	directID string,
) (aghcontract.NetworkDirectRoomPayload, error) {
	var response aghcontract.NetworkDirectRoomResponse
	path, err := h.networkScopedAPIPath("", "/channels/"+url.PathEscape(channel)+
		"/directs/"+url.PathEscape(directID))
	if err != nil {
		return aghcontract.NetworkDirectRoomPayload{}, err
	}
	if err := h.UDSJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return aghcontract.NetworkDirectRoomPayload{}, err
	}
	return response.Direct, nil
}

// NetworkDirectRoomMessages fetches messages isolated to one direct room.
func (h *RuntimeHarness) NetworkDirectRoomMessages(
	ctx context.Context,
	channel string,
	directID string,
) ([]aghcontract.NetworkConversationMessagePayload, error) {
	var response aghcontract.NetworkDirectRoomMessagesResponse
	path, err := h.networkScopedAPIPath("", "/channels/"+url.PathEscape(channel)+
		"/directs/"+url.PathEscape(directID)+"/messages")
	if err != nil {
		return nil, err
	}
	if err := h.UDSJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	return response.Messages, nil
}

// NetworkDirectResolve resolves the deterministic direct room for a local session and peer.
func (h *RuntimeHarness) NetworkDirectResolve(
	ctx context.Context,
	channel string,
	request aghcontract.NetworkDirectResolveRequest,
) (aghcontract.NetworkDirectRoomPayload, error) {
	var response aghcontract.NetworkDirectRoomResponse
	path, err := h.networkScopedAPIPath("", "/channels/"+url.PathEscape(channel)+"/directs/resolve")
	if err != nil {
		return aghcontract.NetworkDirectRoomPayload{}, err
	}
	if err := h.UDSJSON(ctx, http.MethodPost, path, request, &response); err != nil {
		return aghcontract.NetworkDirectRoomPayload{}, err
	}
	return response.Direct, nil
}

// NetworkWork fetches one lifecycle-bearing work row.
func (h *RuntimeHarness) NetworkWork(
	ctx context.Context,
	workID string,
) (aghcontract.NetworkWorkPayload, error) {
	var response aghcontract.NetworkWorkResponse
	path, err := h.networkScopedAPIPath("", "/work/"+url.PathEscape(workID))
	if err != nil {
		return aghcontract.NetworkWorkPayload{}, err
	}
	if err := h.UDSJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return aghcontract.NetworkWorkPayload{}, err
	}
	return response.Work, nil
}

// NetworkChannelMessages fetches public-thread messages for one channel.
func (h *RuntimeHarness) NetworkChannelMessages(
	ctx context.Context,
	channel string,
) ([]aghcontract.NetworkConversationMessagePayload, error) {
	threads, err := h.NetworkThreads(ctx, channel)
	if err != nil {
		return nil, err
	}
	messages := make([]aghcontract.NetworkConversationMessagePayload, 0)
	for _, thread := range threads {
		threadMessages, err := h.NetworkThreadMessages(ctx, channel, thread.ThreadID)
		if err != nil {
			return nil, err
		}
		messages = append(messages, threadMessages...)
	}
	return messages, nil
}

// NetworkInbox fetches the queued inbox projection for one local session.
func (h *RuntimeHarness) NetworkInbox(
	ctx context.Context,
	sessionID string,
) ([]aghcontract.NetworkEnvelopePayload, error) {
	values := url.Values{}
	if trimmed := strings.TrimSpace(sessionID); trimmed != "" {
		values.Set("session_id", trimmed)
	}
	path, err := h.networkScopedAPIPath("", "/inbox"+encodeQuery(values))
	if err != nil {
		return nil, err
	}

	var response aghcontract.NetworkInboxResponse
	if err := h.UDSJSON(
		ctx,
		http.MethodGet,
		path,
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	return response.Messages, nil
}

// NetworkSend sends one envelope through the public network operator surface.
func (h *RuntimeHarness) NetworkSend(
	ctx context.Context,
	request aghcontract.NetworkSendRequest,
) (aghcontract.NetworkSendPayload, error) {
	var response aghcontract.NetworkSendResponse
	path, err := h.networkScopedAPIPath(request.WorkspaceID, "/send")
	if err != nil {
		return aghcontract.NetworkSendPayload{}, err
	}
	if strings.TrimSpace(request.WorkspaceID) == "" {
		request.WorkspaceID = strings.TrimSpace(h.WorkspaceID)
	}
	if err := h.UDSJSON(ctx, http.MethodPost, path, request, &response); err != nil {
		return aghcontract.NetworkSendPayload{}, err
	}
	return response.Message, nil
}

// NetworkAuditSnapshot decodes the current daemon-owned network audit file into a stable snapshot.
func (h *RuntimeHarness) NetworkAuditSnapshot() ([]store.NetworkAuditEntry, error) {
	if _, err := os.Stat(h.HomePaths.NetworkAuditFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat network audit file %q: %w", h.HomePaths.NetworkAuditFile, err)
	}
	return readNetworkAuditSnapshot(h.HomePaths.NetworkAuditFile)
}

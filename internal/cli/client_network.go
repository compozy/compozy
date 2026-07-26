package cli

import (
	"context"

	"net/http"
	"net/url"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

func (c *unixSocketClient) NetworkStatus(ctx context.Context) (NetworkStatusRecord, error) {
	var response struct {
		Network NetworkStatusRecord `json:"network"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/network/status", nil, nil, &response); err != nil {
		return NetworkStatusRecord{}, err
	}
	return response.Network, nil
}

func (c *unixSocketClient) NetworkPeers(ctx context.Context, query NetworkPeersQuery) ([]NetworkPeerRecord, error) {
	var response struct {
		Peers []NetworkPeerRecord `json:"peers"`
	}
	path, err := networkBasePath(query.WorkspaceRef)
	if err != nil {
		return nil, err
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		path+"/peers",
		networkPeersValues(query),
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	return response.Peers, nil
}

func (c *unixSocketClient) NetworkChannels(ctx context.Context, workspaceRef string) ([]NetworkChannelRecord, error) {
	var response struct {
		Channels []NetworkChannelRecord `json:"channels"`
	}
	path, err := networkBasePath(workspaceRef)
	if err != nil {
		return nil, err
	}
	if err := c.doJSON(ctx, http.MethodGet, path+"/channels", nil, nil, &response); err != nil {
		return nil, err
	}
	return response.Channels, nil
}

func (c *unixSocketClient) CreateNetworkChannel(
	ctx context.Context,
	workspaceRef string,
	request CreateNetworkChannelRequest,
) (NetworkChannelDetailRecord, error) {
	var response contract.CreateNetworkChannelResponse
	path, err := networkBasePath(workspaceRef)
	if err != nil {
		return NetworkChannelDetailRecord{}, err
	}
	body := struct {
		Channel           string   `json:"channel"`
		Purpose           string   `json:"purpose"`
		FanoutPolicy      string   `json:"fanout_policy,omitempty"`
		CoordinatorPeerID string   `json:"coordinator_peer_id,omitempty"`
		AgentNames        []string `json:"agent_names"`
	}{
		Channel:           request.Channel,
		Purpose:           request.Purpose,
		FanoutPolicy:      request.FanoutPolicy,
		CoordinatorPeerID: request.CoordinatorPeerID,
		AgentNames:        request.AgentNames,
	}
	if err := c.doJSON(ctx, http.MethodPost, path+"/channels", nil, body, &response); err != nil {
		return NetworkChannelDetailRecord{}, err
	}
	return response.Channel, nil
}

func (c *unixSocketClient) UpdateNetworkChannel(
	ctx context.Context,
	workspaceRef string,
	channel string,
	request UpdateNetworkChannelRequest,
) (NetworkChannelDetailRecord, error) {
	channel, err := requireNetworkPathValue("channel", channel)
	if err != nil {
		return NetworkChannelDetailRecord{}, err
	}
	var response contract.NetworkChannelResponse
	path, err := networkChannelPath(workspaceRef, channel)
	if err != nil {
		return NetworkChannelDetailRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodPatch, path, nil, request, &response); err != nil {
		return NetworkChannelDetailRecord{}, err
	}
	return response.Channel, nil
}

func (c *unixSocketClient) ListNetworkSubscriptions(
	ctx context.Context,
	query NetworkSubscriptionsQuery,
) ([]NetworkSubscriptionRecord, error) {
	channel, err := requireNetworkPathValue("channel", query.Channel)
	if err != nil {
		return nil, err
	}
	var response contract.NetworkSubscriptionsResponse
	path, err := networkChannelPath(query.WorkspaceRef, channel)
	if err != nil {
		return nil, err
	}
	path += "/subscriptions"
	if err := c.doJSON(ctx, http.MethodGet, path, networkSubscriptionsValues(query), nil, &response); err != nil {
		return nil, err
	}
	return response.Subscriptions, nil
}

func (c *unixSocketClient) SetNetworkSubscription(
	ctx context.Context,
	workspaceRef string,
	channel string,
	request NetworkSubscriptionRequest,
) (NetworkSubscriptionRecord, error) {
	channel, err := requireNetworkPathValue("channel", channel)
	if err != nil {
		return NetworkSubscriptionRecord{}, err
	}
	var response contract.NetworkSubscriptionResponse
	path, err := networkChannelPath(workspaceRef, channel)
	if err != nil {
		return NetworkSubscriptionRecord{}, err
	}
	path += "/subscriptions"
	if err := c.doJSON(ctx, http.MethodPut, path, nil, request, &response); err != nil {
		return NetworkSubscriptionRecord{}, err
	}
	return response.Subscription, nil
}

func (c *unixSocketClient) DeleteNetworkSubscription(
	ctx context.Context,
	workspaceRef string,
	channel string,
	sessionID string,
	threadID string,
) error {
	channel, err := requireNetworkPathValue("channel", channel)
	if err != nil {
		return err
	}
	sessionID, err = requireNetworkPathValue("session_id", sessionID)
	if err != nil {
		return err
	}
	path, err := networkChannelPath(workspaceRef, channel)
	if err != nil {
		return err
	}
	values := url.Values{}
	if trimmed := strings.TrimSpace(threadID); trimmed != "" {
		values.Set("thread_id", trimmed)
	}
	path += "/subscriptions/" + url.PathEscape(sessionID)
	return c.doJSON(ctx, http.MethodDelete, path, values, nil, nil)
}

func (c *unixSocketClient) NetworkThreads(
	ctx context.Context,
	query NetworkThreadsQuery,
) (contract.NetworkThreadsResponse, error) {
	channel, err := requireNetworkPathValue("channel", query.Channel)
	if err != nil {
		return contract.NetworkThreadsResponse{}, err
	}
	var response contract.NetworkThreadsResponse
	path, err := networkChannelPath(query.WorkspaceRef, channel)
	if err != nil {
		return contract.NetworkThreadsResponse{}, err
	}
	path += "/threads"
	if err := c.doJSON(ctx, http.MethodGet, path, networkThreadsValues(query), nil, &response); err != nil {
		return contract.NetworkThreadsResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) NetworkThread(
	ctx context.Context,
	workspaceRef string,
	channel string,
	threadID string,
) (NetworkThreadRecord, error) {
	path, err := networkThreadPath(workspaceRef, channel, threadID)
	if err != nil {
		return NetworkThreadRecord{}, err
	}
	var response struct {
		Thread NetworkThreadRecord `json:"thread"`
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return NetworkThreadRecord{}, err
	}
	return response.Thread, nil
}

func (c *unixSocketClient) NetworkThreadMessages(
	ctx context.Context,
	query NetworkConversationMessagesQuery,
) (contract.NetworkThreadMessagesResponse, error) {
	path, err := networkThreadMessagesPath(query.WorkspaceRef, query.Channel, query.ThreadID)
	if err != nil {
		return contract.NetworkThreadMessagesResponse{}, err
	}
	var response contract.NetworkThreadMessagesResponse
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		path,
		networkConversationMessagesValues(query),
		nil,
		&response,
	); err != nil {
		return contract.NetworkThreadMessagesResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) NetworkDirects(
	ctx context.Context,
	query NetworkDirectsQuery,
) (contract.NetworkDirectRoomsResponse, error) {
	channel, err := requireNetworkPathValue("channel", query.Channel)
	if err != nil {
		return contract.NetworkDirectRoomsResponse{}, err
	}
	var response contract.NetworkDirectRoomsResponse
	path, err := networkChannelPath(query.WorkspaceRef, channel)
	if err != nil {
		return contract.NetworkDirectRoomsResponse{}, err
	}
	path += "/directs"
	if err := c.doJSON(ctx, http.MethodGet, path, networkDirectsValues(query), nil, &response); err != nil {
		return contract.NetworkDirectRoomsResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) NetworkDirectResolve(
	ctx context.Context,
	workspaceRef string,
	channel string,
	request NetworkDirectResolveRequest,
) (NetworkDirectRoomRecord, error) {
	channel, err := requireNetworkPathValue("channel", channel)
	if err != nil {
		return NetworkDirectRoomRecord{}, err
	}
	var response struct {
		Direct NetworkDirectRoomRecord `json:"direct"`
	}
	path, err := networkChannelPath(workspaceRef, channel)
	if err != nil {
		return NetworkDirectRoomRecord{}, err
	}
	path += "/directs/resolve"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return NetworkDirectRoomRecord{}, err
	}
	return response.Direct, nil
}

func (c *unixSocketClient) NetworkDirect(
	ctx context.Context,
	workspaceRef string,
	channel string,
	directID string,
) (NetworkDirectRoomRecord, error) {
	path, err := networkDirectPath(workspaceRef, channel, directID)
	if err != nil {
		return NetworkDirectRoomRecord{}, err
	}
	var response struct {
		Direct NetworkDirectRoomRecord `json:"direct"`
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return NetworkDirectRoomRecord{}, err
	}
	return response.Direct, nil
}

func (c *unixSocketClient) NetworkDirectMessages(
	ctx context.Context,
	query NetworkConversationMessagesQuery,
) (contract.NetworkDirectRoomMessagesResponse, error) {
	path, err := networkDirectMessagesPath(query.WorkspaceRef, query.Channel, query.DirectID)
	if err != nil {
		return contract.NetworkDirectRoomMessagesResponse{}, err
	}
	var response contract.NetworkDirectRoomMessagesResponse
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		path,
		networkConversationMessagesValues(query),
		nil,
		&response,
	); err != nil {
		return contract.NetworkDirectRoomMessagesResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) NetworkWork(
	ctx context.Context,
	workspaceRef string,
	workID string,
) (NetworkWorkRecord, error) {
	workID, err := requireNetworkPathValue("work_id", workID)
	if err != nil {
		return NetworkWorkRecord{}, err
	}
	var response struct {
		Work NetworkWorkRecord `json:"work"`
	}
	path, err := networkBasePath(workspaceRef)
	if err != nil {
		return NetworkWorkRecord{}, err
	}
	path += "/work/" + url.PathEscape(workID)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return NetworkWorkRecord{}, err
	}
	return response.Work, nil
}

func (c *unixSocketClient) NetworkSend(ctx context.Context, request NetworkSendRequest) (NetworkSendRecord, error) {
	var response struct {
		Message NetworkSendRecord `json:"message"`
	}
	path, err := networkBasePath(request.WorkspaceID)
	if err != nil {
		return NetworkSendRecord{}, err
	}
	body := request
	body.WorkspaceID = ""
	if err := c.doJSON(ctx, http.MethodPost, path+"/send", nil, body, &response); err != nil {
		return NetworkSendRecord{}, err
	}
	return response.Message, nil
}

func (c *unixSocketClient) NetworkInbox(
	ctx context.Context,
	workspaceRef string,
	sessionID string,
) ([]NetworkEnvelopeRecord, error) {
	var response struct {
		Messages []NetworkEnvelopeRecord `json:"messages"`
	}
	path, err := networkBasePath(workspaceRef)
	if err != nil {
		return nil, err
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		path+"/inbox",
		networkInboxValues(sessionID),
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	return response.Messages, nil
}

func (c *unixSocketClient) PromoteNetworkThreadTask(
	ctx context.Context,
	workspaceRef string,
	channel string,
	threadID string,
	request PromoteNetworkThreadTaskRequest,
) (PromoteNetworkThreadTaskRecord, error) {
	path, err := networkThreadPath(workspaceRef, channel, threadID)
	if err != nil {
		return PromoteNetworkThreadTaskRecord{}, err
	}
	var response contract.PromoteNetworkThreadTaskResponse
	if err := c.doJSON(ctx, http.MethodPost, path+"/promote-task", nil, request, &response); err != nil {
		return PromoteNetworkThreadTaskRecord{}, err
	}
	return response, nil
}

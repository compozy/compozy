package spec

import "github.com/compozy/agh/internal/api/contract"

func registryNetworkOperations() []OperationSpec {
	return []OperationSpec{
		getNetworkStatusOperationSpec(),
		listNetworkPeersOperationSpec(),
		getNetworkPeerOperationSpec(),
		listNetworkChannelsOperationSpec(),
		createNetworkChannelOperationSpec(),
		getNetworkChannelOperationSpec(),
		updateNetworkChannelOperationSpec(),
		listNetworkSubscriptionsOperationSpec(),
		upsertNetworkSubscriptionOperationSpec(),
		deleteNetworkSubscriptionOperationSpec(),
		listNetworkThreadsOperationSpec(),
		getNetworkThreadOperationSpec(),
		promoteNetworkThreadTaskOperationSpec(),
		listNetworkThreadMessagesOperationSpec(),
		listNetworkDirectRoomsOperationSpec(),
		resolveNetworkDirectRoomOperationSpec(),
		getNetworkDirectRoomOperationSpec(),
		listNetworkDirectRoomMessagesOperationSpec(),
		getNetworkWorkOperationSpec(),
		sendNetworkMessageOperationSpec(),
		listNetworkInboxOperationSpec(),
	}
}
func getNetworkStatusOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/network/status",
		OperationID: "getNetworkStatus",
		Summary:     "Get the network runtime status snapshot",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkStatusResponse{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listNetworkPeersOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/network/peers",
		OperationID: "listNetworkPeers",
		Summary:     "List visible network peers",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			queryParam("channel", "Filter peers by channel", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkPeersResponse{}},
			{Status: 400, Description: "Invalid network filter", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getNetworkPeerOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/network/peers/{peer_id}",
		OperationID: "getNetworkPeer",
		Summary:     "Get one visible network peer detail",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("peer_id", "Network peer id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkPeerResponse{}},
			{Status: 404, Description: "Network peer not found", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listNetworkChannelsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/network/channels",
		OperationID: "listNetworkChannels",
		Summary:     "List materialized network channels",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			intQueryParam("recent_limit", "Maximum cross-channel recent conversations to include"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkChannelsResponse{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func createNetworkChannelOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/network/channels",
		OperationID: "createNetworkChannel",
		Summary:     "Create a network channel by spawning agent sessions",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
		},
		RequestBody: contract.CreateNetworkChannelRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.CreateNetworkChannelResponse{}},
			{Status: 400, Description: "Invalid network channel request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getNetworkChannelOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/network/channels/{channel}",
		OperationID: "getNetworkChannel",
		Summary:     "Get one network channel detail",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("channel", "Network channel"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkChannelResponse{}},
			{Status: 400, Description: "Invalid network channel", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Network channel not found", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func updateNetworkChannelOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        "/api/workspaces/{workspace_id}/network/channels/{channel}",
		OperationID: "updateNetworkChannel",
		Summary:     "Update mutable delivery policy for one network channel",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("channel", "Network channel"),
		},
		RequestBody: contract.UpdateNetworkChannelRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkChannelResponse{}},
			{Status: 400, Description: "Invalid network channel policy", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Network channel not found", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listNetworkSubscriptionsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/network/channels/{channel}/subscriptions",
		OperationID: "listNetworkSubscriptions",
		Summary:     "List delivery subscriptions for one network channel",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("channel", "Network channel"),
			queryParam("session_id", "Filter subscriptions by session id", false),
			queryParam("thread_id", "Filter subscriptions by thread id", false),
			intQueryParam("limit", "Maximum number of subscriptions to return"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkSubscriptionsResponse{}},
			{Status: 400, Description: "Invalid network subscription filter", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func upsertNetworkSubscriptionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPut,
		Path:        "/api/workspaces/{workspace_id}/network/channels/{channel}/subscriptions",
		OperationID: "upsertNetworkSubscription",
		Summary:     "Create or update one network delivery subscription",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("channel", "Network channel"),
		},
		RequestBody: contract.NetworkSubscriptionRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkSubscriptionResponse{}},
			{Status: 400, Description: "Invalid network subscription", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func deleteNetworkSubscriptionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        "/api/workspaces/{workspace_id}/network/channels/{channel}/subscriptions/{session_id}",
		OperationID: "deleteNetworkSubscription",
		Summary:     "Delete one network delivery subscription",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("channel", "Network channel"),
			pathParam("session_id", "Network session id"),
			queryParam("thread_id", "Delete the thread-scoped subscription for this session", false),
		},
		Responses: []ResponseSpec{
			{Status: 204, Description: specNoContentDescription},
			{Status: 400, Description: "Invalid network subscription", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listNetworkThreadsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/network/channels/{channel}/threads",
		OperationID: "listNetworkThreads",
		Summary:     "List public threads in one network channel",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: append([]ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("channel", "Network channel"),
		}, networkConversationListQueryParams("threads", true)...),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkThreadsResponse{}},
			{Status: 400, Description: "Invalid public-thread request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getNetworkThreadOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/network/channels/{channel}/threads/{thread_id}",
		OperationID: "getNetworkThread",
		Summary:     "Get one public-thread summary",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("channel", "Network channel"),
			pathParam("thread_id", "Public thread id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkThreadResponse{}},
			{Status: 400, Description: "Invalid public thread", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specNetworkThreadNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func promoteNetworkThreadTaskOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/network/channels/{channel}/threads/{thread_id}/promote-task",
		OperationID: "promoteNetworkThreadTask",
		Summary:     "Promote one network thread message into a durable task",
		Tags:        []string{specNetworkKey, specTasksKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("channel", "Network channel"),
			pathParam("thread_id", "Public thread id"),
		},
		RequestBody: contract.PromoteNetworkThreadTaskRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.PromoteNetworkThreadTaskResponse{}},
			{Status: 400, Description: "Invalid thread promotion request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specNetworkThreadNotFoundDescription, Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specTaskOrBridgeServiceIsNotConfiguredDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listNetworkThreadMessagesOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/network/channels/{channel}/threads/{thread_id}/messages",
		OperationID: "listNetworkThreadMessages",
		Summary:     "List messages in one public thread",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: append([]ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("channel", "Network channel"),
			pathParam("thread_id", "Public thread id"),
		}, networkConversationMessageQueryParams()...),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkThreadMessagesResponse{}},
			{Status: 400, Description: "Invalid public-thread messages request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specNetworkThreadNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listNetworkDirectRoomsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/network/channels/{channel}/directs",
		OperationID: "listNetworkDirectRooms",
		Summary:     "List direct rooms in one network channel",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: append([]ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("channel", "Network channel"),
		}, networkConversationListQueryParams("direct rooms", true)...),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkDirectRoomsResponse{}},
			{Status: 400, Description: "Invalid direct-room request", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func resolveNetworkDirectRoomOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/network/channels/{channel}/directs/resolve",
		OperationID: "resolveNetworkDirectRoom",
		Summary:     "Create or return a deterministic direct room",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("channel", "Network channel"),
		},
		RequestBody: contract.NetworkDirectResolveRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkDirectRoomResponse{}},
			{Status: 400, Description: "Invalid direct-room resolve request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Network direct-room peer not found", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Direct-room collision", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getNetworkDirectRoomOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/network/channels/{channel}/directs/{direct_id}",
		OperationID: "getNetworkDirectRoom",
		Summary:     "Get one direct-room summary",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("channel", "Network channel"),
			pathParam("direct_id", "Direct-room id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkDirectRoomResponse{}},
			{Status: 400, Description: "Invalid direct room", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Network direct room not found", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listNetworkDirectRoomMessagesOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/network/channels/{channel}/directs/{direct_id}/messages",
		OperationID: "listNetworkDirectRoomMessages",
		Summary:     "List messages in one direct room",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: append([]ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("channel", "Network channel"),
			pathParam("direct_id", "Direct-room id"),
		}, networkConversationMessageQueryParams()...),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkDirectRoomMessagesResponse{}},
			{Status: 400, Description: "Invalid direct-room messages request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Network direct room not found", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getNetworkWorkOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/network/work/{work_id}",
		OperationID: "getNetworkWork",
		Summary:     "Get one network work item",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("work_id", "Network work id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkWorkResponse{}},
			{Status: 400, Description: "Invalid network work id", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Network work not found", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func sendNetworkMessageOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/network/send",
		OperationID: "sendNetworkMessage",
		Summary:     "Send one network message",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
		},
		RequestBody: contract.NetworkSendRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkSendResponse{}},
			{Status: 400, Description: "Invalid network send request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Network target not found", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listNetworkInboxOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/network/inbox",
		OperationID: "listNetworkInbox",
		Summary:     "List queued network inbox messages for one local session",
		Tags:        []string{specNetworkKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			queryParam("session_id", "Target local session id", true),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.NetworkInboxResponse{}},
			{Status: 400, Description: "Invalid inbox request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Network target not found", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specNetworkRuntimeIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

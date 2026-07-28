package contract

import "time"

// NetworkRecentPayload is one cross-channel recent thread/direct projection.
type NetworkRecentPayload struct {
	Channel            string     `json:"channel"`
	Surface            string     `json:"surface"`
	ContainerID        string     `json:"container_id"`
	LastActivityAt     *time.Time `json:"last_activity_at,omitempty"`
	LastMessagePreview string     `json:"last_message_preview,omitempty"`
	Title              string     `json:"title,omitempty"`
	ParticipantCount   int        `json:"participant_count,omitempty"`
	SessionA           string     `json:"session_a,omitempty"`
	SessionB           string     `json:"session_b,omitempty"`
}

// NetworkStatusResponse wraps the network runtime status payload.
type NetworkStatusResponse struct {
	Network NetworkStatusPayload `json:"network"`
}

// NetworkPeersResponse wraps the visible peer list payload.
type NetworkPeersResponse struct {
	Peers []NetworkPeerPayload `json:"peers"`
}

// NetworkChannelsResponse wraps channels and bounded cross-channel recents.
type NetworkChannelsResponse struct {
	Channels []NetworkChannelPayload `json:"channels"`
	Recents  []NetworkRecentPayload  `json:"recents,omitempty"`
}

// CreateNetworkChannelResponse wraps the created channel detail payload.
type CreateNetworkChannelResponse struct {
	Channel NetworkChannelDetailPayload `json:"channel"`
}

// NetworkChannelResponse wraps one channel detail payload.
type NetworkChannelResponse struct {
	Channel NetworkChannelDetailPayload `json:"channel"`
}

// NetworkSubscriptionsResponse wraps channel/thread subscription preferences.
type NetworkSubscriptionsResponse struct {
	Subscriptions []NetworkSubscriptionPayload `json:"subscriptions"`
}

// NetworkSubscriptionResponse wraps one subscription preference.
type NetworkSubscriptionResponse struct {
	Subscription NetworkSubscriptionPayload `json:"subscription"`
}

// NetworkChannelMessagesResponse wraps one cursor page of channel messages.
type NetworkChannelMessagesResponse struct {
	Messages []NetworkConversationMessagePayload `json:"messages"`
	Page     CursorPagePayload                   `json:"page"`
}

// NetworkPeerResponse wraps one selected peer detail payload.
type NetworkPeerResponse struct {
	Peer NetworkPeerDetailPayload `json:"peer"`
}

// NetworkPeerMessagesResponse wraps one cursor page of peer-room messages.
type NetworkPeerMessagesResponse struct {
	Messages []NetworkConversationMessagePayload `json:"messages"`
	Page     CursorPagePayload                   `json:"page"`
}

// NetworkSendResponse wraps the outbound send result payload.
type NetworkSendResponse struct {
	Message NetworkSendPayload `json:"message"`
}

// NetworkThreadsResponse wraps one cursor page of public-thread summaries.
type NetworkThreadsResponse struct {
	Threads []NetworkThreadSummaryPayload `json:"threads"`
	Page    CountedCursorPagePayload      `json:"page"`
}

// NetworkThreadResponse wraps one public-thread summary.
type NetworkThreadResponse struct {
	Thread       NetworkThreadSummaryPayload       `json:"thread"`
	SessionCosts []NetworkThreadSessionCostPayload `json:"session_costs,omitempty"`
	TaskLinks    []NetworkTaskThreadOriginPayload  `json:"task_links,omitempty"`
}

// NetworkThreadMessagesResponse wraps one cursor page of public-thread messages.
type NetworkThreadMessagesResponse struct {
	Messages []NetworkConversationMessagePayload `json:"messages"`
	Page     CursorPagePayload                   `json:"page"`
}

// PromoteNetworkThreadTaskResponse wraps a promoted task and its network origin link.
type PromoteNetworkThreadTaskResponse struct {
	Task   TaskPayload                    `json:"task"`
	Origin NetworkTaskThreadOriginPayload `json:"origin"`
}

// NetworkDirectRoomsResponse wraps one cursor page of direct-room summaries.
type NetworkDirectRoomsResponse struct {
	Directs []NetworkDirectRoomPayload `json:"directs"`
	Page    CountedCursorPagePayload   `json:"page"`
}

// NetworkDirectRoomResponse wraps one direct-room summary.
type NetworkDirectRoomResponse struct {
	Direct NetworkDirectRoomPayload `json:"direct"`
}

// NetworkDirectRoomMessagesResponse wraps one cursor page of direct-room messages.
type NetworkDirectRoomMessagesResponse struct {
	Messages []NetworkConversationMessagePayload `json:"messages"`
	Page     CursorPagePayload                   `json:"page"`
}

// NetworkWorkResponse wraps one network work lookup.
type NetworkWorkResponse struct {
	Work NetworkWorkPayload `json:"work"`
}

// NetworkInboxResponse wraps the queued inbox payload.
type NetworkInboxResponse struct {
	Messages []NetworkEnvelopePayload `json:"messages"`
}

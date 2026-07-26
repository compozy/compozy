package contract

import apicontract "github.com/compozy/agh/internal/api/contract"

// NetworkChannelsParams filters visible channels by workspace.
type NetworkChannelsParams struct {
	WorkspaceID string `json:"workspace_id"`
}

// NetworkUsageParams filters one bounded workspace usage page.
type NetworkUsageParams struct {
	WorkspaceID string `json:"workspace_id"`
	OwnerKind   string `json:"owner_kind,omitempty"`
	OwnerID     string `json:"owner_id,omitempty"`
	RunID       string `json:"run_id,omitempty"`
	Channel     string `json:"channel,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
	Limit       *int   `json:"limit,omitempty"`
}

// NetworkPeersParams filters visible peers by workspace and channel.
type NetworkPeersParams struct {
	WorkspaceID string `json:"workspace_id"`
	Channel     string `json:"channel,omitempty"`
}

// NetworkThreadsParams filters one counted public-thread page.
type NetworkThreadsParams struct {
	WorkspaceID string `json:"workspace_id"`
	Channel     string `json:"channel"`
	Query       string `json:"query,omitempty"`
	PeerID      string `json:"peer_id,omitempty"`
	Sort        string `json:"sort,omitempty"`
	HasWork     *bool  `json:"has_work,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	After       string `json:"after,omitempty"`
}

// NetworkThreadTargetParams identifies one public thread.
type NetworkThreadTargetParams struct {
	WorkspaceID string `json:"workspace_id"`
	Channel     string `json:"channel"`
	ThreadID    string `json:"thread_id"`
}

// NetworkThreadMessagesParams filters messages inside one public thread.
type NetworkThreadMessagesParams struct {
	WorkspaceID string `json:"workspace_id"`
	Channel     string `json:"channel"`
	ThreadID    string `json:"thread_id"`
	Before      string `json:"before,omitempty"`
	After       string `json:"after,omitempty"`
	Kind        string `json:"kind,omitempty"`
	WorkID      string `json:"work_id,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

// NetworkDirectsParams filters one counted direct-room page.
type NetworkDirectsParams struct {
	WorkspaceID string `json:"workspace_id"`
	Channel     string `json:"channel"`
	Query       string `json:"query,omitempty"`
	PeerID      string `json:"peer_id,omitempty"`
	Sort        string `json:"sort,omitempty"`
	HasWork     *bool  `json:"has_work,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	After       string `json:"after,omitempty"`
}

// NetworkDirectResolveParams creates or returns a deterministic direct room.
type NetworkDirectResolveParams struct {
	WorkspaceID string `json:"workspace_id"`
	Channel     string `json:"channel"`
	apicontract.NetworkDirectResolveRequest
}

// NetworkDirectMessagesParams filters messages inside one direct room.
type NetworkDirectMessagesParams struct {
	WorkspaceID string `json:"workspace_id"`
	Channel     string `json:"channel"`
	DirectID    string `json:"direct_id"`
	Before      string `json:"before,omitempty"`
	After       string `json:"after,omitempty"`
	Kind        string `json:"kind,omitempty"`
	WorkID      string `json:"work_id,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

// NetworkWorkGetParams identifies one network work row.
type NetworkWorkGetParams struct {
	WorkspaceID string `json:"workspace_id"`
	WorkID      string `json:"work_id"`
}

// NetworkSendParams is the shared daemon network send request payload.
type NetworkSendParams = apicontract.NetworkSendRequest

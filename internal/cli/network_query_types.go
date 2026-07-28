package cli

// NetworkPeersQuery captures CLI filters for peer listing.
type NetworkPeersQuery struct {
	WorkspaceRef string
	Channel      string
}

// NetworkSubscriptionsQuery captures CLI filters for delivery preferences.
type NetworkSubscriptionsQuery struct {
	WorkspaceRef string
	Channel      string
	ThreadID     string
	SessionID    string
	Limit        int
}

// NetworkThreadsQuery captures CLI filters for public-thread listing.
type NetworkThreadsQuery struct {
	WorkspaceRef string
	Channel      string
	Query        string
	SessionID    string
	Sort         string
	HasWork      *bool
	Limit        int
	After        string
}

// NetworkDirectsQuery captures CLI filters for direct-room listing.
type NetworkDirectsQuery struct {
	WorkspaceRef string
	Channel      string
	Query        string
	SessionID    string
	Sort         string
	HasWork      *bool
	Limit        int
	After        string
}

// NetworkConversationMessagesQuery captures CLI filters for conversation messages.
type NetworkConversationMessagesQuery struct {
	WorkspaceRef string
	Channel      string
	ThreadID     string
	DirectID     string
	Limit        int
	Before       string
	After        string
	Kind         string
	WorkID       string
}

package contract

import (
	"encoding/json"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

// NetworkStatusPayload is the shared network diagnostics response payload.
type NetworkStatusPayload struct {
	Enabled              bool                            `json:"enabled"`
	Status               string                          `json:"status"`
	LocalPeers           int                             `json:"local_peers,omitempty"`
	Channels             int                             `json:"channels,omitempty"`
	MessagesSent         int64                           `json:"messages_sent,omitempty"`
	MessagesReceived     int64                           `json:"messages_received,omitempty"`
	MessagesRejected     int64                           `json:"messages_rejected,omitempty"`
	MessagesDelivered    int64                           `json:"messages_delivered,omitempty"`
	WorkflowTaggedEvents int64                           `json:"workflow_tagged_events,omitempty"`
	HandoffTaggedEvents  int64                           `json:"handoff_tagged_events,omitempty"`
	OpenThreads          int64                           `json:"open_threads,omitempty"`
	OpenDirectRooms      int64                           `json:"open_direct_rooms,omitempty"`
	OpenWorkItems        int64                           `json:"open_work_items,omitempty"`
	ConversationMessages int64                           `json:"conversation_messages,omitempty"`
	WorkTransitions      int64                           `json:"work_transitions,omitempty"`
	DirectResolves       int64                           `json:"direct_resolves,omitempty"`
	DeclaredChannels     []DeclaredNetworkChannelPayload `json:"declared_channels,omitempty"`
	KindMetrics          []NetworkKindMetricPayload      `json:"kind_metrics,omitempty"`
}

// NetworkKindMetricPayload is the per-kind network runtime metric snapshot.
type NetworkKindMetricPayload struct {
	Kind      string `json:"kind"`
	Sent      int64  `json:"sent,omitempty"`
	Received  int64  `json:"received,omitempty"`
	Rejected  int64  `json:"rejected,omitempty"`
	Delivered int64  `json:"delivered,omitempty"`
}

// NetworkSendRequest is the shared daemon network send request payload.
type NetworkSendRequest struct {
	WorkspaceID string                     `json:"workspace_id,omitempty"`
	SessionID   string                     `json:"session_id"`
	Channel     string                     `json:"channel"`
	Surface     string                     `json:"surface,omitempty"`
	ThreadID    string                     `json:"thread_id,omitempty"`
	DirectID    string                     `json:"direct_id,omitempty"`
	Kind        string                     `json:"kind"`
	To          string                     `json:"to,omitempty"`
	Mentions    []string                   `json:"mentions,omitempty"`
	Body        json.RawMessage            `json:"body"`
	WorkID      string                     `json:"work_id,omitempty"`
	ReplyTo     string                     `json:"reply_to,omitempty"`
	TraceID     string                     `json:"trace_id,omitempty"`
	CausationID string                     `json:"causation_id,omitempty"`
	ExpiresAt   *int64                     `json:"expires_at,omitempty"`
	ID          string                     `json:"id,omitempty"`
	Ext         map[string]json.RawMessage `json:"ext,omitempty"`
}

// NetworkSendPayload is the shared daemon network send response payload.
type NetworkSendPayload struct {
	ID          string                     `json:"id"`
	WorkspaceID string                     `json:"workspace_id,omitempty"`
	SessionID   string                     `json:"session_id"`
	Channel     string                     `json:"channel"`
	Surface     string                     `json:"surface,omitempty"`
	ThreadID    string                     `json:"thread_id,omitempty"`
	DirectID    string                     `json:"direct_id,omitempty"`
	Kind        string                     `json:"kind"`
	To          string                     `json:"to,omitempty"`
	Mentions    []string                   `json:"mentions,omitempty"`
	WorkID      string                     `json:"work_id,omitempty"`
	ReplyTo     string                     `json:"reply_to,omitempty"`
	TraceID     string                     `json:"trace_id,omitempty"`
	CausationID string                     `json:"causation_id,omitempty"`
	ExpiresAt   *int64                     `json:"expires_at,omitempty"`
	Ext         map[string]json.RawMessage `json:"ext,omitempty"`
}

// CreateNetworkChannelRequest is the shared network channel creation payload.
type CreateNetworkChannelRequest struct {
	Channel           string   `json:"channel"`
	WorkspaceID       string   `json:"workspace_id"`
	Purpose           string   `json:"purpose"`
	FanoutPolicy      string   `json:"fanout_policy,omitempty"`
	CoordinatorPeerID string   `json:"coordinator_peer_id,omitempty"`
	AgentNames        []string `json:"agent_names"`
}

// UpdateNetworkChannelRequest captures mutable channel delivery policy fields.
type UpdateNetworkChannelRequest struct {
	Purpose           *string `json:"purpose,omitempty"`
	FanoutPolicy      *string `json:"fanout_policy,omitempty"`
	CoordinatorPeerID *string `json:"coordinator_peer_id,omitempty"`
}

// NetworkSubscriptionRequest captures one session delivery preference mutation.
type NetworkSubscriptionRequest struct {
	ThreadID  string `json:"thread_id,omitempty"`
	SessionID string `json:"session_id"`
	Mode      string `json:"mode"`
}

// NetworkSubscriptionPayload exposes one session delivery preference.
type NetworkSubscriptionPayload struct {
	WorkspaceID string     `json:"workspace_id,omitempty"`
	Channel     string     `json:"channel"`
	ThreadID    string     `json:"thread_id,omitempty"`
	SessionID   string     `json:"session_id"`
	Mode        string     `json:"mode"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

// PromoteNetworkThreadTaskRequest promotes a thread message into a durable task.
type PromoteNetworkThreadTaskRequest struct {
	OriginMessageID string          `json:"origin_message_id"`
	Title           string          `json:"title,omitempty"`
	Description     string          `json:"description,omitempty"`
	Priority        string          `json:"priority,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
}

// NetworkTaskThreadOriginPayload links a task back to a source thread.
type NetworkTaskThreadOriginPayload struct {
	TaskID           string     `json:"task_id"`
	WorkspaceID      string     `json:"workspace_id,omitempty"`
	Channel          string     `json:"channel"`
	ThreadID         string     `json:"thread_id"`
	OriginMessageID  string     `json:"origin_message_id"`
	Digest           string     `json:"digest"`
	SourceMessageIDs []string   `json:"source_message_ids,omitempty"`
	CreatedAt        *time.Time `json:"created_at,omitempty"`
	UpdatedAt        *time.Time `json:"updated_at,omitempty"`
}

// TaskFanOutRunDesignationRequest describes one designated sibling run.
type TaskFanOutRunDesignationRequest struct {
	Brief          string          `json:"brief"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
}

// FanOutTaskRunsRequest captures one designated fan-out enqueue request.
type FanOutTaskRunsRequest struct {
	NetworkParticipation *participation.Request            `json:"network_participation,omitempty"`
	Designations         []TaskFanOutRunDesignationRequest `json:"designations"`
	IdempotencyKey       string                            `json:"idempotency_key,omitempty"`
}

// NetworkCapabilityBriefPayload is the shared brief discovery projection for
// one peer capability.
type NetworkCapabilityBriefPayload struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
}

// NetworkCapabilityPayload is the shared rich capability payload surfaced by
// daemon APIs.
type NetworkCapabilityPayload struct {
	ID                string   `json:"id"`
	Summary           string   `json:"summary"`
	Outcome           string   `json:"outcome"`
	Version           string   `json:"version,omitempty"`
	Digest            string   `json:"digest,omitempty"`
	ContextNeeded     []string `json:"context_needed,omitempty"`
	ArtifactsExpected []string `json:"artifacts_expected,omitempty"`
	ExecutionOutline  []string `json:"execution_outline,omitempty"`
	Constraints       []string `json:"constraints,omitempty"`
	Examples          []string `json:"examples,omitempty"`
	Requirements      []string `json:"requirements,omitempty"`
}

// NetworkCapabilityCatalogPayload is the shared rich discovery catalog surfaced
// by peer-detail APIs when explicit rich capability data is available.
type NetworkCapabilityCatalogPayload struct {
	Capabilities []NetworkCapabilityPayload `json:"capabilities"`
}

// NetworkPeerCardPayload is the shared JSON representation of one peer card.
type NetworkPeerCardPayload struct {
	PeerID              string                          `json:"peer_id"`
	DisplayName         *string                         `json:"display_name,omitempty"`
	ProfilesSupported   []string                        `json:"profiles_supported"`
	Capabilities        []NetworkCapabilityBriefPayload `json:"capabilities"`
	ArtifactsSupported  []string                        `json:"artifacts_supported"`
	TrustModesSupported []string                        `json:"trust_modes_supported"`
	Ext                 map[string]json.RawMessage      `json:"ext,omitempty"`
}

const (
	// NetworkPresenceLocal identifies a daemon-local peer joined to a channel.
	NetworkPresenceLocal = "local"
)

// NetworkPeerPayload is the shared JSON representation of one visible peer.
type NetworkPeerPayload struct {
	WorkspaceID   string                 `json:"workspace_id,omitempty"`
	SessionID     *string                `json:"session_id,omitempty"`
	PeerID        string                 `json:"peer_id"`
	DisplayName   string                 `json:"display_name,omitempty"`
	Channel       string                 `json:"channel"`
	Local         bool                   `json:"local"`
	PeerCard      NetworkPeerCardPayload `json:"peer_card"`
	JoinedAt      *time.Time             `json:"joined_at,omitempty"`
	PresenceState string                 `json:"presence_state"`
}

// NetworkChannelPayload is the shared JSON representation of one active channel.
type NetworkChannelPayload struct {
	Channel                    string     `json:"channel"`
	WorkspaceID                string     `json:"workspace_id,omitempty"`
	Purpose                    string     `json:"purpose,omitempty"`
	FanoutPolicy               string     `json:"fanout_policy,omitempty"`
	CoordinatorPeerID          string     `json:"coordinator_peer_id,omitempty"`
	CreatedBy                  string     `json:"created_by,omitempty"`
	CreatedAt                  *time.Time `json:"created_at,omitempty"`
	PeerCount                  int        `json:"peer_count"`
	LocalPeerCount             int        `json:"local_peer_count,omitempty"`
	SessionCount               int        `json:"session_count,omitempty"`
	MessageCount               int        `json:"message_count,omitempty"`
	PresenceCount              int        `json:"presence_count,omitempty"`
	HistoricalParticipantCount int        `json:"historical_participant_count,omitempty"`
	LastActivityAt             *time.Time `json:"last_activity_at,omitempty"`
	LastPresenceAt             *time.Time `json:"last_presence_at,omitempty"`
	LastMessagePreview         string     `json:"last_message_preview,omitempty"`
}

// NetworkEnvelopePayload is the shared JSON representation of one surfaced
// network envelope used by inbox and audit-facing views.
type NetworkEnvelopePayload struct {
	Protocol    string                     `json:"protocol"`
	ID          string                     `json:"id"`
	Kind        string                     `json:"kind"`
	WorkspaceID string                     `json:"workspace_id,omitempty"`
	Channel     string                     `json:"channel"`
	Surface     *string                    `json:"surface,omitempty"`
	ThreadID    *string                    `json:"thread_id,omitempty"`
	DirectID    *string                    `json:"direct_id,omitempty"`
	From        string                     `json:"from"`
	To          *string                    `json:"to,omitempty"`
	Mentions    []string                   `json:"mentions,omitempty"`
	WorkID      *string                    `json:"work_id,omitempty"`
	ReplyTo     *string                    `json:"reply_to,omitempty"`
	TraceID     *string                    `json:"trace_id,omitempty"`
	CausationID *string                    `json:"causation_id,omitempty"`
	TS          int64                      `json:"ts"`
	ExpiresAt   *int64                     `json:"expires_at,omitempty"`
	Body        json.RawMessage            `json:"body"`
	Proof       map[string]json.RawMessage `json:"proof,omitempty"`
	Ext         map[string]json.RawMessage `json:"ext,omitempty"`
}

// NetworkChannelDetailPayload is the shared channel detail payload used by the network UI.
type NetworkChannelDetailPayload struct {
	Channel                    string                           `json:"channel"`
	WorkspaceID                string                           `json:"workspace_id,omitempty"`
	Purpose                    string                           `json:"purpose,omitempty"`
	FanoutPolicy               string                           `json:"fanout_policy,omitempty"`
	CoordinatorPeerID          string                           `json:"coordinator_peer_id,omitempty"`
	CreatedBy                  string                           `json:"created_by,omitempty"`
	CreatedAt                  *time.Time                       `json:"created_at,omitempty"`
	PeerCount                  int                              `json:"peer_count"`
	LocalPeerCount             int                              `json:"local_peer_count,omitempty"`
	SessionCount               int                              `json:"session_count,omitempty"`
	MessageCount               int                              `json:"message_count,omitempty"`
	PresenceCount              int                              `json:"presence_count,omitempty"`
	HistoricalParticipantCount int                              `json:"historical_participant_count,omitempty"`
	LastActivityAt             *time.Time                       `json:"last_activity_at,omitempty"`
	LastPresenceAt             *time.Time                       `json:"last_presence_at,omitempty"`
	LastMessagePreview         string                           `json:"last_message_preview,omitempty"`
	KindCounts                 []NetworkChannelKindCountPayload `json:"kind_counts,omitempty"`
	Sessions                   []SessionPayload                 `json:"sessions,omitempty"`
	Peers                      []NetworkPeerPayload             `json:"peers,omitempty"`
}

// NetworkChannelKindCountPayload reports one channel-level kind count.
type NetworkChannelKindCountPayload struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

// NetworkConversationMessagePayload is the shared network conversation timeline payload.
type NetworkConversationMessagePayload struct {
	MessageID   string          `json:"message_id"`
	WorkspaceID string          `json:"workspace_id,omitempty"`
	Channel     string          `json:"channel"`
	Surface     string          `json:"surface,omitempty"`
	ThreadID    string          `json:"thread_id,omitempty"`
	DirectID    string          `json:"direct_id,omitempty"`
	Kind        string          `json:"kind"`
	Direction   string          `json:"direction"`
	PeerFrom    string          `json:"peer_from"`
	PeerTo      string          `json:"peer_to,omitempty"`
	Mentions    []string        `json:"mentions,omitempty"`
	DisplayName string          `json:"display_name,omitempty"`
	SessionID   string          `json:"session_id,omitempty"`
	Local       bool            `json:"local,omitempty"`
	WorkID      string          `json:"work_id,omitempty"`
	ReplyTo     string          `json:"reply_to,omitempty"`
	TraceID     string          `json:"trace_id,omitempty"`
	CausationID string          `json:"causation_id,omitempty"`
	Intent      string          `json:"intent,omitempty"`
	Text        string          `json:"text,omitempty"`
	PreviewText string          `json:"preview_text,omitempty"`
	SizeBytes   int64           `json:"size_bytes,omitempty"`
	Body        json.RawMessage `json:"body"`
	Timestamp   time.Time       `json:"timestamp"`
}

// NetworkThreadSummaryPayload is the public-thread list/detail projection.
type NetworkThreadSummaryPayload struct {
	WorkspaceID        string                          `json:"workspace_id,omitempty"`
	Channel            string                          `json:"channel"`
	ThreadID           string                          `json:"thread_id"`
	RootMessageID      string                          `json:"root_message_id"`
	Title              string                          `json:"title,omitempty"`
	OpenedByPeerID     string                          `json:"opened_by_peer_id,omitempty"`
	OpenedSessionID    string                          `json:"opened_session_id,omitempty"`
	OpenedAt           *time.Time                      `json:"opened_at,omitempty"`
	LastActivityAt     *time.Time                      `json:"last_activity_at,omitempty"`
	MessageCount       int                             `json:"message_count"`
	ParticipantCount   int                             `json:"participant_count"`
	OpenWorkCount      int                             `json:"open_work_count"`
	CoordinationCost   *NetworkCoordinationCostPayload `json:"coordination_cost,omitempty"`
	LastMessagePreview string                          `json:"last_message_preview,omitempty"`
}

// NetworkCoordinationCostPayload reports aggregate prompt delivery cost for one public thread.
type NetworkCoordinationCostPayload struct {
	DeliveredCount        int64 `json:"delivered_count,omitempty"`
	PromptSizeBytes       int64 `json:"prompt_size_bytes,omitempty"`
	EstimatedPromptTokens int64 `json:"estimated_prompt_tokens,omitempty"`
}

// NetworkThreadSessionCostPayload reports prompt delivery cost for one session in a public thread.
type NetworkThreadSessionCostPayload struct {
	SessionID             string     `json:"session_id"`
	DeliveredCount        int64      `json:"delivered_count,omitempty"`
	PromptSizeBytes       int64      `json:"prompt_size_bytes,omitempty"`
	EstimatedPromptTokens int64      `json:"estimated_prompt_tokens,omitempty"`
	FirstDeliveredAt      *time.Time `json:"first_delivered_at,omitempty"`
	LastDeliveredAt       *time.Time `json:"last_delivered_at,omitempty"`
}

// NetworkDirectRoomPayload is the direct-room list/detail projection.
type NetworkDirectRoomPayload struct {
	WorkspaceID        string     `json:"workspace_id,omitempty"`
	Channel            string     `json:"channel"`
	DirectID           string     `json:"direct_id"`
	SessionA           string     `json:"session_a"`
	SessionB           string     `json:"session_b"`
	OpenedAt           *time.Time `json:"opened_at,omitempty"`
	LastActivityAt     *time.Time `json:"last_activity_at,omitempty"`
	MessageCount       int        `json:"message_count"`
	OpenWorkCount      int        `json:"open_work_count"`
	LastMessagePreview string     `json:"last_message_preview,omitempty"`
}

// NetworkWorkPayload is the public network work lookup projection.
type NetworkWorkPayload struct {
	WorkID          string     `json:"work_id"`
	WorkspaceID     string     `json:"workspace_id,omitempty"`
	Channel         string     `json:"channel"`
	Surface         string     `json:"surface"`
	ThreadID        string     `json:"thread_id,omitempty"`
	DirectID        string     `json:"direct_id,omitempty"`
	OpenedSessionID string     `json:"opened_session_id,omitempty"`
	TargetSessionID string     `json:"target_session_id,omitempty"`
	State           string     `json:"state"`
	OpenedAt        *time.Time `json:"opened_at,omitempty"`
	LastActivityAt  *time.Time `json:"last_activity_at,omitempty"`
	TerminalAt      *time.Time `json:"terminal_at,omitempty"`
}

// NetworkDirectResolveRequest requests creation or lookup of a direct room.
type NetworkDirectResolveRequest struct {
	SessionID string `json:"session_id"`
	PeerID    string `json:"peer_id"`
}

// NetworkPeerMetricsPayload is the shared peer-level counter payload.
type NetworkPeerMetricsPayload struct {
	Sent               int64 `json:"sent,omitempty"`
	Received           int64 `json:"received,omitempty"`
	Rejected           int64 `json:"rejected,omitempty"`
	Delivered          int64 `json:"delivered,omitempty"`
	SentSizeBytes      int64 `json:"sent_size_bytes,omitempty"`
	ReceivedSizeBytes  int64 `json:"received_size_bytes,omitempty"`
	RejectedSizeBytes  int64 `json:"rejected_size_bytes,omitempty"`
	DeliveredSizeBytes int64 `json:"delivered_size_bytes,omitempty"`
	TotalSizeBytes     int64 `json:"total_size_bytes,omitempty"`
}

// NetworkPeerDetailPayload is the shared selected-peer detail payload.
type NetworkPeerDetailPayload struct {
	SessionID         *string                          `json:"session_id,omitempty"`
	PeerID            string                           `json:"peer_id"`
	DisplayName       string                           `json:"display_name,omitempty"`
	Channel           string                           `json:"channel,omitempty"`
	Local             bool                             `json:"local,omitempty"`
	PeerCard          NetworkPeerCardPayload           `json:"peer_card"`
	CapabilityCatalog *NetworkCapabilityCatalogPayload `json:"capability_catalog,omitempty"`
	JoinedAt          *time.Time                       `json:"joined_at,omitempty"`
	PresenceState     string                           `json:"presence_state"`
	Metrics           NetworkPeerMetricsPayload        `json:"metrics"`
}

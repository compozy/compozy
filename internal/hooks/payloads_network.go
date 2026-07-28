package hooks

import "time"

// NetworkPayload is the shared observation payload for committed network conversation writes.
type NetworkPayload struct {
	PayloadBase
	WorkspaceID string     `json:"workspace_id,omitempty"`
	SessionID   string     `json:"session_id,omitempty"`
	Channel     string     `json:"channel,omitempty"`
	Surface     string     `json:"surface,omitempty"`
	ThreadID    string     `json:"thread_id,omitempty"`
	DirectID    string     `json:"direct_id,omitempty"`
	MessageID   string     `json:"message_id,omitempty"`
	Kind        string     `json:"kind,omitempty"`
	Direction   string     `json:"direction,omitempty"`
	WorkID      string     `json:"work_id,omitempty"`
	WorkState   string     `json:"work_state,omitempty"`
	PeerID      string     `json:"peer_id,omitempty"`
	PeerFrom    string     `json:"peer_from,omitempty"`
	PeerTo      string     `json:"peer_to,omitempty"`
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty"`
	TraceID     string     `json:"trace_id,omitempty"`
	CausationID string     `json:"causation_id,omitempty"`
}

// NetworkThreadOpenedPayload observes a newly opened public thread.
type NetworkThreadOpenedPayload = NetworkPayload

// NetworkDirectRoomOpenedPayload observes a newly opened direct room.
type NetworkDirectRoomOpenedPayload = NetworkPayload

// NetworkMessagePersistedPayload observes a committed conversation message.
type NetworkMessagePersistedPayload = NetworkPayload

// NetworkWorkOpenedPayload observes a newly opened work item.
type NetworkWorkOpenedPayload = NetworkPayload

// NetworkWorkTransitionedPayload observes a work lifecycle transition.
type NetworkWorkTransitionedPayload = NetworkPayload

// NetworkWorkClosedPayload observes a terminal work lifecycle transition.
type NetworkWorkClosedPayload = NetworkPayload

// NetworkPeerJoinedPayload observes a peer becoming visible on a runtime channel.
type NetworkPeerJoinedPayload = NetworkPayload

// NetworkPeerLeftPayload observes a peer leaving or expiring from a runtime channel.
type NetworkPeerLeftPayload = NetworkPayload

// NetworkObservationPatch captures optional labels for network observation hooks.
type NetworkObservationPatch struct {
	Labels map[string]string `json:"labels,omitempty"`
}

// AuthoredContextObservationPatch is the no-op patch surface for authored-context observation hooks.
type AuthoredContextObservationPatch = AutonomyObservationPatch

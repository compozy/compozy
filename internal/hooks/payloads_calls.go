package hooks

import "strings"

// CallPayload is the sanitized observation payload for committed call and mailbox transitions.
type CallPayload struct {
	PayloadBase
	ProfileID        string `json:"profile_id"`
	Scope            string `json:"scope"`
	WorkspaceID      string `json:"workspace_id,omitempty"`
	CallID           string `json:"call_id,omitempty"`
	MessageID        string `json:"message_id,omitempty"`
	ParentSessionID  string `json:"parent_session_id,omitempty"`
	ChildSessionID   string `json:"child_session_id,omitempty"`
	RootSessionID    string `json:"root_session_id,omitempty"`
	AgentName        string `json:"agent_name,omitempty"`
	State            string `json:"state,omitempty"`
	Verdict          string `json:"verdict,omitempty"`
	ActorKind        string `json:"actor_kind,omitempty"`
	ActorID          string `json:"actor_id,omitempty"`
	Channel          string `json:"channel,omitempty"`
	ThreadID         string `json:"thread_id,omitempty"`
	NetworkMessageID string `json:"network_message_id,omitempty"`
	Delivery         string `json:"delivery,omitempty"`
	StoppedChildren  int    `json:"stopped_children,omitempty"`
	ClosedCalls      int    `json:"closed_calls,omitempty"`
	PreservedResults int    `json:"preserved_results,omitempty"`
}

// HookProfileID returns the immutable profile owner for declaration isolation.
func (p CallPayload) HookProfileID() string { return strings.TrimSpace(p.ProfileID) }

// CallObservationPatch is intentionally observation-only.
type CallObservationPatch = AutonomyObservationPatch

package bridges

import (
	"encoding/json"
	"time"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

// NetworkConversationSurface identifies one explicit AGH network conversation container.
type NetworkConversationSurface string

const (
	// NetworkConversationSurfaceThread maps bridge ingress into a public AGH thread.
	NetworkConversationSurfaceThread NetworkConversationSurface = NetworkConversationSurface(
		bridgecontract.NetworkConversationSurfaceThread,
	)
	// NetworkConversationSurfaceDirect maps bridge ingress into a resolved AGH direct room.
	NetworkConversationSurfaceDirect NetworkConversationSurface = NetworkConversationSurface(
		bridgecontract.NetworkConversationSurfaceDirect,
	)
)

// Normalize returns the canonical bridge conversation surface.
func (s NetworkConversationSurface) Normalize() NetworkConversationSurface {
	return NetworkConversationSurface(bridgecontract.NetworkConversationSurface(s).Normalize())
}

// NetworkConversationRef carries an explicit bridge-to-AGH conversation mapping.
type NetworkConversationRef struct {
	Channel     string                     `json:"channel"`
	Surface     NetworkConversationSurface `json:"surface"`
	ThreadID    string                     `json:"thread_id,omitempty"`
	DirectID    string                     `json:"direct_id,omitempty"`
	WorkID      string                     `json:"work_id,omitempty"`
	ReplyTo     string                     `json:"reply_to,omitempty"`
	TraceID     string                     `json:"trace_id,omitempty"`
	CausationID string                     `json:"causation_id,omitempty"`
}

// Validate reports whether the explicit bridge mapping selects one AGH conversation container.
func (r NetworkConversationRef) Validate() error {
	return networkConversationRefToContract(r).Validate()
}

// InboundMessageEnvelope is the normalized bridge ingest payload delivered by adapters.
type InboundMessageEnvelope struct {
	BridgeInstanceID  string                  `json:"bridge_instance_id"`
	Scope             Scope                   `json:"scope"`
	WorkspaceID       string                  `json:"workspace_id,omitempty"`
	PeerID            string                  `json:"peer_id,omitempty"`
	ThreadID          string                  `json:"thread_id,omitempty"`
	GroupID           string                  `json:"group_id,omitempty"`
	PlatformMessageID string                  `json:"platform_message_id,omitempty"`
	ReceivedAt        time.Time               `json:"received_at"`
	Sender            MessageSender           `json:"sender"`
	Content           MessageContent          `json:"content,omitzero"`
	Attachments       []MessageAttachment     `json:"attachments,omitempty"`
	EventFamily       InboundEventFamily      `json:"event_family"`
	Command           *InboundCommand         `json:"command,omitempty"`
	Action            *InboundAction          `json:"action,omitempty"`
	Reaction          *InboundReaction        `json:"reaction,omitempty"`
	Edit              *InboundEdit            `json:"edit,omitempty"`
	ReplyToText       string                  `json:"reply_to_text,omitempty"`
	ReplyToAuthorID   string                  `json:"reply_to_author_id,omitempty"`
	ReplyToAuthorName string                  `json:"reply_to_author_name,omitempty"`
	Conversation      *NetworkConversationRef `json:"conversation,omitempty"`
	ProviderMetadata  json.RawMessage         `json:"provider_metadata,omitempty"`
	IdempotencyKey    string                  `json:"idempotency_key"`
}

// Validate reports whether the inbound envelope contains the required identifying fields.
func (e InboundMessageEnvelope) Validate() error {
	return inboundEnvelopeToContract(e).Validate()
}

// NetworkConversationRef returns only the explicit AGH conversation mapping.
func (e InboundMessageEnvelope) NetworkConversationRef() (NetworkConversationRef, bool, error) {
	reference, ok, err := inboundEnvelopeToContract(e).NetworkConversationRef()
	if err != nil {
		return NetworkConversationRef{}, false, err
	}
	if !ok {
		return NetworkConversationRef{}, false, nil
	}
	return networkConversationRefFromContract(reference), true, nil
}

package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// NetworkConversationSurface identifies an AGH conversation container.
type NetworkConversationSurface string

const (
	NetworkConversationSurfaceThread NetworkConversationSurface = "thread"
	NetworkConversationSurfaceDirect NetworkConversationSurface = "direct"
)

// Normalize returns the canonical conversation surface.
func (s NetworkConversationSurface) Normalize() NetworkConversationSurface {
	return NetworkConversationSurface(strings.ToLower(strings.TrimSpace(string(s))))
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

// NormalizeNetworkConversationRef returns a canonical explicit conversation mapping.
func NormalizeNetworkConversationRef(reference NetworkConversationRef) NetworkConversationRef {
	return reference.normalize()
}

// Validate reports whether the mapping selects one AGH conversation.
func (r NetworkConversationRef) Validate() error {
	normalized := r.normalize()
	if err := requireField(normalized.Channel, "network conversation channel"); err != nil {
		return err
	}
	switch normalized.Surface {
	case NetworkConversationSurfaceThread:
		if err := validateBridgeNetworkConversationID(normalized.ThreadID, "thread_id"); err != nil {
			return err
		}
		if normalized.DirectID != "" {
			return errors.New("bridges: network conversation direct_id must be empty for thread surface")
		}
	case NetworkConversationSurfaceDirect:
		if err := validateBridgeNetworkConversationID(normalized.DirectID, "direct_id"); err != nil {
			return err
		}
		if normalized.ThreadID != "" {
			return errors.New("bridges: network conversation thread_id must be empty for direct surface")
		}
	default:
		return fmt.Errorf(
			"bridges: network conversation surface must be one of %q or %q",
			NetworkConversationSurfaceThread,
			NetworkConversationSurfaceDirect,
		)
	}
	if normalized.WorkID != "" {
		return validateBridgeNetworkConversationID(normalized.WorkID, "work_id")
	}
	return nil
}

func (r NetworkConversationRef) normalize() NetworkConversationRef {
	return NetworkConversationRef{
		Channel: strings.TrimSpace(r.Channel), Surface: r.Surface.Normalize(),
		ThreadID: strings.TrimSpace(r.ThreadID), DirectID: strings.TrimSpace(r.DirectID),
		WorkID: strings.TrimSpace(r.WorkID), ReplyTo: strings.TrimSpace(r.ReplyTo),
		TraceID: strings.TrimSpace(r.TraceID), CausationID: strings.TrimSpace(r.CausationID),
	}
}

// InboundMessageEnvelope is the normalized bridge ingest payload.
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

// NormalizeInboundMessageEnvelope returns the canonical wire envelope with
// independently owned mutable payloads.
func NormalizeInboundMessageEnvelope(envelope InboundMessageEnvelope) InboundMessageEnvelope {
	normalized := envelope.normalize()
	normalized.Attachments = slices.Clone(normalized.Attachments)
	normalized.ProviderMetadata = slices.Clone(normalized.ProviderMetadata)
	return normalized
}

// Validate reports whether the inbound envelope is internally consistent.
func (e InboundMessageEnvelope) Validate() error {
	normalized := e.normalize()
	if err := requireField(normalized.BridgeInstanceID, "inbound message bridge instance id"); err != nil {
		return err
	}
	if err := ValidateScopeWorkspaceID(normalized.Scope, normalized.WorkspaceID); err != nil {
		return err
	}
	if normalized.ReceivedAt.IsZero() {
		return errors.New("bridges: inbound message received at is required")
	}
	if err := normalized.EventFamily.Validate(); err != nil {
		return err
	}
	if _, err := normalizeRawJSON(normalized.ProviderMetadata, "inbound provider metadata"); err != nil {
		return err
	}
	if normalized.Conversation != nil {
		if err := normalized.Conversation.Validate(); err != nil {
			return err
		}
	}
	if err := requireField(normalized.IdempotencyKey, "inbound message idempotency key"); err != nil {
		return err
	}
	return normalized.validatePayload()
}

// NetworkConversationRef returns only the explicit AGH conversation mapping.
func (e InboundMessageEnvelope) NetworkConversationRef() (NetworkConversationRef, bool, error) {
	normalized := e.normalize()
	if normalized.Conversation == nil {
		return NetworkConversationRef{}, false, nil
	}
	ref := normalized.Conversation.normalize()
	if err := ref.Validate(); err != nil {
		return NetworkConversationRef{}, false, err
	}
	return ref, true, nil
}

func (e InboundMessageEnvelope) normalize() InboundMessageEnvelope {
	if len(e.Attachments) > 0 {
		e.Attachments = append([]MessageAttachment(nil), e.Attachments...)
	}
	e.BridgeInstanceID = strings.TrimSpace(e.BridgeInstanceID)
	e.Scope = e.Scope.Normalize()
	e.WorkspaceID = strings.TrimSpace(e.WorkspaceID)
	e.PeerID = strings.TrimSpace(e.PeerID)
	e.ThreadID = strings.TrimSpace(e.ThreadID)
	e.GroupID = strings.TrimSpace(e.GroupID)
	e.PlatformMessageID = strings.TrimSpace(e.PlatformMessageID)
	e.Sender = MessageSender{
		ID: strings.TrimSpace(e.Sender.ID), Username: strings.TrimSpace(e.Sender.Username),
		DisplayName: strings.TrimSpace(e.Sender.DisplayName),
	}
	e.Content = MessageContent{Text: strings.TrimSpace(e.Content.Text)}
	e.EventFamily = e.EventFamily.Normalize()
	if e.EventFamily == "" && e.Command == nil && e.Action == nil && e.Reaction == nil && e.Edit == nil {
		e.EventFamily = InboundEventFamilyMessage
	}
	e.ReplyToText = strings.TrimSpace(e.ReplyToText)
	e.ReplyToAuthorID = strings.TrimSpace(e.ReplyToAuthorID)
	e.ReplyToAuthorName = strings.TrimSpace(e.ReplyToAuthorName)
	e.IdempotencyKey = strings.TrimSpace(e.IdempotencyKey)
	e.ProviderMetadata = bytes.TrimSpace(e.ProviderMetadata)
	for index := range e.Attachments {
		e.Attachments[index] = MessageAttachment{
			ID:       strings.TrimSpace(e.Attachments[index].ID),
			Name:     strings.TrimSpace(e.Attachments[index].Name),
			MIMEType: strings.TrimSpace(e.Attachments[index].MIMEType),
			URL:      strings.TrimSpace(e.Attachments[index].URL),
		}
	}
	if e.Command != nil {
		command := e.Command.normalize()
		e.Command = &command
	}
	if e.Action != nil {
		action := e.Action.normalize()
		e.Action = &action
	}
	if e.Reaction != nil {
		reaction := e.Reaction.normalize()
		e.Reaction = &reaction
	}
	if e.Edit != nil {
		edit := e.Edit.normalize()
		e.Edit = &edit
	}
	if e.Conversation != nil {
		conversation := e.Conversation.normalize()
		e.Conversation = &conversation
	}
	return e
}

func validateBridgeNetworkConversationID(value string, field string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("bridges: network conversation %s is required", field)
	}
	if len(trimmed) > 128 || strings.ContainsAny(trimmed, `/\\`) || containsControlCharacter(trimmed) {
		return fmt.Errorf("bridges: invalid network conversation %s %q", field, value)
	}
	switch field {
	case "thread_id":
		if !strings.HasPrefix(trimmed, "thread_") {
			return fmt.Errorf("bridges: invalid network conversation thread_id %q", value)
		}
	case "direct_id":
		if !strings.HasPrefix(trimmed, "direct_") || len(trimmed) != len("direct_")+32 {
			return fmt.Errorf("bridges: invalid network conversation direct_id %q", value)
		}
	}
	return nil
}

func containsControlCharacter(value string) bool {
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return true
		}
	}
	return false
}

package store

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// NetworkDirectRoomEntry is the write DTO for a direct-room row.
type NetworkDirectRoomEntry struct {
	WorkspaceID    string
	Channel        string
	DirectID       string
	SessionA       string
	SessionB       string
	OpenedAt       time.Time
	LastActivityAt time.Time
}

// Validate ensures direct-room membership is stable and ordered.
func (e NetworkDirectRoomEntry) Validate() error {
	if err := validateNetworkDirectRoom(e.WorkspaceID, e.Channel, e.DirectID, e.SessionA, e.SessionB); err != nil {
		return err
	}
	if e.OpenedAt.IsZero() {
		return fmt.Errorf("store: network direct room opened_at is required")
	}
	if e.LastActivityAt.IsZero() {
		return fmt.Errorf("store: network direct room last_activity_at is required")
	}
	return nil
}

// NetworkWorkEntry stores lifecycle metadata for work inside one conversation.
type NetworkWorkEntry struct {
	WorkID            string
	WorkspaceID       string
	Channel           string
	Surface           string
	ThreadID          string
	DirectID          string
	OpenedBySessionID string
	TargetSessionID   string
	State             string
	OpenedAt          time.Time
	LastActivityAt    time.Time
	TerminalAt        *time.Time
}

// Validate ensures a work row is bound to exactly one conversation container.
func (e NetworkWorkEntry) Validate() error {
	if err := validateNetworkConversationID(e.WorkID, "work_id"); err != nil {
		return err
	}
	ref := NetworkConversationRef{
		WorkspaceID: e.WorkspaceID,
		Channel:     e.Channel,
		Surface:     e.Surface,
		ThreadID:    e.ThreadID,
		DirectID:    e.DirectID,
	}
	if err := ref.Validate(); err != nil {
		return err
	}
	if err := requireField(e.OpenedBySessionID, "network work opened_by_session_id"); err != nil {
		return err
	}
	if err := validateNetworkWorkState(e.State); err != nil {
		return err
	}
	if e.OpenedAt.IsZero() {
		return fmt.Errorf("store: network work opened_at is required")
	}
	if e.LastActivityAt.IsZero() {
		return fmt.Errorf("store: network work last_activity_at is required")
	}
	if e.TerminalAt != nil && !networkWorkStateIsTerminal(e.State) {
		return fmt.Errorf("store: network work terminal_at requires terminal state")
	}
	return nil
}

// NetworkConversationMessage is one persisted network conversation or presence message.
type NetworkConversationMessage struct {
	Sequence    int64
	MessageID   string
	SessionID   string
	WorkspaceID string
	Channel     string
	Surface     string
	ThreadID    string
	DirectID    string
	Direction   string
	PeerFrom    string
	PeerTo      string
	Kind        string
	WorkID      string
	ReplyTo     string
	TraceID     string
	CausationID string
	Intent      string
	Text        string
	PreviewText string
	Mentions    []string
	ExtJSON     json.RawMessage
	Body        json.RawMessage
	SizeBytes   int64
	Timestamp   time.Time
}

// NetworkMessageEntry is the persisted network timeline row used by existing store interfaces.
type NetworkMessageEntry = NetworkConversationMessage

// Validate ensures the persisted network message is complete and internally consistent.
func (e NetworkConversationMessage) Validate() error {
	if e.Sequence < 0 {
		return fmt.Errorf("store: network message sequence must be zero or positive")
	}
	if err := requireField(e.MessageID, "network message id"); err != nil {
		return err
	}
	if err := (NetworkChannelRef{WorkspaceID: e.WorkspaceID, Channel: e.Channel}).Validate(); err != nil {
		return err
	}
	if err := requireField(e.Direction, "network message direction"); err != nil {
		return err
	}
	direction := strings.TrimSpace(e.Direction)
	if direction != e.Direction {
		return fmt.Errorf("store: unsupported network message direction %q", e.Direction)
	}
	switch direction {
	case typesSentKey, "received":
	default:
		return fmt.Errorf("store: unsupported network message direction %q", e.Direction)
	}
	if err := requireField(e.PeerFrom, "network message peer_from"); err != nil {
		return err
	}
	if err := requireField(e.Kind, "network message kind"); err != nil {
		return err
	}
	if err := validateNetworkMessageKind(e.Kind); err != nil {
		return err
	}
	if err := e.validateConversationFields(); err != nil {
		return err
	}
	if _, err := NormalizeNetworkPeerIDs(e.Mentions, "network message mentions"); err != nil {
		return err
	}
	if len(e.Body) == 0 {
		return fmt.Errorf("store: network message body is required")
	}
	if !json.Valid(e.Body) {
		return fmt.Errorf("store: network message body must be valid JSON")
	}
	if networkRawJSONContainsClaimToken(e.Body) {
		return fmt.Errorf("store: network message body contains raw claim_token material")
	}
	if err := validateNetworkMessageExtJSON(e.ExtJSON); err != nil {
		return err
	}
	if networkConversationMessageContainsRawClaimToken(e) {
		return fmt.Errorf("store: network message entry contains raw claim_token material")
	}
	return nil
}

func validateNetworkMessageExtJSON(raw json.RawMessage) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
		return fmt.Errorf("store: network message ext_json must be a JSON object: %w", err)
	}
	if networkRawJSONContainsClaimToken([]byte(trimmed)) {
		return fmt.Errorf("store: network message ext_json contains raw claim_token material")
	}
	return nil
}

func (e NetworkConversationMessage) validateConversationFields() error {
	switch strings.TrimSpace(e.Kind) {
	case NetworkKindGreet, NetworkKindWhois:
		if strings.TrimSpace(e.Surface) != "" ||
			strings.TrimSpace(e.ThreadID) != "" ||
			strings.TrimSpace(e.DirectID) != "" ||
			strings.TrimSpace(e.WorkID) != "" {
			return fmt.Errorf("store: network %s message cannot carry conversation or work fields", e.Kind)
		}
		return nil
	case NetworkKindSay:
		if err := (NetworkConversationRef{
			WorkspaceID: e.WorkspaceID,
			Channel:     e.Channel,
			Surface:     e.Surface,
			ThreadID:    e.ThreadID,
			DirectID:    e.DirectID,
		}).Validate(); err != nil {
			return err
		}
		if strings.TrimSpace(e.WorkID) != "" {
			return validateNetworkConversationID(e.WorkID, "work_id")
		}
		return nil
	case NetworkKindCapability, NetworkKindReceipt, NetworkKindTrace:
		if err := (NetworkConversationRef{
			WorkspaceID: e.WorkspaceID,
			Channel:     e.Channel,
			Surface:     e.Surface,
			ThreadID:    e.ThreadID,
			DirectID:    e.DirectID,
		}).Validate(); err != nil {
			return err
		}
		if err := validateNetworkConversationID(e.WorkID, "work_id"); err != nil {
			return err
		}
		return nil
	default:
		return validateNetworkMessageKind(e.Kind)
	}
}

// NetworkConversationWriteResult reports the transactional write outcome.
type NetworkConversationWriteResult struct {
	MessageID          string
	Duplicate          bool
	ConversationOpened bool
	WorkOpened         bool
	WorkTransitioned   bool
	WorkState          string
	LastActivityAt     time.Time
}

// NetworkTaskThreadOrigin links a promoted task back to its source thread.

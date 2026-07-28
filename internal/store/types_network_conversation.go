package store

import (
	"fmt"
	"strings"
	"time"
)

// NetworkConversationRef identifies one persisted network conversation container.
type NetworkConversationRef struct {
	WorkspaceID string
	Channel     string
	Surface     string
	ThreadID    string
	DirectID    string
}

// Validate ensures the reference identifies exactly one supported container.
func (r NetworkConversationRef) Validate() error {
	if err := (NetworkChannelRef{WorkspaceID: r.WorkspaceID, Channel: r.Channel}).Validate(); err != nil {
		return err
	}
	surface := strings.TrimSpace(r.Surface)
	switch surface {
	case NetworkSurfaceThread:
		if err := validateNetworkConversationID(r.ThreadID, typesThreadIDKey); err != nil {
			return err
		}
		if strings.TrimSpace(r.DirectID) != "" {
			return fmt.Errorf("store: network conversation direct_id must be empty for surface %q", surface)
		}
	case NetworkSurfaceDirect:
		if err := validateNetworkConversationID(r.DirectID, typesDirectIDKey); err != nil {
			return err
		}
		if strings.TrimSpace(r.ThreadID) != "" {
			return fmt.Errorf("store: network conversation thread_id must be empty for surface %q", surface)
		}
	default:
		return fmt.Errorf(
			"store: network conversation surface must be one of %q or %q: %q",
			NetworkSurfaceThread,
			NetworkSurfaceDirect,
			r.Surface,
		)
	}
	return nil
}

// ContainerID returns the thread or direct-room id selected by the surface.
func (r NetworkConversationRef) ContainerID() string {
	switch strings.TrimSpace(r.Surface) {
	case NetworkSurfaceThread:
		return strings.TrimSpace(r.ThreadID)
	case NetworkSurfaceDirect:
		return strings.TrimSpace(r.DirectID)
	default:
		return ""
	}
}

// NetworkThreadParticipant is one session recorded as present in a public thread.
type NetworkThreadParticipant struct {
	WorkspaceID    string
	Channel        string
	ThreadID       string
	SessionID      string
	FirstMessageID string
	FirstSeenAt    time.Time
	LastSeenAt     time.Time
}

// Validate ensures the participant belongs to one concrete public thread.
func (p NetworkThreadParticipant) Validate() error {
	ref := NetworkConversationRef{
		WorkspaceID: p.WorkspaceID,
		Channel:     p.Channel,
		Surface:     NetworkSurfaceThread,
		ThreadID:    p.ThreadID,
	}
	if err := ref.Validate(); err != nil {
		return err
	}
	if err := requireField(p.SessionID, "network thread participant session_id"); err != nil {
		return err
	}
	if err := requireField(p.FirstMessageID, "network thread participant first_message_id"); err != nil {
		return err
	}
	if p.FirstSeenAt.IsZero() {
		return fmt.Errorf("store: network thread participant first_seen_at is required")
	}
	if p.LastSeenAt.IsZero() {
		return fmt.Errorf("store: network thread participant last_seen_at is required")
	}
	return nil
}

// NetworkThreadSessionTokenStats aggregates delivered prompt cost by thread session.
type NetworkThreadSessionTokenStats struct {
	WorkspaceID           string
	Channel               string
	ThreadID              string
	SessionID             string
	DeliveredCount        int64
	PromptSizeBytes       int64
	EstimatedPromptTokens int64
	FirstDeliveredAt      time.Time
	LastDeliveredAt       time.Time
	UpdatedAt             time.Time
}

// NetworkThreadSessionTokenStatsUpdate increments one thread/session prompt-cost row.
type NetworkThreadSessionTokenStatsUpdate struct {
	WorkspaceID           string
	Channel               string
	ThreadID              string
	SessionID             string
	DeliveredCount        int64
	PromptSizeBytes       int64
	EstimatedPromptTokens int64
	DeliveredAt           time.Time
	UpdatedAt             time.Time
}

// Validate ensures the prompt-cost update is scoped and additive.
func (u NetworkThreadSessionTokenStatsUpdate) Validate() error {
	ref := NetworkConversationRef{
		WorkspaceID: u.WorkspaceID,
		Channel:     u.Channel,
		Surface:     NetworkSurfaceThread,
		ThreadID:    u.ThreadID,
	}
	if err := ref.Validate(); err != nil {
		return err
	}
	if err := requireField(u.SessionID, "network thread session token stats session_id"); err != nil {
		return err
	}
	if u.DeliveredCount < 0 {
		return fmt.Errorf(
			"store: network thread session token stats delivered_count must be zero or positive: %d",
			u.DeliveredCount,
		)
	}
	if u.PromptSizeBytes < 0 {
		return fmt.Errorf(
			"store: network thread session token stats prompt_size_bytes must be zero or positive: %d",
			u.PromptSizeBytes,
		)
	}
	if u.EstimatedPromptTokens < 0 {
		return fmt.Errorf(
			"store: network thread session token stats estimated_prompt_tokens must be zero or positive: %d",
			u.EstimatedPromptTokens,
		)
	}
	return nil
}

// NetworkThreadSessionTokenStatsQuery filters thread/session prompt-cost aggregates.
type NetworkThreadSessionTokenStatsQuery struct {
	WorkspaceID string
	Channel     string
	ThreadID    string
	SessionID   string
	Limit       int
}

// Validate ensures aggregate reads cannot cross workspace/thread boundaries.
func (q NetworkThreadSessionTokenStatsQuery) Validate() error {
	ref := NetworkConversationRef{
		WorkspaceID: q.WorkspaceID,
		Channel:     q.Channel,
		Surface:     NetworkSurfaceThread,
		ThreadID:    q.ThreadID,
	}
	if err := ref.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(q.SessionID) != "" {
		if err := requireField(q.SessionID, "network thread session token stats session_id"); err != nil {
			return err
		}
	}
	return requirePositiveLimit(q.Limit, "network thread session token stats limit")
}

// Validate ensures the public-thread summary is internally consistent.
func (s NetworkThreadSummary) Validate() error {
	ref := NetworkConversationRef{
		WorkspaceID: s.WorkspaceID,
		Channel:     s.Channel,
		Surface:     NetworkSurfaceThread,
		ThreadID:    s.ThreadID,
	}
	if err := ref.Validate(); err != nil {
		return err
	}
	if err := requireField(s.RootMessageID, "network thread root message id"); err != nil {
		return err
	}
	if err := requireField(s.OpenedByPeerID, "network thread opened_by_peer_id"); err != nil {
		return err
	}
	if s.OpenedAt.IsZero() {
		return fmt.Errorf("store: network thread opened_at is required")
	}
	if s.LastActivityAt.IsZero() {
		return fmt.Errorf("store: network thread last_activity_at is required")
	}
	if s.OpenedSequence < 0 || s.LastActivitySequence < 0 {
		return fmt.Errorf("store: network thread sequences must be zero or positive")
	}
	if s.MessageCount < 0 {
		return fmt.Errorf("store: network thread message_count must be zero or positive: %d", s.MessageCount)
	}
	if s.ParticipantCount < 0 {
		return fmt.Errorf("store: network thread participant_count must be zero or positive: %d", s.ParticipantCount)
	}
	if s.OpenWorkCount < 0 {
		return fmt.Errorf("store: network thread open_work_count must be zero or positive: %d", s.OpenWorkCount)
	}
	if s.DeliveredCount < 0 {
		return fmt.Errorf("store: network thread delivered_count must be zero or positive: %d", s.DeliveredCount)
	}
	if s.PromptSizeBytes < 0 {
		return fmt.Errorf("store: network thread prompt_size_bytes must be zero or positive: %d", s.PromptSizeBytes)
	}
	if s.EstimatedPromptTokens < 0 {
		return fmt.Errorf(
			"store: network thread estimated_prompt_tokens must be zero or positive: %d",
			s.EstimatedPromptTokens,
		)
	}
	return nil
}

// Validate ensures the direct-room summary is internally consistent.
func (s NetworkDirectRoomSummary) Validate() error {
	if err := validateNetworkDirectRoom(s.WorkspaceID, s.Channel, s.DirectID, s.SessionA, s.SessionB); err != nil {
		return err
	}
	if s.OpenedAt.IsZero() {
		return fmt.Errorf("store: network direct room opened_at is required")
	}
	if s.LastActivityAt.IsZero() {
		return fmt.Errorf("store: network direct room last_activity_at is required")
	}
	if s.OpenedSequence < 0 || s.LastActivitySequence < 0 {
		return fmt.Errorf("store: network direct room sequences must be zero or positive")
	}
	if s.MessageCount < 0 {
		return fmt.Errorf("store: network direct room message_count must be zero or positive: %d", s.MessageCount)
	}
	if s.OpenWorkCount < 0 {
		return fmt.Errorf("store: network direct room open_work_count must be zero or positive: %d", s.OpenWorkCount)
	}
	return nil
}

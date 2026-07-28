package store

import (
	"fmt"
	"strings"
	"time"
)

// NetworkChannelProjection is the bounded read model for one channel's
// persisted timeline activity.
type NetworkChannelProjection struct {
	WorkspaceID                string
	Channel                    string
	MessageCount               int
	PresenceCount              int
	HistoricalParticipantCount int
	LastActivityAt             *time.Time
	LastActivitySequence       int64
	LastPresenceAt             *time.Time
	LastPresenceSequence       int64
	LastMessageSequence        int64
	LastMessagePreview         string
}

// Validate ensures the projection remains workspace-qualified and additive.
func (p NetworkChannelProjection) Validate() error {
	if err := (NetworkChannelRef{WorkspaceID: p.WorkspaceID, Channel: p.Channel}).Validate(); err != nil {
		return err
	}
	if p.MessageCount < 0 || p.PresenceCount < 0 || p.HistoricalParticipantCount < 0 {
		return fmt.Errorf("store: network channel projection counts must be zero or positive")
	}
	if p.LastActivitySequence < 0 || p.LastPresenceSequence < 0 || p.LastMessageSequence < 0 {
		return fmt.Errorf("store: network channel projection sequences must be zero or positive")
	}
	return nil
}

// NetworkChannelKindCount is one materialized public-message kind total.
type NetworkChannelKindCount struct {
	WorkspaceID string
	Channel     string
	Kind        string
	Count       int
}

// NetworkChannelProjectionQuery scopes bounded projection reads.
type NetworkChannelProjectionQuery struct {
	WorkspaceID string
	Channel     string
	Limit       int
}

// Validate ensures projection reads cannot cross a workspace.
func (q NetworkChannelProjectionQuery) Validate() error {
	if err := requireField(q.WorkspaceID, "network channel projection query workspace_id"); err != nil {
		return err
	}
	if strings.TrimSpace(q.Channel) != "" {
		if err := requireField(q.Channel, "network channel projection query channel"); err != nil {
			return err
		}
	}
	return requirePositiveLimit(q.Limit, "network channel projection limit")
}

// NetworkRecentSummary is one cross-channel thread or direct-room list item.
type NetworkRecentSummary struct {
	WorkspaceID          string
	Channel              string
	Surface              string
	ContainerID          string
	LastActivityAt       time.Time
	LastActivitySequence int64
	LastMessagePreview   string
	Title                string
	ParticipantCount     int
	SessionA             string
	SessionB             string
}

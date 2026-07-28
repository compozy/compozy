package store

import (
	"fmt"
	"strings"
	"time"
)

// NetworkSubscriptionRef identifies one session's channel or thread delivery mode.
type NetworkSubscriptionRef struct {
	WorkspaceID string
	Channel     string
	ThreadID    string
	SessionID   string
}

// Validate ensures the subscription target is workspace-qualified.
func (r NetworkSubscriptionRef) Validate() error {
	if err := (NetworkChannelRef{WorkspaceID: r.WorkspaceID, Channel: r.Channel}).Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.ThreadID) != "" {
		if err := validateNetworkConversationID(r.ThreadID, typesThreadIDKey); err != nil {
			return err
		}
	}
	return requireField(r.SessionID, "network subscription session_id")
}

// NetworkSubscriptionEntry stores one session delivery preference.
type NetworkSubscriptionEntry struct {
	WorkspaceID string
	Channel     string
	ThreadID    string
	SessionID   string
	Mode        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Validate ensures one subscription row is usable by zero-token routing.
func (e NetworkSubscriptionEntry) Validate() error {
	if err := (NetworkSubscriptionRef{
		WorkspaceID: e.WorkspaceID,
		Channel:     e.Channel,
		ThreadID:    e.ThreadID,
		SessionID:   e.SessionID,
	}).Validate(); err != nil {
		return err
	}
	return ValidateNetworkSubscriptionMode(e.Mode)
}

// NetworkSubscriptionQuery filters session delivery preferences.
type NetworkSubscriptionQuery struct {
	WorkspaceID string
	Channel     string
	ThreadID    string
	SessionID   string
	ExactThread bool
	Limit       int
}

// Validate ensures subscription reads remain workspace-scoped.
func (q NetworkSubscriptionQuery) Validate() error {
	if err := (NetworkChannelRef{WorkspaceID: q.WorkspaceID, Channel: q.Channel}).Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(q.ThreadID) != "" {
		if err := validateNetworkConversationID(q.ThreadID, typesThreadIDKey); err != nil {
			return err
		}
	}
	if strings.TrimSpace(q.SessionID) != "" {
		if err := requireField(q.SessionID, "network subscription session_id"); err != nil {
			return err
		}
	}
	return requirePositiveLimit(q.Limit, "network subscription limit")
}

// ValidateNetworkSubscriptionMode checks one delivery preference mode.
func ValidateNetworkSubscriptionMode(mode string) error {
	switch strings.TrimSpace(mode) {
	case NetworkSubscriptionModeMute, NetworkSubscriptionModeFull:
		return nil
	default:
		return fmt.Errorf("store: unsupported network subscription mode %q", mode)
	}
}

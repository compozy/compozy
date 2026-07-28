package store

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type NetworkAuditEntry struct {
	ID          string
	SessionID   string
	Direction   string
	Kind        string
	WorkspaceID string
	Channel     string
	Surface     string
	ThreadID    string
	DirectID    string
	WorkID      string
	PeerFrom    string
	PeerTo      string
	MessageID   string
	Reason      string
	Size        int
	Timestamp   time.Time
}

// Validate ensures the network audit entry is complete and internally consistent.
func (e NetworkAuditEntry) Validate() error {
	if err := requireField(e.SessionID, "network audit session id"); err != nil {
		return err
	}
	if err := requireField(e.Direction, "network audit direction"); err != nil {
		return err
	}
	direction := strings.TrimSpace(e.Direction)
	switch direction {
	case typesSentKey, "received", typesRejectedKey, "delivered":
	default:
		return fmt.Errorf(
			"store: network audit direction must be one of %q, %q, %q, %q: %q",
			typesSentKey,
			"received",
			typesRejectedKey,
			"delivered",
			e.Direction,
		)
	}
	if direction != e.Direction {
		return fmt.Errorf("store: network audit direction must not contain surrounding whitespace: %q", e.Direction)
	}
	if err := requireField(e.Kind, "network audit kind"); err != nil {
		return err
	}
	if err := (NetworkChannelRef{WorkspaceID: e.WorkspaceID, Channel: e.Channel}).Validate(); err != nil {
		return err
	}
	if err := requireField(e.PeerFrom, "network audit peer_from"); err != nil {
		return err
	}
	if err := validateOptionalNetworkConversation(
		e.WorkspaceID,
		e.Channel,
		e.Surface,
		e.ThreadID,
		e.DirectID,
		"network audit conversation",
	); err != nil {
		return err
	}
	if strings.TrimSpace(e.WorkID) != "" {
		if err := validateNetworkConversationID(e.WorkID, "work_id"); err != nil {
			return err
		}
	}
	if err := requireField(e.MessageID, "network audit message id"); err != nil {
		return err
	}
	if e.Size < 0 {
		return fmt.Errorf("store: network audit size must be zero or positive: %d", e.Size)
	}
	if direction == typesRejectedKey && strings.TrimSpace(e.Reason) == "" {
		return fmt.Errorf("store: network audit reason is required when direction is %q", e.Direction)
	}
	if networkAuditEntryContainsRawClaimToken(e) {
		return fmt.Errorf("store: network audit entry contains raw claim_token material")
	}
	return nil
}

// NetworkAuditQuery filters network audit lookups.
type NetworkAuditQuery struct {
	SessionID   string
	WorkspaceID string
	// Global explicitly allows daemon-admin aggregate callers to scan audit rows
	// across workspaces. Workspace-scoped API surfaces must leave this false.
	Global    bool
	Direction string
	Kind      string
	Channel   string
	Surface   string
	ThreadID  string
	DirectID  string
	WorkID    string
	MessageID string
	Since     time.Time
	Limit     int
}

// Validate ensures the query uses sane bounds.
func (q NetworkAuditQuery) Validate() error {
	workspaceID := strings.TrimSpace(q.WorkspaceID)
	if q.Global && workspaceID != "" {
		return errors.New("store: network audit query cannot combine global scan with workspace_id")
	}
	if !q.Global {
		if err := requireField(workspaceID, "network audit query workspace_id"); err != nil {
			return err
		}
	}
	return requirePositiveLimit(q.Limit, "network audit limit")
}

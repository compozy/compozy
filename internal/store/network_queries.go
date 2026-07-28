package store

import (
	"fmt"
	"strings"
	"time"
)

const (
	NetworkConversationSortRecentActivity = "recent_activity"
	NetworkConversationSortCreated        = "created"
	NetworkConversationSortAlphabetical   = "alphabetical"

	NetworkConversationListDefaultLimit = 50
	NetworkConversationListMaxLimit     = 200
	NetworkMessageDefaultLimit          = 120
	NetworkMessageMaxLimit              = 200
)

// NetworkMessageQuery filters persisted network timeline lookups.
type NetworkMessageQuery struct {
	SessionID       string
	WorkspaceID     string
	Channel         string
	PeerID          string
	PeerFrom        string
	PeerTo          string
	Kind            string
	Direction       string
	MessageID       string
	BeforeMessageID string
	AfterMessageID  string
	DirectedOnly    bool
	PublicOnly      bool
	IncludePresence bool
	Since           time.Time
	Limit           int
}

// Validate ensures the query uses sane bounds.
func (q NetworkMessageQuery) Validate() error {
	if err := requireField(q.WorkspaceID, "network message query workspace_id"); err != nil {
		return err
	}
	if strings.TrimSpace(q.BeforeMessageID) != "" && strings.TrimSpace(q.AfterMessageID) != "" {
		return fmt.Errorf("store: network message query cannot specify both before and after cursors")
	}
	if q.PublicOnly && q.DirectedOnly {
		return fmt.Errorf("store: network message query cannot combine public_only with directed_only")
	}
	return requirePositiveLimit(q.Limit, "network message limit")
}

// NetworkThreadQuery filters and orders public-thread summary lookups.
type NetworkThreadQuery struct {
	Search    string
	SessionID string
	Sort      string
	HasWork   *bool
	Limit     int
	After     string
}

// NetworkThreadPage is one bounded, counted page of public-thread summaries.
type NetworkThreadPage struct {
	Threads    []NetworkThreadSummary
	NextCursor string
	HasMore    bool
	Total      int
	Limit      int
}

// Validate ensures the query uses a supported order and sane bounds.
func (q NetworkThreadQuery) Validate() error {
	if err := validateNetworkConversationSort(q.Sort); err != nil {
		return err
	}
	if sessionID := strings.TrimSpace(q.SessionID); sessionID != "" {
		if err := requireField(sessionID, "network thread query session_id"); err != nil {
			return err
		}
	}
	return requirePositiveLimit(q.Limit, "network thread limit")
}

// Normalize applies canonical whitespace, sort, and server-owned bounds.
func (q NetworkThreadQuery) Normalize() NetworkThreadQuery {
	q.Search = strings.ToLower(strings.TrimSpace(q.Search))
	q.SessionID = strings.TrimSpace(q.SessionID)
	q.Sort = NormalizeNetworkConversationSort(q.Sort)
	q.After = strings.TrimSpace(q.After)
	q.Limit = NormalizeNetworkConversationListLimit(q.Limit)
	return q
}

// NetworkDirectRoomQuery filters and orders direct-room summary lookups.
type NetworkDirectRoomQuery struct {
	Search    string
	SessionID string
	Sort      string
	HasWork   *bool
	Limit     int
	After     string
}

// NetworkDirectRoomPage is one bounded, counted page of direct-room summaries.
type NetworkDirectRoomPage struct {
	Directs    []NetworkDirectRoomSummary
	NextCursor string
	HasMore    bool
	Total      int
	Limit      int
}

// Validate ensures the query uses a supported order and sane bounds.
func (q NetworkDirectRoomQuery) Validate() error {
	if err := validateNetworkConversationSort(q.Sort); err != nil {
		return err
	}
	if sessionID := strings.TrimSpace(q.SessionID); sessionID != "" {
		if err := requireField(sessionID, "network direct room query session_id"); err != nil {
			return err
		}
	}
	return requirePositiveLimit(q.Limit, "network direct room limit")
}

// Normalize applies canonical whitespace, sort, and server-owned bounds.
func (q NetworkDirectRoomQuery) Normalize() NetworkDirectRoomQuery {
	q.Search = strings.ToLower(strings.TrimSpace(q.Search))
	q.SessionID = strings.TrimSpace(q.SessionID)
	q.Sort = NormalizeNetworkConversationSort(q.Sort)
	q.After = strings.TrimSpace(q.After)
	q.Limit = NormalizeNetworkConversationListLimit(q.Limit)
	return q
}

// NetworkConversationMessageQuery filters conversation message lookups.
type NetworkConversationMessageQuery struct {
	BeforeMessageID string
	AfterMessageID  string
	Kind            string
	WorkID          string
	Limit           int
}

// Validate ensures the query uses sane bounds and one cursor direction.
func (q NetworkConversationMessageQuery) Validate() error {
	if strings.TrimSpace(q.BeforeMessageID) != "" && strings.TrimSpace(q.AfterMessageID) != "" {
		return fmt.Errorf("store: network conversation message query cannot specify both before and after cursors")
	}
	if strings.TrimSpace(q.Kind) != "" {
		if err := validateNetworkMessageKind(q.Kind); err != nil {
			return err
		}
	}
	if strings.TrimSpace(q.WorkID) != "" {
		if err := validateNetworkConversationID(q.WorkID, "work_id"); err != nil {
			return err
		}
	}
	return requirePositiveLimit(q.Limit, "network conversation message limit")
}

// NetworkRecentQuery limits the cross-channel recent-conversation projection.
type NetworkRecentQuery struct {
	WorkspaceID string
	Limit       int
}

// Validate ensures recents never cross workspace boundaries.
func (q NetworkRecentQuery) Validate() error {
	if err := requireField(q.WorkspaceID, "network recent query workspace_id"); err != nil {
		return err
	}
	return requirePositiveLimit(q.Limit, "network recent limit")
}

// NormalizeNetworkConversationSort applies the default recent-activity order.
func NormalizeNetworkConversationSort(sort string) string {
	if trimmed := strings.TrimSpace(sort); trimmed != "" {
		return trimmed
	}
	return NetworkConversationSortRecentActivity
}

// NormalizeNetworkConversationListLimit applies the bounded catalog contract.
func NormalizeNetworkConversationListLimit(limit int) int {
	return normalizeNetworkLimit(limit, NetworkConversationListDefaultLimit, NetworkConversationListMaxLimit)
}

// NormalizeNetworkMessageLimit applies the bounded message-history contract.
func NormalizeNetworkMessageLimit(limit int) int {
	return normalizeNetworkLimit(limit, NetworkMessageDefaultLimit, NetworkMessageMaxLimit)
}

func normalizeNetworkLimit(limit int, defaultLimit int, maxLimit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func validateNetworkConversationSort(sort string) error {
	switch NormalizeNetworkConversationSort(sort) {
	case NetworkConversationSortRecentActivity,
		NetworkConversationSortCreated,
		NetworkConversationSortAlphabetical:
		return nil
	default:
		return fmt.Errorf("store: unsupported network conversation sort %q", sort)
	}
}

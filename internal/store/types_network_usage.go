package store

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/listcursor"
	"github.com/compozy/agh/internal/network/participation"
)

const (
	NetworkWakeUsageReserved       = "reserved"
	DefaultNetworkUsageLimit       = 50
	MaxNetworkUsageLimit           = 200
	MaxJavaScriptSafeInteger int64 = 9_007_199_254_740_991

	networkUsageCursorVersion = 1
	networkUsageCursorKind    = "network_usage"
)

var (
	ErrNetworkUsageQueryInvalid  = errors.New("network_usage_query_invalid")
	ErrNetworkUsageLimitInvalid  = errors.New("network_usage_limit_invalid")
	ErrNetworkUsageCursorInvalid = errors.New("network_usage_cursor_invalid")
	ErrNetworkUsageUnsafeInteger = errors.New("network_usage_unsafe_integer")
)

// NetworkUsageQuery scopes wake accounting to one authenticated workspace.
type NetworkUsageQuery struct {
	WorkspaceID string
	Owner       *participation.OwnerRef
	Channel     string
	RunID       string
	Cursor      string
	Limit       int
	LimitSet    bool
}

// NetworkUsageCursorPosition is the stable descending keyset position of one page.
type NetworkUsageCursorPosition struct {
	ReservedAt time.Time `json:"reserved_at"`
	WakeID     string    `json:"wake_id"`
}

type networkUsageCursorFingerprint struct {
	WorkspaceID string                  `json:"workspace_id"`
	Owner       *participation.OwnerRef `json:"owner,omitempty"`
	Channel     string                  `json:"channel,omitempty"`
	RunID       string                  `json:"run_id,omitempty"`
}

// NormalizeNetworkUsageQuery validates filters, applies defaults, and decodes the cursor.
func NormalizeNetworkUsageQuery(
	query NetworkUsageQuery,
) (NetworkUsageQuery, *NetworkUsageCursorPosition, error) {
	normalized := NetworkUsageQuery{
		WorkspaceID: strings.TrimSpace(query.WorkspaceID),
		Channel:     strings.TrimSpace(query.Channel),
		RunID:       strings.TrimSpace(query.RunID),
		Cursor:      strings.TrimSpace(query.Cursor),
		Limit:       query.Limit,
		LimitSet:    query.LimitSet,
	}
	if normalized.WorkspaceID == "" {
		return NetworkUsageQuery{}, nil, fmt.Errorf(
			"%w: workspace_id is required",
			ErrNetworkUsageQueryInvalid,
		)
	}
	if query.Owner != nil {
		owner := participation.OwnerRef{
			WorkspaceID: strings.TrimSpace(query.Owner.WorkspaceID),
			Kind:        participation.OwnerKind(strings.TrimSpace(string(query.Owner.Kind))),
			ID:          strings.TrimSpace(query.Owner.ID),
		}
		if err := participation.ValidateOwner(owner); err != nil {
			return NetworkUsageQuery{}, nil, fmt.Errorf(
				"%w: validate owner: %w",
				ErrNetworkUsageQueryInvalid,
				err,
			)
		}
		if owner.WorkspaceID != normalized.WorkspaceID {
			return NetworkUsageQuery{}, nil, fmt.Errorf(
				"%w: owner workspace_id %q does not match query workspace_id %q",
				ErrNetworkUsageQueryInvalid,
				owner.WorkspaceID,
				normalized.WorkspaceID,
			)
		}
		normalized.Owner = &owner
	}
	if normalized.Owner != nil && normalized.RunID != "" {
		return NetworkUsageQuery{}, nil, fmt.Errorf(
			"%w: owner and run_id are mutually exclusive",
			ErrNetworkUsageQueryInvalid,
		)
	}
	if normalized.Limit == 0 && !normalized.LimitSet {
		normalized.Limit = DefaultNetworkUsageLimit
	}
	if normalized.Limit < 1 || normalized.Limit > MaxNetworkUsageLimit {
		return NetworkUsageQuery{}, nil, fmt.Errorf(
			"%w: limit must be between 1 and %d",
			ErrNetworkUsageLimitInvalid,
			MaxNetworkUsageLimit,
		)
	}
	if normalized.Cursor == "" {
		return normalized, nil, nil
	}
	position, err := decodeNetworkUsageCursor(normalized)
	if err != nil {
		return NetworkUsageQuery{}, nil, err
	}
	return normalized, &position, nil
}

// EncodeNetworkUsageCursor binds one keyset position to the normalized filter set.
func EncodeNetworkUsageCursor(
	query NetworkUsageQuery,
	position NetworkUsageCursorPosition,
) (string, error) {
	query.Cursor = ""
	normalized, _, err := NormalizeNetworkUsageQuery(query)
	if err != nil {
		return "", err
	}
	position.ReservedAt = position.ReservedAt.UTC()
	position.WakeID = strings.TrimSpace(position.WakeID)
	if position.ReservedAt.IsZero() || position.WakeID == "" {
		return "", fmt.Errorf(
			"%w: reserved_at and wake_id are required",
			ErrNetworkUsageCursorInvalid,
		)
	}
	fingerprint, err := networkUsageQueryFingerprint(normalized)
	if err != nil {
		return "", err
	}
	encoded, err := listcursor.Encode(
		networkUsageCursorVersion,
		networkUsageCursorKind,
		fingerprint,
		position,
	)
	if err != nil {
		return "", fmt.Errorf("encode network usage cursor: %w", err)
	}
	return encoded, nil
}

func decodeNetworkUsageCursor(query NetworkUsageQuery) (NetworkUsageCursorPosition, error) {
	fingerprint, err := networkUsageQueryFingerprint(query)
	if err != nil {
		return NetworkUsageCursorPosition{}, err
	}
	position, err := listcursor.Decode[NetworkUsageCursorPosition](
		query.Cursor,
		networkUsageCursorVersion,
		networkUsageCursorKind,
		fingerprint,
		listcursor.DefaultMaxEncodedSize,
	)
	if err != nil {
		return NetworkUsageCursorPosition{}, fmt.Errorf(
			"%w: %w",
			ErrNetworkUsageCursorInvalid,
			err,
		)
	}
	position.ReservedAt = position.ReservedAt.UTC()
	position.WakeID = strings.TrimSpace(position.WakeID)
	if position.ReservedAt.IsZero() || position.WakeID == "" {
		return NetworkUsageCursorPosition{}, fmt.Errorf(
			"%w: reserved_at and wake_id are required",
			ErrNetworkUsageCursorInvalid,
		)
	}
	return position, nil
}

func networkUsageQueryFingerprint(query NetworkUsageQuery) (string, error) {
	fingerprint, err := listcursor.Fingerprint(networkUsageCursorFingerprint{
		WorkspaceID: query.WorkspaceID,
		Owner:       query.Owner,
		Channel:     query.Channel,
		RunID:       query.RunID,
	})
	if err != nil {
		return "", fmt.Errorf("fingerprint network usage query: %w", err)
	}
	return fingerprint, nil
}

// ValidateJavaScriptSafeInteger rejects counters that cannot round-trip through JSON numbers.
func ValidateJavaScriptSafeInteger(field string, value int64) error {
	if value < 0 || value > MaxJavaScriptSafeInteger {
		return fmt.Errorf(
			"%w: %s must be between 0 and %d",
			ErrNetworkUsageUnsafeInteger,
			strings.TrimSpace(field),
			MaxJavaScriptSafeInteger,
		)
	}
	return nil
}

// NetworkWakeUsageDetail is the charged usage truth for one admitted wake.
type NetworkWakeUsageDetail struct {
	WakeID          string
	TaskRunID       string
	OwnerKey        string
	WorkspaceID     string
	Channel         string
	RootID          string
	Depth           int
	State           string
	UsageState      string
	ChargedWallTime time.Duration
	InputTokens     int64
	OutputTokens    int64
	ReservedAt      time.Time
	SettledAt       *time.Time
	Reason          string
}

// NetworkUsageSummary aggregates the complete filtered ledger scope.
type NetworkUsageSummary struct {
	WakeCount            int
	ReservedWakeCount    int
	ActualWakeCount      int
	UnavailableWakeCount int
	ChargedWallTime      time.Duration
	InputTokens          int64
	OutputTokens         int64
}

// NetworkBudgetUsage reports the durable consumption counters for one participation owner.
type NetworkBudgetUsage struct {
	OwnerKey         string
	WakesUsed        int
	WallTimeUsed     time.Duration
	InputTokensUsed  int64
	OutputTokensUsed int64
	ExhaustedReason  string
	UpdatedAt        time.Time
}

// NetworkUsageReport pairs a bounded page with full-filter totals and budget truth.
type NetworkUsageReport struct {
	Details    []NetworkWakeUsageDetail
	Total      NetworkUsageSummary
	Budget     *NetworkBudgetUsage
	NextCursor string
}

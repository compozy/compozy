package bridges

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/notifications"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	// BridgeTaskSubscriptionConsumerPrefix namespaces bridge task-delivery cursors.
	BridgeTaskSubscriptionConsumerPrefix = "bridge_task_subscription:"
	// BridgeTaskNotificationStream is the durable task event stream consumed by subscriptions.
	BridgeTaskNotificationStream = "task_events"
)

var (
	// ErrBridgeTaskSubscriptionNotFound reports that no bridge task subscription matched the lookup.
	ErrBridgeTaskSubscriptionNotFound = errors.New("bridges: bridge task subscription not found")
	// ErrInvalidBridgeTaskSubscription reports malformed bridge task subscription data.
	ErrInvalidBridgeTaskSubscription = errors.New("bridges: invalid bridge task subscription")
)

// BridgeTaskSubscription stores one bridge terminal-notification target for one task.
type BridgeTaskSubscription struct {
	SubscriptionID   string                `json:"subscription_id"`
	TaskID           string                `json:"task_id"`
	BridgeInstanceID string                `json:"bridge_instance_id"`
	Scope            Scope                 `json:"scope"`
	WorkspaceID      string                `json:"workspace_id,omitempty"`
	PeerID           string                `json:"peer_id,omitempty"`
	ThreadID         string                `json:"thread_id,omitempty"`
	GroupID          string                `json:"group_id,omitempty"`
	DeliveryMode     DeliveryMode          `json:"delivery_mode"`
	CreatedBy        taskpkg.ActorIdentity `json:"created_by"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

// BridgeTaskSubscriptionQuery filters persisted bridge task subscriptions.
type BridgeTaskSubscriptionQuery struct {
	TaskID           string `json:"task_id,omitempty"`
	BridgeInstanceID string `json:"bridge_instance_id,omitempty"`
	Scope            Scope  `json:"scope,omitempty"`
	WorkspaceID      string `json:"workspace_id,omitempty"`
	Limit            int    `json:"limit,omitempty"`
}

// BridgeTaskSubscriptionStore persists bridge task subscription targets.
type BridgeTaskSubscriptionStore interface {
	PutBridgeTaskSubscription(ctx context.Context, subscription BridgeTaskSubscription) error
	GetBridgeTaskSubscription(ctx context.Context, subscriptionID string) (BridgeTaskSubscription, error)
	ListBridgeTaskSubscriptions(
		ctx context.Context,
		query BridgeTaskSubscriptionQuery,
	) ([]BridgeTaskSubscription, error)
	DeleteBridgeTaskSubscription(ctx context.Context, subscriptionID string) error
}

// TerminalTaskNotification is the bridge-delivered accepted-final task envelope.
type TerminalTaskNotification struct {
	DeliveryID     string          `json:"delivery_id"`
	EventType      string          `json:"event_type"`
	Final          bool            `json:"final"`
	Seq            int64           `json:"seq"`
	TaskID         string          `json:"task_id"`
	RunID          string          `json:"run_id,omitempty"`
	Status         taskpkg.Status  `json:"status"`
	Summary        string          `json:"summary,omitempty"`
	Error          string          `json:"error,omitempty"`
	Payload        json.RawMessage `json:"payload,omitempty"`
	SubscriptionID string          `json:"subscription_id"`
}

// Validate reports whether a subscription contains a valid task delivery target.
func (s BridgeTaskSubscription) Validate() error {
	normalized := s.Normalize()
	if err := requireField(normalized.SubscriptionID, "bridge task subscription id"); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidBridgeTaskSubscription, err)
	}
	if err := requireField(normalized.TaskID, "bridge task subscription task id"); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidBridgeTaskSubscription, err)
	}
	if err := normalized.DeliveryTarget().Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidBridgeTaskSubscription, err)
	}
	if err := ValidateScopeWorkspaceID(normalized.Scope, normalized.WorkspaceID); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidBridgeTaskSubscription, err)
	}
	if err := normalized.CreatedBy.Validate("bridge_task_subscription.created_by"); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidBridgeTaskSubscription, err)
	}
	return nil
}

// Normalize returns the canonical subscription representation.
func (s BridgeTaskSubscription) Normalize() BridgeTaskSubscription {
	normalized := s
	normalized.SubscriptionID = strings.TrimSpace(normalized.SubscriptionID)
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.BridgeInstanceID = strings.TrimSpace(normalized.BridgeInstanceID)
	normalized.Scope = normalized.Scope.Normalize()
	normalized.WorkspaceID = strings.TrimSpace(normalized.WorkspaceID)
	normalized.PeerID = strings.TrimSpace(normalized.PeerID)
	normalized.ThreadID = strings.TrimSpace(normalized.ThreadID)
	normalized.GroupID = strings.TrimSpace(normalized.GroupID)
	normalized.DeliveryMode = normalized.DeliveryMode.Normalize()
	normalized.CreatedBy = taskpkg.ActorIdentity{
		Kind: normalized.CreatedBy.Kind.Normalize(),
		Ref:  strings.TrimSpace(normalized.CreatedBy.Ref),
	}
	return normalized
}

// CursorKey returns the fixed cursor identity for this subscription.
func (s BridgeTaskSubscription) CursorKey() notifications.CursorKey {
	normalized := s.Normalize()
	return notifications.CursorKey{
		ConsumerID: BridgeTaskSubscriptionConsumerPrefix + normalized.SubscriptionID,
		StreamName: BridgeTaskNotificationStream,
		SubjectID:  normalized.TaskID,
	}
}

// RoutingKey returns the subscription's bridge routing identity.
func (s BridgeTaskSubscription) RoutingKey() RoutingKey {
	normalized := s.Normalize()
	return RoutingKey{
		Scope:            normalized.Scope,
		WorkspaceID:      normalized.WorkspaceID,
		BridgeInstanceID: normalized.BridgeInstanceID,
		PeerID:           normalized.PeerID,
		ThreadID:         normalized.ThreadID,
		GroupID:          normalized.GroupID,
	}
}

// DeliveryTarget returns the outbound bridge delivery target for the subscription.
func (s BridgeTaskSubscription) DeliveryTarget() DeliveryTarget {
	normalized := s.Normalize()
	return DeliveryTarget{
		BridgeInstanceID: normalized.BridgeInstanceID,
		PeerID:           normalized.PeerID,
		ThreadID:         normalized.ThreadID,
		GroupID:          normalized.GroupID,
		Mode:             normalized.DeliveryMode,
	}
}

// Normalize trims subscription query filters.
func (q BridgeTaskSubscriptionQuery) Normalize() BridgeTaskSubscriptionQuery {
	normalized := BridgeTaskSubscriptionQuery{
		TaskID:           strings.TrimSpace(q.TaskID),
		BridgeInstanceID: strings.TrimSpace(q.BridgeInstanceID),
		Scope:            q.Scope.Normalize(),
		WorkspaceID:      strings.TrimSpace(q.WorkspaceID),
		Limit:            q.Limit,
	}
	if normalized.Limit < 0 {
		normalized.Limit = 0
	}
	return normalized
}

package bridges

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrDeliveryLedgerNotFound reports that no durable delivery matched a checkpoint.
	ErrDeliveryLedgerNotFound = errors.New("bridges: delivery ledger record not found")
	// ErrDeliveryLedgerConflict reports a duplicate identity or non-monotonic checkpoint.
	ErrDeliveryLedgerConflict = errors.New("bridges: delivery ledger conflict")
)

// DeliveryLedgerState is the durable lifecycle state of one bridge delivery.
type DeliveryLedgerState string

const (
	// DeliveryLedgerStateActive identifies a delivery that still needs reconciliation.
	DeliveryLedgerStateActive DeliveryLedgerState = "active"
	// DeliveryLedgerStateTerminalOK identifies a successfully completed delivery.
	DeliveryLedgerStateTerminalOK DeliveryLedgerState = "terminal_ok"
	// DeliveryLedgerStateTerminalError identifies a delivery completed with an error.
	DeliveryLedgerStateTerminalError DeliveryLedgerState = "terminal_error"
)

// DeliveryLedgerStateValues returns the closed durable state values in stable order.
func DeliveryLedgerStateValues() []string {
	return []string{
		string(DeliveryLedgerStateActive),
		string(DeliveryLedgerStateTerminalOK),
		string(DeliveryLedgerStateTerminalError),
	}
}

// Normalize returns the canonical delivery-ledger state.
func (s DeliveryLedgerState) Normalize() DeliveryLedgerState {
	return DeliveryLedgerState(strings.ToLower(strings.TrimSpace(string(s))))
}

// Validate reports whether the delivery-ledger state is supported.
func (s DeliveryLedgerState) Validate() error {
	switch s.Normalize() {
	case DeliveryLedgerStateActive, DeliveryLedgerStateTerminalOK, DeliveryLedgerStateTerminalError:
		return nil
	case "":
		return errors.New("bridges: delivery ledger state is required")
	default:
		return fmt.Errorf("bridges: unsupported delivery ledger state %q", s)
	}
}

// DeliveryLedgerRecord is the checkpoint-only durable identity and state of one delivery.
type DeliveryLedgerRecord struct {
	DeliveryID       string              `json:"delivery_id"`
	SessionID        string              `json:"session_id"`
	TurnID           string              `json:"turn_id"`
	RoutingKey       RoutingKey          `json:"routing_key"`
	BridgeInstanceID string              `json:"bridge_instance_id"`
	Scope            Scope               `json:"scope"`
	WorkspaceID      string              `json:"workspace_id,omitempty"`
	State            DeliveryLedgerState `json:"state"`
	LastSentSeq      int64               `json:"last_sent_seq"`
	LastAckedSeq     int64               `json:"last_acked_seq"`
	RemoteMessageID  string              `json:"remote_message_id,omitempty"`
	TerminalError    string              `json:"terminal_error,omitempty"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

// Canonicalize normalizes and validates a durable delivery record.
func (r DeliveryLedgerRecord) Canonicalize() (DeliveryLedgerRecord, error) {
	normalized := r
	normalized.DeliveryID = strings.TrimSpace(normalized.DeliveryID)
	normalized.SessionID = strings.TrimSpace(normalized.SessionID)
	normalized.TurnID = strings.TrimSpace(normalized.TurnID)
	normalized.RoutingKey = normalized.RoutingKey.normalize()
	normalized.BridgeInstanceID = strings.TrimSpace(normalized.BridgeInstanceID)
	normalized.Scope = normalized.Scope.Normalize()
	normalized.WorkspaceID = strings.TrimSpace(normalized.WorkspaceID)
	normalized.State = normalized.State.Normalize()
	normalized.RemoteMessageID = strings.TrimSpace(normalized.RemoteMessageID)
	normalized.TerminalError = strings.TrimSpace(normalized.TerminalError)
	normalized.CreatedAt = normalized.CreatedAt.UTC()
	normalized.UpdatedAt = normalized.UpdatedAt.UTC()
	if err := normalized.Validate(); err != nil {
		return DeliveryLedgerRecord{}, err
	}
	return normalized, nil
}

// Validate reports whether a durable delivery record is complete and internally consistent.
func (r DeliveryLedgerRecord) Validate() error {
	if err := requireField(r.DeliveryID, "delivery ledger id"); err != nil {
		return err
	}
	if err := requireField(r.SessionID, "delivery ledger session id"); err != nil {
		return err
	}
	if err := requireField(r.TurnID, "delivery ledger turn id"); err != nil {
		return err
	}
	if err := requireField(r.BridgeInstanceID, "delivery ledger bridge instance id"); err != nil {
		return err
	}
	if err := ValidateScopeWorkspaceID(r.Scope, r.WorkspaceID); err != nil {
		return err
	}
	if err := r.RoutingKey.Validate(); err != nil {
		return err
	}
	if r.RoutingKey.BridgeInstanceID != strings.TrimSpace(r.BridgeInstanceID) ||
		r.RoutingKey.Scope.Normalize() != r.Scope.Normalize() ||
		strings.TrimSpace(r.RoutingKey.WorkspaceID) != strings.TrimSpace(r.WorkspaceID) {
		return errors.New("bridges: delivery ledger route identity must match direct scope columns")
	}
	if err := validateDeliveryLedgerSequences(r.LastSentSeq, r.LastAckedSeq); err != nil {
		return err
	}
	if err := validateDeliveryLedgerTerminal(r.State, r.TerminalError); err != nil {
		return err
	}
	if r.CreatedAt.IsZero() {
		return errors.New("bridges: delivery ledger created at is required")
	}
	if r.UpdatedAt.IsZero() {
		return errors.New("bridges: delivery ledger updated at is required")
	}
	if r.UpdatedAt.Before(r.CreatedAt) {
		return errors.New("bridges: delivery ledger updated at cannot precede created at")
	}
	return nil
}

// DeliveryLedgerCheckpoint advances an existing durable delivery without reidentifying it.
type DeliveryLedgerCheckpoint struct {
	DeliveryID      string                `json:"delivery_id"`
	State           DeliveryLedgerState   `json:"state"`
	LastSentSeq     int64                 `json:"last_sent_seq"`
	LastAckedSeq    int64                 `json:"last_acked_seq"`
	RemoteMessageID string                `json:"remote_message_id,omitempty"`
	TerminalError   string                `json:"terminal_error,omitempty"`
	UpdatedAt       time.Time             `json:"updated_at"`
	Metrics         *DeliveryMetricRecord `json:"metrics,omitempty"`
}

// Canonicalize normalizes and validates one delivery checkpoint.
func (c DeliveryLedgerCheckpoint) Canonicalize() (DeliveryLedgerCheckpoint, error) {
	normalized := c
	normalized.DeliveryID = strings.TrimSpace(normalized.DeliveryID)
	normalized.State = normalized.State.Normalize()
	normalized.RemoteMessageID = strings.TrimSpace(normalized.RemoteMessageID)
	normalized.TerminalError = strings.TrimSpace(normalized.TerminalError)
	normalized.UpdatedAt = normalized.UpdatedAt.UTC()
	if normalized.Metrics != nil {
		metrics, err := normalized.Metrics.Canonicalize()
		if err != nil {
			return DeliveryLedgerCheckpoint{}, err
		}
		normalized.Metrics = &metrics
	}
	if err := normalized.Validate(); err != nil {
		return DeliveryLedgerCheckpoint{}, err
	}
	return normalized, nil
}

// Validate reports whether a checkpoint contains a valid monotonic target state.
func (c DeliveryLedgerCheckpoint) Validate() error {
	if err := requireField(c.DeliveryID, "delivery checkpoint id"); err != nil {
		return err
	}
	if err := validateDeliveryLedgerSequences(c.LastSentSeq, c.LastAckedSeq); err != nil {
		return err
	}
	if err := validateDeliveryLedgerTerminal(c.State, c.TerminalError); err != nil {
		return err
	}
	if c.UpdatedAt.IsZero() {
		return errors.New("bridges: delivery checkpoint updated at is required")
	}
	return nil
}

// DeliveryMetricRecord binds durable broker counters to one direct scope identity.
type DeliveryMetricRecord struct {
	BridgeDeliveryMetrics
	Scope       Scope     `json:"scope"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Canonicalize normalizes and validates one durable metric snapshot.
func (r DeliveryMetricRecord) Canonicalize() (DeliveryMetricRecord, error) {
	normalized := r
	normalized.BridgeInstanceID = strings.TrimSpace(normalized.BridgeInstanceID)
	normalized.Scope = normalized.Scope.Normalize()
	normalized.WorkspaceID = strings.TrimSpace(normalized.WorkspaceID)
	normalized.LastError = strings.TrimSpace(normalized.LastError)
	normalized.LastErrorAt = normalized.LastErrorAt.UTC()
	normalized.LastSuccessAt = normalized.LastSuccessAt.UTC()
	normalized.UpdatedAt = normalized.UpdatedAt.UTC()
	normalized.DeliveryDroppedByReason = normalizeDeliveryDropReasons(normalized.DeliveryDroppedByReason)
	if err := normalized.Validate(); err != nil {
		return DeliveryMetricRecord{}, err
	}
	return normalized, nil
}

// Validate reports whether durable delivery metrics are complete and internally consistent.
func (r DeliveryMetricRecord) Validate() error {
	if err := requireField(r.BridgeInstanceID, "delivery metric bridge instance id"); err != nil {
		return err
	}
	if err := ValidateScopeWorkspaceID(r.Scope, r.WorkspaceID); err != nil {
		return err
	}
	if r.DeliveryBacklog < 0 || r.DeliveryDroppedTotal < 0 || r.DeliveryFailuresTotal < 0 {
		return errors.New("bridges: delivery metric counters cannot be negative")
	}
	droppedTotal := 0
	for reason, count := range r.DeliveryDroppedByReason {
		if strings.TrimSpace(reason) == "" || count <= 0 {
			return errors.New("bridges: delivery drop reasons require a name and positive count")
		}
		droppedTotal += count
	}
	if droppedTotal != r.DeliveryDroppedTotal {
		return errors.New("bridges: delivery dropped total must equal the reason counters")
	}
	if r.LastError == "" && !r.LastErrorAt.IsZero() {
		return errors.New("bridges: delivery metric last error timestamp requires an error")
	}
	if r.LastError != "" && r.LastErrorAt.IsZero() {
		return errors.New("bridges: delivery metric last error requires a timestamp")
	}
	if r.UpdatedAt.IsZero() {
		return errors.New("bridges: delivery metric updated at is required")
	}
	return nil
}

// DeliveryLedgerQuery scopes ledger and metric reads without an instance join.
type DeliveryLedgerQuery struct {
	Scope            Scope               `json:"scope"`
	WorkspaceID      string              `json:"workspace_id,omitempty"`
	BridgeInstanceID string              `json:"bridge_instance_id,omitempty"`
	State            DeliveryLedgerState `json:"state,omitempty"`
	Limit            int                 `json:"limit,omitempty"`
}

// Canonicalize normalizes and validates a delivery-ledger query.
func (q DeliveryLedgerQuery) Canonicalize() (DeliveryLedgerQuery, error) {
	normalized := q
	normalized.Scope = normalized.Scope.Normalize()
	normalized.WorkspaceID = strings.TrimSpace(normalized.WorkspaceID)
	normalized.BridgeInstanceID = strings.TrimSpace(normalized.BridgeInstanceID)
	normalized.State = normalized.State.Normalize()
	if err := normalized.Validate(); err != nil {
		return DeliveryLedgerQuery{}, err
	}
	return normalized, nil
}

// Validate reports whether a query has one exact global or workspace scope.
func (q DeliveryLedgerQuery) Validate() error {
	if err := ValidateScopeWorkspaceID(q.Scope, q.WorkspaceID); err != nil {
		return err
	}
	if q.State.Normalize() != "" {
		if err := q.State.Validate(); err != nil {
			return err
		}
	}
	if q.Limit < 0 || q.Limit > 1000 {
		return fmt.Errorf("bridges: delivery ledger query limit %d must be between 0 and 1000", q.Limit)
	}
	return nil
}

// DeliveryLedgerStore persists checkpoint-only delivery state and durable counters.
type DeliveryLedgerStore interface {
	CreateBridgeDelivery(ctx context.Context, record DeliveryLedgerRecord) error
	CheckpointBridgeDelivery(ctx context.Context, checkpoint DeliveryLedgerCheckpoint) error
	ListBridgeDeliveries(ctx context.Context, query DeliveryLedgerQuery) ([]DeliveryLedgerRecord, error)
	ListBridgeDeliveryMetrics(ctx context.Context, query DeliveryLedgerQuery) (map[string]BridgeDeliveryMetrics, error)
	PutBridgeDeliveryMetrics(ctx context.Context, record DeliveryMetricRecord) error
}

func validateDeliveryLedgerSequences(lastSentSeq int64, lastAckedSeq int64) error {
	if lastSentSeq < 0 || lastAckedSeq < 0 {
		return errors.New("bridges: delivery ledger sequences cannot be negative")
	}
	if lastAckedSeq > lastSentSeq {
		return errors.New("bridges: delivery ledger acknowledged sequence cannot exceed sent sequence")
	}
	return nil
}

func validateDeliveryLedgerTerminal(state DeliveryLedgerState, terminalError string) error {
	normalizedState := state.Normalize()
	trimmedError := strings.TrimSpace(terminalError)
	if err := normalizedState.Validate(); err != nil {
		return err
	}
	if normalizedState == DeliveryLedgerStateTerminalError && trimmedError == "" {
		return errors.New("bridges: terminal error delivery requires an error")
	}
	if normalizedState != DeliveryLedgerStateTerminalError && trimmedError != "" {
		return errors.New("bridges: only terminal error deliveries may include an error")
	}
	return nil
}

func normalizeDeliveryDropReasons(reasons map[string]int) map[string]int {
	normalized := make(map[string]int, len(reasons))
	for reason, count := range reasons {
		normalized[strings.TrimSpace(reason)] = count
	}
	return normalized
}

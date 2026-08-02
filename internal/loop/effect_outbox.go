package loop

import (
	"encoding/json"
	"time"
)

// EffectOutboxState is the closed delivery state vocabulary.
type EffectOutboxState string

const (
	EffectPending   EffectOutboxState = "pending"
	EffectDelivered EffectOutboxState = "delivered"
	EffectFailed    EffectOutboxState = "failed"
)

// Valid reports whether value belongs to the closed outbox-state vocabulary.
func (s EffectOutboxState) Valid() bool {
	switch s {
	case EffectPending, EffectDelivered, EffectFailed:
		return true
	default:
		return false
	}
}

// EffectOutboxEntry is one immutable rendered effect delivery.
type EffectOutboxEntry struct {
	LoopRunID     RunID
	DeliveryID    string
	SourceEventID string
	Trigger       string
	Generation    int
	NodeID        NodeID
	ItemIndex     int
	EntryIndex    int
	Entry         json.RawMessage
	State         EffectOutboxState
	Attempts      int
	CreatedAt     time.Time
	DeliveredAt   *time.Time
}

package loop

import (
	"fmt"
	"strings"
	"time"
)

const strategyCanceledReasonCode = "canceled_by_strategy"

// StrategyCancellationIntent atomically closes pending waits and requests for one canceled lane cell.
type StrategyCancellationIntent struct {
	NodeID     string    `json:"node_id"`
	ItemIndex  int       `json:"item_index"`
	ActorKind  string    `json:"actor_kind"`
	ActorID    string    `json:"actor_id"`
	ReasonCode string    `json:"reason_code"`
	At         time.Time `json:"at"`
}

func (i StrategyCancellationIntent) normalized() StrategyCancellationIntent {
	i.NodeID = strings.TrimSpace(i.NodeID)
	i.ActorKind = strings.TrimSpace(i.ActorKind)
	i.ActorID = strings.TrimSpace(i.ActorID)
	i.ReasonCode = strings.TrimSpace(i.ReasonCode)
	if !i.At.IsZero() {
		i.At = i.At.UTC()
	}
	return i
}

func (i StrategyCancellationIntent) validate() error {
	if i.NodeID == "" || i.ItemIndex < 0 || i.ActorKind == "" || i.ActorID == "" ||
		i.ReasonCode != strategyCanceledReasonCode || i.At.IsZero() {
		return fmt.Errorf("%w: strategy cancellation identity is incomplete", ErrValidation)
	}
	return nil
}

func normalizeStrategyCancellationIntents(
	intents []StrategyCancellationIntent,
) ([]StrategyCancellationIntent, error) {
	normalized := make([]StrategyCancellationIntent, len(intents))
	for index, intent := range intents {
		intent = intent.normalized()
		if err := intent.validate(); err != nil {
			return nil, err
		}
		normalized[index] = intent
	}
	return normalized, nil
}

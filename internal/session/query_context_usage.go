package session

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

type UsageEventEnvelope struct {
	Sequence int64
	At       time.Time
	TurnID   string
	Usage    acp.TokenUsage
}

type DeliveryEventEnvelope struct {
	Sequence int64
	At       time.Time
	Manifest acp.DeliveryManifest
}

type CompactionFiredPayload struct {
	CompactionID string `json:"compaction_id"`
	Trigger      string `json:"trigger"`
	ContextUsed  *int64 `json:"context_used"`
	ContextSize  *int64 `json:"context_size"`
}

type CompactionEnvelope struct {
	Sequence int64
	At       time.Time
	TurnID   string
	Payload  CompactionFiredPayload
	Status   string
}

type SettledTurn struct {
	TurnID   string
	Sequence int64
}

func (m *Manager) UsageEvents(ctx context.Context, id string) ([]UsageEventEnvelope, error) {
	result := make([]UsageEventEnvelope, 0)
	for _, kind := range []string{acp.EventTypeUsage, acp.EventTypeDone} {
		rows, err := m.Events(ctx, id, store.EventQuery{Type: kind})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			event, err := transcript.UnmarshalAgentEvent(row.Content)
			if err != nil {
				return nil, fmt.Errorf("session: decode usage event %d: %w", row.Sequence, err)
			}
			if event.Usage == nil {
				continue
			}
			usage := *event.Usage
			usage.Sequence = row.Sequence
			result = append(
				result,
				UsageEventEnvelope{Sequence: row.Sequence, At: row.Timestamp, TurnID: row.TurnID, Usage: usage},
			)
		}
	}
	slices.SortFunc(result, func(a, b UsageEventEnvelope) int { return cmp.Compare(a.Sequence, b.Sequence) })
	return result, nil
}

func (m *Manager) Deliveries(ctx context.Context, id string) ([]DeliveryEventEnvelope, error) {
	rows, err := m.Events(ctx, id, store.EventQuery{Type: acp.EventTypePromptDelivery})
	if err != nil {
		return nil, err
	}
	result := make([]DeliveryEventEnvelope, 0, len(rows))
	for _, row := range rows {
		event, err := transcript.UnmarshalAgentEvent(row.Content)
		if err != nil {
			return nil, fmt.Errorf("session: decode delivery event %d: %w", row.Sequence, err)
		}
		if event.DeliveryManifest() == nil {
			return nil, fmt.Errorf("session: delivery event %d has no manifest", row.Sequence)
		}
		manifest := *event.DeliveryManifest()
		manifest.TurnID = row.TurnID
		result = append(result, DeliveryEventEnvelope{Sequence: row.Sequence, At: row.Timestamp, Manifest: manifest})
	}
	return result, nil
}

func (m *Manager) Compactions(ctx context.Context, id string) ([]CompactionEnvelope, error) {
	rows, err := m.Events(ctx, id, store.EventQuery{})
	if err != nil {
		return nil, err
	}
	attributed := make(map[string]CompactionFiredPayload)
	latest := make(map[string]CompactionEnvelope)
	for _, row := range rows {
		if row.Type != events.SessionCompactionFired && row.Type != acp.EventTypeCompaction {
			continue
		}
		event, err := transcript.UnmarshalAgentEvent(row.Content)
		if err != nil {
			return nil, fmt.Errorf("session: decode compaction event %d: %w", row.Sequence, err)
		}
		if row.Type == events.SessionCompactionFired {
			var payload CompactionFiredPayload
			if err := json.Unmarshal(event.Raw, &payload); err != nil {
				return nil, fmt.Errorf("session: decode compaction payload %d: %w", row.Sequence, err)
			}
			if payload.CompactionID != "" {
				attributed[payload.CompactionID] = payload
			}
		} else if event.Compaction != nil && event.Compaction.CompactionID != "" {
			latest[event.Compaction.CompactionID] = CompactionEnvelope{
				Sequence: row.Sequence,
				At:       row.Timestamp,
				TurnID:   row.TurnID,
				Status:   event.Compaction.Status,
			}
		}
	}
	result := make([]CompactionEnvelope, 0, len(latest))
	for id, marker := range latest {
		payload, ok := attributed[id]
		if !ok {
			continue
		}
		marker.Payload = payload
		result = append(result, marker)
	}
	slices.SortFunc(result, func(a, b CompactionEnvelope) int { return cmp.Compare(a.Sequence, b.Sequence) })
	return result, nil
}

func (m *Manager) CompactionBoundary(ctx context.Context, id string) (*int64, error) {
	rows, err := m.Events(ctx, id, store.EventQuery{Type: acp.EventTypeCompaction})
	if err != nil {
		return nil, err
	}
	var boundary *int64
	for _, row := range rows {
		event, err := transcript.UnmarshalAgentEvent(row.Content)
		if err != nil {
			return nil, fmt.Errorf("session: decode compaction event %d: %w", row.Sequence, err)
		}
		if event.Compaction != nil && event.Compaction.Terminal && (boundary == nil || row.Sequence > *boundary) {
			boundary = new(row.Sequence)
		}
	}
	return boundary, nil
}

func (m *Manager) LatestSettledTurn(ctx context.Context, id string) (SettledTurn, error) {
	row, err := m.LatestSessionEventByType(ctx, id, acp.EventTypeDone)
	if err != nil {
		return SettledTurn{}, err
	}
	if row == nil {
		return SettledTurn{}, nil
	}
	return SettledTurn{TurnID: row.TurnID, Sequence: row.Sequence}, nil
}

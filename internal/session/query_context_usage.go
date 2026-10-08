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
	"github.com/compozy/compozy/internal/session/contextusage"
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
	ContextAfter *contextusage.ContextAfter
	Sequence     int64
	At           time.Time
	TurnID       string
	Payload      CompactionFiredPayload
	Status       string
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
	boundaries, err := firstCompactionBoundaries(rows)
	if err != nil {
		return nil, err
	}
	observations, err := contextAfterObservations(rows)
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
		if boundary, terminal := boundaries[id]; terminal {
			marker.ContextAfter = firstContextAfter(boundary, boundaries, observations)
		}
		result = append(result, marker)
	}
	slices.SortFunc(result, func(a, b CompactionEnvelope) int { return cmp.Compare(a.Sequence, b.Sequence) })
	return result, nil
}

func contextAfterObservations(rows []store.SessionEvent) ([]contextusage.ContextAfter, error) {
	observations := make([]contextusage.ContextAfter, 0)
	for _, row := range rows {
		if row.Type != acp.EventTypeUsage && row.Type != acp.EventTypeDone {
			continue
		}
		event, err := transcript.UnmarshalAgentEvent(row.Content)
		if err != nil {
			return nil, fmt.Errorf("session: decode usage event %d: %w", row.Sequence, err)
		}
		if event.Usage != nil && event.Usage.ContextUsed != nil {
			observations = append(observations, contextusage.ContextAfter{
				Used: *event.Usage.ContextUsed, Size: event.Usage.ContextSize, Sequence: row.Sequence,
			})
		}
	}
	return observations, nil
}

func firstContextAfter(
	boundary int64,
	boundaries map[string]int64,
	observations []contextusage.ContextAfter,
) *contextusage.ContextAfter {
	var next int64
	for _, sequence := range boundaries {
		if sequence > boundary && (next == 0 || sequence < next) {
			next = sequence
		}
	}
	var after *contextusage.ContextAfter
	for _, observation := range observations {
		if observation.Sequence > boundary && (next == 0 || observation.Sequence < next) &&
			(after == nil || observation.Sequence < after.Sequence) {
			after = new(observation)
		}
	}
	return after
}

func firstCompactionBoundaries(rows []store.SessionEvent) (map[string]int64, error) {
	boundaries := make(map[string]int64)
	for _, row := range rows {
		if row.Type != acp.EventTypeCompaction {
			continue
		}
		event, err := transcript.UnmarshalAgentEvent(row.Content)
		if err != nil {
			return nil, fmt.Errorf("session: decode compaction event %d: %w", row.Sequence, err)
		}
		if event.Compaction == nil || !event.Compaction.Terminal || event.Compaction.CompactionID == "" {
			continue
		}
		id := event.Compaction.CompactionID
		if previous, found := boundaries[id]; !found || row.Sequence < previous {
			boundaries[id] = row.Sequence
		}
	}
	return boundaries, nil
}

func (m *Manager) CompactionClearBoundary(ctx context.Context, id string) (*contextusage.ClearedBy, error) {
	rows, err := m.Events(ctx, id, store.EventQuery{Type: acp.EventTypeCompaction})
	if err != nil {
		return nil, err
	}
	boundaries, err := firstCompactionBoundaries(rows)
	if err != nil {
		return nil, err
	}
	var boundary *contextusage.ClearedBy
	for compactionID, sequence := range boundaries {
		if boundary == nil || sequence > boundary.Sequence {
			boundary = &contextusage.ClearedBy{CompactionID: compactionID, Sequence: sequence}
		}
	}
	return boundary, nil
}

func (m *Manager) CompactionBoundary(ctx context.Context, id string) (*int64, error) {
	boundary, err := m.CompactionClearBoundary(ctx, id)
	if err != nil || boundary == nil {
		return nil, err
	}
	return new(boundary.Sequence), nil
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

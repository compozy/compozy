package transcript

import (
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/store"
)

type CompactionItem struct {
	Kind         string     `json:"kind"`
	CompactionID string     `json:"compaction_id"`
	Status       string     `json:"status"`
	Summary      string     `json:"summary,omitempty"`
	Error        string     `json:"error,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitzero"`
}

func projectCompactionEntry(events []store.SessionEvent, identity EntryIdentity) (*Entry, error) {
	item := CompactionItem{Kind: "compaction"}
	for _, stored := range events {
		decoded := decodeStoredEvent(stored)
		observation := decoded.agent.Compaction
		if observation == nil {
			continue
		}
		if item.CompactionID == "" {
			item.CompactionID = observation.CompactionID
			item.StartedAt = decoded.agent.Timestamp
		}
		item.Status = observation.Status
		item.Summary = boundedCompactionSummary(observation.Summary)
		item.Error = observation.Error
		if observation.Terminal && item.EndedAt == nil {
			item.EndedAt = new(decoded.agent.Timestamp)
		}
	}
	data, err := json.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("transcript: encode compaction item: %w", err)
	}
	message := UIMessage{
		ID:    fallbackMessageID(identity.MessageID, identity.BaseMessageID),
		Role:  UIRoleSystem,
		Parts: []UIMessagePart{{Type: "data-compozy-compaction", ID: item.CompactionID, Data: data}},
	}
	return &Entry{
		Message:       message,
		StartSequence: identity.StartSequence,
		Sequence:      identity.UpdatedSequence,
		EventType:     "compaction",
	}, nil
}

func boundedCompactionSummary(summary string) string {
	const limit = 16 * 1024
	const mark = " [summary truncated]"
	if len(summary) <= limit {
		return summary
	}
	end := limit - len(mark)
	for !utf8.ValidString(summary[:end]) {
		end--
	}
	return summary[:end] + mark
}

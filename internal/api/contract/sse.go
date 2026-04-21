package contract

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
)

const (
	DefaultHeartbeatInterval = 15 * time.Second
	HeartbeatGapTolerance    = 45 * time.Second
)

type StreamCursor struct {
	Timestamp time.Time
	Sequence  uint64
}

type SSEMessage struct {
	ID    string
	Event string
	Data  any
}

type HeartbeatPayload struct {
	RunID  string    `json:"run_id"`
	Cursor string    `json:"cursor,omitempty"`
	TS     time.Time `json:"ts"`
}

type OverflowPayload struct {
	RunID  string    `json:"run_id"`
	Cursor string    `json:"cursor,omitempty"`
	Reason string    `json:"reason,omitempty"`
	TS     time.Time `json:"ts"`
}

func FormatCursor(timestamp time.Time, sequence uint64) string {
	if timestamp.IsZero() || sequence == 0 {
		return ""
	}
	return fmt.Sprintf("%s|%020d", timestamp.UTC().Format(time.RFC3339Nano), sequence)
}

func FormatCursorPointer(cursor *StreamCursor) string {
	if cursor == nil {
		return ""
	}
	return FormatCursor(cursor.Timestamp, cursor.Sequence)
}

func CursorFromEvent(event events.Event) StreamCursor {
	return StreamCursor{
		Timestamp: event.Timestamp.UTC(),
		Sequence:  event.Seq,
	}
}

func ParseCursor(raw string) (StreamCursor, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return StreamCursor{}, nil
	}

	parts := strings.SplitN(value, "|", 2)
	if len(parts) != 2 {
		return StreamCursor{}, fmt.Errorf("invalid cursor %q", value)
	}

	timestamp, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(parts[0]))
	if err != nil {
		return StreamCursor{}, fmt.Errorf("invalid cursor timestamp %q: %w", parts[0], err)
	}

	sequence, err := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		return StreamCursor{}, fmt.Errorf("invalid cursor sequence %q: %w", parts[1], err)
	}
	if sequence == 0 {
		return StreamCursor{}, fmt.Errorf("invalid cursor sequence %q", parts[1])
	}

	return StreamCursor{
		Timestamp: timestamp.UTC(),
		Sequence:  sequence,
	}, nil
}

func EventAfterCursor(event events.Event, cursor StreamCursor) bool {
	if cursor.Timestamp.IsZero() || cursor.Sequence == 0 {
		return true
	}

	timestamp := event.Timestamp.UTC()
	switch {
	case timestamp.After(cursor.Timestamp):
		return true
	case timestamp.Before(cursor.Timestamp):
		return false
	default:
		return event.Seq > cursor.Sequence
	}
}

func HeartbeatMessage(runID string, cursor StreamCursor, now time.Time) SSEMessage {
	return SSEMessage{
		Event: "heartbeat",
		Data: HeartbeatPayload{
			RunID:  strings.TrimSpace(runID),
			Cursor: FormatCursor(cursor.Timestamp, cursor.Sequence),
			TS:     now.UTC(),
		},
	}
}

func OverflowMessage(runID string, cursor StreamCursor, now time.Time, reason string) SSEMessage {
	return SSEMessage{
		Event: "overflow",
		Data: OverflowPayload{
			RunID:  strings.TrimSpace(runID),
			Cursor: FormatCursor(cursor.Timestamp, cursor.Sequence),
			Reason: strings.TrimSpace(reason),
			TS:     now.UTC(),
		},
	}
}

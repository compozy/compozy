package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/pkg/compozy/events"
)

// StreamCursor is the canonical run-stream cursor.
type StreamCursor struct {
	Timestamp time.Time
	Sequence  uint64
}

// SSEMessage is one transport-level server-sent event.
type SSEMessage struct {
	ID    string
	Event string
	Data  any
}

// FlushWriter is the subset of the response writer needed for streaming.
type FlushWriter interface {
	io.Writer
	http.Flusher
}

var sseBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// PrepareSSE configures one Gin response for server-sent events.
func PrepareSSE(c *gin.Context) (FlushWriter, error) {
	if c == nil {
		return nil, errors.New("sse context is required")
	}
	writer, ok := c.Writer.(FlushWriter)
	if !ok {
		return nil, errors.New("response writer does not support flushing")
	}

	headers := c.Writer.Header()
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	c.Writer.WriteHeaderNow()
	writer.Flush()
	return writer, nil
}

// WriteSSE writes one SSE message with JSON-encoded data.
func WriteSSE(writer FlushWriter, msg SSEMessage) error {
	if writer == nil {
		return errors.New("sse writer is required")
	}

	payload, err := json.Marshal(msg.Data)
	if err != nil {
		return fmt.Errorf("marshal sse payload: %w", err)
	}
	if len(payload) == 0 {
		payload = []byte("null")
	}

	buffer, ok := sseBufferPool.Get().(*bytes.Buffer)
	if !ok || buffer == nil {
		buffer = new(bytes.Buffer)
	}
	buffer.Reset()
	defer sseBufferPool.Put(buffer)

	if strings.TrimSpace(msg.ID) != "" {
		buffer.WriteString("id: ")
		buffer.WriteString(strings.TrimSpace(msg.ID))
		buffer.WriteByte('\n')
	}

	if strings.TrimSpace(msg.Event) != "" {
		buffer.WriteString("event: ")
		buffer.WriteString(strings.TrimSpace(msg.Event))
		buffer.WriteByte('\n')
	}

	buffer.WriteString("data: ")
	if _, err := buffer.Write(payload); err != nil {
		return fmt.Errorf("write sse payload: %w", err)
	}
	buffer.WriteString("\n\n")
	if _, err := writer.Write(buffer.Bytes()); err != nil {
		return fmt.Errorf("write sse payload: %w", err)
	}
	writer.Flush()
	return nil
}

// FormatCursor renders the canonical cursor form.
func FormatCursor(timestamp time.Time, sequence uint64) string {
	if timestamp.IsZero() || sequence == 0 {
		return ""
	}
	return fmt.Sprintf("%s|%020d", timestamp.UTC().Format(time.RFC3339Nano), sequence)
}

// CursorFromEvent builds the canonical cursor for one persisted event.
func CursorFromEvent(event events.Event) StreamCursor {
	return StreamCursor{
		Timestamp: event.Timestamp.UTC(),
		Sequence:  event.Seq,
	}
}

// ParseCursor parses a Last-Event-ID or pagination cursor.
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
	if err != nil || sequence == 0 {
		return StreamCursor{}, fmt.Errorf("invalid cursor sequence %q", parts[1])
	}

	return StreamCursor{
		Timestamp: timestamp.UTC(),
		Sequence:  sequence,
	}, nil
}

// EventAfterCursor reports whether an event should be emitted after the given cursor.
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

// HeartbeatMessage builds the canonical heartbeat SSE event.
func HeartbeatMessage(runID string, cursor StreamCursor, now time.Time) SSEMessage {
	return SSEMessage{
		Event: "heartbeat",
		Data: map[string]any{
			"run_id": runID,
			"cursor": FormatCursor(cursor.Timestamp, cursor.Sequence),
			"ts":     now.UTC(),
		},
	}
}

// OverflowMessage builds the canonical overflow SSE event.
func OverflowMessage(runID string, cursor StreamCursor, now time.Time, reason string) SSEMessage {
	return SSEMessage{
		Event: "overflow",
		Data: map[string]any{
			"run_id": runID,
			"cursor": FormatCursor(cursor.Timestamp, cursor.Sequence),
			"reason": strings.TrimSpace(reason),
			"ts":     now.UTC(),
		},
	}
}

func resetTimer(timer *time.Timer, interval time.Duration) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(interval)
}

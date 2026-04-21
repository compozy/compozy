package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/pkg/compozy/events"
)

type StreamCursor = contract.StreamCursor
type SSEMessage = contract.SSEMessage
type HeartbeatPayload = contract.HeartbeatPayload
type OverflowPayload = contract.OverflowPayload

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

const (
	// RunSnapshotSSEEvent is the canonical snapshot event name for daemon run streams.
	RunSnapshotSSEEvent = "run.snapshot"
	// RunEventSSEEvent is the canonical live event name for daemon run streams.
	RunEventSSEEvent = "run.event"
	// RunHeartbeatSSEEvent is the canonical heartbeat event name for daemon run streams.
	RunHeartbeatSSEEvent = "run.heartbeat"
	// RunOverflowSSEEvent is the canonical overflow event name for daemon run streams.
	RunOverflowSSEEvent = "run.overflow"
)

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
	if written, err := writer.Write(buffer.Bytes()); err != nil {
		return fmt.Errorf("write sse payload: %w", err)
	} else if written != buffer.Len() {
		return fmt.Errorf("write sse payload: %w", io.ErrShortWrite)
	}
	writer.Flush()
	return nil
}

// FormatCursor renders the canonical cursor form.
func FormatCursor(timestamp time.Time, sequence uint64) string {
	return contract.FormatCursor(timestamp, sequence)
}

// CursorFromEvent builds the canonical cursor for one persisted event.
func CursorFromEvent(event events.Event) StreamCursor {
	return contract.CursorFromEvent(event)
}

// ParseCursor parses a Last-Event-ID or pagination cursor.
func ParseCursor(raw string) (StreamCursor, error) {
	return contract.ParseCursor(raw)
}

// EventAfterCursor reports whether an event should be emitted after the given cursor.
func EventAfterCursor(event events.Event, cursor StreamCursor) bool {
	return contract.EventAfterCursor(event, cursor)
}

// EventMessage builds the canonical live-event SSE frame.
func EventMessage(event events.Event) SSEMessage {
	return SSEMessage{
		ID:    FormatCursor(event.Timestamp, event.Seq),
		Event: RunEventSSEEvent,
		Data:  event,
	}
}

// HeartbeatMessage builds the canonical heartbeat SSE event.
func HeartbeatMessage(runID string, cursor StreamCursor, now time.Time) SSEMessage {
	return SSEMessage{
		Event: RunHeartbeatSSEEvent,
		Data: HeartbeatPayload{
			RunID:  strings.TrimSpace(runID),
			Cursor: FormatCursor(cursor.Timestamp, cursor.Sequence),
			TS:     now.UTC(),
		},
	}
}

// OverflowMessage builds the canonical overflow SSE event.
func OverflowMessage(runID string, cursor StreamCursor, now time.Time, reason string) SSEMessage {
	return SSEMessage{
		Event: RunOverflowSSEEvent,
		Data: OverflowPayload{
			RunID:  strings.TrimSpace(runID),
			Cursor: FormatCursor(cursor.Timestamp, cursor.Sequence),
			Reason: strings.TrimSpace(reason),
			TS:     now.UTC(),
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

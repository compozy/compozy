package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	ssepkg "github.com/compozy/agh/internal/sse"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

// PrepareSSE configures a Gin response for SSE streaming.
func PrepareSSE(c *gin.Context) (FlushWriter, error) {
	writer, ok := c.Writer.(FlushWriter)
	if !ok {
		return nil, errors.New("response writer does not support flushing")
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
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

	return writeSSERaw(writer, msg.ID, payload, msg.Name)
}

// WriteTaskStreamEvent writes one task-native live event as a standard SSE message.
// TaskStreamEventPayload.Type is the event identity consumed by clients.
func WriteTaskStreamEvent(writer FlushWriter, event taskpkg.StreamEvent) error {
	return WriteSSE(writer, SSEMessage{
		ID:   strconv.FormatInt(event.Sequence, 10),
		Data: TaskStreamEventPayloadFromEvent(event),
	})
}

// WriteSSEComment writes one SSE comment frame. EventSource clients ignore
// comment frames, so stream keep-alives must use this helper instead of data events.
func WriteSSEComment(writer FlushWriter, comment string) error {
	if writer == nil {
		return errors.New("sse writer is required")
	}
	trimmed := strings.TrimSpace(comment)
	if trimmed == "" {
		return errors.New("sse comment is required")
	}
	if strings.ContainsAny(trimmed, "\r\n") {
		return errors.New("sse comment must be a single line")
	}
	if err := writeSSEString(writer, "write sse comment prefix", ": "); err != nil {
		return err
	}
	if err := writeSSEString(writer, "write sse comment", trimmed); err != nil {
		return err
	}
	if err := writeSSEString(writer, "write sse comment terminator", "\n\n"); err != nil {
		return err
	}
	writer.Flush()
	return nil
}

func (h *BaseHandlers) writeSSEBestEffort(writer FlushWriter, msg SSEMessage) {
	if err := WriteSSE(writer, msg); err != nil && h != nil && h.Logger != nil {
		h.Logger.Warn("api: failed to emit sse message", "event", msg.Name, "error", err)
	}
}

func (h *BaseHandlers) logSSEWriteFailure(eventName string, err error) {
	if err != nil && h != nil && h.Logger != nil {
		h.Logger.Warn("api: failed to emit sse message", "event", eventName, "error", err)
	}
}

// WriteSSERaw writes one SSE message using a pre-encoded payload.
func WriteSSERaw(writer FlushWriter, id string, raw string, names ...string) error {
	return writeSSERaw(writer, id, []byte(raw), names...)
}

func writeSSERaw(writer FlushWriter, id string, raw []byte, names ...string) error {
	if writer == nil {
		return errors.New("sse writer is required")
	}
	if len(raw) == 0 {
		raw = []byte("null")
	}
	raw = ssepkg.ScrubMemoryContextBytes(raw)

	if id != "" {
		if err := writeSSEString(writer, "write sse id prefix", "id: "); err != nil {
			return err
		}
		if err := writeSSEString(writer, "write sse id", id); err != nil {
			return err
		}
		if err := writeSSEString(writer, "write sse id terminator", "\n"); err != nil {
			return err
		}
	}
	if len(names) > 0 && strings.TrimSpace(names[0]) != "" {
		if err := writeSSEString(writer, "write sse event prefix", "event: "); err != nil {
			return err
		}
		if err := writeSSEString(writer, "write sse event", names[0]); err != nil {
			return err
		}
		if err := writeSSEString(writer, "write sse event terminator", "\n"); err != nil {
			return err
		}
	}
	if err := writeSSEString(writer, "write sse data prefix", "data: "); err != nil {
		return err
	}
	if _, err := writer.Write(raw); err != nil {
		return fmt.Errorf("write sse data payload: %w", err)
	}
	if err := writeSSEString(writer, "write sse message terminator", "\n\n"); err != nil {
		return err
	}
	writer.Flush()
	return nil
}

func writeSSEString(writer FlushWriter, operation string, value string) error {
	if _, err := io.WriteString(writer, value); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}

// EmitLogs writes log events newer than the supplied cursor.
func EmitLogs(writer FlushWriter, events []store.EventSummary, cursor LogsCursor) LogsCursor {
	next := cursor
	for _, event := range events {
		if !LogEventAfterCursor(event, next) {
			continue
		}
		if err := WriteSSE(writer, SSEMessage{
			ID:   LogEventID(event),
			Name: event.Type,
			Data: LogEventPayloadFromSummary(event),
		}); err != nil {
			return next
		}
		next = LogsCursor{
			Timestamp: event.Timestamp.UTC(),
			Sequence:  event.Sequence,
			ID:        event.ID,
		}
	}
	return next
}

// LogEventAfterCursor reports whether a log event should be emitted after the cursor.
func LogEventAfterCursor(event store.EventSummary, cursor LogsCursor) bool {
	if cursor.Timestamp.IsZero() && cursor.Sequence == 0 && strings.TrimSpace(cursor.ID) == "" {
		return true
	}

	timestamp := event.Timestamp.UTC()
	switch {
	case timestamp.After(cursor.Timestamp):
		return true
	case timestamp.Before(cursor.Timestamp):
		return false
	default:
		if cursor.Sequence > 0 && event.Sequence > 0 {
			return event.Sequence > cursor.Sequence
		}
		return event.ID > cursor.ID
	}
}

// LogEventID builds a stable Last-Event-ID value for logs streaming.
func LogEventID(event store.EventSummary) string {
	buffer := make([]byte, 0, len(time.RFC3339Nano)+1+20)
	buffer = event.Timestamp.UTC().AppendFormat(buffer, time.RFC3339Nano)
	buffer = append(buffer, '|')
	if event.Sequence > 0 {
		buffer = appendZeroPaddedInt64(buffer, event.Sequence, 20)
		return string(buffer)
	}
	buffer = append(buffer, event.ID...)
	return string(buffer)
}

func appendZeroPaddedInt64(buffer []byte, value int64, width int) []byte {
	for digitCount := decimalDigitCount(value); digitCount < width; digitCount++ {
		buffer = append(buffer, '0')
	}
	return strconv.AppendInt(buffer, value, 10)
}

func decimalDigitCount(value int64) int {
	count := 1
	for value >= 10 {
		value /= 10
		count++
	}
	return count
}

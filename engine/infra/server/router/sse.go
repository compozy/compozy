package router

import (
	"bytes"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

const (
	sseContentType     = "text/event-stream"
	sseCacheControl    = "no-cache"
	sseConnection      = "keep-alive"
	sseAccelBuffering  = "no"
	heartbeatFrameBody = ": ping\n\n"
)

var eventNameSanitizer = strings.NewReplacer("\r", "", "\n", "")

// SSEStream manages writing Server-Sent Events frames with flushing support.
type SSEStream struct {
	writer      http.ResponseWriter
	controller  *http.ResponseController
	writeLocker sync.Mutex
}

// StartSSE configures the response headers for SSE and returns a stream helper.
func StartSSE(w http.ResponseWriter) *SSEStream {
	if w == nil {
		return nil
	}
	headers := w.Header()
	headers.Set("Content-Type", sseContentType)
	headers.Set("Cache-Control", sseCacheControl)
	headers.Set("Connection", sseConnection)
	headers.Set("X-Accel-Buffering", sseAccelBuffering)
	return &SSEStream{writer: w, controller: http.NewResponseController(w)}
}

// LastEventID extracts and parses the Last-Event-ID header from the request when provided.
func LastEventID(r *http.Request) (int64, bool, error) {
	if r == nil {
		return 0, false, nil
	}
	value := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	if value == "" {
		return 0, false, nil
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, true, err
	}
	return id, true, nil
}

// WriteEvent writes a data event with the provided id and type, flushing the response.
func (s *SSEStream) WriteEvent(id int64, event string, data []byte) error {
	if s == nil || s.writer == nil {
		return errors.New("sse: nil stream")
	}
	s.writeLocker.Lock()
	defer s.writeLocker.Unlock()
	var payload bytes.Buffer
	payload.Grow(len(data) + 64)
	payload.WriteString("id: ")
	payload.WriteString(strconv.FormatInt(id, 10))
	payload.WriteByte('\n')
	if sanitized := sanitizeSSEToken(event); sanitized != "" {
		payload.WriteString("event: ")
		payload.WriteString(sanitized)
		payload.WriteByte('\n')
	}
	if len(data) == 0 {
		payload.WriteString("data:\n\n")
	} else {
		lines := bytes.Split(data, []byte{'\n'})
		for _, line := range lines {
			payload.WriteString("data: ")
			payload.Write(line)
			payload.WriteByte('\n')
		}
		payload.WriteByte('\n')
	}
	if _, err := s.writer.Write(payload.Bytes()); err != nil {
		return err
	}
	return s.flush()
}

func sanitizeSSEToken(value string) string {
	if value == "" {
		return ""
	}
	return eventNameSanitizer.Replace(value)
}

// WriteHeartbeat emits an SSE heartbeat comment frame and flushes the response.
func (s *SSEStream) WriteHeartbeat() error {
	if s == nil || s.writer == nil {
		return errors.New("sse: nil stream")
	}
	s.writeLocker.Lock()
	defer s.writeLocker.Unlock()
	if _, err := s.writer.Write([]byte(heartbeatFrameBody)); err != nil {
		return err
	}
	return s.flush()
}

func (s *SSEStream) flush() error {
	if s == nil {
		return errors.New("sse: nil stream")
	}
	if s.controller != nil {
		if err := s.controller.Flush(); err != nil {
			if !errors.Is(err, http.ErrNotSupported) {
				return err
			}
		} else {
			return nil
		}
	}
	flusher, ok := s.writer.(http.Flusher)
	if !ok {
		return errors.New("sse: response writer does not support flushing")
	}
	flusher.Flush()
	return nil
}

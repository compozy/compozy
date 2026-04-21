package core_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/core"
)

var errFailingFlushWrite = errors.New("write failed")

type flushBuffer struct {
	bytes.Buffer
	flushed bool
}

func (f *flushBuffer) Flush() {
	f.flushed = true
}

type countingFlushWriter struct {
	bytes.Buffer
	writeCalls int
	flushCalls int
}

func (w *countingFlushWriter) Write(p []byte) (int, error) {
	w.writeCalls++
	return w.Buffer.Write(p)
}

func (w *countingFlushWriter) Flush() {
	w.flushCalls++
}

func TestWriteSSEFormatsFramesWithCanonicalCursor(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, 4, 17, 12, 0, 0, 123456789, time.UTC)
	cursor := core.FormatCursor(timestamp, 7)

	writer := &flushBuffer{}
	err := core.WriteSSE(writer, core.SSEMessage{
		ID:    cursor,
		Event: core.RunEventSSEEvent,
		Data: struct {
			Status string `json:"status"`
		}{Status: "started"},
	})
	if err != nil {
		t.Fatalf("WriteSSE() error = %v", err)
	}

	text := writer.String()
	if !strings.Contains(text, "id: "+cursor+"\n") {
		t.Fatalf("SSE output missing canonical cursor:\n%s", text)
	}
	if !strings.Contains(text, "event: "+core.RunEventSSEEvent+"\n") {
		t.Fatalf("SSE output missing event name:\n%s", text)
	}
	if !strings.Contains(text, `data: {"status":"started"}`) {
		t.Fatalf("SSE output missing JSON payload:\n%s", text)
	}
	if !writer.flushed {
		t.Fatal("expected writer.Flush to be called")
	}
}

func TestWriteSSEUsesSingleWriteAndFlushPerFrame(t *testing.T) {
	t.Parallel()

	writer := &countingFlushWriter{}
	err := core.WriteSSE(writer, core.SSEMessage{
		ID:    "cursor-1",
		Event: "run.started",
		Data:  map[string]string{"status": "started"},
	})
	if err != nil {
		t.Fatalf("WriteSSE() error = %v", err)
	}
	if writer.writeCalls != 1 {
		t.Fatalf("write calls = %d, want 1", writer.writeCalls)
	}
	if writer.flushCalls != 1 {
		t.Fatalf("flush calls = %d, want 1", writer.flushCalls)
	}
}

func TestParseCursorRejectsInvalidShapes(t *testing.T) {
	testCases := []string{
		"bad",
		"2026-04-17T12:00:00Z|",
		"nope|00000000000000000001",
		"2026-04-17T12:00:00Z|abc",
		"2026-04-17T12:00:00Z|0",
	}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			t.Parallel()

			if _, err := core.ParseCursor(tc); err == nil {
				t.Fatalf("ParseCursor(%q) error = nil, want error", tc)
			}
		})
	}
}

func TestHeartbeatAndOverflowMessagesUseExpectedEventNames(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC)
	cursor := core.StreamCursor{Timestamp: timestamp, Sequence: 11}

	testCases := []struct {
		name    string
		message core.SSEMessage
		want    []string
	}{
		{
			name:    "heartbeat",
			message: core.HeartbeatMessage("run-1", cursor, timestamp.Add(time.Second)),
			want: []string{
				"event: " + core.RunHeartbeatSSEEvent,
				`"run_id":"run-1"`,
				`"cursor":"` + core.FormatCursor(cursor.Timestamp, cursor.Sequence) + `"`,
			},
		},
		{
			name:    "overflow",
			message: core.OverflowMessage("run-1", cursor, timestamp.Add(2*time.Second), "slow consumer"),
			want: []string{
				"event: " + core.RunOverflowSSEEvent,
				`"run_id":"run-1"`,
				`"cursor":"` + core.FormatCursor(cursor.Timestamp, cursor.Sequence) + `"`,
				`"reason":"slow consumer"`,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			writer := &flushBuffer{}
			if err := core.WriteSSE(writer, tc.message); err != nil {
				t.Fatalf("WriteSSE(%s) error = %v", tc.name, err)
			}

			text := writer.String()
			for _, want := range tc.want {
				if !strings.Contains(text, want) {
					t.Fatalf("SSE output missing %q:\n%s", want, text)
				}
			}
		})
	}
}

func TestWriteSSEErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("nil writer", func(t *testing.T) {
		t.Parallel()

		if err := core.WriteSSE(nil, core.SSEMessage{Data: "payload"}); err == nil {
			t.Fatal("WriteSSE(nil) error = nil, want error")
		}
	})

	t.Run("marshal failure", func(t *testing.T) {
		t.Parallel()

		writer := &flushBuffer{}
		if err := core.WriteSSE(writer, core.SSEMessage{Data: func() {}}); err == nil {
			t.Fatal("WriteSSE(marshal failure) error = nil, want error")
		}
	})

	t.Run("write failure", func(t *testing.T) {
		t.Parallel()

		writer := &failingFlushWriter{}
		err := core.WriteSSE(writer, core.SSEMessage{
			ID:    "cursor",
			Event: "run.started",
			Data:  map[string]string{"status": "started"},
		})
		if err == nil {
			t.Fatal("WriteSSE(write failure) error = nil, want error")
		}
		if !errors.Is(err, errFailingFlushWrite) {
			t.Fatalf("WriteSSE(write failure) error = %v, want sentinel %v", err, errFailingFlushWrite)
		}
		if err == errFailingFlushWrite {
			t.Fatal("WriteSSE(write failure) returned sentinel directly, want wrapped error")
		}
	})

	t.Run("short write failure", func(t *testing.T) {
		t.Parallel()

		writer := &shortWriteFlushWriter{}
		err := core.WriteSSE(writer, core.SSEMessage{
			ID:    "cursor",
			Event: "run.started",
			Data:  map[string]string{"status": "started"},
		})
		if !errors.Is(err, io.ErrShortWrite) {
			t.Fatalf("WriteSSE(short write) error = %v, want %v", err, io.ErrShortWrite)
		}
	})
}

type failingFlushWriter struct{}

func (*failingFlushWriter) Write([]byte) (int, error) {
	return 0, errFailingFlushWrite
}

func (*failingFlushWriter) Flush() {}

type shortWriteFlushWriter struct{}

func (*shortWriteFlushWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return len(p) - 1, nil
}

func (*shortWriteFlushWriter) Flush() {}

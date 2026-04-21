package client

import (
	"bytes"
	"context"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/pkg/compozy/events"
)

func TestClientRunStreamDispatchFrameSupportsCanonicalSSEContract(t *testing.T) {
	t.Parallel()

	connection := &streamConnection{
		ctx:    context.Background(),
		items:  make(chan streamEnvelope, 4),
		errors: make(chan error, 1),
	}

	snapshotCursor := apicore.FormatCursor(time.Date(2026, 4, 21, 3, 0, 0, 0, time.UTC), 3)
	frames := []sseFrame{
		newSSEFrame(
			apicore.RunSnapshotSSEEvent,
			`{"run":{"run_id":"run-1","status":"running"},"next_cursor":"`+snapshotCursor+`"}`,
		),
		newSSEFrame(
			apicore.RunHeartbeatSSEEvent,
			`{"cursor":"2026-04-21T03:00:01Z|00000000000000000004","ts":"2026-04-21T03:00:01Z"}`,
		),
		newSSEFrame(
			apicore.RunOverflowSSEEvent,
			`{"cursor":"2026-04-21T03:00:02Z|00000000000000000005","reason":"lagging","ts":"2026-04-21T03:00:02Z"}`,
		),
		newSSEFrame(
			apicore.RunEventSSEEvent,
			`{"schema_version":"1.0","run_id":"run-1","seq":6,"ts":"2026-04-21T03:00:03Z","kind":"run.completed"}`,
		),
	}

	for _, frame := range frames {
		if err := connection.dispatchFrame(frame); err != nil {
			t.Fatalf("dispatchFrame(%q) error = %v", frame.event, err)
		}
	}

	snapshot := <-connection.items
	if snapshot.item.Snapshot == nil || snapshot.item.Snapshot.Snapshot.NextCursor == nil {
		t.Fatalf("snapshot item = %#v, want snapshot cursor", snapshot)
	}
	if snapshot.item.Snapshot.Snapshot.NextCursor.Sequence != 3 {
		t.Fatalf("snapshot cursor seq = %d, want 3", snapshot.item.Snapshot.Snapshot.NextCursor.Sequence)
	}

	heartbeat := <-connection.items
	if heartbeat.item.Heartbeat == nil || heartbeat.item.Heartbeat.Cursor.Sequence != 4 {
		t.Fatalf("heartbeat item = %#v, want cursor seq 4", heartbeat)
	}

	overflow := <-connection.items
	if overflow.item.Overflow == nil || overflow.item.Overflow.Cursor.Sequence != 5 {
		t.Fatalf("overflow item = %#v, want cursor seq 5", overflow)
	}

	event := <-connection.items
	if event.item.Event == nil || event.item.Event.Kind != events.EventKindRunCompleted || event.item.Event.Seq != 6 {
		t.Fatalf("event item = %#v, want completed event seq 6", event)
	}
}

func TestClientRunStreamDispatchFrameSupportsLegacyHeartbeatAndOverflowNames(t *testing.T) {
	t.Parallel()

	connection := &streamConnection{
		ctx:    context.Background(),
		items:  make(chan streamEnvelope, 2),
		errors: make(chan error, 1),
	}

	frames := []sseFrame{
		newSSEFrame("heartbeat", `{"cursor":"2026-04-21T04:00:01Z|00000000000000000007","ts":"2026-04-21T04:00:01Z"}`),
		newSSEFrame(
			"overflow",
			`{"cursor":"2026-04-21T04:00:02Z|00000000000000000008","reason":"slow","ts":"2026-04-21T04:00:02Z"}`,
		),
	}

	for _, frame := range frames {
		if err := connection.dispatchFrame(frame); err != nil {
			t.Fatalf("dispatchFrame(%q) error = %v", frame.event, err)
		}
	}

	if got := (<-connection.items).item.Heartbeat; got == nil || got.Cursor.Sequence != 7 {
		t.Fatalf("legacy heartbeat = %#v, want cursor seq 7", got)
	}
	if got := (<-connection.items).item.Overflow; got == nil || got.Cursor.Sequence != 8 {
		t.Fatalf("legacy overflow = %#v, want cursor seq 8", got)
	}
}

func newSSEFrame(event string, raw string) sseFrame {
	var data bytes.Buffer
	data.WriteString(raw)
	return sseFrame{
		event: event,
		data:  data,
	}
}

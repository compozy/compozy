package runs

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
)

type runFixture struct {
	runJSON       map[string]any
	resultJSON    map[string]any
	events        []events.Event
	rawEventLines []string
	partialTail   string
}

func writeRunFixture(t *testing.T, workspaceRoot, runID string, fixture runFixture) string {
	t.Helper()

	runDir := filepath.Join(workspaceRoot, ".compozy", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	if fixture.runJSON != nil {
		payload, err := json.Marshal(fixture.runJSON)
		if err != nil {
			t.Fatalf("marshal run.json: %v", err)
		}
		if err := os.WriteFile(filepath.Join(runDir, "run.json"), payload, 0o600); err != nil {
			t.Fatalf("write run.json: %v", err)
		}
	}

	if fixture.resultJSON != nil {
		payload, err := json.Marshal(fixture.resultJSON)
		if err != nil {
			t.Fatalf("marshal result.json: %v", err)
		}
		if err := os.WriteFile(filepath.Join(runDir, "result.json"), payload, 0o600); err != nil {
			t.Fatalf("write result.json: %v", err)
		}
	}

	if fixture.events != nil || fixture.rawEventLines != nil || fixture.partialTail != "" {
		file, err := os.OpenFile(filepath.Join(runDir, "events.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			t.Fatalf("open events.jsonl: %v", err)
		}
		defer func() {
			_ = file.Close()
		}()

		encoder := json.NewEncoder(file)
		for _, ev := range fixture.events {
			if err := encoder.Encode(ev); err != nil {
				t.Fatalf("encode event: %v", err)
			}
		}
		for _, line := range fixture.rawEventLines {
			if _, err := file.WriteString(line + "\n"); err != nil {
				t.Fatalf("write raw event line: %v", err)
			}
		}
		if fixture.partialTail != "" {
			if _, err := file.WriteString(fixture.partialTail); err != nil {
				t.Fatalf("write partial tail: %v", err)
			}
		}
	}

	return runDir
}

func appendEvents(t *testing.T, eventsPath string, items []events.Event) {
	t.Helper()

	file, err := os.OpenFile(eventsPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatalf("open events append file: %v", err)
	}
	defer func() {
		_ = file.Close()
	}()

	encoder := json.NewEncoder(file)
	for _, ev := range items {
		if err := encoder.Encode(ev); err != nil {
			t.Fatalf("append event: %v", err)
		}
	}
}

func collectReplay(run *Run, fromSeq uint64) ([]events.Event, []error) {
	var (
		gotEvents []events.Event
		gotErrors []error
	)

	for ev, err := range run.Replay(fromSeq) {
		if err != nil {
			gotErrors = append(gotErrors, err)
			continue
		}
		gotEvents = append(gotEvents, ev)
	}
	return gotEvents, gotErrors
}

func collectTailEvents(
	t *testing.T,
	eventsCh <-chan events.Event,
	errsCh <-chan error,
	want int,
	timeout time.Duration,
) []events.Event {
	t.Helper()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	got := make([]events.Event, 0, want)
	for len(got) < want {
		select {
		case <-deadline.C:
			t.Fatalf("timed out waiting for %d tail events, got %d", want, len(got))
		case err, ok := <-errsCh:
			if ok && err != nil {
				t.Fatalf("unexpected tail error: %v", err)
			}
		case ev, ok := <-eventsCh:
			if !ok {
				t.Fatalf("tail events channel closed after %d events, want %d", len(got), want)
			}
			got = append(got, ev)
		}
	}
	return got
}

func awaitRunEvent(t *testing.T, eventsCh <-chan RunEvent, errsCh <-chan error, timeout time.Duration) RunEvent {
	t.Helper()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		select {
		case <-deadline.C:
			t.Fatalf("timed out waiting for run event")
		case err, ok := <-errsCh:
			if ok && err != nil {
				t.Fatalf("unexpected watch error: %v", err)
			}
		case event, ok := <-eventsCh:
			if !ok {
				t.Fatalf("run events channel closed before event arrived")
			}
			return event
		}
	}
}

func waitForEventChannelClose[T any](t *testing.T, ch <-chan T, label string, timeout time.Duration) {
	t.Helper()

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatalf("%s channel still has values", label)
		}
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for %s channel to close", label)
	}
}

func captureWarnLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() {
		slog.SetDefault(previous)
	})
	return &buf
}

func collectedSeqs(items []events.Event) []uint64 {
	seqs := make([]uint64, 0, len(items))
	for _, item := range items {
		seqs = append(seqs, item.Seq)
	}
	return seqs
}

func testEvent(runID string, seq uint64, kind events.EventKind) events.Event {
	return events.Event{
		SchemaVersion: events.SchemaVersion,
		RunID:         runID,
		Seq:           seq,
		Timestamp:     time.Unix(int64(seq), 0).UTC(),
		Kind:          kind,
		Payload:       json.RawMessage(`{"seq":1}`),
	}
}

func cancelAndAwaitClose[T any](t *testing.T, cancel context.CancelFunc, eventsCh <-chan T, errsCh <-chan error) {
	t.Helper()

	cancel()
	waitForEventChannelClose(t, eventsCh, "events", time.Second)
	waitForEventChannelClose(t, errsCh, "errors", time.Second)
}

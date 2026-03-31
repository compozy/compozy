package run

import (
	"bytes"
	"testing"
	"time"
)

func TestLineRingKeepsMostRecentLines(t *testing.T) {
	t.Parallel()

	ring := newLineRing(2)
	ring.appendLine("first")
	ring.appendLine("")
	ring.appendLine("second")
	ring.appendLine("third")

	got := ring.snapshot()
	want := []string{"second", "third"}
	if len(got) != len(want) {
		t.Fatalf("snapshot length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("snapshot[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestActivityWriterRecordsActivityAndWritesBytes(t *testing.T) {
	t.Parallel()

	monitor := newActivityMonitor()
	time.Sleep(10 * time.Millisecond)
	before := monitor.timeSinceLastActivity()

	var dst bytes.Buffer
	writer := newActivityWriter(&dst, monitor)

	if _, err := writer.Write([]byte("terminal-output")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if got := dst.String(); got != "terminal-output" {
		t.Fatalf("buffer = %q, want %q", got, "terminal-output")
	}
	if after := monitor.timeSinceLastActivity(); after < before {
		return
	}
	t.Fatalf("expected activity timestamp to advance")
}

func TestActivityWriterFallsBackToDiscardWhenDestinationIsNil(t *testing.T) {
	t.Parallel()

	monitor := newActivityMonitor()
	writer := newActivityWriter(nil, monitor)

	if _, err := writer.Write([]byte("ignored")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if got := monitor.timeSinceLastActivity(); got <= 0 {
		t.Fatalf("expected positive activity duration, got %v", got)
	}
}

func TestActivityMonitorTimeSinceLastActivityDecreasesAfterRecord(t *testing.T) {
	t.Parallel()

	monitor := newActivityMonitor()
	time.Sleep(15 * time.Millisecond)
	before := monitor.timeSinceLastActivity()
	monitor.recordActivity()
	after := monitor.timeSinceLastActivity()

	if after >= before {
		t.Fatalf("expected activity duration to decrease after record; before=%v after=%v", before, after)
	}
}

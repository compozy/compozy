package task

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func BenchmarkTaskStatusFromSnapshotLatestTerminal(b *testing.B) {
	runs := benchmarkTerminalRuns(256)

	b.ReportAllocs()

	for b.Loop() {
		if got := taskStatusFromSnapshot(TaskStatusReady, false, runs); got != TaskStatusCompleted {
			b.Fatalf("taskStatusFromSnapshot() = %q, want %q", got, TaskStatusCompleted)
		}
	}
}

func BenchmarkTaskStatusFromSnapshotQueuedAfterTerminal(b *testing.B) {
	runs := append(benchmarkTerminalRuns(255), Run{
		Status:   TaskRunStatusQueued,
		Attempt:  256,
		QueuedAt: time.Date(2026, 4, 14, 16, 4, 16, 0, time.UTC),
	})

	b.ReportAllocs()

	for b.Loop() {
		if got := taskStatusFromSnapshot(TaskStatusReady, false, runs); got != TaskStatusReady {
			b.Fatalf("taskStatusFromSnapshot() = %q, want %q", got, TaskStatusReady)
		}
	}
}

func BenchmarkNormalizeRawJSONTrimmed256B(b *testing.B) {
	raw := benchmarkRawJSONWithWhitespace(256)

	b.ReportAllocs()

	for b.Loop() {
		if got := normalizeRawJSON(raw); len(got) == 0 {
			b.Fatal("normalizeRawJSON() = nil, want non-empty payload")
		}
	}
}

func BenchmarkSameRawJSONTrimmed256B(b *testing.B) {
	trimmed := json.RawMessage(strings.TrimSpace(string(benchmarkRawJSONWithWhitespace(256))))
	spaced := benchmarkRawJSONWithWhitespace(256)

	b.ReportAllocs()

	for b.Loop() {
		if !sameRawJSON(spaced, trimmed) {
			b.Fatal("sameRawJSON() = false, want true")
		}
	}
}

func benchmarkTerminalRuns(count int) []Run {
	runs := make([]Run, count)
	base := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)
	for i := range runs {
		runs[i] = Run{
			Status:   TaskRunStatusCompleted,
			Attempt:  int32(i + 1),
			QueuedAt: base.Add(time.Duration(i) * time.Second),
		}
	}
	return runs
}

func benchmarkRawJSONWithWhitespace(size int) json.RawMessage {
	if size < 16 {
		size = 16
	}
	payload := `{"value":"` + strings.Repeat("x", size-12) + `"}`
	return json.RawMessage(" \n\t" + payload + "\n\t ")
}

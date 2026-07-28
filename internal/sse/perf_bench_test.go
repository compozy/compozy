package sse

import (
	"context"
	"io"
	"strings"
	"testing"
)

func BenchmarkDecodeSingleLineEvents(b *testing.B) {
	benchmarkDecode(b, benchmarkDecodeCase{
		body: buildBenchmarkStream(strings.Join([]string{
			"id: 1",
			"event: message",
			`data: {"ok":true}`,
			"",
		}, "\n"), 32),
		wantEvents: 32,
	})
}

func BenchmarkDecodeMultiLineDataEvents(b *testing.B) {
	benchmarkDecode(b, benchmarkDecodeCase{
		body: buildBenchmarkStream(strings.Join([]string{
			"id: 1",
			"event: message",
			`data: {"part":1}`,
			`data: {"part":2}`,
			`data: {"part":3}`,
			"",
		}, "\n"), 32),
		wantEvents: 32,
	})
}

type benchmarkDecodeCase struct {
	body       string
	wantEvents int
}

func benchmarkDecode(b *testing.B, benchCase benchmarkDecodeCase) {
	b.Helper()
	b.ReportAllocs()

	ctx := context.Background()

	for b.Loop() {
		events := 0
		if err := Decode(ctx, io.NopCloser(strings.NewReader(benchCase.body)), func(Event) error {
			events++
			return nil
		}); err != nil {
			b.Fatalf("Decode() error = %v", err)
		}
		if events != benchCase.wantEvents {
			b.Fatalf("Decode() events = %d, want %d", events, benchCase.wantEvents)
		}
	}
}

func buildBenchmarkStream(frame string, count int) string {
	frames := make([]string, count)
	for i := range count {
		frames[i] = frame
	}
	return strings.Join(frames, "\n")
}

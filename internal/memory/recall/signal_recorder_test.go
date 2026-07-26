package recall

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func TestSignalRecorder(t *testing.T) {
	t.Parallel()

	t.Run("Should record submitted signals asynchronously", func(t *testing.T) {
		t.Parallel()

		source := newSignalRecorderFakeSource(nil)
		recorder := newTestSignalRecorder(t, source, SignalRecorderConfig{QueueCapacity: 2})
		result := recorder.Submit(t.Context(), memcontract.Query{QueryText: "alpha beta"}, []Signal{
			{ChunkID: "chunk-1", Score: 0.9},
		})
		if !result.Submitted || result.Dropped {
			t.Fatalf("Submit() = %#v, want submitted without drop", result)
		}

		closeSignalRecorder(t, recorder)
		stats := recorder.Stats()
		if stats.Submitted != 1 || stats.Recorded != 1 || stats.Failed != 0 || stats.Dropped != 0 {
			t.Fatalf("SignalRecorder stats = %#v, want one recorded signal", stats)
		}
		if got := source.recordedChunkIDs(); len(got) != 1 || got[0] != "chunk-1" {
			t.Fatalf("recorded chunks = %#v, want chunk-1", got)
		}
	})

	t.Run("Should emit failure event after retries fail", func(t *testing.T) {
		t.Parallel()

		source := newSignalRecorderFakeSource(errors.New("catalog busy"))
		recorder := newTestSignalRecorder(t, source, SignalRecorderConfig{QueueCapacity: 2, WorkerRetryMax: 1})
		recorder.Submit(t.Context(), memcontract.Query{QueryText: "alpha beta"}, []Signal{
			{ChunkID: "chunk-failed", Score: 0.9},
		})

		closeSignalRecorder(t, recorder)
		stats := recorder.Stats()
		if stats.Failed != 1 || stats.Recorded != 0 {
			t.Fatalf("SignalRecorder stats = %#v, want one failed signal", stats)
		}
		if failures := source.failureCount(); failures != 1 {
			t.Fatalf("failure events = %d, want 1", failures)
		}
	})

	t.Run("Should drop oldest queued batch when the queue overflows", func(t *testing.T) {
		t.Parallel()

		release := make(chan struct{})
		source := newSignalRecorderFakeSource(nil)
		source.release = release
		recorder := newTestSignalRecorder(t, source, SignalRecorderConfig{QueueCapacity: 1})

		recorder.Submit(t.Context(), memcontract.Query{QueryText: "first query"}, []Signal{
			{ChunkID: "chunk-1", Score: 0.9},
		})
		source.waitForFirstRecord(t)

		recorder.Submit(t.Context(), memcontract.Query{QueryText: "old query"}, []Signal{
			{ChunkID: "chunk-2", Score: 0.8},
		})
		result := recorder.Submit(t.Context(), memcontract.Query{QueryText: "new query"}, []Signal{
			{ChunkID: "chunk-3", Score: 0.7},
		})
		if !result.Submitted || !result.Dropped {
			t.Fatalf("Submit(overflow) = %#v, want submitted with drop", result)
		}

		close(release)
		closeSignalRecorder(t, recorder)
		stats := recorder.Stats()
		if stats.Dropped != 1 || stats.Recorded != 2 {
			t.Fatalf("SignalRecorder stats = %#v, want one dropped and two recorded", stats)
		}
		if got := source.recordedChunkIDs(); len(got) != 2 || got[0] != "chunk-1" || got[1] != "chunk-3" {
			t.Fatalf("recorded chunks = %#v, want chunk-1 and chunk-3", got)
		}
		if got := source.droppedChunkIDs(); len(got) != 1 || got[0] != "chunk-2" {
			t.Fatalf("dropped chunks = %#v, want chunk-2", got)
		}
		if got := source.droppedQueryTexts(); len(got) != 1 || got[0] != "old query" {
			t.Fatalf("dropped queries = %#v, want old query", got)
		}
	})

	t.Run("Should reject submissions after the worker has stopped", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		source := newSignalRecorderFakeSource(nil)
		recorder, err := NewSignalRecorder(
			ctx,
			source,
			SignalRecorderConfig{QueueCapacity: 2},
			slog.New(slog.DiscardHandler),
		)
		if err != nil {
			t.Fatalf("NewSignalRecorder() error = %v", err)
		}
		cancel()
		waitSignalRecorderStopped(t, recorder)

		result := recorder.Submit(t.Context(), memcontract.Query{QueryText: "late query"}, []Signal{
			{ChunkID: "chunk-late", Score: 0.8},
		})
		if result.Submitted || result.Dropped {
			t.Fatalf("Submit(after worker stopped) = %#v, want not submitted", result)
		}
		stats := recorder.Stats()
		if stats.Submitted != 0 || stats.Recorded != 0 || stats.Failed != 0 || stats.Dropped != 0 {
			t.Fatalf("SignalRecorder stats after stopped submit = %#v, want zero counters", stats)
		}
	})
}

func newTestSignalRecorder(t *testing.T, source *signalRecorderFakeSource, cfg SignalRecorderConfig) *SignalRecorder {
	t.Helper()

	recorder, err := NewSignalRecorder(t.Context(), source, cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewSignalRecorder() error = %v", err)
	}
	return recorder
}

func closeSignalRecorder(t *testing.T, recorder *SignalRecorder) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if err := recorder.Close(ctx); err != nil {
		t.Fatalf("SignalRecorder.Close() error = %v", err)
	}
}

func waitSignalRecorderStopped(t *testing.T, recorder *SignalRecorder) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		recorder.wg.Wait()
		close(done)
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatalf("wait for SignalRecorder worker: %v", ctx.Err())
	}
}

type signalRecorderFakeSource struct {
	mu             sync.Mutex
	err            error
	release        <-chan struct{}
	started        chan struct{}
	startedOnce    sync.Once
	recorded       []Signal
	dropped        []Signal
	droppedQueries []memcontract.Query
	failures       []error
}

func newSignalRecorderFakeSource(err error) *signalRecorderFakeSource {
	return &signalRecorderFakeSource{
		err:     err,
		started: make(chan struct{}),
	}
}

func (f *signalRecorderFakeSource) RecordRecall(_ context.Context, signals []Signal) error {
	f.startedOnce.Do(func() {
		close(f.started)
	})
	if f.release != nil {
		<-f.release
	}
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recorded = append(f.recorded, signals...)
	return nil
}

func (f *signalRecorderFakeSource) RecordRecallSignalFailed(
	_ context.Context,
	_ memcontract.Query,
	cause error,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failures = append(f.failures, cause)
	return nil
}

func (f *signalRecorderFakeSource) RecordRecallSignalDropped(
	_ context.Context,
	query memcontract.Query,
	signals []Signal,
	_ int,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dropped = append(f.dropped, signals...)
	f.droppedQueries = append(f.droppedQueries, query)
	return nil
}

func (f *signalRecorderFakeSource) waitForFirstRecord(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	select {
	case <-f.started:
	case <-ctx.Done():
		t.Fatalf("wait for first RecordRecall: %v", ctx.Err())
	}
}

func (f *signalRecorderFakeSource) recordedChunkIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return signalChunkIDs(f.recorded)
}

func (f *signalRecorderFakeSource) droppedChunkIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return signalChunkIDs(f.dropped)
}

func (f *signalRecorderFakeSource) droppedQueryTexts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	texts := make([]string, 0, len(f.droppedQueries))
	for _, query := range f.droppedQueries {
		texts = append(texts, query.QueryText)
	}
	return texts
}

func (f *signalRecorderFakeSource) failureCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.failures)
}

func signalChunkIDs(signals []Signal) []string {
	ids := make([]string, 0, len(signals))
	for _, signal := range signals {
		ids = append(ids, signal.ChunkID)
	}
	return ids
}

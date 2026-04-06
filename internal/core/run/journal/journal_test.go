package journal

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/runs"
)

func TestJournalAssignsGapFreeSequencesAndPublishesMatchingBusEvents(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	runID := "journal-seq"
	prepareRunLayout(t, workspaceRoot, runID)

	bus := events.New[events.Event](16)
	_, updates, unsubscribe := bus.Subscribe()
	defer unsubscribe()

	journal, eventsPath := openTestJournal(t, workspaceRoot, runID, bus, 16, openOptions{
		batchSize:     3,
		flushInterval: time.Second,
	})

	for i := 1; i <= 3; i++ {
		if err := journal.Submit(
			context.Background(),
			testJournalEvent(runID, events.EventKindJobStarted, i),
		); err != nil {
			t.Fatalf("Submit() error = %v", err)
		}
	}

	published := collectBusEvents(t, updates, 3, time.Second)
	if err := journal.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	replayed := replayRunEvents(t, workspaceRoot, runID, 0)
	if got := collectedSeqs(replayed); !slices.Equal(got, []uint64{1, 2, 3}) {
		t.Fatalf("replayed seqs = %v, want [1 2 3]", got)
	}
	if got := collectedSeqs(published); !slices.Equal(got, collectedSeqs(replayed)) {
		t.Fatalf("published seqs = %v, want %v", collectedSeqs(published), collectedSeqs(replayed))
	}
	if journal.EventsWritten() != 3 {
		t.Fatalf("EventsWritten() = %d, want 3", journal.EventsWritten())
	}
	if _, err := os.Stat(eventsPath); err != nil {
		t.Fatalf("expected events file to exist: %v", err)
	}
}

func TestJournalFlushesWhenBatchSizeReached(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	runID := "journal-batch"
	prepareRunLayout(t, workspaceRoot, runID)

	bus := events.New[events.Event](16)
	_, updates, unsubscribe := bus.Subscribe()
	defer unsubscribe()

	journal, _ := openTestJournal(t, workspaceRoot, runID, bus, 16, openOptions{
		batchSize:     3,
		flushInterval: time.Hour,
	})

	for i := 1; i <= 3; i++ {
		if err := journal.Submit(
			context.Background(),
			testJournalEvent(runID, events.EventKindJobStarted, i),
		); err != nil {
			t.Fatalf("Submit() error = %v", err)
		}
	}

	published := collectBusEvents(t, updates, 3, time.Second)
	if got := collectedSeqs(published); !slices.Equal(got, []uint64{1, 2, 3}) {
		t.Fatalf("published seqs = %v, want [1 2 3]", got)
	}
	if err := journal.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestJournalFlushesOnInterval(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	runID := "journal-interval"
	prepareRunLayout(t, workspaceRoot, runID)

	bus := events.New[events.Event](16)
	_, updates, unsubscribe := bus.Subscribe()
	defer unsubscribe()

	journal, _ := openTestJournal(t, workspaceRoot, runID, bus, 16, openOptions{
		batchSize:     32,
		flushInterval: 20 * time.Millisecond,
	})

	if err := journal.Submit(context.Background(), testJournalEvent(runID, events.EventKindJobStarted, 1)); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	published := collectBusEvents(t, updates, 1, time.Second)
	if got := collectedSeqs(published); !slices.Equal(got, []uint64{1}) {
		t.Fatalf("published seqs = %v, want [1]", got)
	}
	if err := journal.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestJournalTerminalEventForcesImmediateSync(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	runID := "journal-terminal"
	prepareRunLayout(t, workspaceRoot, runID)

	bus := events.New[events.Event](16)
	_, updates, unsubscribe := bus.Subscribe()
	defer unsubscribe()

	synced := make(chan struct{}, 2)
	journal, _ := openTestJournal(t, workspaceRoot, runID, bus, 16, openOptions{
		batchSize:     32,
		flushInterval: time.Hour,
		afterSync: func() {
			select {
			case synced <- struct{}{}:
			default:
			}
		},
	})

	if err := journal.Submit(context.Background(), testJournalEvent(runID, events.EventKindJobStarted, 1)); err != nil {
		t.Fatalf("Submit(started) error = %v", err)
	}
	if err := journal.Submit(
		context.Background(),
		testJournalEvent(runID, events.EventKindRunCompleted, 2),
	); err != nil {
		t.Fatalf("Submit(completed) error = %v", err)
	}

	select {
	case <-synced:
	case <-time.After(time.Second):
		t.Fatal("expected terminal submit to force sync")
	}

	published := collectBusEvents(t, updates, 2, time.Second)
	if got := collectedSeqs(published); !slices.Equal(got, []uint64{1, 2}) {
		t.Fatalf("published seqs = %v, want [1 2]", got)
	}
	if err := journal.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestJournalCloseDrainsQueuedEvents(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	runID := "journal-close"
	prepareRunLayout(t, workspaceRoot, runID)

	journal, _ := openTestJournal(t, workspaceRoot, runID, nil, 16, openOptions{
		batchSize:     32,
		flushInterval: time.Hour,
	})

	for i := 1; i <= 5; i++ {
		if err := journal.Submit(
			context.Background(),
			testJournalEvent(runID, events.EventKindJobStarted, i),
		); err != nil {
			t.Fatalf("Submit() error = %v", err)
		}
	}

	if err := journal.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	replayed := replayRunEvents(t, workspaceRoot, runID, 0)
	if got := collectedSeqs(replayed); !slices.Equal(got, []uint64{1, 2, 3, 4, 5}) {
		t.Fatalf("replayed seqs = %v, want [1 2 3 4 5]", got)
	}
}

func TestJournalCloseReturnsContextErrorWithoutHanging(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	runID := "journal-close-timeout"
	prepareRunLayout(t, workspaceRoot, runID)

	journal, _ := openTestJournal(t, workspaceRoot, runID, nil, 16, openOptions{})
	if err := journal.Submit(context.Background(), testJournalEvent(runID, events.EventKindJobStarted, 1)); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := journal.Close(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Close() error = %v, want context canceled", err)
	}
	select {
	case <-journal.done:
	case <-time.After(time.Second):
		t.Fatal("expected writer to exit after close request")
	}
}

func TestJournalSubmitReturnsContextErrorBeforeInboxAccepts(t *testing.T) {
	t.Parallel()

	journal := &Journal{
		runID:          "journal-submit-context",
		inbox:          make(chan events.Event, 1),
		closeRequested: make(chan struct{}),
		done:           make(chan struct{}),
		submitTimeout:  time.Second,
	}
	journal.inbox <- testJournalEvent("journal-submit-context", events.EventKindJobStarted, 1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := journal.Submit(
		ctx,
		testJournalEvent("journal-submit-context", events.EventKindJobStarted, 2),
	); !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatalf("Submit() error = %v, want context canceled", err)
	}
}

func TestJournalSubmitTimeoutIncrementsDrops(t *testing.T) {
	t.Parallel()

	journal := &Journal{
		runID:          "journal-submit-timeout",
		inbox:          make(chan events.Event, 1),
		closeRequested: make(chan struct{}),
		done:           make(chan struct{}),
		submitTimeout:  10 * time.Millisecond,
	}
	journal.inbox <- testJournalEvent("journal-submit-timeout", events.EventKindJobStarted, 1)

	err := journal.Submit(
		context.Background(),
		testJournalEvent("journal-submit-timeout", events.EventKindJobStarted, 2),
	)
	if !errors.Is(err, ErrSubmitTimeout) {
		t.Fatalf("Submit() error = %v, want ErrSubmitTimeout", err)
	}
	if journal.DropsOnSubmit() != 1 {
		t.Fatalf("DropsOnSubmit() = %d, want 1", journal.DropsOnSubmit())
	}
}

func TestJournalConcurrentSubmitProducesGapFreeSequence(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	runID := "journal-concurrent"
	prepareRunLayout(t, workspaceRoot, runID)

	journal, _ := openTestJournal(t, workspaceRoot, runID, nil, 128, openOptions{
		batchSize:     32,
		flushInterval: 10 * time.Millisecond,
	})

	var wg sync.WaitGroup
	for worker := 0; worker < 10; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for idx := 0; idx < 20; idx++ {
				err := journal.Submit(
					context.Background(),
					testJournalEvent(runID, events.EventKindJobStarted, worker*100+idx),
				)
				if err != nil {
					t.Errorf("Submit() error = %v", err)
					return
				}
			}
		}(worker)
	}
	wg.Wait()

	if err := journal.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	replayed := replayRunEvents(t, workspaceRoot, runID, 0)
	if len(replayed) != 200 {
		t.Fatalf("replayed events = %d, want 200", len(replayed))
	}
	for idx, ev := range replayed {
		want := uint64(idx + 1)
		if ev.Seq != want {
			t.Fatalf("event[%d].Seq = %d, want %d", idx, ev.Seq, want)
		}
	}
}

func TestJournalFlushHookSupportsCrashRecoveryReplay(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	runID := "journal-crash"
	runDir := prepareRunLayout(t, workspaceRoot, runID)
	eventsPath := filepath.Join(runDir, "events.jsonl")

	var injected atomic.Bool
	journal, _ := openTestJournal(t, workspaceRoot, runID, nil, 16, openOptions{
		batchSize:     2,
		flushInterval: time.Hour,
		flushHook: func() {
			if !injected.CompareAndSwap(false, true) {
				return
			}
			file, err := os.OpenFile(eventsPath, os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				t.Errorf("open partial tail: %v", err)
				return
			}
			defer func() {
				_ = file.Close()
			}()
			if _, err := file.WriteString(`{"schema_version":"1.0","run_id":"journal-crash"`); err != nil {
				t.Errorf("write partial tail: %v", err)
			}
		},
	})

	for i := 1; i <= 2; i++ {
		if err := journal.Submit(
			context.Background(),
			testJournalEvent(runID, events.EventKindJobStarted, i),
		); err != nil {
			t.Fatalf("Submit() error = %v", err)
		}
	}
	if err := journal.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	run, err := runs.Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("runs.Open() error = %v", err)
	}

	var (
		replayed  []events.Event
		replayErr error
	)
	for ev, err := range run.Replay(0) {
		if err != nil {
			replayErr = err
			break
		}
		replayed = append(replayed, ev)
	}

	if got := collectedSeqs(replayed); !slices.Equal(got, []uint64{1, 2}) {
		t.Fatalf("replayed seqs = %v, want [1 2]", got)
	}
	if !errors.Is(replayErr, runs.ErrPartialEventLine) {
		t.Fatalf("replay error = %v, want partial-final-line error", replayErr)
	}
	if _, err := os.Stat(filepath.Join(runDir, "events.jsonl")); err != nil {
		t.Fatalf("expected events.jsonl to exist: %v", err)
	}
}

func openTestJournal(
	t *testing.T,
	workspaceRoot string,
	runID string,
	bus *events.Bus[events.Event],
	bufCap int,
	opts openOptions,
) (*Journal, string) {
	t.Helper()

	runDir := filepath.Join(workspaceRoot, ".compozy", "runs", runID)
	eventsPath := filepath.Join(runDir, "events.jsonl")
	journal, err := openWithOptions(eventsPath, bus, bufCap, opts)
	if err != nil {
		t.Fatalf("openWithOptions() error = %v", err)
	}
	return journal, eventsPath
}

func prepareRunLayout(t *testing.T, workspaceRoot, runID string) string {
	t.Helper()

	runDir := filepath.Join(workspaceRoot, ".compozy", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	runJSON := map[string]any{
		"run_id":         runID,
		"status":         "running",
		"mode":           "prd-tasks",
		"ide":            "codex",
		"model":          "gpt-5.4",
		"workspace_root": workspaceRoot,
		"artifacts_dir":  runDir,
		"created_at":     time.Now().UTC(),
	}
	payload, err := json.Marshal(runJSON)
	if err != nil {
		t.Fatalf("marshal run.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "run.json"), payload, 0o600); err != nil {
		t.Fatalf("write run.json: %v", err)
	}
	return runDir
}

func collectBusEvents(
	t *testing.T,
	ch <-chan events.Event,
	want int,
	timeout time.Duration,
) []events.Event {
	t.Helper()

	got := make([]events.Event, 0, want)
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for len(got) < want {
		select {
		case ev := <-ch:
			got = append(got, ev)
		case <-deadline.C:
			t.Fatalf("timed out waiting for %d bus events, got %d", want, len(got))
		}
	}
	return got
}

func replayRunEvents(t *testing.T, workspaceRoot, runID string, fromSeq uint64) []events.Event {
	t.Helper()

	run, err := runs.Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("runs.Open() error = %v", err)
	}

	var replayed []events.Event
	for ev, err := range run.Replay(fromSeq) {
		if err != nil {
			t.Fatalf("Replay() error = %v", err)
		}
		replayed = append(replayed, ev)
	}
	return replayed
}

func collectedSeqs(items []events.Event) []uint64 {
	seqs := make([]uint64, 0, len(items))
	for _, item := range items {
		seqs = append(seqs, item.Seq)
	}
	return seqs
}

func testJournalEvent(runID string, kind events.EventKind, index int) events.Event {
	return events.Event{
		SchemaVersion: events.SchemaVersion,
		RunID:         runID,
		Timestamp:     time.Unix(int64(index), 0).UTC(),
		Kind:          kind,
		Payload:       json.RawMessage(`{"index":1}`),
	}
}

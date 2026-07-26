//go:build integration

package sessiondb

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestSessionDBLifecyclePersistsAcrossReopen(t *testing.T) {
	t.Parallel()

	t.Run("Should persist events and usage across reopen", func(t *testing.T) {
		t.Parallel()

		sessionDir := t.TempDir()
		path := filepath.Join(sessionDir, SessionDatabaseName)
		ctx := testutil.Context(t)

		sessionDB, err := OpenSessionDB(ctx, "sess-integration", path)
		if err != nil {
			t.Fatalf("OpenSessionDB() error = %v", err)
		}
		sessionDB.now = func() time.Time {
			return time.Date(2026, 4, 3, 19, 0, 0, 0, time.UTC)
		}

		if err := sessionDB.Record(ctx, SessionEvent{
			TurnID:    "turn-1",
			Type:      "agent_message",
			AgentName: "coder",
			Content:   `{"text":"hello"}`,
		}); err != nil {
			t.Fatalf("Record() error = %v", err)
		}
		if err := sessionDB.RecordTokenUsage(ctx, TokenUsage{
			TurnID:       "turn-1",
			OutputTokens: int64Pointer(42),
		}); err != nil {
			t.Fatalf("RecordTokenUsage() error = %v", err)
		}
		if err := sessionDB.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}

		reopened, err := OpenSessionDB(ctx, "sess-integration", path)
		if err != nil {
			t.Fatalf("OpenSessionDB(reopen) error = %v", err)
		}
		defer func() {
			if closeErr := reopened.Close(ctx); closeErr != nil {
				t.Fatalf("Close() error = %v", closeErr)
			}
		}()

		events, err := reopened.Query(ctx, EventQuery{})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("len(events) = %d, want %d", got, want)
		}
		if events[0].Sequence != 1 || events[0].TurnID != "turn-1" {
			t.Fatalf("events[0] = %#v, want sequence=1 turn-1", events[0])
		}
	})
}

func TestSessionDBSupportsConcurrentReadersWithSingleWriter(t *testing.T) {
	t.Parallel()

	t.Run("Should support concurrent readers with a single writer", func(t *testing.T) {
		t.Parallel()

		sessionDB := openTestSessionDB(t, "sess-concurrency")
		ctx := testutil.Context(t)

		const (
			readerCount = 6
			eventCount  = 150
		)

		errCh := make(chan error, readerCount+1)
		var writerWG sync.WaitGroup
		writerWG.Add(1)
		go func() {
			defer writerWG.Done()
			for i := 0; i < eventCount; i++ {
				if err := sessionDB.Record(ctx, SessionEvent{
					TurnID:    fmt.Sprintf("turn-%03d", i),
					Type:      "agent_message",
					AgentName: "coder",
					Content:   fmt.Sprintf(`{"index":%d}`, i),
				}); err != nil {
					errCh <- fmt.Errorf("writer: %w", err)
					return
				}
			}
		}()

		var readersWG sync.WaitGroup
		for i := 0; i < readerCount; i++ {
			readersWG.Add(1)
			go func() {
				defer readersWG.Done()
				for j := 0; j < eventCount; j++ {
					events, err := sessionDB.Query(ctx, EventQuery{Limit: 10})
					if err != nil {
						errCh <- fmt.Errorf("reader: %w", err)
						return
					}
					if len(events) > 10 {
						errCh <- fmt.Errorf("reader: len(events) = %d, want <= 10", len(events))
						return
					}
				}
			}()
		}

		writerWG.Wait()
		readersWG.Wait()
		close(errCh)

		for err := range errCh {
			if err != nil {
				t.Fatal(err)
			}
		}

		events, err := sessionDB.Query(ctx, EventQuery{})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if got, want := len(events), eventCount; got != want {
			t.Fatalf("len(events) = %d, want %d", got, want)
		}
	})
}

func TestReadOnlyPoolServesConcurrentInactiveReads(t *testing.T) {
	t.Parallel()

	t.Run("Should reuse same-session read-only handles until TTL expiry", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		writer, err := OpenSessionDB(ctx, "sess-read-only-pool", path)
		if err != nil {
			t.Fatalf("OpenSessionDB() error = %v", err)
		}
		for index := range 3 {
			if err := writer.Record(ctx, SessionEvent{
				TurnID:    fmt.Sprintf("turn-%d", index),
				Type:      "agent_message",
				AgentName: "coder",
				Content:   fmt.Sprintf(`{"text":"message-%d"}`, index),
			}); err != nil {
				t.Fatalf("Record(%d) error = %v", index, err)
			}
		}
		if err := writer.Close(ctx); err != nil {
			t.Fatalf("Close(writer) error = %v", err)
		}

		var (
			openCount  atomic.Int64
			closeCount atomic.Int64
			clockMu    sync.Mutex
			now        = time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
		)
		currentTime := func() time.Time {
			clockMu.Lock()
			defer clockMu.Unlock()
			return now
		}
		advanceTime := func(delta time.Duration) {
			clockMu.Lock()
			defer clockMu.Unlock()
			now = now.Add(delta)
		}
		pool := NewReadOnlyPool(ReadOnlyPoolConfig{
			TTL: time.Second,
			Now: currentTime,
			Open: func(ctx context.Context, sessionID string, path string) (store.EventRecorder, error) {
				openCount.Add(1)
				reader, err := OpenSessionDBReadOnly(ctx, sessionID, path)
				if err != nil {
					return nil, err
				}
				return &closeCountingRecorder{
					EventRecorder: reader,
					onClose: func() {
						closeCount.Add(1)
					},
				}, nil
			},
		})
		t.Cleanup(func() {
			if err := pool.Close(testutil.Context(t)); err != nil {
				t.Fatalf("Close(pool cleanup) error = %v", err)
			}
		})

		const readerCount = 8
		var readersWG sync.WaitGroup
		errCh := make(chan error, readerCount)
		for range readerCount {
			readersWG.Add(1)
			go func() {
				defer readersWG.Done()
				lease, err := pool.Open(ctx, "sess-read-only-pool", path)
				if err != nil {
					errCh <- fmt.Errorf("Open(pool): %w", err)
					return
				}
				events, err := lease.Query(ctx, EventQuery{Limit: 2})
				if err != nil {
					errCh <- fmt.Errorf("Query(pool): %w", err)
					return
				}
				if got, want := len(events), 2; got != want {
					errCh <- fmt.Errorf("len(events) = %d, want %d", got, want)
					return
				}
				if err := lease.Close(ctx); err != nil {
					errCh <- fmt.Errorf("Close(lease): %w", err)
					return
				}
			}()
		}
		readersWG.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				t.Fatal(err)
			}
		}
		if got, want := openCount.Load(), int64(1); got != want {
			t.Fatalf("read-only open count after concurrent reads = %d, want %d", got, want)
		}
		if got, want := closeCount.Load(), int64(0); got != want {
			t.Fatalf("underlying close count before TTL = %d, want %d", got, want)
		}

		advanceTime(2 * time.Second)
		if err := pool.CloseExpired(ctx); err != nil {
			t.Fatalf("CloseExpired() error = %v", err)
		}
		if got, want := closeCount.Load(), int64(1); got != want {
			t.Fatalf("underlying close count after TTL = %d, want %d", got, want)
		}

		lease, err := pool.Open(ctx, "sess-read-only-pool", path)
		if err != nil {
			t.Fatalf("Open(pool after TTL) error = %v", err)
		}
		if got, want := openCount.Load(), int64(2); got != want {
			t.Fatalf("read-only open count after TTL = %d, want %d", got, want)
		}
		if err := lease.Close(ctx); err != nil {
			t.Fatalf("Close(lease after TTL) error = %v", err)
		}
		if err := pool.Close(ctx); err != nil {
			t.Fatalf("Close(pool) error = %v", err)
		}
		if got, want := closeCount.Load(), int64(2); got != want {
			t.Fatalf("underlying close count after pool close = %d, want %d", got, want)
		}
	})

	t.Run("Should keep read-only handles isolated by session id", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		writer, err := OpenSessionDB(ctx, "sess-read-only-pool-a", path)
		if err != nil {
			t.Fatalf("OpenSessionDB() error = %v", err)
		}
		if err := writer.Record(ctx, SessionEvent{
			TurnID:    "turn-1",
			Type:      "agent_message",
			AgentName: "coder",
			Content:   `{"text":"message"}`,
		}); err != nil {
			t.Fatalf("Record() error = %v", err)
		}
		if err := writer.Close(ctx); err != nil {
			t.Fatalf("Close(writer) error = %v", err)
		}

		var (
			openCount  atomic.Int64
			closeCount atomic.Int64
		)
		pool := NewReadOnlyPool(ReadOnlyPoolConfig{
			TTL: time.Second,
			Open: func(ctx context.Context, sessionID string, path string) (store.EventRecorder, error) {
				openCount.Add(1)
				reader, err := OpenSessionDBReadOnly(ctx, sessionID, path)
				if err != nil {
					return nil, err
				}
				return &closeCountingRecorder{
					EventRecorder: reader,
					onClose: func() {
						closeCount.Add(1)
					},
				}, nil
			},
		})
		t.Cleanup(func() {
			if err := pool.Close(testutil.Context(t)); err != nil {
				t.Fatalf("Close(pool cleanup) error = %v", err)
			}
		})

		first, err := pool.Open(ctx, "sess-read-only-pool-a", path)
		if err != nil {
			t.Fatalf("Open(first) error = %v", err)
		}
		second, err := pool.Open(ctx, "sess-read-only-pool-b", path)
		if err != nil {
			t.Fatalf("Open(second) error = %v", err)
		}
		if got, want := openCount.Load(), int64(2); got != want {
			t.Fatalf("read-only open count for distinct sessions = %d, want %d", got, want)
		}
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first lease) error = %v", err)
		}
		if err := second.Close(ctx); err != nil {
			t.Fatalf("Close(second lease) error = %v", err)
		}
		if err := pool.Close(ctx); err != nil {
			t.Fatalf("Close(pool) error = %v", err)
		}
		if got, want := closeCount.Load(), int64(2); got != want {
			t.Fatalf("underlying close count = %d, want %d", got, want)
		}
	})

	t.Run("Should unblock in-flight quiescence when the pool closes", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		writer, err := OpenSessionDB(ctx, "sess-read-only-pool-shutdown", path)
		if err != nil {
			t.Fatalf("OpenSessionDB() error = %v", err)
		}
		if err := writer.Close(ctx); err != nil {
			t.Fatalf("Close(writer) error = %v", err)
		}

		pool := NewReadOnlyPool(ReadOnlyPoolConfig{})
		lease, err := pool.Open(ctx, "sess-read-only-pool-shutdown", path)
		if err != nil {
			t.Fatalf("Open(pool) error = %v", err)
		}
		quiesceDone := make(chan error, 1)
		go func() {
			_, quiesceErr := pool.Quiesce(ctx, "sess-read-only-pool-shutdown", path)
			quiesceDone <- quiesceErr
		}()

		for {
			probe, openErr := pool.Open(ctx, "sess-read-only-pool-shutdown", path)
			if errors.Is(openErr, ErrReadOnlyPoolQuiescing) {
				break
			}
			if openErr != nil {
				t.Fatalf("Open(quiescence probe) error = %v", openErr)
			}
			if closeErr := probe.Close(ctx); closeErr != nil {
				t.Fatalf("Close(quiescence probe) error = %v", closeErr)
			}
			runtime.Gosched()
		}

		if err := pool.Close(ctx); err != nil {
			t.Fatalf("Close(pool) error = %v", err)
		}
		if err := lease.Close(ctx); err != nil {
			t.Fatalf("Close(lease) error = %v", err)
		}
		select {
		case quiesceErr := <-quiesceDone:
			if !errors.Is(quiesceErr, errReadOnlyPoolClosed) {
				t.Fatalf("Quiesce() error = %v, want errReadOnlyPoolClosed", quiesceErr)
			}
		case <-ctx.Done():
			t.Fatalf("Quiesce() did not return after pool close: %v", ctx.Err())
		}
	})
}

type closeCountingRecorder struct {
	store.EventRecorder
	onClose func()
	once    sync.Once
}

func (r *closeCountingRecorder) Close(ctx context.Context) error {
	err := r.EventRecorder.Close(ctx)
	r.once.Do(func() {
		if r.onClose != nil {
			r.onClose()
		}
	})
	return err
}

func int64Pointer(value int64) *int64 {
	return &value
}

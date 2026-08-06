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
	"testing/synctest"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestSessionDBLifecyclePersistsAcrossReopen(t *testing.T) {
	t.Parallel()

	t.Run("Should persist events and usage across reopen", func(t *testing.T) {
		t.Parallel()

		sessionDir := t.TempDir()
		path := filepath.Join(sessionDir, SessionDatabaseName)
		owner := testSessionDBOwner("sess-integration")
		ctx := testutil.Context(t)

		sessionDB, err := OpenSessionDB(ctx, owner, path)
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

		reopened, err := OpenSessionDB(ctx, owner, path)
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

func TestReadOnlyPoolLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve conversation rewind reads through a pooled lease", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		pool := NewReadOnlyPool(ReadOnlyPoolConfig{
			Open: func(context.Context, store.SessionDBOwner, string) (store.EventReadCloser, error) {
				return &readOnlyPoolTestReader{}, nil
			},
		})
		lease, err := pool.Open(ctx, testSessionDBOwner("sess-rewind-reader"), "rewind.db")
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		reader, ok := lease.(store.ConversationRewindReader)
		if !ok {
			t.Fatalf("pooled lease = %T, want ConversationRewindReader", lease)
		}
		target, err := reader.ConversationRewindTarget(ctx, "msg-rewind")
		if err != nil {
			t.Fatalf("ConversationRewindTarget() error = %v", err)
		}
		if target.MessageID != "msg-rewind" {
			t.Fatalf("ConversationRewindTarget() = %#v", target)
		}
		if err := lease.Close(ctx); err != nil {
			t.Fatalf("Close(lease) error = %v", err)
		}
		if err := pool.Close(ctx); err != nil {
			t.Fatalf("Close(pool) error = %v", err)
		}
	})

	t.Run("Should keep the same session path isolated by workspace owner", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		var openCount atomic.Int64
		pool := NewReadOnlyPool(ReadOnlyPoolConfig{
			Open: func(context.Context, store.SessionDBOwner, string) (store.EventReadCloser, error) {
				openCount.Add(1)
				return &readOnlyPoolTestReader{}, nil
			},
		})
		first, err := pool.Open(ctx, store.SessionDBOwner{
			SessionID: "sess-shared", WorkspaceID: "ws-first",
		}, path)
		if err != nil {
			t.Fatalf("Open(first workspace) error = %v", err)
		}
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first workspace lease) error = %v", err)
		}
		second, err := pool.Open(ctx, store.SessionDBOwner{
			SessionID: "sess-shared", WorkspaceID: "ws-second",
		}, path)
		if err != nil {
			t.Fatalf("Open(second workspace) error = %v", err)
		}
		if err := second.Close(ctx); err != nil {
			t.Fatalf("Close(second workspace lease) error = %v", err)
		}
		if got := openCount.Load(); got != 2 {
			t.Fatalf("pool opener calls = %d, want 2 workspace-isolated entries", got)
		}
		if err := pool.Close(ctx); err != nil {
			t.Fatalf("Close(pool) error = %v", err)
		}
	})

	t.Run("Should admit a different session while an opener re-enters the pool", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		t.Cleanup(cancel)

		var pool *ReadOnlyPool
		pool = NewReadOnlyPool(ReadOnlyPoolConfig{
			Open: func(ctx context.Context, owner store.SessionDBOwner, path string) (store.EventReadCloser, error) {
				sessionID := owner.SessionID
				if sessionID == "sess-read-only-reentrant" {
					lease, err := pool.Open(ctx, testSessionDBOwner("sess-read-only-independent"), path+".independent")
					if err != nil {
						return nil, fmt.Errorf("open re-entrant independent reader: %w", err)
					}
					if err := lease.Close(ctx); err != nil {
						return nil, fmt.Errorf("close re-entrant independent reader lease: %w", err)
					}
				}
				return &readOnlyPoolTestReader{}, nil
			},
		})

		result := make(chan error, 1)
		go func() {
			lease, err := pool.Open(ctx, testSessionDBOwner("sess-read-only-reentrant"), "reentrant.db")
			if err == nil {
				err = lease.Close(ctx)
			}
			result <- err
		}()

		select {
		case err := <-result:
			if err != nil {
				t.Fatalf("Open(re-entrant) error = %v", err)
			}
		case <-ctx.Done():
			t.Fatalf("Open(re-entrant) did not admit an unrelated key: %v", ctx.Err())
		}

		if err := pool.Close(ctx); err != nil {
			t.Fatalf("Close(pool) error = %v", err)
		}
	})

	t.Run("Should admit a different session while an expired recorder close re-enters the pool", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		t.Cleanup(cancel)

		now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
		var pool *ReadOnlyPool
		pool = NewReadOnlyPool(ReadOnlyPoolConfig{
			TTL: time.Second,
			Now: func() time.Time {
				return now
			},
			Open: func(ctx context.Context, owner store.SessionDBOwner, path string) (store.EventReadCloser, error) {
				sessionID := owner.SessionID
				reader := &readOnlyPoolTestReader{}
				if sessionID == "sess-read-only-expired" {
					reader.onClose = func(ctx context.Context) error {
						lease, err := pool.Open(ctx, testSessionDBOwner("sess-read-only-independent"), path+".independent")
						if err != nil {
							return fmt.Errorf("open independent reader during expired close: %w", err)
						}
						if err := lease.Close(ctx); err != nil {
							return fmt.Errorf("close independent reader lease during expired close: %w", err)
						}
						return nil
					}
				}
				return reader, nil
			},
		})

		lease, err := pool.Open(ctx, testSessionDBOwner("sess-read-only-expired"), "expired.db")
		if err != nil {
			t.Fatalf("Open(expired) error = %v", err)
		}
		if err := lease.Close(ctx); err != nil {
			t.Fatalf("Close(expired lease) error = %v", err)
		}
		now = now.Add(2 * time.Second)

		result := make(chan error, 1)
		go func() {
			result <- pool.CloseExpired(ctx)
		}()

		select {
		case err := <-result:
			if err != nil {
				t.Fatalf("CloseExpired() error = %v", err)
			}
		case <-ctx.Done():
			t.Fatalf("CloseExpired() did not admit an unrelated key: %v", ctx.Err())
		}

		if err := pool.Close(ctx); err != nil {
			t.Fatalf("Close(pool) error = %v", err)
		}
	})

	t.Run("Should reject a re-entrant open without holding the mutex during pool close", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		t.Cleanup(cancel)

		var pool *ReadOnlyPool
		pool = NewReadOnlyPool(ReadOnlyPoolConfig{
			Open: func(ctx context.Context, owner store.SessionDBOwner, path string) (store.EventReadCloser, error) {
				sessionID := owner.SessionID
				reader := &readOnlyPoolTestReader{}
				if sessionID == "sess-read-only-close" {
					reader.onClose = func(ctx context.Context) error {
						lease, err := pool.Open(ctx, testSessionDBOwner("sess-read-only-after-close"), path+".after")
						if !errors.Is(err, errReadOnlyPoolClosed) {
							return fmt.Errorf("Open(after close) error = %v, want errReadOnlyPoolClosed", err)
						}
						if lease != nil {
							return errors.New("Open(after close) lease = non-nil, want nil")
						}
						return nil
					}
				}
				return reader, nil
			},
		})

		lease, err := pool.Open(ctx, testSessionDBOwner("sess-read-only-close"), "close.db")
		if err != nil {
			t.Fatalf("Open(close) error = %v", err)
		}
		if err := lease.Close(ctx); err != nil {
			t.Fatalf("Close(close lease) error = %v", err)
		}

		result := make(chan error, 1)
		go func() {
			result <- pool.Close(ctx)
		}()

		select {
		case err := <-result:
			if err != nil {
				t.Fatalf("Close(pool) error = %v", err)
			}
		case <-ctx.Done():
			t.Fatalf("Close(pool) did not release the mutex for a re-entrant open: %v", ctx.Err())
		}
	})

	t.Run("Should complete shutdown when recorder close re-enters pool close", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		t.Cleanup(cancel)

		closeErr := errors.New("re-entrant close sentinel")
		var pool *ReadOnlyPool
		pool = NewReadOnlyPool(ReadOnlyPoolConfig{
			Open: func(context.Context, store.SessionDBOwner, string) (store.EventReadCloser, error) {
				return &readOnlyPoolTestReader{onClose: func(ctx context.Context) error {
					if err := pool.Close(ctx); err != nil {
						return err
					}
					return closeErr
				}}, nil
			},
		})

		lease, err := pool.Open(ctx, testSessionDBOwner("sess-read-only-close-self"), "close-self.db")
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		if err := lease.Close(ctx); err != nil {
			t.Fatalf("Close(lease) error = %v", err)
		}

		if err := pool.Close(ctx); !errors.Is(err, closeErr) {
			t.Fatalf("Close(pool) error = %v, want re-entrant close sentinel", err)
		}
	})

	t.Run("Should join another recorder close from re-entrant shutdown", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		t.Cleanup(cancel)

		now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
		blockedCloseStarted := make(chan struct{})
		reentrantCloseStarted := make(chan struct{})
		releaseBlockedClose := make(chan struct{})
		closeErr := errors.New("joined close sentinel")
		var pool *ReadOnlyPool
		pool = NewReadOnlyPool(ReadOnlyPoolConfig{
			TTL: time.Second,
			Now: func() time.Time {
				return now
			},
			Open: func(_ context.Context, owner store.SessionDBOwner, _ string) (store.EventReadCloser, error) {
				sessionID := owner.SessionID
				reader := &readOnlyPoolTestReader{}
				switch sessionID {
				case "sess-read-only-blocked-close":
					reader.onClose = func(ctx context.Context) error {
						close(blockedCloseStarted)
						select {
						case <-releaseBlockedClose:
							return closeErr
						case <-ctx.Done():
							return ctx.Err()
						}
					}
				case "sess-read-only-reentrant-close":
					reader.onClose = func(ctx context.Context) error {
						close(reentrantCloseStarted)
						return pool.Close(ctx)
					}
				}
				return reader, nil
			},
		})

		blockedLease, err := pool.Open(ctx, testSessionDBOwner("sess-read-only-blocked-close"), "blocked-close.db")
		if err != nil {
			t.Fatalf("Open(blocked close) error = %v", err)
		}
		if err := blockedLease.Close(ctx); err != nil {
			t.Fatalf("Close(blocked lease) error = %v", err)
		}
		reentrantLease, err := pool.Open(
			ctx, testSessionDBOwner(

				"sess-read-only-reentrant-close"),

			"reentrant-close.db")

		if err != nil {
			t.Fatalf("Open(re-entrant close) error = %v", err)
		}
		now = now.Add(2 * time.Second)

		expiryResult := make(chan error, 1)
		go func() {
			expiryResult <- pool.CloseExpired(ctx)
		}()
		select {
		case <-blockedCloseStarted:
		case <-ctx.Done():
			t.Fatalf("CloseExpired() did not start recorder close: %v", ctx.Err())
		}
		if err := reentrantLease.Close(ctx); err != nil {
			t.Fatalf("Close(re-entrant lease) error = %v", err)
		}

		shutdownResult := make(chan error, 1)
		go func() {
			shutdownResult <- pool.Close(ctx)
		}()
		select {
		case <-reentrantCloseStarted:
		case <-ctx.Done():
			t.Fatalf("Close(pool) did not start re-entrant recorder close: %v", ctx.Err())
		}
		select {
		case err := <-shutdownResult:
			t.Fatalf("Close(pool) returned before the other recorder close: %v", err)
		default:
		}

		close(releaseBlockedClose)
		select {
		case err := <-expiryResult:
			if !errors.Is(err, closeErr) {
				t.Fatalf("CloseExpired() error = %v, want joined close sentinel", err)
			}
		case <-ctx.Done():
			t.Fatalf("CloseExpired() did not complete: %v", ctx.Err())
		}
		select {
		case err := <-shutdownResult:
			if !errors.Is(err, closeErr) {
				t.Fatalf("Close(pool) error = %v, want joined close sentinel", err)
			}
		case <-ctx.Done():
			t.Fatalf("Close(pool) did not join the other recorder close: %v", ctx.Err())
		}
	})

	t.Run("Should execute a reserved recorder close with its owner context", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		ownerCtx := context.WithValue(ctx, readOnlyPoolTestContextKey{}, "owner")
		waiterCtx := context.WithValue(ctx, readOnlyPoolTestContextKey{}, "waiter")
		observedContext := make(chan string, 1)
		pool := NewReadOnlyPool(ReadOnlyPoolConfig{})
		key := readOnlyPoolKey{
			owner: testSessionDBOwner("sess-read-only-close-context"),
			path:  "close-context.db",
		}
		reader := &readOnlyPoolTestReader{onClose: func(ctx context.Context) error {
			value, ok := ctx.Value(readOnlyPoolTestContextKey{}).(string)
			if !ok {
				return errors.New("close context owner is missing")
			}
			observedContext <- value
			return nil
		}}

		pool.mu.Lock()
		reserved := pool.startCloseLocked(ownerCtx, key, reader, "close context-owned recorder")
		pool.mu.Unlock()
		executing := &readOnlyPoolClose{ready: make(chan struct{}), started: true}
		reentrantWaiterCtx := context.WithValue(
			waiterCtx,
			readOnlyPoolCloseContextKey{},
			readOnlyPoolCloseExecution{pool: pool, operation: executing},
		)
		if err := pool.waitForCloseOperations(reentrantWaiterCtx, []*readOnlyPoolClose{reserved}); err != nil {
			t.Fatalf("waitForCloseOperations() error = %v", err)
		}
		select {
		case got := <-observedContext:
			if want := "owner"; got != want {
				t.Fatalf("recorder close context owner = %q, want %q", got, want)
			}
		case <-ctx.Done():
			t.Fatalf("recorder close did not publish its context owner: %v", ctx.Err())
		}
		if err := pool.Close(ctx); err != nil {
			t.Fatalf("Close(pool) error = %v", err)
		}
	})

	t.Run("Should quiesce the same session from its expired recorder close", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		t.Cleanup(cancel)

		now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
		const (
			sessionID = "sess-read-only-quiesce-self"
			path      = "quiesce-self.db"
		)
		var pool *ReadOnlyPool
		pool = NewReadOnlyPool(ReadOnlyPoolConfig{
			TTL: time.Second,
			Now: func() time.Time {
				return now
			},
			Open: func(context.Context, store.SessionDBOwner, string) (store.EventReadCloser, error) {
				return &readOnlyPoolTestReader{onClose: func(ctx context.Context) error {
					release, err := pool.Quiesce(ctx, testSessionDBOwner(sessionID), path)
					if err != nil {
						return fmt.Errorf("quiesce session from recorder close: %w", err)
					}
					release()
					return nil
				}}, nil
			},
		})

		lease, err := pool.Open(ctx, testSessionDBOwner(sessionID), path)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		if err := lease.Close(ctx); err != nil {
			t.Fatalf("Close(lease) error = %v", err)
		}
		now = now.Add(2 * time.Second)

		if err := pool.CloseExpired(ctx); err != nil {
			t.Fatalf("CloseExpired() error = %v", err)
		}
		if err := pool.Close(ctx); err != nil {
			t.Fatalf("Close(pool) error = %v", err)
		}
	})

	t.Run("Should complete quiescence when its recorder close re-enters the same session", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		t.Cleanup(cancel)

		const (
			sessionID = "sess-read-only-owned-quiescence"
			path      = "owned-quiescence.db"
		)
		var pool *ReadOnlyPool
		pool = NewReadOnlyPool(ReadOnlyPoolConfig{
			Open: func(context.Context, store.SessionDBOwner, string) (store.EventReadCloser, error) {
				return &readOnlyPoolTestReader{onClose: func(ctx context.Context) error {
					release, err := pool.Quiesce(ctx, testSessionDBOwner(sessionID), path)
					if err != nil {
						return fmt.Errorf("re-enter owned quiescence: %w", err)
					}
					release()
					return nil
				}}, nil
			},
		})

		lease, err := pool.Open(ctx, testSessionDBOwner(sessionID), path)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		if err := lease.Close(ctx); err != nil {
			t.Fatalf("Close(lease) error = %v", err)
		}

		release, err := pool.Quiesce(ctx, testSessionDBOwner(sessionID), path)
		if err != nil {
			t.Fatalf("Quiesce() error = %v", err)
		}
		release()
		if err := pool.Close(ctx); err != nil {
			t.Fatalf("Close(pool) error = %v", err)
		}
	})

	t.Run("Should keep quiescence blocked by an active close after a no-op re-entry", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
			const (
				activeSessionID  = "sess-read-only-active-close"
				activePath       = "active-close.db"
				waitingSessionID = "sess-read-only-waiting-close"
				waitingPath      = "waiting-close.db"
			)
			noOpReentryReturned := make(chan struct{})
			allowActiveClose := make(chan struct{})
			waitingQuiescenceStarted := make(chan struct{})
			waitingQuiescenceReturned := make(chan struct{})
			var pool *ReadOnlyPool
			pool = NewReadOnlyPool(ReadOnlyPoolConfig{
				TTL: time.Second,
				Now: func() time.Time {
					return now
				},
				Open: func(_ context.Context, owner store.SessionDBOwner, _ string) (store.EventReadCloser, error) {
					sessionID := owner.SessionID
					switch sessionID {
					case activeSessionID:
						return &readOnlyPoolTestReader{onClose: func(ctx context.Context) error {
							if err := pool.CloseExpired(ctx); err != nil {
								return fmt.Errorf("no-op close-expired re-entry: %w", err)
							}
							close(noOpReentryReturned)
							<-allowActiveClose
							return nil
						}}, nil
					case waitingSessionID:
						return &readOnlyPoolTestReader{onClose: func(ctx context.Context) error {
							close(waitingQuiescenceStarted)
							release, err := pool.Quiesce(ctx, testSessionDBOwner(activeSessionID), activePath)
							if err != nil {
								return fmt.Errorf("quiesce active recorder close: %w", err)
							}
							release()
							close(waitingQuiescenceReturned)
							return nil
						}}, nil
					default:
						return nil, fmt.Errorf("unexpected session id %q", sessionID)
					}
				},
			})

			activeLease, err := pool.Open(ctx, testSessionDBOwner(activeSessionID), activePath)
			if err != nil {
				t.Fatalf("Open(active) error = %v", err)
			}
			waitingLease, err := pool.Open(ctx, testSessionDBOwner(waitingSessionID), waitingPath)
			if err != nil {
				t.Fatalf("Open(waiting) error = %v", err)
			}
			if err := activeLease.Close(ctx); err != nil {
				t.Fatalf("Close(active lease) error = %v", err)
			}
			now = now.Add(2 * time.Second)

			expiryResult := make(chan error, 1)
			go func() {
				expiryResult <- pool.CloseExpired(ctx)
			}()
			<-noOpReentryReturned

			if err := waitingLease.Close(ctx); err != nil {
				t.Fatalf("Close(waiting lease) error = %v", err)
			}
			type quiescenceResult struct {
				release func()
				err     error
			}
			waitingResult := make(chan quiescenceResult, 1)
			go func() {
				release, err := pool.Quiesce(ctx, testSessionDBOwner(waitingSessionID), waitingPath)
				waitingResult <- quiescenceResult{release: release, err: err}
			}()
			<-waitingQuiescenceStarted
			synctest.Wait()
			returnedBeforeActiveClose := false
			select {
			case <-waitingQuiescenceReturned:
				returnedBeforeActiveClose = true
			default:
			}

			close(allowActiveClose)
			if err := <-expiryResult; err != nil {
				t.Fatalf("CloseExpired() error = %v", err)
			}
			result := <-waitingResult
			if result.err != nil {
				t.Fatalf("Quiesce(waiting) error = %v", result.err)
			}
			result.release()
			if returnedBeforeActiveClose {
				t.Fatal("same-session quiescence succeeded before the active recorder close completed")
			}
			if err := pool.Close(ctx); err != nil {
				t.Fatalf("Close(pool) error = %v", err)
			}
		})
	})

	t.Run("Should fail closed when re-entrant quiescences form a real close cycle", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
			const (
				firstSessionID  = "sess-read-only-cycle-first"
				firstPath       = "cycle-first.db"
				secondSessionID = "sess-read-only-cycle-second"
				secondPath      = "cycle-second.db"
			)
			firstCloseStarted := make(chan struct{})
			allowFirstQuiescence := make(chan struct{})
			secondQuiescenceStarted := make(chan struct{})
			firstQuiescenceSucceeded := make(chan struct{})
			secondQuiescenceSucceeded := make(chan struct{})
			var pool *ReadOnlyPool
			pool = NewReadOnlyPool(ReadOnlyPoolConfig{
				TTL: time.Second,
				Now: func() time.Time {
					return now
				},
				Open: func(_ context.Context, owner store.SessionDBOwner, _ string) (store.EventReadCloser, error) {
					sessionID := owner.SessionID
					switch sessionID {
					case firstSessionID:
						return &readOnlyPoolTestReader{onClose: func(ctx context.Context) error {
							close(firstCloseStarted)
							<-allowFirstQuiescence
							release, err := pool.Quiesce(ctx, testSessionDBOwner(secondSessionID), secondPath)
							if err != nil {
								return fmt.Errorf("quiesce second close: %w", err)
							}
							release()
							close(firstQuiescenceSucceeded)
							return nil
						}}, nil
					case secondSessionID:
						return &readOnlyPoolTestReader{onClose: func(ctx context.Context) error {
							close(secondQuiescenceStarted)
							release, err := pool.Quiesce(ctx, testSessionDBOwner(firstSessionID), firstPath)
							if err != nil {
								return fmt.Errorf("quiesce first close: %w", err)
							}
							release()
							close(secondQuiescenceSucceeded)
							return nil
						}}, nil
					default:
						return nil, fmt.Errorf("unexpected session id %q", sessionID)
					}
				},
			})

			firstLease, err := pool.Open(ctx, testSessionDBOwner(firstSessionID), firstPath)
			if err != nil {
				t.Fatalf("Open(first) error = %v", err)
			}
			secondLease, err := pool.Open(ctx, testSessionDBOwner(secondSessionID), secondPath)
			if err != nil {
				t.Fatalf("Open(second) error = %v", err)
			}
			if err := firstLease.Close(ctx); err != nil {
				t.Fatalf("Close(first lease) error = %v", err)
			}
			now = now.Add(2 * time.Second)

			firstResult := make(chan error, 1)
			go func() {
				firstResult <- pool.CloseExpired(ctx)
			}()
			<-firstCloseStarted
			if err := secondLease.Close(ctx); err != nil {
				t.Fatalf("Close(second lease) error = %v", err)
			}
			now = now.Add(2 * time.Second)

			secondResult := make(chan error, 1)
			go func() {
				secondResult <- pool.CloseExpired(ctx)
			}()
			<-secondQuiescenceStarted
			synctest.Wait()
			close(allowFirstQuiescence)
			synctest.Wait()

			var firstErr, secondErr error
			firstCompleted := false
			secondCompleted := false
			select {
			case firstErr = <-firstResult:
				firstCompleted = true
			default:
			}
			select {
			case secondErr = <-secondResult:
				secondCompleted = true
			default:
			}
			if !firstCompleted || !secondCompleted {
				cancel()
				synctest.Wait()
				if !firstCompleted {
					firstErr = <-firstResult
				}
				if !secondCompleted {
					secondErr = <-secondResult
				}
				t.Error("re-entrant quiescence cycle required cancellation to complete")
			}
			if !errors.Is(firstErr, errReadOnlyPoolCloseDependencyCycle) {
				t.Errorf("CloseExpired(first) error = %v, want dependency-cycle sentinel", firstErr)
			}
			if !errors.Is(secondErr, errReadOnlyPoolCloseDependencyCycle) {
				t.Errorf("CloseExpired(second) error = %v, want dependency-cycle sentinel", secondErr)
			}
			select {
			case <-firstQuiescenceSucceeded:
				t.Error("first quiescence succeeded while the second recorder close remained active")
			default:
			}
			select {
			case <-secondQuiescenceSucceeded:
				t.Error("second quiescence succeeded while the first recorder close remained active")
			default:
			}
			if err := pool.Close(t.Context()); err != nil {
				t.Fatalf("Close(pool) error = %v", err)
			}
		})
	})

	t.Run("Should fail a cycle when a close joins an existing quiescence", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			const (
				firstSessionID  = "sess-read-only-joined-cycle-first"
				firstPath       = "joined-cycle-first.db"
				secondSessionID = "sess-read-only-joined-cycle-second"
				secondPath      = "joined-cycle-second.db"
			)
			type quiescenceResult struct {
				release func()
				err     error
			}
			firstCloseStarted := make(chan struct{})
			allowFirstJoin := make(chan struct{})
			secondCloseStarted := make(chan struct{})
			firstJoinResult := make(chan quiescenceResult, 1)
			secondJoinResult := make(chan quiescenceResult, 1)
			firstJoinSucceeded := make(chan struct{})
			secondJoinSucceeded := make(chan struct{})
			var pool *ReadOnlyPool
			pool = NewReadOnlyPool(ReadOnlyPoolConfig{
				Open: func(_ context.Context, owner store.SessionDBOwner, _ string) (store.EventReadCloser, error) {
					sessionID := owner.SessionID
					switch sessionID {
					case firstSessionID:
						return &readOnlyPoolTestReader{onClose: func(ctx context.Context) error {
							close(firstCloseStarted)
							<-allowFirstJoin
							release, err := pool.Quiesce(ctx, testSessionDBOwner(secondSessionID), secondPath)
							firstJoinResult <- quiescenceResult{release: release, err: err}
							if err != nil {
								return fmt.Errorf("first close join second quiescence: %w", err)
							}
							close(firstJoinSucceeded)
							release()
							return nil
						}}, nil
					case secondSessionID:
						return &readOnlyPoolTestReader{onClose: func(ctx context.Context) error {
							close(secondCloseStarted)
							release, err := pool.Quiesce(ctx, testSessionDBOwner(firstSessionID), firstPath)
							secondJoinResult <- quiescenceResult{release: release, err: err}
							if err != nil {
								return fmt.Errorf("second close join first quiescence: %w", err)
							}
							close(secondJoinSucceeded)
							release()
							return nil
						}}, nil
					default:
						return nil, fmt.Errorf("unexpected session id %q", sessionID)
					}
				},
			})

			firstLease, err := pool.Open(ctx, testSessionDBOwner(firstSessionID), firstPath)
			if err != nil {
				t.Fatalf("Open(first) error = %v", err)
			}
			secondLease, err := pool.Open(ctx, testSessionDBOwner(secondSessionID), secondPath)
			if err != nil {
				t.Fatalf("Open(second) error = %v", err)
			}
			if err := firstLease.Close(ctx); err != nil {
				t.Fatalf("Close(first lease) error = %v", err)
			}
			if err := secondLease.Close(ctx); err != nil {
				t.Fatalf("Close(second lease) error = %v", err)
			}

			firstExternalResult := make(chan quiescenceResult, 1)
			go func() {
				release, err := pool.Quiesce(ctx, testSessionDBOwner(firstSessionID), firstPath)
				firstExternalResult <- quiescenceResult{release: release, err: err}
			}()
			<-firstCloseStarted

			secondExternalResult := make(chan quiescenceResult, 1)
			go func() {
				release, err := pool.Quiesce(ctx, testSessionDBOwner(secondSessionID), secondPath)
				secondExternalResult <- quiescenceResult{release: release, err: err}
			}()
			<-secondCloseStarted
			synctest.Wait()
			close(allowFirstJoin)
			synctest.Wait()

			var firstExternal, secondExternal quiescenceResult
			firstCompleted := false
			secondCompleted := false
			select {
			case firstExternal = <-firstExternalResult:
				firstCompleted = true
			default:
			}
			select {
			case secondExternal = <-secondExternalResult:
				secondCompleted = true
			default:
			}
			if !firstCompleted || !secondCompleted {
				cancel()
				synctest.Wait()
				if !firstCompleted {
					firstExternal = <-firstExternalResult
				}
				if !secondCompleted {
					secondExternal = <-secondExternalResult
				}
				t.Error("joined quiescence cycle required cancellation to complete")
			}

			for name, result := range map[string]quiescenceResult{
				"first external":  firstExternal,
				"second external": secondExternal,
				"first joined":    <-firstJoinResult,
				"second joined":   <-secondJoinResult,
			} {
				if !errors.Is(result.err, errReadOnlyPoolCloseDependencyCycle) {
					t.Errorf("%s Quiesce() error = %v, want dependency-cycle sentinel", name, result.err)
				}
				if result.release != nil {
					t.Errorf("%s Quiesce() release is non-nil after dependency cycle", name)
					result.release()
				}
			}
			select {
			case <-firstJoinSucceeded:
				t.Error("first joined quiescence succeeded during the dependency cycle")
			default:
			}
			select {
			case <-secondJoinSucceeded:
				t.Error("second joined quiescence succeeded during the dependency cycle")
			default:
			}
			if err := pool.Close(t.Context()); err != nil {
				t.Fatalf("Close(pool) error = %v", err)
			}
		})
	})

	t.Run("Should wait for peer close when joining its own quiescence", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
			const (
				sessionID = "sess-read-only-self-joined-peer"
				path      = "self-joined-peer.db"
			)
			type quiescenceResult struct {
				release func()
				err     error
			}
			peerCloseStarted := make(chan struct{})
			allowPeerClose := make(chan struct{})
			selfJoinStarted := make(chan struct{})
			allowSelfCloseReturn := make(chan struct{})
			selfJoinResult := make(chan quiescenceResult, 1)
			var (
				openCount atomic.Int64
				pool      *ReadOnlyPool
			)
			pool = NewReadOnlyPool(ReadOnlyPoolConfig{
				TTL: time.Second,
				Now: func() time.Time {
					return now
				},
				Open: func(context.Context, store.SessionDBOwner, string) (store.EventReadCloser, error) {
					if openCount.Add(1) == 1 {
						return &readOnlyPoolTestReader{onClose: func(context.Context) error {
							close(peerCloseStarted)
							<-allowPeerClose
							return nil
						}}, nil
					}
					return &readOnlyPoolTestReader{onClose: func(ctx context.Context) error {
						close(selfJoinStarted)
						release, err := pool.Quiesce(ctx, testSessionDBOwner(sessionID), path)
						selfJoinResult <- quiescenceResult{release: release, err: err}
						<-allowSelfCloseReturn
						if err != nil {
							return fmt.Errorf("join own quiescence with peer: %w", err)
						}
						return nil
					}}, nil
				},
			})

			firstLease, err := pool.Open(ctx, testSessionDBOwner(sessionID), path)
			if err != nil {
				t.Fatalf("Open(first) error = %v", err)
			}
			if err := firstLease.Close(ctx); err != nil {
				t.Fatalf("Close(first lease) error = %v", err)
			}
			now = now.Add(2 * time.Second)
			peerCloseResult := make(chan error, 1)
			go func() {
				peerCloseResult <- pool.CloseExpired(ctx)
			}()
			<-peerCloseStarted

			secondLease, err := pool.Open(ctx, testSessionDBOwner(sessionID), path)
			if err != nil {
				t.Fatalf("Open(second) error = %v", err)
			}
			if err := secondLease.Close(ctx); err != nil {
				t.Fatalf("Close(second lease) error = %v", err)
			}
			externalResult := make(chan quiescenceResult, 1)
			go func() {
				release, err := pool.Quiesce(ctx, testSessionDBOwner(sessionID), path)
				externalResult <- quiescenceResult{release: release, err: err}
			}()
			<-selfJoinStarted
			synctest.Wait()

			var joined quiescenceResult
			returnedBeforePeer := false
			select {
			case joined = <-selfJoinResult:
				returnedBeforePeer = true
			default:
			}
			close(allowPeerClose)
			if err := <-peerCloseResult; err != nil {
				t.Fatalf("CloseExpired(peer) error = %v", err)
			}
			if !returnedBeforePeer {
				joined = <-selfJoinResult
			}
			if joined.err != nil {
				t.Errorf("joined Quiesce() error = %v", joined.err)
			}
			if joined.release == nil {
				t.Error("joined Quiesce() release is nil after peer publication")
			} else {
				joined.release()
			}
			close(allowSelfCloseReturn)

			external := <-externalResult
			if external.err != nil {
				t.Fatalf("external Quiesce() error = %v", external.err)
			}
			if external.release == nil {
				t.Fatal("external Quiesce() release is nil")
			}
			external.release()
			if returnedBeforePeer {
				t.Error("self-joined Quiesce() returned before the peer recorder close published")
			}
			if got, want := openCount.Load(), int64(2); got != want {
				t.Errorf("opener call count = %d, want %d", got, want)
			}
			if err := pool.Close(ctx); err != nil {
				t.Fatalf("Close(pool) error = %v", err)
			}
		})
	})

	t.Run("Should carry a self-joined quiescence across an active lease and successor close", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
			const (
				sessionID = "sess-read-only-self-joined-active-lease"
				path      = "self-joined-active-lease.db"
			)
			type quiescenceResult struct {
				release func()
				err     error
			}
			oldCloseStarted := make(chan struct{})
			allowOldJoin := make(chan struct{})
			newCloseStarted := make(chan struct{})
			allowNewClose := make(chan struct{})
			selfJoinResult := make(chan quiescenceResult, 1)
			var (
				openCount     atomic.Int64
				newCloseCount atomic.Int64
				pool          *ReadOnlyPool
			)
			pool = NewReadOnlyPool(ReadOnlyPoolConfig{
				TTL: time.Second,
				Now: func() time.Time {
					return now
				},
				Open: func(context.Context, store.SessionDBOwner, string) (store.EventReadCloser, error) {
					if openCount.Add(1) == 1 {
						return &readOnlyPoolTestReader{onClose: func(ctx context.Context) error {
							close(oldCloseStarted)
							<-allowOldJoin
							release, err := pool.Quiesce(ctx, testSessionDBOwner(sessionID), path)
							selfJoinResult <- quiescenceResult{release: release, err: err}
							if err != nil {
								return fmt.Errorf("join quiescence across active lease: %w", err)
							}
							return nil
						}}, nil
					}
					return &readOnlyPoolTestReader{onClose: func(context.Context) error {
						newCloseCount.Add(1)
						close(newCloseStarted)
						<-allowNewClose
						return nil
					}}, nil
				},
			})

			oldLease, err := pool.Open(ctx, testSessionDBOwner(sessionID), path)
			if err != nil {
				t.Fatalf("Open(old) error = %v", err)
			}
			if err := oldLease.Close(ctx); err != nil {
				t.Fatalf("Close(old lease) error = %v", err)
			}
			now = now.Add(2 * time.Second)

			oldCloseResult := make(chan error, 1)
			go func() {
				oldCloseResult <- pool.CloseExpired(ctx)
			}()
			<-oldCloseStarted

			activeLease, err := pool.Open(ctx, testSessionDBOwner(sessionID), path)
			if err != nil {
				t.Fatalf("Open(active) error = %v", err)
			}
			aliasLease, err := pool.Open(ctx, testSessionDBOwner(sessionID), path)
			if err != nil {
				t.Fatalf("Open(active alias) error = %v", err)
			}
			externalResult := make(chan quiescenceResult, 1)
			go func() {
				release, err := pool.Quiesce(ctx, testSessionDBOwner(sessionID), path)
				externalResult <- quiescenceResult{release: release, err: err}
			}()
			synctest.Wait()
			probe, probeErr := pool.Open(ctx, testSessionDBOwner(sessionID), path)
			if !errors.Is(probeErr, ErrReadOnlyPoolQuiescing) {
				t.Fatalf("Open(quiescence probe) error = %v, want ErrReadOnlyPoolQuiescing", probeErr)
			}
			if probe != nil {
				t.Fatalf("Open(quiescence probe) lease = %v, want nil", probe)
			}
			waitingCtx, cancelWaiting := context.WithCancel(ctx)
			canceledResult := make(chan quiescenceResult, 1)
			go func() {
				release, err := pool.Quiesce(waitingCtx, testSessionDBOwner(sessionID), path)
				canceledResult <- quiescenceResult{release: release, err: err}
			}()
			synctest.Wait()
			cancelWaiting()
			canceled := <-canceledResult
			if !errors.Is(canceled.err, context.Canceled) {
				t.Errorf("canceled joined Quiesce() error = %v, want context.Canceled", canceled.err)
			}
			if canceled.release != nil {
				t.Error("canceled joined Quiesce() release is non-nil")
				canceled.release()
			}

			close(allowOldJoin)
			synctest.Wait()
			var joined quiescenceResult
			joinedReturned := false
			select {
			case joined = <-selfJoinResult:
				joinedReturned = true
			default:
			}
			returnedBeforeLeaseDrain := joinedReturned
			if err := pool.CloseExpired(ctx); err != nil {
				t.Fatalf("CloseExpired(active responsibility) error = %v", err)
			}

			if err := activeLease.Close(ctx); err != nil {
				t.Fatalf("Close(active lease) error = %v", err)
			}
			synctest.Wait()
			select {
			case <-newCloseStarted:
				t.Fatal("successor recorder close started while an alias lease remained active")
			default:
			}
			select {
			case joined = <-selfJoinResult:
				joinedReturned = true
			default:
			}
			if joinedReturned {
				t.Error("self-joined Quiesce() returned while an alias lease remained active")
			}
			if err := aliasLease.Close(ctx); err != nil {
				t.Fatalf("Close(active alias lease) error = %v", err)
			}
			<-newCloseStarted
			synctest.Wait()
			if !joinedReturned {
				select {
				case joined = <-selfJoinResult:
					joinedReturned = true
				default:
				}
			}
			returnedBeforeSuccessorTerminal := joinedReturned

			close(allowNewClose)
			if !joinedReturned {
				joined = <-selfJoinResult
			}
			if joined.err != nil {
				t.Errorf("self-joined Quiesce() error = %v", joined.err)
			}
			if joined.release == nil {
				t.Error("self-joined Quiesce() release is nil")
			} else {
				joined.release()
			}
			if err := <-oldCloseResult; err != nil {
				t.Errorf("CloseExpired(old) error = %v", err)
			}
			external := <-externalResult
			if external.err != nil {
				t.Errorf("external Quiesce() error = %v", external.err)
			}
			if external.release == nil {
				t.Error("external Quiesce() release is nil")
			} else {
				external.release()
			}
			if returnedBeforeLeaseDrain {
				t.Error("self-joined Quiesce() returned before the active lease drained")
			}
			if returnedBeforeSuccessorTerminal {
				t.Error("self-joined Quiesce() returned before the successor recorder close published")
			}
			if got, want := openCount.Load(), int64(2); got != want {
				t.Errorf("opener call count = %d, want %d", got, want)
			}
			if got, want := newCloseCount.Load(), int64(1); got != want {
				t.Errorf("successor recorder close count = %d, want %d", got, want)
			}
			if err := pool.Close(ctx); err != nil {
				t.Fatalf("Close(pool) error = %v", err)
			}
		})
	})

	t.Run("Should fail a cycle created by late close materialization without cancellation", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
			const (
				firstSessionID  = "sess-read-only-late-cycle-first"
				firstPath       = "late-cycle-first.db"
				secondSessionID = "sess-read-only-late-cycle-second"
				secondPath      = "late-cycle-second.db"
			)
			type quiescenceResult struct {
				release func()
				err     error
			}
			firstCloseStarted := make(chan struct{})
			allowFirstJoin := make(chan struct{})
			firstJoinStarted := make(chan struct{})
			secondCloseStarted := make(chan struct{})
			firstJoinResult := make(chan quiescenceResult, 1)
			secondJoinResult := make(chan quiescenceResult, 1)
			firstJoinSucceeded := make(chan struct{})
			secondJoinSucceeded := make(chan struct{})
			var pool *ReadOnlyPool
			pool = NewReadOnlyPool(ReadOnlyPoolConfig{
				TTL: time.Second,
				Now: func() time.Time {
					return now
				},
				Open: func(_ context.Context, owner store.SessionDBOwner, _ string) (store.EventReadCloser, error) {
					sessionID := owner.SessionID
					switch sessionID {
					case firstSessionID:
						return &readOnlyPoolTestReader{onClose: func(ctx context.Context) error {
							close(firstCloseStarted)
							<-allowFirstJoin
							close(firstJoinStarted)
							release, err := pool.Quiesce(ctx, testSessionDBOwner(secondSessionID), secondPath)
							firstJoinResult <- quiescenceResult{release: release, err: err}
							if err != nil {
								return fmt.Errorf("first close join second quiescence: %w", err)
							}
							close(firstJoinSucceeded)
							if release != nil {
								release()
							}
							return nil
						}}, nil
					case secondSessionID:
						return &readOnlyPoolTestReader{onClose: func(ctx context.Context) error {
							close(secondCloseStarted)
							release, err := pool.Quiesce(ctx, testSessionDBOwner(firstSessionID), firstPath)
							secondJoinResult <- quiescenceResult{release: release, err: err}
							if err != nil {
								return fmt.Errorf("second close join first close: %w", err)
							}
							close(secondJoinSucceeded)
							if release != nil {
								release()
							}
							return nil
						}}, nil
					default:
						return nil, fmt.Errorf("unexpected session id %q", sessionID)
					}
				},
			})

			firstLease, err := pool.Open(ctx, testSessionDBOwner(firstSessionID), firstPath)
			if err != nil {
				t.Fatalf("Open(first) error = %v", err)
			}
			if err := firstLease.Close(ctx); err != nil {
				t.Fatalf("Close(first lease) error = %v", err)
			}
			now = now.Add(2 * time.Second)
			firstCloseResult := make(chan error, 1)
			go func() {
				firstCloseResult <- pool.CloseExpired(ctx)
			}()
			<-firstCloseStarted

			activeLease, err := pool.Open(ctx, testSessionDBOwner(secondSessionID), secondPath)
			if err != nil {
				t.Fatalf("Open(second active) error = %v", err)
			}
			secondKey, err := normalizeReadOnlyPoolKey(testSessionDBOwner(secondSessionID), secondPath)
			if err != nil {
				t.Fatalf("normalizeReadOnlyPoolKey(second) error = %v", err)
			}
			// Hold the public Quiesce lifecycle between admission and its idle
			// continuation so CloseExpired deterministically materializes the
			// successor close before the quiescence owner resumes.
			secondStart, joined, err := pool.beginQuiescence(secondKey)
			if err != nil {
				t.Fatalf("beginQuiescence(second) error = %v", err)
			}
			if joined {
				t.Fatal("beginQuiescence(second) joined an unexpected state")
			}
			probe, probeErr := pool.Open(ctx, testSessionDBOwner(secondSessionID), secondPath)
			if !errors.Is(probeErr, ErrReadOnlyPoolQuiescing) {
				t.Fatalf("Open(quiescence probe) error = %v, want ErrReadOnlyPoolQuiescing", probeErr)
			}
			if probe != nil {
				t.Fatalf("Open(quiescence probe) lease = %v, want nil", probe)
			}

			close(allowFirstJoin)
			<-firstJoinStarted
			synctest.Wait()

			if err := activeLease.Close(ctx); err != nil {
				t.Fatalf("Close(second active lease) error = %v", err)
			}
			now = now.Add(2 * time.Second)
			secondCloseResult := make(chan error, 1)
			go func() {
				secondCloseResult <- pool.CloseExpired(ctx)
			}()
			<-secondCloseStarted
			synctest.Wait()

			externalResult := make(chan quiescenceResult, 1)
			go func() {
				release, err := pool.closeQuiescedEntry(ctx, secondKey, secondStart)
				externalResult <- quiescenceResult{release: release, err: err}
			}()
			synctest.Wait()

			var (
				firstJoined, secondJoined, external quiescenceResult
				firstCloseErr, secondCloseErr       error
				firstJoinedDone, secondJoinedDone   bool
				externalDone, firstCloseDone        bool
				secondCloseDone                     bool
			)
			select {
			case firstJoined = <-firstJoinResult:
				firstJoinedDone = true
			default:
			}
			select {
			case secondJoined = <-secondJoinResult:
				secondJoinedDone = true
			default:
			}
			select {
			case external = <-externalResult:
				externalDone = true
			default:
			}
			select {
			case firstCloseErr = <-firstCloseResult:
				firstCloseDone = true
			default:
			}
			select {
			case secondCloseErr = <-secondCloseResult:
				secondCloseDone = true
			default:
			}
			if !firstJoinedDone || !secondJoinedDone || !externalDone || !firstCloseDone || !secondCloseDone {
				cancel()
				synctest.Wait()
				if !firstJoinedDone {
					firstJoined = <-firstJoinResult
				}
				if !secondJoinedDone {
					secondJoined = <-secondJoinResult
				}
				if !externalDone {
					external = <-externalResult
				}
				if !firstCloseDone {
					firstCloseErr = <-firstCloseResult
				}
				if !secondCloseDone {
					secondCloseErr = <-secondCloseResult
				}
				t.Error("late-materialization dependency cycle required cancellation to complete")
			}

			for name, result := range map[string]quiescenceResult{
				"first joined":  firstJoined,
				"second joined": secondJoined,
				"external":      external,
			} {
				if !errors.Is(result.err, errReadOnlyPoolCloseDependencyCycle) {
					t.Errorf("%s Quiesce() error = %v, want dependency-cycle sentinel", name, result.err)
				}
				if result.release != nil {
					t.Errorf("%s Quiesce() release is non-nil after dependency cycle", name)
					result.release()
				}
			}
			if !errors.Is(firstCloseErr, errReadOnlyPoolCloseDependencyCycle) {
				t.Errorf("CloseExpired(first) error = %v, want dependency-cycle sentinel", firstCloseErr)
			}
			if !errors.Is(secondCloseErr, errReadOnlyPoolCloseDependencyCycle) {
				t.Errorf("CloseExpired(second) error = %v, want dependency-cycle sentinel", secondCloseErr)
			}
			select {
			case <-firstJoinSucceeded:
				t.Error("first joined quiescence succeeded during the late dependency cycle")
			default:
			}
			select {
			case <-secondJoinSucceeded:
				t.Error("second joined quiescence succeeded during the late dependency cycle")
			default:
			}
			if err := pool.Close(t.Context()); err != nil {
				t.Fatalf("Close(pool) error = %v", err)
			}
		})
	})

	t.Run("Should retain a terminal close error across idle-to-close responsibility", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
			closeErr := errors.New("idle-to-close terminal sentinel")
			const (
				sessionID = "sess-read-only-idle-close-error"
				path      = "idle-close-error.db"
			)
			pool := NewReadOnlyPool(ReadOnlyPoolConfig{
				TTL: time.Second,
				Now: func() time.Time {
					return now
				},
				Open: func(context.Context, store.SessionDBOwner, string) (store.EventReadCloser, error) {
					return &readOnlyPoolTestReader{onClose: func(context.Context) error {
						return closeErr
					}}, nil
				},
			})

			activeLease, err := pool.Open(ctx, testSessionDBOwner(sessionID), path)
			if err != nil {
				t.Fatalf("Open(active) error = %v", err)
			}
			key, err := normalizeReadOnlyPoolKey(testSessionDBOwner(sessionID), path)
			if err != nil {
				t.Fatalf("normalizeReadOnlyPoolKey() error = %v", err)
			}
			// Pause Quiesce at the idle handoff to deterministically prove that a
			// close completed by CloseExpired remains part of its terminal result.
			start, joined, err := pool.beginQuiescence(key)
			if err != nil {
				t.Fatalf("beginQuiescence() error = %v", err)
			}
			if joined {
				t.Fatal("beginQuiescence() joined an unexpected state")
			}
			probe, probeErr := pool.Open(ctx, testSessionDBOwner(sessionID), path)
			if !errors.Is(probeErr, ErrReadOnlyPoolQuiescing) {
				t.Fatalf("Open(quiescence probe) error = %v, want ErrReadOnlyPoolQuiescing", probeErr)
			}
			if probe != nil {
				t.Fatalf("Open(quiescence probe) lease = %v, want nil", probe)
			}

			if err := activeLease.Close(ctx); err != nil {
				t.Fatalf("Close(active lease) error = %v", err)
			}
			now = now.Add(2 * time.Second)
			expiredErr := pool.CloseExpired(ctx)
			release, quiescenceErr := pool.closeQuiescedEntry(ctx, key, start)
			if !errors.Is(expiredErr, closeErr) {
				t.Errorf("CloseExpired() error = %v, want idle-to-close terminal sentinel", expiredErr)
			}
			if !errors.Is(quiescenceErr, closeErr) {
				t.Errorf("Quiesce() error = %v, want idle-to-close terminal sentinel", quiescenceErr)
			}
			if release != nil {
				t.Error("Quiesce() release is non-nil after idle-to-close terminal failure")
				release()
			}
			if err := pool.Close(t.Context()); err != nil {
				t.Fatalf("Close(pool) error = %v", err)
			}
		})
	})

	t.Run("Should quiesce the same session from a rejected opener recorder close", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			const (
				sessionID = "sess-read-only-rejected-opening-quiesce"
				path      = "rejected-opening-quiesce.db"
			)
			openErr := errors.New("opener sentinel")
			closeErr := errors.New("rejected opener close sentinel")
			reentrantQuiescenceReturned := make(chan struct{})
			allowRecorderClose := make(chan struct{})
			var (
				closeCount atomic.Int64
				openCount  atomic.Int64
				pool       *ReadOnlyPool
			)
			pool = NewReadOnlyPool(ReadOnlyPoolConfig{
				Open: func(context.Context, store.SessionDBOwner, string) (store.EventReadCloser, error) {
					if openCount.Add(1) > 1 {
						return &readOnlyPoolTestReader{}, nil
					}
					return &readOnlyPoolTestReader{onClose: func(ctx context.Context) error {
						closeCount.Add(1)
						release, err := pool.Quiesce(ctx, testSessionDBOwner(sessionID), path)
						if err != nil {
							return fmt.Errorf("quiesce rejected opener: %w", err)
						}
						release()
						close(reentrantQuiescenceReturned)
						<-allowRecorderClose
						return closeErr
					}}, openErr
				},
			})

			type openResult struct {
				lease store.EventReadCloser
				err   error
			}
			openResultCh := make(chan openResult, 1)
			go func() {
				lease, err := pool.Open(ctx, testSessionDBOwner(sessionID), path)
				openResultCh <- openResult{lease: lease, err: err}
			}()
			<-reentrantQuiescenceReturned

			admissionResultCh := make(chan openResult, 1)
			go func() {
				lease, err := pool.Open(ctx, testSessionDBOwner(sessionID), path)
				admissionResultCh <- openResult{lease: lease, err: err}
			}()
			synctest.Wait()
			var admissionResult openResult
			admissionReturnedWhileActive := false
			select {
			case admissionResult = <-admissionResultCh:
				admissionReturnedWhileActive = true
			default:
			}

			type quiescenceResult struct {
				release func()
				err     error
			}
			quiescenceResultCh := make(chan quiescenceResult, 1)
			go func() {
				release, err := pool.Quiesce(ctx, testSessionDBOwner(sessionID), path)
				quiescenceResultCh <- quiescenceResult{release: release, err: err}
			}()
			synctest.Wait()
			select {
			case result := <-quiescenceResultCh:
				t.Fatalf("joined Quiesce() completed before recorder close publication: %v", result.err)
			default:
			}

			close(allowRecorderClose)
			result := <-quiescenceResultCh
			if !errors.Is(result.err, closeErr) {
				t.Fatalf("joined Quiesce() error = %v, want rejected opener close sentinel", result.err)
			}
			if result.release != nil {
				t.Fatal("joined Quiesce() release is non-nil after close failure")
			}
			if !admissionReturnedWhileActive {
				admissionResult = <-admissionResultCh
				t.Error("Open() remained pending instead of rejecting admission during quiescence")
			}
			if !errors.Is(admissionResult.err, ErrReadOnlyPoolQuiescing) {
				t.Errorf("Open(during close) error = %v, want ErrReadOnlyPoolQuiescing", admissionResult.err)
			}
			if admissionResult.lease != nil {
				t.Errorf("Open(during close) lease = %v, want nil", admissionResult.lease)
				if err := admissionResult.lease.Close(ctx); err != nil {
					t.Errorf("Close(unexpected admission lease) error = %v", err)
				}
			}
			if got, want := openCount.Load(), int64(1); got != want {
				t.Errorf("opener call count before quiescence terminal state = %d, want %d", got, want)
			}
			opened := <-openResultCh
			if !errors.Is(opened.err, openErr) {
				t.Fatalf("Open() error = %v, want opener sentinel", opened.err)
			}
			if !errors.Is(opened.err, closeErr) {
				t.Fatalf("Open() error = %v, want rejected opener close sentinel", opened.err)
			}
			if opened.lease != nil {
				t.Fatalf("Open() lease = %v, want nil", opened.lease)
			}
			if got, want := closeCount.Load(), int64(1); got != want {
				t.Fatalf("recorder close count = %d, want %d", got, want)
			}
			resumedLease, err := pool.Open(ctx, testSessionDBOwner(sessionID), path)
			if err != nil {
				t.Fatalf("Open(after close terminal state) error = %v", err)
			}
			if resumedLease == nil {
				t.Fatal("Open(after close terminal state) lease is nil")
			}
			if err := resumedLease.Close(ctx); err != nil {
				t.Fatalf("Close(resumed lease) error = %v", err)
			}
			if got, want := openCount.Load(), int64(2); got != want {
				t.Fatalf("opener call count after quiescence terminal state = %d, want %d", got, want)
			}
			if err := pool.Close(ctx); err != nil {
				t.Fatalf("Close(pool) error = %v", err)
			}
		})
	})

	t.Run("Should serialize configured clock callbacks", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			fixedNow := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
			var (
				active     int
				concurrent atomic.Bool
				enabled    atomic.Bool
			)
			pool := NewReadOnlyPool(ReadOnlyPoolConfig{
				Now: func() time.Time {
					if !enabled.Load() {
						return fixedNow
					}
					active++
					runtime.Gosched()
					if active > 1 {
						concurrent.Store(true)
					}
					runtime.Gosched()
					active--
					return fixedNow
				},
				Open: func(context.Context, store.SessionDBOwner, string) (store.EventReadCloser, error) {
					return &readOnlyPoolTestReader{}, nil
				},
			})

			lease, err := pool.Open(ctx, testSessionDBOwner("sess-read-only-clock"), "clock.db")
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			enabled.Store(true)

			const operationCount = 32
			start := make(chan struct{})
			operationErrors := make(chan error, operationCount)
			var operationWG sync.WaitGroup
			for index := range operationCount {
				operationWG.Go(func() {
					<-start
					if index == 0 {
						operationErrors <- lease.Close(ctx)
						return
					}
					operationErrors <- pool.CloseExpired(ctx)
				})
			}
			close(start)
			operationWG.Wait()
			close(operationErrors)
			for operationErr := range operationErrors {
				if operationErr != nil {
					t.Fatalf("clock-bearing pool operation error = %v", operationErr)
				}
			}
			if concurrent.Load() {
				t.Fatal("configured Now callbacks overlapped")
			}

			enabled.Store(false)
			if err := pool.Close(ctx); err != nil {
				t.Fatalf("Close(pool) error = %v", err)
			}
		})
	})

	t.Run("Should prefer quiescing admission over unrelated expiry cleanup", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
		closeErr := errors.New("expired close sentinel")
		pool := NewReadOnlyPool(ReadOnlyPoolConfig{
			TTL: time.Second,
			Now: func() time.Time {
				return now
			},
			Open: func(_ context.Context, owner store.SessionDBOwner, _ string) (store.EventReadCloser, error) {
				sessionID := owner.SessionID
				reader := &readOnlyPoolTestReader{}
				if sessionID == "sess-read-only-expired-precedence" {
					reader.onClose = func(context.Context) error {
						return closeErr
					}
				}
				return reader, nil
			},
		})

		expiredLease, err := pool.Open(
			ctx,
			testSessionDBOwner("sess-read-only-expired-precedence"),
			"expired-precedence.db",
		)

		if err != nil {
			t.Fatalf("Open(expired) error = %v", err)
		}
		if err := expiredLease.Close(ctx); err != nil {
			t.Fatalf("Close(expired lease) error = %v", err)
		}
		now = now.Add(2 * time.Second)

		releaseQuiescence, err := pool.Quiesce(
			ctx,
			testSessionDBOwner("sess-read-only-quiescing-precedence"),
			"quiescing-precedence.db",
		)

		if err != nil {
			t.Fatalf("Quiesce() error = %v", err)
		}
		lease, openErr := pool.Open(
			ctx,
			testSessionDBOwner("sess-read-only-quiescing-precedence"),
			"quiescing-precedence.db",
		)

		if !errors.Is(openErr, ErrReadOnlyPoolQuiescing) {
			t.Fatalf("Open(quiescing) error = %v, want ErrReadOnlyPoolQuiescing", openErr)
		}
		if lease != nil {
			t.Fatalf("Open(quiescing) lease = %v, want nil", lease)
		}
		releaseQuiescence()

		if err := pool.CloseExpired(ctx); !errors.Is(err, closeErr) {
			t.Fatalf("CloseExpired() error = %v, want expired close sentinel", err)
		}
		if err := pool.Close(ctx); err != nil {
			t.Fatalf("Close(pool) error = %v", err)
		}
	})

	t.Run("Should cancel a same-session waiter without canceling the reserved opener", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		t.Cleanup(cancel)

		started := make(chan struct{})
		release := make(chan struct{})
		pool := NewReadOnlyPool(ReadOnlyPoolConfig{
			Open: func(ctx context.Context, owner store.SessionDBOwner, path string) (store.EventReadCloser, error) {
				close(started)
				select {
				case <-release:
					return &readOnlyPoolTestReader{}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		})

		type openResult struct {
			lease store.EventReadCloser
			err   error
		}
		firstResult := make(chan openResult, 1)
		go func() {
			lease, err := pool.Open(ctx, testSessionDBOwner("sess-read-only-waiter"), "waiter.db")
			firstResult <- openResult{lease: lease, err: err}
		}()

		select {
		case <-started:
		case <-ctx.Done():
			t.Fatalf("Open(first) did not reach the opener: %v", ctx.Err())
		}

		waitingCtx, cancelWaiting := context.WithCancel(ctx)
		cancelWaiting()
		waitingLease, waitingErr := pool.Open(waitingCtx, testSessionDBOwner("sess-read-only-waiter"), "waiter.db")
		if !errors.Is(waitingErr, context.Canceled) {
			t.Fatalf("Open(waiting) error = %v, want context.Canceled", waitingErr)
		}
		if waitingLease != nil {
			t.Fatalf("Open(waiting) lease = %v, want nil", waitingLease)
		}

		close(release)
		select {
		case result := <-firstResult:
			if result.err != nil {
				t.Fatalf("Open(first) error = %v", result.err)
			}
			if err := result.lease.Close(ctx); err != nil {
				t.Fatalf("Close(first lease) error = %v", err)
			}
		case <-ctx.Done():
			t.Fatalf("Open(first) did not complete after release: %v", ctx.Err())
		}

		if err := pool.Close(ctx); err != nil {
			t.Fatalf("Close(pool) error = %v", err)
		}
	})

	t.Run("Should cancel the initiating caller without poisoning the shared opener", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		t.Cleanup(cancel)

		started := make(chan struct{})
		release := make(chan struct{})
		pool := NewReadOnlyPool(ReadOnlyPoolConfig{
			Open: func(ctx context.Context, owner store.SessionDBOwner, path string) (store.EventReadCloser, error) {
				close(started)
				select {
				case <-release:
					return &readOnlyPoolTestReader{}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		})

		type openResult struct {
			lease store.EventReadCloser
			err   error
		}
		openingCtx, cancelOpening := context.WithCancel(ctx)
		firstResult := make(chan openResult, 1)
		go func() {
			lease, err := pool.Open(
				openingCtx,
				testSessionDBOwner("sess-read-only-initiator"),
				"initiator.db",
			)
			firstResult <- openResult{lease: lease, err: err}
		}()

		select {
		case <-started:
		case <-ctx.Done():
			t.Fatalf("Open(first) did not reach the opener: %v", ctx.Err())
		}
		secondResult := make(chan openResult, 1)
		go func() {
			lease, err := pool.Open(
				ctx,
				testSessionDBOwner("sess-read-only-initiator"),
				"initiator.db",
			)
			secondResult <- openResult{lease: lease, err: err}
		}()
		cancelOpening()
		close(release)

		first := <-firstResult
		if first.err != nil {
			t.Fatalf("Open(first canceled after shared open) error = %v", first.err)
		}
		if first.lease == nil {
			t.Fatal("Open(first canceled after shared open) lease = nil, want completed recorder")
		}
		if err := first.lease.Close(ctx); err != nil {
			t.Fatalf("Close(first lease) error = %v", err)
		}

		select {
		case second := <-secondResult:
			if second.err != nil {
				t.Fatalf("Open(second) error = %v", second.err)
			}
			if second.lease == nil {
				t.Fatal("Open(second) lease = nil, want shared recorder")
			}
			if err := second.lease.Close(ctx); err != nil {
				t.Fatalf("Close(second lease) error = %v", err)
			}
		case <-ctx.Done():
			t.Fatalf("Open(second) did not claim the shared opener: %v", ctx.Err())
		}

		if err := pool.Close(ctx); err != nil {
			t.Fatalf("Close(pool) error = %v", err)
		}
	})

	t.Run("Should claim a completed opener even when its caller is canceled", func(t *testing.T) {
		t.Parallel()

		closeCtx := testutil.Context(t)
		for iteration := range 32 {
			openCtx, cancelOpen := context.WithCancel(t.Context())
			var closeCount atomic.Int64
			pool := NewReadOnlyPool(ReadOnlyPoolConfig{
				Open: func(
					context.Context,
					store.SessionDBOwner,
					string,
				) (store.EventReadCloser, error) {
					cancelOpen()
					return &readOnlyPoolTestReader{onClose: func(context.Context) error {
						closeCount.Add(1)
						return nil
					}}, nil
				},
			})

			lease, err := pool.Open(
				openCtx,
				testSessionDBOwner(fmt.Sprintf("sess-read-only-completed-cancel-%d", iteration)),
				fmt.Sprintf("completed-cancel-%d.db", iteration),
			)
			if err != nil {
				t.Fatalf("Open(iteration %d) error = %v, want completed recorder", iteration, err)
			}
			if lease == nil {
				t.Fatalf("Open(iteration %d) lease = nil, want completed recorder", iteration)
			}
			if err := lease.Close(closeCtx); err != nil {
				t.Fatalf("Close(lease iteration %d) error = %v", iteration, err)
			}
			if err := pool.Close(closeCtx); err != nil {
				t.Fatalf("Close(pool iteration %d) error = %v", iteration, err)
			}
			if got := closeCount.Load(); got != 1 {
				t.Fatalf("close count iteration %d = %d, want 1", iteration, got)
			}
		}
	})

	t.Run("Should close an in-flight opener exactly once before completing shutdown", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		t.Cleanup(cancel)

		started := make(chan struct{})
		release := make(chan struct{})
		var (
			closeCount atomic.Int64
			pool       *ReadOnlyPool
		)
		pool = NewReadOnlyPool(ReadOnlyPoolConfig{
			Open: func(ctx context.Context, owner store.SessionDBOwner, path string) (store.EventReadCloser, error) {
				sessionID := owner.SessionID
				if sessionID != "sess-read-only-shutdown-open" {
					return &readOnlyPoolTestReader{}, nil
				}
				close(started)
				select {
				case <-release:
					return &readOnlyPoolTestReader{onClose: func(ctx context.Context) error {
						closeCount.Add(1)
						return pool.Close(ctx)
					}}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		})

		type openResult struct {
			lease store.EventReadCloser
			err   error
		}
		openResultCh := make(chan openResult, 1)
		go func() {
			lease, err := pool.Open(ctx, testSessionDBOwner("sess-read-only-shutdown-open"), "shutdown-open.db")
			openResultCh <- openResult{lease: lease, err: err}
		}()

		select {
		case <-started:
		case <-ctx.Done():
			t.Fatalf("Open(in-flight) did not reach the opener: %v", ctx.Err())
		}

		closeResult := make(chan error, 1)
		go func() {
			closeResult <- pool.Close(ctx)
		}()

		for {
			probe, probeErr := pool.Open(ctx, testSessionDBOwner("sess-read-only-shutdown-probe"), "shutdown-probe.db")
			if errors.Is(probeErr, errReadOnlyPoolClosed) {
				break
			}
			if probeErr != nil {
				t.Fatalf("Open(shutdown probe) error = %v", probeErr)
			}
			if err := probe.Close(ctx); err != nil {
				t.Fatalf("Close(shutdown probe) error = %v", err)
			}
			runtime.Gosched()
		}

		close(release)
		select {
		case err := <-closeResult:
			if err != nil {
				t.Fatalf("Close(pool) error = %v", err)
			}
		case <-ctx.Done():
			t.Fatalf("Close(pool) did not wait for the in-flight opener: %v", ctx.Err())
		}
		select {
		case result := <-openResultCh:
			if !errors.Is(result.err, errReadOnlyPoolClosed) {
				t.Fatalf("Open(in-flight) error = %v, want errReadOnlyPoolClosed", result.err)
			}
			if result.lease != nil {
				t.Fatalf("Open(in-flight) lease = %v, want nil", result.lease)
			}
		case <-ctx.Done():
			t.Fatalf("Open(in-flight) did not return after shutdown: %v", ctx.Err())
		}
		if got, want := closeCount.Load(), int64(1); got != want {
			t.Fatalf("in-flight recorder close count = %d, want %d", got, want)
		}
	})

	t.Run("Should retain close error identity across repeated shutdown calls", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		closeErr := errors.New("close sentinel")
		var closeCount atomic.Int64
		pool := NewReadOnlyPool(ReadOnlyPoolConfig{
			Open: func(context.Context, store.SessionDBOwner, string) (store.EventReadCloser, error) {
				return &readOnlyPoolTestReader{onClose: func(context.Context) error {
					closeCount.Add(1)
					return closeErr
				}}, nil
			},
		})

		lease, err := pool.Open(ctx, testSessionDBOwner("sess-read-only-close-error"), "close-error.db")
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		if err := lease.Close(ctx); err != nil {
			t.Fatalf("Close(lease) error = %v", err)
		}

		firstCloseErr := pool.Close(ctx)
		if !errors.Is(firstCloseErr, closeErr) {
			t.Fatalf("Close(first) error = %v, want close sentinel", firstCloseErr)
		}
		secondCloseErr := pool.Close(ctx)
		if !errors.Is(secondCloseErr, closeErr) {
			t.Fatalf("Close(second) error = %v, want close sentinel", secondCloseErr)
		}
		if got, want := closeCount.Load(), int64(1); got != want {
			t.Fatalf("recorder close count = %d, want %d", got, want)
		}
	})

	t.Run("Should reuse same-session read-only handles until TTL expiry", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		writer, err := OpenSessionDB(ctx, testSessionDBOwner("sess-read-only-pool"), path)
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
			Open: func(ctx context.Context, owner store.SessionDBOwner, path string) (store.EventReadCloser, error) {
				sessionID := owner.SessionID
				openCount.Add(1)
				reader, err := OpenSessionDBReadOnly(ctx, testSessionDBOwner(sessionID), path)
				if err != nil {
					return nil, err
				}
				return &closeCountingReader{
					EventReadCloser: reader,
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
				lease, err := pool.Open(ctx, testSessionDBOwner("sess-read-only-pool"), path)
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

		lease, err := pool.Open(ctx, testSessionDBOwner("sess-read-only-pool"), path)
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

	t.Run("Should refuse another session without reusing the owner entry", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		writer, err := OpenSessionDB(ctx, testSessionDBOwner("sess-read-only-pool-a"), path)
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
			Open: func(ctx context.Context, owner store.SessionDBOwner, path string) (store.EventReadCloser, error) {
				openCount.Add(1)
				reader, err := OpenSessionDBReadOnly(ctx, owner, path)
				if err != nil {
					return nil, err
				}
				return &closeCountingReader{
					EventReadCloser: reader,
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

		first, err := pool.Open(ctx, testSessionDBOwner("sess-read-only-pool-a"), path)
		if err != nil {
			t.Fatalf("Open(first) error = %v", err)
		}
		second, err := pool.Open(ctx, testSessionDBOwner("sess-read-only-pool-b"), path)
		if !errors.Is(err, ErrSessionDBOwnerMismatch) {
			t.Fatalf("Open(second) error = %v, want ErrSessionDBOwnerMismatch", err)
		}
		if second != nil {
			t.Fatal("Open(second) reader = non-nil, want refusal")
		}
		if got, want := openCount.Load(), int64(2); got != want {
			t.Fatalf("read-only open count for distinct sessions = %d, want %d", got, want)
		}
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first lease) error = %v", err)
		}
		if err := pool.Close(ctx); err != nil {
			t.Fatalf("Close(pool) error = %v", err)
		}
		if got, want := closeCount.Load(), int64(1); got != want {
			t.Fatalf("underlying close count = %d, want %d", got, want)
		}
	})

	t.Run("Should unblock in-flight quiescence when the pool closes", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), SessionDatabaseName)
		writer, err := OpenSessionDB(ctx, testSessionDBOwner("sess-read-only-pool-shutdown"), path)
		if err != nil {
			t.Fatalf("OpenSessionDB() error = %v", err)
		}
		if err := writer.Close(ctx); err != nil {
			t.Fatalf("Close(writer) error = %v", err)
		}

		pool := NewReadOnlyPool(ReadOnlyPoolConfig{})
		lease, err := pool.Open(ctx, testSessionDBOwner("sess-read-only-pool-shutdown"), path)
		if err != nil {
			t.Fatalf("Open(pool) error = %v", err)
		}
		quiesceDone := make(chan error, 1)
		go func() {
			_, quiesceErr := pool.Quiesce(ctx, testSessionDBOwner("sess-read-only-pool-shutdown"), path)
			quiesceDone <- quiesceErr
		}()

		for {
			probe, openErr := pool.Open(ctx, testSessionDBOwner("sess-read-only-pool-shutdown"), path)
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

type closeCountingReader struct {
	store.EventReadCloser
	onClose func()
	once    sync.Once
}

func (r *closeCountingReader) Close(ctx context.Context) error {
	err := r.EventReadCloser.Close(ctx)
	r.once.Do(func() {
		if r.onClose != nil {
			r.onClose()
		}
	})
	return err
}

type readOnlyPoolTestReader struct {
	onClose func(context.Context) error
}

type readOnlyPoolTestContextKey struct{}

var _ store.EventReadCloser = (*readOnlyPoolTestReader)(nil)
var _ store.ConversationRewindReader = (*readOnlyPoolTestReader)(nil)

func (r *readOnlyPoolTestReader) Query(
	context.Context,
	store.EventQuery,
) ([]store.SessionEvent, error) {
	return nil, nil
}

func (r *readOnlyPoolTestReader) History(
	context.Context,
	store.EventQuery,
) ([]store.TurnHistory, error) {
	return nil, nil
}

func (*readOnlyPoolTestReader) ListTokenUsage(context.Context) ([]store.TokenUsage, error) {
	return nil, nil
}

func (*readOnlyPoolTestReader) ConversationRewindTarget(
	_ context.Context,
	messageID string,
) (store.ConversationRewindTarget, error) {
	return store.ConversationRewindTarget{MessageID: messageID}, nil
}

func (*readOnlyPoolTestReader) ConversationRewindState(
	context.Context,
) (store.ConversationRewindState, bool, error) {
	return store.ConversationRewindState{}, false, nil
}

func (*readOnlyPoolTestReader) ConversationRewindReceipt(
	context.Context,
	string,
	string,
) (store.ConversationRewindResult, bool, error) {
	return store.ConversationRewindResult{}, false, nil
}

func (r *readOnlyPoolTestReader) Close(ctx context.Context) error {
	if r.onClose != nil {
		return r.onClose(ctx)
	}
	return nil
}

func int64Pointer(value int64) *int64 {
	return &value
}

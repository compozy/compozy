package sessiondb

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func nilSessionContext() context.Context {
	return nil
}

func TestSessionDBAccessorsAndCloseLifecycle(t *testing.T) {
	t.Parallel()

	sessionDB := openTestSessionDB(t, "sess-lifecycle")
	if got, want := sessionDB.Path(), sessionDB.path; got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
	if got, want := sessionDB.SessionID(), "sess-lifecycle"; got != want {
		t.Fatalf("SessionID() = %q, want %q", got, want)
	}

	if err := sessionDB.Close(testutil.Context(t)); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := sessionDB.Close(testutil.Context(t)); err != nil {
		t.Fatalf("Close(second) error = %v, want nil", err)
	}
	if err := sessionDB.Record(testutil.Context(t), SessionEvent{
		TurnID:    "turn-after-close",
		Type:      "agent_message",
		AgentName: "coder",
	}); !errors.Is(err, store.ErrClosed) {
		t.Fatalf("Record(after close) error = %v, want ErrClosed", err)
	}
	if _, err := sessionDB.Query(testutil.Context(t), EventQuery{}); !errors.Is(err, store.ErrClosed) {
		t.Fatalf("Query(after close) error = %v, want ErrClosed", err)
	}
}

func TestJoinSessionCleanupError(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve both primary and cleanup failures", func(t *testing.T) {
		t.Parallel()

		primaryErr := errors.New("primary")
		cleanupErr := errors.New("cleanup")
		err := primaryErr
		joinSessionCleanupError(&err, cleanupErr)
		if !errors.Is(err, primaryErr) || !errors.Is(err, cleanupErr) {
			t.Fatalf("joinSessionCleanupError() error = %v, want primary and cleanup failures", err)
		}
	})
}

func TestSessionDBGuardClauses(t *testing.T) {
	t.Parallel()

	var nilDB *SessionDB
	if got := nilDB.Path(); got != "" {
		t.Fatalf("nil Path() = %q, want empty", got)
	}
	if got := nilDB.SessionID(); got != "" {
		t.Fatalf("nil SessionID() = %q, want empty", got)
	}
	if err := nilDB.Close(testutil.Context(t)); err != nil {
		t.Fatalf("nil Close() error = %v", err)
	}

	sessionDB := openTestSessionDB(t, "sess-guards")
	if err := sessionDB.Record(
		nilSessionContext(),
		SessionEvent{TurnID: "turn-1", Type: "agent_message", AgentName: "coder"},
	); err == nil {
		t.Fatal("Record(nil ctx) error = nil, want non-nil")
	}
	if err := sessionDB.Record(testutil.Context(t), SessionEvent{
		SessionID: "wrong",
		TurnID:    "turn-1",
		Type:      "agent_message",
		AgentName: "coder",
	}); err == nil {
		t.Fatal("Record(mismatched session id) error = nil, want non-nil")
	}
	if err := sessionDB.RecordTokenUsage(nilSessionContext(), TokenUsage{TurnID: "turn-1"}); err == nil {
		t.Fatal("RecordTokenUsage(nil ctx) error = nil, want non-nil")
	}
	if _, err := sessionDB.Query(nilSessionContext(), EventQuery{}); err == nil {
		t.Fatal("Query(nil ctx) error = nil, want non-nil")
	}
	if err := sessionDB.Close(nilSessionContext()); err == nil {
		t.Fatal("Close(nil ctx) error = nil, want non-nil")
	}
}

func TestSessionDBInternalWriteHelpers(t *testing.T) {
	t.Parallel()

	sessionDB := openTestSessionDB(t, "sess-internal")

	canceledCtx, cancel := context.WithCancel(testutil.Context(t))
	cancel()
	if result := sessionDB.executeWrite(
		sessionWriteRequest{ctx: canceledCtx, kind: sessionWriteEvent},
	); result.err == nil {
		t.Fatal("executeWrite(canceled) error = nil, want non-nil")
	}
	if result := sessionDB.executeWrite(
		sessionWriteRequest{ctx: testutil.Context(t), kind: sessionWriteKind(99)},
	); result.err == nil {
		t.Fatal("executeWrite(unsupported kind) error = nil, want non-nil")
	}

	blocked := &SessionDB{writeCh: make(chan sessionWriteRequest), shutdownCh: make(chan sessionShutdownRequest, 1)}
	blocked.state.Store(sessionStateOpen)
	if err := blocked.enqueueWrite(canceledCtx, sessionWriteRequest{
		ctx:    canceledCtx,
		kind:   sessionWriteEvent,
		result: make(chan sessionWriteResult, 1),
	}); err == nil {
		t.Fatal("enqueueWrite(canceled) error = nil, want non-nil")
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(testutil.Context(t), time.Nanosecond)
	defer timeoutCancel()
	time.Sleep(time.Millisecond)
	if err := waitForShutdownResult(timeoutCtx, make(chan error)); !errors.Is(err, store.ErrDrainTimeout) {
		t.Fatalf("waitForShutdownResult(timeout) error = %v, want ErrDrainTimeout", err)
	}
	if err := waitForWriterExit(timeoutCtx, make(chan struct{})); !errors.Is(err, store.ErrDrainTimeout) {
		t.Fatalf("waitForWriterExit(timeout) error = %v, want ErrDrainTimeout", err)
	}

	done := make(chan struct{})
	close(done)
	if err := waitForWriterExit(testutil.Context(t), done); err != nil {
		t.Fatalf("waitForWriterExit(done) error = %v", err)
	}
	if err := waitForWriterExit(testutil.Context(t), nil); err != nil {
		t.Fatalf("waitForWriterExit(nil) error = %v", err)
	}

	writerCtx, cancelWriter := context.WithCancel(context.Background())
	writerStopped := make(chan struct{})
	canceledWriter := &SessionDB{
		writeCh:    make(chan sessionWriteRequest),
		shutdownCh: make(chan sessionShutdownRequest, 1),
		writerCtx:  writerCtx,
	}
	go func() {
		defer close(writerStopped)
		canceledWriter.writerLoop()
	}()
	cancelWriter()
	if err := waitForWriterExit(testutil.Context(t), writerStopped); err != nil {
		t.Fatalf("waitForWriterExit(canceled writer) error = %v", err)
	}

	drainReq := sessionWriteRequest{
		ctx:    testutil.Context(t),
		kind:   sessionWriteEvent,
		event:  SessionEvent{ID: "event-1", TurnID: "turn-1", Type: "agent_message", AgentName: "coder"},
		result: make(chan sessionWriteResult, 1),
	}
	draining := &SessionDB{
		db:        sessionDB.db,
		sessionID: "sess-internal",
		writeCh:   make(chan sessionWriteRequest, 1),
		now: func() time.Time {
			return time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
		},
	}
	draining.writeCh <- drainReq
	if err := draining.drainWrites(testutil.Context(t)); err != nil {
		t.Fatalf("drainWrites() error = %v", err)
	}
	if result := <-drainReq.result; result.err != nil {
		t.Fatalf("drainWrites() result = %v", result.err)
	}
}

func TestOpenSessionSQLiteCreatesSchema(t *testing.T) {
	t.Parallel()

	db, err := openSessionSQLite(testutil.Context(t), filepath.Join(t.TempDir(), SessionDatabaseName))
	if err != nil {
		t.Fatalf("openSessionSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var count int
	if err := db.QueryRowContext(testutil.Context(t), `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='events'`).
		Scan(&count); err != nil {
		t.Fatalf("QueryRowContext() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("events table count = %d, want 1", count)
	}
	if got, err := currentMaxSequence(testutil.Context(t), db); err != nil || got != 0 {
		t.Fatalf("currentMaxSequence() = (%d, %v), want (0, nil)", got, err)
	}
}

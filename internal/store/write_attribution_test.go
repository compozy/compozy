package store

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Not parallel: cases install the process-wide write observer.
func TestWriteObservation(t *testing.T) {
	// Invariant UT-043: both write entry points reach the one executor measurement, once per call, with
	// retries inside it. Owner: store executor.
	t.Run("Should observe each entry point once with retries inside one measurement [UT-043]", func(t *testing.T) {
		observed := recordWriteObservations(t)
		ctx := t.Context()
		db := openExecuteWriteTestDB(t, filepath.Join(t.TempDir(), "observed.db"))
		attempts := 0
		err := ExecuteWriteOperation(ctx, db, "observe.retention", func(context.Context, *WriteTx) error {
			attempts++
			if attempts == 1 {
				return errSQLiteCheckpointBusy
			}
			return nil
		})
		if err != nil || attempts != 2 {
			t.Fatalf("ExecuteWriteOperation() error = %v after %d attempts, want success on the retry", err, attempts)
		}
		if err := ExecuteWrite(ctx, db, func(context.Context, *WriteTx) error { return nil }); err != nil {
			t.Fatalf("ExecuteWrite() error = %v", err)
		}

		retention := observed.named("observe.retention")
		if len(retention) != 1 || retention[0].Err != nil || retention[0].Coalesced {
			t.Fatalf("observe.retention observations = %+v, want one successful non-coalesced write", retention)
		}
		if retention[0].Duration <= 0 || retention[0].AdmissionWait > retention[0].Duration {
			t.Fatalf("observation duration %s admission wait %s, want wait within a positive duration",
				retention[0].Duration, retention[0].AdmissionWait)
		}
		caller := observed.named(
			"github.com/compozy/compozy/internal/store.TestWriteObservation.func1")
		if len(caller) != 1 {
			t.Fatalf("ExecuteWrite observations = %+v, want one named after its caller", observed.all())
		}
	})

	// Invariant: a write counts toward its turn only once its transaction begins; failed executed writes count, and a
	// coalesced attribution reaches the observer. Owner: store executor.
	t.Run("Should count only writes that began a transaction toward the attributed turn", func(t *testing.T) {
		observed := recordWriteObservations(t)
		ctx := t.Context()
		db := openExecuteWriteTestDB(t, filepath.Join(t.TempDir(), "attributed.db"))
		account := NewTurnWriteAccount()
		attributed := WithWriteAttribution(ctx, WriteAttribution{Turn: account, Coalesced: true})
		failure := errors.New("callback failed")
		if err := ExecuteWriteOperation(attributed, db, "turn.failed", func(context.Context, *WriteTx) error {
			return failure
		}); !errors.Is(err, failure) {
			t.Fatalf("ExecuteWriteOperation() error = %v, want %v", err, failure)
		}

		started, release := make(chan struct{}), make(chan struct{})
		done := make(chan error, 1)
		go func() {
			done <- ExecuteWriteOperation(ctx, db, "turn.holder", func(context.Context, *WriteTx) error {
				close(started)
				<-release
				return nil
			})
		}()
		<-started
		waitCtx, cancel := context.WithTimeout(attributed, 20*time.Millisecond)
		defer cancel()
		err := ExecuteWriteOperation(waitCtx, db, "turn.admission", func(context.Context, *WriteTx) error {
			t.Error("write canceled at admission must not execute")
			return nil
		})
		close(release)
		if holderErr := <-done; holderErr != nil {
			t.Fatal(holderErr)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("admission-blocked write error = %v, want the caller deadline", err)
		}

		// Admitted but the pool has no free connection: no transaction begins, so the turn must not count it.
		db.SetMaxOpenConns(1)
		held, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("db.Conn() error = %v", err)
		}
		poolCtx, cancelPool := context.WithTimeout(attributed, 20*time.Millisecond)
		defer cancelPool()
		poolErr := ExecuteWriteOperation(poolCtx, db, "turn.pool", func(context.Context, *WriteTx) error {
			t.Error("write without a connection must not execute")
			return nil
		})
		if closeErr := held.Close(); closeErr != nil {
			t.Fatalf("held.Close() error = %v", closeErr)
		}
		if !errors.Is(poolErr, context.DeadlineExceeded) {
			t.Fatalf("pool-blocked write error = %v, want the caller deadline", poolErr)
		}
		if pooled := observed.named("turn.pool"); len(pooled) != 1 || pooled[0].AdmissionWait >= pooled[0].Duration {
			t.Fatalf("turn.pool observations = %+v, want one admitted write that never began", pooled)
		}

		failed := observed.named("turn.failed")
		if len(failed) != 1 || !failed[0].Coalesced || !errors.Is(failed[0].Err, failure) {
			t.Fatalf("turn.failed observations = %+v, want one coalesced failure", failed)
		}
		canceled := observed.named("turn.admission")
		if len(canceled) != 1 || canceled[0].AdmissionWait != canceled[0].Duration {
			t.Fatalf("turn.admission observations = %+v, want one write that never left admission", canceled)
		}
		flushes, total := account.Close()
		if flushes != 1 || total != failed[0].Duration {
			t.Fatalf("account = %d writes %s, want only the executed write (%s)", flushes, total, failed[0].Duration)
		}
	})
}

func TestTurnWriteAccount(t *testing.T) {
	t.Parallel()

	// Invariant UT-044: the account sums concurrent writes and ignores writes after the turn closed it.
	t.Run("Should sum concurrent writes and ignore writes after Close [UT-044]", func(t *testing.T) {
		t.Parallel()
		account := NewTurnWriteAccount()
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				for range 100 {
					account.Add(time.Millisecond)
				}
			})
		}
		wg.Wait()
		flushes, total := account.Close()
		if flushes != 800 || total != 800*time.Millisecond {
			t.Fatalf("Close() = %d, %s, want 800, 800ms", flushes, total)
		}
		account.Add(time.Second)
		if flushes, total := account.Close(); flushes != 800 || total != 800*time.Millisecond {
			t.Fatalf("Close() after a late Add = %d, %s, want 800, 800ms", flushes, total)
		}
	})
}

type writeObservations struct {
	mu       sync.Mutex
	observed []WriteObservation
}

// recordWriteObservations installs a capturing write observer until the test ends.
func recordWriteObservations(t *testing.T) *writeObservations {
	t.Helper()
	recorded := &writeObservations{}
	SetWriteObserver(func(_ context.Context, write WriteObservation) {
		recorded.mu.Lock()
		defer recorded.mu.Unlock()
		recorded.observed = append(recorded.observed, write)
	})
	t.Cleanup(func() { SetWriteObserver(nil) })
	return recorded
}

func (w *writeObservations) all() []WriteObservation {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]WriteObservation(nil), w.observed...)
}

func (w *writeObservations) named(operation string) []WriteObservation {
	var matched []WriteObservation
	for _, write := range w.all() {
		if write.Operation == operation {
			matched = append(matched, write)
		}
	}
	return matched
}

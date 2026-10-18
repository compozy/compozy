package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// WriteContentionError preserves SQLite error identity and safe write provenance.
// Owner is empty when the lock belongs to an untracked transaction or another
// database handle/process; SQLite does not expose that owner's identity.
type WriteContentionError struct {
	Operation     string
	Attempts      int
	Wait          time.Duration
	Owner         string
	TransactionID uint64
	OwnerElapsed  time.Duration
	Err           error
}

func (e *WriteContentionError) Error() string {
	return fmt.Sprintf(
		"store: sqlite contention operation=%s attempts=%d wait=%s owner=%s transaction=%d owner_elapsed=%s: %v",
		e.Operation,
		e.Attempts,
		e.Wait,
		e.Owner,
		e.TransactionID,
		e.OwnerElapsed,
		e.Err,
	)
}

func (e *WriteContentionError) Unwrap() error { return e.Err }

type writeOwner struct {
	operation string
	id        uint64
	started   time.Time
}

var writeOwners = struct {
	sync.Mutex
	active map[*sql.DB]writeOwner
}{active: make(map[*sql.DB]writeOwner)}

var writeTransactionSequence atomic.Uint64

// ExecuteWriteOperation names a code-owned operation, never SQL or request data.
func ExecuteWriteOperation(
	ctx context.Context,
	db *sql.DB,
	operation string,
	fn func(context.Context, *WriteTx) error,
) error {
	cfg := defaultExecuteWriteConfig()
	cfg.operation = operation
	return executeWrite(ctx, db, cfg, fn)
}

func writeCallerOperation() string {
	pc, _, _, ok := runtime.Caller(2)
	if !ok {
		return defaultWriteOperation
	}
	if fn := runtime.FuncForPC(pc); fn != nil {
		return fn.Name()
	}
	return defaultWriteOperation
}

func trackWriteOwner(db *sql.DB, operation string) func() {
	owner := writeOwner{operation: operation, id: writeTransactionSequence.Add(1), started: time.Now()}
	writeOwners.Lock()
	writeOwners.active[db] = owner
	writeOwners.Unlock()
	return func() {
		writeOwners.Lock()
		delete(writeOwners.active, db)
		writeOwners.Unlock()
		if elapsed := time.Since(owner.started); elapsed >= time.Second {
			slog.Warn("sqlite write transaction held for at least one second",
				"operation", owner.operation, "transaction_id", owner.id, "elapsed_ms", elapsed.Milliseconds())
		}
	}
}

func writeContentionError(db *sql.DB, operation string, attempts int, started time.Time, cause error) error {
	writeOwners.Lock()
	owner, known := writeOwners.active[db]
	writeOwners.Unlock()
	err := &WriteContentionError{Operation: operation, Attempts: attempts, Wait: time.Since(started), Err: cause}
	if known {
		err.Owner, err.TransactionID, err.OwnerElapsed = owner.operation, owner.id, time.Since(owner.started)
	}
	slog.Warn("sqlite write contention exhausted bounded retry",
		"operation", operation, "attempts", attempts, "wait_ms", err.Wait.Milliseconds(),
		"owner_known", known, "owner_operation", err.Owner, "transaction_id", err.TransactionID,
		"owner_elapsed_ms", err.OwnerElapsed.Milliseconds())
	return err
}

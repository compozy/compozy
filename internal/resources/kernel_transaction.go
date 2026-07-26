package resources

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/store"
)

func rollbackTx(tx *sql.Tx) error {
	if tx == nil {
		return nil
	}
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return err
	}
	return nil
}

func rollbackImmediate(ctx context.Context, conn *sql.Conn) error {
	if conn == nil {
		return nil
	}
	if _, err := conn.ExecContext(ctx, "ROLLBACK"); err != nil {
		return err
	}
	return nil
}

func joinCleanupError(target *error, cleanupErr error) {
	if cleanupErr == nil || target == nil {
		return
	}
	if *target == nil {
		*target = cleanupErr
		return
	}
	*target = errors.Join(*target, cleanupErr)
}

func (k *Kernel) withImmediateTransaction(
	ctx context.Context,
	action string,
	run func(exec sqlExecutor) error,
) error {
	if err := store.ExecuteWrite(ctx, k.db, func(_ context.Context, tx *store.WriteTx) error {
		return run(tx)
	}); err != nil {
		return fmt.Errorf("resources: %s transaction: %w", action, err)
	}
	return nil
}

func (k *Kernel) lockSource(source ResourceSource) func() {
	key := string(source.Kind) + "\x00" + source.ID

	k.sourceLocksMu.Lock()
	lock, ok := k.sourceLocks[key]
	if !ok {
		lock = &sourceLock{}
		k.sourceLocks[key] = lock
	}
	lock.refs++
	k.sourceLocksMu.Unlock()

	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()

		k.sourceLocksMu.Lock()
		defer k.sourceLocksMu.Unlock()

		lock.refs--
		if lock.refs == 0 {
			delete(k.sourceLocks, key)
		}
	}
}

func resourceKey(kind ResourceKind, id string) string {
	return string(kind) + "\x00" + id
}

func recordsEqual(left RawRecord, right RawRecord) bool {
	return left.Kind == right.Kind &&
		left.ID == right.ID &&
		left.Scope == right.Scope &&
		left.Owner == right.Owner &&
		left.Source == right.Source &&
		bytes.Equal(left.SpecJSON, right.SpecJSON)
}

func nullableScopeID(scope ResourceScope) any {
	if scope.Kind == ResourceScopeKindGlobal {
		return nil
	}
	return scope.ID
}

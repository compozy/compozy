package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// ProfileWriteExecutor is the transaction surface used by the profile domain.
// The profile manager owns cross-table lifecycle mutations, while GlobalDB
// retains ownership of SQLite write serialization.
type ProfileWriteExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// ExecuteProfileWrite runs one profile lifecycle mutation in an immediate
// SQLite transaction.
func (g *GlobalDB) ExecuteProfileWrite(
	ctx context.Context,
	action string,
	run func(ProfileWriteExecutor) error,
) error {
	if g == nil || g.db == nil {
		return errors.New("store: global database is required")
	}
	if ctx == nil {
		return fmt.Errorf("store: %s context is required", action)
	}
	if run == nil {
		return fmt.Errorf("store: %s callback is required", action)
	}
	if err := store.ExecuteWrite(ctx, g.db, func(_ context.Context, tx *store.WriteTx) error {
		return run(tx)
	}); err != nil {
		return fmt.Errorf("store: %s transaction: %w", action, err)
	}
	return nil
}

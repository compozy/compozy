package store

import (
	"context"
	"database/sql"
)

// EnsureSchema executes each schema statement in order.
func EnsureSchema(ctx context.Context, db *sql.DB, statements []string) error {
	for _, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

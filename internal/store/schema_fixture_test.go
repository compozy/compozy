package store

import (
	"context"
	"database/sql"
	"fmt"
)

func executeTestSchema(ctx context.Context, db *sql.DB, statements []string) error {
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("execute test schema statement: %w", err)
		}
	}
	return nil
}

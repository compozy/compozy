package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// DeleteSession removes one durable session catalog row and tombstones any derive
// receipt whose child it was, in the same transaction.
func (g *SessionRepo) DeleteSession(ctx context.Context, sessionID string) error {
	if err := g.checkReady(ctx, "delete session"); err != nil {
		return err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return fmt.Errorf("store: session id is required")
	}
	deletedAt := g.now()
	return g.withImmediateTransaction(ctx, "delete session", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		affected, err := queries.DeleteSession(ctx, target)
		if err != nil {
			return fmt.Errorf("store: delete session %q: %w", target, err)
		}
		if affected == 0 {
			return fmt.Errorf("store: delete session %q: %w", target, store.ErrSessionNotFound)
		}
		return markDerivationChildDeleted(ctx, queries, target, deletedAt)
	})
}

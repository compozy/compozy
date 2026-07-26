package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
)

// DeleteSession removes one durable session catalog row.
func (g *SessionRepo) DeleteSession(ctx context.Context, sessionID string) error {
	if err := g.checkReady(ctx, "delete session"); err != nil {
		return err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return fmt.Errorf("store: session id is required")
	}
	affected, err := g.queries.DeleteSession(ctx, target)
	if err != nil {
		return fmt.Errorf("store: delete session %q: %w", target, err)
	}
	if affected == 0 {
		return fmt.Errorf("store: delete session %q: %w", target, store.ErrSessionNotFound)
	}
	return nil
}

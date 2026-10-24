package globaldb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// UpdateSessionLineages rewrites the hierarchy columns of every listed session in
// one transaction. A missing session aborts the whole batch.
func (g *SessionRepo) UpdateSessionLineages(
	ctx context.Context,
	workspaceID string,
	updates []store.SessionLineageUpdate,
) error {
	if err := g.checkReady(ctx, "update session lineages"); err != nil {
		return err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if len(updates) == 0 {
		return nil
	}
	params := make([]sqlcgen.UpdateSessionLineageHierarchyParams, 0, len(updates))
	for i := range updates {
		update := &updates[i]
		sessionID := strings.TrimSpace(update.SessionID)
		if sessionID == "" {
			return errors.New("store: session lineage update session id is required")
		}
		lineage := store.NormalizeSessionLineage(sessionID, &update.Lineage)
		if err := store.ValidateSessionLineage(sessionID, lineage); err != nil {
			return fmt.Errorf("store: validate lineage update for %q: %w", sessionID, err)
		}
		updatedAt := update.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = g.now()
		}
		params = append(params, sqlcgen.UpdateSessionLineageHierarchyParams{
			ParentSessionID:  nullableSessionString(lineage.ParentSessionID),
			RootSessionID:    nullableSessionString(lineage.RootSessionID),
			SpawnDepth:       int64(lineage.SpawnDepth),
			LineageKind:      string(lineage.Kind),
			AutoStopOnParent: lineage.AutoStopOnParent,
			UpdatedAt:        store.FormatTimestamp(updatedAt),
			WorkspaceID:      workspaceID,
			ID:               sessionID,
		})
	}
	return g.withImmediateTransaction(ctx, "update session lineages", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		for i := range params {
			affected, err := q.UpdateSessionLineageHierarchy(ctx, params[i])
			if err != nil {
				return fmt.Errorf("update lineage for %q: %w", params[i].ID, err)
			}
			if affected == 0 {
				return fmt.Errorf("%w: %s", store.ErrSessionNotFound, params[i].ID)
			}
		}
		return nil
	})
}

var _ store.SessionLineageStore = (*SessionRepo)(nil)

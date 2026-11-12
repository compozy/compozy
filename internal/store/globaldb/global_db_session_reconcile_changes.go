package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// reconcileSessionChunkSize bounds how many catalog rows one reconcile write transaction holds.
const reconcileSessionChunkSize = 200

var _ store.SessionChangeReconciler = (*SessionRepo)(nil)

// ReconcileSessionChanges upserts the changed on-disk sessions in bounded transactions and marks
// orphaned every catalog session that is neither changed nor listed as unchanged.
func (g *SessionRepo) ReconcileSessionChanges(
	ctx context.Context,
	changed []store.SessionInfo,
	unchanged []string,
) (store.ReconcileResult, error) {
	if err := g.checkReady(ctx, "reconcile session changes"); err != nil {
		return store.ReconcileResult{}, err
	}
	sessions, present, err := g.normalizeReconciledSessions(changed)
	if err != nil {
		return store.ReconcileResult{}, err
	}
	for _, id := range unchanged {
		if id = strings.TrimSpace(id); id != "" {
			present[id] = struct{}{}
		}
	}

	result := store.ReconcileResult{Indexed: make([]string, 0), Orphaned: make([]string, 0)}
	for start := 0; start < len(sessions); start += reconcileSessionChunkSize {
		chunk := sessions[start:min(start+reconcileSessionChunkSize, len(sessions))]
		var indexed []string
		chunkErr := g.withImmediateTransaction(ctx, "reconcile session changes", func(exec globalSQLExecutor) error {
			existing, err := g.loadSessionIDs(ctx, exec)
			if err != nil {
				return err
			}
			attempt := make([]string, 0)
			for index := range chunk {
				session := &chunk[index]
				if _, ok := existing[session.ID]; !ok {
					attempt = append(attempt, session.ID)
				}
				if err := g.registerSession(ctx, exec, session); err != nil {
					return fmt.Errorf("store: reconcile session %q: %w", session.ID, err)
				}
			}
			indexed = attempt
			return nil
		})
		if chunkErr != nil {
			return store.ReconcileResult{}, chunkErr
		}
		result.Indexed = append(result.Indexed, indexed...)
	}

	err = g.withImmediateTransaction(ctx, "reconcile orphaned sessions", func(exec globalSQLExecutor) error {
		existing, err := g.loadSessionIDs(ctx, exec)
		if err != nil {
			return err
		}
		orphaned := make([]string, 0)
		orphanedAt := store.FormatTimestamp(g.now())
		queries := sqlcgen.New(exec)
		for id := range existing {
			if _, ok := present[id]; ok {
				continue
			}
			if err := queries.MarkSessionOrphaned(ctx, sqlcgen.MarkSessionOrphanedParams{
				State: sessionStateOrphaned, UpdatedAt: orphanedAt, ID: id,
			}); err != nil {
				return fmt.Errorf("store: mark orphaned session %q: %w", id, err)
			}
			orphaned = append(orphaned, id)
		}
		result.Orphaned = orphaned
		return nil
	})
	if err != nil {
		return store.ReconcileResult{}, err
	}
	return result, nil
}

// normalizeReconciledSessions validates and de-duplicates the sessions a reconcile writes.
func (g *SessionRepo) normalizeReconciledSessions(
	sessions []store.SessionInfo,
) ([]store.SessionInfo, map[string]struct{}, error) {
	normalized := make([]store.SessionInfo, 0, len(sessions))
	workspaces := make(map[string]string, len(sessions))
	present := make(map[string]struct{}, len(sessions))
	for index := range sessions {
		session := sessions[index]
		if err := session.Validate(); err != nil {
			return nil, nil, err
		}
		if session.CreatedAt.IsZero() {
			session.CreatedAt = g.now()
		}
		if session.UpdatedAt.IsZero() {
			session.UpdatedAt = session.CreatedAt
		}
		if workspaceID, ok := workspaces[session.ID]; ok {
			if workspaceID != session.WorkspaceID {
				return nil, nil, fmt.Errorf("%w: %s", store.ErrSessionWorkspaceMismatch, session.ID)
			}
			continue
		}
		workspaces[session.ID] = session.WorkspaceID
		present[session.ID] = struct{}{}
		normalized = append(normalized, session)
	}
	return normalized, present, nil
}

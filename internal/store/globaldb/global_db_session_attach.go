package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// SweepExpiredSessionAttachLocks clears expired attach leases from the session catalog.
func (g *SessionRepo) SweepExpiredSessionAttachLocks(ctx context.Context, now time.Time) (int64, error) {
	if err := g.checkReady(ctx, "sweep expired session attach locks"); err != nil {
		return 0, err
	}
	if now.IsZero() {
		now = g.now()
	}
	now = now.UTC()
	nowRaw := store.FormatTimestamp(now)
	expiredLocks, err := g.readQueries.CountExpiredSessionAttachLocks(ctx, nullableSessionTime(now))
	if err != nil {
		return 0, fmt.Errorf("store: count expired session attach locks: %w", err)
	}
	if expiredLocks == 0 {
		return 0, nil
	}
	rows, err := executeWriteQueryResult(
		ctx,
		g.repoBase,
		"globaldb.SweepExpiredSessionAttachLocks",
		func(ctx context.Context, queries *sqlcgen.Queries) (int64, error) {
			return queries.SweepExpiredSessionAttachLocks(ctx, nowRaw)
		},
	)
	if err != nil {
		return 0, fmt.Errorf("store: sweep expired session attach locks: %w", err)
	}
	return rows, nil
}

// AttachSession acquires a short-lived attach lease for a resumable session.
func (g *SessionRepo) AttachSession(ctx context.Context, req store.SessionAttachRequest) (store.SessionAttach, error) {
	if err := g.checkReady(ctx, "attach session"); err != nil {
		return store.SessionAttach{}, err
	}
	normalized := req.Normalize()
	if normalized.Now.IsZero() {
		normalized.Now = g.now().UTC()
	}
	if err := normalized.Validate(); err != nil {
		return store.SessionAttach{}, err
	}
	expiresAt := normalized.Now.Add(normalized.TTL).UTC()
	nowRaw := store.FormatTimestamp(normalized.Now)

	affected, err := executeWriteQueryResult(
		ctx,
		g.repoBase,
		"globaldb.AttachSession",
		func(ctx context.Context, queries *sqlcgen.Queries) (int64, error) {
			return queries.AttachSession(ctx, sqlcgen.AttachSessionParams{
				AttachedTo: normalized.AttachedTo, AttachExpiresAt: nullableSessionTime(expiresAt),
				UpdatedAt: nowRaw, ID: normalized.SessionID,
			})
		},
	)
	if err != nil {
		return store.SessionAttach{}, fmt.Errorf("store: attach session %q: %w", normalized.SessionID, err)
	}
	if affected == 0 {
		if classifyErr := g.classifyAttachFailure(ctx, normalized.SessionID, normalized.Now); classifyErr != nil {
			return store.SessionAttach{}, classifyErr
		}
		return store.SessionAttach{}, fmt.Errorf("%w: %s", store.ErrSessionAttachLocked, normalized.SessionID)
	}
	return store.SessionAttach{
		SessionID:       normalized.SessionID,
		AttachedTo:      normalized.AttachedTo,
		AttachExpiresAt: expiresAt,
		AttachedAt:      normalized.Now,
	}, nil
}

func (g *SessionRepo) classifyAttachFailure(ctx context.Context, sessionID string, now time.Time) error {
	row, err := g.readQueries.GetSessionAttachState(ctx, sessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %s", store.ErrSessionNotFound, sessionID)
		}
		return fmt.Errorf("store: classify attach session %q: %w", sessionID, err)
	}
	if strings.TrimSpace(row.State) != globalDBSessionStateActive ||
		strings.TrimSpace(row.FailureKind.String) != "" ||
		strings.TrimSpace(row.StallState) != "" {
		return fmt.Errorf("%w: %s", store.ErrSessionNotAttachable, sessionID)
	}
	if strings.TrimSpace(row.AttachedTo) == "" ||
		!row.AttachExpiresAt.Valid ||
		strings.TrimSpace(row.AttachExpiresAt.String) == "" {
		return nil
	}
	expiresAt, err := store.ParseTimestamp(row.AttachExpiresAt.String)
	if err != nil {
		return fmt.Errorf("store: parse session attach expiry for %q: %w", sessionID, err)
	}
	if expiresAt.After(now.UTC()) {
		return fmt.Errorf("%w: %s", store.ErrSessionAttachLocked, sessionID)
	}
	return nil
}

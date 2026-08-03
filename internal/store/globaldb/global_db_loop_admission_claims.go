package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// GetAdmissionClaim returns one workspace-scoped admission tombstone.
func (g *LoopRepo) GetAdmissionClaim(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	loopName string,
	sourceKey string,
	eventKey string,
) (looppkg.AdmissionClaim, error) {
	if err := g.checkReady(ctx, "get admission claim"); err != nil {
		return looppkg.AdmissionClaim{}, err
	}
	row, err := g.queries.GetLoopAdmissionClaim(ctx, sqlcgen.GetLoopAdmissionClaimParams{
		WorkspaceID: string(workspaceID),
		LoopName:    loopName,
		SourceKey:   sourceKey,
		EventKey:    eventKey,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return looppkg.AdmissionClaim{}, fmt.Errorf("store: admission claim not found: %w", sql.ErrNoRows)
		}
		return looppkg.AdmissionClaim{}, fmt.Errorf("store: get loop admission claim: %w", err)
	}
	return looppkg.AdmissionClaim{
		WorkspaceID:      looppkg.WorkspaceID(row.WorkspaceID),
		LoopName:         row.LoopName,
		SourceKey:        row.SourceKey,
		EventKey:         row.EventKey,
		LoopRunID:        looppkg.RunID(row.LoopRunID),
		ClaimedAt:        row.ClaimedAt.UTC(),
		ExpiresAt:        row.ExpiresAt.UTC(),
		SuppressedCount:  int(row.SuppressedCount),
		LastSuppressedAt: loopTimePointer(row.LastSuppressedAt),
	}, nil
}

// SweepExpiredAdmissionClaims deletes tombstones only after their pinned retention horizon.
func (g *LoopRepo) SweepExpiredAdmissionClaims(ctx context.Context, before time.Time) (int64, error) {
	if err := g.checkReady(ctx, "sweep expired admission claims"); err != nil {
		return 0, err
	}
	if before.IsZero() {
		return 0, fmt.Errorf("%w: admission claim sweep time is required", looppkg.ErrValidation)
	}
	deleted, err := g.queries.DeleteExpiredLoopAdmissionClaims(ctx, before.UTC())
	if err != nil {
		return 0, fmt.Errorf("store: sweep expired loop admission claims: %w", err)
	}
	return deleted, nil
}

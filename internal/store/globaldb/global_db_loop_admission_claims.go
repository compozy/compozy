package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
		SuppressedCount:  int(row.SuppressedCount),
		LastSuppressedAt: loopTimePointer(row.LastSuppressedAt),
	}, nil
}

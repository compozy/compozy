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

func claimLoopAdmission(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	now time.Time,
) (bool, looppkg.Run, looppkg.AdmissionClaim, error) {
	identity := run.Admission
	if identity == nil {
		return false, looppkg.Run{}, looppkg.AdmissionClaim{}, nil
	}
	admissionIdentity := identity.Identity
	if _, err := exec.ExecContext(ctx, `DELETE FROM loop_admission_claims
		WHERE workspace_id = ? AND loop_name = ? AND source_key = ? AND event_key = ?
		AND expires_at <= ?`, run.WorkspaceID, run.LoopName, admissionIdentity.SourceKey,
		admissionIdentity.EventKey, now.UTC()); err != nil {
		return false, looppkg.Run{}, looppkg.AdmissionClaim{}, fmt.Errorf(
			"store: expire matching Loop admission claim: %w", err,
		)
	}
	result, err := exec.ExecContext(ctx, `INSERT OR IGNORE INTO loop_admission_claims (
		workspace_id, loop_name, source_key, event_key, loop_run_id, claimed_at, expires_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`, run.WorkspaceID, run.LoopName, admissionIdentity.SourceKey,
		admissionIdentity.EventKey, run.ID, now.UTC(), now.UTC().Add(admissionIdentity.Horizon))
	if err != nil {
		return false, looppkg.Run{}, looppkg.AdmissionClaim{}, fmt.Errorf("store: claim Loop admission: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, looppkg.Run{}, looppkg.AdmissionClaim{}, fmt.Errorf(
			"store: inspect Loop admission claim: %w",
			err,
		)
	}
	if affected == 1 {
		return false, looppkg.Run{}, looppkg.AdmissionClaim{}, nil
	}
	claim, err := getAdmissionClaimWithExecutor(ctx, exec, run.WorkspaceID, run.LoopName, admissionIdentity)
	if err != nil {
		return false, looppkg.Run{}, looppkg.AdmissionClaim{}, err
	}
	if _, err := exec.ExecContext(ctx, `UPDATE loop_admission_claims SET
		suppressed_count = suppressed_count + 1, last_suppressed_at = ?
		WHERE workspace_id = ? AND loop_name = ? AND source_key = ? AND event_key = ?`,
		now.UTC(), run.WorkspaceID, run.LoopName, admissionIdentity.SourceKey, admissionIdentity.EventKey); err != nil {
		return false, looppkg.Run{}, looppkg.AdmissionClaim{}, fmt.Errorf(
			"store: count suppressed Loop admission: %w",
			err,
		)
	}
	claim.SuppressedCount++
	suppressedAt := now.UTC()
	claim.LastSuppressedAt = &suppressedAt
	winner, err := getLoopRunByIDWithExecutor(ctx, exec, claim.LoopRunID)
	if err != nil {
		return false, looppkg.Run{}, looppkg.AdmissionClaim{}, err
	}
	if winner.WorkspaceID != run.WorkspaceID {
		return false, looppkg.Run{}, looppkg.AdmissionClaim{}, fmt.Errorf(
			"%w: admission winner escaped workspace scope", looppkg.ErrTransitionConflict,
		)
	}
	if err := appendLoopRunEventWithExecutor(ctx, exec, winner.ID, winner.WorkspaceID,
		loopRunEventDuplicateSuppressed, map[string]any{
			"source_key": admissionIdentity.SourceKey, "event_key": admissionIdentity.EventKey,
			"winner_loop_run_id": winner.ID, "suppressed_count": claim.SuppressedCount,
		}, suppressedAt); err != nil {
		return false, looppkg.Run{}, looppkg.AdmissionClaim{}, err
	}
	return true, winner, claim, nil
}

func getAdmissionClaimWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID looppkg.WorkspaceID,
	loopName string,
	identity looppkg.AdmissionIdentity,
) (looppkg.AdmissionClaim, error) {
	var claim looppkg.AdmissionClaim
	var lastSuppressedAt sql.NullTime
	err := exec.QueryRowContext(ctx, `SELECT workspace_id, loop_name, source_key, event_key,
		loop_run_id, claimed_at, expires_at, suppressed_count, last_suppressed_at FROM loop_admission_claims
		WHERE workspace_id = ? AND loop_name = ? AND source_key = ? AND event_key = ?`,
		workspaceID, loopName, identity.SourceKey, identity.EventKey).Scan(&claim.WorkspaceID,
		&claim.LoopName, &claim.SourceKey, &claim.EventKey, &claim.LoopRunID, &claim.ClaimedAt,
		&claim.ExpiresAt, &claim.SuppressedCount, &lastSuppressedAt)
	if err != nil {
		return looppkg.AdmissionClaim{}, fmt.Errorf("store: load winning Loop admission claim: %w", err)
	}
	claim.ClaimedAt = claim.ClaimedAt.UTC()
	claim.ExpiresAt = claim.ExpiresAt.UTC()
	claim.LastSuppressedAt = loopTimePointer(lastSuppressedAt)
	return claim, nil
}

// SweepAdmissionClaims removes claims whose persisted effective horizon has elapsed.
func (g *LoopRepo) SweepAdmissionClaims(ctx context.Context, now time.Time, limit int) (int, error) {
	if err := g.checkReady(ctx, "sweep Loop admission claims"); err != nil {
		return 0, err
	}
	if now.IsZero() || limit <= 0 || limit > 1000 {
		return 0, fmt.Errorf("%w: admission sweep boundary or limit is invalid", looppkg.ErrValidation)
	}
	result, err := g.db.ExecContext(ctx, `DELETE FROM loop_admission_claims WHERE rowid IN (
		SELECT rowid FROM loop_admission_claims WHERE expires_at <= ? ORDER BY expires_at LIMIT ?
	)`, now.UTC(), limit)
	if err != nil {
		return 0, fmt.Errorf("store: sweep Loop admission claims: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("store: inspect Loop admission sweep: %w", err)
	}
	return int(affected), nil
}

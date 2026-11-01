package globaldb

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ heartbeat.InstanceWakeStore = (*InstanceRepo)(nil)

func (r *InstanceRepo) WakeState(ctx context.Context, id string) (heartbeat.InstanceWakeState, error) {
	row, err := r.queries.GetInstanceWakeState(ctx, id)
	return instanceWakeState(row), err
}
func (r *InstanceRepo) Due(ctx context.Context, now time.Time, limit int) ([]heartbeat.InstanceWakeState, error) {
	if limit < 1 {
		return nil, errors.New("wake due limit must be positive")
	}
	rows, err := r.queries.ListDueInstanceWakes(
		ctx,
		sqlcgen.ListDueInstanceWakesParams{Now: store.FormatTimestamp(now), PageLimit: int64(limit)},
	)
	if err != nil {
		return nil, err
	}
	result := make([]heartbeat.InstanceWakeState, 0, len(rows))
	for _, row := range rows {
		result = append(result, instanceWakeState(row))
	}
	return result, nil
}
func (r *InstanceRepo) Inflight(ctx context.Context) ([]heartbeat.InstanceWakeState, error) {
	rows, err := r.queries.ListInflightInstanceWakes(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]heartbeat.InstanceWakeState, 0, len(rows))
	for _, row := range rows {
		result = append(result, instanceWakeState(row))
	}
	return result, nil
}
func (r *InstanceRepo) Admit(ctx context.Context, req heartbeat.AdmitRequest) (bool, error) {
	if req.InstanceID == "" || req.OccurrenceID == "" || req.Now.IsZero() || req.MaxConcurrent < 1 ||
		(!req.ManualCheck && !req.NextWakeAt.After(req.Now)) {
		return false, errors.New("invalid wake admission")
	}
	manual := 0
	if req.ManualCheck {
		manual = 1
	}
	count, err := r.queries.AdmitInstanceWake(ctx, sqlcgen.AdmitInstanceWakeParams{
		InstanceID: req.InstanceID, OccurrenceID: req.OccurrenceID, Now: store.FormatTimestamp(req.Now),
		NextWakeAt: store.FormatTimestamp(req.NextWakeAt), ManualCheck: manual, MaxConcurrent: int64(req.MaxConcurrent),
	})
	return count == 1, err
}
func (r *InstanceRepo) BindTriage(ctx context.Context, id, occurrence, sessionID string) (bool, error) {
	if id == "" || occurrence == "" || sessionID == "" {
		return false, errors.New("triage binding requires instance occurrence and session")
	}
	count, err := r.queries.BindInstanceTriage(
		ctx,
		sqlcgen.BindInstanceTriageParams{InstanceID: id, OccurrenceID: occurrence, SessionID: sessionID},
	)
	return count == 1, err
}
func (r *InstanceRepo) Skip(ctx context.Context, req heartbeat.SkipRequest) (bool, error) {
	if !req.NextWakeAt.After(req.Now) || !req.ExpiresAt.After(req.Now) {
		return false, errors.New("invalid wake skip schedule")
	}
	changed := false
	err := store.ExecuteWriteOperation(
		ctx,
		r.db,
		"skip instance wake",
		func(ctx context.Context, tx *store.WriteTx) error {
			q := sqlcgen.New(tx)
			count, err := q.SkipInstanceWake(
				ctx,
				sqlcgen.SkipInstanceWakeParams{
					InstanceID:  req.InstanceID,
					ExpectedDue: req.ExpectedDue,
					Now:         store.FormatTimestamp(req.Now),
					NextWakeAt:  store.FormatTimestamp(req.NextWakeAt),
				},
			)
			if err != nil || count == 0 {
				return err
			}
			id, err := store.NewID("hwe")
			if err != nil {
				return err
			}
			if err := q.InsertInstanceWakeEvent(
				ctx,
				sqlcgen.InsertInstanceWakeEventParams{
					ID:         id,
					InstanceID: req.InstanceID,
					Source:     "timer",
					Result:     "skipped",
					Reason:     req.Reason,
					Detail:     req.Detail,
					CreatedAt:  store.FormatTimestamp(req.Now),
					ExpiresAt:  store.FormatTimestamp(req.ExpiresAt),
				},
			); err != nil {
				return err
			}
			changed = true
			return nil
		},
	)
	return changed, err
}
func instanceWakeState(row sqlcgen.AgentInstanceWakeState) heartbeat.InstanceWakeState {
	return heartbeat.InstanceWakeState{
		InstanceID: row.InstanceID, NextWakeAt: row.NextWakeAt, LastWakeAt: row.LastWakeAt, LastDigest: row.LastDigest,
		LastDigestAt: row.LastDigestAt, AutomationCursor: row.AutomationCursor, OccurrenceID: row.OccurrenceID,
		TriageSessionID: row.TriageSessionID, OccurrenceStartedAt: row.OccurrenceStartedAt,
		QuietStreak: row.QuietStreak, FailureStreak: row.FailureStreak, Cancelled: row.OccurrenceCancelled != 0,
	}
}

package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
	"github.com/compozy/compozy/internal/remoteproto"
)

func (q *Queries) SetLifecycleState(ctx context.Context, id, state string, suspendedAt time.Time) error {
	return q.generated.UpdateLifecycleState(ctx, sqlcgen.UpdateLifecycleStateParams{
		AccountID: id, State: state, SuspendedAt: timestamp(suspendedAt),
	})
}

func (q *Queries) RevokeAccountCredentials(ctx context.Context, id string, now time.Time) error {
	at := timestamp(now)
	if err := q.generated.RevokeLifecycleDeviceSessions(ctx, sqlcgen.RevokeLifecycleDeviceSessionsParams{
		AccountID: id, AsOf: at,
	}); err != nil {
		return err
	}
	if err := q.generated.RevokeLifecycleWebSessions(ctx, sqlcgen.RevokeLifecycleWebSessionsParams{
		AccountID: id, AsOf: at,
	}); err != nil {
		return err
	}
	return q.generated.RevokeLifecycleAccountTokens(ctx, sqlcgen.RevokeLifecycleAccountTokensParams{
		AccountID: id, AsOf: at,
	})
}

func (q *Queries) BeginAccountDeletion(ctx context.Context, id string, now time.Time) error {
	if err := q.generated.ExpireLifecycleCheckouts(ctx, sqlcgen.ExpireLifecycleCheckoutsParams{
		AccountID: id, AsOf: timestamp(now),
	}); err != nil {
		return err
	}
	return q.generated.StartAccountDeletion(ctx, sqlcgen.StartAccountDeletionParams{
		AccountID: id, AsOf: timestamp(now),
	})
}

func (q *Queries) AccountDeletionRequestedAt(ctx context.Context, id string) (time.Time, error) {
	at, err := q.generated.AccountDeletionRequestedAt(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	return at.Time, err
}

func (q *Queries) DueAccountDeletions(ctx context.Context, now time.Time) ([]string, error) {
	return q.generated.DueAccountDeletions(ctx, timestamp(now))
}

func (q *Queries) ClaimAccountDeletion(ctx context.Context, id string, now, lease time.Time) (int16, bool, error) {
	step, err := q.generated.ClaimAccountDeletion(ctx, sqlcgen.ClaimAccountDeletionParams{
		AccountID: id, AsOf: timestamp(now), LeaseUntil: timestamp(lease),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	return step, err == nil, err
}

func (q *Queries) LockClaimedAccountDeletion(ctx context.Context, id string, lease time.Time) error {
	_, err := q.generated.LockClaimedAccountDeletion(ctx, sqlcgen.LockClaimedAccountDeletionParams{
		AccountID: id, LeaseUntil: timestamp(lease),
	})
	return authLookupError(err)
}

func (q *Queries) AdvanceAccountDeletion(ctx context.Context, id string, now time.Time) error {
	return q.generated.AdvanceAccountDeletion(ctx, sqlcgen.AdvanceAccountDeletionParams{AccountID: id, AsOf: timestamp(now)})
}

func (q *Queries) RetryAccountDeletion(ctx context.Context, id string, lease, retry time.Time) error {
	return q.generated.RetryAccountDeletion(ctx, sqlcgen.RetryAccountDeletionParams{
		AccountID: id, LeaseUntil: timestamp(lease), RetryAt: timestamp(retry),
	})
}

func (q *Queries) RemoveDeletingAccountIdentities(ctx context.Context, id string) error {
	return q.generated.RemoveDeletingAccountIdentities(ctx, id)
}

func (q *Queries) MarkAccountTunnelsDeleting(ctx context.Context, id string) error {
	return q.generated.MarkLifecycleTunnelsDeleting(ctx, id)
}

func (q *Queries) DeletingAccountTunnels(ctx context.Context, id string) ([]TunnelRef, error) {
	rows, err := q.generated.DeletingAccountTunnelRefs(ctx, id)
	if err != nil {
		return nil, err
	}
	results := make([]TunnelRef, 0, len(rows))
	for _, row := range rows {
		results = append(results, TunnelRef{ComputerID: remoteproto.ComputerID(row.ComputerID), Generation: row.Generation})
	}
	return results, nil
}

func (q *Queries) FinishAccountDeletion(ctx context.Context, id string, now time.Time) error {
	if err := q.generated.InsertDeletedAccount(ctx, sqlcgen.InsertDeletedAccountParams{
		AccountID: id, DeletedAt: timestamp(now),
	}); err != nil {
		return err
	}
	if err := q.generated.CompactDeletedAccountEvents(ctx, id); err != nil {
		return err
	}
	return q.generated.RemoveCloudAccount(ctx, id)
}

func (q *Queries) SweepDeletedAccounts(ctx context.Context, now time.Time) error {
	return q.generated.SweepDeletedAccounts(ctx, timestamp(now.AddDate(0, 0, -90)))
}
